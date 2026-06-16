# HelixCode Feature Status — Summary

| | |
|---|---|
| Revision | 3 |
| Created | 2026-06-15 |
| Last modified | 2026-06-16 |
| Status | active (rev6 completeness gap-pass) |
| Status detail | docs/features/Status.md |

Authoritative rollup of `docs/features/Status.md`. Anti-bluff (CONST-035 /
§11.4.83 / §11.4.107): the video-confirmed count rises ONLY as real analyzed
recordings land in `/Volumes/T7/Downloads/Recordings`; an un-recorded feature is
honestly `📹 pending`/`no`, never bluffed green. Counts below are computed over
the **564 twelve-column feature rows** in the live table (Area-prefixed rows).

## Rollup — by Overall

| Overall rollup | Count | Meaning |
|---|---|---|
| working-untaped | 333 | real + tested, no analyzed video yet |
| partial | 169 | real but thin/unverified coverage OR partial wiring |
| gap | 49 | scaffold / untested / planned-not-landed |
| confirmed | 10 | real analyzed recording exists (📹 yes) |
| n/a (test-support) | 3 | mocks/testutil/pprofutil — not user features |
| **Total feature rows** | **564** | |

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
| confirmed (📹 yes, real analyzed recording) | 10 rows |
| pending / no (not yet recorded or non-visual) | 554 rows |
| `helixcode-*.mp4` real recordings produced so far | 12 |

**Video-confirmed surfaces (real analyzed recordings):** CLI (`/generate`,
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
