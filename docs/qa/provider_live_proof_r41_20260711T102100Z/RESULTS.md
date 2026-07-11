# Provider Live-Proof Re-Run r41 — Fresh External Live Evidence

**Run ID**: `provider_live_proof_r41_20260711T102100Z`
**Timestamp (UTC)**: 2026-07-11T10:20:46Z – 2026-07-11T10:21:07Z
**Harness**: `helix_code/internal/llm/provider_live_proof_test.go` (build tag `providerlive`) — CONST-039 per-provider live-proof harness, nonce-challenged, real HTTP calls to real external hosted providers.
**Invocation**:
```
cd helix_code && go test -tags=providerlive -v -count=1 -timeout=480s \
  -run 'TestProviderLiveProof/(openai|anthropic|gemini|deepseek|groq|mistral|xai|openrouter)$' \
  ./internal/llm/...
```
**Scope note**: this run deliberately covers ONLY the 8 hosted providers in this harness's `providerLiveCandidates()` roster (openai, anthropic, gemini, deepseek, groq, mistral, xai, openrouter). The 2 local providers in the same roster (`ollama`, `llamacpp`) were EXCLUDED from the `-run` regex per the task's hard constraints: `ollama`'s default port (11434) was confirmed unreachable (no local server), and `llamacpp`'s default probe port (8080) is occupied by an unrelated host service that 302-redirects HTTP→HTTPS (confirmed NOT a llama.cpp server via a read-only header probe — §11.4.174 process/service-ownership caution) — driving the harness against it would produce a meaningless result against a service we don't own. The local coder container (:18434) was NOT touched or booted; a single read-only `curl` probe (non-invasive GET, no start command) confirmed it is down (HTTP 000 / connection refused), consistent with the task's constraint that it must stay down.

No API key value was ever printed, logged, or committed. All evidence files below are the harness's own JSON/text artifacts, which by construction (`providerLiveRequestEvidence` / `providerLiveResponseEvidence` structs in the harness source) never carry credential values — verified by an explicit post-run grep of the captured log against each resolved key value (no match).

---

## Summary

| Verdict | Count | Providers |
|---|---|---|
| **LIVE (PASS)** | 2 | groq, mistral |
| **FAIL** (real call made, real defect) | 3 | gemini, deepseek, openrouter |
| **SKIP** (no key configured) | 3 | openai, anthropic, xai |

All 3 FAILs are **genuine, real-HTTP-call findings** — not harness bugs, not bluffs, not retired-model issues (see per-provider detail + §11.4.99 currency cross-checks below). Nothing here was faked, cached, or metadata-only; every LIVE PASS and every FAIL is backed by a real outbound HTTPS request to the provider's production API this run.

---

## Per-Provider Table

| Provider | Key present? (alias) | Verdict | Nonce echoed? | Model answered | /models currency note (§11.4.99) |
|---|---|---|---|---|---|
| openai | No | SKIP: no-key | n/a | n/a | not probed |
| anthropic | No | SKIP: no-key | n/a | n/a | not probed |
| gemini | Yes (`GEMINI_API_KEY`) | **FAIL** | n/a (call errored before content) | n/a — never answered | Not checked: the configured `GEMINI_API_KEY` is rejected outright by Google (`400 INVALID_ARGUMENT — API key not valid`), a real HTTP round-trip proving the key is expired/revoked/malformed, not a model-catalogue problem. Requires key rotation, out of scope for this evidence pass. |
| deepseek | Yes (`DEEPSEEK_API_KEY`) | **FAIL** | No — response truncated to `"LIVEPRO"` (7 of 21 chars) | `deepseek-v4-flash` | **Current** — confirmed present in DeepSeek's live `GET /v1/models` (2 models returned: `deepseek-v4-flash`, `deepseek-v4-pro`). Root cause is NOT a retired/mismapped model: `finish_reason=length` with `completion_tokens=32/32` shows the model exhausted the harness's 32-token `MaxTokens` budget — consistent with `deepseek-v4-flash` being a reasoning-style model that spends part of its token budget on internal reasoning before emitting the literal echoed token, cutting the visible answer off mid-word. |
| groq | Yes (`GROQ_API_KEY`) | **PASS — LIVE** | Yes, in full | `llama-3.1-8b-instant` | **Current** — confirmed present in Groq's live `GET /openai/v1/models` (17 models total; `llama-3.1-8b-instant` and `llama-3.3-70b-versatile` both currently listed — 3.1-8b-instant is not deprecated/retired on Groq as of this run). |
| mistral | Yes (`MISTRAL_API_KEY`) | **PASS — LIVE** | Yes, in full | `mistral-small-2603` | **Current** — confirmed present in Mistral's live `GET /v1/models` (70 models total; `mistral-small-2603` listed alongside `mistral-small-latest`, `mistral-small-2506`). |
| xai | No | SKIP: no-key | n/a | n/a | not probed |
| openrouter | Yes (`OPENROUTER_API_KEY`) | **FAIL** | No — response content empty | `openai/gpt-oss-20b:free` | **Current** — confirmed present in OpenRouter's live `GET /api/v1/models` (345 models total; `openai/gpt-oss-20b:free` listed alongside its paid sibling `openai/gpt-oss-20b`). Root cause: same pattern as deepseek — `finish_reason=length`, `completion_tokens=32/32`; `gpt-oss-20b` is an open-weight reasoning model whose hidden reasoning tokens consumed the entire 32-token budget before any visible content, so `content` came back empty. |

