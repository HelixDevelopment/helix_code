package logging

import (
	stdctx "context"
	"sync"
	"testing"

	loggingi18n "dev.helix.code/internal/logging/i18n"
)

// panicTranslator is a hostile/buggy Translator whose T always panics. It proves
// the tr() recover guard isolates a panicking injected translator (HXC-014b).
type panicTranslator struct{}

func (panicTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	panic("HXC-014b: hostile translator panic")
}

func (panicTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	panic("HXC-014b: hostile translator panic (plural)")
}

// TestTranslator_HXC014b_ConcurrentSetAndResolve is the anti-bluff regression
// proof for the systemic translator.go guarding sweep (HXC-014b). It exercises
// the two defects the sweep fixes across ~54 packages, using internal/logging as
// the representative guarded seam (every fixed package shares the identical
// pattern):
//
//  1. DATA RACE — SetTranslator (write) racing tr (read) on the package-level
//     `translator` var. Run under `go test -race`: without translatorMu this
//     reports WARNING: DATA RACE.
//  2. PANIC CRASH — a panicking translator must degrade to the message ID, not
//     crash the emitting goroutine (and thus the process).
//
// Restore the seam to its pre-HXC-014b shape (drop translatorMu + the recover)
// and this test fails: the -race detector fires on the unguarded var, and the
// panic path crashes the test binary.
func TestTranslator_HXC014b_ConcurrentSetAndResolve(t *testing.T) {
	t.Cleanup(func() { SetTranslator(loggingi18n.NoopTranslator{}) })

	const goroutines = 16
	const iters = 300

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers: continuously swap the translator (boot-style reconfiguration
	// racing the readers below).
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if (g+i)%3 == 0 {
					SetTranslator(panicTranslator{})
				} else {
					SetTranslator(loggingi18n.NoopTranslator{})
				}
			}
		}(g)
	}

	// Readers: resolve message IDs concurrently. A panicTranslator swapped in
	// mid-flight must NOT crash this goroutine — tr must degrade to the msgID.
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				got := tr(stdctx.Background(), "test_msg_id", nil)
				if got == "" {
					// tr must never return empty — it degrades to the msgID.
					t.Errorf("tr returned empty string; expected msgID fallback")
					return
				}
			}
		}()
	}

	wg.Wait()

	// After a panicking translator is active, tr must still return the msgID.
	SetTranslator(panicTranslator{})
	if got := tr(stdctx.Background(), "panic_path_msg", nil); got != "panic_path_msg" {
		t.Fatalf("tr under panicking translator = %q; want msgID fallback %q", got, "panic_path_msg")
	}
}
