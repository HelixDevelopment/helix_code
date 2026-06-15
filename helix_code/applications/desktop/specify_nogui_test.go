//go:build nogui

package main

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIApp_Specify_EmptyRequest exercises the real Specify method's
// input-validation error path. This is hermetic (no network): an empty/
// whitespace request MUST return a non-nil error and an empty string BEFORE any
// provider is contacted. It also proves the method is real (not a stub
// returning a fixed success string regardless of input), mirroring
// generate_nogui_test.go's TestCLIApp_Generate_EmptyPrompt.
func TestCLIApp_Specify_EmptyRequest(t *testing.T) {
	app := NewCLIApp()

	out, err := app.Specify("")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "request must not be empty")

	out, err = app.Specify("   \t\n  ")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "request must not be empty")
}

// TestCLIApp_Specify_NoProviderReachable proves the no-provider / phase error
// path is honest (anti-bluff): with a deliberately empty HELIX_LLM_PROVIDER (so
// the resolver falls through to the local Ollama default) and no reachable
// backend, a real request MUST NOT yield a fabricated specify-phase output. The
// real provider.Generate call (driven by the speckit debate) fails against the
// unreachable local Ollama default, so the result is an error + empty output —
// never a fake response.
//
// Mirrors generate_nogui_test.go's TestCLIApp_Generate_NoProviderReachable.
func TestCLIApp_Specify_NoProviderReachable(t *testing.T) {
	// Point the resolver at a non-cloud value so it falls through to the local
	// Ollama default.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	// Determinism guard: if a real Ollama is listening locally it would
	// legitimately drive the debate, so this negative-path assertion does not
	// apply. Skip honestly in that case rather than assert a false expectation.
	if conn, derr := net.DialTimeout("tcp", "localhost:11434", 200*time.Millisecond); derr == nil {
		_ = conn.Close()
		t.Skip("SKIP-OK: local Ollama is reachable on :11434; negative no-provider path not applicable")
	}

	app := NewCLIApp()
	out, err := app.Specify("Build a TODO CLI app with add/list/done commands.")

	// Anti-bluff: a stub would return a canned non-empty phase output with nil
	// error. The real implementation, with no reachable backend, returns an
	// error and empty content. (The Ollama provider may advertise a default
	// model even when unreachable, so the failure surfaces from the real
	// debate/provider call, prefixed "specify:".)
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "specify:")
}
