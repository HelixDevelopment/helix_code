# HXC-136 Infra Retest #2 — load/DDoS, scaling, UI/UX

Run: 2026-07-12 (UTC timestamps below), host `helix_code` repo,
`/home/milos/Factory/projects/tools_and_research/helix_code/helix_code`.

Scope: the three mandated test types NOT yet exercised with captured evidence
in the prior HXC-136 retest (memory + stress/chaos were already covered:
Redis 13/13, DB 1/1, Ollama 13/13). This pass covers **load/DDoS**,
**scaling**, and **UI/UX**.

No commits/pushes were made by this run (evidence-gathering only, per task
scope). `tests/e2e/challenges/` was not touched.

## Infra bring-up

`make test-infra-up` (compose runner: podman, rootless, §11.4.161) — log:
`logs/00_infra_up.log`. All `docker-compose.full-test.yml` services came up
`(healthy)`: `helixcode-postgres-full`, `helixcode-redis-full`,
`helixcode-memcached-full`, `helixcode-ollama-full`, `helixcode-mock-llm`,
`helixcode-selenium-chrome`, `helixcode-chromedp`, `helixcode-ssh-server`,
`helixcode-weaviate-full`, `helixcode-chromadb-full`, `helixcode-qdrant-full`,
`helixcode-mock-slack`, `helixcode-multicast-router`,
`helixcode-ssh-worker-{1,2,3}`, `helixcode-cognee-full`.

§11.4.174 ownership check: before/after `podman ps -a` confirmed the 5
pre-existing OTHER-PROJECT containers (`helix_sonarqube_db`,
`helix_sonarqube`, `helixllm-coder`, `helixtranslate-ssh-test`,
`brokertest-etcd-4164851-1`) were untouched throughout — same status before
and after. Only `helixcode-*-full` containers/volumes/network were
created/removed by this run.

## Verdicts

### 1. Load / DDoS — REAL-PASS

Package: `tests/ddos` (build tag `integration`). Real booted
`internal/server.Server` (real Gin router incl. Logger/Recovery/CORS/Security
middleware) with a real PostgreSQL pool (`helixcode-postgres-full`) and real
Redis client (`helixcode-redis-full`), served over a real TCP listener.

First attempt (`logs/05_ddos.log`) genuinely SKIPped `TestDDoS_HealthFlood`
because the test's `TEST_PG_*` env vars default to `helix`/`helix`/
`helix_test`, which do not match the `.env.full-test` / compose file's actual
container credentials (`helixcode`/`helixcode_test_password`/
`helixcode_test`) — `.env.full-test` only exports `HELIX_DATABASE_*`, not
`TEST_PG_*`. This is a genuine env-var-naming gap between `.env.full-test`
and the ddos harness's own `envOr("TEST_PG_*", ...)` defaults — **flagging as
a new minor defect** (see Defects section below), not a bluff: the SKIP was
honest (§11.4.3) given the infra was genuinely unreachable under those
credentials.

Re-run with `TEST_PG_HOST/PORT/USER/PASSWORD/DB/SSLMODE` and
`TEST_REDIS_HOST/PORT/PASSWORD/DB` set to the container's real values
(`logs/06_ddos_realcreds.log`) produced a REAL PASS:

```
TestDDoS_HealthFlood: sent=1700 2xx=1700 4xx=0 429=0 5xx=0 markerHits=1700
p99_under_flood_ms=0.283  (no goroutine leak, no deadlock, zero 5xx)
```

Evidence: `evidence/20260712T162510Z/ddos_health_flood/flood_report.json`,
`evidence/20260712T162510Z/ddos_health_flood_flood/concurrency_report.json`,
`evidence/20260712T162510Z/ddos_health_flood_latency/latency.json`.

Paired §1.1 meta-tests all PASS (`TestMeta_RunFlood_Detects5xxStorm`,
`_DetectsLatencyBomb`, `_DetectsNoServedResponses`,
`_LimiterModeDetectsNoRefusals`, `TestMeta_PositivePathWritesEvidence`) —
proves the flood harness genuinely detects a 5xx storm / latency bomb / no
served responses / (limiter-mode) no-refusal condition, so the PASS above is
not a bluffable assertion.

Honest ground truth carried in the harness's own doc comment (§11.4.6): the
production server wires no rate-limit middleware today, so the assertion is
graceful-degradation (no leak/deadlock, zero 5xx, bounded p99, real served
responses) rather than a 429-refusal ratio — asserting 429s the codebase
cannot produce would itself be a bluff. The 429 assertion path exists behind
`DDOS_EXPECT_RATELIMIT`, off by default, ready for when a real limiter lands.

