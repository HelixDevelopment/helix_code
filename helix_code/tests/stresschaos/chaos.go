package stresschaos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// EventCategory classifies how the system-under-test responded to an injected
// fault, per the §11.4.85(B) closed three-bucket taxonomy.
type EventCategory int

const (
	// Recovered: the operation completed correctly despite the fault (retry,
	// circuit-break, fallback, reschedule).
	Recovered EventCategory = iota
	// Degraded: the operation did not complete but the system stayed up and
	// returned a controlled error / backpressure signal (graceful degradation).
	Degraded
	// Fatal: the system crashed, panicked, deadlocked, or corrupted state — a
	// §11.4.85 failure.
	Fatal
)

func (c EventCategory) String() string {
	switch c {
	case Recovered:
		return "RECOVERED"
	case Degraded:
		return "DEGRADED"
	case Fatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ChaosRecorder accumulates categorised recovery events during a chaos run and
// writes the §11.4.85 recovery_trace artefact. It is safe for concurrent use.
type ChaosRecorder struct {
	t         testing.TB
	name      string
	faultKind string
	mu        sync.Mutex
	trace     RecoveryTrace
}

// NewChaosRecorder starts a recovery trace for a named chaos run.
func NewChaosRecorder(t testing.TB, name, faultKind string) *ChaosRecorder {
	t.Helper()
	r := &ChaosRecorder{
		t:         t,
		name:      name,
		faultKind: faultKind,
		trace: RecoveryTrace{
			Name:      name,
			FaultKind: faultKind,
			Events:    make([]string, 0, 16),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	// trap-equivalent cleanup (§11.4.14): the trace is always flushed, even if a
	// later assertion fails, so the evidence survives the failure.
	t.Cleanup(func() { r.flush() })
	return r
}

// Record logs a categorised event. detail is a short human-readable description.
func (r *ChaosRecorder) Record(cat EventCategory, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch cat {
	case Recovered:
		r.trace.Recovered++
	case Degraded:
		r.trace.Degraded++
	case Fatal:
		r.trace.Fatal++
	}
	r.trace.Events = append(r.trace.Events, fmt.Sprintf("%s %s: %s",
		time.Now().UTC().Format(time.RFC3339Nano), cat, detail))
}

// AssertNoFatal flushes the trace and FAILS the test if any Fatal event was
// recorded. A chaos PASS means the system recovered or degraded gracefully — it
// never means the system crashed.
func (r *ChaosRecorder) AssertNoFatal() RecoveryTrace {
	r.t.Helper()
	tr := r.flush()
	if tr.Fatal > 0 {
		r.t.Fatalf("stresschaos: chaos %q (%s) recorded %d FATAL event(s) — system did not survive failure (evidence: recovery_trace)",
			r.name, r.faultKind, tr.Fatal)
	}
	if tr.Recovered == 0 && tr.Degraded == 0 {
		r.t.Fatalf("stresschaos: chaos %q (%s) recorded no recovered/degraded events — no positive recovery evidence (not a PASS per §11.4.85)",
			r.name, r.faultKind)
	}
	return tr
}

// flush writes the recovery_trace.{json,log} artefacts and returns a snapshot.
func (r *ChaosRecorder) flush() RecoveryTrace {
	r.mu.Lock()
	snapshot := r.trace
	r.mu.Unlock()

	dir := evidenceDir(r.t, r.name)
	jsonPath := filepath.Join(dir, "recovery_trace.json")
	writeJSON(r.t, jsonPath, snapshot)

	// Human-readable categorised log alongside the JSON (the §11.4.85 named shape).
	logPath := filepath.Join(dir, "recovery_trace.log")
	var b []byte
	b = append(b, fmt.Sprintf("# recovery_trace %s fault=%s recovered=%d degraded=%d fatal=%d\n",
		snapshot.Name, snapshot.FaultKind, snapshot.Recovered, snapshot.Degraded, snapshot.Fatal)...)
	for _, e := range snapshot.Events {
		b = append(b, (e + "\n")...)
	}
	if err := os.WriteFile(logPath, b, 0o644); err != nil {
		r.t.Fatalf("stresschaos: write recovery_trace.log %s: %v", logPath, err)
	}
	verifyArtefact(r.t, logPath)
	return snapshot
}

// ChaosKillDuring injects a process-death fault (§11.4.85(B)(1)). It starts the
// long-running operation `op` (which honours the supplied context), waits `after`,
// then cancels the context mid-operation — simulating a worker/goroutine killed
// while busy. `op` should classify its outcome into the recorder. The helper
// guarantees op is not left running (context is always cancelled via t.Cleanup).
//
// op signature: it receives a cancellable context and the recorder, runs until
// either it finishes or the context is cancelled, and records the outcome.
func ChaosKillDuring(t testing.TB, name string, after time.Duration, op func(ctx context.Context, rec *ChaosRecorder)) RecoveryTrace {
	t.Helper()
	rec := NewChaosRecorder(t, name, "process-death")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // §11.4.14 trap-equivalent: never leak the goroutine/op

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if p := recover(); p != nil {
				rec.Record(Fatal, fmt.Sprintf("op panicked under process-death: %v", p))
			}
		}()
		op(ctx, rec)
	}()

	time.Sleep(after)
	rec.Record(Degraded, "injected process-death: cancelling context mid-operation")
	cancel()

	// Bounded wait for op to observe the cancellation and unwind.
	select {
	case <-done:
		rec.Record(Recovered, "operation unwound cleanly after cancellation")
	case <-time.After(10 * time.Second):
		rec.Record(Fatal, "operation did not unwind within 10s after cancellation (deadlock)")
	}

	return rec.AssertNoFatal()
}

// ChaosCorruptInputDuring injects an input-corruption fault (§11.4.85(B)(3)). For
// each corrupt input produced by `corruptInputs`, it calls `feed`; the system is
// expected to reject the malformed input without crashing. `feed` returning an
// error is the desired graceful-rejection path (recorded Degraded); a panic is
// recorded Fatal.
func ChaosCorruptInputDuring(t testing.TB, name string, corruptInputs [][]byte, feed func(input []byte) error) RecoveryTrace {
	t.Helper()
	rec := NewChaosRecorder(t, name, "input-corruption")

	for idx, in := range corruptInputs {
		func(i int, payload []byte) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(Fatal, fmt.Sprintf("feed[%d] panicked on corrupt input: %v", i, p))
				}
			}()
			err := feed(payload)
			if err != nil {
				rec.Record(Degraded, fmt.Sprintf("feed[%d] rejected corrupt input cleanly: %v", i, err))
			} else {
				// Accepting/normalising without a crash is a non-fatal outcome;
				// callers requiring strict rejection can tighten with an assertion.
				rec.Record(Recovered, fmt.Sprintf("feed[%d] accepted/normalised input without crash", i))
			}
		}(idx, in)
	}

	return rec.AssertNoFatal()
}

