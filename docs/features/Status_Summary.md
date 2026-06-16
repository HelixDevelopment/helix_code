# HelixCode Feature Status — Summary

| | |
|---|---|
| Revision | 4 |
| Created | 2026-06-15 |
| Last modified | 2026-06-16 |
| Status | active (rev7 — 2026-06-16 video-confirmation sweep) |
| Status detail | docs/features/Status.md |

Authoritative rollup of `docs/features/Status.md`. Anti-bluff (CONST-035 /
§11.4.83 / §11.4.107): the video-confirmed count rises ONLY as real analyzed
recordings land in `/Volumes/T7/Downloads/Recordings`; an un-recorded feature is
honestly `📹 pending`/`no`, never bluffed green. Counts below are computed over
the **567 twelve-column feature rows** in the live table (Area-prefixed rows; +3
new CLI flag rows added by the 2026-06-16 sweep).

## Rollup — by Overall

| Overall rollup | Count | Meaning |
|---|---|---|
| working-untaped | 324 | real + tested, no analyzed video yet |
| partial | 168 | real but thin/unverified coverage OR partial wiring |
| gap | 49 | scaffold / untested / planned-not-landed |
| confirmed | 23 | real analyzed recording exists (📹 yes) — +13 from the 2026-06-16 sweep |
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

## Coverage completeness (rev6 gap-pass, 2026-06-16)

- **Internal packages:** 73/73 have ≥1 feature row (the last gap,
  `internal/i18n_wiring`, closed this rev). 234 internal-feature rows.
- **cmd tools:** all 11 `helix_code/cmd/*` dirs rowed.
- **Client apps:** all 6 surfaces (CLI, TUI, Web, Desktop, Android, iOS) +
  Aurora OS / Harmony OS rowed.
- **HTTP API:** 18 endpoint groups rowed.
- **Submodules:** 50 inventoried (55 capability rows; principal features).
- **Ported cli_agents:** 33 rows (20 landed, 3 partial, 10 planned — honest).

See `docs/features/Status.md` for the full per-feature table.
