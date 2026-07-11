# D3 — HelixCode Feature-Wiring / Runtime-Signature Audit

Scope: read-only audit of `$INNER` (`/home/milos/Factory/projects/tools_and_research/helix_code/helix_code`)
against §11.4.108 (SOURCE present ≠ WIRED) and CONST-036..040. All claims below cite
`file:line`. Anything not directly confirmed from source is marked `UNCONFIRMED:`.

---

## 1. Server HTTP routes (`internal/server/server.go`, `internal/server/llm_generate.go`)

Route registration lives in `internal/server/server.go:219-458` (`SetupRoutes`-equivalent
block). Full enumerated table:

| Route | Method | Handler | Real impl? (file:line) |
|---|---|---|---|
| `/health` | GET | `s.healthCheck` | server.go:219 |
| `/api/v1/health` | GET | `s.healthCheck` | server.go:228 |
| `/api/v1/auth/register,login,logout,refresh` | POST | `s.register` etc | server.go:233-236 |
| `/api/v1/users/me` | GET/PUT/DELETE | `s.getCurrentUser` etc (auth-gated) | server.go:243-245 |
| `/api/v1/workers*` | GET/POST/PUT/DELETE | worker CRUD (auth-gated) | server.go:252-258 |
| `/api/v1/tasks*` | GET/POST/PUT/DELETE | task CRUD + lifecycle (auth-gated) | server.go:265-276 |
| `/api/v1/projects*` (+ workflow triggers) | GET/POST/PUT/DELETE | project CRUD (auth-gated, see hardening note below) | server.go:293-302 |
| `/api/v1/sessions*` | GET/POST/PUT/DELETE | session CRUD (auth-gated) | server.go:309-313 |
| `/api/v1/system/stats,status` | GET | (auth-gated) | server.go:320-321 |
| `/api/v1/server/info`, `/api/v1/metrics` | GET | public | server.go:325-326 |
| `/api/v1/llm/providers`, `/providers/:id`, `/models` | GET | `s.listLLMProviders`/`s.getLLMProvider`/`s.listLLMModels` (public, metadata-only) | server.go:343-345 |
| `/api/v1/llm/generate` | POST | **`s.generateLLM`** (auth-gated) | server.go:350 → `internal/server/llm_generate.go:234` |
| `/api/v1/llm/stream` | POST | **`s.streamLLM`** (auth-gated) | server.go:351 → `internal/server/llm_generate.go:295` |
| `/specify` | POST | `s.specifyHandler` (auth-gated, real speckit engine per comment) | server.go:358-360 |
| `/api/v1/memory/systems,stats` | GET | public | server.go:365-366 |
| `/api/v1/qa/*` | POST/GET/DELETE | QA session lifecycle (auth-gated) | server.go:373-378 |
| `/api/v1/screenshot/*` | GET | (auth-gated) | server.go:385-386 |
| `/v1/chat/completions` | POST | `s.chatCompletions` (wire-facade API-key auth) | server.go:432 |
| `/v1/messages` | POST | `s.anthropicMessages` (wire-facade API-key auth) | server.go:433 |
| `/ws` | GET | `s.handleWebSocket` (ws-auth-gated) | server.go:453 |
| `/debug/pprof/*` | opt-in via env/log-level | `s.mountPprof()` | server.go:401-403 |

**Generate/Stream — real impl confirmed (not stub):**
- `generateLLM` (`llm_generate.go:234-289`) binds JSON → builds `llm.LLMRequest` →
  `provider, err := llmProviderResolver(req.Provider, req.Model)` (line 250) →
  `resp, genErr := provider.Generate(ctx, llmReq)` (line 265) → returns
  `resp.Content`/`resp.Usage`/`resp.FinishReason` as JSON. Real provider errors are
  surfaced as HTTP 502 with the raw error text (line 266-274), never masked as success.
- `streamLLM` (`llm_generate.go:295-360`) same resolution path, then
  `errCh <- provider.GenerateStream(ctx, llmReq, chunkChan)` (line 342) piped to SSE via
  `streamProviderToSSE` (line 360).
- `resolveLLMProvider` (`llm_generate.go:110-187`) is the construction path: resolves a
  named provider via `llm.Select` + `llm.NewCloudProvider(ptype, entry)` (line 149), a
  local HelixLLM/llama.cpp sidecar via `resolveHelixLLMLocalProvider` (line 137-138), or
  defaults to a real `llm.NewOllamaProvider` (line 178-182). No branch returns a
  synthetic/simulated response.
