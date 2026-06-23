# HelixCode Feature Status — Summary

| | |
|---|---|
| Revision | 5 |
| Created | 2026-06-15 |
| Last modified | 2026-06-22 |
| Status | active (rev8 — HXC-107 code-reconciliation audit synced from Status.md rev8: internal-pkg 73→72, cli_agents 51→50, submodules 67-on-disk → 65 rowed + 2 documented exclusions) |
| Status detail | docs/features/Status.md |

Authoritative rollup of `docs/features/Status.md`. Anti-bluff (CONST-035 /
§11.4.83 / §11.4.107): the video-confirmed count rises ONLY as real analyzed
recordings land as DURABLE committed evidence under `docs/qa/` (NOT the rotatable
raw corpus — see the HXC-107 audit `docs/qa/HXC-107_ledger_audit.md`); an
un-recorded feature is honestly `📹 pending`/`no`, never bluffed green. The
`feature_video_evidence_gate.sh` gate (§11.4.86) mechanically fails if any
`confirmed` row's cited durable path goes missing. Counts below are computed over
the **567 twelve-column feature rows** in the live table.

## Rollup — by Overall

| Overall rollup | Count | Meaning |
|---|---|---|
| working-untaped | 337 | real + tested, no DURABLE analyzed video yet |
| partial | 168 | real but thin/unverified coverage OR partial wiring |
| gap | 49 | scaffold / untested / planned-not-landed |
| confirmed | 10 | real analyzed recording with DURABLE committed evidence (§11.4.83) — corrected down from 23 per the HXC-107 audit: the 13 rows the 2026-06-16 sweep marked confirmed cited the rotatable raw corpus (§11.4.154) with no durable anchor and were honestly reclassified `working-untaped` |
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
| none | 31 | untested surfaces — flagged honestly (e.g. `cmd/security_scan`) |
| §11.4.118/§11.4.135 guard/race upgrade | 36 pkgs (43 files) | standing regression + `-race` guards landed this session |

## Rollup — by Video confirmation (anti-bluff)

| 📹 Video | Count |
|---|---|
| confirmed (📹 yes, real analyzed recording) | 23 rows |
| pending / no (not yet recorded or non-visual) | 544 rows |
| `helixcode-*` real recordings produced so far | 30+ (incl. 2026-06-16 CLI/API/TUI/Web sweep) |

**Video-confirmation sweep 2026-06-16 (§11.4.153/§11.4.158)** — real DeepSeek,
fresh binaries, all analyzed: **CLI 9 features** (stream, generate, list-models,
command/os-exec, health, list-workers, notify, model+max-tokens, approval/permission),
**API 5** (health, models [7-model catalog], generate [tokens=203], auth
[register/login-JWT/bad-pw], tasks-crud [401+UUID]), **TUI 2** (llm-chat DeepSeek
"capital of Japan is Tokyo", navigation/theme tour), **Web 2 incl. SSE streaming**
(llm-console non-stream tokens=203, SSE "Python,JavaScript,Rust"). The `-stream`
recording caught a §11.4.108 STALE-BINARY break (`invalid character 'd'`) →
rebuilt `bin/cli` → fixed + §11.4.135 guard added — the anti-bluff payoff of
reading the recorded screen, not just producing a file. **GUI desktop/mobile
native-window video this sweep = host Screen-Recording TCC-blocked →
operator-attended §11.4.52 SKIP** (tracked, never faked green).

**Prior video-confirmed surfaces (rev2–5):** CLI (`/generate`,
`/models`, `/health`, themed brand banner), Web (LLM generate + themed), TUI
(themed real-LLM stream), Desktop (Fyne dashboard + chat, autonomous
software-painter capture), Android (connect + task list + themed). **iOS themed
re-record is OPERATOR-BLOCKED** (§11.4.52 — `CoreSimulatorService` denied write to
`/Volumes/T7`; host TCC change out of agent scope). Everything else is honestly
`pending` — the conductor owns video confirmation.

## Coverage completeness (rev8 code-reconciliation, verified vs live tree 2026-06-22)

- **Internal packages:** 72/72 dirs have ≥1 feature row (71 with production code +
  `internal/i18n_wiring`, the sole test-only dir). 234 internal-feature rows.
  (rev8: count corrected 73→72 per `ls -d helix_code/internal/*/` = 72 — the prior
  "73/73" was an off-by-one.)
- **cmd tools:** all 11 `helix_code/cmd/*` dirs rowed.
- **Client apps:** all 6 `applications/*` dirs (Desktop, Terminal-UI, iOS, Android,
  Aurora OS, Harmony OS) rowed; 8 client *surfaces* total counting CLI (`cmd/cli`) +
  Web (`internal/server`). iOS/Android confirmed buildable rev8 (real Gradle/Xcode
  chains + gomobile-bind artifacts present — prior "not buildable" claim struck).
- **HTTP API:** 18 endpoint groups rowed.
- **Submodules:** 67 on disk (`ls -d submodules/*/`) → 65 carry ≥1 `| submodule |`
  capability row + 2 documented exclusions (`docs_chain`, `challenges`) = 0 silently
  missing (rev8: `claude-toolkit` row added; 135 feature rows across 65 distinct
  names. The prior "50 inventoried / 55 rows" undercounted.)
- **Ported cli_agents:** 33 rows (20 landed, 3 partial, 10 planned — honest), over
  the 50-agent `cli_agents/` catalogue (= 50 `.gitmodules` entries; rev8: 51→50).

See `docs/features/Status.md` for the full per-feature table.
