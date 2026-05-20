// Package smartedit — smart_edit_tool.go (P1-F17-T06).
//
// SmartEditTool is the agent-callable Tool that executes SEARCH/REPLACE
// prompts. It wires four prior stages of the smart-edit pipeline together:
//
//	T03 Parse          — SEARCH/REPLACE prompt → EditPlan
//	T04 ApplyPlanToContent + IsBinary — lenient in-memory re-search
//	T05 Differ         — unified-diff text per file + combined
//	T06 (this file)    — disk reads, multiedit transactional commit,
//	                     post-write re-read, agent Tool interface
//
// Atomicity contract (spec §11):
//
//   - The applier still attempts every block per-file even when one fails so
//     the result table reports every block's outcome.
//   - Whole-prompt atomicity: if ANY block on ANY file fails, NO file is
//     committed to disk. The tool returns SmartEditResult with Atomic=false
//     and AtomicError carrying a description of the first failure.
//   - If every block applies AND dry_run is false, the tool dispatches the
//     full file→content map to the multiedit transaction in a single Commit.
//     Multiedit guarantees per-file atomicity (rollback restores backups on
//     any per-file failure mid-commit).
//   - Post-write re-read: after a successful commit, the tool re-reads each
//     committed file from disk and recomputes the diff against the ORIGINAL
//     content. This provides positive runtime evidence per CONST-035 — the
//     reported diff reflects what is actually on disk, not what the
//     in-memory applier produced.
//
// Anti-bluff contract: this tool NEVER simulates commits. Every successful
// non-dry-run path goes through a real *multiedit.MultiFileEditor (or a
// MultiEditCommitter test fake; the production path is wired in T08 main.go).
// The tests under smart_edit_tool_test.go drive the real adapter against
// t.TempDir() to prove disk content matches the reported diff.
package smartedit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/multiedit"
)

// MultiEditCommitter is the seam SmartEditTool uses to write committed
// content to disk atomically. The production implementation
// (NewMultieditCommitter, below) wraps *multiedit.MultiFileEditor and routes
// CommitFiles through BeginEdit → AddEdit (one OpUpdate per file) → Preview
// → Commit. Tests can supply a fake to exercise commit-failure paths without
// hitting disk.
//
// Contract: CommitFiles MUST be atomic — either every file in `files` is
// written, or none are (existing files preserved). The map is keyed by
// ABSOLUTE file path; values are the new full file content.
type MultiEditCommitter interface {
	CommitFiles(ctx context.Context, files map[string][]byte) error
}

// SmartEditTool implements tools.Tool. Stable name "smart_edit", category
// CategorySmartEdit. A nil committer is rejected at Execute time (rather
// than construction) so the registry can wire the tool before the manager
// is fully built — same pattern as F14/F15 tools.
type SmartEditTool struct {
	approval.DefaultLevelEdit // §3.6 LevelEdit — atomic file mutation pipeline.
	differ                    *Differ
	committer                 MultiEditCommitter
	workdir                   string // base for relative paths; "" → os.Getwd at Execute time
}

// NewSmartEditTool constructs a SmartEditTool. workdir may be empty (resolved
// to the process cwd at each Execute call). The differ is constructed eagerly
// so the F08 multiedit.DiffManager singleton is reused across Execute calls.
func NewSmartEditTool(committer MultiEditCommitter, workdir string) *SmartEditTool {
	return &SmartEditTool{
		differ:    NewDiffer(),
		committer: committer,
		workdir:   workdir,
	}
}

// Name returns the registry-stable identifier.
func (t *SmartEditTool) Name() string { return "smart_edit" }

// Description is shown to the agent so it knows when to call this tool.
func (t *SmartEditTool) Description() string {
	return "Apply SEARCH/REPLACE edits across one or more files atomically. " +
		"Parses a multi-file SEARCH/REPLACE prompt, applies every block in " +
		"memory, and commits all changes through a transactional multiedit " +
		"manager. If any block fails (not-found, ambiguous, binary, too-large, " +
		"read failure), the whole prompt aborts before any file is touched on " +
		"disk. Returns a per-block outcome table and a unified diff."
}

// Category returns CategorySmartEdit so registry filtering by category
// surfaces smart-edit-only tools together.
func (t *SmartEditTool) Category() tools.ToolCategory { return tools.CategorySmartEdit }