- Both `/api/v1/llm/generate|stream` and the OpenAI/Anthropic wire-facade routes
  (`/v1/chat/completions`, `/v1/messages`) were previously unauthenticated per the
  in-source security-fix comments (server.go:334-341, :417-431) — now gated by
  `s.authMiddleware()` / `s.wireFacadeAuthMiddleware()` respectively. This is a resolved
  historical bluff (cost-abuse hole), not a live one.

No stub/placeholder handler bodies were found in this file for the LLM-facing routes.

---

## 2. CLI anti-bluff regression check (`cmd/cli/main.go`) — BLUFF-001/002/003

| Bluff | Function | Evidence | Verdict |
|---|---|---|---|
| BLUFF-001 (simulated generation) | `handleGenerate` | `cmd/cli/main.go:1714-1857`: guards `c.llmProvider == nil` (1716); real streaming path calls `provider.GenerateStream(ctx, req, chunkChan)` in a goroutine (line 1823); real non-stream path calls `provider.Generate(ctx, req)` (line 1856, confirmed via follow-up read at offset 1854-1857). | **RESOLVED, no regression** |
| BLUFF-002 (hardcoded model list) | `handleListModels` | `cmd/cli/main.go:1418-1483`: Priority 1 — `c.verifierAdapter.GetWorkingModels(ctx, present)` (line 1437, CONST-036 verifier as source of truth); Priority 2 — `c.llmProvider.GetModels()` real provider query (line 1453); Priority 3 — an explicitly-labeled `verifier.FallbackModels` list rendered only with a `"⚠ unverified fallback"` tag (line 1483, 1495-1502) so it is never presented as verified/working. | **RESOLVED, no regression** (documented 3-tier fallback, not a silent hardcode) |
| BLUFF-003 (simulated exec) | `handleCommand` | `cmd/cli/main.go:2252-2281`: `cmd := exec.CommandContext(ctx, "sh", "-c", command)` (line 2256); real `cmd.Run()`, exit code extracted via `exitErr.ProcessState.ExitCode()` (line 2267) and surfaced through `exitCodeError`. | **RESOLVED, no regression** |

Anti-bluff grep (case-insensitive `simulated|for now|TODO implement|in production this would`)
against `cmd/cli/main.go` excluding `_test.go`: **zero hits** (command run, exit 1/no match).

---

## 3. LLM providers (CONST-039) — `internal/llm/*_provider.go`

All ten CONST-039-mandated providers are present as dedicated files, and each makes real
outbound HTTP calls (confirmed via `http.NewRequestWithContext` / `http.Client{}` grep,
not a mocked transport):

| Provider | File | Real HTTP evidence |
|---|---|---|
| OpenAI | `internal/llm/openai_provider.go` (639 lines) | `http.NewRequestWithContext(ctx,"GET",.../models)` :186; `.../chat/completions` POST :385, :418 |
| Anthropic | `internal/llm/anthropic_provider.go` (1011 lines) | `http.Client{...}` :186; POST to `ap.endpoint` :774, :824 |
| Gemini | `internal/llm/gemini_provider.go` (753 lines) | POST `url` :528, :581 |
| DeepSeek | `internal/llm/deepseek_provider.go` (514 lines) | GET `/models` :162; POST `/chat/completions` :336, :367 |
| Groq | `internal/llm/groq_provider.go` (818 lines) | `http.Client{}` :151; POST `url` :444, :503 |
| Mistral | `internal/llm/mistral_provider.go` (489 lines) | `http.Client{}` :55; GET `/models` :148; POST `/chat/completions` :341, :372 |
| xAI | `internal/llm/xai_provider.go` (424 lines) | `http.Client{}` :46; GET `/models` :137; POST `/chat/completions` :306, :339 |
| OpenRouter | `internal/llm/openrouter_provider.go` (606 lines) | `http.Client{}` :46; GET `/models` :138,:236; POST `/chat/completions` :446,:481 |
| Ollama | `internal/llm/ollama_provider.go` (600 lines) | `apiClient: &http.Client{}` :161; POST :496, :537 |
| Llama.cpp | `internal/llm/llamacpp_provider.go` (471 lines) | POST `apiURL` :178; `client := &http.Client{Timeout: p.config.ServerTimeout}` :191; streaming POST `.../completion` :345 (`Timeout: 0` for streaming, :349) |

**CONST-039 verdict: all 10 mandated providers present with real-impl evidence — no
missing provider, no stub found.** Additional (beyond-mandate) providers also present with
their own files: Bedrock, Azure, VertexAI, Copilot, Qwen, Xiaomi, KoboldAI, local_provider,
plus an OpenAI-compatible generic adapter (`openai_compatible_provider.go`) and a LiteLLM
unified adapter package (`internal/llm/litellm/`).

