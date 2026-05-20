package main

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// lazy_startup_test.go — speed-programme Phase 1 task P1-T03.
//
// P1-T03 converted the eager subsystem monolith in Run() (R1 B01/B13/B14/B18)
// into sync.Once-guarded lazy getters: telemetry(), ensureLLMProvider() and
// ensureSubsystems(). These tests prove the two invariants the conversion
// MUST hold:
//
//   1. Laziness — a getter constructs its subsystem EXACTLY ONCE; a second
//      call returns without re-running the constructor (sync.Once contract).
//   2. On-demand — a command path that needs subsystem X triggers exactly one
//      construction of X; a command path that does NOT need X triggers zero.
//
// The CLI exposes the construction-probe counter (ConstructionCount) precisely
// so these assertions are mechanical and falsifiable (CONST-035 anti-bluff —
// "lazy" without a counted before/after is a bluff).
//
// No mocks: every getter is exercised against the real telemetry / LLM /
// subsystem code paths. telemetry() defaults to a real OTel noop provider
// (no OTEL_* env vars set in the test process); ensureLLMProvider() exercises
// the real F12 selector + real provider construction; ensureSubsystems()
// builds the real tool registry + real F07-F30 wiring. Mocks would only be
// permitted here because this is a unit-test file (CONST-050(A)) — but none
// are needed, so none are used.

// newTestCLI returns a fresh *CLI as the process would (NewCLI) but is safe to
// call repeatedly in a single test process. NewCLI calls config.Load() which
// touches the process-global Viper singleton; that is single-goroutine-safe so
// serial construction in tests is fine.
func newTestCLI(t *testing.T) *CLI {
	t.Helper()
	restore := redirectStdout(t)
	defer restore()
	c := NewCLI()
	if c == nil {
		t.Fatal("NewCLI returned nil")
	}
	return c
}

// redirectStdout silences stdout for the duration of a test — NewCLI prints a
// config-file notice via fmt.Println. Test-process I/O concern only (mirrors
// silenceStdout in bench_test.go, which takes a *testing.B).
func redirectStdout(t *testing.T) func() {
	t.Helper()
	orig := os.Stdout
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	os.Stdout = devnull
	return func() {
		os.Stdout = orig
		_ = devnull.Close()
	}
}

// --- Unit: telemetry() getter is lazy + constructs exactly once -------------

func TestP1T03_Telemetry_ConstructsExactlyOnce(t *testing.T) {
	c := newTestCLI(t)
	if got := c.ConstructionCount("telemetry"); got != 0 {
		t.Fatalf("telemetry constructed before first access: count=%d, want 0", got)
	}

	first := c.telemetry()
	if first == nil {
		t.Fatal("telemetry() returned nil provider")
	}
	if got := c.ConstructionCount("telemetry"); got != 1 {
		t.Fatalf("after first telemetry() call: count=%d, want 1", got)
	}

	// Subsequent calls MUST return the SAME instance and NOT re-construct.
	for i := 0; i < 5; i++ {
		again := c.telemetry()
		if again != first {
			t.Fatalf("telemetry() call %d returned a different instance — sync.Once violated", i+2)
		}
	}
	if got := c.ConstructionCount("telemetry"); got != 1 {
		t.Fatalf("after 6 telemetry() calls: count=%d, want 1 (constructor must run once)", got)
	}
}

// --- Unit: ensureLLMProvider() getter is lazy + constructs exactly once -----

