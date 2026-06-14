package tools

// LSP wiring helper.
//
// WireLSP is the single reusable entry point a top-level binary (the TUI
// main thread, the server, a test) calls to turn on LSP support on a
// ToolRegistry. It owns the four steps that otherwise have to be repeated
// (and kept in sync) at every call site:
//
//  1. detect which curated LSP servers are actually installed on PATH,
//  2. build an LSPManager rooted at the workspace,
//  3. wire that manager onto the registry (so Edit-class tools auto-trigger
//     NotifyChange and lsp_get_diagnostics sees fresh state),
//  4. register the two read-only diagnostics tools so the agent can call
//     them.
//
// The diagnostics tools are pure reads (approval.LevelReadOnly), so they
// pass a ReadOnlyOnly agent tool loop unchanged.
//
// IMPORTANT — open-before-read: gopls (and every LSP server) only publishes
// diagnostics for documents that have been opened via textDocument/didOpen.
// lsp_get_diagnostics therefore returns an EMPTY summary for a file the
// manager has never opened — that is correct, not a bug. Before diagnostics
// for a file are meaningful, something must call LSPManager.NotifyOpen(ctx,
// path) for it. The auto-trigger wired by SetLSPManager calls NotifyChange
// on Edit-class tool success, which keeps an already-open file fresh, but
// the FIRST open still has to happen. Callers that want diagnostics for a
// file the user is merely viewing (not editing) should call
// NotifyOpen(ctx, path) on the *LSPManager that WireLSP returns — see
// WireLSPAndOpen for a convenience that opens a set of files up front.

import (
	"context"

	"go.uber.org/zap"
)

// WireLSP builds an LSPManager for workspaceRoot from the curated server
// allowlist (filtered to servers present on PATH), wires it onto reg, and
// registers the two read-only LSP diagnostics tools (lsp_get_diagnostics,
// lsp_analyze_diagnostic).
//
// It returns the constructed manager so the caller can drive NotifyOpen /
// NotifyChange / Close on it. The manager is always non-nil, even when no
// LSP server is installed: in that case it simply has an empty spec set and
// every EnsureFor/NotifyOpen is a no-op error-free call (so the diagnostics
// tools return empty summaries rather than failing). This keeps the wiring
// branch-free at the call site.
//
// log may be nil (a Nop logger is substituted by NewLSPManager).
func WireLSP(reg *ToolRegistry, workspaceRoot string, log *zap.Logger) *LSPManager {
	specs := DetectAvailableServers(CuratedServerSpecs())
	mgr := NewLSPManager(workspaceRoot, specs, log)
	reg.SetLSPManager(mgr)
	reg.Register(NewLSPGetDiagnosticsTool(mgr))
	reg.Register(NewLSPAnalyzeDiagnosticTool(mgr))
	return mgr
}

// WireLSPAndOpen is WireLSP plus an up-front NotifyOpen for each path in
// openPaths, so diagnostics are populated for files the user is viewing
// (not just editing). Open failures are returned joined but do NOT prevent
// wiring — the manager and tools are wired regardless, and a per-file open
// error simply means that file has no diagnostics yet. Files whose
// extension matches no installed server are silently skipped (NotifyOpen is
// a no-op for them).
func WireLSPAndOpen(ctx context.Context, reg *ToolRegistry, workspaceRoot string, log *zap.Logger, openPaths ...string) (*LSPManager, error) {
	mgr := WireLSP(reg, workspaceRoot, log)
	var firstErr error
	for _, p := range openPaths {
		if err := mgr.NotifyOpen(ctx, p); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return mgr, firstErr
}
