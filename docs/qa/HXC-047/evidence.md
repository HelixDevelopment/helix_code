# HXC-047 — redis TestNewClient_WithDatabase: needs-live-Redis (no SKIP-OK §11.4.98) + i18n error contract drift
internal/redis TestNewClient_WithDatabase — redis_test.go:112: error "internal_redis_failed_connect: dial tcp
127.0.0.1:6379: connect: connection refused" does not contain "Redis". TWO findings: (a §11.4.98/§11.4.3) the
test silently requires a live Redis at :6379 with no SKIP-OK guard; (b) the i18n-keyed error no longer contains
the literal "Redis" the test asserts → contract mismatch. Fix: SKIP-OK guard when no Redis + reconcile the
assertion to the i18n key (or have the error surface the backend name). (HEAD 54ab4e95)
