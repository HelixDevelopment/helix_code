# HelixCode — Status Summary

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-19 |
| Last modified | 2026-06-19 |
| Status | Active Development |
| Status detail | docs/features/helixcode-status.md |
| Prior summary | docs/features/Status_Summary.md (Revision 4, 2026-06-16) |

Authoritative rollup of `docs/features/helixcode-status.md`. Anti-bluff
(CONST-035 / §11.4.83 / §11.4.107): the video-confirmed count rises ONLY as
real analyzed recordings land in `/Volumes/T7/Downloads/Recordings`; an
un-recorded feature is honestly `pending`/`no`, never bluffed green.

## Rollup — by Overall

| Overall rollup | Count | Meaning |
|---|---|---|
| working-untaped | 324 | real + tested, no analyzed video yet |
| partial | 168 | real but thin/unverified coverage OR partial wiring |
| gap | 49 | scaffold / untested / planned-not-landed |
| confirmed | 23 | real analyzed recording exists (Video: yes) |
| n/a (test-support) | 3 | mocks/testutil/pprofutil — not user features |
| **Total feature rows** | **567** | |

## Rollup — by Development status

| Dev | Count |
|---|---|
| done | 507 |
| partial | 40 |
| absent (planned-not-landed) | 12 |
| stub | 4 |
| n/a | 1 |

## Rollup — by Tests coverage

| Tests | Count | Notes |
|---|---|---|
| has integ/e2e | 67 | real-infra integration / end-to-end suites |
| unit-level baseline | ~466 | `unit` (combinable) |
| none | 31 | untested surfaces — flagged honestly |
| §11.4.118/§11.4.135 guard/race upgrade | 36 pkgs (43 files) | standing regression + `-race` guards |

## Rollup — by Video confirmation (anti-bluff)

| Video | Count |
|---|---|
| confirmed (Video: yes, real analyzed recording) | 23 rows |
| pending / no (not yet recorded or non-visual) | 544 rows |
| `helixcode-*` real recordings produced so far | 30+ (incl. 2026-06-16 CLI/API/TUI/Web sweep) |

## Key Counts

| Category | Count |
|---|---|
| Client Applications | 8 (server, CLI, TUI, Desktop, Web, Android, iOS, Aurora OS, HarmonyOS) |
| LLM Providers | 21 (including Xiaomi MiMo, Ensemble, OpenAI Compatible) |
| Internal Packages | 67 (64 with tests, 3 test-support) |
| CLI Top-Level Commands | 6 |
| CLI Subcommand Groups | 9 |
| CLI Interactive Commands | 18 |
| API Endpoints | 56 (12 public, 44 authenticated) |
| Core Platform Submodules | 19 (HelixDevelopment + vasic-digital) |
| vasic-digital Library Submodules | 48 |
| Third-Party Dependency Submodules | 3 |
| CLI Agent References | 55 |
| Total Submodules | 129 |
| Provider Supporting Components | 19 |
| Feature Rows (total) | 567 |
| Video-Confirmed Features | 23 |
| Real Recordings | 30+ |

## Coverage Completeness

- **Internal packages:** 73/73 have ≥1 feature row (234 internal-feature rows)
- **cmd tools:** all 11 `helix_code/cmd/*` dirs rowed
- **Client apps:** all 8 surfaces (CLI, TUI, Web, Desktop, Android, iOS, Aurora OS, HarmonyOS)
- **HTTP API:** 18 endpoint groups rowed (56 endpoints)
- **LLM Providers:** 21 providers (19 with tests)
- **Submodules:** 50 inventoried (55 capability rows)
- **Ported cli_agents:** 33 rows (20 landed, 3 partial, 10 planned)

## Video-Confirmation Sweep 2026-06-16

Real DeepSeek, fresh binaries, all analyzed:
- **CLI 9 features** (stream, generate, list-models, command, health, list-workers, notify, model+max-tokens, approval)
- **API 5 features** (health, models, generate, auth, tasks-crud)
- **TUI 2 features** (llm-chat DeepSeek, navigation/theme tour)
- **Web 2 features incl. SSE streaming** (llm-console, SSE stream)

The `-stream` recording caught a §11.4.108 STALE-BINARY break → rebuilt → fixed + §11.4.135 guard added. The anti-bluff payoff of reading the recorded screen.

## Anti-Bluff Payoff

- **BLUFF-001 resolved:** LLM generation calls real `provider.Generate`/`GenerateStream`
- **BLUFF-002 resolved:** Model listing queries `providerManager.GetProviders()`
- **BLUFF-003 resolved:** Command execution uses `os/exec` with real exit codes
- **Stale-binary caught:** CLI `-stream` recording detected §11.4.108 break
- **All recordings analyzed:** Every video is READ, not just produced

## Infrastructure

- **Services on nezha.local:** 11 deployed
- **Build system:** `make build` → `bin/helixcode` (server), `bin/cli` (client)
- **Database:** PostgreSQL 15+ via pgx/v5
- **Cache:** Redis 7+ via go-redis/v9
- **Auth:** JWT + bcrypt/argon2 + OAuth2
- **Config:** Viper v1.21.0 + Cobra v1.8.0

## Sources verified 2026-06-19

- Live codebase: `helix_code/internal/` (67 packages), `helix_code/cmd/` (11 dirs), `helix_code/applications/` (8 apps)
- LLM providers: `helix_code/internal/llm/` (21 provider files)
- API endpoints: `helix_code/internal/server/handlers.go` (56 endpoints)
- Prior status: `docs/features/Status.md` Revision 7 (2026-06-16)
- Prior summary: `docs/features/Status_Summary.md` Revision 4 (2026-06-16)
- Video corpus: `/Volumes/T7/Downloads/Recordings/` (30+ recordings)