// ChaosResourcePressureDuring injects a bounded resource-exhaustion fault
// (§11.4.85(B)(4)). It allocates a bounded number of fixed-size buffers to create
// memory pressure while running `op`, asserting the system applies backpressure /
// degrades gracefully rather than OOM-crashing. The allocation is STRICTLY bounded
// (totalMB capped at 128) to honour the §12.6 host-safety ceiling — this never
// tries to exhaust real host RAM, only to create localised pressure. All buffers
// are freed via t.Cleanup.
func ChaosResourcePressureDuring(t testing.TB, name string, totalMB int, op func(rec *ChaosRecorder)) RecoveryTrace {
	t.Helper()
	if totalMB <= 0 {
		totalMB = 16
	}
	const maxMB = 128 // hard cap regardless of caller input — stays under §12.6 ceiling
	if totalMB > maxMB {
		totalMB = maxMB
	}

	rec := NewChaosRecorder(t, name, "resource-exhaustion")

	const chunkMB = 4
	chunks := totalMB / chunkMB
	if chunks < 1 {
		chunks = 1
	}
	ballast := make([][]byte, 0, chunks)
	t.Cleanup(func() {
		ballast = nil
		runtime.GC()
	})

	for i := 0; i < chunks; i++ {
		buf := make([]byte, chunkMB*1024*1024)
		for j := 0; j < len(buf); j += 4096 { // touch pages so allocation is real
			buf[j] = byte(i)
		}
		ballast = append(ballast, buf)
	}
	rec.Record(Degraded, fmt.Sprintf("allocated %d MB bounded memory pressure (cap %d MB)", chunks*chunkMB, maxMB))

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(Fatal, fmt.Sprintf("op panicked under memory pressure: %v", p))
			}
		}()
		op(rec)
	}()
	rec.Record(Recovered, "operation completed under bounded memory pressure without OOM-crash")

	return rec.AssertNoFatal()
}
