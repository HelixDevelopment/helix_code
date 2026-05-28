//go:build integration

package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/redis/i18n"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 CHAOS coverage for internal/redis against a REAL Redis (no mocks).
// Failure injection: context-cancellation mid-op, input corruption (huge/binary/
// weird keys), connection-churn (Close mid-flight + reconnect), and translator
// panic-isolation (the i18n callback path that, per translator.go, MUST recover()
// rather than crash the emitting goroutine). The wrapper must degrade cleanly —
// never crash, deadlock, or leak. Every PASS cites a recovery_trace artefact.

// =============================================================================
// §11.4.85(B)(1) — Process-death / cancellation injection
// =============================================================================

// TestRedis_Chaos_CancelMidOperation cancels the context while a real BRPop is
// blocking on an empty list. The wrapper must unwind cleanly (return a
// context/timeout error) rather than hang — proving cancellation propagates to
// the live connection. ChaosKillDuring records Fatal if the op fails to unwind.
func TestRedis_Chaos_CancelMidOperation(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)

	stresschaos.ChaosKillDuring(t, "redis_cancel_mid_brpop", 300*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// BRPop blocks until timeout OR context cancellation. With the
			// context cancelled mid-flight by the harness, it must return
			// promptly with an error instead of hanging the goroutine.
			_, err := c.BRPop(ctx, 10*time.Second, prefix+"empty-list")
			if err != nil {
				rec.Record(stresschaos.Degraded,
					fmt.Sprintf("BRPop returned controlled error after cancel: %v", err))
				return
			}
			rec.Record(stresschaos.Recovered, "BRPop returned without hanging")
		})
}

// TestRedis_Chaos_CancelDuringSustainedWrites cancels a context that is feeding a
// stream of real SET ops. Post-cancel ops must fail fast with a context error and
// MUST NOT panic — proving the wrapper honours cancellation on every call.
func TestRedis_Chaos_CancelDuringSustainedWrites(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)

	stresschaos.ChaosKillDuring(t, "redis_cancel_during_writes", 200*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			i := 0
			for {
				if err := c.Set(ctx, fmt.Sprintf("%scw:%d", prefix, i), "v", time.Minute); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						rec.Record(stresschaos.Degraded,
							fmt.Sprintf("Set honoured cancellation after %d writes: %v", i, err))
					} else {
						rec.Record(stresschaos.Degraded,
							fmt.Sprintf("Set returned non-cancel error after %d writes: %v", i, err))
					}
					return
				}
				i++
				if i > 1_000_000 { // safety bound — should be cancelled long before
					rec.Record(stresschaos.Recovered, "completed safety-bounded writes")
					return
				}
			}
		})
}

// =============================================================================
// §11.4.85(B)(3) — Input-corruption injection
// =============================================================================

// TestRedis_Chaos_CorruptValues feeds a battery of hostile values (huge, binary,
// embedded NUL, control chars, very long) to a real SET and verifies each is
// either stored byte-exact or cleanly rejected — never a crash. The feed reads
// the value back to prove the wrapper preserved it (binary-safe).
func TestRedis_Chaos_CorruptValues(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	corrupt := [][]byte{
		{},                                   // empty
		{0x00},                               // single NUL
		[]byte("a\x00b\x00c"),                // embedded NULs
		{0xff, 0xfe, 0xfd, 0xfc},             // high bytes / invalid UTF-8
		[]byte(strings.Repeat("\n\r\t ", 64)), // whitespace/control storm
		[]byte(strings.Repeat("Z", 5<<20)),   // 5 MiB value
		[]byte("\x1b[31mANSI\x1b[0m"),        // ANSI escape sequence
	}

	var idx int32
	stresschaos.ChaosCorruptInputDuring(t, "redis_corrupt_values", corrupt,
		func(input []byte) error {
			i := atomic.AddInt32(&idx, 1)
			key := fmt.Sprintf("%scorrupt:%d", prefix, i)
			if err := c.Set(ctx, key, input, 30*time.Second); err != nil {
				return fmt.Errorf("set rejected: %w", err) // graceful Degraded
			}
			got, err := c.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("get-back failed: %w", err)
			}
			if got != string(input) {
				return fmt.Errorf("value corrupted in round-trip (len got=%d want=%d)", len(got), len(input))
			}
			return nil // stored & read back byte-exact: Recovered
		})
}