---

## 4. CONST-040 capability flags (MCP, LSP, ACP, Embedding, RAG, Skills, Plugins)

**Verifier type definitions** — `internal/verifier/types.go`:
- `VerifiedModel` (lines 24-61) has `SupportsStreaming, SupportsTools, SupportsFunctions,
  SupportsCode, SupportsVision, SupportsAudio, SupportsVideo, SupportsReasoning,
  SupportsEmbeddings, SupportsJSONMode` (lines 35-44) plus a generic `Capabilities []string`
  (line 59).
- `VerificationResult` (lines 81-108) has `SupportsToolUse, SupportsCodeGeneration,
  SupportsEmbeddings, SupportsStreaming, SupportsJSONMode, SupportsReasoning` plus several
  code-quality booleans (`CodeDebugging`, `TestGeneration`, etc, lines 93-105).

**Finding: only `Embedding` of the seven CONST-040-named capabilities has a concrete,
named, verifier-sourced boolean field (`SupportsEmbeddings` — `types.go:43,95`).**
`MCP`, `LSP`, `ACP`, `RAG`, `Skills`, `Plugins` have **no corresponding named field**
anywhere in `VerifiedModel` or `VerificationResult`. Confirmed by exhaustive grep across
`internal/` and `cmd/` for `SupportsMCP|SupportsLSP|SupportsACP|SupportsSkills|
SupportsPlugins|SupportsRAG` — **zero matches**.

