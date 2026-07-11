# R41 — huggingface + together catalogue-first provider-coverage closure

**Track/branch:** (T1/feature/helixllm-full-extension - claude3)
**Date (UTC):** 2026-07-11
**File changed:** `helix_code/internal/llm/openai_compatible_catalogue.go` (+ its test file)
**Strategy:** catalogue-first (config rows, 0 new custom adapters, CONST-036) —
the pre-existing bespoke clients at `internal/llm/providers/huggingface` and
`internal/llm/providers/together` were confirmed (via the sibling
`scratchpad/r41_provider_currency_audit.md` audit) to be orphaned/unwired dead
code with retired endpoints. They were **intentionally left untouched**; this
task closes the capability gap by adding two catalogue rows instead.

## Step 1 — schema understanding

`HostedOpenAICompatible` row schema: `Name`, `BaseURL`, `KeyEnvAliases`,
`ModelEndpoint`, `ChatEndpoint` (no `DefaultModel` field on any of the 12
pre-existing rows — `NewHostedOpenAICompatibleProvider` never sets
`OpenAICompatibleConfig.DefaultModel`, so the underlying provider falls back
to auto-discovered models / `"gpt-3.5-turbo"`, matching every existing row).
Rows are consumed as a **degraded offline fallback** in
`internal/clientcore/providers.go`'s `buildOpenAICompatibleProviders` — the
primary source is LLMsVerifier (CONST-036); the hardcoded catalogue only
engages when the verifier is unreachable, and only builds a provider when a
key is present + non-placeholder (`firstPresentHostedKey`).

## Step 2 — §11.4.99 WebFetch confirmation

| Provider | OpenAI-compat confirmed? | Base URL | Source (fetched 2026-07-11) |
|---|---|---|---|
| huggingface | **YES** — official docs section "Alternative: OpenAI-Compatible Chat Completions Endpoint" | `https://router.huggingface.co/v1` | https://huggingface.co/docs/inference-providers/en/index |
| together | **YES** — "Together AI fully supports OpenAI-compatible Chat Completions APIs" | `https://api.together.ai/v1` | https://docs.together.ai/docs/openai-api-compatibility , https://docs.together.ai/reference/chat-completions-1 |

Both confirmed live via unauthenticated `curl` in addition to the docs fetch:
`GET router.huggingface.co/v1/models` → HTTP 200 (120 models, no auth
required); `GET api.together.ai/v1/models` and `GET api.together.xyz/v1/models`
→ both HTTP 401 "Missing API key" (live, reachable, correctly-shaped — a dead
host would not return this).

**Current/default models cited (§11.4.99, never invented per §11.4.6):**
- huggingface: `openai/gpt-oss-120b` — HF's own docs' primary worked example,
  independently cross-verified present in the live `GET /v1/models` response
  on 2026-07-11.
- together: `Qwen/Qwen3.5-9B` — Together's own official API-reference sample
  request, independently cross-verified as a live, currently-served model via
  `GET https://router.huggingface.co/v1/models` (which lists `together` as
  one of `Qwen/Qwen3.5-9B`'s serving providers) on the same date. The old
  bespoke client's hardcoded default `mistralai/Mixtral-8x22B-Instruct-v0.1`
  is confirmed retired — Together's serverless-models catalog (per the
  sibling `r41_provider_currency_audit.md` research) no longer lists Mistral
  at all.

## Step 3 — rows added + unit test

Both rows added following the EXACT existing pattern (comment block citing
sources/dates, struct literal with `ModelEndpoint: "/models"` +
`ChatEndpoint: "/chat/completions"` since both base URLs already end in
`/v1` — the file's documented URL-composition gotcha).

```go
{
    Name:          "huggingface",
    BaseURL:       "https://router.huggingface.co/v1",
    KeyEnvAliases: []string{"HF_TOKEN", "HUGGINGFACE_TOKEN", "HUGGINGFACE_API_KEY", "ApiKey_HuggingFace"},
    ModelEndpoint: "/models",
    ChatEndpoint:  "/chat/completions",
},
{
    Name:          "together",
    BaseURL:       "https://api.together.ai/v1",
    KeyEnvAliases: []string{"TOGETHER_API_KEY", "TOGETHERAI_API_KEY", "ApiKey_Together"},
    ModelEndpoint: "/models",
    ChatEndpoint:  "/chat/completions",
},
```

New test: `TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent`
in `openai_compatible_catalogue_test.go` — asserts both rows present with the
exact live-verified `BaseURL`/`ModelEndpoint`/`ChatEndpoint`, and
regression-guards against ever reverting to the retired
`api-inference.huggingface.co` host or a `Mixtral-8x22B` reference.

**§1.1 paired mutation:** reverted huggingface's `BaseURL` to the retired
`api-inference.huggingface.co` host → test FAILed with 3 distinct assertion
messages naming the regression. Mutation reverted, test re-confirmed GREEN.
Mutation is load-bearing.

```
$ go build ./internal/llm/...                                    # clean
$ go test ./internal/llm/ -run TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent -v
--- PASS: TestHostedOpenAICompatibleCatalogue_HuggingFaceAndTogetherPresent (0.00s)
$ go test ./internal/llm/ -count=1
ok  	dev.helix.code/internal/llm	102.923s
```

(`go build ./...` at the module root fails on `github.com/go-gl/gl` /
`github.com/go-gl/glfw` — a pre-existing missing-X11/OpenGL-devel-package
issue on this host, reproduced identically on `git stash` with zero changes
applied; unrelated to this task and not something a catalogue-data change
could cause. `go build ./internal/llm/...` — the actually-touched package —
is clean.)

## Step 4 — live-proof / honest SKIP

**huggingface — key present (`HF_TOKEN`, 37 chars, non-placeholder) — LIVE
PROOF, partial:**
- `GET https://router.huggingface.co/v1/models` → **PASS**, real HTTP 200,
  120 models, `openai/gpt-oss-120b` confirmed present.
- `POST https://router.huggingface.co/v1/chat/completions` (nonce
  round-trip) → **honest account-side block**, HTTP 402 "You have depleted
  your monthly included credits" — request reached the server and
  authenticated successfully (a bad key would be HTTP 401, not 402); this is
  the same honest-finding class already documented in this file for
  `fireworks` (412) and `kimi` (401 key-rejected) — endpoint/URL/model are
  correct, the nonce round-trip just cannot complete until the operator tops
  up HF credits. Recorded honestly, not faked as a full PASS.

**together — no key present in either `.env` (`TOGETHER_API_KEY`,
`TOGETHERAI_API_KEY`, `ApiKey_Together` all absent) — honest SKIP-with-reason
(§11.4.3).** Endpoint reachability independently confirmed instead via the
unauthenticated §11.4.99 research call (`HTTP 401 Missing API key` on both
candidate hosts, proving both are live).

Full transcript + sources: `docs/qa/r41_catalogue_hf_together_20260711T112230Z/EVIDENCE.md`.

## CONST-042 disclosure

While confirming `HF_TOKEN`'s presence/length via `sed`, an early diagnostic
command under-redacted and briefly echoed ~31 of the token's 37 characters
into this session's tool-output transcript (never into any committed file).
No repo file, commit, or docs/qa artefact contains any part of the key.
Flagging per CONST-042 — recommend the operator rotate
`HF_TOKEN`/`HUGGINGFACE_TOKEN` out of an abundance of caution.

## Commit

Local commit only, NOT pushed. Staged: catalogue file, test file, docs/qa
evidence dir only (`§11.4.84` — no `git add -A`, no unrelated files from other
concurrent streams).
