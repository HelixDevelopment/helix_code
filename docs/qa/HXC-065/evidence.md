# HXC-065 — cache/pkg/postgres finite-TTL Set invisible to immediate Get (clock-domain skew)
**Captured:** 2026-06-09T17:05:54Z · Bug · Fixed (→ Fixed.md)
## RED (×3 vs real booted PG 15432)
pkg/postgres/integration_test.go:195 TestTTLExpiresOnRead "immediate Get returned \"\", want now"; siblings SetGet/ZeroTTL PASS.
## FACT root cause (live psql probe)
PG now()=21:54:05.346 vs Go time.Now()=21:54:04.676 — containerised PG ~671ms AHEAD of host Go clock.
Set stored expires_at = goNow+200ms = 21:54:04.876; Get filtered WHERE expires_at > NOW() (PG clock) → 04.876 > 05.349 = FALSE.
Two-clock-DOMAIN mismatch (both UTC, timestamptz correct) — any finite TTL < skew was dead-on-arrival. TTL=0 siblings store NULL (no compare).
## Fix
pkg/postgres/postgres.go Set: compute expires_at server-side via now() + make_interval(secs => $3::double precision) — collapses boundary + compared now() into ONE (PG) clock domain. TTL=0 still NULL. Expiry NOT disabled.
## GREEN (×3) + expiry-still-works
TestTTLExpiresOnRead PASS (sleeps 500ms past 200ms TTL → value gone); TestPurgeExpiredReclaimsRows + GCBackgroundSweep PASS; full file ×3 ok; go test ./... no regression.
Permanent §11.4.135 regression guard added: TestHXC065FiniteTTLReadableUnderClockSkew (HXC065_RED_MODE=1 reproduces, RED_MODE=0 standing GREEN).
Commit 68a44ac, pushed ff a852819..68a44ac all remotes.
