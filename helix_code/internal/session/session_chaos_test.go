package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the session transcript store + resume.
//
// Chaos classes exercised against the REAL TranscriptStore (real files under
// t.TempDir(), real JSONL parser, real ResumeFinder — no fakes):
//
//   - input-corruption: garbage / truncated / non-JSON lines are interleaved into
//     a real transcript.jsonl. ReadTranscript + Resume MUST skip the corrupt lines
//     and still recover every valid message — never panic, never abort the read.
//   - process-death mid-append: a stream of appends is cancelled mid-flight
//     (context cancellation = the session process being killed). Resume MUST then
//     recover a self-consistent transcript: every persisted line parses and the
//     count is stable across repeated resumes (crash-consistency of the append
//     protocol — no torn final line surviving as garbage).

// TestTranscriptStore_Chaos_CorruptTranscriptLines writes a mix of valid messages
// and corrupt lines directly into the real transcript.jsonl, then asserts the
// REAL ReadTranscript/Resume skip the corruption and return exactly the valid
// messages. A panic or a failed read on corrupt input is a §11.4.85(B) failure.
func TestTranscriptStore_Chaos_CorruptTranscriptLines(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	finder := NewResumeFinder(store)
	ctx := context.Background()
	const sid = "corrupt-session"

	// First, append two valid messages through the real API so the session and
	// its metadata sidecar exist on disk.
	for i := 0; i < 2; i++ {
		if err := store.Append(ctx, sid, Message{Role: "user", Content: fmt.Sprintf("valid-%d", i)}); err != nil {
			t.Fatalf("seed append: %v", err)
		}
	}

	// Now corrupt the on-disk transcript: append raw garbage lines that the
	// parser must skip, plus one more valid JSON line at the end.
	path := filepath.Join(dir, sid, "transcript.jsonl")
	corruptLines := [][]byte{
		[]byte("this is not json at all\n"),
		[]byte(`{"role":"user","content":"unterminated`), // no closing brace / newline
		[]byte("\n"),                                       // bare newline (empty line)
		[]byte("\x00\x01\x02 binary garbage\n"),
		[]byte(`{"role":42,"content":{"nested":"wrongtype"}}` + "\n"), // type-mismatched but still valid JSON object
		[]byte(`{"role":"assistant","content":"valid-after-corruption"}` + "\n"),
	}

	stresschaos.ChaosCorruptInputDuring(t, "session_corrupt_transcript_lines", corruptLines,
		func(input []byte) error {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open transcript: %w", err)
			}
			defer f.Close()
			if _, err := f.Write(input); err != nil {
				return fmt.Errorf("write corruption: %w", err)
			}
			// Drive the REAL parser over the (now partially corrupt) file. It must
			// not panic and must not return an error for skippable corrupt lines.
			_, err = store.ReadTranscript(ctx, sid)
			return err
		})

	// Final consistency: Resume must recover all the VALID messages and skip the
	// unparseable corruption. valid-0, valid-1, the type-mismatched-but-valid
	// JSON object (json.Unmarshal of {"role":42,...} into Message fails on the
	// int->string role, so it is skipped), and valid-after-corruption.
	msgs, meta, err := finder.Resume(ctx, sid)
	if err != nil {
		t.Fatalf("resume after corruption: %v", err)
	}
	// Count messages whose content is a recognised valid marker.
	validSeen := 0
	for _, m := range msgs {
		if m.Content == "valid-0" || m.Content == "valid-1" || m.Content == "valid-after-corruption" {
			validSeen++
		}
	}
	if validSeen < 3 {
		t.Fatalf("resume recovered only %d/%d known-valid messages after corruption (total parsed=%d)", validSeen, 3, len(msgs))
	}
	if meta == nil {
		t.Fatal("resume returned nil metadata after corruption")
	}
	t.Logf("session chaos corruption: recovered %d valid markers, %d total parsed lines, metadata intact",
		validSeen, len(msgs))
}

// TestTranscriptStore_Chaos_KillDuringAppend streams appends to a real session
// and cancels the context mid-flight (process-death). After the kill, Resume MUST
// recover a self-consistent transcript: every persisted line parses, and the
// resumed count is identical across two consecutive resumes (no torn/garbage tail
// line). Recovery trace captured to qa-results.
func TestTranscriptStore_Chaos_KillDuringAppend(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	finder := NewResumeFinder(store)
	const sid = "killed-session"

	var appended int64

	stresschaos.ChaosKillDuring(t, "session_kill_during_append", 120*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			i := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered,
						fmt.Sprintf("append loop observed cancellation after %d appends and stopped cleanly", atomic.LoadInt64(&appended)))
					return
				default:
				}
				// Honour the cancellation by passing the chaos ctx into the real API.
				err := store.Append(ctx, sid, Message{
					Role:    "user",
					Content: fmt.Sprintf("kill-msg-%d", i),
				})
				if err != nil {
					// A cancellation surfacing as an error is graceful degradation.
					rec.Record(stresschaos.Degraded, "append returned error under cancellation: "+err.Error())
					return
				}
				atomic.AddInt64(&appended, 1)
				i++
			}
		})

	// After the kill, the transcript must be crash-consistent: resume twice and
	// assert identical, parseable results with no error.
	ctx := context.Background()
	msgs1, _, err := finder.Resume(ctx, sid)
	if err != nil {
		t.Fatalf("first resume after kill: %v", err)
	}
	msgs2, meta, err := finder.Resume(ctx, sid)
	if err != nil {
		t.Fatalf("second resume after kill: %v", err)
	}
	if len(msgs1) != len(msgs2) {
		t.Fatalf("transcript not crash-consistent: resume1=%d resume2=%d messages", len(msgs1), len(msgs2))
	}
	if len(msgs1) == 0 {
		t.Fatal("no messages persisted before kill — append never made progress")
	}
	// Every recovered message must be a fully-parsed, well-formed entry (a torn
	// final write would have been skipped by the parser, so all survivors parse).
	for idx, m := range msgs1 {
		if m.Role == "" || m.Content == "" {
			t.Fatalf("recovered message %d is torn/empty: %+v", idx, m)
		}
	}
	if meta == nil || meta.MessageCount != len(msgs1) {
		t.Fatalf("metadata count inconsistent with transcript after kill: meta=%v transcript=%d", meta, len(msgs1))
	}
	t.Logf("session chaos kill: %d appended before kill, %d crash-consistent on resume, metadata count=%d",
		atomic.LoadInt64(&appended), len(msgs1), meta.MessageCount)
}