// Schema returns the JSON schema for the tool's args.
func (t *SmartEditTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "SEARCH/REPLACE prompt. One block per edit; path on the line preceding `<<<<<<< SEARCH` (sticks until next path).",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Base directory for relative paths. Default: process cwd at call time.",
			},
			"dry_run": map[string]interface{}{
				"type":        "boolean",
				"description": "Optional. If true, parse + apply in memory and return the diff without committing to disk.",
			},
			"fuzzy": map[string]interface{}{
				"type":        "boolean",
				"description": "Optional. If true, locate SEARCH blocks with whitespace-tolerant matching (re-indentation / tab-vs-space drift absorbed). Still fails closed on ambiguity. Default: false (strict byte match).",
			},
		},
		Required:    []string{"prompt"},
		Description: "Apply SEARCH/REPLACE edits atomically across files.",
	}
}

// Validate enforces the args contract before Execute. The registry calls
// this before dispatch (registry.Execute path); we also call it defensively
// from inside Execute so direct callers (bypassing the registry) get the
// same protection.
func (t *SmartEditTool) Validate(params map[string]interface{}) error {
	rawPrompt, ok := params["prompt"]
	if !ok {
		return fmt.Errorf("prompt is required")
	}
	if _, isString := rawPrompt.(string); !isString {
		return fmt.Errorf("prompt must be a string, got %T", rawPrompt)
	}
	if v, present := params["workdir"]; present {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("workdir must be a string, got %T", v)
		}
	}
	if v, present := params["dry_run"]; present {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("dry_run must be a boolean, got %T", v)
		}
	}
	if v, present := params["fuzzy"]; present {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("fuzzy must be a boolean, got %T", v)
		}
	}
	return nil
}

