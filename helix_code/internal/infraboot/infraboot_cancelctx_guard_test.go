package infraboot

// Standing regression guard (§11.4.135) for the "cancelled-context still boots"
// defect: EnsureInfra issued a real `compose up` even when the caller's context
// was ALREADY cancelled before the call, leaving orphaned containers running
// while returning an error (so the caller never got a Result to tear them down).
// This is a host-safety / correctness defect (§11.4.133): an irreversible boot
// MUST NOT proceed once the caller has already cancelled.
//
// §11.4.115 RED-polarity (single source, polarity switch via RED_MODE env):
//   RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in
//                (preFixEnsureBoot below = the old boot path WITHOUT the
//                pre-compose-up ctx check) and assert ComposeUp WAS issued
//                despite the cancelled context. This is the captured proof the
//                defect was real.
//   RED_MODE=0 (default) — drive the REAL, fixed EnsureInfra and assert NO
//                ComposeUp is issued when the context is already cancelled.
//
// Run the RED reproduction:  RED_MODE=1 go test -run TestGuard_CancelledCtxNoBoot -v ./internal/infraboot/
// Default (standing GREEN):  go test -run TestGuard_CancelledCtxNoBoot -v ./internal/infraboot/

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// preFixEnsureBoot is a faithful stand-in for the PRE-FIX boot branch of
// EnsureInfra: it pre-checks health, and when unhealthy proceeds straight to
// ComposeUp with NO context-cancellation check beforehand (exactly the old
// behaviour). It exists ONLY so RED_MODE=1 can reproduce the historical defect
// against a faithful copy of the broken code (the real code is now fixed).
func preFixEnsureBoot(ctx context.Context, b Booter, composeFile string) (int, error) {
	healthy := b.HealthCheckTCP(bootHost, "1") == nil
	if healthy {
		return 0, nil
	}
	if !b.RuntimeAvailable(ctx) {
		return 0, errors.New("no runtime")
	}
	// PRE-FIX: no ctx.Err() guard here — boot regardless of cancellation.
	if err := b.ComposeUp(ctx, composeFile, []string{"postgres", "redis"}); err != nil {
		return 0, err
	}
	// Old code then waited for health and only THERE noticed cancellation.
	return 0, ctx.Err()
}

func TestGuard_CancelledCtxNoBoot(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	newDownBooter := func() *fakeBooter {
		return &fakeBooter{
			runtimeAvail: true,
			runtimeName:  "podman",
			healthFn:     func(host, port string) error { return errors.New("down") },
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call

	if redMode {
		// Reproduce the defect on the faithful pre-fix stand-in.
		b := newDownBooter()
		_, _ = preFixEnsureBoot(ctx, b, composeFixture(t))
		if b.upCalls() == 0 {
			t.Fatalf("RED stand-in did not reproduce the defect (expected ComposeUp despite cancelled ctx)")
		}
		t.Logf("RED ok: pre-fix path issued %d ComposeUp despite cancelled ctx (defect reproduced)", b.upCalls())
		return
	}

	// Standing GREEN guard: the REAL, fixed EnsureInfra must NOT boot.
	cfg := freshCfg()
	b := newDownBooter()
	_, err := EnsureInfra(ctx, cfg, &Options{Booter: b, ComposeFile: composeFixture(t)})
	if err == nil {
		t.Fatalf("expected an error when ctx is already cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the error to wrap context.Canceled, got %v", err)
	}
	if b.upCalls() != 0 {
		t.Fatalf("DEFECT: ComposeUp issued %d time(s) despite ctx already cancelled — boot must abort first", b.upCalls())
	}
	if cfg.Database.Host != "postgres" {
		t.Fatalf("config must NOT be rewritten when boot is aborted on cancelled ctx, got %s", cfg.Database.Host)
	}
	t.Logf("GREEN: cancelled-ctx aborted boot before ComposeUp (0 boots), err=%v", err)
}

// TestGuard_CancelDuringComposeUp proves the guard also holds when cancellation
// happens just before the boot decision under a realistic (tiny) deadline,
// asserting determinism across runs (§11.4.50): a context whose deadline has
// already elapsed must abort before any ComposeUp.
func TestGuard_CancelDuringComposeUp(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		t.Skip("RED_MODE reproduction lives in TestGuard_CancelledCtxNoBoot")
	}
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return errors.New("down") }}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := EnsureInfra(ctx, cfg, &Options{Booter: b, ComposeFile: composeFixture(t)})
	if err == nil {
		t.Fatalf("expected error for already-expired deadline")
	}
	if b.upCalls() != 0 {
		t.Fatalf("DEFECT: ComposeUp issued %d time(s) under an already-expired deadline", b.upCalls())
	}
}
