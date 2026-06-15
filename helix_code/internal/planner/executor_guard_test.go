package planner

// Standing regression guards (§11.4.135) for two reproduced defects in the
// SequentialExecutor, each with §11.4.115 RED_MODE polarity:
//
//   RED_MODE=1  reproduces the historical defect on a faithful pre-fix
//               stand-in and asserts the WRONG (defective) behaviour — this
//               is the captured proof the guard exercises a real bug.
//   RED_MODE=0  (default) drives the REAL fixed code and asserts the defect
//               is ABSENT.
//
// Defect 1 (DEF-PLANNER-CTXCANCEL): ExecuteStep's retry loop ignored
//   parent-context cancellation — a cancelled/expired parent ctx burned
//   through MaxRetries with exponential time.Sleep backoff (6 calls / ~31 s
//   observed) instead of aborting immediately.
//
// Defect 2 (DEF-PLANNER-UTF8TRUNC): sanitizeOutput truncated step output on
//   a byte index (output[:maxLen]), splitting multi-byte UTF-8 runes and
//   producing invalid UTF-8 that json.Marshal silently rewrites to U+FFFD.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"unicode/utf8"
)

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// preFixSanitizeOutput is the faithful pre-fix algorithm — a plain byte-index
// slice. Inlined so RED_MODE=1 can reproduce the historical corruption.
func preFixSanitizeOutput(output string, maxLen int) string {
	if len(output) > maxLen {
		output = output[:maxLen]
	}
	return output
}

// TestGuard_SanitizeOutput_UTF8Boundary is the standing regression guard for
// DEF-PLANNER-UTF8TRUNC.
func TestGuard_SanitizeOutput_UTF8Boundary(t *testing.T) {
	// "héllo": 'é' = 0xC3 0xA9 (2 bytes). Truncating to 2 bytes lands
	// mid-rune ("h" + 0xC3), the canonical multi-byte split.
	const in = "héllo"
	const maxLen = 2

	if redMode() {
		// Reproduce the defect on the pre-fix stand-in: invalid UTF-8.
		got := preFixSanitizeOutput(in, maxLen)
		if utf8.ValidString(got) {
			t.Fatalf("RED_MODE: expected pre-fix algorithm to produce INVALID UTF-8 for %q[:%d], got valid %q", in, maxLen, got)
		}
		t.Logf("RED_MODE reproduced defect: pre-fix output %q is invalid UTF-8", got)
		return
	}

	// GREEN: the real fixed code must never emit invalid UTF-8.
	got := sanitizeOutput(in, maxLen)
	if !utf8.ValidString(got) {
		t.Fatalf("sanitizeOutput(%q, %d) = %q which is INVALID UTF-8 (defect DEF-PLANNER-UTF8TRUNC reintroduced)", in, maxLen, got)
	}
	// It must drop the partial rune entirely, yielding "h".
	if got != "h" {
		t.Fatalf("sanitizeOutput(%q, %d) = %q, want %q (rune-boundary truncation)", in, maxLen, got, "h")
	}
}

// TestGuard_SanitizeOutput_PureASCII confirms the rune-safe fix did not
// regress the common ASCII path (still truncates to exactly maxLen bytes).
func TestGuard_SanitizeOutput_PureASCII(t *testing.T) {
	if redMode() {
		t.Skip("RED_MODE: ASCII path is not the reproduced defect") // SKIP-OK: polarity guard, ASCII never broke
	}
	got := sanitizeOutput("abcdef", 3)
	if got != "abc" {
		t.Fatalf("sanitizeOutput(\"abcdef\", 3) = %q, want \"abc\"", got)
	}
}

// preFixExecuteStep is the faithful pre-fix retry loop: no parent-ctx guard,
// blocking time.Sleep backoff. Inlined so RED_MODE=1 can reproduce the
// retry-storm against a cancelled context without time-travel.
//
// It uses a 1ms backoff base (instead of the production 1s) so the RED
// reproduction completes quickly while still proving the loop does NOT abort
// on a cancelled parent context.
func preFixExecuteStep(ctx context.Context, runner ShellRunner, step *TaskStep) {
	for attempt := 0; attempt <= step.MaxRetries; attempt++ {
		if attempt > 0 {
			step.RetryCount = attempt
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Millisecond)
		}
		stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
		_, _ = runner(stepCtx, step.Command)
		cancel()
	}
}