**Doc-comment claims CONST-040 compliance without field-level backing:**
- `internal/verifier/doc.go:33`: `// - CONST-040: MCP/LSP/ACP/Embedding/RAG/Skills/Plugins`
  — listed as satisfied, but the struct it documents (same file's package) does not carry
  those flags.
- `internal/llm/ensemble_resolver.go:9-21` cites CONST-040 and enumerates what
  `VerifiedModel` "carries": `Provider, Verified, Deprecated, SupportsEmbeddings,
  Capabilities, and OverallScore` — MCP/LSP/ACP/RAG/Skills/Plugins are conspicuously
  absent from that list too, i.e. the implementers were aware only Embeddings shipped.

**Subsystem-level reality check (are the *features* real, even if not verifier-gated?):**
- MCP: real subsystem at `internal/mcp/` (client.go-equivalent, `server.go`, `registry.go`,
  `lifecycle.go`, `oauth.go`, chaos/stress test files) — genuinely implemented, but its
  availability/use is **not gated by any verifier `VerificationResult` flag**.
- LSP: real CLI subsystem, `cmd/cli/lsp_cmd.go` (`newLSPCmd`, `newLSPStatusCmd`,
  `newLSPListServersCmd`, `newLSPRestartCmd`, `newLSPStopCmd` — lines 43-108) — likewise
  not verifier-flag-gated.
- Skills: real CLI subsystem, `cmd/cli/skills_cmd.go` (`newSkillsCmd`, `newSkillsListCmd`,
  `newSkillsShowCmd`, `newSkillsInvokeCmd`, `newSkillsReloadCmd` — lines 22-97) — not
  verifier-flag-gated.
- Plugins: real subsystem, `internal/plugins/` (`loader.go`, `manifest.go`, `exec.go`,
  `registry.go`, `activation.go`, `base_plugin.go`) — not verifier-flag-gated.
- RAG: **no subsystem found at all.** No `internal/rag` package, no
  `RetrievalAugmented*` symbol anywhere in `internal/`.
- ACP: **no subsystem found at all.** No `"acp"` string literal, no
  `AgentCommunicationProtocol` symbol anywhere in `internal/` or `cmd/`.

So the picture is nuanced: MCP/LSP/Skills/Plugins are NOT hardcoded-fake capability flags —
they're real, working subsystems that simply predate/bypass the CONST-040 "must be
verifier-sourced" requirement (a governance gap, not a BLUFF-001-style simulation). RAG and
ACP are missing entirely (neither implemented nor verifier-flagged) — CONST-039/040's
implicit promise that these are part of the platform is unmet for those two.

---

## Capability matrix

| Capability | Wired (real impl evidence) | Declared-only / doc-claims-compliance | Missing |
|---|---|---|---|
| `/api/v1/llm/generate` | YES — `llm_generate.go:234-289` → `provider.Generate` | | |
| `/api/v1/llm/stream` | YES — `llm_generate.go:295-360` → `provider.GenerateStream` | | |
| CLI `handleGenerate` | YES — `main.go:1714-1857` real provider calls | | |
| CLI `handleListModels` | YES — verifier→provider→labeled-fallback 3-tier | | |
| CLI `handleCommand` | YES — `exec.CommandContext` | | |
| OpenAI provider | YES — `openai_provider.go` | | |
| Anthropic provider | YES — `anthropic_provider.go` | | |
| Gemini provider | YES — `gemini_provider.go` | | |
| DeepSeek provider | YES — `deepseek_provider.go` | | |
| Groq provider | YES — `groq_provider.go` | | |
| Mistral provider | YES — `mistral_provider.go` | | |
| xAI provider | YES — `xai_provider.go` | | |
| OpenRouter provider | YES — `openrouter_provider.go` | | |
| Ollama provider | YES — `ollama_provider.go` | | |
| Llama.cpp provider | YES — `llamacpp_provider.go` | | |
| CONST-040 Embedding flag | YES — `verifier/types.go:43,95 SupportsEmbeddings` | | |
| CONST-040 MCP flag | subsystem real (`internal/mcp/`) | doc.go:33 claims CONST-040 done | **no verifier-sourced flag** |
| CONST-040 LSP flag | subsystem real (`cmd/cli/lsp_cmd.go`) | doc.go:33 claims CONST-040 done | **no verifier-sourced flag** |
| CONST-040 Skills flag | subsystem real (`cmd/cli/skills_cmd.go`) | doc.go:33 claims CONST-040 done | **no verifier-sourced flag** |
| CONST-040 Plugins flag | subsystem real (`internal/plugins/`) | doc.go:33 claims CONST-040 done | **no verifier-sourced flag** |
| CONST-040 RAG flag | | doc.go:33 claims CONST-040 done | **no subsystem, no flag — fully missing** |
| CONST-040 ACP flag | | doc.go:33 claims CONST-040 done | **no subsystem, no flag — fully missing** |

---

## TOP FINDINGS

- F-D3-01 | HIGH | CONST-040 doc claims (`internal/verifier/doc.go:33`, `internal/llm/ensemble_resolver.go:9-21`) list MCP/LSP/ACP/Embedding/RAG/Skills/Plugins as verifier-sourced, but `VerificationResult`/`VerifiedModel` (`internal/verifier/types.go`) only implement `SupportsEmbeddings` — the other six are declared-compliant-but-unimplemented as capability flags.
- F-D3-02 | HIGH | RAG has zero implementation anywhere in `internal/` — no package, no symbol, no verifier flag — despite CONST-040 listing it as a mandated capability.
- F-D3-03 | HIGH | ACP (Agent Communication Protocol) has zero implementation anywhere in `internal/` or `cmd/` — no package, no `"acp"` literal, no verifier flag — despite CONST-040 listing it as mandated.
- F-D3-04 | MEDIUM | MCP/LSP/Skills/Plugins are real, working subsystems (`internal/mcp/`, `cmd/cli/lsp_cmd.go`, `cmd/cli/skills_cmd.go`, `internal/plugins/`) but their capability/availability is never gated by a verifier `VerificationResult` flag — CONST-040's "sourced from verifier" clause is unmet even though the features themselves are genuine (not a BLUFF-style fake, but a governance gap).
- F-D3-05 | LOW | No regressions found: BLUFF-001/002/003 in `cmd/cli/main.go` remain resolved (handleGenerate → real `provider.Generate`/`GenerateStream` at main.go:1823,1856; handleListModels → verifier→provider→labeled-fallback at main.go:1437,1453,1483; handleCommand → real `exec.CommandContext` at main.go:2256). Anti-bluff grep clean.
- F-D3-06 | LOW (positive) | All 10 CONST-039-mandated LLM providers confirmed present with real outbound HTTP calls (file:line evidence in §3 table above) — no missing provider, no stub.
- F-D3-07 | LOW (positive) | `/api/v1/llm/generate` and `/api/v1/llm/stream` routes confirmed wired end-to-end to real `llm.Provider.Generate`/`GenerateStream` via `resolveLLMProvider` (`llm_generate.go:110-187`), with documented historical auth-hardening fixes (previously unauthenticated cost-abuse hole, now `authMiddleware`-gated).
- F-D3-08 | INFO | UNCONFIRMED: whether `internal/mcp/`, `cmd/cli/lsp_cmd.go`, `cmd/cli/skills_cmd.go`, `internal/plugins/` subsystems are fully end-to-end reachable/wired from a running server (this audit read only route registration + provider/CLI layers per the assigned scope; did not trace MCP/LSP/Skills/Plugins subsystem internals in depth).
