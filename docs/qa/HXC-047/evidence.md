# HXC-047 — redis TestNewClient_WithDatabase: needs-live-Redis (no SKIP-OK §11.4.98) + i18n error contract drift
internal/redis TestNewClient_WithDatabase — redis_test.go:112: error "internal_redis_failed_connect: dial tcp
127.0.0.1:6379: connect: connection refused" does not contain "Redis". TWO findings: (a §11.4.98/§11.4.3) the
test silently requires a live Redis at :6379 with no SKIP-OK guard; (b) the i18n-keyed error no longer contains
the literal "Redis" the test asserts → contract mismatch. Fix: SKIP-OK guard when no Redis + reconcile the
assertion to the i18n key (or have the error surface the backend name). (HEAD 54ab4e95)

## FIXED (commit 7c8c6c91) — §11.4.120 reconcile + §11.4.98 SKIP-OK
RED reproduced: redis_test.go:112 "internal_redis_failed_connect: dial tcp [::1]:6379: connect: connection refused" does not contain "Redis" → FAIL.
Root: internal/redis was CONST-046-migrated (redis.go:47 surfaces i18n msg-ID internal_redis_failed_connect; NoopTranslator echoes the ID in unit tests). FIX:
(1) reconciled assertion Contains("Redis")→Contains("internal_redis_failed_connect") (the stable surfaced id, matching sibling tests at lines 45/60);
(2) added t.Skip("SKIP-OK: #HXC-047 no live Redis ...") so the live-Redis success path is an honest skip not silent pass.
GREEN: go build exit 0; go test -count=3 -run TestNewClient_WithDatabase → SKIP 3/3 deterministic; full pkg ok 3x no flake.
