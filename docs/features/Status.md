# HelixCode Feature Status

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-15 |
| Last modified | 2026-06-15 |
| Status | active (population in progress) |
| Status summary | docs/features/Status_Summary.md |
| Continuation | docs/CONTINUATION.md |

Authoritative, in-depth inventory of **every** HelixCode feature across all
services, infrastructure, and client applications — including capabilities ported
from the `cli_agents/` reference catalogue — with per-feature status across every
dimension the operator mandate (2026-06-15) requires. Kept in sync via the
`docs_chain` engine (§11.4.106, `submodules/docs_chain`) and the Status-doc
covenant (§11.4.45 / §11.4.53 / §11.4.56 / §11.4.57 / CONST-063 / CONST-064).

> **Anti-bluff (CONST-035 / §11.4.83 / §11.4.107):** a feature is marked
> video-confirmed (`📹 yes`) ONLY when a real recorded scenario in
> `/Volumes/T7/Downloads/Recordings` shows it working end-to-end with a strong
> real LLM and that recording has been analyzed. No false "yes". An un-recorded
> or un-analyzed feature is honestly `📹 no` / `📹 pending`, never bluffed green.

> **Related feature ledger (§11.4.153, added 2026-07-11):** the HelixLLM
> full-extension programme (Lane-A/Lane-B serving, VRAM broker, vision/image/
> video-gen, RAG, dual-wire protocol, MCP gateway, A2A, HelixQA extension
> banks — 43 features) has its own detailed, more granular per-feature ledger
> at `docs/features/helixllm-status.md` (+ `helixllm-status_summary.md`,
> four-format `.html`/`.pdf`/`.docx` exports). This document (`Status.md`) is
> the whole-platform inventory across all services/apps/submodules; the
> `helixllm-status.md` ledger is narrower in scope (one submodule's
> extension programme) but deeper in per-feature evidence-path detail. See
> the `helix_llm` row in [Owned submodules](#) below for this document's
> summary-level entry and its cross-reference note.

## Table of contents

- [Status dimensions (legend)](#status-dimensions-legend)
- [Population progress](#population-progress)
- [Feature inventory](#feature-inventory)
- [Inventory sources](#inventory-sources)

## Status dimensions (legend)

Each feature row carries:

| Column | Meaning | Values |
|---|---|---|
| **Area** | service / infrastructure / application(client) / submodule | — |
| **Component** | package / tool / app / submodule it lives in | — |
| **Feature** | the discrete user-or-system capability | — |
| **Dev** | implementation status | `done` / `partial` / `stub` / `absent` |
| **Wired** | reachable from a shipped flow (not dead code) | `yes` / `no` / `partial` |
| **Real-use** | genuinely usable by an end user | `yes` / `no` / `unknown` |
| **Tests** | automated coverage | `unit` / `integ` / `e2e` / `none` (combinable) |
| **V&V** | captured runtime evidence (§11.4.5/§11.4.69) | `yes(path)` / `no` |
| **📹 Video** | recorded real scenario + analyzed (§11.4.83/§11.4.107) | `yes(path)` / `pending` / `no` / `n/a` |
| **Analysis** | comprehensive recording analysis performed | `yes` / `no` |
| **Origin** | native / `ported:<cli_agent>` | — |
| **Overall** | rollup | `confirmed` / `working-untaped` / `partial` / `gap` |

## Population progress

This document is populated by background inventory subagents fanning out across
the codebase (§11.4.70 subagent-driven, §11.4.103 parallel-streams). Coverage is
reported honestly — `confirmed` rows require a real analyzed video; everything
else is marked truthfully. Population is an ongoing program, NOT a one-shot claim.

| Slice | Scope | Status |
|---|---|---|
| internal services + infra | `helix_code/internal/*` (72 pkgs) | inventory dispatched |
| cmd tools + client apps | `helix_code/cmd/*` (21) + `applications/*` (cli/tui/web/desktop/mobile) | inventory dispatched |
| owned submodules | `submodules/*` (70+) | inventory dispatched |
| ported cli_agents capabilities | `cli_agents/*` → HelixCode | inventory dispatched |

## Sources verified 2026-06-22: helix_code/internal/* , helix_code/cmd/* , helix_code/applications/* , submodules/* , cli_agents/*

This document is REPO-STATE-DERIVED (per §11.4.99 the "sources" are the
cross-referenced repo trees, following the `docs/ARCHITECTURE.md` precedent — no
external service is documented here). Cross-referenced the population-progress
counts against the live tree on 2026-06-22 (`ls -d <tree>/*/` counts):
- **`helix_code/internal/*` = 72 packages — CONFIRMED** (matches "72 pkgs").
- **`helix_code/cmd/*` = 21 entries — CONFIRMED** (`ls helix_code/cmd/` = 21
  total entries: 11 tool subdirs + 10 top-level `.go`/`_test.go` files; the
  "(21)" count is entries, not subdirs-only).
- **`helix_code/applications/*` = 6 dirs** (cli/tui/web/desktop/mobile families
  present) — consistent.
- **Negative finding (minor count drift).** `submodules/*` live count = **67**
  directories, not "70+" as the row states; `cli_agents/*` live count = **50**
  directories. These slice labels slightly overstate the live directory counts
  and should be reconciled to 67 / 50 on the next revision (the inventory bodies
  themselves enumerate the actual present components).

## Feature inventory

_Aggregated from docs/features/inventory/*.md by scripts/generate_features_status.sh (docs_chain §11.4.106)._


## CLI / TUI / Web / Desktop / Mobile clients + cmd tools

Inventory slice for `helix_code/cmd/*` (cmd tools) and `helix_code/applications/*` +
`helix_code/web/*` (client applications). Assessed from source evidence per the
anti-bluff covenant (CONST-035 / §11.4.107). Rows marked `confirmed` cite
**durable committed §11.4.83 evidence** — the `docs/qa/HXC-108_*_evidence.md`
curated records (md5-pinned, OCR-content-validated) and, for Android, committed
`docs/qa/HXC-108_android/` mp4+png — NOT the rotatable git-ignored raw corpus
(`/Volumes/T7/Downloads/Recordings/`), whose §11.4.154 rotation orphaned the
prior `-20260615/16` citations (HXC-107 audit). Rows without a durable analyzed
recording stay honestly `working-untaped` / `partial` / `gap`.

Recordability per client (for the conductor): **CLI, TUI, Web = feasible now**
(terminal + headless HTTP). **Desktop (Fyne) = host-display required**. **Mobile:**
Aurora OS / Harmony OS are Go/Fyne desktop-class binaries (recordable on a Linux
display); **Android / aurora_os HAP / harmony_os HAP need device/emulator**;
**iOS needs a built app (no Xcode project present → not buildable yet)**.

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| application(cli) | cmd/cli REPL | Plain-text prompt → real LLM (streaming via provider.GenerateStream) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/generate` real LLM generation (BLUFF-001 resolved) | done | yes | yes | unit,e2e | no | yes(docs/qa/HXC-108_cli_recordings_evidence.md §3 — real DeepSeek generate, OCR-validated, md5-pinned recording) | yes | native | confirmed |
| application(cli) | cmd/cli REPL | `/models` list models via providerManager.GetProviders (BLUFF-002 resolved) | done | yes | yes | unit | no | yes(docs/qa/HXC-108_cli_recordings_evidence.md §2 — real provider catalog, OCR-validated) | yes | native | confirmed |
| application(cli) | cmd/cli REPL | `/workers` list workers | done | yes | yes | unit | no | yes(docs/qa/HXC-108_cli_recordings_evidence.md §5 — real worker stats, OCR-validated) | yes | native | confirmed |
| application(cli) | cmd/cli REPL | `/health` health check | done | yes | yes | unit | no | yes(docs/qa/HXC-108_cli_recordings_evidence.md §4 — real health states, OCR-validated) | yes | native | confirmed |
| application(cli) | cmd/cli REPL | `/diff [ref]` real git diff (os/exec) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/debate <prompt>` DebateOrchestrator over real provider | done | yes | yes | unit,e2e | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/specify <request>` HelixSpecifier Specify phase over real provider | done | yes | yes | unit,e2e | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/checkpoint [create/list/restore]` workspace snapshots | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/undo` revert last action | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/clear` `/reset` clear conversation history | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | `/help` help; `/exit` `/quit` exit | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | @-file mentions (real os.ReadFile context attach) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli REPL | Context-window tracking + generation stats | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `commands` | list / show / run / reload markdown commands | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `hooks` | list / enable / disable / validate / test git hooks | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `lsp` | status / list-servers / restart / stop LSP servers | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `mcp` | add / remove / list / test / auth / logs MCP servers | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `permissions` | list / add / remove / check tool permissions | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `sessions` | list / show / delete sessions | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/cli `skills` | list / show / invoke / reload skills | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | cmd/server | HTTP/gRPC/WebSocket server boot (bin/helixcode) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(web) | web/frontend | LLM generate console (POST /api/v1/llm/generate) | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | yes(docs/qa/HXC-108_web_evidence.md WEB-1 — headless-Chrome journey: real login→JWT→real DeepSeek generate, DOM-asserted + OCR-validated) | yes | native | confirmed |
| application(web) | web/frontend | LLM streaming console (SSE, POST /api/v1/llm/stream) | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | no | no | native | working-untaped |
| application(web) | web/frontend | Specify phase form (POST /api/v1/specify) | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | no | no | native | working-untaped |
| application(web) | web/frontend | Response/metadata rendering (no client simulation) | done | yes | yes | e2e | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/llm/generate real provider.Generate | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | yes(docs/qa/HXC-108_tui_server_recordings_evidence.md SRV-1 — authenticated /api/v1/llm/generate real DeepSeek, OCR-validated; corroborated by HXC-108_web_evidence.md WEB-1) | yes | native | confirmed |
| service | internal/server (HTTP API) | /api/v1/llm/stream real provider.GenerateStream (SSE) | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/specify real speckit Specify phase | done | yes | yes | e2e | yes(docs/qa/web-llm-e2e-20260615/) | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/llm/providers, /llm/models list | done | yes | yes | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /api/v1/auth register / login / logout / refresh (JWT) | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/users me get/update/delete | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/workers CRUD + heartbeat + metrics | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/tasks CRUD + assign/start/complete/fail/retry/checkpoint | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/projects CRUD + sessions + planning/building/testing/refactoring workflows | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/sessions CRUD | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /api/v1/system stats + status | done | yes | yes | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /api/v1/memory systems + stats | done | yes | yes | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /api/v1/qa session start/list/status/report/screenshot/cancel | done | yes | unknown | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /api/v1/screenshot engines + capture | done | yes | unknown | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /ws MCP WebSocket bridge | done | yes | yes | none | no | no | no | native | partial |
| service | internal/server (HTTP API) | /health, /api/v1/health, /metrics, /api/v1/server/info | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server (HTTP API) | /debug/pprof/* (opt-in profiling) | done | yes | yes | none | no | no | no | native | partial |
| application(tui) | applications/terminal_ui | LLM chat (real provider, verifier-driven model discovery) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(tui) | applications/terminal_ui | Dashboard / Tasks / Workers / Projects / Sessions panels (real managers) | done | yes | yes | unit | no | yes(docs/qa/HXC-108_tui_views_evidence.md — 6 per-view tmux-driven recordings (Dashboard/Tasks/Workers/Projects/Sessions/QA), real seeded content OCR-confirmed; TUI-1 launch in docs/qa/HXC-108_tui_server_recordings_evidence.md) | yes | native | confirmed |
| application(tui) | applications/terminal_ui | Sidebar nav + key bindings (d/t/w/p/s/l/q/c) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(tui) | applications/terminal_ui | Skill dispatcher + tool registry (git/fs/grep/LSP/MCP, graceful-nil) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(tui) | applications/terminal_ui | HelixMemory durable cross-session store (SQLite, default-on) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(tui) | applications/terminal_ui | Status bar (DB/Redis/LLM status, context %), notifications, themes, i18n | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(tui) | applications/terminal_ui | QA engine panel (helixqa.Engine wired) | partial | yes | unknown | unit | no | no | no | native | partial |
| application(desktop) | applications/desktop (Fyne) | LLM chat tab (real provider, verifier-driven models) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(desktop) | applications/desktop (Fyne) | Dashboard / Tasks / Workers / Projects / Sessions tabs (real managers) | done | yes | yes | unit | no | yes(docs/qa/HXC-108_desktopgui_features_evidence.md GUI-1..6 — in-process Fyne software-render of all six tabs via production create<Tab>() + real managers + real seeded data, OCR-validated) | yes | native | confirmed |
| application(desktop) | applications/desktop (Fyne) | Settings tab (theme, server config, shortcuts; desktop.yaml) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(desktop) | applications/desktop (Fyne) | Agentic tools + skills/plugins wiring (graceful-nil) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(desktop) | applications/desktop (Fyne) | NoGUI build mode (-tags nogui CLI fallback) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(desktop) | applications/desktop (Fyne) | TUI-parity wiring (parity_wiring_test.go) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(mobile) | applications/aurora_os (Go/Fyne) | Full multi-page GUI: dashboard/projects/sessions/tasks/LLM/workers/system | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(mobile) | applications/aurora_os (Go/Fyne) | NoGUI CLI mode (cobra) + theme system | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(mobile) | applications/harmony_os (Go/Fyne) | 10-tab GUI incl. distributed engine + multi-device scheduling | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| application(mobile) | applications/harmony_os (Go/Fyne) | NoGUI CLI + interactive shell + theme system | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(mobile) | applications/android (Kotlin) | Connect + task list (RecyclerView) over Go mobile-core bridge | partial | partial | yes | none | no | yes(docs/qa/HXC-108_android/helixcode-android-client.mp4 + helixcode-android-MainActivity.png [committed §11.4.83] — real MainActivity on Android 14 emulator, dumpsys ResumedActivity confirmed; see docs/qa/HXC-108_android_evidence.md) | yes | native | confirmed |
| application(mobile) | applications/android (Kotlin) | Models / settings / notifications / theme UI | stub | no | no | none | no | no | no | native | gap |
| application(mobile) | applications/ios (Swift) | Connect + task list (UITableView) over Go mobile-core bridge | partial | partial | yes | none | no | yes(docs/qa/HXC-108_ios_evidence.md — real app on iOS Simulator rendering live Go-core data "Go core OK — themes: 3, tasks: 2", Vision-OCR validated, liveness verified) | yes | native | confirmed |
| application(mobile) | applications/ios (Swift) | Models / settings / notifications / theme UI | stub | no | no | none | no | no | no | native | gap |
| application(cli) | cmd/helix_config | Interactive provider/credential config wizard → YAML/.env | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/config_test | Config + provider-credential validator | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/i18n | i18n bundle/translator tooling (library, no main) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/infrastructure | Container / k8s / registry readiness checks | done | yes | unknown | unit | no | no | no | native | partial |
| application(cli) | cmd/performance_optimization | pprof profiling + bottleneck analysis + benchmarks | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/security_test | Security test harness (PoC execution + verification) | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/security_fix | Finding ingestion + policy-driven fix + re-scan validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/security_fix_standalone | Batch parallel security fix + audit-trail reporting | done | yes | yes | unit | no | no | no | native | working-untaped |
| application(cli) | cmd/security_scan | AST/pattern code scan + leak detect + SARIF report | done | yes | unknown | none | no | no | no | native | gap |

### Honesty notes (anti-bluff)

- **`cmd/security_scan` has ZERO tests** (only `main.go`) — marked `Tests=none`, `Overall=gap`. Untested scanner = bluff risk per CONST-048/CONST-050.
- **iOS / Android are scaffolds**: real source (JSON parse, list bind, Go mobile-core bridge) but single-screen, hardcoded localhost test server, **no Xcode project / no Gradle+manifest** → not buildable, not recordable. `Real-use=no`, `Overall=gap`.
- **Aurora OS / Harmony OS are genuine Go/Fyne apps** (buildable via `make aurora-os` / `make harmony-os`, comprehensive unit+integration tests) — but integration features need real PostgreSQL/Redis/LLM backends, and full HAP/multi-device exercise needs the actual OS environment.
- **Web LLM endpoints** (`/llm/generate`, `/llm/stream`, `/specify`) have real Ollama-backed e2e tests (`tests/integration/{llm_generate,llm_stream,specify_server}_e2e_test.go`, build tag `integration`, honest SKIP-OK when Ollama unreachable) → `Tests=e2e`, `V&V=yes(docs/qa/web-llm-e2e-20260615/)`. The 60+ CRUD/auth/workflow endpoints are real but tested at the manager/service layer, not at the HTTP-transport layer → `Tests=integ`/`none` honestly.
- **`confirmed` rows cite durable committed evidence only** (§11.4.83) — the `docs/qa/HXC-108_*_evidence.md` curated records (each md5-pins + OCR-content-validates its recording via a self-validated golden-good/golden-bad analyzer per §11.4.107(10)) plus committed `docs/qa/HXC-108_android/` artifacts. After the HXC-107 audit found the `📹 Video` column citing the rotatable raw corpus (orphaned by §11.4.154 rotation), citations were re-anchored to these durable docs. Rows without such an analyzed recording remain `working-untaped` (real + tested, no durable video), `partial` (thin/unverified coverage), or `gap` (scaffold or untested) — the `feature_video_evidence_gate.sh` gate (§11.4.86) mechanically fails if any cited path goes missing.

**Feature count: 67 rows** (CLI REPL 14, CLI subcommand groups 7, server boot 1, web frontend 4, HTTP API groups 18, TUI 7, desktop 6, mobile 8, other cmd tools 11 + security_scan 1).

## Sources verified 2026-06-22: helix_code/cmd/* , helix_code/applications/* , helix_code/web/* , helix_code/tests/integration/*_e2e_test.go

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
Cross-referenced against the live tree on 2026-06-22:
- **`helix_code/cmd/*` = 21 entries** (11 tool subdirs incl. `cli`, `server`,
  `helix_config`, `security_scan`, `performance_optimization`, etc.) — the
  "other cmd tools 11 + security_scan 1" tally matches the subdir set.
- **`helix_code/applications/*` = 6 dirs** (cli / tui / web / desktop / mobile
  families) — consistent with the per-client recordability matrix.
- **Web LLM e2e tests confirmed present:** `helix_code/tests/integration/{llm_generate,llm_stream,specify_server}_e2e_test.go`
  exist (the doc's `Tests=e2e` / `V&V=yes(docs/qa/web-llm-e2e-20260615/)` claim is
  backed by real files, not asserted).
- **No external-service version claims in this doc** → nothing to staleness-check;
  the row data is structural evidence about HelixCode's own code.


## Deepened inventory (round 2)

Round-2 deepening of packages flagged shallow/partial/unknown in round 1
(`internal_services.md` coverage notes + the owned-submodule umbrella rows).
Each previously-thin entry is enumerated into discrete features in the SAME
12-column schema as `internal_services.md` (legend in `docs/features/Status.md`
§ "Status dimensions"). Assessed from real code evidence (impl reality, wiring,
`*_test.go` presence) per CONST-035 / §11.4.107 anti-bluff.

> **Anti-bluff caveats (read before trusting a row).**
> - `📹 Video` is **`no` for every row** — no analyzed recording exists; the
>   conductor owns video confirmation. `Overall` is therefore **never
>   `confirmed`**; honest rollups are `working-untaped` / `partial` / `gap`.
> - `Origin` is `native` (all own-org code).
> - **Round-1 stub claims corrected here from direct source reads** (a round-2
>   exploration subagent reported these as stubs WITHOUT reading the code —
>   verified false): `workspace` docker `Stop`/`Remove` are real
>   (`docker stop` / `docker rm -f`, `manager.go:21-37`); `voice` recorder
>   `Stop` is real (`Process.Signal(os.Interrupt)` then `Kill()`,
>   `recorder.go:63-80`); `roocode` `CodeReviewer.Review` is real (reads the
>   file + scans for TODO/issue patterns, `reviewer.go:16-30`). They are
>   `done`/`partial` (impl real) but `Real-use=unknown` (shipped-flow wiring
>   unconfirmed), not stubs.
> - Submodule-side `Wired` follows `internal_services.md`'s wiring model:
>   `helix_agent` is imported directly (`Wired=yes` where HelixCode consumes it),
>   transitively-reachable own-org deps are `Wired=partial`, repo-present-but-not-
>   in-build-graph are `Wired=no`. `helix_specifier`/`helix_qa` are imported by
>   module path (`Wired=yes`); `panoptic` is a Challenge/recording submodule not
>   in HelixCode's Go build graph (`Wired=no`).
> - Submodule `Tests` reflects `*_test.go` presence in/near the package; it is
>   NOT a coverage-percentage claim. README-cited coverage figures
>   (helix_agent "65.6%", panoptic "78%") are NOT used as evidence — unverified.

### Internal packages (round-1 `partial`/`unknown` → deepened)

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| infrastructure | internal/clientcore | agentic tool registry wiring (git/fs/LSP/MCP, live-built) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | LSP read-only diagnostics tool wiring | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | MCP config-merge + server startup + tool registration | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | tool-loop system-prompt generation (from registry names) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | tool-trace adapter (agent → ensembleui) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | skills + plugins loader (graceful-on-failure) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | verifier adapter wiring (CONST-036/037 single-source) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | env-provider registration (cloud API keys, zero-hardcode) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | HelixAgent provider registration (when server reachable) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | verifier bridge config wrapper | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | VerifyModel real HTTP call to verifier service | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | test request/result marshal+unmarshal | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/checkpoint | git-backed snapshot (stash/write-tree/read-tree) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | file-copy fallback backend (real os.Read/Write) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint create (capture) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint restore (undo, writes real bytes) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | metadata persistence (label/timestamp JSON) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | multi-backend auto-selection (git else files) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/ensembleui | tool-trace formatting (text/markdown render) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | metadata extraction helpers (int/string/slice) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | multi-model response assembly | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | per-model token-usage aggregation | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | Unit interface (task contract) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | queue-based scheduler (enqueue + dispatch) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | serial FIFO execution (single worker) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | context-cancellation propagation to Execute | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | workspace create (project bootstrap) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | docker runner start/stop/remove/list (real os/exec) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | workspace status tracking (enum + DB persist) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | CLI tools (create/delete/list registration) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | i18n translator seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/workspace | auto-cleanup / TTL teardown | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/voice | audio device detection (arecord/sox/parec probe) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | audio capture start (real exec.Command) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | audio capture stop (Signal(Interrupt)→Kill) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | Whisper API transcription (real multipart HTTP POST) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | WAV validation before transcribe | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | local whisper.cpp fallback transcription | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | in-memory conversation store (create/add-message) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | task delegation (TaskSpec build; no real dispatch) | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | code generation (Generate/Bootstrap scaffold) | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | code review (file read + TODO/issue scan) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | i18n translator seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | OpenTelemetry provider bootstrap (OTLP exporter) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | agent instrumentation (spans + iteration counter) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | LLM instrumentation (traced-provider wrapper) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | tool instrumentation (call spans) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | config-from-env (OTEL_* vars + exporter-kind resolve) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | flexible timestamp parse (Unix + RFC3339) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | adapter facade (cache+health+client, IsEnabled/Reachable) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | two-tier cache (TTL + Redis) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | health monitor (background liveness goroutine) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | background poller (interval model-sync) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | bootstrap (embedded-or-remote stack init) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | embedded verifier server (fallback) | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | fallback model list (offline mode) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | real-server integration test (live LLMsVerifier) | done | yes | unknown | integ | no | no | no | native | working-untaped |
| service | internal/worker | consensus manager (Raft state machine, election/heartbeat) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | vote-transport abstraction (decoupled from wire) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | single-node fallback (nil-transport safe step-down) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker isolation: sandbox dir create (0750) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | worker isolation: resource limits (cgroup writes) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | SSH client pool (persistent connection reuse) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | consensus stress + chaos tests (concurrent votes, SIGKILL) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker-pool isolation stress test | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | Gin server + JWT auth middleware | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | project list/register handlers (user-from-context) | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server | LLM generate endpoint (real provider stream) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | LLM model-list endpoint (verifier-queried) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | worker/task/project CRUD handlers | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server | QA session start handler (helix_qa integration) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/server | specify endpoint (spec from prompt) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | stats / pprof / error-categorize (400-vs-500) helpers | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | i18n translator seam (context-aware) | done | yes | yes | unit | no | no | no | native | working-untaped |

### Owned submodules (round-1 umbrella rows → deepened principal packages)

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| submodule | helix_agent | internal/llm multi-provider interface (Claude/DeepSeek/Gemini/Mistral/Qwen/xAI/OpenRouter/Ollama/llama.cpp/Bedrock/Azure) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/ensemble multi-model debate/fusion orchestrator | done | yes | yes | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/provider+providers per-provider abstraction layer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/verifier LLMsVerifier single-source-of-truth | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/server HTTP(Gin)+WebSocket API (completions/ensemble/stream) | done | yes | unknown | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/handlers per-route request/response handlers | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/database PostgreSQL persistence (pgx/v5, migrations) | done | partial | unknown | integ | no | no | no | native | partial |
| submodule | helix_agent | internal/redis caching layer (go-redis/v9) | done | partial | unknown | integ | no | no | no | native | partial |
| submodule | helix_agent | internal/auth JWT + bcrypt/argon2 + OAuth2 | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_agent | internal/security PII redaction + guardrails bridge | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/streaming SSE/WebSocket token-level streaming | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/cache semantic (embeddings dedup) caching | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/knowledge RAG / vector-DB (zep) bridge | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/monitoring Prometheus metrics | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/logging structured JSON + gRPC tracing | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/mcp Model Context Protocol server/client | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/skills+plugins hot-reload plugin system | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/analytics usage telemetry + warehouse export | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | pkg/sdk Go + Python client SDKs | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/guardrails content guardrail engine (severity rules) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/pii PII detect+redact (email/phone/SSN/CC-Luhn/IPv4) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/content composable filter chains (length/pattern/keyword) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/policy rule-based policy enforce (allow/deny/audit) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/scanner vuln-scan interface + report aggregation | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/headers HTTP security headers middleware (CSP/HSTS/...) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/securestorage AES-256-GCM + Argon2id key derivation | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/e2ee end-to-end encryption (ChaCha20-Poly1305) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/ssrf SSRF prevention (outbound HTTP guard) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/attestation + pkg/gpuattest platform/GPU attestation | partial | no | unknown | unit | no | no | no | native | partial |
| submodule | helix_specifier | parallel execution (bounded-concurrency dispatch) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | constitution-as-code (machine-readable rule enforce) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | Nyquist TDD (≥2x test-to-impl ratio enforce) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | debate architecture (multi-round spec refinement) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | skill learning (running-avg proficiency tracking) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | brownfield analysis (legacy pattern/dep detection) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | predictive specification (future-req prediction) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | cross-project transfer (shared knowledge base) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | adaptive ceremony (dynamic level by quality metrics) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | spec memory (persistent index + semantic search) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/autonomous autonomous QA session engine (~9.5k loc, 21 tests) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/orchestrator QA run orchestration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/navigator UI navigation/driving (~3.8k loc, 16 tests) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/detector + pkg/issuedetector issue/error detection | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/recordingqa recording-validator (anti-bluff §11.4.107) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/regression standing regression-guard suite | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/evidence captured-evidence management | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/challengegen Challenge generation | partial | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/reporter QA reporting | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/conduit + pkg/bridge sync-channel / agent bridge (§11.4.116) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/replay + pkg/reproduce replay / defect reproduction | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/distributed + pkg/maestro distributed/maestro coordination | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | banks/ test-bank corpus (JSON/YAML suites) | done | yes | unknown | none | no | no | no | native | partial |
| submodule | helix_qa | cmd/* tooling (axtree/capture/recvalidate/omniparser/uitars/lpips/...) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/launcher multi-platform test launcher (web/desktop/mobile) | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/executor UI automation executor | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/vision computer-vision element detection | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/ocr OCR text extraction | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/recvalidate recording validator | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/ai AI test generation/enhancement | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/cloud cloud storage/integration | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/platforms per-platform automation adapters | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | screenshot + video recording capture | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | cmd/record + cmd/vision + cmd/testgen CLI tooling | done | no | unknown | unit | no | no | no | native | partial |

### Round-2 coverage notes

- **Internal deepening:** the 12 round-1-flagged packages
  (`clientcore`, `agentbridge`, `checkpoint`, `ensembleui`, `substrate`,
  `workspace`, `voice`, `roocode`, `telemetry`, `verifier`, `worker`, `server`)
  are now broken out into 73 discrete feature rows. `checkpoint`, `telemetry`,
  most of `verifier`, `server`, and several `worker` features upgrade from
  round-1 `partial` to `working-untaped` on direct-source evidence (real git
  plumbing, real OTEL exporter, real Raft + cgroup writes, real Gin handlers
  with integration tests). `roocode` task-delegation/code-gen and
  `voice`/`workspace` auto-cleanup remain honest `gap` (real types, no shipped
  wiring / incomplete impl).
- **Stub-claim corrections** (see top caveat): three round-2-explorer "stub"
  reports were verified FALSE by direct read and corrected to `done`/`partial`.
  Logged here so the conductor does not propagate the wrong call.
- **Submodule `Real-use=unknown` throughout:** each submodule package compiles
  and has tests, but genuine end-user reachability THROUGH HelixCode (vs the
  submodule's own standalone tests) was not confirmed by static inspection —
  honestly `unknown`, never green.
- **`helix_agent` `Wired`:** `yes` for the packages HelixCode consumes via the
  direct `dev.helix.agent` import (llm/ensemble/server/provider/verifier/
  streaming surface reachable through the agent dependency); `partial` for
  packages reachable only transitively (database/redis/auth/monitoring/etc).
- **`panoptic` `Wired=no`:** present as an equal-codebase submodule
  (CONST-051) but NOT in HelixCode's Go build graph — it is a
  Challenge/recording framework consumed out-of-process, not imported.
- Every `Real-use=unknown` / `working-untaped` row is a candidate for a
  recorded scenario; none is video-confirmed (📹 `no` throughout).

**Round-2 deepened feature count: 145 rows** (73 internal across 12 packages +
72 submodule across helix_agent 19, security 11, helix_specifier 10,
helix_qa 14, panoptic 10, distributed across the wiring model).

## Sources verified 2026-06-22: helix_code/internal/* , submodules/{helix_agent,security,helixspecifier,helix_qa,panoptic}/

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
This round-2 deepening re-reads packages round-1 flagged shallow; cross-referenced
against the live tree on 2026-06-22:
- The deepened rows resolve against real `helix_code/internal/*` package source and
  the named own-org submodules under `submodules/*` (`helix_agent`, `security`,
  `helix_qa`, `panoptic`, helix_specifier).
- The doc already records its own source-read corrections (round-1 stub claims
  verified false by direct source reads: `workspace` docker stop/remove, `voice`
  recorder) — those ARE the §11.4.99 negative findings for this slice, captured
  inline at the head of the file.
- No external-service version claims in this doc → no staleness check applies;
  the evidence is structural (impl reality / wiring / `_test.go` presence).


## Internal services + infrastructure

Inventory of every feature under `helix_code/internal/*` (72 packages). Assessed
from code evidence (impl reality, wiring, `_test.go` presence) per CONST-035 /
§11.4.107 anti-bluff. `📹 Video` is `no` for every row (recordings are the
conductor's job once a real analyzed recording exists); `Overall` is never
`confirmed` for the same reason. `Origin` is `native` (HelixCode's own code).

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| service | internal/adapters | i18n message resolution | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/adapters | translator injection seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/adapters | speckit debate adapter | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/adapters | container adapter | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/agent | agent execution interface | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent capability management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent health check | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent collaboration | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/agentbridge | verifier bridge | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/approval | approval request management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/approval | approval mode selector | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/approvalwire | yes/no prompter | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | user authentication | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | session management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | password hashing (bcrypt/argon2) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | JWT token generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/autocommit | git auto-commit | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/autocommit | secret filter | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | multi-tier cache | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | redis tier | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | disk tier | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint manager | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/clarification | clarification engine | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/clarification | question generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | agentic-tools provider | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | skills provider | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/cognee | cognee client | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/cognee | cognee manager | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cognee | cognee cache manager | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | aider command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | approval command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | browser command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | edit command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | git auto-commit command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | mcp command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | markdown commands | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | skills command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | subagents command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | tasks command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | worktree command | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command executor | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command registry | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command parser | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | viper-based config loader | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | cognee configuration management | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | verifier configuration | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | platform-UI adapters config | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/context | context builder | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/context | token counter | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/context | history condenser | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | completion engine | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | continue-edit tool | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | continue-complete tool | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/database | database connection pool | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/database | postgres integration | done | yes | yes | integ | no | no | no | native | working-untaped |
| infrastructure | internal/deployment | production deployer | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/deployment | deployment strategy | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | service registry with TTL + health checks | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | UDP multicast broadcast discovery | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/discovery | dynamic port allocation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | health monitoring (HTTP/gRPC/TCP) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | unified-diff code editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | whole-file replacement editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | search-and-replace editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | line-range editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | model-specific format selection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/ensembleui | ensemble UI rendering | partial | partial | unknown | none | no | no | no | native | gap |
| service | internal/event | publish-subscribe event bus | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/event | async + sync event handling | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/event | task/workflow/worker event types | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/fix | security issue auto-fix | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | hierarchical focus management | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | priority-based focus tracking | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | focus chain tracking | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/hardware | CPU detection + profiling | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hardware | GPU detection (NVIDIA/AMD/Apple/Intel) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hardware | optimal LLM model-size inference | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/helixqa | HelixQA test wrapper | partial | yes | unknown | unit,integ | no | no | no | native | partial |
| service | internal/hooks | hook registration + triggering | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/hooks | priority-based hook execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hooks | async/sync hook execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/i18nwiring | i18n catalog wire-all (multi-lang) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/infraboot | on-demand infra boot (EnsureInfra) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/kilocode | call-graph build + query | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | symbol rename engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | change-impact analyzer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | refactor (extract-method/inline-call) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | multi-edit tool | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/llm | OpenAI provider (GPT models) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | Anthropic provider (Claude models) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | Google Gemini provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Azure OpenAI provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | AWS Bedrock provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Ollama local provider | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | llama.cpp local inference | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Mistral provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | DeepSeek provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Groq provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | streaming response handling | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | token counting + accounting | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | model discovery/listing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | load balancing across providers | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | provider health monitoring | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | ensemble provider orchestration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | embeddings generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logging | structured logging with levels | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logging | named loggers | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logo | logo processing + assets | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/logo | color extraction + icon generation | done | yes | unknown | none | no | no | no | native | partial |
| infrastructure | internal/mcp | Model Context Protocol server | done | yes | partial | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | JSON-RPC 2.0 tool invocation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | WebSocket session management | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | OAuth token management | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/memory | Cognee LLM memory integration | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/memory | memory state persistence | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/monitoring | system metrics collection | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/monitoring | health check monitoring | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/notification | multi-channel notification engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Slack integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Discord integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Telegram integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Email/SMTP integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Teams integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | rate limiting + retry | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/performance | performance optimizer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/performance | CPU optimization | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/performance | memory optimization | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/persistence | file-based state store | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/persistence | session serialization | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/persistence | auto-save manager | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/planner | task executor | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/planner | OpenHands plan execution | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/plantree | plan tree system | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/plantree | plan node management | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/plantree | plan branching + merging | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin base framework | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin activation | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin hooks registry | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/project | project manager | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/project | project metadata | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/project | database-backed storage | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | memory loader | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | filesystem watcher | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | memory registry | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/provider | provider interface | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/provider | multi-provider support | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/providers | AI integration | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/providers | fallback + load balancing | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/quality | quality gate scoring | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/quality | build verification | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/quality | linting validation | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | redis client | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | key-value operations | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | pub/sub messaging | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | ANSI renderer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | fancy-mode output | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | frame buffer management | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | streaming block renderer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | symbol extraction | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | file ranking engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | repo cache | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | tree-sitter integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/roocode | Roo-Code CLI port | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | template-based code generation | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/rules | rule hierarchy management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/rules | rule pattern matching (glob/regex/exact) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/rules | rule category + tag querying | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/secrets | secret loader from .env files | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/secrets | secret validation + missing-var detection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/security | security manager (global + local) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/security | feature scanning + zero-tolerance validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | auth endpoints (register/login/logout/refresh) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | user profile endpoints (GET/PUT /me) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | worker mgmt endpoints (list/register/heartbeat) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | task CRUD endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | project CRUD endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | workflow execution endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | system stats + health-check endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | LLM provider + model list endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | memory system statistics endpoint | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | WebSocket real-time communication | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/session | session lifecycle (create/start/complete/pause) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | session modes (planning/building/testing/refactoring) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | focus chain integration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | context + metadata tracking | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | session querying + filtering | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/substrate | substrate abstraction layer | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/task | task creation + assignment | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | task status lifecycle | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | priority-based task queue | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | checkpoint creation + recovery | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | dependency + circular-dependency validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | dependent task tracking | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | redis caching for tasks | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | agent instrumentation (prompt/response) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | LLM instrumentation (calls/latency/tokens) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | tool instrumentation (execution/errors) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | OpenTelemetry provider integration | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/template | template creation + registration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/template | variable substitution + validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/template | built-in template library | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | theme detection (OS/system prefs) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | theme loading + customization | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | built-in theme collection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file read/write operations | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file editing with string replacement | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file globbing (pattern matching) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file content search (grep) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | shell command execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | background shell execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | web page fetching + parsing | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | web search integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | browser automation (launch/navigate/screenshot) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | codebase mapping + symbol definitions | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | multi-file transactional editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | user confirmation prompts | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | notebook read/edit operations | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/tools | tool registry + execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/verifier | LLMsVerifier HTTP client | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/verifier | model metadata adapter | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | two-tier cache (LRU + Redis) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | health monitoring + circuit breaker | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | background poller (real-time updates) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | event publishing to HelixCode bus | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/version | version string retrieval | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/version | build metadata exposure | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/voice | audio capture (arecord/sox) | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | Whisper API transcription | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | local whisper.cpp fallback | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/worker | worker registration + management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | SSH-based remote execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | health monitoring + metrics | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | capability auto-detection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker isolation + sandboxing | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | consensus protocol (leader election) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | host-key verification + SSH security | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/workflow | planning workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | building workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | testing workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | refactoring workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | DAG-based step orchestration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/workflow | LLM provider integration for workflows | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | container-based workspace management | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | Docker/Podman container orchestration | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | project directory mounting + isolation | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | auto-cleanup TTL enforcement | partial | no | unknown | unit | no | no | no | native | gap |
| infrastructure | internal/mocks | unit-test mock fixtures (test-only, not a user feature) | done | yes | n/a | none | no | n/a | no | native | n/a |
| infrastructure | internal/pprofutil | pprof profiling helper (dev/test support) | done | partial | unknown | none | no | no | no | native | partial |
| infrastructure | internal/testutil | shared test utilities (test-only, not a user feature) | done | yes | n/a | none | no | n/a | no | native | n/a |

### Coverage notes

- **Partially assessed (need deeper inventory):** `clientcore`, `agentbridge`,
  `checkpoint`, `ensembleui`, `substrate`, `workspace`, `voice`, `roocode`,
  `telemetry`, `verifier` (poller/event-bus wiring), `worker` (isolation/consensus),
  and `server` real-use (declared endpoints, no integration evidence inspected).
  Their core types compile and have unit tests, but wiring into a shipped flow
  and genuine end-user reachability could not be fully confirmed from static
  inspection alone — flagged `partial`/`unknown` honestly rather than green.
- `mocks`, `pprofutil`, `testutil` are dev/test-support packages, not user
  features — listed for completeness, marked `n/a`.
- Every `Real-use=unknown` and every `working-untaped` row is a candidate for a
  recorded scenario; none is video-confirmed yet (📹 `no` throughout).

233 features inventoried across 72 packages.

## Sources verified 2026-06-22: helix_code/internal/*

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
Cross-referenced against the live tree on 2026-06-22:
- **`helix_code/internal/*` = 72 packages — CONFIRMED** (`ls -d helix_code/internal/*/`
  = 72), matching the "across 72 packages" rollup. The per-row `Dev`/`Wired`/`Tests`
  assessments are structural evidence (impl reality + `_test.go` presence) about
  HelixCode's own code.
- No external-service version claims in this doc → no §11.4.99 staleness check
  applies; nothing to contradict against an upstream source.


## Ported cli_agents capabilities

Evidence-backed inventory of capabilities **actually ported into HelixCode** from the
`cli_agents/` reference catalogue (51 vendored reference agents). Per CONST-035 anti-bluff:
this lists ONLY capabilities with landed code evidence (package `doc.go` origin headers,
CONTINUATION.md P2-Fxx CLOSED ledger, POWER_FEATURES_PORTING_PLAN rev2 file:line
reconciliation, git port commits) — NOT every cli_agent's full feature set. Planned-but-not-landed
items are marked `Dev=absent`/`partial` honestly. **No feature is `📹 yes`** (no recordings analyzed
for ported features). **Overall is never `confirmed`** (a confirmed rollup requires an analyzed video).

Evidence basis: (1) CONTINUATION.md ports ledger lists `P2-F21..P2-F30` as CLOSED with landing
packages; (2) each landed package's `doc.go` names its source agent ("aider voice input port P2-F27",
"Roo-code CLI agent port", "Continue.dev IDE integration", "Go port of upstream gptme's profile mechanism");
(3) POWER_FEATURES_PORTING_PLAN.md rev2 reconciles Phase-1 against HEAD with file:line;
(4) `HXC-031-codex-cline-port.md` is DRAFT (plan only, NO code) — codex multimodal + cline computer-use are PLANNED, not landed.

| Area | Component (HelixCode pkg) | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| service | internal/approval | Approval/exec-policy modes (per-tool yes/no, autopilot tiers) | done | yes | unknown | unit | no | no | no | ported:codex | working-untaped |
| service | internal/autocommit | Git auto-commit per change (generated msg, secret-filter, summariser) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/tools/browser | Browser tool suite (launch/click/type/scroll/screenshot via chromedp) | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| service | internal/projectmemory | Project-memory context files (AGENTS.md/.clinerules-style loader+watcher) | done | yes | unknown | unit | no | no | no | ported:codex | working-untaped |
| service | internal/plantree | Plan trees (branching persistent implementation plans, JSON-backed) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/session (condense.go) | Context compaction / history condense (CompactIfNeeded/ShouldCompact) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/workspace | Container per-task workspace (Docker/Podman mount, TTL cleanup) | done | partial | unknown | unit | no | no | no | ported:openhands | partial |
| service | internal/voice | Voice-to-code input (arecord/sox capture, Whisper API + whisper.cpp fallback) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/repomap | Repo-map (tree-sitter ranked incremental project map) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/kilocode | AST-aware refactoring (cross-file rename, impact/call-graph, extract/move/inline) | done | yes | unknown | unit | no | no | no | ported:kilo-code | working-untaped |
| service | internal/roocode | Roo-code full port (task delegation, template gen, diff review, conv memory) | done | yes | unknown | unit | no | no | no | ported:roo-code | working-untaped |
| application | internal/continua | Continue.dev IDE integration (inline completions, editor, chat panel, diff, model selector) | done | partial | unknown | unit | no | no | no | ported:continue | partial |
| service | internal/agent/profiles | Verifier work profile + RoleVerify posture (subagent review/validation) | done | yes | unknown | unit | no | no | no | ported:gptme | working-untaped |
| application | cmd/cli (main.go:2118,2326) + autocommit/git.go | /undo + /diff (git-aware, force-push-free revert) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/workflow/autonomy | Configurable autonomy presets (None/Basic/BasicPlus/SemiAuto/FullAuto, 5 tiers) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/workflow/planmode | First-class Plan/Act mode controller (tool-gated by mode) | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| service | internal/tools/askuser | ask_user interactive clarification tool | done | yes | unknown | unit | no | no | no | ported:gemini-cli | working-untaped |
| service | internal/commands (markdown_skills.go) | Markdown skills subsystem (project>user 2-tier precedence) | partial | yes | unknown | unit | no | no | no | ported:gemini-cli | partial |
| service | internal/checkpoint | Workspace file-snapshot checkpoint + /checkpoint create/list/restore | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| application | cmd/cli (main.go:1727) | Per-request token-usage counter (real provider Usage, anti-fabrication) | done | yes | unknown | unit | no | no | no | ported:codai | working-untaped |
| service | (no production code) | Context-window-% indicator | absent | no | no | none | no | no | no | ported:codai (planned) | gap |
| service | (no production code) | TODO/step tracker surface (/tasks is background-job list, not TODO tracker) | partial | partial | no | none | no | no | no | ported:gemini-cli (planned) | partial |
| service | markdown_skills.go | SKILL.md built-in/bundled tier + canonical SKILL.md filename | absent | no | no | none | no | no | no | ported:gemini-cli (planned) | gap |
| service | internal/llm (HXC-031 plan) | Codex multimodal image-content LLM request surface | absent | no | no | none | no | no | no | ported:codex (planned/DRAFT) | gap |
| service | internal/tools/browser (HXC-031 plan) | Cline computer-use feedback loop (screenshot-per-action coord control) | absent | no | no | none | no | no | no | ported:cline (planned/DRAFT) | gap |
| service | (planned, POWER_FEATURES F22) | Cumulative diff-review sandbox (stage-before-apply, apply/reject hunks) | absent | no | no | none | no | no | no | ported:plandex (planned) | gap |
| service | (planned, POWER_FEATURES F25) | Conversation branches / fork | absent | no | no | none | no | no | no | ported:plandex (planned) | gap |
| service | (planned, POWER_FEATURES F36/F43) | /rewind + Tangent mode | absent | no | no | none | no | no | no | ported:gemini-cli/amazon-q (planned) | gap |
| service | (planned, POWER_FEATURES F18) | Messaging connectors (Slack/Telegram/Discord) | absent | no | no | none | no | no | no | ported:cline (planned) | gap |
| service | (planned, POWER_FEATURES F35) | ACP mode (Agent Client Protocol over stdio) | absent | no | no | none | no | no | no | ported:gemini-cli (planned) | gap |
| service | (planned, POWER_FEATURES F56) | OpenAI-compatible server endpoints | absent | no | no | none | no | no | no | ported:shai/aichat (planned) | gap |
| service | (planned, POWER_FEATURES F61) | Spec-driven workflow surface (specify→plan→tasks→implement) | absent | no | no | none | no | no | no | ported:spec-kit (planned) | gap |
| service | (planned, POWER_FEATURES F33) | OS-level exec sandbox (Seatbelt/Landlock/bwrap) | absent | no | no | none | no | no | no | ported:codex (planned) | gap |

Count: 33 rows total — **20 landed ports** (`Dev=done`, working-untaped), **3 partial-landed**
(workspace, continua, markdown-skills), **10 planned/not-landed** (`Dev=absent`/`partial`, `gap`).
Zero `📹 yes`; zero `confirmed`.

Honest assessment: The **Phase-2 port wave (P2-F21..F30) genuinely landed** — 10 named source-agent
ports (codex/aider/cline/plandex/openhands/kilo-code/roo-code/continue) ship as real packages with
origin-attributed `doc.go` headers, unit tests, and CONTINUATION CLOSED records, plus the gptme
verifier-profile port. The **POWER_FEATURES Phase-1 wrap (autonomy/Plan-Act/undo-diff/ask_user/skills/
token-count/checkpoint) is largely landed** per the rev2 file:line reconciliation. Beyond that, the
porting is **mostly PLANNED, not landed**: POWER_FEATURES_PORTING_PLAN is explicitly "DRAFT — research +
plan only, NO code", its Phases 2–7 (diff-sandbox, branches, rewind, tangent, connectors, ACP, OpenAI
server, spec-driven, OS sandbox) are unimplemented, and HXC-031 (codex multimodal + cline computer-use)
is a DRAFT plan with no code. No ported feature has an analyzed recording, so none can be `confirmed`;
all landed ports are honestly `working-untaped` pending video V&V.

## Sources verified 2026-06-22: cli_agents/* , helix_code/internal/* (port doc.go origin headers) , docs/CONTINUATION.md (P2-Fxx ports ledger) , docs/plans/POWER_FEATURES_PORTING_PLAN.md , docs/plans/HXC-031-codex-cline-port.md

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees +
in-repo ledgers, following the `docs/ARCHITECTURE.md` precedent — the `cli_agents/`
reference catalogue is vendored third-party, but the CLAIM here is which of its
capabilities have LANDED in HelixCode's own code, evidenced repo-side). Cross-
referenced against the live tree on 2026-06-22:
- **`cli_agents/* = 50 directories`** present (the reference catalogue this slice
  draws from; note `_status_header.md`/this doc's prose says "51 vendored
  reference agents" — live `ls` count is **50**, a minor count drift to reconcile).
- Landed ports are evidenced by package `doc.go` origin headers under
  `helix_code/internal/*`, the `docs/CONTINUATION.md` P2-Fxx CLOSED ledger, and the
  `POWER_FEATURES_PORTING_PLAN.md` rev2 file:line reconciliation — not by the
  upstream agents' full feature sets.
- **Negative finding restated:** `docs/plans/HXC-031-codex-cline-port.md` is a DRAFT
  plan with NO code (codex multimodal + cline computer-use PLANNED, not landed) —
  the doc already marks these honestly; no green bluff.
- No external-service version claim in this doc → no upstream-staleness check
  applies (the vendored `cli_agents/` are pinned reference copies, not a live
  service this doc instructs against).


## Owned-submodule capabilities

Inventory of the discrete features each owned (own-org `vasic-digital` /
`HelixDevelopment`) submodule under `submodules/*` provides to HelixCode.
Assessed from each submodule's `README.md`, exported package surface, and
`*_test.go` presence (CONST-035 / §11.4.107 — evidence-based, no bluffs).

> **Wiring model (load-bearing).** HelixCode's inner Go module
> (`helix_code/go.mod`) `replace`s + imports only **3** submodules directly
> — `helix_agent` (`dev.helix.agent`), `dag_orchestrator` (`dev.helix.dag`),
> `pipeline_runtime` (`dev.helix.pipeline`). HelixCode source additionally
> imports a handful by module path (`containers`, `helixspecifier`, `helixqa`,
> `concurrency`, `helixmemory`, `debate`, `lazy`, `llmsverifier`, `memory`,
> `llmprovider`). A large set is wired **transitively** because
> `helix_agent/go.mod` requires ~35 own-org submodules — these are reachable
> through the agent dependency but NOT imported by HelixCode directly
> (`Wired=partial`). The remainder are present in the repo as equal-codebase
> submodules (CONST-051) but are NOT in HelixCode's Go build graph at all
> (`Wired=no`). `📹 Video=no` for every row (conductor owns video confirmation;
> nothing here is `confirmed`). Origin=`native` throughout (all own-org).

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| submodule | helix_agent | Ensemble multi-LLM agent service (response fusion) | done | yes | yes | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | ReAct/tool-calling agent runtime (consumed by HelixCode `internal/agent`) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| submodule | helix_agent | Skill registry + debate + verifier wiring (umbrella, 1583 test files) | done | yes | unknown | unit,integ,e2e | no | no | no | native | partial |
| submodule | dag_orchestrator | Agent-free DAG scheduler (topo dispatch, worker pool, retry/backoff) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | pipeline_runtime | Staged streaming dataflow runtime (push operators + FBP backpressure) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | Spec-driven dev fusion (SpecKit 7-phase + TDD + GSD milestones) | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_specifier | 10 power features (parallel exec, constitution-as-code, debate phases) | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_memory | Unified cognitive memory (Mem0+Cognee+Letta+Graphiti fusion) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_memory | Parallel cross-backend search + 3-stage fusion/dedup/re-rank | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | debate_orchestrator | Multi-agent debate orchestration (consensus + dissent, LessonBank) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | debate_orchestrator | Aux packages (validation/audit/evaluation/reflexion/tools) | stub | partial | no | unit | no | no | no | native | gap |
| submodule | concurrency | Worker pools, priority queues, rate limiters, circuit breakers, semaphores | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | lazy | Type-safe lazy-init primitives (sync.Once generics) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | memory | Mem0-style scoped memory + entity extraction + knowledge graph + leak detect | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_provider | LLMProvider interface + circuit breaker + health monitor + retry + lazy | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llms_verifier | LLM verification platform (existence/latency/streaming/vision/embeddings) | done | yes | unknown | unit,integ,e2e | no | no | no | native | partial |
| submodule | llms_verifier | Single-source-of-truth model/provider metadata (CONST-036/037) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | containers | Container lifecycle (boot/compose/health) for infra-on-demand (§11.4.76) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_qa | Autonomous QA / Challenge orchestration (test-bank runner) [QA infra] | done | yes | unknown | unit,integ,e2e | no | n/a | no | native | partial |
| submodule | panoptic | Recording-validator / observation harness [QA infra] | done | partial | unknown | unit,integ | no | n/a | no | native | partial |
| submodule | agentic | Graph-based agentic workflow engine (branch/checkpoint/self-correct) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | auth | JWT / API-key / OAuth2 / HTTP auth middleware / token store | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | cache | Multi-backend cache (mem/Redis/PG) + distributed patterns + TTL/eviction | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | storage | Object storage (S3/MinIO/local) + cloud credential mgmt | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | vector_db | Unified vector store (Qdrant/Pinecone/Milvus/pgvector) | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | embeddings | Text embeddings across 7 providers (OpenAI/Cohere/Voyage/Jina/Google/Bedrock) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | event_bus | Typed pub/sub bus + glob/prefix/metadata filtering + middleware | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | messaging | Message-broker abstraction (in-mem/Kafka/RabbitMQ) + producer/consumer patterns | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | streaming | SSE / WebSocket(rooms) / gRPC streaming / webhook(HMAC) / HTTP+breaker / Gin | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | rag | RAG pipeline (3 chunkers, BM25+semantic+hybrid retrieve, MMR rerank, RRF fusion) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | tool_schema | Tool schema/validation/exec + 14 built-in safe tool handlers | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | plugins | Plugin lifecycle (dep-ordered) + .so/process loaders + sandbox + output parse | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | planning | AI planning algorithms (HiPlan + MCTS + Tree-of-Thoughts) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | optimization | Semantic cache + prompt compression + structured-output + SGLang/LlamaIndex/LangChain | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | self_improve | RLAIF pipeline (reward models, feedback, policy/prompt optimization) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | benchmark | LLM benchmarking (SWE-bench/HumanEval/MMLU/GSM8K) + leaderboard | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_ops | LLMOps (continuous eval, A/B experiments, prompt versioning, alerting) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | mcp_module | MCP server+client (JSON-RPC 2.0 stdio+HTTP/SSE) + adapter registry | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | models | Shared AI/LLM data types (LLMRequest/Response, MCP/LSP/ACP, code-intelligence) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | formatters | Pluggable code-formatter registry + engine + cache + native-binary shims | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | vision_engine | CV + LLM-vision UI analysis + navigation-graph construction | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | doc_processor | Doc-feature-map extraction + verification-coverage tracking (QA-oriented) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | Security tooling module (round-300 deep-doc) — needs deeper inventory | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | red_team | YAML-driven adversarial-prompt fixture harness (defensive guardrail regression) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_orchestrator | Headless CLI-agent orchestration (OpenCode/Claude/Gemini/Junie/Qwen) | done | no | no | unit | no | no | no | native | gap |
| submodule | helix_llm | Distributed LLM service (OpenAI/Anthropic APIs, llama.cpp, RAG, ReAct, HTTP/3) | done | no | no | unit,integ,e2e | no | no | no | native | gap |
| submodule | conversation | Infinite-context (event-sourcing) + LLM compression + LRU cache | done | no | no | unit | no | no | no | native | gap |
| submodule | database | Driver-agnostic DB (PG/SQLite) + pool + migrations + repo + query builder | done | no | no | unit,integ | no | no | no | native | gap |
| submodule | document | Document model (18-format detect, change tracking, JSON serialize) | done | no | no | unit | no | no | no | native | gap |
| submodule | filesystem | Multi-protocol FS client (SMB/FTP/NFS/WebDAV/Local) | done | no | no | unit | no | no | no | native | gap |
| submodule | config | Config mgmt (JSON files, env binding, validation, 8 storage-protocol types) | done | no | no | unit | no | no | no | native | gap |
| submodule | i18n | i18n library (bundles, loader, HTTP middleware) | done | no | no | unit | no | no | no | native | gap |
| submodule | observability | Distributed tracing + metrics + structured logging + health + analytics | done | no | no | unit | no | no | no | native | gap |
| submodule | middleware | Reusable net/http middleware (requestid/logging/recovery/cors/chain) | done | no | no | unit | no | no | no | native | gap |
| submodule | rate_limiter | Rate limiting (in-mem + Redis) + HTTP middleware | done | no | no | unit,integ | no | no | no | native | gap |
| submodule | recovery | Named circuit breakers + periodic health checks + resilience facade | done | no | no | unit | no | no | no | native | gap |
| submodule | watcher | Filesystem change monitoring (debounce, filters, handler chains) | done | no | no | unit | no | no | no | native | gap |
| submodule | background_tasks | Persistent task queue (PG) + worker pool + stuck-detect + DLQ + progress | done | no | no | unit | no | no | no | native | gap |
| submodule | skill_registry | Skill mgmt (load YAML/JSON/MD, register, execute, validate, store) | done | no | no | unit | no | no | no | native | gap |
| submodule | embeddings | (see embeddings row — module also standalone) | — | — | — | — | — | no | — | native | — |
| submodule | auto_temp | LLM temperature auto-tuning (multi-temp run + multi-judge scoring) | done | no | no | unit | no | no | no | native | gap |
| submodule | hyper_tune | LLM hyperparameter optimization (random/grid/Bayesian-lite) | done | no | no | unit | no | no | no | native | gap |
| submodule | i_llm | Structured-reasoning patterns (CoT/ToT/ReAct/few-shot/prompt-chain) | done | no | no | unit | no | no | no | native | gap |
| submodule | veritas | AI-truthfulness verification, fact-check, hallucination detection | done | no | no | unit | no | no | no | native | gap |
| submodule | leak_hub | System-prompt-leak detection + searchable archive | done | no | no | unit | no | no | no | native | gap |
| submodule | claritas | Defensive guardrail (leaked-prompt archive + extraction-attempt detector) | done | no | no | unit | no | no | no | native | gap |
| submodule | gandalf_solutions | Read-only Gandalf prompt-hacking solutions archive (research) | done | no | no | unit | no | no | no | native | gap |
| submodule | ouroborous | Self-referential AI-safety (recursive self-improve, runaway-loop detect) | done | no | no | unit | no | no | no | native | gap |
| submodule | normalize | Adversarial-input canonicalization (base64/leet/homoglyph/NFKC/ROT13…) | done | no | no | unit | no | no | no | native | gap |
| submodule | plinius_common | Plinius shared lib (config validators, error types, gRPC client, i18n, types) | done | no | no | unit | no | no | no | native | gap |
| submodule | toon | Token-Oriented Object Notation encode/decode | stub | no | no | unit | no | no | no | native | gap |

55 features across 50 submodules.

> **HelixLLM extension cross-reference (added 2026-07-11, §11.4.153/§11.4.91):**
> the `helix_llm` row above (line ~885, summary-level, marked `gap` as of the
> 2026-06-22 inventory pass) is now additionally covered by a detailed, more
> current §11.4.153 per-feature ledger — see `docs/features/helixllm-status.md`
> (43 rows, 34 PASS as of 2026-07-11) for the HelixLLM full-extension
> programme's serving/capabilities/protocols/test-infra coverage in depth.
> This row is left unchanged here per §11.4.122 (no destructive rewrite of an
> existing row); `helixllm-status.md` is the authoritative, current source for
> HelixLLM-extension-specific status, while this row remains this document's
> summary-level entry pending a future full-ledger reconciliation pass.

### Coverage-depth honesty

- **Shallow-inventoried (principal features only; deeper inventory warranted):**
  `helix_agent` (1583 test files — only top umbrella capabilities captured;
  internal package-level features not enumerated), `helix_qa` (646 test files,
  QA infra — single capability row), `helix_specifier` (37 tests, 10 power
  features collapsed to 2 rows), `security` (deep-doc module — only a single
  umbrella row; per-package security capabilities NOT enumerated, flagged as
  needs-deeper-inventory), `panoptic` (QA infra — single row).
- **`Real-use` is `unknown`** for nearly every transitively-wired (`partial`)
  and standalone (`no`) submodule because reachability through `helix_agent`'s
  go.mod ≠ proof an end user exercises the capability via HelixCode; only the
  3 directly-replaced modules + `concurrency`/`lazy` show `yes`.
- **`Dev` reflects README claims + test presence**, NOT runtime verification.
  `toon` is honestly `stub` (PENDING_IMPLEMENTATION per its own README);
  `debate_orchestrator` aux packages are `stub` (NotYetImplemented per ACK-STUB).
- **`Tests` column** marks `unit` baseline (all 70 own-org submodules have
  `*_test.go`); `integ`/`e2e` added only where README/dir evidence shows real
  infra/integration suites (cache, storage, vector_db, llms_verifier, helix_qa,
  helix_agent, containers, database, rate_limiter, helix_llm, observability-N/A).
- Infra/tooling submodules excluded from feature rows per task scope:
  `docs_chain`, `challenges` (and `containers`/`helix_qa`/`panoptic` included
  only as the capabilities HelixCode consumes, marked QA/infra).

## Sources verified 2026-06-22: submodules/* , helix_code/go.mod , submodules/helix_agent/go.mod , each submodule's README.md + *_test.go

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
Cross-referenced against the live tree on 2026-06-22:
- **`submodules/* = 67 directories`** present. The doc states "all 70 own-org
  submodules have `*_test.go`" / "70+"; the live directory count is **67** — a
  minor count drift to reconcile on the next revision (the per-row enumeration
  itself reflects the actually-present submodules).
- **Wiring model confirmed at the build-graph level:** the doc's load-bearing
  claim that HelixCode's inner module directly `replace`s+imports only 3
  submodules (`helix_agent`/`dev.helix.agent`, `dag_orchestrator`/`dev.helix.dag`,
  `pipeline_runtime`/`dev.helix.pipeline`) while ~35 are reachable transitively
  via `helix_agent/go.mod` is verifiable against `helix_code/go.mod` +
  `submodules/helix_agent/go.mod` (the cited `Wired=partial` rows).
- No external-service version claim in this doc → no §11.4.99 staleness check
  applies; the evidence is each submodule's README + exported surface + tests.


## Inventory sources

- `docs/features/inventory/cmd_and_clients.md`
- `docs/features/inventory/deepened_round2.md`
- `docs/features/inventory/internal_services.md`
- `docs/features/inventory/ported_cli_agents.md`
- `docs/features/inventory/submodules.md`
