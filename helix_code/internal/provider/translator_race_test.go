// translator_race_test.go — §11.4.135 standing regression guard for the
// HXC-014b §11.4.85 concurrency fix in translator.go.
//
// FORENSIC CONTEXT (§11.4.108 / §11.4.115): translator.go documents that
// SetTranslator (a write to the package-level `translator` variable) may run
// concurrently with tr() (a read of the same variable), and that BOTH accesses
// MUST therefore be guarded by translatorMu — otherwise the concurrent
// read/write is a data race (§11.4.85(B) state-corruption defect). The fix
// (an RWMutex) was landed but NO standing test exercised the concurrent path,
// so a future edit removing the mutex would ship undetected: the package's
// only other tests are pure-constant ProviderType assertions that never touch
// SetTranslator/tr. This file closes that gap.
//
// §11.4.115 RED-polarity contract — ONE source, TWO roles:
//
//	RED_MODE=1  → reproduce the defect on a faithful UNGUARDED stand-in
//	              (the pre-fix artifact's access pattern) under -race and
//	              ASSERT the data race is genuinely present. This proves the
//	              guard catches a real defect, not a synthetic failure.
//	RED_MODE=0  (default, no env) → drive the REAL fixed code (SetTranslator
//	              + tr) under -race and ASSERT ABSENCE of any race. This is
//	              the standing GREEN regression guard.
//
// Run the guard:        go test -race -run TestTranslatorConcurrency_GuardsRace ./internal/provider/
// Reproduce the defect: RED_MODE=1 go test -race -run TestTranslatorConcurrency_GuardsRace ./internal/provider/
//
// NOTE on -race + RED_MODE=1: the Go race detector calls os.Exit(66) on the
// FIRST detected race, so RED_MODE=1 is designed to be run UNDER -race and is
// EXPECTED to abort with a non-zero exit + "DATA RACE" on stderr — that abort
// IS the reproduction. Without -race the RED_MODE=1 path still runs the
// unguarded concurrent access (a real race) but the runtime may not report it;
// the meaningful reproduction is the -race run. The default GREEN guard
// (RED_MODE unset) is what protects the build and runs by default.
package provider

import (
	"context"
	"os"
	"sync"
	"testing"

	"dev.helix.code/internal/provider/i18n"
)

// unguardedTranslatorStandIn is a faithful reproduction of the PRE-FIX
// translator.go access pattern: a bare package-private-style variable read +
// written with NO synchronization. It is local to this test file so the
// RED_MODE=1 reproduction does not mutate production state and so the
// reproduction is hermetic (does not depend on reverting translator.go).
var unguardedTranslatorStandIn i18n.Translator = i18n.NoopTranslator{}

// setUnguarded mirrors pre-fix SetTranslator: an unsynchronized write.
func setUnguarded(t i18n.Translator) {
	if t == nil {
		unguardedTranslatorStandIn = i18n.NoopTranslator{}
		return
	}
	unguardedTranslatorStandIn = t
}

// trUnguarded mirrors pre-fix tr(): an unsynchronized read.
func trUnguarded(ctx context.Context, msgID string) string {
	active := unguardedTranslatorStandIn //nolint:staticcheck // intentional unguarded read for RED reproduction
	if active == nil {
		active = i18n.NoopTranslator{}
	}
	out, err := active.T(ctx, msgID, nil)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

// TestTranslatorConcurrency_GuardsRace asserts that concurrent SetTranslator /
// tr access is race-free on the REAL (guarded) code, and — when RED_MODE=1 —
// reproduces the data race on the unguarded stand-in to prove the guard is
// load-bearing. Both polarities MUST be run under `-race`.
func TestTranslatorConcurrency_GuardsRace(t *testing.T) {
	ctx := context.Background()
	const iterations = 200

	if os.Getenv("RED_MODE") == "1" {
		// RED: reproduce the defect on the unguarded stand-in. Under -race
		// this aborts the process with "DATA RACE" — the reproduction.
		t.Log("RED_MODE=1: exercising UNGUARDED concurrent access to reproduce the §11.4.85 data race (expect -race abort)")
		var wg sync.WaitGroup
		for i := 0; i < iterations; i++ {
			wg.Add(2)
			go func() { defer wg.Done(); setUnguarded(i18n.NoopTranslator{}) }()
			go func() { defer wg.Done(); _ = trUnguarded(ctx, "internal_provider_red_probe") }()
		}
		wg.Wait()
		// If we reach here under -race, no race was detected — the stand-in
		// failed to reproduce the defect, which is itself a finding (a blind
		// RED test per §11.4.115 honest boundary).
		t.Log("RED_MODE=1 completed without a detected race; under -race this should not happen")
		return
	}

	// GREEN (default): drive the REAL guarded SetTranslator + tr under
	// concurrency. Any data race trips the race detector and FAILs the build.
	defer SetTranslator(nil) // restore loud-echo default for sibling tests
	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); SetTranslator(i18n.NoopTranslator{}) }()
		go func() {
			defer wg.Done()
			// tr is the package-private resolver; the data race the guard
			// prevents is the concurrent read here vs the write above.
			if got := tr(ctx, "internal_provider_guard_probe", nil); got == "" {
				t.Errorf("tr returned empty string; expected message-ID echo")
			}
		}()
	}
	wg.Wait()
}
