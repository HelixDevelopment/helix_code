# HXC-067 — inner internal/redis stress read TEST_REDIS_* not HELIX_REDIS_*
**Captured:** 2026-06-09T17:05:54Z · Bug · Fixed (→ Fixed.md)
RED: redis_stress_test.go:38-39 read TEST_REDIS_HOST/PORT (default :6379) → false 100%-error vs booted Redis on 16379.
Fix: firstNonEmptyEnv/Int helpers prefer HELIX_REDIS_* then legacy TEST_REDIS_* then default.
GREEN: go test -tags=integration ./internal/redis -run Stress → ok 1.778s vs booted Redis 16379.
