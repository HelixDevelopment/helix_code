# §11.4.169 Race/Deadlock Coverage Sweep — HelixCode Inner Go Module

**Run ID**: `race_sweep_20260711_142529`
**Date**: 2026-07-11
**Repo**: `/home/milos/Factory/projects/tools_and_research/helix_code`
**Inner Go module**: `helix_code/helix_code/` (module `dev.helix.code`, `go 1.26`)
**Track/branch**: `(T1/feature/helixllm-full-extension - claude3)`
**Go toolchain used**: `go version go1.26.4-X:nodwarf5 linux/amd64`

## Scope

§11.4.169 mandates race-detector coverage as a required test type. This sweep
targets the concurrency-sensitive packages of the inner Go module (mutex /
goroutine / channel usage), scoped strictly to `helix_code/helix_code/internal/**`:

- `internal/llm` (+ subpackages: compression, compressioniface, i18n, litellm,
  promptcache, providers/{cerebras,cohere,helixagent,huggingface,replicate,together},
  routing, vision)
- `internal/server` (+ i18n)
- `internal/worker` (+ i18n)
- `internal/task` (+ i18n)
- `internal/session` (+ i18n)
- `internal/provider` (+ i18n)
- `internal/providers` (+ httpclient, i18n)
- `internal/redis` (+ i18n)
- `internal/memory` (+ i18n, providers)

Candidate identification: `grep -rl 'sync\.' / 'go func' / 'chan '` across
`internal/{llm,server,worker,task,session,provider,providers,redis,memory}`
(non-test `.go` files) — 30/8/8 llm files, 7/1/2 worker, 2/0/0 task, 4/1/1
session, 1/1/1 provider, 3/0/0 providers, 1/0/0 redis, 17/0/0 memory,
2/2/2 server (sync/gofunc/chan file counts respectively). `internal/security`
and `internal/tools/browser` were explicitly excluded per the operator's hard
constraint (another stream just fixed those).

**Coder :18434 was READ-ONLY and never touched** — verified before starting
that its listener (`llama-server`, pid 1980342) was left alone; no LLM calls
were made by any test in this sweep (all packages compile-checked and tested
purely in-process/offline).

## Step 1 — compile check

```
$ go build ./internal/llm/... ./internal/server/... ./internal/worker/... \
    ./internal/task/... ./internal/session/... ./internal/provider/... \
    ./internal/providers/... ./internal/redis/... ./internal/memory/...
(no output — clean compile)
```

## Step 2 — `go test -race -count=1` full sweep (run 1, pre-guard-addition)

Executed in 3 parallel groups (background for `internal/llm`, foreground for
the other two groups) to fit tool time budgets. Raw logs:

- `01_llm_run1.txt` — `internal/llm/...` — **14 sub-packages, all `ok`, 0 races**
- `02_group1_server_worker_task_session_run1.txt` — **8 sub-packages, all `ok`, 0 races**
- `03_group2_provider_providers_redis_memory_run1.txt` — **10 sub-packages, all `ok`, 0 races**

Total: **32 packages tested** (27 with test binaries executed + 5
`[no test files]` i18n leaf packages), **0 DATA RACE reports, 0 FAILs, 0
panics**, all exit code 0.

Notable per-package timings (race detector overhead visible): `internal/llm`
137.7s, `internal/worker` 45.5s, `internal/memory/providers` 45.9s,
`internal/redis` 9.4s — consistent with the heavy goroutine/mutex usage
identified in step 1's grep survey.

## Step 3 — real race found? Pre-existing non-race FAIL?

**No real data race was found** anywhere in the swept scope on run 1. **No
pre-existing non-race FAIL was found either** — every package that has test
files reported `ok`. Per the task's honest-reporting instruction: this sweep
found the codebase already race-clean and fully passing under `-race` in this
scope; there was nothing to systematic-debug or fix per §11.4.102, and no
tracked follow-up defect to record.

## Step 4 — race-CLEAN: new concurrency guard test + load-bearing mutation

Since the sweep was clean, per the task's step 4 fallback: a survey for a
concurrency-critical path that still **lacked** a `-race` guard was performed.