func TestP1T03_EnsureLLMProvider_ConstructsExactlyOnce(t *testing.T) {
	c := newTestCLI(t)
	if got := c.ConstructionCount("llmProvider"); got != 0 {
		t.Fatalf("llmProvider constructed before first access: count=%d, want 0", got)
	}

	ctx := context.Background()
	if err := c.ensureLLMProvider(ctx, ""); err != nil {
		t.Fatalf("ensureLLMProvider returned unexpected error: %v", err)
	}
	if c.llmProvider == nil {
		t.Fatal("ensureLLMProvider left c.llmProvider nil")
	}
	if got := c.ConstructionCount("llmProvider"); got != 1 {
		t.Fatalf("after first ensureLLMProvider: count=%d, want 1", got)
	}
	settled := c.llmProvider

	// Repeated calls must be no-ops — same provider, no re-construction.
	for i := 0; i < 5; i++ {
		if err := c.ensureLLMProvider(ctx, ""); err != nil {
			t.Fatalf("ensureLLMProvider repeat call %d errored: %v", i+2, err)
		}
		if c.llmProvider != settled {
			t.Fatalf("ensureLLMProvider repeat call %d re-assigned the provider — sync.Once violated", i+2)
		}
	}
	if got := c.ConstructionCount("llmProvider"); got != 1 {
		t.Fatalf("after 6 ensureLLMProvider calls: count=%d, want 1", got)
	}
}

// --- Unit: an explicitly-unknown --provider value is a fatal error ---------

func TestP1T03_EnsureLLMProvider_UnknownProviderIsFatal(t *testing.T) {
	c := newTestCLI(t)
	err := c.ensureLLMProvider(context.Background(), "definitely-not-a-real-provider")
	if err == nil {
		t.Fatal("ensureLLMProvider accepted an unknown --provider value; want a fatal error")
	}
	// The sync.Once still ran exactly once even though it produced an error.
	if got := c.ConstructionCount("llmProvider"); got != 1 {
		t.Fatalf("llmProvider construction count=%d, want 1 even on error", got)
	}
}

// --- Integration: ensureSubsystems builds the heavy cluster on demand ------
//
// This proves the on-demand invariant: a fresh *CLI has subsystems count==0;
// after ensureSubsystems it is exactly 1; and ensureSubsystems triggered the
// llmProvider getter too (getter-calls-getter ordering — the F22 auto-committer
// and F15 subagent manager capture c.llmProvider at construction time).

func TestP1T03_EnsureSubsystems_BuildsHeavyClusterOnDemand(t *testing.T) {
	c := newTestCLI(t)
	restore := redirectStdout(t)
	defer restore()

	if got := c.ConstructionCount("subsystems"); got != 0 {
		t.Fatalf("subsystems constructed before first access: count=%d, want 0", got)
	}
	if got := c.ConstructionCount("llmProvider"); got != 0 {
		t.Fatalf("llmProvider constructed before subsystems access: count=%d, want 0", got)
	}

	ctx := context.Background()
	if err := c.ensureSubsystems(ctx); err != nil {
		t.Fatalf("ensureSubsystems returned an error: %v", err)
	}
	defer c.runCleanups()

	if got := c.ConstructionCount("subsystems"); got != 1 {
		t.Fatalf("after ensureSubsystems: subsystems count=%d, want 1", got)
	}
	// ensureSubsystems MUST have driven the llmProvider getter (ordering anchor).
	if got := c.ConstructionCount("llmProvider"); got != 1 {
		t.Fatalf("ensureSubsystems did not trigger llmProvider construction: count=%d, want 1", got)
	}
	// The real heavy subsystems must actually be wired onto the struct.
	if c.toolRegistry == nil {
		t.Error("ensureSubsystems left c.toolRegistry nil — tool registry not built")
	}
	if c.commandRegistry == nil {
		t.Error("ensureSubsystems left c.commandRegistry nil — slash-command registry not built")
	}
	if c.sessionMgr == nil {
		t.Error("ensureSubsystems left c.sessionMgr nil — session manager not built")
	}
	if c.mcpManager == nil {
		t.Error("ensureSubsystems left c.mcpManager nil — MCP manager not built")
	}

	// Idempotency: a second call must not re-build (sync.Once).
	if err := c.ensureSubsystems(ctx); err != nil {
		t.Fatalf("second ensureSubsystems call errored: %v", err)
	}
	if got := c.ConstructionCount("subsystems"); got != 1 {
		t.Fatalf("after 2 ensureSubsystems calls: count=%d, want 1 (sync.Once)", got)
	}
}

