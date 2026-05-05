package tools

// LSP auto-trigger: post-Execute hook that re-publishes diagnostics for files
// edited by Edit-class tools (fs_edit, fs_write, multiedit_commit). Wired up
// via ToolRegistry.SetLSPManager and consulted inside Execute after a
// successful tool run.
//
// Design notes:
//
//   - We deliberately keep the trigger BEST-EFFORT and SYNCHRONOUS with a
//     hard 2s timeout. The auto-trigger never propagates errors back to the
//     caller — diagnostics are a side-channel; the agent reads them via
//     LSPManager.GetDiagnostics in a separate step (T07's lsp_get_diagnostics
//     tool, surfaced to the agent in T10).
//
//   - We do NOT read tool args for content. We re-read the affected file from
//     disk after Execute. This is robust against tools that mutate args, and
//     it matches the semantics agents expect: "the file on disk is the
//     authoritative state."
//
//   - extractEditedPaths returns the paths an Edit-class tool wrote, if those
//     paths are derivable from the args alone (fs_edit, fs_write). For
//     multiedit_commit, the args carry only transaction_id; the registry
//     resolves the path list via the multi-edit subsystem (see
//     resolveAutoTriggerPaths).

import (
	"context"
	"os"
	"time"
)

// autoTriggerTimeout caps how long the post-Execute LSP notification can
// take. LSP servers typically publish in <500ms; 2s leaves headroom for
// cold-start without making the agent loop visibly stall.
const autoTriggerTimeout = 2 * time.Second

// extractEditedPaths returns the file paths edited by an Edit-class tool when
// the paths are derivable from the args alone. Returns an empty slice for
// tools that are not Edit-class, or when the args don't carry enough info to
// pinpoint a path (e.g. multiedit_commit, whose paths live on the
// transaction).
//
// Recognised tools:
//   - fs_edit: args["path"]
//   - fs_write: args["path"]
//   - multiedit_commit: returns nil (paths resolved via transaction lookup)
func extractEditedPaths(toolName string, args map[string]interface{}) []string {
	switch toolName {
	case "fs_edit", "fs_write":
		p, ok := args["path"].(string)
		if !ok || p == "" {
			return nil
		}
		return []string{p}
	case "multiedit_commit":
		// Paths live on the transaction, not in the args. The registry
		// resolves them via resolveAutoTriggerPaths after Execute, while
		// the transaction is still alive in StateCommitted.
		return nil
	default:
		return nil
	}
}

// ExtractEditedPathsForTest is the test-only export of extractEditedPaths.
// Kept intentionally narrow so production code never depends on it.
func ExtractEditedPathsForTest(toolName string, args map[string]interface{}) []string {
	return extractEditedPaths(toolName, args)
}

// resolveAutoTriggerPaths returns the full list of file paths the auto-trigger
// should NotifyChange against, given the tool name + args + the registry's
// component handles. For multiedit_commit, it walks the transaction's Files;
// for fs_edit / fs_write it falls back to extractEditedPaths.
//
// The registry handle is passed in so we can look up the multi-edit
// transaction without taking another lock on the registry mutex.
func (r *ToolRegistry) resolveAutoTriggerPaths(ctx context.Context, toolName string, args map[string]interface{}) []string {
	if paths := extractEditedPaths(toolName, args); len(paths) > 0 {
		return paths
	}
	if toolName != "multiedit_commit" {
		return nil
	}
	txID, ok := args["transaction_id"].(string)
	if !ok || txID == "" {
		return nil
	}
	if r.multiEdit == nil {
		return nil
	}
	tx, err := r.multiEdit.GetTransaction(ctx, txID)
	if err != nil || tx == nil {
		return nil
	}
	out := make([]string, 0, len(tx.Files))
	for _, e := range tx.Files {
		if e == nil || e.FilePath == "" {
			continue
		}
		out = append(out, e.FilePath)
	}
	return out
}

// triggerLSPAfterEdit fires NotifyChange against the manager for each edited
// file, reading current content from disk. Best-effort: errors are swallowed
// so the agent loop never sees an LSP-side failure.
//
// Called from Execute only when:
//   - r.lspManager != nil
//   - the tool returned no error
//   - the tool name is an Edit-class tool
func (r *ToolRegistry) triggerLSPAfterEdit(ctx context.Context, toolName string, args map[string]interface{}) {
	r.mu.RLock()
	mgr := r.lspManager
	r.mu.RUnlock()
	if mgr == nil {
		return
	}
	paths := r.resolveAutoTriggerPaths(ctx, toolName, args)
	if len(paths) == 0 {
		return
	}
	triggerCtx, cancel := context.WithTimeout(ctx, autoTriggerTimeout)
	defer cancel()
	for _, p := range paths {
		// Verify the file still exists on disk. multiedit_commit OpDelete
		// removes the file; nothing useful to push for a deleted path
		// (and NotifyOpen would error reading it). We skip silently.
		if _, err := os.Stat(p); err != nil {
			continue
		}
		// NotifyOpen lazily spawns the right server (matched by file
		// extension) and opens the document — idempotent if already
		// open. After Open we follow up with NotifyChange so subsequent
		// edits to the same file republish diagnostics. Both calls are
		// best-effort; LSP-side errors are swallowed.
		if err := mgr.NotifyOpen(triggerCtx, p); err != nil {
			// NotifyOpen failed (no spec match, server spawn failure,
			// read failure): nothing more we can do — diagnostics simply
			// won't be available for this file.
			continue
		}
		// Read content from disk so NotifyChange receives the
		// authoritative post-edit state (rather than relying on
		// args["content"], which not every Edit-class tool carries —
		// fs_edit only carries old_string/new_string).
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		_ = mgr.NotifyChange(triggerCtx, p, string(content))
	}
}