// TestGuard_ExecuteStep_AbortsOnCancelledParentCtx is the standing regression
// guard for DEF-PLANNER-CTXCANCEL.
func TestGuard_ExecuteStep_AbortsOnCancelledParentCtx(t *testing.T) {
	const maxRetries = 5

	if redMode() {
		// Reproduce on the pre-fix stand-in: the loop keeps invoking the
		// runner despite an already-cancelled parent ctx.
		calls := 0
		runner := func(_ context.Context, _ string) (string, error) {
			calls++
			return "", errors.New("always fail")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancelled before execution
		step := &TaskStep{Type: StepShell, Command: "fail", Status: StepPending, MaxRetries: maxRetries, Timeout: time.Second}
		preFixExecuteStep(ctx, runner, step)
		if calls <= 1 {
			t.Fatalf("RED_MODE: expected pre-fix loop to keep retrying despite cancelled ctx, got calls=%d", calls)
		}
		t.Logf("RED_MODE reproduced defect: pre-fix loop called runner %d times on a cancelled ctx", calls)
		return
	}

	// GREEN: the real fixed ExecuteStep must abort immediately — exactly
	// zero (or at most one) runner invocations and no exponential sleeps.
	calls := 0
	runner := func(_ context.Context, _ string) (string, error) {
		calls++
		return "", errors.New("always fail")
	}
	executor := NewSequentialExecutor(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	step := &TaskStep{Type: StepShell, Command: "fail", Status: StepPending, MaxRetries: maxRetries, Timeout: time.Second}

	start := time.Now()
	err := executor.ExecuteStep(ctx, step)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ExecuteStep on a cancelled parent ctx must return an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteStep error = %v, want context.Canceled", err)
	}
	// A pre-cancelled ctx must abort BEFORE the first runner call: the runtime
	// signature of the fix is calls == 0. `!= 0` (not `> 1`) so a partial revert
	// that drops only the top-of-loop ctx check (leaving calls == 1) is caught.
	if calls != 0 {
		t.Fatalf("ExecuteStep invoked the runner %d times on a cancelled ctx (defect DEF-PLANNER-CTXCANCEL reintroduced); want 0", calls)
	}
	// No exponential backoff sleeps may have run. The production base is 1s;
	// a correct abort returns well under that.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("ExecuteStep took %v on a cancelled ctx — exponential backoff was not skipped (defect reintroduced)", elapsed)
	}
	if step.Status != StepFailed {
		t.Fatalf("step.Status = %v, want StepFailed after cancellation", step.Status)
	}
}

// TestGuard_ExecuteStep_AbortsOnCancelDuringBackoff proves the backoff sleep
// itself is context-aware: a parent ctx cancelled WHILE the executor is
// sleeping between retries must abort the sleep, not block for the full
// interval.
func TestGuard_ExecuteStep_AbortsOnCancelDuringBackoff(t *testing.T) {
	if redMode() {
		t.Skip("RED_MODE: covered by the cancelled-before-exec reproduction above") // SKIP-OK: same defect class
	}

	attempts := 0
	runner := func(_ context.Context, _ string) (string, error) {
		attempts++
		return "", errors.New("transient")
	}
	executor := NewSequentialExecutor(runner)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel shortly after the first attempt fails and the executor enters
	// the (production 1s) backoff sleep.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	step := &TaskStep{Type: StepShell, Command: "fail", Status: StepPending, MaxRetries: 5, Timeout: time.Second}
	start := time.Now()
	err := executor.ExecuteStep(ctx, step)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteStep error = %v, want context.Canceled", err)
	}
	// Must abort during the first 1s backoff — well under the full
	// 1+2+4+8+16=31s a non-context-aware loop would burn.
	if elapsed > 2*time.Second {
		t.Fatalf("ExecuteStep took %v — backoff sleep was not context-aware (defect reintroduced)", elapsed)
	}
}
