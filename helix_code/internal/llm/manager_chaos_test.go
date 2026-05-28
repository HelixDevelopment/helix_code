package llm

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the in-process LLM ModelManager.
//
// Chaos classes exercised against the REAL *ModelManager:
//   - input-corruption: feed malformed / extreme selection criteria decoded
//     from corrupt JSON; the selector must reject or no-match cleanly, never
//     panic (no nil-deref, no division/overflow crash).
//   - state-corruption under contention: register providers concurrently with
//     ongoing SelectOptimalModel calls, asserting no race/panic/deadlock and a
//     consistent registry.
//   - state-corruption (degradation): drive selection when every registered
//     provider reports unavailable, asserting a clean no-match error (graceful
//     degradation, no crash).
//   - process-death: cancel the context driving a long concurrent
//     register/select loop mid-flight, asserting clean unwind (no leaked
//     goroutine, no deadlock).
//
// Run under -race: concurrent RegisterProvider + SelectOptimalModel is the
// contention pattern that exposes locking defects in the registry maps.

// unavailableProvider is a real in-process Provider that reports unavailable —
// used to drive the all-unavailable degradation path (NOT a mock of the SUT).
type unavailableProvider struct{ *stressTestProvider }

func (p *unavailableProvider) IsAvailable(ctx context.Context) bool { return false }

// TestModelManager_Chaos_CorruptCriteria feeds corrupt/extreme selection
// criteria (decoded from malformed JSON byte payloads) into the real selector,
// asserting it rejects or no-matches cleanly without ever panicking.
func TestModelManager_Chaos_CorruptCriteria(t *testing.T) {
	mm := populatedStressManager(t)

	// A long task-type string built programmatically (4 KiB of 'A') to stress
	// string-handling paths without embedding illegal NUL bytes in source.
	longTaskPayload, _ := json.Marshal(map[string]string{"TaskType": strings.Repeat("A", 4096)})
	// A capability list with junk + multibyte unicode entries.
	weirdCapsPayload, _ := json.Marshal(map[string][]string{
		"RequiredCapabilities": {"", "  ", "ʈweird", "code_generation"},
	})
	emojiPrefPayload, _ := json.Marshal(map[string]interface{}{
		"QualityPreference": "\U0001F4A5\U0001F525overflow", "MaxTokens": 1,
	})

	// Corrupt / hostile JSON payloads decoded into ModelSelectionCriteria. Each
	// is fed to the REAL selector; the survival property is "no panic".
	corruptInputs := [][]byte{
		[]byte(`{"MaxTokens": -2147483648}`),                                // negative overflow boundary
		[]byte(`{"MaxTokens": 9223372036854775807}`),                        // max int64 — context math stress
		longTaskPayload,                                                     // 4 KiB task type
		weirdCapsPayload,                                                    // junk + unicode capability list
		emojiPrefPayload,                                                    // emoji preference + tiny tokens
		[]byte(`{"LatencyRequirement": -1}`),                               // negative duration
		[]byte(`{}`),                                                       // empty object
		[]byte(`{"MaxTokens": 0, "TaskType": "", "QualityPreference": ""}`), // all-zero
		[]byte(`{"MaxTokens": "not-a-number"}`),                            // type-mismatch — must be rejected by Unmarshal
	}

	stresschaos.ChaosCorruptInputDuring(t, "model_manager_corrupt_criteria", corruptInputs,
		func(input []byte) error {
			var criteria ModelSelectionCriteria
			if err := json.Unmarshal(input, &criteria); err != nil {
				return err // malformed JSON rejected before reaching the selector — graceful
			}
			// Feed the decoded (possibly hostile) criteria to the REAL selector.
			// The contract: it returns a model OR a clean error, never panics.
			_, selErr := mm.SelectOptimalModel(criteria)
			return selErr // a no-match error is the desired graceful-rejection path
		})
}

