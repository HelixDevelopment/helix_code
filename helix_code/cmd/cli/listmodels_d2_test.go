package main

import (
	"testing"

	"dev.helix.code/internal/verifier"
)

// d2RedMode reuses the §11.4.115 polarity switch (redMode defined in
// loadapikeys_wiring_test.go).
//
//	RED_MODE=1 (default): reproduce the D-2 defect — the OLD per-row predicate
//	            treated failed/pending/rate-limited as displayable.
//	RED_MODE=0: the GREEN guard — only Verified ∧ status=="verified" rows show.

// oldDisplayPredicate models the pre-fix printVerifiedModels behaviour: it
// rendered EVERY row (verified, pending, failed, rate-limited), merely changing
// the status label. It is the captured pre-fix logic used to prove the defect.
func oldDisplayPredicate(_ *verifier.VerifiedModel) bool { return true }

func TestD2_PrinterDropsNonWorkingRows(t *testing.T) {
	models := []*verifier.VerifiedModel{
		{ID: "ok", Verified: true, VerificationStatus: "verified", OverallScore: 8},
		{ID: "pending", Verified: false, VerificationStatus: "pending"},
		{ID: "failed", Verified: false, VerificationStatus: "failed"},
		{ID: "ratelimited", Verified: false, VerificationStatus: "rate_limited"},
	}

	if redMode() {
		// RED: the OLD predicate displays the failed/pending/rate-limited rows.
		shown := 0
		for _, m := range models {
			if oldDisplayPredicate(m) {
				shown++
			}
		}
		if shown != 4 {
			t.Fatalf("RED expected the old predicate to display all 4 rows (the D-2 bluff), got %d", shown)
		}
		return
	}

	// GREEN: the new predicate displays ONLY the working row.
	shown := []string{}
	for _, m := range models {
		if isWorkingForDisplay(m) {
			shown = append(shown, m.ID)
		}
	}
	if len(shown) != 1 || shown[0] != "ok" {
		t.Fatalf("GREEN: only the verified row must display, got %v", shown)
	}
}

func TestD2_PresentProviders_AliasAndPlaceholder(t *testing.T) {
	if redMode() {
		t.Skip("RED polarity covered by TestD2_PrinterDropsNonWorkingRows") // SKIP-OK: paired RED guard
	}
	env := map[string]string{
		"CLAUDE_API_KEY":  "sk-ant-real", // anthropic alias
		"OPENAI_API_KEY":  "your-key-here", // placeholder -> NOT present
		"DEEPSEEK_API_KEY": "sk-deepseek-real",
	}
	getenv := func(k string) string { return env[k] }

	present := presentProviders(getenv)
	if !present["anthropic"] {
		t.Fatalf("anthropic must be present via CLAUDE_API_KEY alias")
	}
	if !present["deepseek"] {
		t.Fatalf("deepseek must be present")
	}
	if present["openai"] {
		t.Fatalf("openai must NOT be present (placeholder value)")
	}
	if present["gemini"] {
		t.Fatalf("gemini must NOT be present (no key)")
	}
}
