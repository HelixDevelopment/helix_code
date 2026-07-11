# HXC-118 Phase 1 — internal/rag adapter — evidence

Status: DONE (Phase 1 scope only — no request-flow wiring, per instructions)

## Files created / modified

- Created: `helix_code/internal/rag/adapter.go` (production adapter)
- Created: `helix_code/internal/rag/adapter_test.go` (unit test, fake retriever per §11.4.27)
- Modified: `helix_code/go.mod` — exactly one line added (`require digital.vasic.rag v0.0.0-00010101000000-000000000000`)
- `helix_code/go.sum` — untouched (0 diff; local `replace` targets need no checksum, confirmed no other digital.vasic.* module has a go.sum entry either)
- `submodules/rag/` — untouched (confirmed via `git -C submodules/rag status --porcelain` → empty)

## Dependency-resolution mechanism used (discovered, then reused verbatim)

Grepped `helix_code/go.mod` for `digital.vasic` + `replace` first. Found that
`replace digital.vasic.rag => ../submodules/rag` **already existed** at line 290,
pre-provisioned exactly like ~15 other `digital.vasic.*` replace directives
(agentic, auth, background, storage, etc.) that have a replace line but no
`require` line yet — i.e. "reserved but not yet imported," the same state
`digital.vasic.rag` was in before this task. Only 8 `digital.vasic.*` modules
(concurrency, containers, debate, helixmemory, helixqa, helixspecifier, lazy,
memory) plus `challenges` (indirect) had an actual `require` entry driving real
imports.

Mechanism = add exactly one `require` line using the same placeholder
pseudo-version most sibling entries use (`v0.0.0-00010101000000-000000000000`),
next to the other `digital.vasic.*` entries in the require block. No `go.work`
file exists in the repo (confirmed via `find . -maxdepth 2 -iname "go.work*"` →
empty) — `replace` in the inner module's own `go.mod` is the established,
sole mechanism for sibling-submodule consumption. Zero go.sum change needed
(confirmed: `grep -c digital.vasic helix_code/go.sum` → 0 before and after).

This is the lowest-risk possible change: reusing an already-existing,
already-working pattern for 8+ other submodules, adding a single line.

## RED -> GREEN (§11.4.115)

RED (round 1 — package didn't exist, go.mod not yet updated):
```
$ go test -tags=nogui ./internal/rag/...
go: updates to go.mod needed; to update it:
	go mod tidy
```

RED (round 2 — go.mod fixed, adapter.go still missing):
```
$ go test -tags=nogui ./internal/rag/...
# dev.helix.code/internal/rag [dev.helix.code/internal/rag.test]
internal/rag/adapter_test.go:32:7: undefined: NewAdapter
internal/rag/adapter_test.go:56:7: undefined: NewAdapter
internal/rag/adapter_test.go:77:5: invalid operation: fake.calledOpts != opts (struct containing map[string]any cannot be compared)
internal/rag/adapter_test.go:88:7: undefined: NewAdapter
internal/rag/adapter_test.go:104:7: undefined: NewAdapter
internal/rag/adapter_test.go:114:21: undefined: ErrNoRetriever
internal/rag/adapter_test.go:124:7: undefined: NewAdapter
FAIL	dev.helix.code/internal/rag [build failed]
FAIL
```
(Also caught and fixed a genuine test bug found during RED: `retriever.Options`
contains a `map[string]any` field, so `!=` comparison doesn't compile in Go —
switched to `reflect.DeepEqual`.)

GREEN:
```
$ go build -tags=nogui ./internal/rag/...
exit=0

$ go test -tags=nogui -v -count=1 ./internal/rag/...
=== RUN   TestAdapter_DefaultOff
--- PASS: TestAdapter_DefaultOff (0.00s)
=== RUN   TestAdapter_EnabledDelegatesToRetriever
--- PASS: TestAdapter_EnabledDelegatesToRetriever (0.00s)
=== RUN   TestAdapter_EnabledPropagatesRetrieverError
--- PASS: TestAdapter_EnabledPropagatesRetrieverError (0.00s)
=== RUN   TestAdapter_EnabledButNoRetrieverConfigured
--- PASS: TestAdapter_EnabledButNoRetrieverConfigured (0.00s)
=== RUN   TestAdapter_SetEnabledToggle
--- PASS: TestAdapter_SetEnabledToggle (0.00s)
PASS
ok  	dev.helix.code/internal/rag	0.002s
```

## Full-app build check (exit code must be 0)

```
$ go build -tags=nogui ./... 2>&1 | tail -30
BUILD_EXIT=0
```
No output (no errors) and exit 0.

## Self-review checklist

- [x] `submodules/rag/` untouched — confirmed empty `git -C submodules/rag status --porcelain` and empty `git diff --stat submodules/rag`.
- [x] Only `helix_code/go.mod` (+1 line) and `helix_code/internal/rag/*.go` (new) changed — confirmed via `git status --porcelain=v1`.
- [x] `helix_code/cmd/cli`, `helix_code/internal/verifier`, `helix_code/internal/acp` untouched — confirmed empty `git status --porcelain=v1` on those paths.
- [x] Adapter is default-OFF: `NewAdapter(r)` always starts with `enabled=false`; `Enabled()` returns false until `SetEnabled(true)` is called; a disabled `Retrieve` call never touches the underlying `retriever.Retriever` (asserted via `fake.called` in `TestAdapter_DefaultOff`).
- [x] No request-flow wiring: nothing in `cmd/cli` or any handler calls `internal/rag` — this is Phase 2, explicitly out of scope here.
- [x] Real delegation, no fabricated results: enabled adapter calls the real injected `retriever.Retriever.Retrieve` and returns its exact documents/errors unmodified (asserted via query/opts/docs/error equality in the enabled-path tests).
- [x] Anti-bluff grep on `internal/rag/` clean (no "simulated"/"for now"/"TODO implement"/"in production this would").
- [x] No `git add`/`commit` performed. No `--force` used. Nothing edited outside declared scope.

## Design doc alignment

Matches `docs/research/const040_capability_model_20260712/DESIGN.md` §3.2 item 1
("Add `digital.vasic.rag` as a `go.mod` require ... with a `replace` directive")
and §3.4 "New package: `internal/rag/`" — Phase 1 delivers exactly the adapter
package; the concrete storage-backed `retriever.Retriever` implementation and
the `cmd/cli` request-flow hook remain explicitly for a later phase per the
task's own scope boundary.
