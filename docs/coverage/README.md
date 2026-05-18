# docs/coverage/ — CONST-048 Four-Layer Coverage Ledger Suite

**Round 68 creation** — closes 50+ round deferred governance debt for the CONST-048 submodule × feature × platform × invariant ledger. Complements the pre-existing feature-axis `ledger.md` (round 41+) with a submodule-axis, fine-grained view.

## What lives here

| File | Purpose | Edited by |
|------|---------|-----------|
| `COVERAGE_LEDGER.md` | The main ledger. Row = (submodule, feature, platform); columns = the 6 CONST-048 invariants + Overall + Notes. | Operator (hand-marks PASS) + generator script (refreshes scaffold) |
| `SCHEMA.md` | Row format, status vocabulary, cross-reference points, invariant definitions, regeneration rules. | Operator on schema changes |
| `README.md` | This file — explains the suite. | Operator |
| `ledger.md` | Pre-existing feature × invariant rollup (round 41+). Complementary, NOT superseded. | `scripts/regenerate-coverage-ledger.sh` |
| `decoupling_review.md` | CONST-051(B) decoupling audit. | Operator |
| `rename_safety_guardrails.md` | CONST-052 rename-safety capture. | Operator |

## Why two ledgers?

`ledger.md` (round 41) answers: *"For each user-facing feature F01..F30, what is the test posture across all six invariants?"* — useful for release-readiness sweeps focused on the end-user product surface.

`COVERAGE_LEDGER.md` (round 68) answers: *"For each owned-org submodule we ship, what features does it expose, and what is the test posture per platform per invariant?"* — useful for CONST-051(A) equal-codebase audits (does HelixSpecifier get the same engineering attention as `helix_code/`?) and CONST-047 recursive-application audits.

Both ledgers MUST stay in sync with reality — divergence between them surfaces gaps in either the feature roster (user manual) or the submodule roster (`docs/improvements/submodule_owned.txt`).

## CONST-048 invariants (verbatim from constitution §11.4.25)

| # | Invariant | Sourced from |
|---|-----------|--------------|
| I1 | Anti-bluff posture with captured runtime evidence | CONST-035 / Article XI §11.9 |
| I2 | Proof of working capability end-to-end on target topology (no mocks beyond unit tests) | CONST-050(A) |
| I3 | Implementation matches the documented promise | §11.4.12 |
| I4 | No open issues / bugs surfaced | Issues.md / Issues_Summary.md cross-check |
| I5 | Full documentation in sync | §11.4.12 |
| I6 | Four-layer test floor (pre-build + post-build + runtime + paired mutation) | §11.4 paired-mutation discipline |

## Regeneration cadence

Triggers (per constitution §11.4.25 release-gate sweep):
1. **Per-round close-out** — the round agent appends evidence rows for the submodule it touched.
2. **Pre-release sweep** — operator runs `bash scripts/generate-coverage-ledger.sh` to refresh the scaffold + verify every owned submodule has at least one row (exit non-zero if missing).
3. **Post `constitution/` pull** (CONST-055) — re-run the generator to surface any new submodule added to `submodule_owned.txt`.

## Status vocabulary (one-line summary; full table in SCHEMA.md)

| Symbol | Meaning |
|--------|---------|
| `PASS` | Captured runtime evidence in current cycle |
| `PENDING_FORENSICS:` | Work landed but evidence not yet pasted into the ledger |
| `OPERATOR-BLOCKED:` | Needs operator action (real LLM key, hardware, network) — §11.4.21 audit applies |
| `UNCONFIRMED:` | Not yet audited / no evidence captured |
| `SKIP-OK: #<ticket>` | Intentionally not covered, with documented reason |
| `PARTIAL` | Covered for some platforms / scenarios but not all — see Notes |
| `N/A` | Invariant does not apply to this row |

**Anti-bluff guarantee** (CONST-035 / Article XI §11.9): default cell status is `UNCONFIRMED:` — never `PASS`. PASS marker requires either (a) a CONTINUATION.md round reference quoting the captured evidence, or (b) an in-ledger evidence snippet pasted in the Notes column. PASS without one of these is a §11.4 PASS-bluff at the governance layer, severity equivalent to a false-success test result.

## Cross-reference points (per SCHEMA.md §3)

- **CONTINUATION.md** — round-by-round narrative is the canonical source for PASS justifications.
- **Issues.md / Issues_Summary.md** — feeds I4 (no-open-bugs). HelixCode does not currently maintain a root-level Issues tracker; cross-check is via grep of `// TODO(round-N):` / `// BUG #NN:` markers in source.
- **SKIP-OK markers** in `*_test.go` — feeds I6 (mutation-test partial-coverage signals).
- **`docs/improvements/submodule_owned.txt`** — canonical roster of owned submodules; one row per submodule minimum.
- **CONST-051(C) layout audit** in `ledger.md` — feeds I3 (decoupling matches the documented promise of project-not-aware reuse).

## Initial population approach (round 68)

Per the round 68 brief and CONST-035 anti-bluff discipline: the ledger is populated **conservatively**.

- `PASS` markers landed ONLY where CONTINUATION.md round narrative documents captured evidence (≈ a dozen submodule × feature rows from rounds 23, 37, 41, 53, 60, 62, 63, 64, 65).
- Every other cell is `UNCONFIRMED:` until a future round captures evidence and promotes it.
- Auto-populating every cell `PASS` would itself be a CONST-035 PASS-bluff at the governance layer — explicitly forbidden by the brief.

Subsequent rounds promote cells `UNCONFIRMED:` → `PASS` ONLY by adding (a) the CONTINUATION round reference, AND (b) the captured evidence path/snippet in Notes. The generator NEVER auto-promotes.

## See also

- `scripts/generate-coverage-ledger.sh` — scaffold generator + violation checker (round 68)
- `scripts/regenerate-coverage-ledger.sh` — pre-existing feature-axis regenerator (round 41+)
- `docs/coverage/ledger.md` — pre-existing feature-axis ledger
- `docs/coverage/SCHEMA.md` — full schema documentation
