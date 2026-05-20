// Package commands — edit_command.go (P1-F17-T07).
//
// EditCommand implements the /edit slash command with four subcommands:
// status (default), diff, dry-run, commit. It is the user-facing inspection
// surface for HelixCode's F17 SEARCH/REPLACE smart-edit feature.
//
// Subcommands:
//
//	/edit                    alias of /edit status
//	/edit status             reports whether the smart_edit tool is wired
//	/edit diff <prompt>      parse the SEARCH/REPLACE prompt and print a
//	                         per-block summary WITHOUT applying anything;
//	                         pure inspection (no disk reads, no committer)
//	/edit dry-run <prompt>   exercise the full pipeline (parse + read +
//	                         apply in memory + diff) but DO NOT write to
//	                         disk; renders the unified diff that WOULD be
//	                         written
//	/edit commit  <prompt>   exercise the full pipeline INCLUDING the
//	                         disk write through the multiedit committer
//
// The prompt may span multiple lines. Two forms are supported:
//
//   - Inline: /edit dry-run <prompt body...>
//     The prompt body is taken from cc.RawInput by stripping the
//     leading "/edit <subcommand>" prefix; this preserves newlines that
//     the slash-command parser would otherwise flatten.
//   - File:   /edit dry-run --from <path>
//     The prompt body is read verbatim from <path>.
//
// Anti-bluff contract: every subcommand that exercises the pipeline calls
// the SmartEditInspector seam — there is no fmt.Printf + sleep simulation
// path. The fake inspector used in tests records exact dispatch (commit
// subcommand → Commit method, NOT DryRun) so any drift is caught by tests.
//
// Production wiring (T08 main.go) hands the command the real
// *smartedit.SmartEditTool, which satisfies SmartEditInspector
// structurally via its ParsePrompt / DryRun / Commit helpers added in
// smart_edit_tool.go.
package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/tools/smartedit"
)

// SmartEditInspector is the subset of *smartedit.SmartEditTool surface that
// EditCommand depends on. Defined here for testability — Go satisfies
// interfaces structurally so the production wiring (T08 main.go) can pass
// the real *smartedit.SmartEditTool directly.
//
// Method semantics:
//
//   - ParsePrompt  — pure parse, no disk access, no applier. Used by /edit
//     diff to render a per-block summary.
//   - DryRun       — full pipeline (parse + read + apply in memory + diff)
//     WITHOUT a disk write. Used by /edit dry-run.
//   - Commit       — full pipeline INCLUDING the disk write. Used by /edit
//     commit.
type SmartEditInspector interface {
	// ParsePrompt parses the SEARCH/REPLACE prompt without applying
	// anything. Returns the EditPlan unchanged from smartedit.Parse.
	ParsePrompt(prompt string) (*smartedit.EditPlan, error)

	// DryRun runs the full apply-in-memory pipeline (no disk write) and
	// returns the result with diff. workdir overrides the inspector's
	// configured workdir for this call; "" falls back to the inspector's
	// own default.
	DryRun(ctx context.Context, prompt, workdir string) (*smartedit.SmartEditResult, error)

	// Commit runs the full pipeline INCLUDING disk write.
	Commit(ctx context.Context, prompt, workdir string) (*smartedit.SmartEditResult, error)
}

// EditCommand is the /edit slash command.
type EditCommand struct {
	inspector SmartEditInspector
}

// NewEditCommand constructs the /edit slash command. A nil inspector is
// allowed: /edit status reports "smart-edit unavailable" in that case so
// the CLI keeps working when the smart-edit tool could not be constructed
// (e.g. multiedit committer init failed during boot). diff/dry-run/commit
// will return an error mentioning "unavailable".
func NewEditCommand(insp SmartEditInspector) *EditCommand {
	return &EditCommand{inspector: insp}
}

// Name returns the slash command name (without the leading slash).
func (c *EditCommand) Name() string { return "edit" }

// Aliases returns alternative invocation names. /edit has none.
func (c *EditCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *EditCommand) Description() string {
	return tr(context.Background(), "internal_commands_edit_description", nil)
}

