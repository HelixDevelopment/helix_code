# HelixCode Test-Suite Env-Fragile Threshold Anti-Pattern Audit

**Type:** Task · **Status:** active · **Date:** 2026-06-24 · **Mode:** READ-ONLY discovery/triage (§11.4.118)
**Scope:** `helix_code/` inner Go module — `tests/`, `internal/`, `applications/` test files.
**Trigger:** A §11.4.6 anti-pattern (hardcoded absolute 50MB heap cap, point-snapshot HeapAlloc that swung 8.7→127MB across hosts) was found+fixed in `tests/memory` GC-pressure test. This sweep maps SIBLING + SIMILAR env-fragile hardcoded-threshold assertions.
**Evidence basis:** FACT = lines read directly. UNCONFIRMED = inferred, not run (no execution performed). `tests/memory/*` and `tests/security/*` NOT edited (other streams own them); `tests/memory` only READ.

---

## Findings table

| file:line | test | threshold (assertion) | env-fragility cause | severity | suggested robust fix |
|---|---|---|---|---|---|
| `tests/memory/memory_test.go:140-143` | `TestMemory_LeakDetection_RepeatedRequests` | `delta.HeapAllocDelta < 10MB` (point-snapshot after iterations×10 GETs + 2×GC) | **Point-snapshot HeapAlloc-delta vs absolute MB cap** — identical to the just-fixed 50MB defect. Post-GC HeapAlloc varies with GOGC, scavenger timing, heap fragmentation, host RAM; the same code swung 8.7→127MB. GC keeping up still yields wildly different absolute readings. | **HIGH** | Replace single before/after delta with the *bounded-growth-across-samples* pattern already used in `TestMemory_GCPressure_HighAllocationRate` (sample live post-GC heap at intervals; assert regression-slope / late-window baseline ≤ early-window baseline × small factor, not an absolute MB cap). |
| `tests/memory/memory_test.go:195-197` | `TestMemory_LeakDetection_ConcurrentRequests` | `delta.HeapAllocDelta < 20MB` (10 waves × concurrency × 10 GETs) | Same point-snapshot pattern; concurrency makes the post-GC snapshot even noisier (more in-flight buffers retained at the instant of capture). | **HIGH** | Same as above — multi-sample bounded-growth invariant. |
| `tests/memory/memory_test.go:258-260` | `TestMemory_LeakDetection_JSONParsing` | `delta.HeapAllocDelta < 15MB` (iterations POST w/ 1KB payloads) | Same point-snapshot pattern; JSON encode/decode buffers + GC scavenger timing dominate the absolute reading. | **HIGH** | Same multi-sample bounded-growth invariant. |
| `tests/memory/memory_test.go:780-782` | `TestMemory_ResourceCleanup_Contexts` | `delta.HeapAllocDelta < 10MB` (iterations ctx create/cancel) | Same point-snapshot HeapAlloc-delta-vs-absolute-cap class. | **HIGH** | Same fix. |
| `tests/memory/memory_test.go:381-383` | `TestMemory_Allocation_ConnectionPooling` | `delta.TotalAllocDelta < 50MB` (500 GETs) | Hardcoded absolute alloc cap. NOTE: `TotalAllocDelta` is cumulative-allocated (monotonic, NOT live-heap) so it is far more deterministic than HeapAlloc — 500 small GETs are unlikely to allocate 50MB cumulatively. Generous margin. | **LOW** | Acceptable as-is; optionally scale cap to `requests × per-request-budget` for self-documentation. |
| `tests/memory/memory_test.go:328-330` | `TestMemory_Allocation_LargePayloads` | `delta.TotalAllocDelta < size×100` (per payload size) | RELATIVE to payload size (already a ratio invariant), and `TotalAllocDelta` is cumulative not live-heap. Robust by design. | **LOW** | None — this is the correct ratio-invariant pattern. |
| `tests/memory/memory_test.go:706-708` | `TestMemory_ResourceCleanup_Goroutines` | `goroutineDelta < 50` (final−initial after 5 rounds + 2s sleep + GC) | Fixed-slack goroutine-delta. Fragile vs the `settleGoroutines`-poll pattern: the single 2s sleep may not be enough for HTTP transport idle-conn reaper / DNS goroutines to retire on a slow/loaded host → transient elevated count. 50 slack is generous though. | **MEDIUM** | Adopt the `settleGoroutines(baseline)` poll-until-stable helper already present in `internal/llm/ensemble_stress_chaos_test.go` / `internal/worker/lifecycle_regression_test.go` instead of a fixed `time.Sleep`. |
| `internal/config/advanced_config_test.go:698` | `TestAdvancedConfiguration_Performance` | `duration < 1s` for 1000 `Validate()` calls | Absolute wall-clock cap for CPU-bound loop. Flakes on slow/loaded CI runner, shared host under load, or `-race`/coverage instrumentation (which can 5-20× CPU work). | **MEDIUM** | Drop the timing assertion (it tests the host, not the code) OR raise to a generous ceiling + skip under `-race`/`testing.Short`-loaded. Better: assert no-error + use `go test -bench` for perf tracking, not a unit assertion. |
| `internal/config/advanced_config_test.go:713` | `TestAdvancedConfiguration_Performance` | `duration < 500ms` for 1000 `Transform()` calls | Same as above; tighter 500ms bound = more flake-prone. | **MEDIUM** | Same fix. |
| `internal/event/bus_test.go:180` | (async publish test) | `duration < 50ms` for a single async `bus.Publish` | Tight absolute latency cap on a single op. Intends "returns immediately (async)". A GC pause / scheduler hiccup / loaded host can exceed 50ms for an otherwise-instant call. | **MEDIUM** | Assert the *semantic* invariant: publish returns before the handler completes (e.g. handler blocks on a channel; assert publish returns while channel still blocked), not a wall-clock number. If keeping a cap, raise to ~1s. |
| `internal/tools/web/web_test.go:452` | (FetchMultiple concurrency test) | `duration < 50ms` for 3 concurrent 10ms fetches | Tight absolute cap proving "concurrent faster than 3×sequential(30ms)". 50ms is razor-thin vs scheduler/GC jitter; goroutine startup + httptest overhead can push past it on a loaded host. | **MEDIUM** | Assert `duration < sequentialEstimate` (e.g. `< 25ms` is the *concurrency* claim) but with a generous absolute ceiling, OR assert ratio: concurrent time < 2× single-fetch time. Avoid sub-100ms absolute caps. |
| `internal/discovery/health_monitor_test.go:119` | (warm-up branch) | `time.Since(start) < 50ms` (loop break condition, NOT an assertion) | Control-flow only (decides when binary is "warm"); not a pass/fail assertion. No flake risk to the verdict. | **LOW** | None — informational; it self-adjusts by looping. |
| `internal/discovery/integration_test.go:441` | `WaitForService` timeout test | `elapsed < 1s` (after a 500ms WaitForService timeout) | Upper-bound-on-a-timeout. 500ms slack above the 500ms timeout. Could flake if the host is so loaded the timer fires late, but 2× margin is reasonable. | **LOW** | Acceptable; could widen to 1.5–2s for extra margin under load. |
| `internal/hooks/shell_runner_test.go:49` | (timeout test) | `elapsed < 2s` (timeout must fire before a 5s sleep) | Upper bound well below the 5s sleep it guards — 3s margin. Tests that the timeout mechanism fires, not raw speed. Robust. | **LOW** | None. |
| `tests/e2e/phase3/production_validation_test.go:160` | (production validation) | `duration < 2s` "Response time should be < 2 seconds" | Absolute response-time SLA on a real server call. Depends on server warmth, network, host load. E2E/SLA context (less likely on dev host), but a cold server or loaded CI can exceed 2s. | **MEDIUM** | If this is a genuine SLA gate, keep but document as SLA (not a leak/perf-regression check) and ensure server warm-up precedes timing. Otherwise widen / make percentile-based. |
| `tests/e2e/phase3/performance_test.go:517` | (throughput scalability) | `actualThroughput > target×0.8` (req/s floor) | **Hardcoded throughput FLOOR** relative to a target req/s. Achievable throughput is entirely host/CPU/network-bound; an 80%-of-target floor will FAIL on a slow or loaded runner even with correct code. | **MEDIUM** | Make target self-calibrating (measure baseline single-thread rate first, assert scaling ratio) OR gate behind a dedicated perf-host tag / skip on loaded/CI. Throughput floors are not portable across hosts. |
| `tests/e2e/phase3/performance_test.go:518` | (same) | `successRate > 0.95` | Ratio invariant (correctness, not timing) — robust; a real server should serve ≥95% under nominal load. | **LOW** | None (correctness invariant, not env-fragile). |
| `tests/e2e/phase3/performance_test.go:519` | (same) | `avgResponseTime < 2s` | Absolute latency SLA; same class as the 2s production-validation cap. | **MEDIUM** | Same as production_validation:160 — treat as documented SLA + warm-up, or percentile-based. |
| `tests/performance/benchmark_test.go:274` | `TestPerformance_*` (RPS gate) | `rps < 10 → Errorf` (throughput floor) | Hardcoded absolute RPS floor. 10 req/s is a low bar, but on a heavily-loaded shared host or `-race` build it can dip below. Lives under a `testing.Short()` skip (perf tests). | **MEDIUM** | Low floor mitigates; consider skip-on-loaded-host or calibrate. Pair with the P95<5s gate below (also absolute). |
| `tests/performance/benchmark_test.go:~277` | same | `stats.P95 > 5s → Errorf` | Absolute P95 latency SLA. 5s is generous; flake only on severe host contention. | **LOW** | Acceptable; generous margin. |
| `tests/performance/scenarios/runner_test.go:64` | `TestRunScenario_*` (harness-noise gate) | `cv >= 35.0 → Fatalf` (coefficient-of-variation cap) | CV (relative dispersion) cap — this is *self-calibrating by construction* (CV is host-independent dispersion). 35% is an explicitly-generous bound documented as "typical is single-digit". Robust DESIGN. | **LOW** | None — this is the correct env-independent pattern (ratio/dispersion, not absolute). |
| `tests/qa/qa_test.go:711` | `TestQA_BuildQuality_NoRaceConditions` | `runtime.NumGoroutine() < 1 → Fatal` | Sanity check (runtime always has ≥1 goroutine); never flakes — `<1` is structurally impossible in a live runtime. | **LOW** | None. |

