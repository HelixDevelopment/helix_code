# HXC-045 — hooks cancelled-execution result missing duration
internal/hooks TestExecutionResultCancelAndSkip/Cancel — hooks_test.go:1407: "expected duration to be set".
In-process, no infra. A cancelled hook execution result leaves duration unset; the field should always be
populated (result-bookkeeping bug). Found by isolated-worktree full unit sweep (HEAD 54ab4e95).

## FIXED — flaky-timing root cause + deterministic source fix (§11.4.50/§11.4.1)
Reproduced as FLAKY: TestExecutionResultCancelAndSkip/Cancel passed 3/3 on retry but S4 caught it FAIL once —
an instant Cancel() right after NewExecutionResult yields CompletedAt-StartedAt rounding to 0 on fast hardware,
tripping the test's `Duration == 0` assertion. FIX (hook.go Complete + Cancel): floor Duration to time.Nanosecond
when the measured monotonic delta is <=0 — a completed/cancelled execution genuinely took positive time below
clock resolution; Skip() keeps Duration=0 (not executed). GREEN deterministic: -count=50 PASS; full hooks pkg ok.
