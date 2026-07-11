# Wave-2 Extension Docs — Gap Report

Author: doc-verification stream (read-only against `submodules/helix_llm`;
no coder/service access per task constraints). Date: 2026-07-11.
Repo: `/home/milos/Factory/projects/tools_and_research/helix_code`,
submodule `submodules/helix_llm` (branch checked out at task start).

## Scope

Verified + drafted setup/usage docs for the four new HelixLLM extension
capabilities named in the task: RAG-Qdrant + cross-encoder reranker,
vtracer vectorization service, the OpenAI+Anthropic dual-wire facade, and
Lane-B (second coder/agent instance). Cross-checked the extension's actual
Go implementation against latest upstream docs for Qdrant, HuggingFace TEI,
vtracer, OpenAI Chat Completions, and Anthropic Messages.

## Deliverables (this directory)

| File | Capability | Status |
|---|---|---|
| `rag_qdrant_reranker.md` | RAG-Qdrant + cross-encoder reranker (TEI) | Drafted, one production-wiring gap flagged |
| `vectorize_vtracer.md` | vtracer vectorization service (:18452) | Drafted, existing source README already strong; one missing-artifact gap flagged |
| `openai_anthropic_facade.md` | Dual-wire OpenAI + Anthropic facade | Drafted, one accuracy gap flagged (Anthropic tools/structured-content not forwarded) |
| `lane_b_agentgen.md` | Lane-B second coder instance (:18435) | Drafted from scratch — no prior user-facing doc existed |

## Upstream sources fetched + verified (2026-07-11)

- Qdrant collections REST API: https://qdrant.tech/documentation/concepts/collections/ — fetched successfully. Confirms `PUT /collections/{name}` with `{vectors:{size,distance}}`, distance metrics `Dot|Cosine|Euclid|Manhattan`, and `POST /collections/{name}/points/{upsert,search}` path convention. HelixLLM's `QdrantStore` (`internal/knowledge/qdrant.go`) matches this shape (via the vendored `digital.vasic.vectordb/pkg/qdrant` client) and always requests `Cosine` distance.
- HuggingFace Text Embeddings Inference (TEI) quick tour: https://huggingface.co/docs/text-embeddings-inference/en/quick_tour — fetched successfully. Confirms `/embed` and `/rerank` HTTP endpoint shapes (`{"query":..., "texts": [...], "raw_scores": bool}` → array of `{index, score}`), and the CPU Docker image convention (`ghcr.io/huggingface/text-embeddings-inference:cpu-1.9`, matching what the extension's proof harness uses in `compose.qdrant_rerank.yml`).
- Anthropic Messages API: attempted `https://docs.anthropic.com/en/api/messages` — received a 301 redirect to `https://platform.claude.com/docs/en/api/messages`, which was then fetched successfully. Confirms request shape (`model`, `max_tokens` required, `messages`, `system`, `tools`, `tool_choice`, `stream`), response shape (`id`, `type`, `role`, `content[]`, `stop_reason` enum, `usage.{input_tokens,output_tokens}`), the six-event SSE streaming sequence (`message_start` → `content_block_start` → `content_block_delta`* → `content_block_stop` → `message_delta` → `message_stop`), and required headers (`x-api-key`, `anthropic-version: 2023-06-01`).
- OpenAI Chat Completions API: attempted `https://platform.openai.com/docs/api-reference/chat/create` — **received HTTP 403 Forbidden** (bot-blocked). Per §11.4.99's fallback clause ("seek secondary authoritative sources when the official source is sparse/silent"), used the official machine-readable spec instead: `https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml` — fetched successfully. Confirms request fields (`model`, `messages`, `tools`, `tool_choice`, `stream`, `temperature`), response fields (`id`, `object`, `choices[].message.tool_calls`, `choices[].finish_reason` enum incl. `stop|tool_calls|length`, `usage.{prompt_tokens,completion_tokens,total_tokens}`), streaming `chat.completion.chunk` object type, and the literal `data: [DONE]` SSE terminator.
- vtracer CLI: https://github.com/visioncortex/vtracer — fetched successfully. Confirms `-i`/`-o` required flags and the `--preset bw|poster|photo` closed set that `services/vectorize/main.go: handleVectorize` validates against. Minor low-confidence note: this fetch's summary reported vtracer "0.6.4 (released April 20, 2024)" as the latest crates.io version, while the extension's own `services/vectorize/README.md` and `Containerfile` cite "vtracer 0.6.5" verified 2026-07-11 via `objdump -T`/direct `cargo install vtracer --locked`. This is NOT flagged as a hard contradiction — GitHub's README/release-tag metadata can lag crates.io's actual published versions, and the extension's own citation is a first-hand `cargo install --locked` result (stronger evidence than a summarized GitHub fetch). Recorded here for transparency per §11.4.6 (no guessing, state both data points).

