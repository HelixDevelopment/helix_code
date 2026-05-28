# `verify_qa_evidence.sh` — companion guide

**Revision:** 1
**Last modified:** 2026-05-28T00:00:00Z

| Field | Value |
|---|---|
| Script | `scripts/verify_qa_evidence.sh` |
| Authority | constitution submodule `Constitution.md` §11.4.83 (docs/qa/ end-user evidence mandate); §11.4.18 (script documentation mandate) |
| Status | active — ADVISORY (warn-mode) |
| Last verified | 2026-05-28 |

---

## Table of contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Usage examples](#usage-examples)
- [Edge cases](#edge-cases)
- [Internal behaviour](#internal-behaviour)
- [Why warn-mode](#why-warn-mode)
- [Related scripts](#related-scripts)

## Overview

`scripts/verify_qa_evidence.sh` is the **advisory** enforcement seam for
the §11.4.83 `docs/qa/` end-user evidence mandate. It scans recent
feature-shipping commits and prints a notice when a feature commit has no
matching `docs/qa/<run-id>/` directory carrying its end-user evidence
transcript.

The script is **warn-mode**: it ALWAYS exits 0 and never blocks a commit,
push, or build. It is NOT wired into any git hook.

## Prerequisites

- Run from inside the HelixCode git repository (any cwd — the script
  resolves the repo root via `git rev-parse --show-toplevel`).
- `git` and POSIX coreutils on `PATH`.
- `bash` (the shebang is honest — the script uses bash and `set -u`).

## Usage examples

Scan the default window (last 20 commits):

```
scripts/verify_qa_evidence.sh
```

Scan a custom window (last 50 commits):

```
scripts/verify_qa_evidence.sh 50
```

Expected output is a per-commit advisory report ending with a
`RESULT: advisory scan complete (exit 0 — warn-mode).` line.

## Edge cases

- **Not in a git repo** — prints a notice and exits 0.
- **No `docs/qa/` tree** — every feature commit is reported as a WARN
  (no run-id directories exist to match against); still exits 0.
- **Pure refactor that touches watched paths** — may produce a false
  positive WARN. Acceptable for a warn-mode advisory notice.
- **Run-id matched by commit subject** — a commit whose subject contains
  a known `docs/qa/<run-id>/` directory name (e.g. `HXC-019`) is reported
  `ok`.

## Internal behaviour

1. Resolves repo root and `cd`s into it.
2. Enumerates existing `docs/qa/<run-id>/` directories.
3. Walks the last N commits; a commit is "feature-shipping" if it touches
   non-test code under `helix_code/internal/**`, `helix_code/cmd/**`, or
   `helix_code/applications/**` (excluding `*_test.go`).
4. For each feature commit, checks whether its subject references a known
   run-id directory; prints `ok` or `WARN` accordingly.
5. Prints a summary and ALWAYS exits 0.

## Why warn-mode

§11.4.83 operative rule (5) describes a CI / release gate that refuses to
tag a version with a feature-shipping commit lacking its
`docs/qa/<run-id>/` directory. The operator has NOT yet authorised that
hard gate for HelixCode. Establishing the convention first (this tree +
README + seed example + advisory scanner) lets the discipline take root
without disrupting in-flight work. **Promotion of this scanner to a
blocking commit/release gate is a future operator decision.**

## Related scripts

- `docs/qa/README.md` — the convention this script enforces.
- `scripts/verify-governance-cascade.sh` — the cascade gate exercised in
  the `docs/qa/HXC-016/` seed example.