// Usage returns the usage string shown by /help.
func (c *EditCommand) Usage() string {
	return tr(context.Background(), "internal_commands_edit_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — it answers "is smart-edit
// wired" which is the most common entry-point question.
func (c *EditCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return c.handleStatus(), nil
	case "diff":
		return c.handleDiff(cc)
	case "dry-run":
		return c.handleDryRun(ctx, cc)
	case "commit":
		return c.handleCommit(ctx, cc)
	default:
		return nil, fmt.Errorf("/edit: unknown subcommand %q (want status|diff|dry-run|commit)", sub)
	}
}

// handleStatus renders the smart-edit status. When the inspector is nil
// (smart-edit not wired) we lead with a "smart-edit unavailable" line so
// the user immediately sees why the other subcommands won't work.
func (c *EditCommand) handleStatus() *CommandResult {
	var sb strings.Builder
	sb.WriteString(trc("internal_commands_edit_status_heading", nil) + "\n")
	if c.inspector == nil {
		sb.WriteString("  " + trc("internal_commands_edit_status_unavailable", nil) + "\n")
		return &CommandResult{Success: true, Output: sb.String()}
	}
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  smart-edit:\t%s\n", trc("internal_commands_edit_status_available", nil))
	fmt.Fprintf(tw, "  format:\tSEARCH/REPLACE blocks (markers <<<<<<< SEARCH / ======= / >>>>>>> REPLACE)\n")
	fmt.Fprintf(tw, "  inspection:\t/edit diff <prompt>\n")
	fmt.Fprintf(tw, "  dry-run:\t/edit dry-run <prompt> (no disk write)\n")
	fmt.Fprintf(tw, "  commit:\t/edit commit <prompt> (writes to disk via multiedit)\n")
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// handleDiff parses the prompt and renders a per-block summary WITHOUT
// applying anything. This is the cheapest inspection — no disk reads, no
// committer, no diff. The output mentions the total block count and the
// per-file path with the line range each block spans in the source prompt.
func (c *EditCommand) handleDiff(cc *CommandContext) (*CommandResult, error) {
	if c.inspector == nil {
		return nil, fmt.Errorf("/edit diff: smart-edit unavailable (tool not initialised)")
	}
	prompt, err := c.extractPrompt(cc, "diff")
	if err != nil {
		return nil, err
	}

	plan, err := c.inspector.ParsePrompt(prompt)
	if err != nil {
		return nil, fmt.Errorf("/edit diff: parse: %w", err)
	}

	var sb strings.Builder
	if plan == nil || len(plan.Blocks) == 0 {
		sb.WriteString(trc("internal_commands_edit_diff_no_blocks", nil) + "\n")
		return &CommandResult{Success: true, Output: sb.String()}, nil
	}

	sb.WriteString(trc("internal_commands_edit_diff_parsed", map[string]any{
		"Blocks": len(plan.Blocks), "Files": len(plan.PerFile),
	}) + "\n")

	// Stable per-file ordering for reproducible output.
	files := make([]string, 0, len(plan.PerFile))
	for f := range plan.PerFile {
		files = append(files, f)
	}
	sort.Strings(files)

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  PATH\tBLOCK\tLINES\n")
	for _, f := range files {
		blocks := plan.PerFile[f]
		for i, blk := range blocks {
			fmt.Fprintf(tw, "  %s\t#%d\t%d-%d\n", f, i+1, blk.LineStart, blk.LineEnd)
		}
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}, nil
}

// handleDryRun exercises the full pipeline (parse + read + apply in memory
// + diff) WITHOUT writing to disk. The output renders the unified diff that
// WOULD be written if the user ran /edit commit.
func (c *EditCommand) handleDryRun(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	if c.inspector == nil {
		return nil, fmt.Errorf("/edit dry-run: smart-edit unavailable (tool not initialised)")
	}
	prompt, err := c.extractPrompt(cc, "dry-run")
	if err != nil {
		return nil, err
	}

	res, err := c.inspector.DryRun(ctx, prompt, cc.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("/edit dry-run: %w", err)
	}
	return &CommandResult{Success: true, Output: renderResult("dry-run", res)}, nil
}

// handleCommit exercises the full pipeline INCLUDING the disk write. The
// output renders the unified diff that was actually applied to disk (the
// committer re-reads from disk for positive runtime evidence per
// CONST-035).
func (c *EditCommand) handleCommit(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	if c.inspector == nil {
		return nil, fmt.Errorf("/edit commit: smart-edit unavailable (tool not initialised)")
	}
	prompt, err := c.extractPrompt(cc, "commit")
	if err != nil {
		return nil, err
	}

	res, err := c.inspector.Commit(ctx, prompt, cc.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("/edit commit: %w", err)
	}
	return &CommandResult{Success: true, Output: renderResult("commit", res)}, nil
}

// extractPrompt resolves the SEARCH/REPLACE prompt body from the command
// context for the given subcommand. Resolution order:
//
//  1. If --from <path> flag is set, read the file at <path> verbatim.
//  2. Otherwise, take cc.RawInput (the unparsed user input) and strip the
//     leading "/edit <sub>" prefix; this preserves multi-line bodies that
//     the slash parser would otherwise flatten into a single args entry.
//  3. As a final fallback, join cc.Args[1:] with single spaces — this
//     covers tests that pass Args directly without RawInput.
//
// An empty prompt is rejected with a helpful error mentioning the
// subcommand.
func (c *EditCommand) extractPrompt(cc *CommandContext, sub string) (string, error) {
	if cc.Flags != nil {
		if path, ok := cc.Flags["from"]; ok && path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("/edit %s: --from %s: %w", sub, path, err)
			}
			body := string(data)
			if strings.TrimSpace(body) == "" {
				return "", fmt.Errorf("/edit %s: --from %s: file is empty", sub, path)
			}
			return body, nil
		}
	}

	// Inline form. Prefer RawInput so multi-line prompts survive.
	if raw := cc.RawInput; raw != "" {
		// Strip optional leading slash + "edit" + subcommand.
		// We only strip "/edit <sub>" or "edit <sub>" prefixes to keep this
		// surgical; anything else falls through to the Args fallback.
		trimmed := strings.TrimLeft(raw, " \t")
		prefixes := []string{
			"/edit " + sub,
			"/edit\t" + sub,
			"edit " + sub,
		}
		for _, p := range prefixes {
			if strings.HasPrefix(trimmed, p) {
				body := trimmed[len(p):]
				body = strings.TrimLeft(body, " \t")
				// Strip a single leading newline so a user that starts the
				// prompt on the next line gets a clean body.
				body = strings.TrimPrefix(body, "\n")
				if body != "" && !strings.HasPrefix(strings.TrimSpace(body), "--from") {
					return body, nil
				}
			}
		}
	}

	// Args fallback (tests that don't set RawInput).
	if len(cc.Args) > 1 {
		body := strings.Join(cc.Args[1:], " ")
		if strings.TrimSpace(body) != "" {
			return body, nil
		}
	}

	return "", fmt.Errorf("/edit %s: prompt is required (inline body or --from <path>)", sub)
}