## §11.4.99 drift/deprecation findings

None of the four capabilities show upstream API drift or deprecation against
the current wire formats they implement:
- Qdrant collection/upsert/search REST shape: matches current docs.
- TEI `/embed` and `/rerank` shapes: match current docs.
- OpenAI Chat Completions request/response/streaming shape: matches current OpenAPI spec.
- Anthropic Messages request/response/streaming shape + required headers: matches current docs.
- vtracer CLI presets: match current upstream CLI.

The one live risk item is **not** upstream drift but a **local schema-vs-behavior gap** (documented below) — the Anthropic wire's Go struct correctly mirrors the current upstream schema, but the extension's own request-conversion code does not forward two of those fields to the backend.

## Impl-vs-doc mismatches found (the load-bearing findings of this pass)

### 1. RAG-Qdrant: cross-encoder reranker is proven, not production-wired

`docs/qa/phase3_rag_qdrant_rerank_20260711T142237Z/harness/` (a standalone
Go binary with its own `go.mod`) proves a real TEI `/rerank` call
re-scoring Qdrant ANN results and correcting a deliberately-adversarial
ranking. However:
- `internal/knowledge/pipeline.go: Pipeline.Query` has exactly two ranking
  paths — raw store ANN order, or (if `HybridEnabled`) MMR diversity
  reordering (`internal/knowledge/mmr.go`).
- `internal/knowledge/reranker.go` defines a `Reranker` interface with
  `ScoreReranker` (passthrough) and `LLMReranker` (asks the *chat* LLM to
  score relevance, a different mechanism from a cross-encoder) — neither
  calls TEI's `/rerank`.
- `Pipeline` never holds a `Reranker` field.
- **Consequence:** an end user hitting `POST /internal/knowledge/query`
  today does not benefit from cross-encoder reranking, despite the commit
  message "feat(rag): Qdrant vector-DB + cross-encoder reranker fusion —
  live-proven" (commit `70ac952`) potentially reading as a production
  claim. The doc drafted in this pass (`rag_qdrant_reranker.md`) states
  this explicitly rather than implying reranking is live.

### 2. Anthropic wire: `tools`/`tool_choice`/structured `content` accepted but dropped

`pkg/api.MessageRequest` correctly declares `Tools []AnthropicTool`,
`ToolChoice interface{}`, and `Content interface{}` (string or content-block
array) matching the real upstream schema. But
`internal/gateway/anthropic.go: anthropicToInternal` — the function that
converts an incoming request into the internal representation sent to the
backend — copies neither `Tools` nor `ToolChoice`, and its `switch
v := m.Content.(type) { case string: ... }` silently produces an empty
string for any non-string (i.e. multi-block/image/tool_result) `content`
value. No error is returned to the caller in either case. The OpenAI wire,
by contrast, DOES implement tool calls end-to-end
(`internal/gateway/openai.go`, incl. a normalization bridge for models that
emit tool calls as text instead of native `tool_calls`). Documented as a
"Known limitation" section in `openai_anthropic_facade.md` with exact
`file:line` citations.

### 3. RAG: default embedding provider is not semantic

