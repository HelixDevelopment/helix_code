# HXC-146 — QA evidence (§11.4.83)

**Item:** HXC-146 (Task/Med) — challenge runner REST interface drives real server API
**Source commit:** helix_code `e5adc0fc` (4 files)
**Review:** Direct conductor review — GO (no subagent needed for this scope)
**Date (UTC):** 2026-07-12T19:10:00Z
**Closure vocab:** Fixed (§11.4.33, Task)

## What changed
executeREST now POSTs to the real HelixCode server /api/v1/llm/generate via helixcode_server_client.go,
using ChallengeConfig.HelixCodeHost/Port/Auth (previously dead code). Unreachable server → honest
StatusSkipped (§11.4.3). cli/tui/websocket interfaces unchanged (operator decision deferred).

## Review findings
- Diff scope: 4 files only (executor.go, executor_rest_server_test.go, helixcode_server_client.go, types.go)
- StatusSkipped: confirmed at types.go:178-184
- Tests: ok dev.helix.code/tests/e2e/challenges 3.147s (pre-existing stale test-results file unrelated)
- Wire shape: POST /api/v1/llm/generate with real request body, real response parsing
