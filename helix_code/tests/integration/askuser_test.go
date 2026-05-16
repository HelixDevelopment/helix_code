//go:build integration

// askuser_test.go (P1-F19-T05): integration tests covering the F19 ask_user
// tool end-to-end through the askuser package + the wired registry. These
// tests are always-run (no docker-compose / external infra needed) because the
// tool's I/O seam (Reader/Writer/IsTTY) is exercised through bytes.Buffer +
// closures — every assertion runs against a real askuser.AskUserTool with a
// real *stdinPrompter, just with deterministic I/O instead of os.Stdin/os.Stdout.
//
// Anti-bluff anchor (CONST-035 §11.9): each test asserts on positive runtime
// evidence — actual Result map fields, actual writer bytes, actual error
// sentinel matches via errors.Is — never on metadata-only "no error" PASS.

package integration

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/askuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTool wires a fresh AskUserTool around a stdinPrompter built from the
// supplied I/O — the helper centralises the option boilerplate so each test
// stays focused on its own dispatch.
func newTool(t *testing.T, reader *bytes.Buffer, writer *bytes.Buffer, isTTY bool) *askuser.AskUserTool {
	t.Helper()
	prompter, err := askuser.NewStdinPrompter(askuser.StdinPrompterOptions{
		Reader:     reader,
		Writer:     writer,
		IsTTY:      func() bool { return isTTY },
		MaxRetries: 3,
		Timeout:    2 * time.Second,
	})
	require.NoError(t, err, "stdinPrompter construction must succeed for integration test")
	return askuser.NewAskUserTool(prompter)
}

// sampleParams returns a canonical 3-choice question whose values are stable
// across tests. Tests that need a Default override the "default" entry.
func sampleParams() map[string]any {
	return map[string]any{
		"question": "Pick a fruit:",
		"choices": []any{
			map[string]any{"label": "Apple", "value": "apple"},
			map[string]any{"label": "Banana", "value": "banana"},
			map[string]any{"label": "Cherry", "value": "cherry"},
		},
	}
}

// TestAskUser_TTYWithInput_ReturnsChoice — TTY path, user picks "2", expect
// the second choice's value/index back. Validates the happy path: user input
// flows from the buffer into the prompter and back out as a structured Result.
func TestAskUser_TTYWithInput_ReturnsChoice(t *testing.T) {
	reader := bytes.NewBufferString("2\n")
	writer := &bytes.Buffer{}
	tool := newTool(t, reader, writer, true)

	res, err := tool.Execute(context.Background(), sampleParams())
	require.NoError(t, err)

	m, ok := res.(map[string]any)
	require.True(t, ok, "Execute must return map[string]any, got %T", res)
	assert.Equal(t, "banana", m["value"])
	assert.Equal(t, 1, m["index"])
	assert.Equal(t, false, m["used_default"])
}

// TestAskUser_NonTTYWithDefault_ReturnsDefault — non-TTY path with a Default,
// expect the default to be returned with UsedDefault=true and the writer to
// stay empty (the prompter must NOT render in non-TTY mode).
func TestAskUser_NonTTYWithDefault_ReturnsDefault(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	tool := newTool(t, reader, writer, false)

	params := sampleParams()
	params["default"] = "cherry"

	res, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)

	m, ok := res.(map[string]any)
	require.True(t, ok, "Execute must return map[string]any, got %T", res)
	assert.Equal(t, "cherry", m["value"])
	assert.Equal(t, 2, m["index"])
	assert.Equal(t, true, m["used_default"])
	assert.Empty(t, writer.Bytes(), "non-TTY path must not render to writer")
}

// TestAskUser_NonTTYNoDefault_Errors — non-TTY path WITHOUT a Default must
// surface ErrInteractiveTerminalRequired so callers know the question can't
// be answered. errors.Is must reach the sentinel through the %w chain.
func TestAskUser_NonTTYNoDefault_Errors(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	tool := newTool(t, reader, writer, false)

	res, err := tool.Execute(context.Background(), sampleParams())
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, askuser.ErrInteractiveTerminalRequired),
		"err must wrap ErrInteractiveTerminalRequired, got %v", err)
}