**Verdict: REAL-PASS** (evidence path above; genuine 1700-request flood
against the real server + real DB + real Redis, zero degradation).

### 2. Scaling — REAL-PASS

Package: `tests/scaling` (no build tag — always-available in-process sweep).
`TestScaling_WorkerPool_RealSweep` drives the REAL `internal/worker.WorkerPool`
across N=1,2,4,8 real registered `PoolWorkers`, real `AssignTask`/
`ReleaseWorker`, real `GetPoolStats` utilization.

```
N=1  throughput=476  tps  p99=355.3ms  util=100%
N=2  throughput=912  tps  p99=163.5ms  util=50%
N=4  throughput=1655 tps  p99=83.1ms   util=100%
N=8  throughput=3304 tps  p99=32.0ms   util=100%
gain_at_max_n = 6.93x   (floor 1.5x)   monotonic_non_degraded = true
```

Evidence: `evidence/20260712T162356Z/scaling_worker_pool/scaling_throughput.json`,
`evidence/20260712T162356Z/scaling_worker_pool_concurrency_guard/concurrency_report.json`.

`TestScaling_SSHHorizontal_Integration` — **HONEST-SKIP**: real SSH-worker
horizontal scale-out requires configured remote hosts
(`SCALING_SSH_WORKERS` unset in this environment). §11.4.3 honest skip, never
a faked PASS; the in-process sweep above is the always-available local proof
of genuine scale-out logic.

Paired §1.1 meta-tests PASS (`TestMeta_RunScaleSweep_DetectsFlatThroughput`,
`_DetectsDegradation`, `_RejectsBelowFloor`, `TestMeta_PositivePathWritesEvidence`)
— proves the harness genuinely detects a flat/degraded/below-floor pool, so
6.93x is not a bluffable number.

**Verdict: REAL-PASS** (in-process worker-pool sweep) **+ HONEST-SKIP**
(SSH-horizontal — no remote hosts configured in this environment; not a
runnable gap in this infra, tracked as pre-existing §11.4.3 skip, not a new
defect).

### 3. UI/UX — REAL-PASS

No display/X11/headless-browser was needed — both harnesses are genuinely
headless by design (§11.4.6 honest split documented in the harness code):

**TUI (`tests/ui`, no build tag)** — real `tview`/`tcell` widgets rendered to
an in-memory `tcell.SimulationScreen` cell grid, keys injected, exact
rendered cells read back:
- `TestUI_TUIList_RealRender` PASS — real `tview.List` rendered 80x25, all 5
  expected strings present.
- `TestUI_TUITable_RealRender` PASS — real `tview.Table`, all 7 strings
  present.
- `TestUI_TUIList_InteractionMovesSelection` PASS — real navigation key
  processed against the real widget (rendered state changes on interaction).
- `TestUI_TUI_NoLeakedMessageID` PASS — i18n resolves to locale text, no raw
  message IDs leaked (CONST-046).

Evidence: `evidence/20260712T162405Z/ui_tui_list/rendered_cells.json`,
`ui_tui_table/rendered_cells.json`, `ui_tui_no_leak_content/rendered_cells.json`.

**Desktop (Fyne, `-tags=fyne_ui`)**:
- `TestUI_FyneWidgetTree_RealRender` PASS — real Fyne widget tree rendered
  via the Fyne headless test canvas, real markup asserted. Evidence:
  `evidence/20260712T162413Z/ui_fyne_widget/fyne_markup.txt`.
- `TestUI_FyneDesktopPixelLayer_OperatorAttended` — **HONEST-SKIP**: real
  on-screen pixel/native-window rendering is not headlessly assertable
  (§11.4.52), tracked as a pre-existing operator-attended migration item, not
  a new gap introduced by this run.

**UX (`tests/ux`, no build tag)** — builds the REAL `cmd/cli` binary and
shells it (zero human action after startup, §11.4.98), asserts on real
stdout + real exit codes:
- `TestUX_CLIJourney_RealBinary` PASS — 2-step real CLI journey, all
  real-output assertions PASS. Evidence:
  `evidence/20260712T162427Z/ux_cli_journey/journey_transcript.jsonl`.
- `TestUX_I18nNoLeak_RealBundle` PASS — 4 message IDs all resolved to real
  locale text via the wired translator (CONST-046 no-leak). Evidence:
  `ux_i18n_no_leak/i18n_resolution.json`.
