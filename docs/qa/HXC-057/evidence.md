# HXC-057 — recovery: wire unwired digital.vasic.concurrency require+replace
**Captured:** 2026-06-09T15:06:50Z · Bug · Fixed (→ Fixed.md)
## Defect
recovery/pkg/breaker/breaker.go imports digital.vasic.concurrency/pkg/breaker (existing code,
never wired per §11.4.124) but recovery/go.mod had no require/replace → go build ./... failed:
"no required module provides package digital.vasic.concurrency/pkg/breaker".
## Fix (wire-in, not delete §11.4.124)
Added `require digital.vasic.concurrency v0.0.0` + `replace digital.vasic.concurrency => ../concurrency`; go mod tidy settled go.sum.
## Captured runtime evidence
`go build ./...` → RECOVERY_BUILD_OK ; `go test ./pkg/breaker` → ok 0.754s
## Commit
recovery 105d0b4 (go.mod+go.sum), pushed ff c7efbcb..105d0b4 to all remotes.
