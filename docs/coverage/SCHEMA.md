# docs/coverage/SCHEMA.md — COVERAGE_LEDGER.md row format

**Round 68 creation.** This document defines the schema for `COVERAGE_LEDGER.md`. Schema changes require an entry in the audit-trail table at the bottom of `COVERAGE_LEDGER.md` AND a bump of `SCHEMA_VERSION` below.

`SCHEMA_VERSION = 1.0.0`

## 1. Row format

Every data row in `COVERAGE_LEDGER.md` has exactly 11 columns:

```
| Submodule | Feature | Platform | I1 anti-bluff | I2 e2e-working | I3 doc-match | I4 no-issues | I5 doc-sync | I6 4-layer-tests | Overall | Notes |
```

### 1.1 Column definitions

| # | Column | Type | Required | Description |
|---|--------|------|----------|-------------|
| 1 | Submodule | string | yes | Path-from-repo-root identifier per `docs/improvements/submodule_owned.txt` (e.g. `dependencies/vasic-digital/AutoTemp`, `helix_qa`, `challenges`). |
| 2 | Feature | string | yes | Short description of the capability under audit (e.g. `grid-search temperature tuning`, `OpenCode CLI adapter`). May be `whole-module` for submodules audited as one unit. |
| 3 | Platform | string | yes | Comma-separated platform list from {`linux`, `macos`, `windows`, `ios`, `android`, `aurora-os`, `harmony-os`, `containers`, `headless`}. `all-platforms` permitted only for platform-agnostic Go code. |
| 4-9 | I1..I6 | status | yes | CONST-048 invariant status per §2 vocabulary below. |
| 10 | Overall | status | yes | Rollup: `PASS` only if every I1..I6 is `PASS` or `N/A`; otherwise `PARTIAL` or `UNCONFIRMED:`. Generator computes mechanically. |
| 11 | Notes | string | yes | MUST include either (a) CONTINUATION round reference (e.g. `round 37 wired Real backends`) for any PASS marker, OR (b) free-text gap description for non-PASS cells. Blank Notes column on a PASS row is a CONST-035 PASS-bluff. |

### 1.2 Row uniqueness

The tuple `(Submodule, Feature, Platform)` MUST be unique. Duplicate rows are a violation; collapse them by merging Notes.

### 1.3 Submodule coverage requirement

Every entry in `docs/improvements/submodule_owned.txt` MUST appear in at least one row. The generator's `--check` mode (`scripts/generate-coverage-ledger.sh --check`) exits non-zero if any owned submodule has zero ledger rows — this is the round 68 enforcement gate for CONST-048's "rows that quietly omit a platform are CONST-048 violations" mandate.

## 2. Status vocabulary

Closed set; values outside this list are violations.

| Symbol | Semantics | Evidence requirement |
|--------|-----------|----------------------|
| `PASS` | Invariant met in current cycle with captured runtime evidence. | Notes column MUST cite CONTINUATION round OR pasted evidence path. |
| `PENDING_FORENSICS:` | Work landed in source/tests but evidence not yet captured into ledger. | Notes SHOULD cite the commit SHA that introduced the work. |
| `OPERATOR-BLOCKED:` | Cannot complete without operator action (real LLM key, hardware, real network). | Notes MUST cite the operator dependency per §11.4.21 audit pattern. |
| `UNCONFIRMED:` | Not yet audited. Default for new rows. | Notes SHOULD describe what audit is needed. |
| `SKIP-OK: #<ticket>` | Intentionally not covered. | Notes MUST cite the ticket explaining the skip. |
| `PARTIAL` | Some platforms / scenarios covered but not all. | Notes MUST enumerate which platforms/scenarios pass vs. fail. |
| `N/A` | Invariant does not apply (e.g. I2 e2e-working for a build-tooling-only submodule). | Notes SHOULD justify the N/A. |

**Default cell status is `UNCONFIRMED:`** — never `PASS`. PASS without (a) CONTINUATION reference or (b) pasted evidence is a CONST-035 PASS-bluff equivalent in severity to a false-success test result (per Article XI §11.9).

## 3. Cross-reference points

The ledger does NOT duplicate evidence — it points to canonical sources.

