//go:build nogui

package main

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIApp_Generate_EmptyPrompt exercises the real Generate method's
// input-validation error path. This is hermetic (no network): an empty/
// whitespace prompt MUST return a non-nil error and an empty string BEFORE any
// provider is contacted. It also proves the method is real (not a stub
// returning a fixed success string regardless of input), mirroring the
// mobile gomobile core's TestMobileCore_Generate_EmptyPrompt and
// aurora_os's TestCLIApp_Generate_EmptyPrompt.
func TestCLIApp_Generate_EmptyPrompt(t *testing.T) {
	app := NewCLIApp()

	out, err := app.Generate("")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "prompt must not be empty")

	out, err = app.Generate("   \t\n  ")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "prompt must not be empty")
}

// TestCLIApp_Generate_NoProviderReachable proves the no-provider error path is
// honest (anti-bluff): with a deliberately empty HELIX_LLM_PROVIDER (so the
// resolver falls through to the local Ollama default) and no reachable
// backend, a real prompt MUST NOT yield a fabricated success. The real
// provider.Generate call fails against the unreachable local Ollama default,
// so the result is an error + empty output — never a fake response.
//
// Mirrors mobile_core's TestMobileCore_Generate_NoProviderReachable and
// aurora_os's TestCLIApp_Generate_NoProviderReachable.
func TestCLIApp_Generate_NoProviderReachable(t *testing.T) {
	// Point the resolver at a non-cloud value so it falls through to the
	// local Ollama default.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	// Determinism guard: if a real Ollama is listening locally it would
	// legitimately answer, so this negative-path assertion does not apply.
	// Skip honestly in that case rather than assert a false expectation.
	if conn, derr := net.DialTimeout("tcp", "localhost:11434", 200*time.Millisecond); derr == nil {
		_ = conn.Close()
		t.Skip("SKIP-OK: local Ollama is reachable on :11434; negative no-provider path not applicable")
	}

	app := NewCLIApp()
	out, err := app.Generate("Say hello.")

	// Anti-bluff: a stub would return a canned non-empty string with nil
	// error. The real implementation, with no reachable backend, returns an
	// error and empty content.
	require.Error(t, err)
	assert.Empty(t, out)
	// The error must originate from the real generation path, not a stub.
	assert.Contains(t, err.Error(), "generate:")
}