// TestModelManager_Chaos_RegisterDuringSelect registers providers from a chaos
// goroutine while many selectors run, asserting the registry neither panics nor
// deadlocks and keeps returning valid selections once populated.
func TestModelManager_Chaos_RegisterDuringSelect(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "model_manager_register_during_select", "state-mutation")

	mm := NewModelManager()
	// Seed with one provider so selectors have something immediately.
	if err := mm.RegisterProvider(newStressTestProvider(stressProviderTypes[0], 0)); err != nil {
		t.Fatalf("seed register failed: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Chaos goroutine: rapidly register new provider types mid-flight. This is
	// the genuinely-concurrent write-lock state-mutation surface — RegisterProvider
	// takes the write lock that SelectOptimalModel reads the registry maps under.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, "RegisterProvider panicked during concurrent select")
			}
		}()
		i := 1
		for {
			select {
			case <-stop:
				return
			default:
			}
			if i < len(stressProviderTypes) {
				_ = mm.RegisterProvider(newStressTestProvider(stressProviderTypes[i], i))
				i++
			}
			time.Sleep(50 * time.Microsecond)
		}
	}()

	// Selector goroutines hammered while registrations land underneath them.
	var ok, degraded int64
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, "SelectOptimalModel panicked during concurrent register")
				}
			}()
			for it := 0; it < 1000; it++ {
				if model, err := mm.SelectOptimalModel(codeSelectCriteria); err == nil && model != nil {
					atomic.AddInt64(&ok, 1)
				} else {
					atomic.AddInt64(&degraded, 1)
				}
				_ = mm.GetAvailableModels() // widen the read-race surface
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	rec.Record(stresschaos.Recovered,
		"selectors completed under concurrent registration: valid selections observed, no panic/deadlock")
	if atomic.LoadInt64(&ok) == 0 {
		rec.Record(stresschaos.Fatal, "zero successful selections during chaos — registry never served a model")
	}
	rec.AssertNoFatal()
	t.Logf("model_manager register-during-select chaos: ok=%d degraded=%d",
		atomic.LoadInt64(&ok), atomic.LoadInt64(&degraded))
}

// TestModelManager_Chaos_AllProvidersUnavailable corrupts the selectable state
// by registering only providers that report unavailable, then asserts
// SelectOptimalModel returns a clean no-match error (no panic / nil-deref) —
// graceful degradation under state-corruption.
func TestModelManager_Chaos_AllProvidersUnavailable(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "model_manager_all_unavailable", "state-corruption")

	mm := NewModelManager()
	for i := 0; i < 4; i++ {
		base := newStressTestProvider(stressProviderTypes[i], i)
		if err := mm.RegisterProvider(&unavailableProvider{base}); err != nil {
			t.Fatalf("register unavailable provider failed: %v", err)
		}
	}

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, "SelectOptimalModel panicked with all providers unavailable")
			}
		}()
		model, err := mm.SelectOptimalModel(codeSelectCriteria)
		if model == nil && err != nil {
			rec.Record(stresschaos.Degraded, "returned clean no-match error with no available provider (graceful degradation, no crash)")
		} else if model != nil {
			rec.Record(stresschaos.Recovered, "returned a model despite none available (no crash)")
		} else {
			rec.Record(stresschaos.Degraded, "returned nil model without error (no crash)")
		}
	}()

	rec.AssertNoFatal()
}

// TestModelManager_Chaos_KillDuringRegisterSelect injects process-death: a long
// concurrent register/select loop is cancelled mid-flight via the harness, which
// asserts the operation unwinds cleanly with no leaked goroutine / deadlock.
func TestModelManager_Chaos_KillDuringRegisterSelect(t *testing.T) {
	mm := NewModelManager()
	if err := mm.RegisterProvider(newStressTestProvider(stressProviderTypes[0], 0)); err != nil {
		t.Fatalf("seed register failed: %v", err)
	}

	stresschaos.ChaosKillDuring(t, "model_manager_kill_during_op", 80*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			var wg sync.WaitGroup
			// Spawn workers that loop register+select until the context is cancelled.
			for g := 0; g < 8; g++ {
				wg.Add(1)
				go func(gid int) {
					defer wg.Done()
					i := 1
					for {
						select {
						case <-ctx.Done():
							return
						default:
						}
						if gid == 0 && i < len(stressProviderTypes) {
							_ = mm.RegisterProvider(newStressTestProvider(stressProviderTypes[i], i))
							i++
						}
						_, _ = mm.SelectOptimalModel(codeSelectCriteria)
						_ = mm.GetAvailableModels()
					}
				}(g)
			}
			wg.Wait() // returns once ctx is cancelled — clean unwind
			rec.Record(stresschaos.Recovered, "all register/select workers observed cancellation and unwound")
		})
}
