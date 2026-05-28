package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	securityi18n "dev.helix.code/internal/security/i18n"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL internal/security primitives.
//
// Chaos classes exercised against the production security code (no fakes for the
// surface under test; unit-test scope per CONST-050(A)):
//
//   - input-corruption / hostile-input: malicious feature names (injection
//     payloads, NUL + control bytes, megabyte-sized inputs, malformed UTF-8) are
//     fed through the REAL deterministic ScanFeatureContext path. The manager
//     MUST degrade cleanly (store-and-proceed) and NEVER panic/crash.
//   - state-corruption under contention: the SAME feature key is scanned and the
//     SAME counters mutated from many goroutines at once. The RWMutex must keep
//     the map + counters self-consistent — a torn map would panic or race.
//   - callback-panic isolation: a hostile Translator that PANICS is wired via
//     SetTranslator. The tr() resolver MUST isolate the panic and degrade to the
//     message ID rather than letting it crash the process. (This is the canonical
//     "panicking callback with no recover()" hunt class.)
//
// All net/exec scanner paths are deliberately avoided (no flaky real-network in
// chaos loops). Scanners are zero so ScanFeatureContext is fully deterministic.

// hostileFeatureInputs returns a corpus of malicious / malformed byte payloads
// used as feature names. None of these may crash the deterministic scan path.
func hostileFeatureInputs() [][]byte {
	return [][]byte{
		[]byte("'; DROP TABLE features; --"),               // SQL injection
		[]byte("$(rm -rf /)"),                               // command substitution
		[]byte("`reboot`"),                                  // backtick injection
		[]byte("../../../../etc/passwd"),                    // path traversal
		[]byte("%00%0a%0d"),                                 // percent-encoded control
		{0x00, 0x01, 0x02, 0x07, 0x08, 0x1b, 0x7f},          // raw NUL + control bytes
		[]byte("\n\r\t\v\f"),                                // whitespace control
		[]byte("${jndi:ldap://evil/x}"),                     // log4shell-style
		[]byte("<script>alert(1)</script>"),                 // XSS
		{0xff, 0xfe, 0xfd, 0xc0, 0x80},                      // invalid UTF-8
		[]byte(strings.Repeat("A", 1<<20)),                  // 1 MiB oversized input
		[]byte(strings.Repeat("名前", 50000)),                 // large multibyte
		[]byte("feature\x00with\x00embedded\x00nuls"), // embedded NULs
		{},                                            // empty payload
		// Unicode RTL-override (U+202E) prefix attack, byte-constructed so the
		// source file holds no raw bidi control char (avoids editor/parser bidi
		// spoofing) while still feeding the real control codepoint to the scanner.
		append([]byte{0xE2, 0x80, 0xAE}, []byte("RTL-override-attack")...),
	}
}

// TestSecurityManager_Chaos_HostileFeatureNames feeds malicious / malformed
// feature-name byte payloads through the REAL deterministic ScanFeatureContext
// path. Each must degrade into a clean result without crashing — a panic would be
// a §11.4.85(B) failure. The harness records a panic as Fatal.
func TestSecurityManager_Chaos_HostileFeatureNames(t *testing.T) {
	sm := NewSecurityManagerWithScanners()
	ctx := context.Background()

	stresschaos.ChaosCorruptInputDuring(t, "security_hostile_feature_names",
		hostileFeatureInputs(),
		func(input []byte) error {
			feature := string(input)
			res, err := sm.ScanFeatureContext(ctx, feature)
			if err != nil {
				// A returned error is a clean graceful-rejection path.
				return err
			}
			if res == nil {
				// nil-without-error would itself be a defect we must surface.
				return fmt.Errorf("scan returned nil result for hostile input (len=%d)", len(input))
			}
			// Empty feature name -> documented can't-proceed branch; everything
			// else -> store-and-proceed. Either way: no crash, valid result.
			if feature != "" && !res.CanProceed {
				return fmt.Errorf("non-empty hostile input unexpectedly blocked proceed")
			}
			// Accepted/normalised without crash — non-fatal (recorded Recovered).
			return nil
		})
}

// TestSecurityManager_Chaos_HostileScoreInput feeds malicious / unexpected
// SecurityIssue.Severity values through the REAL calculateScore function. Unknown
// severities must hit the default penalty branch, the score must never go below 0
// (floor clamp), and a huge issue slice must not panic. A crash here would be a
// §11.4.85(B) failure.
func TestSecurityManager_Chaos_HostileScoreInput(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "security_hostile_score_input", "input-corruption")

	hostileSeverities := []string{
		"", " ", "blocker", "BLOCKER ", "CRITICALCRITICAL",
		"'; DROP TABLE --", "\x00\x01", strings.Repeat("X", 100000),
		"名前", "\n\r", "${jndi}", "-999", "NaN",
	}

	for i, sev := range hostileSeverities {
		func(idx int, severity string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("calculateScore[%d] sev=%q PANICKED: %v", idx, severity, p))
				}
			}()
			// Mix of 1, 50, and 1000 issues of the hostile severity.
			for _, n := range []int{1, 50, 1000} {
				issues := make([]SecurityIssue, n)
				for j := range issues {
					issues[j] = SecurityIssue{
						Severity:    severity,
						Title:       severity,
						Description: severity,
						RuleID:      severity,
					}
				}
				score := calculateScore(issues)
				if score < 0 {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("calculateScore[%d] sev=%q n=%d returned NEGATIVE score %d (floor breach)", idx, severity, n, score))
					return
				}
				if score > 100 {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("calculateScore[%d] sev=%q n=%d returned >100 score %d", idx, severity, n, score))
					return
				}
			}
			rec.Record(stresschaos.Recovered,
				fmt.Sprintf("calculateScore[%d] sev=%q handled hostile severity without crash", idx, severity))
		}(i, sev)
	}

	rec.AssertNoFatal()
}

