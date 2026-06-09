# HXC-066 — inner internal/database integration hardcoded localhost:5433
**Captured:** 2026-06-09T17:05:54Z · Bug · Fixed (→ Fixed.md)
RED: 5 Config{} blocks hardcoded localhost:5433/helix_test (no env) → internal_database_ping_failed vs booted PG (5433 closed).
Fix: added testDBConfig() + firstEnv() reading DB_*/HELIX_DATABASE_* with legacy defaults; replaced all 5 blocks.
GREEN: go test -tags=integration ./internal/database -run Integration → ok 0.801s vs booted PG 15432.