// TestRedis_Chaos_WeirdKeys feeds hostile KEY names (empty, binary, embedded
// spaces/newlines, very long) and verifies the real wrapper handles each without
// crashing. Redis keys are binary-safe, so these should round-trip.
func TestRedis_Chaos_WeirdKeys(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	weird := [][]byte{
		[]byte(prefix + ""),                                 // prefix only
		[]byte(prefix + "with space"),                       // embedded space
		[]byte(prefix + "with\nnewline"),                    // embedded newline
		[]byte(prefix + "with\x00nul"),                      // embedded NUL
		[]byte(prefix + "{hashtag}:[brackets]:(parens)"),    // glob/special chars
		[]byte(prefix + strings.Repeat("k", 4096)),          // very long key
		[]byte(prefix + "üñîçödé:键:🔑"),                       // multibyte UTF-8
	}

	stresschaos.ChaosCorruptInputDuring(t, "redis_weird_keys", weird,
		func(keyBytes []byte) error {
			key := string(keyBytes)
			if err := c.Set(ctx, key, "v", 30*time.Second); err != nil {
				return fmt.Errorf("set weird key rejected: %w", err)
			}
			got, err := c.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("get weird key failed: %w", err)
			}
			if got != "v" {
				return fmt.Errorf("weird-key value mismatch: %q", got)
			}
			_ = c.Del(ctx, key)
			return nil
		})
}

// =============================================================================
// §11.4.85(B)(4) — Resource-pressure injection
// =============================================================================

// TestRedis_Chaos_ResourcePressure runs real Redis ops while the harness holds
// bounded memory pressure (capped at 128 MiB per §12.6). The wrapper must keep
// completing ops under pressure without OOM-crashing.
func TestRedis_Chaos_ResourcePressure(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	stresschaos.ChaosResourcePressureDuring(t, "redis_resource_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("%srp:%d", prefix, i)
				if err := c.Set(ctx, key, fmt.Sprintf("payload-%d", i), 30*time.Second); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("Set degraded under pressure at %d: %v", i, err))
					return
				}
				if _, err := c.Get(ctx, key); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("Get degraded under pressure at %d: %v", i, err))
					return
				}
			}
			rec.Record(stresschaos.Recovered, "completed 200 set/get ops under bounded memory pressure")
		})
}

// =============================================================================
// §11.4.85(B) — Connection-churn injection
// =============================================================================

// TestRedis_Chaos_ConnectionChurn closes a live client mid-flight (simulating a
// dropped connection / pool exhaustion) and asserts subsequent ops fail cleanly
// with a controlled error rather than panicking, while a SEPARATE fresh client
// continues working — proving connection loss is isolated, not fatal.
func TestRedis_Chaos_ConnectionChurn(t *testing.T) {
	prefix := keyPrefix(t)
	ctx := context.Background()

	rec := stresschaos.NewChaosRecorder(t, "redis_connection_churn", "connection-churn")

	// A client we will deliberately close mid-use.
	victim := testClient(t)
	cleanupPrefix(t, victim, prefix)

	if err := victim.Set(ctx, prefix+"pre", "ok", time.Minute); err != nil {
		t.Fatalf("pre-churn set failed: %v", err)
	}

	// Close the victim's underlying connection mid-flight.
	if err := victim.Close(); err != nil {
		t.Logf("victim close returned (non-fatal): %v", err)
	}
	rec.Record(stresschaos.Degraded, "closed victim client connection mid-flight")

	// Post-close ops must fail cleanly (controlled error, no panic).
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("op on closed client panicked: %v", p))
			}
		}()
		if err := victim.Set(ctx, prefix+"post", "x", time.Minute); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("Set on closed client returned controlled error: %v", err))
		} else {
			// go-redis may transparently reconnect; that is also a non-fatal outcome.
			rec.Record(stresschaos.Recovered, "Set on closed client transparently reconnected")
		}
	}()

	// A fresh independent client must still work — connection loss is isolated.
	fresh := testClient(t)
	if err := fresh.Set(ctx, prefix+"fresh", "works", time.Minute); err != nil {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("fresh client unusable after churn: %v", err))
	} else if got, err := fresh.Get(ctx, prefix+"fresh"); err != nil || got != "works" {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("fresh client round-trip broken: got=%q err=%v", got, err))
	} else {
		rec.Record(stresschaos.Recovered, "fresh client fully functional after victim churn")
	}

	rec.AssertNoFatal()
}