// TestAskUser_PreviewVisibleInOutput — TTY path with a Choice carrying a
// Preview string. The rendered output captured by the writer must contain the
// preview text verbatim — proves the F18 render path actually wrote bytes.
func TestAskUser_PreviewVisibleInOutput(t *testing.T) {
	reader := bytes.NewBufferString("1\n")
	writer := &bytes.Buffer{}
	tool := newTool(t, reader, writer, true)

	params := map[string]any{
		"question": "Pick a colour:",
		"choices": []any{
			map[string]any{
				"label":   "Red",
				"value":   "red",
				"preview": "RGB(255,0,0) — bright crimson",
			},
			map[string]any{"label": "Blue", "value": "blue"},
		},
	}

	_, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)

	out := writer.String()
	assert.Contains(t, out, "RGB(255,0,0) — bright crimson",
		"writer output must contain the preview text — got %q", out)
	assert.Contains(t, out, "Pick a colour:", "writer output must contain the question text")
	assert.Contains(t, out, "1.", "writer output must contain numbered choices")
}

// TestAskUser_InvalidInputThenValid — first input "9" is out of range, second
// input "2" is valid. The prompter must retry, redraw with a hint, and finally
// return choice index 1. The hint message about valid range must appear in
// the writer at least once.
func TestAskUser_InvalidInputThenValid(t *testing.T) {
	reader := bytes.NewBufferString("9\n2\n")
	writer := &bytes.Buffer{}
	tool := newTool(t, reader, writer, true)

	res, err := tool.Execute(context.Background(), sampleParams())
	require.NoError(t, err)

	m, ok := res.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "banana", m["value"])
	assert.Equal(t, 1, m["index"])
	assert.Equal(t, false, m["used_default"])

	out := writer.String()
	// The retry hint mentions the valid range "1-3" (3 choices).
	assert.Contains(t, out, "1-3",
		"writer must contain retry hint mentioning valid range — got %q", out)
}

// TestAskUser_RegisteredInRegistry_ReturnsRealTool — wire a registry the same
// way main.go does (NewToolRegistry + Register the real askuser.AskUserTool)
// and verify Get("ask_user") returns a *askuser.AskUserTool, NOT the deleted
// in-tree bluff stub. This test fails loudly if a future refactor accidentally
// resurrects the bluff stub or registers a different type under "ask_user".
func TestAskUser_RegisteredInRegistry_ReturnsRealTool(t *testing.T) {
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)

	// Mirror the main.go wiring: construct a real prompter (default options
	// — Reader=os.Stdin, Writer=os.Stdout) and register the real tool.
	prompter, err := askuser.NewStdinPrompter(askuser.StdinPrompterOptions{
		// Provide an empty buffer so the prompter doesn't try to probe os.Stdin
		// during construction; we never call Prompt in this test.
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
		IsTTY:  func() bool { return false },
	})
	require.NoError(t, err)
	reg.Register(askuser.NewAskUserTool(prompter))

	got, err := reg.Get("ask_user")
	require.NoError(t, err)

	// The crucial assertion: the registered tool MUST be the real
	// *askuser.AskUserTool, not the deleted bluff stub *tools.AskUserTool
	// (which no longer exists at all — its symbol was removed in T05).
	_, isReal := got.(*askuser.AskUserTool)
	require.True(t, isReal,
		"registry returned %T for \"ask_user\" — expected *askuser.AskUserTool", got)

	// Sanity: Category must match the F19 category constant, not the legacy
	// CategoryInteractive that the bluff stub used.
	assert.Equal(t, tools.CategoryAskUser, got.Category(),
		"real ask_user must use CategoryAskUser, not the legacy CategoryInteractive")

	// Description must mention "block until they respond" — proof we got the
	// description from askuser.AskUserTool, not the terse stub description.
	assert.True(t, strings.Contains(got.Description(), "block"),
		"real ask_user description must mention blocking — got %q", got.Description())
}