// renderResult formats a *smartedit.SmartEditResult for display. Used by
// dry-run and commit handlers so their output stays consistent.
//
// The output structure is:
//
//	<sub> result: applied=<n> failed=<n> atomic=<bool>
//	  per-block lines (path + outcome + optional error)
//	  diff (if any)
//
// When AtomicError is set the message is rendered prominently so the user
// can grep for it.
func renderResult(sub string, res *smartedit.SmartEditResult) string {
	var sb strings.Builder
	if res == nil {
		sb.WriteString(trc("internal_commands_edit_no_result", map[string]any{"Sub": sub}) + "\n")
		return sb.String()
	}
	sb.WriteString(trc("internal_commands_edit_result_heading", map[string]any{"Sub": sub}) + "\n")
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  applied:\t%d\n", res.AppliedCount)
	fmt.Fprintf(tw, "  failed:\t%d\n", res.FailedCount)
	fmt.Fprintf(tw, "  atomic:\t%t\n", res.Atomic)
	if res.AtomicError != "" {
		fmt.Fprintf(tw, "  atomic_error:\t%s\n", res.AtomicError)
	}
	tw.Flush()

	if len(res.Results) > 0 {
		sb.WriteString("\n" + trc("internal_commands_edit_per_block_heading", nil) + "\n")
		btw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintf(btw, "  PATH\tLINES\tOUTCOME\tERROR\n")
		for _, br := range res.Results {
			errMsg := br.Error
			if errMsg == "" {
				errMsg = "-"
			}
			fmt.Fprintf(btw, "  %s\t%d-%d\t%s\t%s\n",
				br.Block.Path, br.Block.LineStart, br.Block.LineEnd, br.Outcome, errMsg)
		}
		btw.Flush()
	}

	if res.Diff != "" {
		sb.WriteString("\n" + trc("internal_commands_edit_unified_diff_heading", nil) + "\n")
		sb.WriteString(res.Diff)
		if !strings.HasSuffix(res.Diff, "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