### Survey

- `internal/task`, `internal/worker`, `internal/session` are heavily covered
  already: dedicated `*_race_test.go`, `*_stress_test.go`, `*_chaos_test.go`
  files exist for their manager/pool types (e.g.
  `internal/task/getter_snapshot_race_test.go`,
  `internal/worker/worker_pool_stress_test.go`,
  `internal/session/manager_callback_race_test.go`).
- `internal/providers/ai_integration.go` (`AIIntegration`, `sync.RWMutex`) and
  `internal/providers/vector_integration.go` (`VectorIntegration`,
  `sync.RWMutex`) are **both already covered** by
  `internal/providers/ai_integration_race_guard_test.go`, which drives
  `Initialize` (writer) concurrently against `GetStats`/`HealthCheck`
  (DEFECT A), `LLMProviderAdapter` cost-info (DEFECT B), and
  `GetVectorStats`/`StoreVectorInProvider`/`SearchVectors`/`HealthCheck`
  (DEFECT C + extension) on `VectorIntegration`.
- `internal/llm` has 28 non-test files with `sync.{Mutex,RWMutex}` fields.
  Cross-checking against files with `go func`/`WaitGroup` in their `_test.go`
  siblings found **five files with ZERO goroutine-based concurrency test
  coverage**: `cross_provider_registry.go`, `integrated_model_manager.go`,
  `model_discovery.go`, `wizard.go`, `model_converter.go`.

### Chosen target: `internal/llm/cross_provider_registry.go` — `CrossProviderRegistry`

A textbook shared-manager-with-`sync.RWMutex` path: one struct
(`CrossProviderRegistry`) guards three maps (`compatibility`, `providers`,
`downloadedModels`) with a single `RWMutex`, exposing one writer
(`RegisterDownloadedModel`, `mu.Lock()`) and **seven** reader methods
(`GetCompatibleFormats`, `CheckCompatibility`, `GetDownloadedModels`,
`FindModelsForProvider`, `FindOptimalProvider`, `GetProviderInfo`,
`ListProviders`, plus the internal `findCompatibleProvidersForModel` RLock
wrapper) — all realistically callable from concurrent goroutines in
production (model-download workers registering models while
request-handling code concurrently looks up compatibility/providers). Before
this sweep, `cross_provider_registry_test.go` had **zero** `go func` /
`WaitGroup` occurrences.

### New guard test

**File**: `helix_code/helix_code/internal/llm/cross_provider_registry_race_test.go`
(new)

`TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace` drives:

- 20 concurrent goroutines calling the real `RegisterDownloadedModel` (writer,
  each registering a distinct model — genuinely mutates the map, not a no-op
  re-write of the same key).
- 15×8 = 120 concurrent goroutines calling all seven real reader methods
  (`GetCompatibleFormats`, `CheckCompatibility`, `GetDownloadedModels`,
  `FindModelsForProvider`, `FindOptimalProvider`, `GetProviderInfo`,
  `ListProviders`, `findCompatibleProvidersForModel`).
- Post-condition: after `wg.Wait()`, `GetDownloadedModels()` returns exactly
  20 models — proving the guard is functionally meaningful (not just
  race-silent), all 20 concurrent writes landed.

**GREEN under `-race`** (run 1):

```
=== RUN   TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace
--- PASS: TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace (0.02s)
PASS
ok  	dev.helix.code/internal/llm	1.045s
```

**Determinism — 3 additional standalone runs, all GREEN** (`05_new_guard_test_3x_determinism.txt`):

```
=== guard-only run 1 ===
ok  	dev.helix.code/internal/llm	1.047s
=== guard-only run 2 ===
ok  	dev.helix.code/internal/llm	1.047s
=== guard-only run 3 ===
ok  	dev.helix.code/internal/llm	1.044s
```

### §1.1 paired mutation — proving the guard is load-bearing

