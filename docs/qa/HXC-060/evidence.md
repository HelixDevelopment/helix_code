# HXC-060 — debate_orchestrator challenges/runner ctx-leak (vet)
**Captured:** 2026-06-09T16:13:17Z · Bug · Fixed (→ Fixed.md)
RED: go vet main.go:516 cancel not used on all paths; :571 return reaches without using cancel.
Fix: cancel() before each of the 4 early-continue validation paths. GREEN: go vet ./... clean exit 0, go build clean.
Commit c82af2f (same as HXC-059), pushed all remotes.
