# HXC-061 — helix_agent legacy test stale memory.GetRelevant signature
**Captured:** 2026-06-09T16:13:17Z · Bug · Fixed (→ Fixed.md)
RED: vet tests/unit/debate_security_legacy/debate_security_test.go:335 not enough args in memory.GetRelevant — have(string,number) want(context.Context,string,int).
Current sig: reflexion.go:353 GetRelevant(ctx, query, limit). Fix: GetRelevant(context.Background(), "DROP TABLE", 5) (intent preserved).
GREEN: go vet ./tests/unit/debate_security_legacy/... exit 0; go test ... ok 0.334s; owned vet (cmd/internal/pkg/tests) exit 0.
Commit 25052066, pushed both remotes (github vasic-digital + HelixDevelopment).