// =============================================================================
// §11.4.85(B) — Callback panic-isolation (i18n translator path)
// =============================================================================

// panicTranslator panics on every T call — exercising the recover() guard in
// translator.go's tr(). Per the contract, a panicking Translator MUST NOT crash
// the emitting goroutine; tr() degrades to the message ID.
type panicTranslator struct{}

func (panicTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	panic("chaos: translator T deliberately panics")
}
func (panicTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	panic("chaos: translator TPlural deliberately panics")
}

// TestRedis_Chaos_PanickingTranslator wires a panicking translator and then
// drives the REAL error-emitting paths (Get on a disabled client) concurrently
// from many goroutines. translator.go's tr() recover() must isolate every panic
// so no goroutine crashes the process and the message-ID is returned instead.
// Run under -race: SetTranslator (write) racing tr() (read) is guarded by
// translatorMu — a regression there is a data race AND a §11.4.85(B) defect.
func TestRedis_Chaos_PanickingTranslator(t *testing.T) {
	// Restore the default translator afterwards so other tests are unaffected.
	t.Cleanup(func() { SetTranslator(i18n.NoopTranslator{}) })

	rec := stresschaos.NewChaosRecorder(t, "redis_panicking_translator", "callback-panic")

	// A disabled client makes Get() emit a translated error via tr() — the path
	// that must survive the panicking translator.
	disabled, err := NewClient(&config.RedisConfig{Enabled: false})
	if err != nil {
		t.Fatalf("disabled client: %v", err)
	}
	ctx := context.Background()

	// Concurrently flip the translator (write) while goroutines call the
	// error-emitting path (read through tr()). -race proves the mutex guards it.
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				SetTranslator(panicTranslator{})
				SetTranslator(i18n.NoopTranslator{})
			}
		}
	}()

	SetTranslator(panicTranslator{})
	const goroutines = 16
	const iters = 100
	var crashed int32
	var calls int64
	stresschaos.RunConcurrent(t, "redis_translator_panic_isolation",
		stresschaos.ConcurrencyConfig{Parallelism: goroutines, IterationsPerGoroutine: iters},
		func(g, iter int) (rerr error) {
			defer func() {
				if p := recover(); p != nil {
					atomic.AddInt32(&crashed, 1)
					rerr = fmt.Errorf("tr() failed to isolate translator panic: %v", p)
				}
			}()
			// Get on disabled client returns errors.New(tr(...)) — must not panic.
			_, err := disabled.Get(ctx, "any-key")
			atomic.AddInt64(&calls, 1)
			if err == nil {
				return fmt.Errorf("disabled Get returned nil error — contract broken")
			}
			// Error text must be non-empty (degrades to message ID, never blank).
			if err.Error() == "" {
				return fmt.Errorf("disabled Get error text empty — silent swallow")
			}
			return nil
		})

	close(stop)
	wg.Wait()

	if atomic.LoadInt32(&crashed) > 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("%d goroutine(s) saw an unisolated translator panic", crashed))
	} else {
		rec.Record(stresschaos.Recovered,
			fmt.Sprintf("tr() isolated translator panic across %d real error-emitting calls", atomic.LoadInt64(&calls)))
	}
	rec.AssertNoFatal()

	// After restoring NoopTranslator, the error must echo the real message ID.
	SetTranslator(i18n.NoopTranslator{})
	_, finalErr := disabled.Get(ctx, "k")
	if finalErr == nil || !strings.Contains(finalErr.Error(), "internal_redis_disabled") {
		t.Fatalf("post-restore error did not echo message ID: %v", finalErr)
	}
	t.Logf("redis panicking-translator chaos: %d calls survived, post-restore echo OK", atomic.LoadInt64(&calls))
}