The write-lock was **temporarily** removed from `RegisterDownloadedModel`
(commented out `r.mu.Lock()` / `defer r.mu.Unlock()`, leaving the seven
reader methods' `RLock`/`RUnlock` untouched — an unsynchronized writer racing
against still-synchronized-but-unprotected-from-the-writer's-perspective
readers). Re-running the **same, unmodified** guard test against the mutated
source:

```
$ go test -race -count=1 -run TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace ./internal/llm/
... (22× "WARNING: DATA RACE" blocks between findCompatibleProvidersForModelLocked's
     map iteration/read and RegisterDownloadedModel's mapassign, across the
     writer and reader goroutines) ...
fatal error: concurrent map iteration and map write
FAIL	dev.helix.code/internal/llm	0.039s
EXIT=1
```

Full captured mutation-run output (22 DATA RACE reports + a genuine Go
runtime `fatal error: concurrent map iteration and map write` crash) is
preserved verbatim in `06_mutation_experiment_race_FAIL.txt`.

This is **rock-solid, non-fabricated evidence** the guard is load-bearing: it
does not merely run without complaint by luck — removing the lock it exists
to verify makes it fail loudly and immediately, with the race detector
pinpointing the exact unsynchronized read/write pair inside the very
production functions the guard exercises.

**Mutation reverted immediately** after capturing the FAIL evidence (§11.4.84
— no mutation residue committed):

```
$ git diff -- internal/llm/cross_provider_registry.go
(empty — file byte-identical to HEAD)
$ go test -race -count=1 -run TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace ./internal/llm/
ok  	dev.helix.code/internal/llm	1.066s
```

## Step 5 — determinism, full-scope

Two independent full-scope `-race` runs of all 9 target package trees
together (`go test -race -count=1 -timeout=600s ./internal/llm/... ./internal/server/...
./internal/worker/... ./internal/task/... ./internal/session/... ./internal/provider/...
./internal/providers/... ./internal/redis/... ./internal/memory/...`):

- **Run 2** (`04_all_packages_run2_determinism.txt`, captured BEFORE the new
  guard test file was added): 27 `ok`, 0 DATA RACE, 0 FAIL, exit 0.
- **Run 3** (`07_all_packages_run3_final_with_new_guard.txt`, captured AFTER
  the new guard test was added and the mutation fully reverted, i.e. the
  final, authoritative state of the tree): 27 `ok`, 0 DATA RACE, 0 FAIL, exit 0.

Both full-scope runs, plus the 4 standalone runs of the new guard test (1
initial + 3 determinism), plus the original 3-way split run 1 — **7
independent `-race` executions total, all reproducing the same clean
verdict** across the swept scope. Determinism (§11.4.50) confirmed.

## Summary table

| Package tree | Sub-packages tested | `-race` verdict | Notes |
|---|---|---|---|
| `internal/llm` | 14 | clean (0 races) | +1 new guard test added, load-bearing per mutation proof |
| `internal/server` | 2 | clean (0 races) | |
| `internal/worker` | 2 | clean (0 races) | already has dedicated race/stress/chaos suites |
| `internal/task` | 2 (1 no-test) | clean (0 races) | already has `getter_snapshot_race_test.go`, stress/chaos |
| `internal/session` | 2 (1 no-test) | clean (0 races) | already has `manager_callback_race_test.go`, deadlock test |
| `internal/provider` | 2 | clean (0 races) | |
| `internal/providers` | 3 | clean (0 races) | already has `ai_integration_race_guard_test.go` (5 defects A–C+ext) |
| `internal/redis` | 2 | clean (0 races) | |
| `internal/memory` | 3 (1 no-test) | clean (0 races) | |

## Files changed

- **New**: `helix_code/helix_code/internal/llm/cross_provider_registry_race_test.go`
  — standing regression guard, `-race`-clean, mutation-proven load-bearing.
- **Unchanged** (mutation applied and reverted during evidence capture only,
  confirmed via empty `git diff`): `helix_code/helix_code/internal/llm/cross_provider_registry.go`

No other files under `helix_code/helix_code/internal/**` were touched.
`internal/security` and `internal/tools/browser` were not touched. Coder
(`:18434`) was never called or modified.
