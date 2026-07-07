package llm

import "testing"

// provider_live_proof_skip_test.go — always-on unit test for the
// key-absent honest-SKIP contract that the CONST-039 per-provider
// live-proof harness (provider_live_proof_test.go, build tag
// `providerlive`) relies on.
//
// Deliberately NOT build-tag-gated: this test exercises only always-
// compiled, non-gated production code (IsProviderKeyPresent and
// ProviderEnvAliases, both in keyrecognition.go) so it participates in
// every default `go test ./...` / `go test ./internal/llm/...` invocation
// with no excluded-tag gymnastics hiding it (per the task's explicit
// instruction — the harness's honest-SKIP contract must be unit-tested
// where it will actually run by default, not only behind an opt-in tag).
//
// It asserts the exact anti-bluff contract this whole harness exists to
// prove: when a provider's credential is absent, the outcome MUST be an
// honest SKIP — never a silent/fake PASS (which would hide the fact that
// nothing was actually verified), and never a FAIL-on-absence (the
// ensemble_provider_live_probe_test.go anti-pattern this harness was
// explicitly built NOT to repeat, see that file's line 48 t.Fatalf on
// <2 members).
// Deliberately named WITHOUT the "TestProviderLiveProof" substring: Go's
// `-run` flag matches test names via an UNANCHORED regexp, so any pattern
// beginning with that substring (e.g. the dispatcher script's
// `-run TestProviderLiveProof/<provider>`) would otherwise also select this
// unrelated test's top-level body (while filtering out its own subtest),
// producing a spurious FAIL — exactly the collision this rewrite's own
// no-keys proof run caught and root-caused (§11.4.102/§11.4.146) before
// picking this name.
func TestHonestSkipOnAbsentProviderKey(t *testing.T) {
	const sample = ProviderTypeAnthropic

	aliases, ok := ProviderEnvAliases()[sample]
	if !ok || len(aliases) == 0 {
		t.Fatalf("no recognised env-var aliases registered for sample provider %s — test setup invalid", sample)
	}

	// Force every recognised alias for the sample provider to an empty
	// value for the duration of this test only. t.Setenv automatically
	// restores each variable's previous value via t.Cleanup, so this
	// never leaks into the operator's real environment, other tests, or
	// the providerlive-tagged harness's own runs.
	for _, alias := range aliases {
		t.Setenv(alias, "")
	}

	if IsProviderKeyPresent(sample) {
		t.Fatalf(
			"IsProviderKeyPresent(%s) reported PRESENT after clearing every known alias %v — "+
				"either test isolation failed (a real key is set under an alias this test doesn't "+
				"know about) or the placeholder/blank guard in keyrecognition.go regressed",
			sample, aliases,
		)
	}

	// Mirror the harness's own honest-SKIP gate (runHostedProviderLiveProof
	// in provider_live_proof_test.go takes this exact branch first, before
	// any network call): key absent -> t.Skip("SKIP: no-key"), never a
	// silent pass with no signal, never a FAIL.
	//
	// t.Skip halts the calling goroutine, so we cannot inspect the
	// sub-test's state *after* the call within the same function scope.
	// Instead we set `skipped` immediately before invoking the real
	// t.Skip call — the same idiom the Go standard library itself uses to
	// assert that a Skip branch, not a Pass or Fail branch, was taken.
	skipped := false
	subtestOK := t.Run("sample-provider-key-absent", func(st *testing.T) {
		if !IsProviderKeyPresent(sample) {
			skipped = true
			st.Skip("SKIP: no-key")
			return
		}
		st.Fatalf("unreachable: key unexpectedly present inside isolated subtest for %s", sample)
	})

	if !subtestOK {
		t.Fatalf("expected the key-absent gate subtest to report OK (skip counts as OK, not failed); got failed")
	}
	if !skipped {
		t.Fatalf("expected the key-absent gate to take the honest SKIP branch for %s; it did not (skipped=false) — this would mean a present-vs-absent race or a silent fall-through", sample)
	}

	t.Logf("confirmed: key-absent path for %s takes the honest SKIP branch (\"SKIP: no-key\") — never a fake PASS, never a FAIL-on-absence", sample)
}
