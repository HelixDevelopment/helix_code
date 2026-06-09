# HXC-045 — hooks cancelled-execution result missing duration
internal/hooks TestExecutionResultCancelAndSkip/Cancel — hooks_test.go:1407: "expected duration to be set".
In-process, no infra. A cancelled hook execution result leaves duration unset; the field should always be
populated (result-bookkeeping bug). Found by isolated-worktree full unit sweep (HEAD 54ab4e95).
