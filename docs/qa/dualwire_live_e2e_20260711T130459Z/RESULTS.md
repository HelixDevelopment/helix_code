# Dual-Wire Facade — Live End-to-End HTTP Proof

**Revision:** 1
**Created:** 2026-07-11
**Last modified:** 2026-07-11
**Status:** active
**Track:** T1/feature/helixllm-full-extension

## Table of contents

- [1. Scope](#1-scope)
- [2. Topology under test](#2-topology-under-test)
- [3. Test command](#3-test-command)
- [4. Result summary](#4-result-summary)
- [5. Evidence — 401 (no auth)](#5-evidence--401-no-auth)
- [6. Evidence — OpenAI `/v1/chat/completions` 200 + nonce echo](#6-evidence--openai-v1chatcompletions-200--nonce-echo)
- [7. Evidence — Anthropic `/v1/messages` 200 + nonce echo](#7-evidence--anthropic-v1messages-200--nonce-echo)
- [8. Evidence — tool-call wire-shape divergence (live)](#8-evidence--tool-call-wire-shape-divergence-live)
- [9. Coder-untouched confirmation](#9-coder-untouched-confirmation)
- [10. Key-leak-free confirmation](#10-key-leak-free-confirmation)

## 1. Scope

Prove the full live HTTP path for HelixCode's dual-wire facade (`helix_code/internal/server/wire_facade.go`):

```
client --curl--> gin router --> wireFacadeAuthMiddleware --> chatCompletions/anthropicMessages
                              --> resolveLLMProvider("", model) --("helixllm"/"local" branch)-->
                              resolveHelixLLMLocalProvider --> llm.OpenAICompatibleProvider
                              --HTTP--> http://localhost:18434 (live Qwen3-Coder-30B-A3B-Instruct,
                              llama.cpp OpenAI-compatible sidecar) --> real answer --> back through
                              the SAME translation layer to the caller's wire shape.
```

Both routes are registered auth-gated in `server.go`:

```go
s.router.POST("/v1/chat/completions", s.wireFacadeAuthMiddleware(), s.chatCompletions)
s.router.POST("/v1/messages", s.wireFacadeAuthMiddleware(), s.anthropicMessages)
```

`wireFacadeAuthMiddleware` (server.go:594) is fail-closed: it accepts either
`Authorization: Bearer <key>` (OpenAI convention) or `x-api-key: <key>`
(Anthropic convention) checked against `cfg.Auth.WireFacadeAPIKeys`
(`HELIX_WIRE_FACADE_API_KEYS`); an empty configured-keys list (the shipped
default) rejects **every** request, even one carrying a token.

`resolveLLMProvider` (llm_generate.go:110) special-cases `"helixllm"`/`"local"`
**before** the F12 cloud-provider selector, routing to
`resolveHelixLLMLocalProvider` (llm_generate.go:454), which builds a real
`llm.OpenAICompatibleProvider` pointed at
`HELIX_LLM_LOCAL_OPENAI_ENDPOINT` (default `http://localhost:18434` — **base
URL, no `/v1` suffix**, since the OpenAI-compatible provider's endpoint
constants already carry that prefix — the documented endpoint gotcha).

## 2. Topology under test

- **Coder**: Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf, llama.cpp
  OpenAI-compatible server, **live** at `http://localhost:18434`, queried
  **read-only** (`GET /v1/models` + `POST /v1/chat/completions` completions —
  no config writes, no restart, per §11.4.122). No GPU used or required.
- **HelixCode server**: NOT the full daemon binary (no live PostgreSQL/Redis
  in this environment). Stood up via `httptest.NewServer` wrapping a real
  `*Server{config, router: gin.New()}` + real `srv.setupRoutes()` — the
  identical construction `wire_facade_auth_test.go`'s fixture already uses.
  The router, `wireFacadeAuthMiddleware`, `chatCompletions`,
  `anthropicMessages`, and `resolveLLMProvider` are the real, unmodified
  production code; only db/redis-dependent route groups (irrelevant to the
  wire-facade routes) are absent.
- **Client**: the real `curl` binary invoked via `os/exec` for every request
  (`curlCapture` helper) — a genuine external HTTP client process, not an
  in-process handler call.

## 3. Test command

```
cd helix_code && go test -v -count=1 -run TestWireFacade_FullHTTP_E2E_LiveRoundTrip ./internal/server/...
```

File: `helix_code/internal/server/wire_facade_live_e2e_test.go` (pre-existing
live-e2e test, extended in this session with a live tool-call
shape-divergence subtest — `tool_calls_shape_divergence_live`).

## 4. Result summary

```
=== RUN   TestWireFacade_FullHTTP_E2E_LiveRoundTrip
=== RUN   TestWireFacade_FullHTTP_E2E_LiveRoundTrip/no_auth_chat_completions_401
    PASS: no-auth request to /v1/chat/completions correctly rejected: status=401
=== RUN   TestWireFacade_FullHTTP_E2E_LiveRoundTrip/openai_chat_completions_authenticated_200
    PASS: OpenAI-shaped 200 via real HTTP+curl+router+middleware+coder:
    model="/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf" finish_reason="stop"
    usage={PromptTokens:55 CompletionTokens:23 TotalTokens:78}
    content="HELIXCODE-FULLHTTP-E2E-203036440e6c"
=== RUN   TestWireFacade_FullHTTP_E2E_LiveRoundTrip/anthropic_messages_authenticated_200
    PASS: Anthropic-shaped 200 via real HTTP+curl+router+middleware+coder:
    model="/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf" stop_reason="end_turn"
    usage={InputTokens:54 OutputTokens:22}
    content="HELIXCODE-FULLHTTP-E2E-f6a30d57b156"
=== RUN   TestWireFacade_FullHTTP_E2E_LiveRoundTrip/tool_calls_shape_divergence_live
=== RUN   .../tool_calls_shape_divergence_live/openai_tool_calls_arguments_is_json_string
    PASS (OpenAI wire): tool_calls[0].function.arguments is a JSON-ENCODED STRING
    = "{\"city\":\"HELIXCODE-FULLHTTP-E2E-292679c493a1\"}"
=== RUN   .../tool_calls_shape_divergence_live/anthropic_tool_use_input_is_json_object
    PASS (Anthropic wire): content[].input is a JSON OBJECT
    = map[city:HELIXCODE-FULLHTTP-E2E-292679c493a1]
--- PASS: TestWireFacade_FullHTTP_E2E_LiveRoundTrip (0.61s)
    --- PASS: .../no_auth_chat_completions_401 (0.01s)
    --- PASS: .../openai_chat_completions_authenticated_200 (0.11s)
    --- PASS: .../anthropic_messages_authenticated_200 (0.10s)
    --- PASS: .../tool_calls_shape_divergence_live (0.39s)
        --- PASS: .../openai_tool_calls_arguments_is_json_string (0.22s)
        --- PASS: .../anthropic_tool_use_input_is_json_object (0.16s)
PASS
ok  	dev.helix.code/internal/server	0.624s
```

Raw transcripts (full request+response bytes, key redacted below) captured
under `helix_code/../docs/qa/phase1_fullhttp_e2e_20260711T130444Z/` by the
test itself (§11.4.83 in-repo evidence trail):

- `01_no_auth_401.txt`
- `02_openai_chat_completions_200.txt`
- `03_anthropic_messages_200.txt`
- `04_openai_tool_calls_200.txt`
- `05_anthropic_tool_use_200.txt`

## 5. Evidence — 401 (no auth)

```
POST /v1/chat/completions   (no Authorization / x-api-key header)

=== RESPONSE ===
HTTP 401
{"error":{"message":"missing or invalid API key: provide 'Authorization: Bearer <key>' or 'x-api-key: <key>'","type":"authentication_error"}}
```

Fail-closed confirmed: `wireFacadeAuthMiddleware` rejects the request before
it ever reaches `chatCompletions`/provider resolution.

## 6. Evidence — OpenAI `/v1/chat/completions` 200 + nonce echo

```
POST /v1/chat/completions
Authorization: Bearer <REDACTED-TEST-KEY>

{"messages":[{"role":"user","content":"...Reply with EXACTLY this token and nothing else: HELIXCODE-FULLHTTP-E2E-203036440e6c"}],"max_tokens":32,"temperature":0}

=== RESPONSE ===
HTTP 200
{"id":"chatcmpl-333ef032-ebef-49a4-b9bc-c5c21c134fd1","object":"chat.completion","created":1783775084,
 "model":"/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
 "choices":[{"index":0,"message":{"role":"assistant","content":"HELIXCODE-FULLHTTP-E2E-203036440e6c"},"finish_reason":"stop"}],
 "usage":{"prompt_tokens":55,"completion_tokens":23,"total_tokens":78}}
```

Fresh nonce (`HELIXCODE-FULLHTTP-E2E-203036440e6c`, generated per-run via
`crypto/rand`) is echoed **verbatim** by the real coder — proves a live,
non-cached, real-model answer, not a fixture.

## 7. Evidence — Anthropic `/v1/messages` 200 + nonce echo

```
POST /v1/messages
x-api-key: <REDACTED-TEST-KEY>

{"messages":[{"role":"user","content":"...Reply with EXACTLY this token and nothing else: HELIXCODE-FULLHTTP-E2E-f6a30d57b156"}],"max_tokens":32,"temperature":0}

=== RESPONSE ===
HTTP 200
{"id":"msg_aac44a27-320b-4dca-b2e9-65a76710c21c","type":"message","role":"assistant",
 "model":"/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
 "content":[{"type":"text","text":"HELIXCODE-FULLHTTP-E2E-f6a30d57b156"}],
 "stop_reason":"end_turn","stop_sequence":null,
 "usage":{"input_tokens":54,"output_tokens":22}}
```

Different fresh nonce (`HELIXCODE-FULLHTTP-E2E-f6a30d57b156`) echoed
verbatim through the Anthropic-shaped route — proves the SAME coder is
reachable and answers genuinely through both facades.

## 8. Evidence — tool-call wire-shape divergence (live)

Both requests below drove the SAME live coder with the SAME `get_weather`
tool definition and a prompt naming a fresh nonce as the "city" — the coder
genuinely invoked the tool (`finish_reason`/`stop_reason` == tool-call, not a
plain text answer) in both cases.

**OpenAI wire** (`/v1/chat/completions`) — `function.arguments` is a
**JSON-ENCODED STRING**:

```
"tool_calls":[{"id":"E9nNYIeMZViyQ4pfihWt4B5RfzVyxfZB","type":"function",
  "function":{"name":"get_weather","arguments":"{\"city\":\"HELIXCODE-FULLHTTP-E2E-292679c493a1\"}"}}]
```

**Anthropic wire** (`/v1/messages`) — `content[].input` is a **JSON OBJECT**:

```
"content":[{"type":"tool_use","id":"FSvOs0UOIudNA4RMoL82RvYIdzFBiwHG","name":"get_weather",
  "input":{"city":"HELIXCODE-FULLHTTP-E2E-292679c493a1"}}]
```

Both carry the same live nonce (`HELIXCODE-FULLHTTP-E2E-292679c493a1`),
confirming the divergence is a genuine wire-shape translation performed by
`llmResponseToOpenAI` / `llmResponseToAnthropic` on the SAME underlying
`llm.ToolCall` (`Function.Arguments map[string]interface{}`) — not two
independently-fabricated responses. This matches the documented behaviour in
`wire_facade.go`'s `openAIWireToolCallIn`/`llmResponseToOpenAI`
(JSON-marshal to string) and `anthropicContentBlockOut`/`llmResponseToAnthropic`
(pass-through as object) comments.

## 9. Coder-untouched confirmation

Every call against `localhost:18434` in this session was a `GET /v1/models`
inventory read or a `POST /v1/chat/completions` completion request (the
coder's normal, intended usage — no config endpoints touched, no restart, no
process management of any kind — §11.4.122). The coder process was not
started, stopped, or reconfigured by this work.

## 10. Key-leak-free confirmation

The only credential used was the hardcoded test-only, non-real facade key
constant `wireFacadeE2ETestAPIKey` already defined in
`wire_facade_live_e2e_test.go` (self-documented as
`"...-not-a-real-secret"`; it authenticates nothing but this ephemeral
`httptest.Server` instance and is never sent to any real upstream). It is
REDACTED in this report (§5/§6/§7 show `<REDACTED-TEST-KEY>` in place of the
literal). `grep -rn "sk-\|ANTHROPIC_API_KEY\|OPENAI_API_KEY"` over this
report and the raw evidence dir returns no hits; no real provider credential
of any kind was read, logged, or transmitted during this task.
