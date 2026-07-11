# MCP Gateway LIVE End-to-End Re-Proof (§11.4.5)

**Track:** T1/feature/helixllm-full-extension
**Run:** 20260711T132250Z (UTC)
**Repo:** `/home/milos/Factory/projects/tools_and_research/helix_code`
**Under test:** `submodules/helix_llm` `cmd/mcp-gateway` + `internal/mcpgateway` (commit `895788c5034c47236b117c62df33465d1d2783a5`)
**Scope:** MCP gateway API surface only. Complements the dual-wire (OpenAI-compatible HTTP) facade proof captured earlier today — this run closes out the MCP-transport half of the API-surface V&V.

## Purpose

Re-prove, with fresh captured evidence and a real `go-sdk` v1.6.1 MCP client (not a curl-faked JSON-RPC POST), that the Streamable-HTTP MCP gateway genuinely works end-to-end against the live coder at `:18434`:

1. Bearer-auth enforcement (no token → 401).
2. `tools/list` returns both real tools live from the server.
3. `tools/call helixllm_generate` with a **fresh per-run nonce** proxies to the live coder and the coder's real output echoes that exact nonce — proof this is a genuine live round-trip, not a cached/canned/simulated response (BLUFF-001 anti-bluff class).
4. `tools/call helixllm_list_models` returns the live coder's real model id (CONST-036 — never a hardcoded list).
5. The coder at `:18434` is read-only throughout — queried before, during (by the gateway), and after, never restarted (§11.4.119 / §11.4.122).

## Method

A self-contained Go harness (`harness/main.go`, go-sdk v1.6.1) that:

- Pre-flight: `GET http://localhost:18434/v1/models` directly (read-only) to confirm the coder is up before touching anything.
- `go build -o mcp-gateway.bin ./cmd/mcp-gateway` in the `helix_llm` module — a real build of the real binary, not the package's own `_test.go` in-process server.
- Starts the built binary as a **real OS subprocess** listening on `:18446` (`HELIX_LLM_LOCAL_OPENAI_ENDPOINT=http://localhost:18434`, `HELIX_MCP_GATEWAY_TOKEN=<fresh random 32-hex test credential>`).
- Step 1: raw `POST` to the gateway with no `Authorization` header → asserts HTTP 401.
- Step 2: a real `mcp.NewClient` + `mcp.StreamableClientTransport` connects with the Bearer token → `session.ListTools()` → asserts both `helixllm_generate` and `helixllm_list_models` are present.
- Step 3: `session.CallTool("helixllm_generate", {"prompt": "Repeat the following token exactly ...: <fresh nonce>"})` → asserts the response is non-error, non-empty, and **contains the exact nonce generated for this run**.
- Step 4: `session.CallTool("helixllm_list_models", {})` → asserts non-error and captures the real model id.
- Post-flight: `GET http://localhost:18434/v1/models` again → asserts still HTTP 200 (coder untouched).
- Subprocess is killed at the end; the coder is never started/stopped/restarted at any point.
- Every evidence file is passed through a `redact()` step that strips the live bearer token before write (§11.4.10) — verified below with a grep sweep for any 32-hex-char substring.

