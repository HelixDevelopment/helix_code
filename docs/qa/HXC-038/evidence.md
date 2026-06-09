# HXC-038 — docs_chain rebaseline affordance (engine extend + green)

**Captured:** 2026-06-09T14:05:59Z
**Type:** Bug · **Status:** Fixed (→ Fixed.md)
**Operator decision:** "Extend engine + green it carefully" (AskUserQuestion)

## Work
docs_chain `rebaseline` subcommand (db-to-md direction only) + `writeGuardStore`
that REJECTS any write to the authority DB + `rebaseline_test.go`. Commit `0c297c4`.

## Captured runtime evidence (real `go test`)
```
go test ./internal/runner/ -run 'TestReBaseline' -count=1 -v
--- PASS: TestReBaseline_NeverWritesAuthorityDB (0.00s)
--- PASS: TestReBaseline_WriteGuardRejectsAuthorityWrite (0.01s)
--- PASS: TestReBaseline_NeverExecutesSyncTransform (0.00s)
PASS
ok  	digital.vasic.docs_chain/internal/runner	0.197s
```
Full runner suite regression: `ok ... 1.622s` (no FAIL).

## Push
- pushed `ae2b411..0c297c4 main` → origin (github.com:vasic-digital/docs_chain), tip confirmed 0c297c4
