# Retroactive QA evidence — commit `9970507d` (run-id `ref-hygiene`)

> **RECONSTRUCTED, AFTER-THE-FACT DOCUMENTATION (HXC-039, §11.4.83).**
> This file is NOT a captured-at-the-time end-user transcript. It is honest
> retroactive documentation created during the HXC-039 §11.4.83 back-fill,
> operator-authorised, to give a pre-existing feature commit its required
> `docs/qa/<run-id>/` evidence directory WITHOUT rewriting git history. No
> runtime output below is fabricated (§11.4.6 / §11.4.123).

| Field | Value |
|---|---|
| Commit SHA | `9970507d75c9c758aab5adfd673ae7460532b8bb` |
| Short SHA | `9970507d` |
| Authored | 2026-06-03T22:51:55+05:00 |
| Subject | `fix(phase3): helix_code cross-platform build (§11.4.81) + flatten ref-hygiene + helix_agent tidy pointer` |
| run-id basename | `ref-hygiene` (a verbatim substring of the commit subject) |

## What the commit changed (from `git show --stat`)

```
helix_code/internal/cache/cache.go                 |  2 +-
helix_code/.../i18n_wiring/wiring_integration_test.go | 2 +-
helix_code/internal/tools/shell/sandbox.go         | 42 +++++-----------------
helix_code/internal/tools/shell/sandbox_linux.go   | 32 +++++++++++++++++
helix_code/internal/tools/shell/sandbox_nonlinux.go | 28 +++++++++++++++
helix_code/tests/integration/cmd/p1f14_challenge/main.go | 10 ++++--
scripts/propagate-governance.sh                    |  1 -
submodules/helix_agent                             |  2 +-
8 files changed, 79 insertions(+), 40 deletions(-)
```

A §11.4.81 **cross-platform-parity** fix: the shell sandbox was split into
`sandbox_linux.go` / `sandbox_nonlinux.go` build-tag variants (so the
`helix_code` build works on non-Linux platforms), plus a cache fix, flatten
ref-hygiene, and a `helix_agent` pointer bump.

## Verification that exists for this commit

- The change is a build-tag split implementing §11.4.81 per-OS parity; its
  correctness is a **compile-on-each-platform** property
  (`make verify-compile` per supported OS) plus the existing sandbox unit
  tests under `internal/tools/shell/`.
- `tests/integration/cmd/p1f14_challenge/main.go` was updated in the same
  commit (integration challenge harness).
- **No captured runtime transcript was recorded at commit time** under
  `docs/qa/`. This directory is retroactive documentation.

## Honesty statement

Reconstructed documentation. NO faked PASS logs, NO fabricated runtime
output. The asserted verification is the cross-platform compile property +
existing sandbox tests, both visible in `git show 9970507d`.
