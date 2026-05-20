package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/smartedit"
)

// fakeSmartEditInspector is a hexagonal-seam impl of the
// commands.SmartEditInspector interface used by /edit tests. It records
// ParsePrompt / DryRun / Commit calls so the tests can assert that the
// /edit subcommands actually delegate to the inspector (no
// fmt.Printf + sleep simulation), and lets us drive parser/applier
// outcomes deterministically without disk I/O.
type fakeSmartEditInspector struct {
	parseResult *smartedit.EditPlan
	parseErr    error

	dryRunResult *smartedit.SmartEditResult
	dryRunErr    error

	commitResult *smartedit.SmartEditResult
	commitErr    error

	parseCalls  int
	dryRunCalls int
	commitCalls int

	lastParsePrompt   string
	lastDryRunPrompt  string
	lastDryRunWorkdir string
	lastCommitPrompt  string
	lastCommitWorkdir string
}

func (f *fakeSmartEditInspector) ParsePrompt(prompt string) (*smartedit.EditPlan, error) {
	f.parseCalls++
	f.lastParsePrompt = prompt
	return f.parseResult, f.parseErr
}

func (f *fakeSmartEditInspector) DryRun(ctx context.Context, prompt, workdir string) (*smartedit.SmartEditResult, error) {
	f.dryRunCalls++
	f.lastDryRunPrompt = prompt
	f.lastDryRunWorkdir = workdir
	return f.dryRunResult, f.dryRunErr
}

func (f *fakeSmartEditInspector) Commit(ctx context.Context, prompt, workdir string) (*smartedit.SmartEditResult, error) {
	f.commitCalls++
	f.lastCommitPrompt = prompt
	f.lastCommitWorkdir = workdir
	return f.commitResult, f.commitErr
}

func newEditCommand(t *testing.T) (*EditCommand, *fakeSmartEditInspector) {
	t.Helper()
	insp := &fakeSmartEditInspector{}
	return NewEditCommand(insp), insp
}

// --- Tool surface ---

func TestEditCommand_NameDescription(t *testing.T) {
	c, _ := newEditCommand(t)
	assert.Equal(t, "edit", c.Name())
	// Description/Usage route through the CONST-046 tr() seam; under
	// the default NoopTranslator they echo the message ID (round-399).
	assert.NotEmpty(t, c.Description())
	assert.Contains(t, c.Usage(), "internal_commands_edit_usage")
	assert.Nil(t, c.Aliases())
}

// --- /edit (default → status) ---

func TestEditCommand_DefaultIsStatus(t *testing.T) {
	c, _ := newEditCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil, RawInput: "/edit"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "smart-edit")
}

func TestEditCommand_StatusShowsToolWired(t *testing.T) {
	c, _ := newEditCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}, RawInput: "/edit status"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "available")
}

func TestEditCommand_StatusShowsToolNotWired(t *testing.T) {
	c := NewEditCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}, RawInput: "/edit status"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, strings.ToLower(res.Output), "unavailable")
}

// --- /edit diff <prompt> ---

func TestEditCommand_DiffShowsPlanSummary(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.parseResult = &smartedit.EditPlan{
		Blocks: []smartedit.EditBlock{
			{Path: "a.go", Search: "x", Replace: "y", LineStart: 2, LineEnd: 6},
			{Path: "b.go", Search: "p", Replace: "q", LineStart: 8, LineEnd: 12},
		},
		PerFile: map[string][]smartedit.EditBlock{
			"a.go": {{Path: "a.go", Search: "x", Replace: "y", LineStart: 2, LineEnd: 6}},
			"b.go": {{Path: "b.go", Search: "p", Replace: "q", LineStart: 8, LineEnd: 12}},
		},
		SourceBytes: 200,
	}

	prompt := "a.go\n<<<<<<< SEARCH\nx\n=======\ny\n>>>>>>> REPLACE\n"
	res, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"diff", prompt},
		RawInput: "/edit diff " + prompt,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, insp.parseCalls)
	assert.Equal(t, 0, insp.dryRunCalls, "diff must NOT exercise the apply pipeline")
	assert.Equal(t, 0, insp.commitCalls)
	assert.Contains(t, res.Output, "2 blocks")
	assert.Contains(t, res.Output, "a.go")
	assert.Contains(t, res.Output, "b.go")
}

func TestEditCommand_DiffParseErrorReturned(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.parseErr = errors.New("invalid SEARCH/REPLACE block structure")
	_, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"diff", "garbage"},
		RawInput: "/edit diff garbage",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SEARCH/REPLACE")
}

func TestEditCommand_DiffMissingPromptErrors(t *testing.T) {
	c, _ := newEditCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"diff"},
		RawInput: "/edit diff",
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "prompt")
}

// --- /edit dry-run <prompt> ---

