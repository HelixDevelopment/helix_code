# HXC-062 — helix_specifier pkg/metrics lock-copy (vet copylocks)
**Captured:** 2026-06-09T16:13:17Z · Bug · Fixed (→ Fixed.md)
RED: go vet metrics.go:143 assignment copies lock; :163 return copies lock — Metrics embeds sync.RWMutex copied by value in Snapshot().
Fix: added mutex-free MetricsSnapshot struct; Snapshot() returns it (field-by-field deep-copy under RLock) — no lock copied. Public API intent preserved.
GREEN: go vet ./... exit 0; go build exit 0; go test ./pkg/metrics -race ok 1.568s; full ./... -short all 29 pkgs ok.
Commit 57035ab, pushed github+gitlab.
