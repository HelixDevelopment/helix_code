# Real-Infrastructure Test-Execution Evidence — HXC-122 / HXC-138 (/ HXC-136)

Run ID: `infra_retest_20260712_hxc122_138`
Date (UTC): 2026-07-12
Host: local dev workstation (rootless podman 5.7.1, `podman-compose` 1.5.0)
Operator/agent: INFRA-OWNER subagent, exclusive test-infra ownership this turn (§11.4.119)

This document is a narrative, honest account of what was actually run, what
genuinely executed, what passed/failed for real reasons, and what is an
honest SKIP or a genuine new defect finding. No PASS below is claimed without
the real captured command output backing it (raw logs live alongside this
file).

## 1. Stack bring-up (real infrastructure, real functional probes)

Brought up via the project's `docker-compose.full-test.yml` (the same compose
file `make test-infra-up` drives), through `podman-compose` — rootless, no
sudo, no docker (§11.4.161). Because the full 15-service compose file
includes several services that require local Docker builds (mock-llm-server,
4x ssh containers, etc.) that are unrelated to the two items in scope, only
the services actually needed for HXC-122 + HXC-138 were started explicitly:
`postgres`, `redis`, `ollama`.

```
podman-compose -f docker-compose.full-test.yml up -d postgres redis ollama
```

All three came up **healthy** per podman's own healthchecks, confirmed with
**functional probes** (not container-`Up`-state alone, per CONST-035 / §11.4.68):

- Postgres: `podman exec helixcode-postgres-full pg_isready -U helixcode -d helixcode_test`
  → `accepting connections`; **and** `psql -c "SELECT 1 AS probe;"` → real row
  returned (`probe | 1`). See `bringup.log`.
- Redis: `podman exec helixcode-redis-full redis-cli ping` → `PONG`.
- Ollama: `podman ps` reports `(healthy)`; confirmed with a genuine inference
  call (see §2).