func TestEditCommand_DryRunShowsDiffOutput(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.dryRunResult = &smartedit.SmartEditResult{
		Atomic:       true,
		AppliedCount: 1,
		FailedCount:  0,
		Diff:         "--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
		Results: []smartedit.EditResult{
			{
				Block:   smartedit.EditBlock{Path: "foo.go", LineStart: 1, LineEnd: 5},
				Outcome: smartedit.OutcomeApplied,
			},
		},
	}

	prompt := "foo.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n"
	res, err := c.Execute(context.Background(), &CommandContext{
		Args:       []string{"dry-run", prompt},
		RawInput:   "/edit dry-run " + prompt,
		WorkingDir: "/tmp/work",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, insp.dryRunCalls)
	assert.Equal(t, 0, insp.commitCalls, "dry-run must NOT call Commit")
	assert.Equal(t, "/tmp/work", insp.lastDryRunWorkdir)
	assert.Contains(t, res.Output, "--- a/foo.go")
	assert.Contains(t, res.Output, "+++ b/foo.go")
	assert.Contains(t, res.Output, "+new")
}

func TestEditCommand_DryRunReportsAtomicError(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.dryRunResult = &smartedit.SmartEditResult{
		Atomic:      false,
		FailedCount: 1,
		AtomicError: "SEARCH text not found in target file",
		Results: []smartedit.EditResult{
			{
				Block:   smartedit.EditBlock{Path: "foo.go"},
				Outcome: smartedit.OutcomeNotFound,
				Error:   "SEARCH text not found in target file",
			},
		},
	}
	prompt := "foo.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n"
	res, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"dry-run", prompt},
		RawInput: "/edit dry-run " + prompt,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "SEARCH text not found")
	assert.Contains(t, res.Output, "not-found")
}

// --- /edit commit <prompt> ---

func TestEditCommand_CommitInvokesInspector(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.commitResult = &smartedit.SmartEditResult{
		Atomic:       true,
		AppliedCount: 1,
		Diff:         "--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n",
		Results: []smartedit.EditResult{
			{
				Block:   smartedit.EditBlock{Path: "x.go"},
				Outcome: smartedit.OutcomeApplied,
			},
		},
	}

	prompt := "x.go\n<<<<<<< SEARCH\na\n=======\nb\n>>>>>>> REPLACE\n"
	res, err := c.Execute(context.Background(), &CommandContext{
		Args:       []string{"commit", prompt},
		RawInput:   "/edit commit " + prompt,
		WorkingDir: "/tmp/wd",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, insp.commitCalls, "commit subcommand MUST call Commit")
	assert.Equal(t, 0, insp.dryRunCalls, "commit must NOT call DryRun")
	assert.Equal(t, "/tmp/wd", insp.lastCommitWorkdir)
	assert.Contains(t, res.Output, "--- a/x.go")
	assert.Contains(t, res.Output, "applied")
}

func TestEditCommand_CommitFailureReportsAtomicError(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.commitResult = &smartedit.SmartEditResult{
		Atomic:      false,
		FailedCount: 1,
		AtomicError: "boom",
	}
	prompt := "x.go\n<<<<<<< SEARCH\na\n=======\nb\n>>>>>>> REPLACE\n"
	res, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"commit", prompt},
		RawInput: "/edit commit " + prompt,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, insp.commitCalls)
	assert.Contains(t, res.Output, "boom")
}

func TestEditCommand_CommitErrorPropagated(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.commitErr = errors.New("multiedit: commit: rename failed")
	prompt := "x.go\n<<<<<<< SEARCH\na\n=======\nb\n>>>>>>> REPLACE\n"
	_, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"commit", prompt},
		RawInput: "/edit commit " + prompt,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename failed")
}

// --- --from <path> form ---

func TestEditCommand_DryRunFromFile(t *testing.T) {
	c, insp := newEditCommand(t)
	insp.dryRunResult = &smartedit.SmartEditResult{
		Atomic:       true,
		AppliedCount: 1,
		Diff:         "--- a/y.go\n+++ b/y.go\n@@ -1 +1 @@\n-a\n+b\n",
		Results: []smartedit.EditResult{{
			Block:   smartedit.EditBlock{Path: "y.go"},
			Outcome: smartedit.OutcomeApplied,
		}},
	}

	dir := t.TempDir()
	promptPath := filepath.Join(dir, "edit.txt")
	prompt := "y.go\n<<<<<<< SEARCH\na\n=======\nb\n>>>>>>> REPLACE\n"
	require.NoError(t, os.WriteFile(promptPath, []byte(prompt), 0o644))

	res, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"dry-run"},
		Flags:    map[string]string{"from": promptPath},
		RawInput: "/edit dry-run --from " + promptPath,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, insp.dryRunCalls)
	assert.Equal(t, prompt, insp.lastDryRunPrompt, "the prompt body must be read from --from path verbatim")
}

// --- error paths ---

func TestEditCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newEditCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"bogus"},
		RawInput: "/edit bogus",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

func TestEditCommand_DryRunInspectorNilErrors(t *testing.T) {
	c := NewEditCommand(nil)
	prompt := "foo.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n"
	_, err := c.Execute(context.Background(), &CommandContext{
		Args:     []string{"dry-run", prompt},
		RawInput: "/edit dry-run " + prompt,
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "unavailable")
}
