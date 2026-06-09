# HXC-053 — conversation build fix (CONST-052 lowercase replace path)

**Captured:** 2026-06-09T14:05:59Z
**Type:** Bug · **Status:** Fixed (→ Fixed.md)

## Defect
`submodules/conversation/go.mod` line 24 replace pointed at `../Messaging`
(capitalised) → `go build ./...` failed after the CONST-052 lowercase rename.

## Fix
go.mod line 24: `replace digital.vasic.messaging => ../messaging`.

## Captured runtime evidence (real `go build`)
```
cd submodules/conversation && go build ./...
CONV_BUILD_OK
```

## Commit + push
- commit `c9d6925` (go.mod only, §11.4.30)
- pushed `cd3dba2..c9d6925 main` → all upstreams