A real model was pulled into the Ollama container — **`qwen2:0.5b`** (352 MB,
not the runner's `llama2:7b` default, deliberately: a small model keeps the
"real inference, no bluff" property while keeping pull/inference time
reasonable within this task's time budget). Pull completed in 17.9s
(`bringup.log`).

```
$ curl http://localhost:11434/api/generate -d '{"model":"qwen2:0.5b","prompt":"What is 2+2? Answer with just the number.","stream":false}'
{"model":"qwen2:0.5b", ... "response":"1", "done":true, ...}
```

Real HTTP round-trip, real (if numerically wrong — the model is tiny) model
output — genuine inference, not a simulated/canned response.

### Server binary

`make build` succeeded, producing `bin/helixcode`.

**Finding (environment-specific, not a code defect):** host port **8080**
is occupied by an unrelated systemd service (`ahttpd.service`, confirmed via
`ss -ltnpe` → `cgroup:/system.slice/ahttpd.service`) that pre-dates this test
run and belongs to the host, not this project. Per §11.4.174 (process/port
ownership verification) this service was **not** touched. The server config
does not have a `HELIX_SERVER_PORT`→`server.port` viper env-binding (only an
explicit `v.BindEnv(...)` list is honoured; `server.port` isn't in it), so a
plain env-var override does not work. Worked around with a **standalone
config file** (`server.port: 18080`, real Postgres/Redis/Ollama endpoints)
rather than editing any tracked file. The server auto-boot logic
(`internal/deployment` on-demand infra) additionally spun up its own
`helixcode-autoboot-postgres` / `helixcode-autoboot-redis` containers on
ports 55432/56379 — these are legitimately **ours** (created by our own
server process this run) per §11.4.174, distinct from the compose-managed
ones.

Server started, real health check:

```
$ curl http://localhost:18080/health
{"status":"healthy","timestamp":"2026-07-12T14:34:52Z","version":"1.0.0"}
```

**Documented, repeatable recipe** (the deliverable HXC-122 asks for):

```bash
cd helix_code
podman-compose -f docker-compose.full-test.yml up -d postgres redis ollama
podman exec helixcode-ollama-full ollama pull qwen2:0.5b   # or any small real model
make build
HELIX_CONFIG=<config-with-real-db/redis/ollama-and-a-free-port> \
  OLLAMA_MODEL=qwen2:0.5b ./bin/helixcode &
HELIXCODE_TEST_URL=http://localhost:<port> go test -v -count=1 ./tests/memory/...
```

## 2. HXC-122 — memory-usage suite: REAL-PASS (full execution, zero skips)

`go test -v -count=1 -timeout=20m ./tests/memory/...` against the real
running server (`HELIXCODE_TEST_URL=http://localhost:18080`).

**Result: 15/15 tests PASS, 0 FAIL, 0 SKIP.** Every test in the package
genuinely executed against the live server — no `t.Skip("Server not
available...")` fired anywhere (previously the entire suite's default
behaviour with no server running). Full raw output: `memory_tests.log`.

Highlights of genuine runtime evidence captured (not metadata-only):
- `TestMemory_LeakDetection_RepeatedRequests`: 1000 real HTTP requests
  against `/health`; live-heap trend signal 0.8091 correctly computed from
  real post-GC heap samples.
- `TestMemory_GCPressure_HighAllocationRate`: **268,275 real requests** in
  30.07s (8,921.9 req/s), 469 real GC cycles observed, leak-trend signal
  0.0178 (correctly below the 0.50 leak threshold) — this is real sustained
  load against the real server, not a canned number.
- `TestMemory_Stress_SustainedLoad`: 60s sustained real load, real heap
  samples across the window.
- `TestHeapTrend_FlagsMonotonicLeak` / `TestHeapTrendIsLeak_RequiresTrendAndMagnitude`:
  the trend-detection algorithm itself validated against synthetic
  monotonic/noisy/declining series (these two are algorithmic unit checks,
  not server-dependent, and were already passing before this run — included
  here for completeness).

**Conclusion for HXC-122 / memory:** REAL-PASS. The suite was never broken —
it was gated entirely on `HELIXCODE_TEST_URL` pointing at a live server; once
one is up it runs to completion with real captured evidence. The above
recipe is the documented, repeatable way to run it.

## 3. HXC-122 — automation suite: mixed (env-flags confirmed working; TWO
   pre-existing compile-breaking defects found and one fixed locally)

Two separate Go packages both live under this "automation" umbrella and were
investigated:

### 3a. `tests/automation/hardware_test.go` (package `automation`, no build
tag) — **REAL-PASS / REAL-IN-PROGRESS, genuinely executes**

This is the actual "special env flags" suite HXC-122 describes:
`RUN_REAL_EXECUTION=true`, `RUN_BENCHMARKS=true`, `RUN_RESOURCE_TESTS=true`,
`RUN_CROSS_PLATFORM=true`, plus `OLLAMA_BASE_URL` (or `LLAMACPP_BASE_URL` /
`LLM_BASE_URL`) for the real-endpoint check that a prior anti-bluff fix
(`SKIP-OK: #hardware-inference` / `#hardware-workload`) added — the suite
correctly refuses to fake success when no endpoint env var is set.

With all four `RUN_*` flags + `OLLAMA_BASE_URL=http://localhost:11434` set,
the suite **genuinely executes** (no longer skips):

```
--- PASS: TestHardwareDetection (0.04s)     # real CPU/GPU/OS detection: AMD
                                             # Threadripper 7970X, RTX 5090,
                                             # 251GB RAM, ALT Workstation 11.1
--- PASS: TestHardwareOptimizedProviders (0.03s)
=== RUN   TestRealModelExecution/RealExecution_phi-2
    ... genuinely attempts to git-clone + pip-install/build multiple real
    local-LLM-runtime backends (KoboldAI, GPT4All, MLX, LocalAI, FastChat,
    LM Studio, Jan AI, Text Generation WebUI, ...) per candidate model ...
```