`HELIX_EMBEDDING_PROVIDER` defaults to `"local"`
(`internal/shared/config/config.go`), which resolves via
`NewEmbedder`'s switch (`internal/knowledge/embedding_providers.go`) to
`HashEmbedder` — a deterministic hash-based pseudo-embedder with no
semantic similarity properties. This is presumably intentional (a
zero-dependency default for wiring tests), but the existing
`docs/user-guide/rag-knowledge.md` describes the embedding step
("Each chunk is converted to a vector embedding... The default local
embedder produces 768-dimensional vectors") without flagging that this
default is not semantically meaningful for real retrieval. Flagged +
clarified in `rag_qdrant_reranker.md`.

### 4. RAG: Qdrant host/port are hardcoded, not env-configurable

`cmd/helixllm/main.go` calls `knowledge.NewVectorStore(cfg.Knowledge.VectorDB, "localhost", 6333)`
with literal `"localhost"`/`6333` — there is no `HELIX_QDRANT_HOST` /
`HELIX_QDRANT_PORT` config field in `internal/shared/config/config.go`.
Operators cannot point HelixLLM at a remote or differently-ported Qdrant
via configuration in this build. Also: connection failure at startup
silently falls back to the in-memory store with only a log line as
evidence — no API-visible indicator of which backend is actually serving
requests. Documented in `rag_qdrant_reranker.md`.

### 5. vtracer service: referenced proof-run directory does not exist in this checkout

`services/vectorize/README.md` cites
`docs/qa/vectorization_liveproof_<run-id>/` as the location of the
fidelity-proof harness and states specific measurements "at proof time
(2026-07-11)" (e.g. 12632 MiB free VRAM). This directory was **not found**
under `docs/qa/` in this checkout as of the time of this documentation
pass (searched, no `vectorization_liveproof_*` match; only
`phase4_imagegen_20260707`, `phase3_rag_qdrant_rerank_20260711T142237Z`,
etc. are present). Possible explanations (not verified — outside this
task's read-only scope): the proof run's artefacts are pending commit by
the coder-owning stream, or the directory has a different actual name than
what the README's `<run-id>` placeholder implies. Flagged, not
fabricated — `vectorize_vtracer.md` states this gap explicitly rather than
inventing a directory listing.

### 6. Lane B: no discovered integration point with HelixLLM's `Brain`/routing layer

`cmd/agentgen-boot` boots a second raw llama-server instance on `:18435`.
No reference to port `18435` or the string "agentgen" was found inside
`internal/brain/` in this pass — meaning Lane B, once booted, is reachable
directly via its own OpenAI-compatible HTTP endpoint but is not (yet, as
far as this pass found) discoverable/routable through HelixLLM's own
`Brain`/fallback-chain/gateway facade. This may be intentional (Lane B as
an independently-addressable benchmark-spike target per its own header
comment, "Serving-plan Task 2.1 benchmark spike") rather than a defect —
documented as an open question in `lane_b_agentgen.md` rather than
asserted as broken.

## Capabilities documented (per task deliverable list)

1. RAG-Qdrant (vector store — production-wired; reranker — proof-harness only, gap noted)
2. vtracer vectorization service (:18452 — fully documented, one referenced-but-missing artefact gap noted)
3. OpenAI+Anthropic dual-wire facade (both wires documented; one Anthropic-side forwarding gap noted)
4. Lane-B second coder instance (:18435 — documented from scratch, no prior doc existed; one integration-scope question noted)

## Doc gaps remaining (beyond what was drafted in this pass)

- No `vectorize-boot` Go harness exists under `cmd/` (unlike `agentgen-boot`,
  `visiongen-boot`, `imagegen-boot`, `videogen-boot`) — the vectorize
  service's own container-boot lifecycle is not yet wrapped in the same
  admit/boot/health-poll/down pattern the other capabilities use. Not
  fixed here (code-write, out of scope for this doc-only pass); noted as a
  documentation-and-implementation follow-up.
- `openai_anthropic_facade.md` intentionally did not transcribe every SSE
  chunk field for the OpenAI streaming path verbatim (to avoid summarizing
  from memory rather than the live source) — a follow-up pass should read
  `internal/gateway/openai.go: streamChatCompletions` directly and add a
  field-by-field table if a strict client-implementation reference is
  needed.
- No `docs/qa/<run-id>/` transcript was generated by this pass for the
  documentation deliverable itself (this is a documentation-verification
  task, not a feature-shipping commit — §11.4.83's "every feature that
  ships" scope does not apply to a docs-only research/gap-analysis
  artefact staged in scratchpad pending integration by the next wave).

## Honesty notes

- All four upstream fetches that succeeded are cited above with exact URLs
  and the 2026-07-11 access date. The one failed fetch
  (`platform.openai.com`, HTTP 403) is disclosed rather than silently
  substituted — the OpenAI OpenAPI spec on GitHub was used instead and is
  cited as such.
- No code was modified. No files under `submodules/helix_llm/` or the repo
  root were written to — all four drafts + this report live only under the
  scratchpad path specified in the task.