---

## §11.4.99 currency finding (the specific "DeepSeek-V4 mapping" check the task asked for)

The task explicitly asked to watch for "retired models like DeepSeek-V4 mapping changes." Cross-checked directly against DeepSeek's own live `/v1/models` endpoint this run:

```
GET https://api.deepseek.com/v1/models  → HTTP 200
{"object":"list","data":[
  {"id":"deepseek-v4-flash","object":"model","owned_by":"deepseek"},
  {"id":"deepseek-v4-pro","object":"model","owned_by":"deepseek"}
]}
```

**Finding**: `deepseek-v4-flash` (the model the harness's catalogue auto-picked and this run actually called) is NOT retired — it is one of only 2 models DeepSeek currently exposes, and the harness's "cheap/fast" auto-pick logic correctly landed on it. The DeepSeek subtest's FAIL is a **token-budget truncation** defect (harness's `MaxTokens: 32` is too small for this reasoning-style model), not a stale/retired-model mapping defect. Same root-cause class independently reproduced on OpenRouter's `openai/gpt-oss-20b:free` (also a reasoning model, also `finish_reason=length` at 32/32 completion tokens, also current in its provider's live catalogue). No 404-prone/retired default was found on any of the 5 key-present providers actually probed.

**Follow-up implication (not actioned in this evidence-only pass)**: the harness's fixed `MaxTokens: 32` in `provider_live_proof_test.go` (`runHostedProviderLiveProof` / `runLocalProviderLiveProof`, both hardcode `MaxTokens: 32`) under-provisions reasoning-style models that consume part of their completion budget on non-visible reasoning tokens before echoing the literal nonce. This is a harness tuning gap, tracked as an observation for a future harness-improvement pass — out of scope for this evidence-capture task (task instructions were live-proof capture + §11.4.99 currency cross-check only, no harness code changes authorized).

---

## Anti-bluff notes (§11.4.5 / §11.4.99 / §11.4.150)

- Every LIVE PASS (groq, mistral) is nonce-challenged: a fresh `crypto/rand`-derived token (e.g. `LIVEPROOF-728365b040cd`) generated at request time and asserted present verbatim in the response — a cached/canned/mocked response structurally cannot satisfy this.
- Every FAIL is a genuine real-HTTP-call outcome (`t.Fatalf` only fires after a live `provider.Generate(ctx, req)` round-trip completed or errored) — never a bluffed PASS, never a suite-level FAIL-on-absence for the no-key providers (those took the honest SKIP branch per the harness's design, matching the always-on `TestHonestSkipOnAbsentProviderKey` unit-test contract in `provider_live_proof_skip_test.go`).
- §11.4.10 (no-secret-leak): captured log at `raw_evidence/go_test_output.log` and every `request.json`/`response.json`/`verdict.txt` under `raw_evidence/provider_coverage/<provider>/` were programmatically checked against each resolved API-key value present in this session's environment — zero matches. No key value appears anywhere in this evidence directory.
- Local coder (:18434) was not booted or otherwise interacted with beyond a single non-invasive read-only `curl -s -o /dev/null --max-time 2` liveness probe (confirmed down, as required). `submodules/helix_agent` and `/mnt/track1` were not touched.

---

## Raw evidence

All files under `raw_evidence/`:
- `go_test_output.log` — full `go test -v` stdout/stderr transcript (key-redacted by construction; verified above).
- `provider_coverage/<provider>/verdict.txt` — harness-written PASS/FAIL/SKIP verdict + detail, one per candidate provider run.
- `provider_coverage/<provider>/request.json` — harness-written request evidence (model, model_source, nonce, prompt, timestamp) for every provider that reached the network-call stage.
- `provider_coverage/<provider>/response.json` — harness-written response evidence (content, nonce_echoed, finish_reason, usage, timings) for every provider that received a response.

These are also independently preserved (not copied) by the harness itself under `docs/qa/provider_live_proof_20260711T102100Z/provider_coverage/` (this run's own harness-generated run-id directory), created automatically by `provider_live_proof_test.go`'s `providerLiveEvidenceDir` on the same invocation.