### Robust exemplars (NOT defects — reference patterns for fixing the HIGHs)
- `tests/memory/memory_test.go:390-525` `TestMemory_GCPressure_HighAllocationRate` — the **fixed** pattern: multi-sample post-GC live-heap, bounded-growth-across-run invariant (the comment at lines 500-512 documents the exact 23MB↔125MB cross-host variance lesson). **This is the template for fixing the 4 HIGH point-snapshot tests.**
- `internal/llm/ensemble_stress_chaos_test.go:44-60` `settleGoroutines(baseline)` — poll-until-stable, "no GROWTH beyond small slack" goroutine-leak invariant (deterministic regardless of scheduler).
- `internal/worker/lifecycle_regression_test.go:35-55` `settleGoroutines(watchdog)` — poll-until-two-stable-samples goroutine invariant.
- `tests/integration/provider_integration_test.go:760-790` `testResourceLeakDetection` — `after ≤ baseline+5` relative-to-baseline goroutine invariant.
- `internal/cognee/performance_optimizer_gpu_*_test.go` (lines ~160-238, all 5 GPU vendors) — `elapsed < <configuredTimeout> + Ns` — latency bound RELATIVE to the code's own configured timeout, not an absolute literal. Robust.

---

## Counts per severity (FACT — from rows above)