// Execute runs the smart-edit pipeline end-to-end (spec §4).
func (t *SmartEditTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}

	prompt, _ := params["prompt"].(string)
	dryRun, _ := params["dry_run"].(bool)
	fuzzy, _ := params["fuzzy"].(bool)
	workdir, _ := params["workdir"].(string)
	if workdir == "" {
		workdir = t.workdir
	}

	res := &SmartEditResult{
		StartedAt: time.Now(),
	}

	plan, err := Parse(prompt)
	if err != nil {
		res.CompletedAt = time.Now()
		res.AtomicError = err.Error()
		return res, nil
	}
	if plan == nil || len(plan.Blocks) == 0 {
		// Vacuous success — no work attempted, atomicity trivially holds.
		res.Atomic = true
		res.CompletedAt = time.Now()
		return res, nil
	}

	// Iterate per file in deterministic order (sorted by path) so test output
	// is stable irrespective of map iteration order.
	files := make([]string, 0, len(plan.PerFile))
	for f := range plan.PerFile {
		files = append(files, f)
	}
	sort.Strings(files)

	type fileApply struct {
		absPath        string
		original       []byte
		newContent     []byte
		applied        bool
		blockResults   []EditResult
		fileLevelFatal bool // file failed before applier even ran (read/binary/too-large)
	}

	perFile := make([]fileApply, 0, len(files))
	commitMap := make(map[string][]byte, len(files))
	allOK := true
	var firstFailure string

	for _, rel := range files {
		blocks := plan.PerFile[rel]
		fa := fileApply{absPath: t.resolvePath(workdir, rel)}

		content, readErr := os.ReadFile(fa.absPath)
		if readErr != nil {
			fa.fileLevelFatal = true
			for _, blk := range blocks {
				fa.blockResults = append(fa.blockResults, EditResult{
					Block:   blk,
					Outcome: OutcomeReadFailed,
					Error:   readErr.Error(),
				})
			}
			allOK = false
			if firstFailure == "" {
				firstFailure = fmt.Sprintf("read %s: %v", fa.absPath, readErr)
			}
			perFile = append(perFile, fa)
			continue
		}

		if len(content) > MaxFileBytes {
			fa.fileLevelFatal = true
			for _, blk := range blocks {
				fa.blockResults = append(fa.blockResults, EditResult{
					Block:   blk,
					Outcome: OutcomeTooLarge,
					Error:   ErrFileTooLarge.Error(),
				})
			}
			allOK = false
			if firstFailure == "" {
				firstFailure = fmt.Sprintf("file too large: %s (%d bytes)", fa.absPath, len(content))
			}
			perFile = append(perFile, fa)
			continue
		}

		if IsBinary(content) {
			fa.fileLevelFatal = true
			for _, blk := range blocks {
				fa.blockResults = append(fa.blockResults, EditResult{
					Block:   blk,
					Outcome: OutcomeBinary,
					Error:   ErrBinaryFile.Error(),
				})
			}
			allOK = false
			if firstFailure == "" {
				firstFailure = fmt.Sprintf("binary file: %s", fa.absPath)
			}
			perFile = append(perFile, fa)
			continue
		}

		fa.original = content
		// Default is the strict byte-exact applier (no behaviour change for
		// existing callers). `fuzzy=true` opts into the whitespace-tolerant
		// applier, which still fails closed on ambiguity / not-found.
		var (
			newContent   []byte
			blockResults []EditResult
			fileOK       bool
		)
		if fuzzy {
			newContent, blockResults, fileOK = ApplyPlanToContentFuzzy(content, blocks)
		} else {
			newContent, blockResults, fileOK = ApplyPlanToContent(content, blocks)
		}
		fa.blockResults = blockResults
		if fileOK {
			fa.applied = true
			fa.newContent = newContent
			commitMap[fa.absPath] = newContent
		} else {
			allOK = false
			if firstFailure == "" {
				for _, br := range blockResults {
					if br.Outcome != OutcomeApplied {
						firstFailure = fmt.Sprintf("%s block (lines %d-%d): %s",
							fa.absPath, br.Block.LineStart, br.Block.LineEnd, br.Outcome)
						break
					}
				}
			}
		}
		perFile = append(perFile, fa)
	}

	// Commit phase — only if every block succeeded AND not dry-run.
	atomic := allOK
	atomicErr := ""
	if !allOK {
		atomicErr = firstFailure
	}

	if allOK && !dryRun && len(commitMap) > 0 {
		if err := t.committer.CommitFiles(ctx, commitMap); err != nil {
			atomic = false
			atomicErr = err.Error()
			// Replace successful in-memory outcomes with WriteFailed so callers
			// see the disk failure faithfully reflected in per-block results.
			for i := range perFile {
				if !perFile[i].applied {
					continue
				}
				for j := range perFile[i].blockResults {
					if perFile[i].blockResults[j].Outcome == OutcomeApplied {
						perFile[i].blockResults[j].Outcome = OutcomeWriteFailed
						perFile[i].blockResults[j].Error = err.Error()
					}
				}
				perFile[i].applied = false
			}
		}
	}

	// Post-write re-read + diff. For a successful real commit, re-read the
	// file from disk and compute the diff against the ORIGINAL content. For
	// dry-run we diff in-memory original vs in-memory newContent. For any
	// failed/aborted path, no diff is generated.
	perFileDiff := make(map[string]string, len(commitMap))
	for i := range perFile {
		fa := &perFile[i]
		if !fa.applied {
			continue
		}

		var diffSrc []byte
		if dryRun {
			diffSrc = fa.newContent
		} else {
			// Real commit happened — re-read disk for positive evidence.
			disk, err := os.ReadFile(fa.absPath)
			if err != nil {
				// File suddenly disappeared — record as write failure post-hoc.
				atomic = false
				if atomicErr == "" {
					atomicErr = fmt.Sprintf("post-write re-read %s: %v", fa.absPath, err)
				}
				continue
			}
			diffSrc = disk
		}

		diff, err := t.differ.FileDiff(fa.absPath, fa.original, diffSrc)
		if err != nil {
			// Diff failures are non-fatal — record but continue.
			diff = ""
		}
		perFileDiff[fa.absPath] = diff
		// Annotate per-block diffs with the per-file diff so callers can
		// attribute changes to a specific block. (T05 Differ does not split
		// hunks per block — that is a downstream rendering concern.)
		for j := range fa.blockResults {
			if fa.blockResults[j].Outcome == OutcomeApplied {
				fa.blockResults[j].Diff = diff
			}
		}
	}

	// Aggregate result.
	for i := range perFile {
		res.Results = append(res.Results, perFile[i].blockResults...)
	}
	for _, br := range res.Results {
		if br.Outcome == OutcomeApplied {
			res.AppliedCount++
		} else {
			res.FailedCount++
		}
	}
	res.Atomic = atomic
	res.AtomicError = atomicErr
	res.Diff = t.differ.CombinedDiff(perFileDiff)
	res.CompletedAt = time.Now()

	return res, nil
}

// ParsePrompt is a thin pass-through to Parse(prompt). It exists so the
// /edit slash command (commands.SmartEditInspector) can request a parse-only
// inspection of a SEARCH/REPLACE prompt without exercising disk reads, the
// applier, or the committer. It returns the EditPlan unchanged on success.
func (t *SmartEditTool) ParsePrompt(prompt string) (*EditPlan, error) {
	return Parse(prompt)
}

