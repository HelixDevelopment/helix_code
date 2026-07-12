# HXC-144 — QA evidence (§11.4.83)

**Item:** HXC-144 (Bug/Med — reclassified: test-infra robustness) — server "goroutine leak under DDoS-flood"
**Module:** `helix_code/tests/stresschaos/` (test harness only — NO production code)
**Fix commit:** helix_code `4e03dd97` (3 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T15:25:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)
**Origin:** 2026-07-12 real-infra retest reported Server chaos 7/8 (goroutine delta 5 > tol 4).
**Discipline:** §11.4.102, §11.4.115 RED-first, §11.4.142 independent review, §11.4.6 honest-classification.

## Root cause (FACT) — CONFIRMED MEASUREMENT ARTIFACT, not a product leak

Systematic tracing of every flooded handler found NO per-request goroutine/ticker/unclosed-channel — no
leak-shaped code in `internal/server`. The `tests/stresschaos/stresschaos.go` `RunConcurrent` harness
measured process-wide `runtime.NumGoroutine()` with a single fixed 50ms settle + one GC; Go's `net/http`
`persistConn` readLoop/writeLoop teardown goroutines exit ASYNCHRONOUSLY, so the coarse count caught
in-flight teardown as a false "leak" (delta 5 > tol 4). **The product server is clean.**

## Fix (test harness only)

`RunConcurrent` settle → `settleGoroutines()`: poll-until-stable (25ms interval, 2s budget, 3 consecutive
equal samples = stable) + `closeIdleHTTPConnections()` to deterministically force idle-transport teardown.
`goroutineLeakTolerance` kept at 4 (NOT loosened — loosening would mask real future leaks). New
`pprof_diff_test.go`: a pprof-goroutine-diff oracle classifying residual goroutines `APP-LEAK:<frame>`
(fails + names the site) vs `http-transport-teardown:<frame>` (confirmed artifact) — the definitive
artifact-vs-real-leak discriminator the coarse count lacked.

## Captured verification

```
TestGoroutineLeakOracle_HTTPFlood_... : 640 real HTTP GETs vs httptest server → zero residual app classes ×3 runs
TestSettle_PollUntilStable_... : RED old-50ms-sleep delta=6>tol vs 6 staggered-exit goroutines; GREEN settleGoroutines → delta=0
go build/vet ./tests/stresschaos/... exit 0 ; all tests PASS -count=3 -race (deterministic)
TestMeta_RunConcurrent_DetectsGoroutineLeak (pre-existing real-leak detector) still PASS
```

## Independent review (§11.4.142) — VERDICT: GO, zero blocking

Anti-mask §1.1 (the crux): a transient real never-exiting goroutine (delta=1, UNDER the coarse
tolerance=4 the old check would miss) was STILL caught by the pprof-diff oracle as
`APP-LEAK:dev.helix.code/...` — proving the oracle catches real leaks the coarse check misses (the
harden removed false-positive noise, did not raise the bar). Never-stabilizing churn: `settleGoroutines()`
terminated at 2.017s (budget respected, no infinite loop, no masking). Meta-test intact. Reclassification
Bug→test-infra concurred. Honest boundary: a ~1-goroutine-per-300-500ms slow-drip could theoretically
stabilize mid-drip — an IDENTICAL limitation of the old single-sleep, not introduced here.

## Release impact

Reclassifies the retest's Server chaos 7/8 → effectively 8/8: the one "fail" was a harness measurement
artifact, not a product defect. The server is clean under the DDoS-flood chaos test.