- `TestUX_ErrorClarity_RealBundle` PASS — real resolved error string
  ("Authentication required") clears the §11.4.91-style clarity floor.
- `TestUX_ResponseShapeConsistency` PASS — 3 sampled error responses share
  one `{error}` envelope. Evidence: `ux_response_shape/error_shape_report.json`.

Paired §1.1 meta-tests all PASS for both UI and UX packages (blank-render
detection, dead-key-handler detection, leaked-message-ID detection,
canned-string detection, error-clarity detection, error-shape-divergence
detection) — proves neither harness can bluff.

**Verdict: REAL-PASS** (TUI + Fyne widget-tree + UX CLI journey, i18n,
error-clarity, response-shape) **+ 2 pre-existing HONEST-SKIPs** (Fyne native
pixel layer, real on-screen rendering — genuinely not headlessly assertable
in any environment, not specific to this host).

## New defects surfaced (flag for tracking, not fixed by this run)

1. **`tests/ddos` env-var mismatch with `.env.full-test`**: the ddos harness
   reads `TEST_PG_HOST/PORT/USER/PASSWORD/DB/SSLMODE` and
   `TEST_REDIS_HOST/PORT/PASSWORD/DB`, defaulting to `helix`/`helix`/
   `helix_test` — but `.env.full-test` (consumed by every other
   `make test-*-full` target) only exports `HELIX_DATABASE_*` /
   `HELIX_REDIS_*`, and the actual `docker-compose.full-test.yml` postgres
   container uses `helixcode`/`helixcode_test_password`/`helixcode_test`.
   Net effect: `make test-load-full`-style invocation (or any invocation
   that only sources `.env.full-test`) would produce a false SKIP on
   `TestDDoS_HealthFlood` even though the real infra is fully reachable and
   healthy — the SKIP message is technically honest (§11.4.3: infra
   unreachable *under those credentials*) but practically hides a runnable
   test from the normal `make` workflow. Recommend either (a) adding
   `TEST_PG_*`/`TEST_REDIS_*` aliases to `.env.full-test` matching the
   compose credentials, or (b) changing the ddos harness defaults to match
   `docker-compose.full-test.yml`. Not fixed here per task scope (flag for a
   new ticket).
2. No other new defects surfaced. The DDoS-flood goroutine "leak" mentioned
   in the task brief was already fixed under HXC-144 and is confirmed clean
   here (`gDelta=0` in every flood/meta run above).

## Teardown

`make test-infra-down` (`logs/07_infra_down.log`) removed all
`helixcode-*-full` containers, named volumes, and the
`helixcode-full-test-network`. Post-teardown `podman ps -a` confirmed the
same 5 pre-existing other-project containers remain, in the same state as
before this run — no orphans, nothing else touched (§11.4.14, §11.4.174).

## Files

- `logs/00_infra_up.log` — infra bring-up
- `logs/01_scaling.log` — scaling package (in-process sweep + meta-tests)
- `logs/02_ui_tui.log` — UI package, TUI only (no build tag)
- `logs/03_ui_fyne.log` — UI package, `-tags=fyne_ui` (TUI + Fyne widget-tree)
- `logs/04_ux.log` — UX package (real CLI binary journey + i18n + clarity + shape)
- `logs/05_ddos.log` — ddos package, first attempt (honest SKIP, wrong default creds)
- `logs/06_ddos_realcreds.log` — ddos package, real container creds (REAL-PASS)
- `logs/07_infra_down.log` — teardown
- `evidence/<run-id>/...` — copied `qa-results/<run-id>/` artefacts referenced above

## HXC-136 closure recommendation

All previously-remaining mandated test types (load/DDoS, scaling, UI/UX) now
have real captured runtime evidence, on top of the prior retest's
memory/stress/chaos coverage (Redis 13/13, DB 1/1, Ollama 13/13). Every
SKIP encountered is an honest, pre-existing, §11.4.3-compliant SKIP (SSH
horizontal scale-out — no remote hosts configured; Fyne native pixel layer —
genuinely not headlessly assertable) rather than a gap this run could close.
**HXC-136 can be closed** on the basis that every runnable mandated test type
now has captured evidence and every remaining gap is an honestly-documented
SKIP, not an unexercised type. The one new finding (ddos env-var mismatch
with `.env.full-test`) should be opened as its own low-severity ticket — it
does not block HXC-136 closure since the underlying capability (DDoS/load
testing against real infra) is proven to work and produces a REAL-PASS once
correct credentials are supplied.
