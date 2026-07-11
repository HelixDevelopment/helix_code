# HXC-125 — Surface integration-tagged tests via Makefile (§11.4.169 test-visibility)

## FACT confirmed (F-D1-06)

`helix_code/tests/integration/**` (32 files, `//go:build integration`) is invisible to a
bare `go test ./...` / `make test`. `make test-integration-full` exists but requires the
full docker-compose stack (`test-infra-up`) — heavy, not a lightweight standalone check.
No lightweight tag-only target existed before this change.

## Change made

File touched: `helix_code/Makefile` (only file touched — no other files edited).

1. Added new target `test-integration-tag` (placed immediately after
   `test-integration-full`, ~line 253) that runs:
   `go test -v -tags=nogui,integration -count=1 ./tests/integration/...`
   with a comment block explaining the F-D1-06 gap and why it's distinct from
   `test-integration-full` (no infra bring-up required; individual tests may still
   honestly SKIP-with-reason at runtime per §11.4.3 when a specific external dep is
   unreachable).
2. Added `test-integration-tag` to the existing `.PHONY` line (line 179).
3. Added a `make help` line documenting the new target (line ~508).
4. Did NOT touch `test-integration-full` or any other existing target.

## Command run + captured output

```
cd helix_code && make test-integration-tag 2>&1 | tee <evidence-run-log> | tail -80
```

Full raw output captured at:
`/tmp/.private/milos/claude-1000/-home-milos-Factory-projects-tools-and-research-helix-code/af14e62d-3bda-4580-9f8d-639f6e1cf74e/scratchpad/discovery/fixes/W2B_run_output.txt`

Summary counts extracted from that log:

```
PASS count: 123
FAIL count: 0
SKIP count: 29   (all honest SKIP-with-reason, e.g. TestCogneeRealLLMTestSuite:
                  "config validation failed: JWT secret must be set and not use
                  default value" — a genuine missing-local-secret condition, not
                  a silent/bare skip)
ok  lines (packages passed): 5
FAIL package lines: 0
```

Tail of run:
```
=== RUN   TestIntegration_PathTraversalRejected
--- PASS: TestIntegration_PathTraversalRejected (0.04s)
PASS
ok  	dev.helix.code/tests/integration/worktree	0.134s
✅ Standalone integration-tagged tests complete
```

Packages exercised (confirmed non-trivial, i.e. NOT "0 tests"/"no test files" for the
actual test packages — the `[no test files]` lines seen belong to
`tests/integration/cmd/p1*_challenge` / `p2*_challenge`, which are separate challenge
runner `cmd/` binaries under that tree, not test packages, so that's expected):
- `dev.helix.code/tests/integration` (the bulk — 158.868s wall time)
- `dev.helix.code/tests/integration/hooks`
- `dev.helix.code/tests/integration/permissions`
- `dev.helix.code/tests/integration/persistence`
- `dev.helix.code/tests/integration/worktree`

## Self-review

- `grep -n "test-integration" Makefile` confirms: `test-integration-tag` defined exactly
  once, added to `.PHONY` exactly once, `test-integration-full` referenced only in its
  original two spots (its own rule + the `test-complete` composite) — unmodified.
- No other Makefile targets renamed/removed.
- No git add/commit performed (per task instructions).
- No files touched outside `helix_code/Makefile`. (Did not add a `tests/README.md`
  pointer — that file documents a `./run_tests.sh` wrapper that does not actually exist
  in the tree; adding a target-name pointer there was judged out of scope /
  potentially confusing given the doc's existing drift, and the task's "MAY add"
  clause makes this optional. Flagging this pre-existing tests/README.md ↔ reality
  drift as a separate observation, not fixed here — out of scope for HXC-125.)

## Status: DONE
