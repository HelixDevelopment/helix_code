# Retroactive QA evidence — commit `5c5c44bc` (run-id `round468d`)

> **RECONSTRUCTED, AFTER-THE-FACT DOCUMENTATION (HXC-039, §11.4.83).**
> This file is NOT a captured-at-the-time end-user transcript. It is honest
> retroactive documentation created during the HXC-039 §11.4.83 back-fill,
> operator-authorised, to give a pre-existing commit its required
> `docs/qa/<run-id>/` evidence directory WITHOUT rewriting git history. No
> runtime output below is fabricated (§11.4.6 / §11.4.123).

| Field | Value |
|---|---|
| Commit SHA | `5c5c44bcfcd648b776320561534a2d9a82c220ff` |
| Short SHA | `5c5c44bc` |
| Authored | 2026-06-04T12:50:46+05:00 |
| Subject | `fix(meta): Wave5 — compose-test creds (CONST-042) + config BackupConfig MkdirAll + 2 pointer bumps (round468d)` |
| run-id basename | `round468d` (a verbatim substring of the commit subject) |

## What the commit changed (from `git show --stat`)

```
docker-compose.test.yml                            | 21 +++++++++------
docs/CONTINUATION.md                               |  8 ++++++
helix_code/internal/config/config.go               |  8 ++++++
helix_code/internal/config/fresh_install_save_test.go | 30 ++++++++++++++++++++++
submodules/doc_processor                           |  2 +-
submodules/helix_qa                                |  2 +-
6 files changed, 61 insertions(+), 10 deletions(-)
```

A "Wave5" meta commit: compose-test credential hygiene (CONST-042), a
`BackupConfig` `MkdirAll` fix in `internal/config/config.go`, and 2 submodule
pointer bumps. The production-code touch tripping the feature heuristic is
the `config.go` `MkdirAll` fix.

## Verification that exists for this commit

- **Paired unit test added in the SAME commit:**
  `internal/config/fresh_install_save_test.go` (30 new lines) covers the
  config `BackupConfig`/`MkdirAll` save path (`go test ./internal/config/...`).
- The compose-test credential change is config hygiene (CONST-042), not an
  end-user runtime feature.
- **No captured runtime transcript was recorded at commit time** under
  `docs/qa/`. This directory is retroactive documentation.

## Honesty statement

Reconstructed documentation. NO faked PASS logs, NO fabricated runtime
output. Asserted verification = the in-commit config unit-test delta visible
in `git show 5c5c44bc`.
