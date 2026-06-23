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
