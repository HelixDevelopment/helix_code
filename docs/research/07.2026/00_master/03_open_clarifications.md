# Open Clarifications — operator-gated ambiguities (§11.4.6 / §11.4.66 / §11.4.105)

These are names/targets the research could NOT resolve to a single unambiguous real API/spec.
Per §11.4.6 I will NOT invent an endpoint. Per §11.4.101 these are PARKED (they do not block the
running research); they will be asked as ONE batched question at the Phase-R→P synthesis boundary.
Confirmed items proceed regardless.

| # | Name (as given) | Research finding | **OPERATOR RESOLUTION (2026-07-06)** |
|---|-----------------|------------------|---------------------|
| C1 | **"GPT SOL"** | OpenAI GPT-5.6 "Sol" (rides existing OpenAI adapter) vs Upstage Solar (new adapter). | ✅ **OpenAI GPT-5.6 "Sol"** — NO new adapter; add model IDs to the existing OpenAI provider when it reaches GA (track as preview-pending). |
| C2 | **"Google OKF"** | Google Cloud Open Knowledge Format v0.1 — a knowledge-packaging spec, not a model/provider. | ✅ **RAG/Skills knowledge format via MCP resources** — implement OKF as the on-disk knowledge format served through an MCP `resources` server (context layer, not provider layer). |
| C3 | **"ACP"** | Zed Agent Client Protocol (editor↔agent) vs IBM/BeeAI ACP vs **Google A2A** (agent↔agent). | ✅ **Google A2A (Agent-to-Agent)** — implement A2A interop, NOT Zed ACP. (Note: A2A is agent↔agent, so "editor auto-recognition" is served separately via the `/v1/models` + MCP `server/discover` capability surface; A2A adds cross-agent interop.) |
| C4 | **Subquadratic (SubQ)** | Private beta only — no public base URL/auth. | ✅ **BLOCKED-until-GA** — tracked as a deferred item; no adapter work possible until a public API (or operator-supplied beta credentials) exists. |

## Resolved WITHOUT needing the operator (recorded for transparency)

- **Poe** → `https://api.poe.com/v1` (OpenAI-compat aggregator) — adapter-ready. ✅
- **Perplexity** → `https://api.perplexity.ai` Sonar models — adapter-ready. ✅
- **Sakana Fugu** → `https://api.sakana.ai/v1` (`fugu`, `fugu-ultra`) — adapter-ready. ✅
- **Xiaomi MiMo** → `https://api.xiaomimimo.com/v1` (+ Anthropic surface) `mimo-v2.5-pro` — adapter-ready. ✅
- **Tencent / Yuanbao** → **Yuanbao is the consumer app; the API target is Hunyuan** `https://api.hunyuan.cloud.tencent.com/v1` (`hunyuan-t1`, `hunyuan-t1-vision`) — adapter-ready. ✅
- **Qwythos 9B** → `empero-ai/Qwythos-9B-…` (Qwen3.5-9B, 1M ctx, Apache-2.0) — **self-host** via the existing local/Ollama/HF path, no new hosted adapter. ✅
- **GOT family** → **GOT-OCR2.0**, an OCR model (not a chat LLM) — belongs to the OCR capability (stream 07), self-host. ✅
- **High-value additions to also implement** (all confirmed public APIs): xAI Grok, Moonshot Kimi, Zhipu GLM, Fireworks, DeepInfra, Novita, AI21 Jamba, Reka (+ verify Hyperbolic/Baseten base URLs at build).

## Protocol decisions recorded (stream 05, ready)

- **MCP** — official Go SDK now exists (`github.com/modelcontextprotocol/go-sdk`, v1.6.1, maintained with Google) → closes the old "no Go SDK" gap. Design the gateway **stateless-first** (2026-07-28 RC drops the session handshake). Gateway = MCP server (expose Helix tools) + MCP host (consume CodeGraph/OpenDesign).
- **Capability advertisement (CONST-040)** — flags sourced from LLMsVerifier, surfaced on 3 channels: `/v1/models[].capabilities` (HelixCode), MCP `server/discover` (Claude Code), ACP `initialize` (editors). Tool-calling normalized to one canonical JSON-Schema + grammar-constrained decoding per local-model parser (anti-bluff).
- **OKF** — used as the on-disk RAG/Skills knowledge format served via an MCP `resources` server (pending C2 confirmation).
