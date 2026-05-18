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

## Release-gate test runner (round 74)

`scripts/release-gate-test.sh` is the EXECUTION counterpart to `generate-coverage-ledger.sh`. Where the ledger tracks STRUCTURAL coverage (what we believe is tested, with captured evidence), the release-gate runner ACTUALLY EXECUTES tests against every owned-org submodule and reports an honest aggregate signal.

### What it covers

- Walks every submodule listed in `docs/improvements/submodule_owned.txt` (round 56 canonical roster).
- For each submodule with a `go.mod`, runs `GOMAXPROCS=2 nice -n 19 go test -count=1 -race -timeout=180s ./...` inside the submodule root.
- Captures stdout+stderr to per-submodule log files under `$RELEASE_GATE_LOG_DIR` (default `/tmp/release-gate-<pid>/`).
- Parses PASS/FAIL/SKIP line counts.
- Detects bare `--- SKIP:` lines without `SKIP-OK:` markers — these are CONST-035 skip-bluff violations.

### What it does NOT cover

- The inner `helix_code/` Go module — that's covered by `make test-full` per CLAUDE.md §3.4 (its own four-layer floor with real PostgreSQL + Redis + Ollama infra).
- Non-Go submodules (`assets/`, `github_pages_website/`) — these are SKIP-NO-GOMOD, legitimately untested by this runner. Their coverage lives elsewhere (the asset/static-site pipelines).
- Integration / E2E tests requiring docker-compose — those need `make test-infra-up` per submodule.
- Mutation tests — paired-mutation runs are tracked in `COVERAGE_LEDGER.md` cell-by-cell.

### Exit code semantics

| Exit | Meaning |
|------|---------|
| 0 | All owned submodules either PASSed (`go test` succeeded) or were legitimately SKIP-NO-GOMOD. Release gate GREEN. |
| 1 | At least one submodule FAILed OR at least one bare SKIP detected. Release gate RED — see the human summary or `--json` output for the failing list. |
| 2 | Invalid flag, missing required file, or `--check` self-validation failed. |

### Flags

| Flag | Effect |
|------|--------|
| `--json` | Emit machine-readable JSON summary for CI integration. Includes per-submodule + aggregate counts + log_dir path. |
| `--quick` | Stop on first FAIL. Useful for fast CI signal where any failure should block. |
| `--only=<glob>` | Restrict to submodules matching the shell glob (e.g. `--only='dependencies/vasic-digital/*'`). Useful for targeted re-runs. |
| `--check` | Self-validate script + paths without running real tests. Used by smoke-tests and CI sanity. |

### Recommended invocation pattern

```bash
# Manual pre-release sweep (full, human-readable):
bash scripts/release-gate-test.sh

# CI integration (machine-readable, fail-fast):
bash scripts/release-gate-test.sh --json --quick > release-gate.json
jq '.exit_code' release-gate.json   # 0 = green, 1 = red

# Targeted re-run after fixing a specific submodule:
bash scripts/release-gate-test.sh --only='dependencies/vasic-digital/Cache'

# Sanity check that the script itself is healthy (no real test runs):
bash scripts/release-gate-test.sh --check
```

### Cross-reference with the coverage ledger

The two tools answer different questions about CONST-048 invariant 6:

- `COVERAGE_LEDGER.md` answers *"do we BELIEVE this submodule × platform combo has captured runtime evidence?"* (structural state).
- `release-gate-test.sh` answers *"does `go test ./...` ACTUALLY pass for this submodule RIGHT NOW?"* (live execution).

When the ledger says `PASS` for invariant I6 on a submodule but `release-gate-test.sh` reports FAIL for that same submodule, the ledger is stale — a §11.4 PASS-bluff at the governance layer. Operator demotes the cell to `PENDING_FORENSICS:` or `OPERATOR-BLOCKED:` until reconciled.

### CONST-048 invariant 6 — closure note (round 74)

Round 68 landed the STRUCTURAL view of invariant 6. Round 74 lands the EXECUTION view. Together they constitute CONST-048 invariant 6's meta-repo-level closure. The inner-module level was already covered by `make test-full` per CLAUDE.md §3.4 + the round 41 feature-axis `ledger.md`.

The verbatim 2026-05-19 operator mandate that motivated round 74 (preserved per CONST-049 §11.4.17 in the script header):

> "all existing tests and Challenges do work in anti-bluff manner - they MUST confirm that all tested codebase really works as expected! We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completition and full usability by end users of the product!"

## See also

- `scripts/release-gate-test.sh` — programme-wide release-gate test runner (round 74; execution counterpart)
- `scripts/generate-coverage-ledger.sh` — scaffold generator + violation checker (round 68; structural counterpart)
- `scripts/regenerate-coverage-ledger.sh` — pre-existing feature-axis regenerator (round 41+)
- `docs/coverage/ledger.md` — pre-existing feature-axis ledger
- `docs/coverage/SCHEMA.md` — full schema documentation