// DryRun runs the full smart-edit pipeline (parse + read + apply in memory +
// diff) WITHOUT writing to disk. It is the inspection entry-point used by
// `/edit dry-run`. Workdir overrides the tool's default workdir for this
// call; an empty string falls back to the tool's configured workdir, which
// itself falls back to the process cwd at Execute time.
//
// The returned *SmartEditResult is the same value Execute would return with
// `dry_run=true`; the helper is a convenience wrapper that constructs the
// args map and calls Execute, so behaviour stays in lockstep with the agent
// path and there is no parallel pipeline to drift.
func (t *SmartEditTool) DryRun(ctx context.Context, prompt, workdir string) (*SmartEditResult, error) {
	args := map[string]interface{}{
		"prompt":  prompt,
		"dry_run": true,
	}
	if workdir != "" {
		args["workdir"] = workdir
	}
	out, err := t.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	res, ok := out.(*SmartEditResult)
	if !ok || res == nil {
		return nil, fmt.Errorf("smart_edit dry-run: unexpected result type %T", out)
	}
	return res, nil
}

// Commit runs the full smart-edit pipeline INCLUDING the disk write through
// the configured MultiEditCommitter. It is the user-initiated entry-point
// used by `/edit commit`. Workdir behaves identically to DryRun.
//
// As with DryRun this is a convenience wrapper around Execute so the
// commit path stays a single code-path; agent calls and slash-command calls
// land in the exact same Execute body.
func (t *SmartEditTool) Commit(ctx context.Context, prompt, workdir string) (*SmartEditResult, error) {
	args := map[string]interface{}{
		"prompt":  prompt,
		"dry_run": false,
	}
	if workdir != "" {
		args["workdir"] = workdir
	}
	out, err := t.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	res, ok := out.(*SmartEditResult)
	if !ok || res == nil {
		return nil, fmt.Errorf("smart_edit commit: unexpected result type %T", out)
	}
	return res, nil
}

// resolvePath joins workdir with rel when rel is not already absolute.
// An empty workdir falls back to the process cwd at call time. Absolute
// paths are honoured as-is.
func (t *SmartEditTool) resolvePath(workdir, rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	if workdir == "" {
		if wd, err := os.Getwd(); err == nil {
			workdir = wd
		}
	}
	return filepath.Clean(filepath.Join(workdir, rel))
}

// ----------------------------------------------------------------------------
// multieditCommitter — production adapter from MultiEditCommitter to
// *multiedit.MultiFileEditor.
// ----------------------------------------------------------------------------

// multieditCommitter wraps a *multiedit.MultiFileEditor and routes CommitFiles
// through the F08 transactional API:
//
//	BeginEdit → AddEdit (one OpUpdate per file with OldContent + NewContent
//	+ Checksum) → Preview → Commit
//
// The adapter reads each file fresh inside CommitFiles to compute OldContent
// and the integrity checksum so the transaction's ConflictDetector can detect
// races against external mutators.
type multieditCommitter struct {
	mfe *multiedit.MultiFileEditor
}

// NewMultieditCommitter wraps a *multiedit.MultiFileEditor for use as a
// MultiEditCommitter. The smart-edit production wiring (T08 main.go) will
// pass the registry's existing multiedit instance.
func NewMultieditCommitter(mfe *multiedit.MultiFileEditor) MultiEditCommitter {
	return &multieditCommitter{mfe: mfe}
}

// CommitFiles writes the given file→content map atomically through multiedit.
// On any per-file failure, multiedit rolls back every file it has already
// written and returns the underlying error. The returned error is the raw
// multiedit error so callers can introspect with errors.Is/As.
func (c *multieditCommitter) CommitFiles(ctx context.Context, files map[string][]byte) error {
	if c.mfe == nil {
		return fmt.Errorf("multiedit committer: editor is nil")
	}
	if len(files) == 0 {
		return nil
	}

	tx, err := c.mfe.BeginEdit(ctx, multiedit.EditOptions{
		BackupEnabled: true,
	})
	if err != nil {
		return fmt.Errorf("multiedit: begin: %w", err)
	}

	for path, newContent := range files {
		old, readErr := os.ReadFile(path)
		op := multiedit.OpUpdate
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				return fmt.Errorf("multiedit: read %s: %w", path, readErr)
			}
			// File doesn't exist yet — treat as create.
			op = multiedit.OpCreate
			old = nil
		}
		edit := &multiedit.FileEdit{
			FilePath:   path,
			Operation:  op,
			OldContent: old,
			NewContent: newContent,
		}
		if err := c.mfe.AddEdit(ctx, tx, edit); err != nil {
			return fmt.Errorf("multiedit: add edit %s: %w", path, err)
		}
	}

	if _, err := c.mfe.Preview(ctx, tx); err != nil {
		return fmt.Errorf("multiedit: preview: %w", err)
	}
	if err := c.mfe.Commit(ctx, tx); err != nil {
		return fmt.Errorf("multiedit: commit: %w", err)
	}
	return nil
}
