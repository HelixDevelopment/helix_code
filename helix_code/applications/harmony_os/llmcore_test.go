// Package main — llmcore_test.go.
//
// Real unit tests for the Harmony OS LLM generation parity surface
// (HarmonyLLMCore.Generate in distributed.go). This file is intentionally
// NOT build-tagged (matching distributed.go) so it executes on every host
// that can build the Go toolchain — NOT only on hosts with Fyne / X11.
//
// Anti-bluff (Article XI §11.9 / BLUFF-001): these tests assert REAL
// behaviour of the canonical llm.Provider path, never a canned stub:
//   - empty/whitespace prompts MUST be rejected before any provider call;
//   - with NO provider reachable (Ollama down at localhost:11434 and no
//     cloud creds), Generate MUST return a real transport error AND an
//     empty string — never a fabricated success.
package main

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestHarmonyGenerate_EmptyPrompt asserts the empty-prompt error path is
// enforced at the Harmony surface BEFORE any provider is constructed —
// matching shared/mobile_core. A nil-provider core proves no network call
// is needed to reject empty input.
func TestHarmonyGenerate_EmptyPrompt(t *testing.T) {
	core := NewHarmonyLLMCore()

	for _, prompt := range []string{"", "   ", "\t\n  "} {
		out, err := core.Generate(prompt)
		if err == nil {
			t.Fatalf("Generate(%q): expected an error for empty/whitespace prompt, got nil (out=%q)", prompt, out)
		}
		if out != "" {
			t.Fatalf("Generate(%q): expected empty output on error, got %q", prompt, out)
		}
		if !strings.Contains(err.Error(), "prompt must not be empty") {
			t.Fatalf("Generate(%q): expected 'prompt must not be empty' error, got %v", prompt, err)
		}
	}
}

// ollamaReachable reports whether a local Ollama daemon is listening on the
// standard port. It performs a real TCP dial — no mock.
func ollamaReachable() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:11434", 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// TestHarmonyGenerate_NoProviderHonestError probes the no-provider honest
// error path against a REAL localhost:11434. To exercise the no-cloud
// fallback deterministically, HELIX_LLM_PROVIDER is cleared so llm.Select
// resolves to ErrNoProviderConfigured and construction falls back to the
// local Ollama provider.
//
//   - If Ollama IS up, the test SKIPs (SKIP-OK: requires Ollama DOWN to
//     assert the honest-error path; a live daemon would legitimately
//     succeed and that success is covered by integration tests, not this
//     unit test).
//   - If Ollama is DOWN, Generate MUST return a real, non-empty error and
//     an empty string — proving no fabricated success.
func TestHarmonyGenerate_NoProviderHonestError(t *testing.T) {
	if ollamaReachable() {
		t.Skip("SKIP-OK: local Ollama is reachable at 127.0.0.1:11434; this test asserts the Ollama-DOWN honest-error path. Live-provider success is covered by integration tests.")
	}

	// Force the no-cloud-configured path so the fallback Ollama provider is
	// selected and its (failing) transport surfaces a real error.
	prev, had := os.LookupEnv("HELIX_LLM_PROVIDER")
	if err := os.Unsetenv("HELIX_LLM_PROVIDER"); err != nil {
		t.Fatalf("failed to unset HELIX_LLM_PROVIDER: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("HELIX_LLM_PROVIDER", prev)
		}
	})

	core := NewHarmonyLLMCore()
	out, err := core.Generate("What is 2+2?")
	if err == nil {
		t.Fatalf("Generate: expected a real error when no LLM provider is reachable, got nil (out=%q)", out)
	}
	if out != "" {
		t.Fatalf("Generate: expected empty output on provider error, got %q", out)
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("Generate: error message must not be blank (a blank error is itself a bluff)")
	}
	t.Logf("honest no-provider error surfaced (Ollama down): %v", err)
}