| Severity | Count | Items |
|---|---|---|
| **HIGH** | 4 | memory_test.go:140 / :195 / :258 / :780 (point-snapshot HeapAlloc-delta vs absolute MB cap) |
| **MEDIUM** | 8 | memory_test.go:706 (goroutine fixed-sleep); advanced_config_test.go:698 / :713 (1s/500ms CPU caps); event/bus_test.go:180 (50ms); web_test.go:452 (50ms); e2e/phase3 production_validation:160 (2s), performance_test:517 (throughput floor) + :519 (2s avg); benchmark_test.go:274 (RPS<10 floor) |
| **LOW** | 9 | memory ConnectionPooling:381, LargePayloads:328; health_monitor:119; discovery integration:441; shell_runner:49; performance_test successRate:518; benchmark P95:277; scenarios cv:64; qa goroutine:711 |

(MEDIUM count = 9 distinct assertions across 8 sites; e2e/phase3/performance_test.go contributes both throughput-floor and avg-latency.)

---

## Prioritized fix list (HIGH first)

1. **HIGH — `tests/memory/memory_test.go:140,195,258,780`** (4 tests): port the 3 leak-detection siblings + the context-cleanup test from point-snapshot-HeapAlloc-vs-absolute-MB to the multi-sample bounded-growth invariant already proven in the same file's `TestMemory_GCPressure_HighAllocationRate` (lines 390-525). Single highest-value fix — same root cause as the already-acknowledged 50MB flake, same blast radius (nondeterministic FAIL on normal hosts). **NOTE: `tests/memory` is owned by another active stream — coordinate before editing.**
2. **MEDIUM — `internal/event/bus_test.go:180` + `internal/tools/web/web_test.go:452`**: sub-100ms absolute latency caps are the most flake-prone of the MEDIUMs (smallest margin vs GC/scheduler jitter). Replace with semantic/ratio invariants.
3. **MEDIUM — `internal/config/advanced_config_test.go:698,713`**: 1s/500ms CPU-bound timing caps flake under `-race`/coverage (which the project runs via `make test-coverage`). Drop timing assertions from unit tests; move perf tracking to benchmarks.
4. **MEDIUM — `tests/e2e/phase3/performance_test.go:517` + `benchmark_test.go:274`**: throughput floors (`target×0.8`, `rps<10`) are host-bound; calibrate to a measured baseline or gate behind a perf-host tag.
5. **MEDIUM — `tests/memory/memory_test.go:706`**: swap fixed `time.Sleep` for the `settleGoroutines` poll helper (same-file/cross-package exemplars exist).
6. **MEDIUM — `tests/e2e/phase3/production_validation_test.go:160` + `performance_test.go:519`**: 2s absolute response-time SLAs — document as SLA + ensure warm-up, or make percentile-based.
7. **LOW**: no action required; several (LargePayloads ratio, cv-cap, GPU `timeout+N`, successRate, goroutine sanity) are already correct env-independent patterns — keep as reference exemplars.

---

## Honest scope notes (§11.4.6)
- **No tests were executed** — all severities are static-analysis judgments of flake-*likelihood*, not observed flakes (except the memory point-snapshot class, whose cross-host variance is documented FACT in the file's own comment lines 500-512 and the originating stream's report).
- `tests/security/*` was excluded from edit but NOT separately swept for threshold patterns beyond the general grep (no hits surfaced in the throughput/latency/goroutine/CV sweeps).
- Sweep covered: absolute latency caps (`time.Since < const`), heap/alloc byte caps, throughput floors, goroutine/connection counts, CV/variance caps, dial/connect timeouts. The previously-fixed discovery 100ms dial timeout did not resurface (already fixed).
- UNCONFIRMED: there may be additional sub-100ms `time.Sleep`-then-assert timing assertions in benchmark-only (`Benchmark*`) functions not surfaced by the `assert.Less`/`Greater` grep; benchmarks are not pass/fail-gated so low priority.
