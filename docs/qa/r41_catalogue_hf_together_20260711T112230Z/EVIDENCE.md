# R41 catalogue-first fix: huggingface + together — live-proof evidence

**Run-id:** `r41_catalogue_hf_together_20260711T112230Z`
**Date (UTC):** 2026-07-11
**Scope:** §11.4.124 catalogue-first closure of the huggingface + together
capability gap in `helix_code/internal/llm/openai_compatible_catalogue.go`
(`HostedOpenAICompatibleCatalogue()`). CONST-042 note: this file NEVER prints
or persists a key VALUE — only presence/absence and HTTP outcome are recorded.

## huggingface — LIVE PROOF (key present: `HF_TOKEN`, 37 chars, non-placeholder)

### Step 1 — `GET https://router.huggingface.co/v1/models`

```json
{
  "http_status": 200,
  "model_count": 120,
  "openai/gpt-oss-120b_present": true
}
```

**Result: PASS.** Real HTTP 200 from the live router with the operator's
actual key, 120 models enumerated, the docs' own example/default model
(`openai/gpt-oss-120b`) confirmed present in the live response. This is
positive evidence the catalogue row's `BaseURL` + `ModelEndpoint` compose to a
genuinely working, current endpoint — not a bluff, not a metadata-only check.

### Step 2 — `POST https://router.huggingface.co/v1/chat/completions` (nonce round-trip)

Request: `model=openai/gpt-oss-120b:fastest`, prompt asked the model to echo
back a fresh nonce token generated at request time.

```json
{
  "http_status": 402,
  "error": "You have depleted your monthly included credits. Purchase pre-paid credits to continue using Inference Providers. Alternatively, subscribe to PRO to get 20x more included usage."
}
```

**Result: HONEST ACCOUNT-SIDE BLOCK — NOT a URL/endpoint/model bug.** The
request reached `router.huggingface.co`, authenticated successfully with the
operator's real key (a bad/expired key would return HTTP 401, not 402), and
was rejected specifically because the HF account's monthly Inference
Providers credit is depleted. This is the SAME class of honest finding
already documented in this catalogue file for `fireworks` (HTTP 412,
account-suspended) and `kimi` (HTTP 401, key rejected) — the row stays in the
catalogue because the endpoint/URL/model choice is correct; the chat-completion
nonce round-trip cannot be completed until the operator tops up HF credits or
upgrades to PRO. Per §11.4.3 this is recorded as an honest partial result, not
faked as a full PASS and not silently hidden.

**Combined verdict for huggingface:** endpoint + auth + model-listing = LIVE
PASS with real captured evidence; chat-completion nonce round-trip = honest
SKIP (account credit depleted, HTTP 402), not a code/URL defect.

## together — HONEST SKIP (no key present)

Checked env var names only (never values) in `helix_code/.env` and the
meta-repo root `.env` for `TOGETHER_API_KEY`, `TOGETHERAI_API_KEY`,
`ApiKey_Together`: none present in either file as of 2026-07-11.

**Result: SKIP-with-reason (§11.4.3) — no credential configured.** No live
call was attempted (attempting one without a key would only reproduce the
documented HTTP 401 "Missing API key" already captured during the §11.4.99
research pass below — that is not a meaningful additional proof and is not
worth spending a real network round-trip on). The catalogue row's `BaseURL` +
`ModelEndpoint`/`ChatEndpoint` composition is still verified correct by the
unit test (`TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent`,
`TestHostedOpenAICompatibleCatalogue_ModelsURLComposition`) and by the
§11.4.99 research call below.

### §11.4.99 research-time endpoint reachability check (no key, unauthenticated)

```
GET https://api.together.ai/v1/models  -> HTTP 401 "Missing API key"
GET https://api.together.xyz/v1/models -> HTTP 401 "Missing API key"
```

Both hosts resolve and respond with the SAME `401 Missing API key` shape,
confirming both are live, reachable OpenAI-compatible-shaped endpoints (a
dead/retired host would time out, DNS-fail, or 404 — not return a structured
"Missing API key" JSON error). `api.together.ai` is used as the catalogue
`BaseURL` because it is the host named in Together's own current official
docs (`docs.together.ai/docs/openai-api-compatibility`,
`docs.together.ai/reference/chat-completions-1`, both fetched 2026-07-11).

## Sources verified (§11.4.99)

- https://huggingface.co/docs/inference-providers/en/index — fetched 2026-07-11
- https://docs.together.ai/docs/openai-api-compatibility — fetched 2026-07-11
- https://docs.together.ai/reference/chat-completions-1 — fetched 2026-07-11
- https://www.together.ai/models — fetched 2026-07-11 (current model roster cross-check)
- Live `GET https://router.huggingface.co/v1/models` — fetched 2026-07-11 (unauthenticated + authenticated, both HTTP 200)
- Live `GET https://api.together.ai/v1/models` / `https://api.together.xyz/v1/models` — fetched 2026-07-11 (unauthenticated, both HTTP 401 "Missing API key")

## Unit test evidence

```
$ go test ./internal/llm/ -run TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent -v
=== RUN   TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent
--- PASS: TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent (0.00s)
PASS
ok  	dev.helix.code/internal/llm	0.008s
```

Paired §1.1 mutation (huggingface `BaseURL` reverted to the retired
`api-inference.huggingface.co` host) made the SAME test FAIL with:

```
openai_compatible_catalogue_test.go:237: huggingface BaseURL = "https://api-inference.huggingface.co/v1", want §11.4.99 live-verified "https://router.huggingface.co/v1"
openai_compatible_catalogue_test.go:262: huggingface BaseURL "https://api-inference.huggingface.co/v1" regressed to the retired api-inference.huggingface.co host
openai_compatible_catalogue_test.go:265: huggingface composed models URL "https://api-inference.huggingface.co/v1/models" does not use the router.huggingface.co host
--- FAIL: TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent (0.00s)
```

Mutation reverted, test re-confirmed GREEN before commit — the mutation is
load-bearing (§1.1).

Full package suite: `go test ./internal/llm/ -count=1` → `ok  dev.helix.code/internal/llm  102.923s`.

## CONST-042 note

While confirming `HF_TOKEN` was present and non-placeholder during this task,
an early diagnostic command (`sed` with an insufficiently long redaction
prefix) briefly echoed roughly 31 of the token's 37 characters into this
session's tool-output transcript (not into any committed file, log, or
docs/qa artefact). No committed file, git object, or this evidence file
contains any part of the key value. Per CONST-042 this partial transcript
exposure is flagged here for the operator's awareness; rotating
`HF_TOKEN`/`HUGGINGFACE_TOKEN` out of an abundance of caution is recommended
even though the exposure was confined to the interactive session transcript,
not persisted storage.