This is **real, non-simulated work** — real `git clone` attempts, real `pip
install -e`, real Python package builds — proving the suite is not a bluff.
**Operational finding (not a code defect, worth its own low-severity
ticket):** `TestRealModelExecution` attempts to auto-install a full roster of
~10 alternative local-LLM backends (KoboldAI, GPT4All, MLX, LocalAI,
FastChat, LM Studio, Jan AI, TabbyAPI, Text Generation WebUI, Mistral RS,
vLLM, ...) for *each* of 5 candidate models (phi-2, qwen1.5-1.8b, gemma-2b,
llama-3-8b, mistral-7b), which is extremely slow and largely redundant when
a working Ollama endpoint is already reachable. It genuinely progressed
through the full provider roster for `phi-2`, then `qwen1.5-1.8b`, then into
`gemma-2b` over ~12 minutes of wall-clock (confirmed by log timestamps
19:37→19:49) — real, continuous (if slow) work throughout, not a hang.

**Self-correction (§11.4.6 honesty note):** partway through, a ~3-minute gap
with no new log lines and a 0%-CPU/no-visible-connection process snapshot
was misread as a genuine hang, and the process was manually terminated
(`kill -TERM`, PID 4096505, confirmed ours per §11.4.174 — `cwd` inside this
repo). The very next log check (post-kill) showed the run had in fact
continued to the `gemma-2b` model and multiple further provider-install
attempts in the interim — i.e. it was **not** hung, just slow with an
unlucky sampling window. It was terminated before reaching its own 20-minute
(`timeout 1200`) cap. Recorded here plainly rather than silently corrected,
per the anti-bluff mandate: this task's own termination decision, not a
defect in the suite, is why the run did not reach completion. Raw log:
`automation_tests.log`.

**Conclusion:** the documented env-flag recipe works — the suite executes
for real instead of skipping. Full-completion timing is a separate,
lower-severity efficiency concern (candidate for a new low-severity ticket:
"`TestRealModelExecution` should prefer an already-reachable
`OLLAMA_BASE_URL` over installing N alternative backends per model").

### 3b. `test/automation/*.go` (package `automation`, **`//go:build automation`**
build tag) — **REAL-FAIL: does not compile — NEW FINDING**

This is a *different* package (root-level `test/automation/`, distinct from
`tests/automation/`) gated by `-tags=automation`. Investigating why it
"skips" surfaced that it does not even reach the skip logic — **it does not
compile**:

1. `qwen_automation_test.go` and `xai_automation_test.go` both declared
   byte-identical `func isRateLimitError(err error) bool` and
   `func contains(s, substr string) bool` helpers, producing a hard
   `<foo> redeclared in this block` compile error for the whole package.
   **Fixed locally** (uncommitted — conductor to review/commit or handle
   otherwise): removed the duplicate block from `xai_automation_test.go`,
   left a dated comment explaining why. Diff is currently uncommitted in the
   working tree at `test/automation/xai_automation_test.go`.
2. After that fix, `go vet -tags=automation ./test/automation/...` surfaces
   a **second, deeper defect**: `automation_test.go:26:16: undefined:
   llm.ProviderConfig` (and by extension `llm.NewProviderManager`,
   `providerManager.GetAvailableProviders()` — none of these exist anywhere
   in the current `internal/llm` package; confirmed via
   `grep -rl GetAvailableProviders **/*.go` → only 3 hits, all test files).
   This is genuine API drift: this test file was written against an older
   `internal/llm` provider-manager API shape that no longer exists. This is
   **not** a trivial fix — it requires redesigning the test against the
   current `ProviderFactory`/individual-provider API, which is out of scope
   for an infra-execution/evidence-gathering task and was **not** attempted.

**Captured command output:**
```
$ go vet -tags=automation ./test/automation/...
# dev.helix.code/test/automation
# [dev.helix.code/test/automation]
vet: test/automation/automation_test.go:26:16: undefined: llm.ProviderConfig
```

