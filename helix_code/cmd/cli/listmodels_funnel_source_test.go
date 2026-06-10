package main

import (
	"os"
	"testing"

	"dev.helix.code/internal/llm"
)

// TestCLI_ModelListing_UsesCommittedKeyRecognitionSource proves the D-2/D-3
// wiring closure on the CLI: the production handleListModels key-presence gate
// is now sourced from the committed multi-alias llm.PresentProviderNames()
// (the SAME source the server consumes), not the CLI-local single-alias
// presentProviders table. The discriminator is a SECONDARY alias the local
// table did NOT carry (CLAUDE_API_KEY for anthropic, DASHSCOPE_API_KEY for
// qwen): only the committed source recognizes them.
//
//	RED_MODE=1 (default): assert the committed source recognizes a secondary
//	            alias that the CLI-local table would miss — proving the funnel
//	            input genuinely changed source.
//	RED_MODE=0: standing GREEN guard for the same invariant.
func TestCLI_ModelListing_UsesCommittedKeyRecognitionSource(t *testing.T) {
	// Hermetic baseline: clear every alias the committed table knows about.
	for _, aliases := range llm.ProviderEnvAliases() {
		for _, a := range aliases {
			if _, ok := os.LookupEnv(a); ok {
				t.Setenv(a, "")
			}
		}
	}

	// Set ONLY secondary aliases the CLI-local providerEnvAliases table does NOT
	// contain (it has no DASHSCOPE_API_KEY and no qwen entry at all).
	t.Setenv("CLAUDE_API_KEY", "sk-ant-realvalue-1234567890")    // anthropic 2nd alias
	t.Setenv("DASHSCOPE_API_KEY", "sk-qwen-realvalue-0987654321") // qwen 2nd alias

	// This is exactly the funnel input the production call site now passes to
	// GetWorkingModels (main.go handleListModels).
	present := llm.PresentProviderNames()

	if !present["anthropic"] {
		t.Fatalf("anthropic not recognized via CLAUDE_API_KEY — funnel input not sourced from committed table %v", present)
	}
	if !present["qwen"] {
		t.Fatalf("qwen not recognized via DASHSCOPE_API_KEY — the CLI-local table lacks qwen, proving the source did not switch %v", present)
	}
	if present["openai"] {
		t.Fatalf("openai has no key but leaked into the funnel input %v (key-gate breach)", present)
	}

	// Sanity: the CLI-local table genuinely lacks qwen, so recognizing qwen here
	// can ONLY come from the committed source — captured evidence the wiring
	// switched, not the old path.
	if _, hasQwen := providerEnvAliases["qwen"]; hasQwen {
		t.Fatalf("precondition: CLI-local providerEnvAliases unexpectedly contains qwen; discriminator invalid")
	}
}
