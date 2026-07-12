# HXC-141 — QA evidence (§11.4.83)

**Item:** HXC-141 (Bug / Medium) — MCP module's Docker adapter crashes with a null-pointer error when asked to stop a container that was never started or does not exist
**Submodule:** `submodules/mcp_module` (module `digital.vasic.mcp`, go 1.24)
**Files:** `pkg/adapter/adapter.go` (fix) + `pkg/adapter/adapter_test.go` (§11.4.120 reconciliation + hermetic matcher test)
**Date (UTC):** 2026-07-12T11:18:49Z
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-reproduce-first, §11.4.120 fix-breaks-its-own-gate reconciliation, §11.4.135 standing guard, §11.4.142 independent review, §11.4.28 decoupling.

## Root cause (Phase 1 — FACT, reproduced)

Two facts established by running the actual code on the host:

1. The host's `docker` binary is a **podman shim** ("Emulate Docker CLI using podman"), where
   `docker rm -f <missing-container>` exits **0** (success, nil error) — unlike real docker,
   which returns a non-zero "No such container" error.
2. `DockerAdapter.Stop` therefore returned **nil** for a never-started / missing container on
   this host, but the existing `TestDockerAdapter_Stop_ContainerDoesNotExist` asserted a
   NON-nil error and then called `err.Error()` on the (nil) error:

   ```
   --- FAIL: TestDockerAdapter_Stop_NotStarted (0.03s)
   --- FAIL: TestDockerAdapter_Stop_ContainerDoesNotExist (0.03s)
   panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
   FAIL	digital.vasic.mcp/pkg/adapter	0.075s
   ```

   The `err.Error()` on a nil `error` interface IS the reported "null-pointer error when asked to
   stop a container that was never started or does not exist."

The defect is twofold: `DockerAdapter.Stop` was not runtime-independently graceful (real docker
errors on missing containers), and the tests encoded real-docker semantics + unsafely dereferenced
a possibly-nil error.

## Fix

1. `pkg/adapter/adapter.go` — `DockerAdapter.Stop` made an **idempotent safe no-op**:
   - never-started guard: `if a.containerID == "" { SetState(StateStopped); return nil }` — no
     shell-out at all, runtime-independent (Start sets `containerID` only on success, so `""`
     ⟺ never-successfully-started);
   - a container that no longer exists is a benign no-op — `isNoSuchContainer(output)` (new,
     case-insensitive "no such container" matcher) → `nil` + StateStopped (real-docker graceful);
   - a genuine removal failure still surfaces as an error (+ StateError).
2. `pkg/adapter/adapter_test.go` — §11.4.120 reconciliation of the 3 tests that asserted the old
   buggy behavior, none fake-passed, none reverted:
   - `TestDockerAdapter_Stop_NotStarted` → assert `NoError` + `StateStopped` (stop-before-start no-op);
   - `TestDockerAdapter_Stop_ContainerDoesNotExist` → set `containerID` to reach the docker path,
     assert `NoError` + `StateStopped`, and **remove the unsafe `err.Error()` deref** (the panic site);
   - `TestDockerAdapter_Stop_ContextTimeout` → set `containerID` to reach the exec-with-expired-context
     path, still asserts `Error` + `StateError` (context handling preserved — genuine failures surface);
   - new hermetic `TestDockerAdapter_isNoSuchContainer` (5 subcases) — matcher tested without a runtime.

## Captured verification (post-fix, mcp_module)

```
[GREEN] go test -run 'TestDockerAdapter_(Stop_(NotStarted|ContainerDoesNotExist|ContextTimeout)|isNoSuchContainer)'
  --- PASS: TestDockerAdapter_Stop_NotStarted
  --- PASS: TestDockerAdapter_isNoSuchContainer  (docker-error, docker-cli-error, lowercase, unrelated-error, empty)
  --- PASS: TestDockerAdapter_Stop_ContainerDoesNotExist
  --- PASS: TestDockerAdapter_Stop_ContextTimeout
  ok  digital.vasic.mcp/pkg/adapter

go build ./...  -> exit 0
go vet   ./...  -> exit 0
go test -count=1 ./pkg/...  -> ok: adapter, client, config, i18n, protocol, registry, server (0 fail)
```

## RED→GREEN (§11.4.115)

- **RED (pre-fix, broken artifact):** the original tests PANIC with `nil pointer dereference` on the
  podman host (captured above) — the exact HXC-141 crash.
- **GREEN (post-fix):** the reconciled + new tests pass; the module builds + vets clean.

## Honest boundary (§11.4.3 / §11.4.6)

The never-started guard's real-docker benefit (graceful where real docker would error) cannot be
reproduced on this podman-as-docker host; the hermetic `isNoSuchContainer` matcher test + the
guard's no-shell-out property + the podman-host graceful behavior together cover the contract, and
the guard is deterministic + runtime-independent by construction. The "no such container" docker
error branch is covered by the hermetic matcher test, not a live real-docker run.

## Remediation (§11.4.134 review round 2)

Round-1 independent review returned GO with one non-blocking nit: `Stop` read `containerID`
unguarded (a concurrent Start/Stop data-race surface, since `Start`'s write was also unguarded).
Rather than defer, it was fixed to a clean GO (§11.4.134 iterate-until-clean + §11.4.145 angle-3
concurrency + §11.4.1 no-new-failure-modes):

- `adapter.go`: added `setContainerID`/`getContainerID` accessors guarding `containerID` with the
  adapter's existing `BaseAdapter.mu`; `Start` writes via the setter, `Stop`'s never-started guard
  reads via the getter (no lock re-entrancy — the accessors and `SetState` lock `a.mu` separately,
  never nested).
- `adapter_test.go`: the two white-box tests set `containerID` via `setContainerID`; new
  `TestDockerAdapter_containerID_ConcurrentAccess` hammers the accessors concurrently and is
  `-race`-clean (and FAILs under `-race` if the guard is reverted — the §1.1 proof).

Re-verified: `go build ./...` = 0, `go vet ./...` = 0, all 5 HXC-141 tests PASS under `-race`
(no DATA RACE), full `go test -race ./pkg/adapter/` = ok.

## Decoupling (§11.4.28)

The fix is pure internal robustness in `digital.vasic.mcp` — no consuming-project context injected.
