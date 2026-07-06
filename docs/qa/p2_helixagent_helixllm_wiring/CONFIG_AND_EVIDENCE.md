# P2 — HelixAgent `helixllm` provider → local HelixLLM router wiring

**Branch:** `feature/helixllm-full-extension` (submodule `submodules/helix_agent`, module `dev.helix.agent`, go 1.26)
**Scope:** BUILD-layer + config wiring. Runtime proof (real completion) is DEFERRED (§11.4.108) — see bottom.

## What was already true (verified in code, §11.4.6)
- `internal/llm/providers/helixllm/provider.go` implements `llm.LLMProvider`
  (`Complete`, `CompleteStream`, `HealthCheck`, `GetCapabilities`, `ValidateConfig`).
- It POSTs OpenAI-compatible `/v1/chat/completions` (+ `/v1/embeddings`, `/v1/models`,
  `/internal/health`) — matches the HelixLLM gateway (`HelixLLM internal/gateway/router.go`
  serves `POST /v1/chat/completions`).
- Endpoint was already env-overridable via `HELIX_LLM_ENDPOINT`, BUT the default
  `https://localhost:8443` was **hardcoded in two places** (provider.go + provider_registry.go:1606).

## Verified HelixLLM contract (reconciling the stale `UseLlamaCpp`)
- HelixLLM gateway does **NOT** read `X-Helix-LLM-Use-LlamaCpp` (its handlers consume only
  `Accept-Language` / `Authorization` / `Accept`). The header HelixAgent sends is a **no-op**.
- The authoritative llama.cpp toggle is **server-side**: HelixLLM's
  `internal/shared/config/config.go` → `LlamaServerEmbed bool env:"HELIX_LLAMA_SERVER_EMBEDDED" default:"true"`,
  set on the HelixLLM process — NOT communicated per-request from HelixAgent.

## Change made
1. **Configurable endpoint, no hardcoded host (CONST-045).** Added exported `DefaultEndpoint`
   as the single source of truth + a `resolveEndpoint(explicit)` precedence helper:
   `explicit cfg.Endpoint` → `HELIX_LLM_LOCAL_OPENAI_ENDPOINT` → `HELIX_LLM_ENDPOINT` → `DefaultEndpoint (https://localhost:8443)`.
   The TLS :8443 default is unchanged (not broken).
2. **New local plain-HTTP OpenAI seam.** `HELIX_LLM_LOCAL_OPENAI_ENDPOINT` — first-class,
   higher-precedence env var to point at the LOCAL plain-HTTP OpenAI-compatible HelixLLM router
   (e.g. the max-perf llama.cpp image serving on `http://localhost:8080`).
3. **provider_registry.go** no longer hardcodes `https://localhost:8443`; it passes `baseURL`
   (empty ⇒ provider resolves via env/default) and logs `provider.Endpoint()` (new accessor)
   so the log reports the real resolved endpoint (§11.4.6).
4. **Reconciled `UseLlamaCpp` docs** to state the verified contract (header no-op on current
   HelixLLM; real toggle = server-side `HELIX_LLAMA_SERVER_EMBEDDED`). Header kept for
   forward-compat (harmless if unread), not silently removed (§11.4.122/§11.4.124).

## Exact config a HelixCode / CLI-agent uses to reach local HelixLLM via HelixAgent
- **Enable the provider:** `USE_HELIX_LLM=true`
- **Point at the local plain-HTTP OpenAI router:** `HELIX_LLM_LOCAL_OPENAI_ENDPOINT=http://localhost:8080`
  (or the general `HELIX_LLM_ENDPOINT=http://localhost:8080`)
  - default (unset) stays `https://localhost:8443` (TLS gateway); for that add `HELIX_LLM_TLS_SKIP_VERIFY=true` for self-signed certs.
- **Provider id:** `helixllm` (registry type + name)
- **Model id surface:** `helixllm-default` by default (or set `HELIX_LLM_MODEL` / the model requested per call —
  the local OpenAI router's model id is forwarded as `model` in the `/v1/chat/completions` body).
- **API key (optional):** `HELIX_LLM_API_KEY` → `Authorization: Bearer <key>` (omit for an unauthenticated local router).

## Build / test evidence (this session, go 1.26.4)
- `go build ./internal/... ./cmd/...` → **exit 0** (see `go_build_app.txt`; empty = success, no output).
- `go build ./internal/llm/providers/helixllm/` → **exit 0**.
- `go test ./internal/llm/providers/helixllm/...` → **`ok  dev.helix.agent/internal/llm/providers/helixllm  0.005s`** (see `go_test_helixllm.txt`).
- `go vet ./internal/llm/providers/helixllm/ ./internal/services/` → **exit 0**.
- Note: `go build ./...` needs a `go mod tidy` (module-graph gap) + fails inside the vendored
  `cli_agents/continue/...` reference tree — BOTH are **pre-existing on `main`** (verified), unrelated to this change.
  The tidy was run throwaway to resolve the graph, then `go.mod`/`go.sum` restored so the commit is scoped to the 2 source files.

## DEFERRED runtime proof (§11.4.108 — honest boundary)
The end-to-end proof (HelixAgent `helixllm` provider → running local HelixLLM router → real
non-simulated completion) requires HelixLLM running (main stream is bringing up the 30B model).
NOT claimed as working here — follow-up: with the router up, `POST` a real prompt through the
HelixAgent OpenAI-compatible server and capture the real completion body.