// --- Integration: a short-command path does NOT build the heavy cluster ----
//
// This is the core P1-T03 win: `--list-models` / `--prompt` resolve the LLM
// provider but the heavy F07-F30 subsystem cluster is never touched. The probe
// counter proves the cluster was skipped.

func TestP1T03_ShortCommandPath_SkipsHeavySubsystems(t *testing.T) {
	c := newTestCLI(t)

	// Simulate the `--list-models` / `--prompt` dispatch arm: it calls
	// ensureLLMProvider and NOTHING else.
	if err := c.ensureLLMProvider(context.Background(), ""); err != nil {
		t.Fatalf("ensureLLMProvider errored: %v", err)
	}

	// The heavy subsystem cluster MUST NOT have been constructed.
	if got := c.ConstructionCount("subsystems"); got != 0 {
		t.Fatalf("short-command path built the heavy subsystem cluster: count=%d, want 0", got)
	}
	// And the heavy struct fields must still be nil — proof nothing wired them.
	if c.toolRegistry != nil {
		t.Error("short-command path constructed the tool registry — heavy cluster leaked")
	}
	if c.commandRegistry != nil {
		t.Error("short-command path constructed the command registry — heavy cluster leaked")
	}
	if c.mcpManager != nil {
		t.Error("short-command path spawned the MCP manager — heavy cluster leaked")
	}
}

// --- Integration: cleanup closures are drained LIFO exactly once -----------

func TestP1T03_RunCleanups_DrainsLIFOExactlyOnce(t *testing.T) {
	c := newTestCLI(t)
	var order []int
	var mu sync.Mutex
	for i := 0; i < 3; i++ {
		i := i
		c.addCleanup(func() {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		})
	}
	c.runCleanups()
	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 0 {
		t.Fatalf("runCleanups drained in %v, want LIFO [2 1 0]", order)
	}
	// Second drain must be a no-op — closures already consumed.
	c.runCleanups()
	if len(order) != 3 {
		t.Fatalf("second runCleanups re-ran closures: order=%v", order)
	}
}

// --- Benchmark: cold-start of the short-command path vs the heavy path -----
//
// BenchmarkColdStartShortCommand measures NewCLI() + ensureLLMProvider() — the
// exact cost a `--list-models` invocation now pays. BenchmarkColdStartFull
// measures NewCLI() + ensureSubsystems() — the cost the OLD eager monolith
// imposed on EVERY command including `--list-models`. The delta between the
// two benchmarks IS the P1-T03 speedup for short commands (CONST-035 Rule 9 —
// the pasted before/after numbers make the claim falsifiable).

// silenceStdErrOut redirects both os.Stdout and os.Stderr to /dev/null for the
// duration of a benchmark. The F12 "no cloud provider" notice is written to
// os.Stderr by ensureLLMProvider; left un-silenced it interleaves into the
// `go test -bench` result line and corrupts the captured numbers.
func silenceStdErrOut(b *testing.B) func() {
	b.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("open %s: %v", os.DevNull, err)
	}
	os.Stdout = devnull
	os.Stderr = devnull
	return func() {
		os.Stdout = origOut
		os.Stderr = origErr
		_ = devnull.Close()
	}
}

func BenchmarkColdStartShortCommand(b *testing.B) {
	restore := silenceStdErrOut(b)
	defer restore()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewCLI()
		if err := c.ensureLLMProvider(context.Background(), ""); err != nil {
			b.Fatalf("ensureLLMProvider: %v", err)
		}
	}
}

func BenchmarkColdStartFull(b *testing.B) {
	restore := silenceStdErrOut(b)
	defer restore()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		c := NewCLI()
		b.StartTimer()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := c.ensureSubsystems(ctx); err != nil {
			cancel()
			b.Fatalf("ensureSubsystems: %v", err)
		}
		c.runCleanups()
		cancel()
	}
}