Harness source: `harness/main.go`. Raw run transcript below (captured directly from the harness's own stdout during this session — not summarized/reconstructed).

## Results

### 1. No-Bearer request → 401

`harness/01_no_bearer_401_response.txt`:
```
HTTP status: 401
Body: no bearer token
```
**PASS** — request with no Bearer token rejected with 401.

### 2. `tools/list` — both tools present, sourced live from the server

`harness/02_tools_list_response.json` (excerpt, full file captured):
```json
[
  {
    "description": "Generate a real chat completion from the live HelixLLM coder (OpenAI-compatible /v1/chat/completions). Never a simulated response.",
    "name": "helixllm_generate",
    ...
  },
  {
    "description": "List the real models currently served by the live HelixLLM coder (OpenAI-compatible /v1/models). Never a hardcoded list (CONST-036).",
    "name": "helixllm_list_models",
    ...
  }
]
```
**PASS** — `tools/list` (with valid Bearer) returned exactly `[helixllm_generate helixllm_list_models]`.

### 3. `tools/call helixllm_generate` — fresh nonce echo (anti-bluff/anti-cache proof)

Nonce generated for this run: `R41F-NONCE-f23370ad49e54ea3`

`harness/03_generate_call_response.json`:
```json
{
  "content": [
    { "type": "text", "text": "R41F-NONCE-f23370ad49e54ea3" }
  ],
  "structuredContent": {
    "content": "R41F-NONCE-f23370ad49e54ea3",
    "model_id": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf"
  }
}
```
**PASS** — the live Qwen3-Coder-30B-A3B-Instruct model, reached through the real gateway proxy to `:18434`, echoed the exact nonce generated moments earlier by this harness run. This is not reproducible from a cache or a canned response — the nonce is fresh random bytes generated at harness-start.

### 4. `tools/call helixllm_list_models` — real model id from `:18434`

`harness/04_list_models_call_response.json`:
```json
{
  "content": [
    { "type": "text", "text": "[/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf]" }
  ],
  "structuredContent": {
    "models": ["/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf"]
  }
}
```
**PASS** — matches the coder's own `/v1/models` id (`/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`), confirming CONST-036 (no hardcoded model list — this is the live verifier/coder-sourced id).

### 5. Coder untouched (read-only throughout)

- Pre-flight `GET :18434/v1/models` → HTTP 200 (`harness/00_coder_preflight_models.json`).
- Post-flight `GET :18434/v1/models` → HTTP 200 (`harness/05_coder_postflight_models.json`).
- Independent out-of-band confirmation (run manually in this session, outside the harness, after gateway teardown):
  ```
  curl -X POST http://localhost:18434/v1/chat/completions -d '{"model":"...","messages":[{"role":"user","content":"Reply with exactly: coder-alive"}],"max_tokens":8}'
  → {"choices":[{"message":{"content":"coder-alive"}}], ...}
  ```
- The coder process was never started, stopped, or restarted by this harness or this session (§11.4.119 / §11.4.122).

### Token redaction check (§11.4.10)

The gateway's fresh 32-hex-char bearer token was generated in-memory per run and never logged by the gateway binary itself (its own `log.Printf` on startup prints only `listen`/`coder`/`okf_root`, never the token — see `harness/gateway_stdout_stderr.log` below). Every evidence file the harness writes is additionally passed through an explicit `redact()` step. A post-run grep sweep for any 32-hex-char substring across all evidence files found **no matches** (clean):

```
$ grep -rEn '\b[0-9a-f]{32}\b' 00_SUMMARY.md 01_*.txt 02_*.json 03_*.json 04_*.json gateway_stdout_stderr.log
NO 32-hex token substrings found (clean)
```

`harness/gateway_stdout_stderr.log` (full contents, token-free by construction):
```
2026/07/11 18:24:12 MCP gateway listening on :18446 (coder=http://localhost:18434 okf_root="")
```

## Full harness stdout (this run)

```
PRE-FLIGHT: coder at :18434 reachable (read-only query, untouched)
building gateway binary: go build -o .../mcp-gateway.bin ./cmd/mcp-gateway (in .../submodules/helix_llm)
PASS: go build ./cmd/mcp-gateway succeeded
starting gateway subprocess: .../mcp-gateway.bin (listen=:18446 coder=http://localhost:18434)
gateway is reachable at http://localhost:18446
PASS: request with no Bearer token rejected with 401
PASS: tools/list (with valid Bearer) contains helixllm_generate + helixllm_list_models: [helixllm_generate helixllm_list_models]
PASS: real tools/call helixllm_generate echoed the FRESH per-run nonce from the live coder (proves genuine live round-trip, not cached/canned)
  nonce: R41F-NONCE-f23370ad49e54ea3
  content: R41F-NONCE-f23370ad49e54ea3
  structured: {"content":"R41F-NONCE-f23370ad49e54ea3","model_id":"/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf"}
PASS: real tools/call helixllm_list_models returned live model list from coder: [/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf]
PASS: coder at :18434 still answers post-run (untouched, read-only throughout)
stopping gateway subprocess (coder at http://localhost:18434 left untouched)...
RESULT: PASS
```

## Verdict

**PASS** — all 4 acceptance criteria + coder-untouched invariant confirmed live, with a real go-sdk MCP client (not a curl-faked protocol interaction), fresh nonce (not a repeat of the earlier `"What is 2+2?"` proof, so no possibility of a stale/cached artifact being mistaken for a fresh one), and token redaction verified by grep sweep. This closes out the MCP-transport half of the API-surface V&V — combined with the earlier dual-wire (OpenAI-compatible HTTP facade) proof from today, both API surfaces of the `helix_llm` coder are now live-proven end-to-end.

## Notes

- `mcp-gateway.bin` (the built binary, ~12 MB) was deleted after the run — build artifacts are not versioned (CONST-053), matching the precedent set by the prior `docs/qa/phase1_mcp_gateway_20260708T074850Z/harness/` evidence directory (whose own `.bin` was likewise untracked).
- `build_output.txt` is empty — `go build` produced no warnings/output, a clean compile.
- Gateway listened on `:18446` in this run (distinct from the `:18444` used by the original 2026-07-08 phase-1 proof) to avoid any port collision with other concurrently-running work streams on this shared host (§11.4.174 process/port-ownership discipline).
