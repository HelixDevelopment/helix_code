# Infra full-test-type run — SUMMARY (2026-07-12, operator-authorized)

Real infra booted via rootless podman 5.7.1 (no docker/sudo — §11.4.161), `make test-infra-up`
(docker-compose.full-test.yml, §11.4.76). **17 containers healthy** (postgres·redis·ollama+llama2:7b·
qdrant·weaviate·chromadb·cognee·memcached·mock-llm·mock-slack·selenium·chromedp·ssh+3 workers·multicast).
Teardown clean (no `helixcode-*` containers/volumes/network left). Raw per-suite logs in this dir.

## Per-test-type results (real infra, -count=1)
| Suite | PASS | FAIL | SKIP | Note |
|---|---:|---:|---:|---|
| memory | 3 | 0 | 11 | 11 SKIP need server:8080 (hardcoded) |
| security | **272** | 0 | 8 | 8 SKIP need server:8080 |
| automation (RUN_*=true) | 2 | timeout | 0 | timeout = `RUN_REAL_EXECUTION` git-clones vllm/etc (infeasible) — NOT a product bug |
| integration (-tags=integration ./...) | 189 pkg ok | 12 pkg | 146 subtest | mixed (below) |
| e2e | -list=6 | `-all` undefined | — | broken target |

## GENUINE new defects (reproduce on current tree — TRACK + FIX)
- **INFRA-1 (real product bug):** `internal/llm` `NewAzureProvider` returns nil-error on missing endpoint → `TestAzureProvider_NewWithoutEndpoint` nil-ptr SIGSEGV kills the whole binary. Product missing validation + test missing `require.Error` guard (`azure_provider_test.go:86`). → W5 fix stream.
- **INFRA-2 (stale test §11.4.120):** `test/integration/integration_test.go:75` refs removed `llm.ProviderConfig`/`NewProviderManager` → go vet exit 1. → W5.
- **INFRA-3:** `make test-e2e-full` runs `cmd/runner -all` but the runner defines NO `-all` flag → exit 2. → W5.
- **INFRA-4:** `internal/cognee` auth/bearer tests (`TestClientBearerLogin`, `TestClientAPIKeyHeader/WithoutAPIKey`, `TestClientHTTP_Stress_*`) — "login endpoint hit 0 times / bearer token not cached" (infra-independent, stable auth drift). → follow-up item.
- **INFRA-5:** compose `helixcode-server` build broken — `build.dockerfile: Dockerfile.test` resolves to `helix_code/Dockerfile.test` (missing; file is at repo root) → no in-stack server; native binary can't bind :8080 (foreign service holds it, §11.4.174) + Jul-8 binary ignores `--config`. Server-black-box tests SKIP honestly. → follow-up item.
- **HXC-124 root cause (reported):** HelixQA `HTTPExecutor` defaults `TokenField="session_token"`, but auth validates the JWT returned in the `token` field → 401 on every authed route. Fix consumer-side (construct the executor for HelixCode banks with `TokenField:"token"` + HTTPBaseURL + registered-user creds); do NOT change the decoupled submodule default (§11.4.28/§11.4.51). (HXC-035 display_name skip-reason is stale — already fixed.)

## Environment/mock (not core bugs)
- `internal/session TestPreWarm_Integration_RealAPI` 401 from REAL Anthropic w/ mock key (should SKIP).
- `internal/llm TestAnthropicProviderFullAutomation/*` + `TestCogneeRealLLMTestSuite/MemoryStorage_*` — mock-llm doesn't fully emulate providers.
- applications/{aurora_os,desktop,harmony_os} build-fail under integration tag (need -tags=nogui) — expected.

## Verdict
§11.4.169 infra-gated coverage genuinely runs against real infra (security 272 PASS is the headline).
HXC-122 = infra boots + tests run (the skip-by-default gap is server:8080-hardcoding, a fixable test-config issue). Real defects INFRA-1..5 + HXC-124 tracked for fix.