| Invariant | Canonical source | Cross-check command |
|-----------|------------------|---------------------|
| I1 anti-bluff | CONTINUATION.md round narrative + commit SHA | `grep -n "round <N>" docs/CONTINUATION.md` |
| I2 e2e-working | Real-infra test logs (`/tmp/coverage-*`) + CONTINUATION captured evidence | `grep -rn "real <provider>\|wire evidence" docs/CONTINUATION.md` |
| I3 doc-match | User manual + `docs/COMPLETE_*.md` references | `grep -rn "<feature>" docs/COMPLETE_*` |
| I4 no-issues | `Issues.md` / `Issues_Summary.md` (if maintained) + source-tree `TODO(round-N)` / `BUG #NN:` markers | `grep -rn "TODO(round-\|BUG #" <submodule>/` |
| I5 doc-sync | Last-modified timestamp of user manual + submodule docs/ tree | `find <submodule>/docs -newer <submodule>/cmd -type f` |
| I6 4-layer-tests | Pre-build gate (`make verify-compile`) + post-build (`make test`) + runtime (challenges/e2e) + paired mutation (search for `_test.go` + sibling mutation gate) | `find <submodule> -name "*_test.go" \| xargs grep -l "SKIP-OK\|TestMutation"` |

### 3.1 No-bugs cross-check (I4) — HelixCode-specific note

HelixCode does not currently maintain a root-level `Issues.md` tracker (per round 32 audit). I4 cross-check therefore relies on:

1. Source-tree markers: `grep -rn "TODO(round-\|BUG #\|FIXME(round-\|XXX(round-" <submodule-path>/` — non-empty result = I4 cannot be `PASS`.
2. `docs/CONTINUATION.md` `Known issues` / `Deferred work` sections — if the submodule is named in a known-issue paragraph, I4 cannot be `PASS`.
3. Open BLUFFS in `docs/issues/BLUFFS.md` (if file exists) naming the submodule — I4 cannot be `PASS`.

Future rounds may introduce a per-submodule `Issues.md` tracker per CONST-057 / §11.4.16; the generator script will adopt it automatically when present.

## 4. Status rollup (column 10 "Overall")

Mechanical rollup computed by `scripts/generate-coverage-ledger.sh --refresh`:

```
if all(I1..I6 in {PASS, N/A}):           Overall = PASS
elif any(I1..I6 in {OPERATOR-BLOCKED:}): Overall = OPERATOR-BLOCKED:
elif any(I1..I6 in {PENDING_FORENSICS:}): Overall = PENDING_FORENSICS:
elif any(I1..I6 in {PARTIAL}):            Overall = PARTIAL
elif any(I1..I6 in {SKIP-OK: ...}):      Overall = PARTIAL  (any skip is a partial)
else:                                     Overall = UNCONFIRMED:
```

The generator NEVER promotes individual invariant cells — it only computes the Overall rollup from whatever cells the operator has hand-marked.

## 5. Anti-bluff guarantees (cascade from CONST-035 / Article XI §11.9)

1. **No PASS-by-default.** Every cell starts `UNCONFIRMED:` until hand-marked with evidence reference.
2. **No silent omissions.** Every owned submodule has at least one row; `generate-coverage-ledger.sh --check` fails otherwise.
3. **Evidence-required Notes.** PASS in any cell requires Notes content; blank Notes on a PASS row = violation.
4. **Status vocabulary is closed.** Values outside the §2 list = violation.
5. **Schema version bump.** Any change to column count / column order / new status symbol requires `SCHEMA_VERSION` bump + audit-trail entry.
6. **Cross-ledger consistency.** Feature names used here MUST be discoverable in either `docs/coverage/ledger.md` (F01..FNN catalogue) OR `docs/user_manual/ZERO_BLUFF_USER_MANUAL.md`; otherwise the feature is a phantom and the row is a violation.

## 6. Regeneration / refresh contract

`scripts/generate-coverage-ledger.sh` has two modes:

- `--scaffold` (default): emit a baseline ledger with one row per owned submodule, all invariants `UNCONFIRMED:`. Used when bootstrapping a fresh ledger.
- `--check`: walk the current `COVERAGE_LEDGER.md`, assert each owned submodule has ≥1 row, assert schema invariants, assert every PASS has Notes. Exit non-zero on any violation.

Neither mode auto-promotes cells. Neither mode rewrites operator-marked PASS values.

## 7. Audit-trail discipline

Schema changes append a row to `COVERAGE_LEDGER.md`'s audit-trail table with:
- Date (ISO YYYY-MM-DD)
- Author (agent ID or operator name)
- Round (close-outNN or round-NN)
- Schema version bump (e.g. `1.0.0 → 1.1.0`)
- One-line rationale

Initial schema (round 68) entry: see `COVERAGE_LEDGER.md` audit trail.
