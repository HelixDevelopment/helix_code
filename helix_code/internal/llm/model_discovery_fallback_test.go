package llm

import (
	"context"
	"testing"

	"dev.helix.code/internal/verifier"
)

// F6 / D-5 (CONST-036) — fetchExternalModels MUST NOT present the hardcoded
// verifier.FallbackModels list as available "external" models when the verifier
// source is unavailable. A fabricated list shown as working is a §11.4 /
// CONST-035 PASS-bluff.
//
// RED_MODE polarity switch (§11.4.115):
//   RED_MODE=1 → reproduce the bug on the pre-fix tree (assert the hardcoded
//                fallback list LEAKS — this FAILs once the fix is in place,
//                proving the RED genuinely reproduced the defect on broken code).
//   RED_MODE=0 → standing GREEN regression guard (assert honest-empty when no
//                verifier source).
//
// In-package unit test (no mocks needed — exercises the real engine with a nil
// verifier source, the exact "verifier unavailable / cold" path).

func TestFetchExternalModels_NoVerifier_HonestEmpty_D5(t *testing.T) {
	// Engine with NO verifier source (verifierSource == nil) — the
	// verifier-unavailable / cold-start path.
	e := &ModelDiscoveryEngine{}
	got := e.fetchExternalModels(context.Background(), &RecommendationRequest{})

	// The historical defect returned exactly the hardcoded fallback list.
	hardcoded := ConvertVerifiedToModelInfo(verifier.FallbackModels)

	if redMode() {
		// RED: on the broken tree this returns the hardcoded list → len matches.
		if len(got) != len(hardcoded) || len(got) == 0 {
			t.Fatalf("RED_MODE=1 expected the hardcoded fallback list to leak (len=%d) — "+
				"defect not reproduced; got len=%d", len(hardcoded), len(got))
		}
		t.Logf("RED reproduced: fetchExternalModels leaked %d hardcoded fallback models", len(got))
		return
	}

	// GREEN: honest-empty — no fabricated list presented as available.
	if len(got) != 0 {
		t.Fatalf("verifier unavailable: fetchExternalModels MUST return honest-empty, "+
			"got %d models (hardcoded fallback list leaked — CONST-036 / §11.4 bluff)", len(got))
	}
}
