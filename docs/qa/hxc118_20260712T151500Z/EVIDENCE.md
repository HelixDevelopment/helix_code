# HXC-118 — QA evidence (§11.4.83)

**Item:** HXC-118 (Feature/High) — RAG module exists but is not wired into the product's server generate path
**Module:** `helix_code/` inner Go app (`dev.helix.code`)
**Fix commit:** helix_code `225cdf77` (2 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T15:15:00Z
**Closure vocab:** Implemented (§11.4.33, Feature)
**Discipline:** §11.4.102, §11.4.115 RED-first (RED_MODE polarity), §11.4.142 independent review, §11.4.145 blast-radius, §11.4.6 honest-boundary.

## Root cause (FACT)

The RAG module (`internal/rag/`: adapter + OllamaEmbedder + InMemoryVectorStore retriever) was fully
implemented and wired into the CLI generate path (`cmd/cli/main.go handleGenerate`, behind
`HELIXCODE_RAG_ENABLED`) by prior work (54a76c3c/edbd5a49), but the HTTP SERVER generate endpoints had
ZERO RAG integration (`grep -rl rag internal/server/` = 0, confirmed on parent 530dc108). So a user
calling the native `POST /api/v1/llm/generate` (or stream) never received retrieval-augmentation even
with RAG enabled.

## Fix

`internal/server/llm_generate.go`: added `ragAdapterResolver` var (production default
`rag.NewFromEnv(os.Getenv)`; test-injection seam) + `applyRAGContext(ctx, adapter, llmReq)` helper,
called in BOTH `generateLLM` and `streamLLM` after ctx-with-timeout, before the provider call. Disabled
(default) → true no-op, `llmReq` byte-identical (Enabled() short-circuit + rag.Retrieve's own internal
gate = defense-in-depth). Enabled → retrieves against the last message content, replaces with
`rag.PrependContext(...)`. Retrieval error → logged, original prompt used (graceful degrade, never fails
the request, §11.4.6). Empty-messages guarded.

## Guard tests (`internal/server/llm_rag_test.go`) — RED_MODE polarity (§11.4.115)

Unit `applyRAGContext` (disabled=byte-identical + zero retriever calls; enabled=augmented + ordering;
error=graceful-degrade no-panic; empty-messages) + httptest end-to-end for generateLLM AND streamLLM
(disabled-default byte-identical vs enabled-augmented, real rag.Adapter over a unit-test fakeRAGRetriever)
+ `TestGenerateLLM_RAG_Enabled_Augments_RegressionGuard` (`RED_MODE=1` replicates pre-fix no-RAG handler
→ asserts NEVER augmented; `RED_MODE=0` drives the wired handler → asserts augmented). No live provider
calls (in-process fakes; env has live keys = real $).

## Captured verification (-tags=nogui)

```
go build ./cmd/... ./internal/...  exit 0
go vet   ./cmd/... ./internal/...  exit 0
go test  ./internal/server/... ./internal/rag/...  all green, 0 new failures
anti-bluff smoke: clean
```

## Independent review (§11.4.142) — VERDICT: GO, zero blocking

Both paths wired identically; disabled truly byte-identical (verified via mutation + the second
independent gate in rag.Retrieve); graceful degrade + empty-messages correct; §1.1 — Mutation A (skip
PrependContext) → 3 tests FAIL, restored; disabled-path property proven by forcing SetEnabled(true) →
the unit test FAILs; RED_MODE both polarities verified against parent's genuine 0-RAG state; production
resolver defaults correctly, no test leakage; no live provider call.

## Honest boundary (§11.4.6) — native path wired; facade endpoints tracked separately

This closes the NATIVE server generate/stream path (`/api/v1/llm/generate`) + the CLI (already wired).
The OpenAI/Anthropic-compatible WIRE-FACADE endpoints (`/v1/chat/completions`, `/v1/messages`) still
bypass RAG — a smaller secondary surface, filed as a follow-up item (HXC-148) rather than over-claimed
here. HXC-118's core gap (the product's primary generate API is RAG-unwired) is closed.