// TestSecurityManager_Chaos_ConcurrentSameKeyChurn scans the SAME feature key and
// mutates the SAME counters from many goroutines at once (state-corruption). The
// RWMutex must keep the scanResults map + counters self-consistent — under -race
// a torn map write would be reported. The terminal state must have the contended
// key resolvable to a valid result.
func TestSecurityManager_Chaos_ConcurrentSameKeyChurn(t *testing.T) {
	sm := NewSecurityManagerWithScanners()
	ctx := context.Background()
	rec := stresschaos.NewChaosRecorder(t, "security_same_key_churn", "state-corruption")

	const goroutines = 16
	const iters = 300
	const target = "churn_target_feature"

	var wg sync.WaitGroup
	var scans, reads, metricWrites int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				// Concurrent map write: overwrite the SAME key from all goroutines.
				if _, err := sm.ScanFeatureContext(ctx, target); err == nil {
					atomic.AddInt64(&scans, 1)
				}
				// Concurrent counter write under Lock.
				sm.UpdateSecurityMetrics(id%5, it%5, (id*it)%101)
				atomic.AddInt64(&metricWrites, 1)
				// Concurrent reads under RLock against the contended state.
				_ = sm.GetCriticalIssues()
				_ = sm.GetSecurityScore()
				_ = sm.ValidateZeroTolerance()
				atomic.AddInt64(&reads, 1)
			}
		}(g)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived same-key churn: %d scans, %d metric-writes, %d reads, no panic/race",
		atomic.LoadInt64(&scans), atomic.LoadInt64(&metricWrites), atomic.LoadInt64(&reads)))

	// Terminal state: the contended key must resolve to a valid stored result.
	sm.mutex.RLock()
	res, ok := sm.scanResults[target]
	sm.mutex.RUnlock()
	if !ok || res == nil {
		rec.Record(stresschaos.Fatal, "contended feature vanished after churn")
	} else if res.FeatureName != target {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("contended feature name corrupted: got %q", res.FeatureName))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("terminal state consistent: %q resolvable", target))
	}

	rec.AssertNoFatal()
	t.Logf("security churn: scans=%d metricWrites=%d reads=%d",
		atomic.LoadInt64(&scans), atomic.LoadInt64(&metricWrites), atomic.LoadInt64(&reads))
}

// panicTranslator is a hostile Translator implementation whose T method PANICS.
// It models a buggy / malicious translator wired via SetTranslator. The tr()
// resolver MUST isolate this panic and degrade to the message ID — a propagated
// panic would crash any goroutine that emits a user-facing string (the canonical
// "panicking callback with no recover()" defect class). Unit-test scope per
// CONST-050(A): this is a genuine Translator implementation, not a mock of a
// production type.
type panicTranslator struct{}

func (panicTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	panic("hostile translator panic during T")
}

func (panicTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	panic("hostile translator panic during TPlural")
}

// TestSecurityManager_Chaos_PanickingTranslatorIsolation wires a Translator that
// panics on every T() call, then drives the REAL tr() resolver. The resolver MUST
// NOT let the panic escape and crash the caller — it must degrade to the message
// ID. The harness records an escaped panic as Fatal.
func TestSecurityManager_Chaos_PanickingTranslatorIsolation(t *testing.T) {
	t.Cleanup(func() { SetTranslator(nil) }) // always restore the safe default
	rec := stresschaos.NewChaosRecorder(t, "security_panicking_translator", "callback-panic")

	SetTranslator(panicTranslator{})

	ctx := context.Background()
	for i := 0; i < 50; i++ {
		func(idx int) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("tr[%d] let hostile-translator panic ESCAPE: %v", idx, p))
				}
			}()
			out := tr(ctx, fmt.Sprintf("msg_%d", idx), map[string]any{"i": idx})
			if out == "" {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("tr[%d] returned empty after translator panic — no fallback", idx))
				return
			}
			// Degraded-but-up: resolver returned the message ID instead of crashing.
			rec.Record(stresschaos.Degraded,
				fmt.Sprintf("tr[%d] isolated translator panic, degraded to message ID %q", idx, out))
		}(i)
	}

	rec.AssertNoFatal()
}

// compile-time assertion: panicTranslator satisfies the real Translator contract,
// so it exercises the exact production injection seam (not a parallel interface).
var _ securityi18n.Translator = panicTranslator{}
