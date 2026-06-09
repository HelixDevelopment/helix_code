# HXC-052 — background_tasks build fix (CONST-052 lowercase replace paths)

**Captured:** 2026-06-09T14:05:59Z
**Type:** Bug · **Status:** Fixed (→ Fixed.md)

## Defect
`submodules/background_tasks/go.mod` replace directives pointed at capitalised
sibling dirs (`../Concurrency`, `../Models`) that no longer exist after the
CONST-052 lowercase rename → `go build ./...` failed on the case-sensitive layout.

## Fix
go.mod lines 38-39: `=> ../concurrency`, `=> ../models`.

## Captured runtime evidence (real `go build`)
```
cd submodules/background_tasks && go build ./...
BT_BUILD_OK
```

## Commit + push
- commit `46ceedb` (go.mod only, §11.4.30)
- pushed `f007922..46ceedb master` → all upstreams (origin/github/upstream/gitlab)
