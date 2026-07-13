# HXC-150 — QA evidence (§11.4.83)

**Item:** HXC-150 (Bug/Low) — tests/ddos env-var names mismatch .env.full-test causing false SKIP
**Fix commit:** helix_code `eefe78a2` (3 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T17:20:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)

## Root cause
The ddos harness `bootRealServer` read TEST_PG_*/TEST_REDIS_* env vars but .env.full-test exports
HELIX_DATABASE_*/HELIX_REDIS_* with different defaults (user helixcode vs helix, password
helixcode_test_password vs helix, db helixcode_test vs helix_test). So under the standard workflow
the suite false-SKIPped even with healthy infra.

## Fix
Moved envOr/envOrInt to always-compiled ddos_harness.go + added envOrHelix/envOrIntHelix helpers
(HELIX_ first → TEST_ fallback → hardcoded default). Updated bootRealServer to use HELIX_ keys.
6 unit tests prove the 3-tier fallback.

## Verification
go vet ./tests/ddos/... + go vet -tags=integration both clean. go test -count=1 -run TestEnvOr → PASS.
Anti-bluff smoke clean.
