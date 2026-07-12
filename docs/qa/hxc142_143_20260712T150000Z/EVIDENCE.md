# HXC-142 + HXC-143 — QA evidence (§11.4.83)

**Items:** HXC-142 (Bug/High) — `test/automation` doesn't compile; HXC-143 (Bug/High) — `test/e2e` doesn't compile
**Module:** `helix_code/` inner Go app (`dev.helix.code`)
**Fix commit:** helix_code `2a3a81e3` (11 test files; pushed to github+gitlab)
**Date (UTC):** 2026-07-12T15:00:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)
**Origin:** surfaced by the 2026-07-12 real-infra retest (§11.4.118); evidence `docs/qa/infra_retest_20260712_hxc122_138/`.
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-first, §11.4.142/§11.4.134 independent review, §11.4.1 (no-gutted-assertions), CONST-045 (no hardcoded SSH host).

## Root cause (FACT)

Both `-tags=automation` and `-tags=e2e` test packages failed to type-check/link. Two layers:
1. Duplicate symbols: `isRateLimitError`/`contains` (xai vs qwen), `getEnvOrDefault` (e2e).
2. Systemic provider-API drift — the tests referenced removed symbols: `llm.ProviderConfig`,
   `NewProviderManager`, `NewReasoningEngine`, `ReasoningRequest`, `GenerateWithReasoning`,
   `LLMRequest.ProviderType`/`.CreatedAt`, `FunctionDefinition`, `worker.HealthStatusHealthy`,
   `SSHWorkerPool.workers` (unexported), `env.Teardown`; plus a malformed `// go:build automation`
   (space) that silently compiled unconditionally.
(`go build` on a test-only package is a false-green no-op — the drift only shows under `go vet` /
`go test -c`, which is why it lay dormant.)

## Fix

Adapted the tests to the CURRENT API (verified to exist): `llm.NewProvider(llm.ProviderConfigEntry)`,
`LLMRequest.Reasoning *ReasoningConfig`, `llm.ToolFunction`, `ProviderMetadata` as
`map[string]interface{}`, `worker.WorkerHealthHealthy`, `SSHWorkerPool.GetWorkerStats`,
`env.TeardownTestEnvironment`, `//go:build`. Dup symbols consolidated to one declaration each.
Real assertions PRESERVED (TestRealAIReasoning/TestRealAIEndToEnd adapted, one strengthened with
`Usage.TotalTokens > 0`); bulk was mechanical field removal. All real-provider tests stay
env-key-GATED with honest `t.Skip` (no accidental API spend).

## Captured verification (RED→GREEN)

```
RED (parent ab773649, isolated worktree): go vet -tags=automation/-tags=e2e FAIL
  (isRateLimitError/getEnvOrDefault redeclared; undefined llm.ReasoningType...; workerPool.workers undefined)
GREEN (2a3a81e3): go vet (both tags) exit 0; go test -c (both tags) → real linked binaries;
  go build -tags=nogui ./cmd/... ./internal/... exit 0 (production untouched)
```

## Independent review (§11.4.142) — VERDICT: GO, zero blocking

Reviewer verified RED-on-parent→GREEN in an isolated worktree; confirmed every new symbol exists with
the claimed signature; NOT-GUTTED spot-check on ≥4 rewritten tests (real assertions preserved);
§1.1 proof — mutating `GetWorkerStats` +999 made all 3 SSHWorkerPool-adapted tests FAIL (live wires),
restored byte-identical; env-gating confirmed; scope = the 11 test files only, no docs/DB, no mutation
residue. One non-blocking nit: 3 SSHWorkerPool tests now assert `TotalWorkers==0` on an empty pool
(the original many-worker path used the unexported `.workers` field and thus NEVER compiled — no working
coverage lost; equivalent many-worker aggregation is covered in `internal/worker/ssh_pool_test.go`;
the alternative requires a live SSH host, forbidden-to-hardcode per CONST-045 → honest limitation).

## Honest boundary (§11.4.6)

These suites now COMPILE and their non-network cases run; the real-provider cases SKIP without keys.
Fully RUNNING them against live providers requires keys (real API spend — this env has some set) and,
for the automation binary, surfaced a separate runtime defect tracked as HXC-147 (OpenRouter nil-ptr).
