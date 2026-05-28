package session

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the session transcript store + resume finder.
//
// The unit under stress is the REAL file-backed TranscriptStore (real os.OpenFile
// O_APPEND writes, real JSONL parsing, real metadata.json sidecars) plus the REAL
// ResumeFinder — no fakes. Storage is rooted at t.TempDir() so the tests exercise
// genuine disk I/O against the production code paths. Sustained Append/Read/Resume
// load (N>=100) + N>=10 concurrent appenders (each on a distinct session so the
// per-session append flow runs under genuine filesystem contention).

// TestTranscriptStore_Stress_SustainedAppendReadResume drives the real
// Append -> ReadTranscript -> Resume lifecycle under sustained load (N>=100),
// recording per-call latency. Each iteration appends a message, reads the full
// transcript back, and resumes it, asserting the resumed transcript grows
// monotonically so the run proves real persisted work.
func TestTranscriptStore_Stress_SustainedAppendReadResume(t *testing.T) {
	store := NewTranscriptStore(t.TempDir())
	finder := NewResumeFinder(store)
	ctx := context.Background()
	const sid = "stress-session"

	var appended int64
	stresschaos.RunSustainedLoad(t, "session_sustained_append_read_resume",
		stresschaos.SustainedConfig{N: 600, MaxErrorRate: 0.0},
		func(i int) error {
			msg := Message{Role: "user", Content: fmt.Sprintf("stress message %d", i), Timestamp: time.Now().UTC()}
			if err := store.Append(ctx, sid, msg); err != nil {
				return fmt.Errorf("append: %w", err)
			}
			atomic.AddInt64(&appended, 1)
			msgs, meta, err := finder.Resume(ctx, sid)
			if err != nil {
				return fmt.Errorf("resume: %w", err)
			}
			// The resumed transcript must contain every message appended so far.
			if len(msgs) != int(atomic.LoadInt64(&appended)) {
				return fmt.Errorf("resume transcript length %d != appended %d", len(msgs), atomic.LoadInt64(&appended))
			}
			if meta.MessageCount != len(msgs) {
				return fmt.Errorf("metadata count %d != transcript length %d", meta.MessageCount, len(msgs))
			}
			return nil
		})

	if atomic.LoadInt64(&appended) == 0 {
		t.Fatal("transcript store appended zero messages under sustained load — not real work")
	}
	t.Logf("session sustained: %d messages appended+read+resumed", atomic.LoadInt64(&appended))
}

// TestTranscriptStore_Stress_ConcurrentAppends hammers Append from N>=10
// concurrent goroutines, each writing to its OWN session, asserting no deadlock,
// no goroutine leak, and no data race (run under -race) across the real disk
// writes + metadata sidecar updates. After the run every session's transcript is
// resumed and asserted to hold exactly the messages that goroutine wrote — proof
// the concurrent writes did not corrupt or lose data.
func TestTranscriptStore_Stress_ConcurrentAppends(t *testing.T) {
	store := NewTranscriptStore(t.TempDir())
	finder := NewResumeFinder(store)
	ctx := context.Background()

	const parallelism = 12
	const iters = 60

	var writes int64
	stresschaos.RunConcurrent(t, "session_concurrent_appends",
		stresschaos.ConcurrencyConfig{Parallelism: parallelism, IterationsPerGoroutine: iters, Timeout: 30 * time.Second},
		func(g, it int) error {
			sid := fmt.Sprintf("sess-%d", g)
			msg := Message{Role: "assistant", Content: fmt.Sprintf("g%d-it%d", g, it), Timestamp: time.Now().UTC()}
			if err := store.Append(ctx, sid, msg); err != nil {
				return fmt.Errorf("append g%d it%d: %w", g, it, err)
			}
			atomic.AddInt64(&writes, 1)
			return nil
		})

	// Post-run consistency: each per-goroutine session must resume to exactly
	// `iters` messages — no lost or duplicated appends under concurrency.
	for g := 0; g < parallelism; g++ {
		sid := fmt.Sprintf("sess-%d", g)
		msgs, _, err := finder.Resume(ctx, sid)
		if err != nil {
			t.Fatalf("resume %s after concurrent appends: %v", sid, err)
		}
		if len(msgs) != iters {
			t.Fatalf("session %s resumed %d messages, want %d (concurrent append lost/duplicated data)", sid, len(msgs), iters)
		}
	}

	if atomic.LoadInt64(&writes) != parallelism*iters {
		t.Fatalf("expected %d total writes, got %d", parallelism*iters, atomic.LoadInt64(&writes))
	}
	t.Logf("session concurrent: %d appends across %d sessions, all transcripts consistent",
		atomic.LoadInt64(&writes), parallelism)
}
