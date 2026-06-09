# HXC-059 — debate_orchestrator sandbox: non-Linux process-tree kill (§11.4.81)
**Captured:** 2026-06-09T16:13:17Z · Bug · Fixed (→ Fixed.md)
## RED (pre-fix, macOS)
go test ./testing: TestSandboxExecute_CtxCancel FAIL elapsed=30.018s; TestSandboxExecute_TimeoutEnforced FAIL elapsed=30.026s
(no-op killProcessGroup → Setpgid never set → only direct child SIGKILLed, sleep-30 grandchild survives).
## Fix
testing/sandbox_other.go (!linux): prepareSandboxAttr sets SysProcAttr.Setpgid=true; killProcessGroup → syscall.Kill(-pid, SIGKILL) — mirrors sandbox_linux.go (macOS/BSD support POSIX process groups). RLIMIT_AS/CPU left as honest §11.4.81 XNU gap.
## GREEN
both tests PASS 5/5 at -count=5, 0.10s each (timeout now enforced); full go test ./testing ok 0.790s no regression.
Commit c82af2f, pushed ff to all remotes.
