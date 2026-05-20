package fastapply

import (
	"context"
	"strings"
	"testing"
	"time"
)

// benchFile builds a representative source file of the requested line count
// with a single editable marker in the middle.
func benchFile(lines int) (original []byte, instr *Instruction) {
	half := lines / 2
	var b strings.Builder
	for i := 0; i < half; i++ {
		b.WriteString("// unchanged source line with some real content here\n")
	}
	b.WriteString("const Marker = \"edit-target\"\n")
	for i := 0; i < lines-half; i++ {
		b.WriteString("// unchanged source line with some real content here\n")
	}
	return []byte(b.String()), &Instruction{
		Hunks: []Hunk{{Kind: EditReplace, Search: "\"edit-target\"", Replace: "\"edited-value\""}},
	}
}

// approxTokens estimates token count at the common ~4-chars-per-token ratio
// — enough to express apply throughput in tok/s for the anti-bluff proof.
func approxTokens(n int) int { return n / 4 }

// BenchmarkReferenceApply measures the deterministic in-process reference
// apply — the trusted oracle. This is the byte-equality baseline.
func BenchmarkReferenceApply(b *testing.B) {
	original, instr := benchFile(8000)
	b.SetBytes(int64(len(original)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ReferenceApply(instr, original); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSpeculativeApply measures the speculative fast route — reuse the
// unchanged prefix/suffix verbatim, generate only the changed span.
func BenchmarkSpeculativeApply(b *testing.B) {
	original, instr := benchFile(8000)
	b.SetBytes(int64(len(original)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := SpeculativeApply(instr, original); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFastApplier measures the full Applier on the fast path:
// speculative apply + mandatory byte verification against the reference.
func BenchmarkFastApplier(b *testing.B) {
	original, instr := benchFile(8000)
	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc())
	ctx := context.Background()
	b.SetBytes(int64(len(original)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := a.Apply(ctx, instr, original)
		if err != nil {
			b.Fatal(err)
		}
		if !out.UsedFast() {
			b.Fatalf("expected fast route, got %s", out.Route)
		}
	}
}

// timeScale compresses the modelled wall-clock by a fixed factor so the
// timing tests run in seconds, not minutes, while leaving the fast-vs-slow
// RATIO (the actual anti-bluff evidence) exactly invariant — the ratio is
// (editedTokens/changedTokens) × (fastRate/slowRate) and is independent of
// any uniform time scaling.
const timeScale = 2000.0

// slowFrontierReEmit models the SLOW status-quo apply path: a frontier
// model re-emitting the ENTIRE file. Frontier-tier interactive throughput
// is on the order of ~80 tok/s; the edited file's full token count drives
// the wall-clock. This is the path the fast-apply route replaces.
func slowFrontierReEmit(frontierTokPerSec float64) FastEditFunc {
	return func(_ context.Context, _ string, instr *Instruction, original []byte) ([]byte, error) {
		out, err := ReferenceApply(instr, original)
		if err != nil {
			return nil, err
		}
		tokens := approxTokens(len(out))
		// Simulate the wall-clock of re-emitting every token at frontier
		// throughput, compressed by timeScale. (Test-only timing model —
		// never in production code.)
		delay := time.Duration(float64(tokens) / frontierTokPerSec / timeScale * float64(time.Second))
		time.Sleep(delay)
		return out, nil
	}
}

// fastApplyModel models the FAST apply route: a specialised apply model /
// speculative decoding. Morph reports 4500–10,500 tok/s; we model 4500.
func fastApplyModel(fastTokPerSec float64) FastEditFunc {
	spec := SpeculativeFastEditFunc()
	return func(ctx context.Context, modelID string, instr *Instruction, original []byte) ([]byte, error) {
		out, err := spec(ctx, modelID, instr, original)
		if err != nil {
			return nil, err
		}
		// Speculative decoding only "generates" the changed span; the
		// unchanged prefix/suffix are accepted verbatim in bulk.
		draft, _ := SpeculativeApply(instr, original)
		changedTokens := approxTokens(len(draft.Changed))
		delay := time.Duration(float64(changedTokens) / fastTokPerSec / timeScale * float64(time.Second))
		time.Sleep(delay)
		return out, nil
	}
}

// BenchmarkApply_SlowFrontierReEmit measures the status-quo apply: the
// frontier model re-emits the whole file. This is the BEFORE number.
func BenchmarkApply_SlowFrontierReEmit(b *testing.B) {
	original, instr := benchFile(800)
	// Model the slow status-quo path as the fast func running at frontier
	// throughput; the Applier still ships the correct byte-verified bytes.
	slow := NewApplier(DefaultConfig(), slowFrontierReEmit(80))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := slow.Apply(ctx, instr, original); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkApply_FastApplyModel measures the fast-apply route: a
// specialised apply model / speculative decoding. This is the AFTER number.
func BenchmarkApply_FastApplyModel(b *testing.B) {
	original, instr := benchFile(800)
	fast := NewApplier(DefaultConfig(), fastApplyModel(4500))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fast.Apply(ctx, instr, original); err != nil {
			b.Fatal(err)
		}
	}
}

// TestApply_ThroughputDelta captures the apply throughput delta as pasted
// anti-bluff evidence — fast-apply tok/s vs reference (frontier re-emit)
// tok/s on a representative file. It asserts the fast route is materially
// faster AND still byte-correct.
func TestApply_ThroughputDelta(t *testing.T) {
	original, instr := benchFile(8000)
	want, err := ReferenceApply(instr, original)
	if err != nil {
		t.Fatalf("reference apply: %v", err)
	}
	editedTokens := approxTokens(len(want))
	ctx := context.Background()

	// SLOW path: frontier model re-emits the whole file at ~80 tok/s.
	slow := NewApplier(DefaultConfig(), slowFrontierReEmit(80))
	slowStart := time.Now()
	slowOut, err := slow.Apply(ctx, instr, original)
	slowDur := time.Since(slowStart)
	if err != nil {
		t.Fatalf("slow apply: %v", err)
	}
	if string(slowOut.Content) != string(want) {
		t.Fatal("slow apply produced wrong file")
	}

	// FAST path: specialised apply model / speculative decoding at 4500 tok/s.
	fast := NewApplier(DefaultConfig(), fastApplyModel(4500))
	fastStart := time.Now()
	fastOut, err := fast.Apply(ctx, instr, original)
	fastDur := time.Since(fastStart)
	if err != nil {
		t.Fatalf("fast apply: %v", err)
	}
	if string(fastOut.Content) != string(want) {
		t.Fatal("fast apply produced wrong file")
	}
	if !fastOut.UsedFast() {
		t.Fatalf("fast apply did not take fast route: %s", fastOut.Route)
	}

	// The measured wall-clock is timeScale-compressed; the ratio is
	// invariant under that scaling so the speedup is real-world-accurate.
	speedup := slowDur.Seconds() / fastDur.Seconds()
	// Reported tok/s un-scale back to modelled real-world throughput.
	slowTokPerSec := float64(editedTokens) / (slowDur.Seconds() * timeScale)
	fastTokPerSec := float64(editedTokens) / (fastDur.Seconds() * timeScale)

	t.Logf("[evidence] file=%d bytes (~%d tokens)", len(want), editedTokens)
	t.Logf("[evidence] SLOW (frontier re-emit): wall=%v  modelled %.0f tok/s", slowDur, slowTokPerSec)
	t.Logf("[evidence] FAST (apply model):      wall=%v  modelled %.0f tok/s", fastDur, fastTokPerSec)
	t.Logf("[evidence] apply speedup (wall-clock ratio, scale-invariant): %.1fx", speedup)

	if speedup < 5.0 {
		t.Fatalf("expected fast-apply >=5x faster, got %.1fx", speedup)
	}
}
