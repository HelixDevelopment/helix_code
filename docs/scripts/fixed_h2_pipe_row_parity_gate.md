# fixed_h2_pipe_row_parity_gate.sh

**Revision:** 1
**Last modified:** 2026-06-16T00:00:00Z

## Overview

§11.4.135 standing regression guard for the §11.4.90 / §11.4.91 / §11.4.53
docs-tooling drift that hid **HXC-044** (`Bug | Obsolete (→ Fixed.md)`) from
`docs/Fixed_Summary.md`.

`docs/Fixed.md` is MIXED: a pipe table
(`| Closure | Title | Type | Status | Round | Commit(s) | Evidence |`) **AND**
H2 detail sections (`## HXC/ATM-NNN — …`). `scripts/generate_fixed_summary.sh`
reads ONLY the pipe table, so an H2 closure section with no matching pipe-table
row — and any `Obsolete (→ Fixed.md)` item the summary did not break out — was
invisible to the summary. HXC-044 had an H2 section + was Obsolete in the DB
but had no pipe row, so it was absent from `Fixed_Summary.md` twice over.

## What it asserts

- **(A)** every `docs/Fixed.md` H2 closure heading (`## HXC-NNN` / `## ATM-NNN`)
  has a matching pipe-table row keyed by `<ID>:` or `<ID> ` in the Title cell.
  Reported as a **WARNING with the full enumerated backlog** by default
  (the pipe table is hand-maintained and was found ~50 HXC items behind the H2
  sections — a pre-existing backlog outside the HXC-044 fix scope, surfaced per
  §11.4.118, never silently backfilled with guessed dates/commits per §11.4.6).
  Promote to a hard FAIL with `FIXED_PARITY_STRICT=1` once the backlog is
  repaired in a dedicated work item.
- **(B)** every `Obsolete (→ Fixed.md)` pipe-table item appears in
  `docs/Fixed_Summary.md`. **Hard FAIL** — this is the exact §11.4.90/.53 bug
  HXC-044 exercised and the invariant this guard primarily locks down.

## Prerequisites

`awk`, `grep`, `mktemp`, `bash`. No network, no credentials.

## Usage

```bash
scripts/gates/fixed_h2_pipe_row_parity_gate.sh
# RED-baseline (§11.4.115) — reproduce the pre-fix defect on a temp copy and
# assert the guard genuinely catches it (exit 0 = guard is not blind):
RED_MODE=1 scripts/gates/fixed_h2_pipe_row_parity_gate.sh
# Promote invariant (A) to a hard FAIL once the H2↔pipe-row backlog is repaired:
FIXED_PARITY_STRICT=1 scripts/gates/fixed_h2_pipe_row_parity_gate.sh
```

## Edge cases

- `RED_MODE=1` operates on copies under `$TMPDIR` (cleaned on `EXIT`); it never
  mutates the live `docs/Fixed.md` / `docs/Fixed_Summary.md`.
- Pre-existing H2↔pipe-row backlog is WARN-only by default so a clean GREEN is
  achievable for the HXC-044-scoped fix while the backlog stays visible.

## Internal behaviour

Parses pipe data rows by column (col1=Closure date, col2=Title, col3=Type,
col4=Status, ...; awk fields shift by one because each line starts with `|`).
Cross-checks H2 ids against Title-cell ids and Obsolete ids against the summary.

## Related scripts

- `scripts/generate_fixed_summary.sh` — the Fixed_Summary generator this guard
  protects (now counts `Obsolete (→ Fixed.md)` + lists Obsolete items).
- `scripts/gates/obsolete_details_gate.sh`, `scripts/gates/obsolete_colorize.sh`
  — §11.4.90 sibling gates.
- `scripts/regenerate-tracker-exports.sh` — HTML/PDF export refresh.

## Last verified

2026-06-16 — RED FAILs on the reproduced defect; GREEN (exit 0) post-fix with
the 74-item backlog enumerated as a WARN.