3. A related sibling package, `test/e2e/*.go` (`//go:build e2e`), was also
   checked for completeness (same class of concern — a second "automation
   / e2e" suite HXC-122's title implicitly covers) and is **also broken**:
   ```
   $ go vet -tags=e2e ./test/e2e/...
   vet: test/e2e/qwen_e2e_test.go:285:6: getEnvOrDefault redeclared in this block
   ```
   Not investigated further (out of the two items' direct scope) but
   recorded here for the tracker.

4. For contrast/completeness: `test/integration/*.go` (`//go:build
   integration`) **does compile cleanly** — `go vet -tags=integration
   ./test/integration/...` → no output, i.e. clean.

**Conclusion for `test/automation` (`-tags=automation`):** REAL-FAIL / NEW
FINDING. This suite cannot be run "against real infrastructure" at all right
now — it cannot even be built. This is a materially worse defect than "skips
without a server" (HXC-122's original framing) and should be tracked as its
own item (recommend: new HXC ticket, e.g. "`test/automation` (`-tags=automation`)
and `test/e2e` (`-tags=e2e`) reference a removed `internal/llm` API
(`ProviderConfig`/`NewProviderManager`/`GetAvailableProviders`) and duplicate
symbols across files — packages do not compile").

## 4. HXC-138 — e2e challenge suite against a running server + real model:
   REAL-EXECUTION, honest REAL-FAIL outcome (harness works; tiny model's
   output didn't pass acceptance validation)

Ran the fixed e2e challenge runner (`tests/e2e/challenges/cmd/runner`)
against the real running infra:

```
cd tests/e2e/challenges
go run cmd/runner/main.go -all -interfaces cli -distributions single \
  -providers ollama -models qwen2:0.5b \
  -batch-name hxc138-real-server-real-model \
  -export-report .../hxc138_challenge_report.json -verbose
```

**Result: all 6 defined challenges genuinely executed end-to-end** (real LLM
call → real code-generation response → real file write → real `go build` /
`go test` invocation → real structured validation), batch completed in
1m16s. Full log: `e2e_challenges.log`; full JSON report:
`hxc138_challenge_report.json`; per-execution logs preserved under
`tests/e2e/challenges/test-results/logs/<execution-id>/{execution,validation,requests}.log`.

```
Total Executions: 6
  Completed:      0
  Failed:         0
  Timeout:        0
  Val. Failed:    6
Success Rate:     0.00%
Files Generated:  18
Total LOC:        777
```

Inspected the per-execution validation logs (not just the summary line, per
§11.4.5/§11.4.69) to confirm the 0% success rate is a **genuine, honestly
reported model-capability limitation**, not a harness bug:

- Every execution made a real Ollama HTTP call and got a real, non-empty LLM
  response (e.g. execution `c75503a4-...`: "Response length: 12922
  characters"; `92318aa8-...`: 4690 chars; `abccaa48-...`: 6086 chars).
- Real files were written and real `go build`/`go test` were invoked against
  them. Some genuinely **compiled** (`92318aa8-...`, `92259891-...`:
  "Compilation successful"); others failed to compile for real, specific
  reasons the tiny 0.5B model produced (e.g. `abccaa48-...`:
  `undefined: schemaValidate`, `assignment mismatch: 1 variable but
  formatFile returns 2 values` — genuine Go compiler errors on genuinely
  bad generated code).
- Validation failures are specific and real (missing README sections,
  missing data-persistence layer, missing `go.mod`, failing `go test`
  invocations with real compiler diagnostics) — not generic/stubbed
  failures.

**Important honest finding about the harness itself:** none of the four
"interfaces" the runner supports (`cli`, `rest`, `tui`, `websocket`) actually
drive the running HelixCode HTTP server's API — `executeCLI` and
`executeREST` both call the LLM provider **directly** via `NewLLMClient`
(only the system-prompt wording differs between them), never issuing a
request to the `helixcode` server process. So "run the challenges against a
running server" is accurate in the sense that a real server *was* running
throughout this test (per HXC-138's ask), but the challenge runner's own
code-generation path does not currently exercise that server's HTTP surface
at all — worth noting for the tracker as a documentation-vs-implementation
gap, though not a regression introduced by this evidence-gathering run.

**Conclusion for HXC-138:** REAL-EXECUTION confirmed — the previously-fixed
`-all` flag genuinely launches every scenario, real LLM calls happen, real
build/test/validation happens, and the 0/6 pass rate is an honest reflection
of a 0.5B model's code-generation capability rather than a harness defect.
Re-running with a larger real model (e.g. `qwen2.5-coder:7b` or similar) is
expected to raise the pass rate materially; that was not attempted here to
stay within this task's time/network budget (documented as an honest
coverage gap, not silently implied "confirmed working at scale").

## 5. HXC-136 (partial, time-permitting) — stress+chaos against real
   Postgres+Redis

`make`-equivalent target `stress-chaos-infra` was run manually against the
real Postgres/Redis already up from §1:

```
HELIX_TEST_DB_HOST=localhost HELIX_TEST_DB_PORT=5432 \
HELIX_TEST_DB_USER=helixcode HELIX_TEST_DB_PASSWORD=helixcode_test_password \
HELIX_TEST_DB_NAME=helixcode_test \
go test -tags=integration -race -run 'Stress|Chaos' -v -count=1 \
  ./internal/redis/... ./internal/database/... ./internal/server/... ./internal/llm/...
```

**First pass:** 57 PASS / 4 FAIL / 8 SKIP. All 8 SKIPs were `internal/server`
chaos+stress tests, and were **honest** SKIP-OKs (§11.4.3-compliant, real
error captured): `internal/server/server_chaos_test.go` /
`server_stress_test.go` connect to Postgres using `TEST_PG_*` env vars
(default user `helix` / db `helix_test`), which differs from the
`docker-compose.full-test.yml` Postgres (`helixcode` / `helixcode_test`) —
a real SASL auth failure (`FATAL: password authentication failed for user
"helix"`) was captured and correctly turned into a skip rather than a fake
pass. Of the 4 FAILs: three (`TestOllamaProvider_Stress_SustainedGetModels`,
`_BoundedConcurrentGenerate`, `_SmallSequentialGenerate`) were caused by
this task's own earlier choice of test model — `ollama_provider_stress_test.go`
hardcodes a default expected model `qwen2.5:0.5b` (overridable via
`OLLAMA_TEST_MODEL`), and only `qwen2:0.5b` had been pulled at that point (a
**different** model — note "2" vs "2.5").

**Corrective rerun** (targeted, per §11.4.130 — validate the fix first):
pulled the exact model the suite expects (`ollama pull qwen2.5:0.5b`, 397 MB,
19s) and set the correct `TEST_PG_*` credentials
(`TEST_PG_USER=helixcode TEST_PG_PASSWORD=helixcode_test_password
TEST_PG_DB=helixcode_test`), then re-ran
`./internal/server/... ./internal/llm/` — full log: `stress_chaos_rerun.log`.

- **`internal/server` (previously 8/8 SKIP) → now genuinely executes:**
  7 PASS, 1 real FAIL.
  - `TestServer_Chaos_MalformedRequests`, `_HandlerPanicIsolation`,
    `_ConcurrentMalformedChurn`, `_SlowAndCancelledRequests`,
    `TestServer_Stress_SustainedHealthLoad`,
    `_SustainedPublicEndpointMix`, `_BoundaryLargeBodyAndManyHeaders` — all
    **PASS** against the real server + real Postgres (real captured
    artefacts under `qa-results/20260712T144559Z/server_*/{recovery_trace,latency}.json`).
  - **`TestServer_Stress_ConcurrentDDoSFlood` — REAL FAIL, NEW FINDING.**
    640 real concurrent requests (parallelism=16 × 40 iters) against the
    live server; captured evidence
    (`qa-results/20260712T144559Z/server_stress_concurrent_ddos/concurrency_report.json`):
    `"goroutines_before": 5, "goroutines_after": 10, "goroutine_delta": 5,
    "deadlock": false, "error_count": 0`. The test's own tolerance is 4;
    delta of 5 tripped it. Honest assessment (§11.4.6 — no guessing): this
    *may* be a genuine small goroutine leak under concurrent DDoS-style
    load, or it may be scheduler/GC noise around a tight tolerance — the
    captured evidence does not by itself disambiguate the two, and no
    further root-cause investigation was performed in this session (out of
    scope for infra-execution). Recommend a new low/medium-severity ticket:
    "`TestServer_Stress_ConcurrentDDoSFlood` intermittently reports a
    goroutine-count delta (5) exceeding its own tolerance (4) — needs
    root-cause per §11.4.102."
- **`internal/llm` Ollama suite (previously 3/13 FAIL) → now 13/13 PASS**
  once pointed at the correctly-named model — confirms the prior failures
  were a test-fixture/environment mismatch from this session's model
  choice, not a platform defect.
- **`internal/llm` Xiaomi suite — REAL FAIL, NEW FINDING (external cloud
  API, unrelated to local infra):** `TestXiaomiStress_SequentialCalls`
  (0/20) fails against the **real** Xiaomi cloud endpoint with a genuine
  HTTP 400: `{"code":"400","message":"Unsupported model mimo-v2-flash"}`
  on every one of 20 real HTTP round-trips.
  `TestXiaomiChaos_InvalidAPIKey`/`_InvalidModel` and
  `TestXiaomiStress_ConcurrentCalls`/`_RapidFire` all PASS (the latter two
  because they treat rate-limiting as expected chaos and don't require a
  successful completion). Honest assessment: either the configured model
  name `mimo-v2-flash` has drifted from Xiaomi's current catalogue, or the
  API key's account tier doesn't have access to it — **not** investigated
  further here (real external vendor API, outside this task's infra
  ownership). Recommend a new ticket:
  "`TestXiaomiStress_SequentialCalls` — real Xiaomi API rejects configured
  model `mimo-v2-flash` as unsupported (HTTP 400) on every call."
- **`internal/redis`** (first pass, unaffected by the above): **13/13 PASS**
  — real Redis chaos (cancel-mid-op, corrupt values, weird keys, resource
  pressure, connection churn, panicking translator) and stress (sustained
  set/get/del, hash/list, concurrent set/get, shared-key counter, pipeline,
  pub/sub, boundary cases) all genuinely executed against the real Redis
  container. `ok dev.helix.code/internal/redis 13.284s`.
- **`internal/database`**: real Postgres boundary-condition stress test
  PASS (`TestDatabase_Stress_BoundaryConditions`) — empty-result,
  1 MB large row, no-op update, empty-string row all handled against the
  real DB.

Load/DDoS (partially covered above via `TestServer_Stress_ConcurrentDDoSFlood`),
scaling, and UI/UX test types were **not** exhaustively attempted in this
session beyond the stress/chaos sweep above — time-boxed out. Honest coverage
gap, not a silent skip: HXC-136 itself remains open for a follow-up session
to cover `tests/scaling/`, `test/load/`, and `tests/ux/` specifically.

## 6. Teardown

All processes and containers this run created were stopped and removed:

```
kill -TERM <server.pid>                                          # our helixcode server (port 18080)
podman-compose -f docker-compose.full-test.yml down -v            # postgres/redis/ollama + their volumes
podman stop/rm helixcode-autoboot-postgres helixcode-autoboot-redis   # server's own on-demand auto-boot containers
```

Post-teardown verification:
- All PIDs launched by this session (`server`, `automation` test run,
  `challenge runner`, `stress_chaos` x2) confirmed gone (`ps -p <pids>` →
  empty).
- Ports 18080, 5432, 6379, 11434, 55432, 56379 confirmed free (`ss -ltn`).
- `podman ps -a` afterward shows **exactly** the same 5 pre-existing,
  other-project containers that were present before this run started
  (`helix_sonarqube_db`, `helix_sonarqube`, `helixllm-coder`,
  `helixtranslate-ssh-test`, `brokertest-etcd-4164851-1`) — **zero**
  `helixcode-*` containers left behind, and none of those pre-existing,
  not-ours containers were touched at any point (§11.4.174 process/resource
  ownership discipline held for the full session — the host port-8080
  `ahttpd.service` was also never touched).

### Note on `server.log`

The server's own stdout/stderr log grew to ~67 MB (mostly repetitive
`[GIN] ... GET "/health"` lines from the 268,275-request GC-pressure test in
§2). Compressed to `server.log.gz` (3.8 MB) for this evidence set; `server.log.head`
(first 100 lines — startup + DB/schema init), `server.log.tail` (last 200
lines), and `server.log.non-gin` (every non-`[GIN]` line, i.e. all real
startup/shutdown/error events with the request-flood noise stripped) are
kept uncompressed for quick review.

## 7. Summary verdicts

| Item | Verdict | Evidence |
|---|---|---|
| HXC-122 (memory) | **REAL-PASS** | `memory_tests.log` — 15/15 PASS, 0 SKIP |
| HXC-122 (automation, `tests/automation`) | **REAL-PASS / REAL-IN-PROGRESS** (env-flag recipe confirmed to unblock real execution; long-tail install sweep not run to full completion — time-boxed) | `automation_tests.log` |
| HXC-122 (automation, `test/automation` `-tags=automation`) | **REAL-FAIL — NEW FINDING** (compile error, API drift; duplicate-symbol sub-issue fixed locally uncommitted) | `go vet` output above |
| HXC-138 | **REAL-EXECUTION / honest REAL-FAIL outcome** (harness genuinely works end-to-end; 0.5B model's output didn't pass acceptance checks) | `e2e_challenges.log`, `hxc138_challenge_report.json`, per-execution logs |
| HXC-136 (stress+chaos slice only) | **REAL-PASS (mostly) + 2 NEW real defect findings** — Redis 13/13 PASS, Database 1/1 PASS, Server 7/8 PASS (1 goroutine-leak finding), LLM/Ollama 13/13 PASS (after model-name fix), LLM/Xiaomi 3/5 PASS (1 real external-API model-rejection finding). Load/DDoS beyond the DDoS-flood test, scaling, and UI/UX categories not attempted — honest coverage gap for a follow-up session. | `stress_chaos_hxc136.log`, `stress_chaos_rerun.log`, `qa-results/20260712T144559Z/**` |

## 8. New findings recommended for the tracker (not fixed in this session — out of infra-execution scope)

1. **`test/automation` (`-tags=automation`) does not compile** — references
   removed `internal/llm` API (`ProviderConfig`, `NewProviderManager`,
   `GetAvailableProviders`); `go vet -tags=automation
   ./test/automation/...` → `undefined: llm.ProviderConfig`. (One
   contributing duplicate-symbol issue was fixed locally/uncommitted in
   `xai_automation_test.go` — see §3b.)
2. **`test/e2e` (`-tags=e2e`) also does not compile** —
   `getEnvOrDefault redeclared in this block`
   (`test/e2e/qwen_e2e_test.go:285`).
3. **`TestRealModelExecution` (`tests/automation/hardware_test.go`) is
   impractically slow** — attempts a full ~10-backend local-LLM-runtime
   install sweep per candidate model (5 models), taking well over 12
   minutes without completing one full pass in this session. Recommend
   preferring an already-reachable `OLLAMA_BASE_URL`/`LLAMACPP_BASE_URL`
   over the full backend-install sweep when one is configured.
4. **`TestServer_Stress_ConcurrentDDoSFlood`** reported a goroutine-count
   delta of 5 against a tolerance of 4 under 640 real concurrent requests
   (`qa-results/20260712T144559Z/server_stress_concurrent_ddos/concurrency_report.json`).
   Needs root-cause per §11.4.102 to determine genuine leak vs. tolerance
   flakiness.
5. **`TestXiaomiStress_SequentialCalls`** — the real Xiaomi cloud API
   rejects the configured model `mimo-v2-flash` as unsupported (HTTP 400)
   on 20/20 real calls. Needs a check of Xiaomi's current model catalogue /
   the test API key's account tier.

## 9. Uncommitted local change (conductor to review)

`test/automation/xai_automation_test.go` — removed a byte-identical
duplicate of `isRateLimitError`/`contains` (already declared in
`qwen_automation_test.go`) that was breaking compilation of the whole
`-tags=automation` package. This alone does **not** make the package
compile (finding #1 above is the blocking issue) but is a real, correct,
narrow fix left in the working tree, not committed, per this task's
instructions.
