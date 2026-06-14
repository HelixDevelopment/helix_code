package main

import (
	"testing"

	"dev.helix.code/internal/llm"
)

// helixAgentProviderType mirrors the private providerType label the helixagent
// adapter reports ("helixagent"). Asserting against it here (rather than against
// a model name) keeps the test CONST-046-safe and pins the picker-sort contract.
const helixAgentProviderType llm.ProviderType = "helixagent"

// TestRegisterHelixAgentProvider_UnreachableRegistersNothing proves the anti-bluff
// liveness gate: when the HelixAgent server is NOT reachable, registerHelixAgentProvider
// MUST register ZERO providers and MUST NOT leave a dead "HelixAgent" entry in the
// picker (§11.4 / CONST-035 — a listed model is a claim it can be used).
//
// HELIXAGENT_BASE_URL is pinned at http://127.0.0.1:1 (a port nothing listens on),
// so the IsAvailable health probe fails fast and the function returns 0.
//
// §1.1 paired-mutation framing: the GREEN assertion is count == 0 AND zero
// helixagent models in the manager. If registerHelixAgentProvider were mutated to
// register UNCONDITIONALLY (dropping the IsAvailable gate), the count would become
// 1 and a "helixagent" provider would appear in GetAvailableModels — BOTH
// assertions below would FAIL. The test cannot pass on the dead-entry behaviour.
func TestRegisterHelixAgentProvider_UnreachableRegistersNothing(t *testing.T) {
	// Pin the agent base URL at an address nothing answers on so the health
	// probe deterministically fails regardless of the developer's environment.
	t.Setenv("HELIXAGENT_BASE_URL", "http://127.0.0.1:1")

	manager := llm.NewModelManager()
	got := registerHelixAgentProvider(manager)

	if got != 0 {
		t.Fatalf("registerHelixAgentProvider registered %d providers with HelixAgent unreachable, want 0 (no dead picker entry)", got)
	}

	for _, m := range manager.GetAvailableModels() {
		if m.Provider == helixAgentProviderType {
			t.Fatalf("GetAvailableModels() exposed a %q model (%q) while HelixAgent is unreachable; a dead provider was registered", helixAgentProviderType, m.Name)
		}
	}
}

// TestRegisterHelixAgentProvider_NilManagerSafe guards the defensive nil path.
func TestRegisterHelixAgentProvider_NilManagerSafe(t *testing.T) {
	if got := registerHelixAgentProvider(nil); got != 0 {
		t.Fatalf("registerHelixAgentProvider(nil) = %d, want 0", got)
	}
}
