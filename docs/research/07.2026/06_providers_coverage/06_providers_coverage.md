# Provider-Coverage Research — LLMsVerifier + HelixAgent

**Scope:** Deep multi-angle web research to enable an adapter to be written for every
provider/model the operator named, plus the highest-value additional providers.
**Grounding fact:** HelixAgent already implements ~45 provider adapters under
`submodules/helix_agent/internal/llm/providers/*` (openai, anthropic, claude, gemini,
deepseek, groq, mistral, ollama, lmstudio, cerebras, chutes, together, nvidia, sambanova,
openrouter, huggingface, helixllm, vertex, azure, cohere, …). LLMsVerifier is the single
source of truth for model/provider metadata (CONST-036/037).
**Author:** T1/main deep-research subagent. **Access date for ALL citations:** 2026-07-06.
**Anti-bluff (§11.4.6):** nothing below was invented. Any endpoint/model I could not confirm
from an official/authoritative source is marked `UNCONFIRMED:` or `NEEDS-CLARIFICATION`.

> **Revision:** 1 · **Last modified:** 2026-07-06 · **Status summary:** initial research pass.

---

## Legend

- **Status** = confidence of the provider having a real, public, callable API:
  - `CONFIRMED` — public API + base URL verified from an official/authoritative source.
  - `UNCONFIRMED` — provider/model is real, but a specific API detail (base URL, auth, compat) could NOT be verified from an official source this pass.
  - `NEEDS-CLARIFICATION` — the operator's name is ambiguous or does not map to a callable LLM API; operator input required before an adapter can be scoped.
- **OAI** = OpenAI-compatible `/v1/chat/completions`. **Anth** = Anthropic-compatible `/v1/messages`.

---

## GROUP A — CONFIRMED, public API, adapter-ready (operator-named targets)

| Provider | Base URL | Auth | OAI | Anth | Models endpoint | Flagship / latest models | Tools | Vision | Status | Source |
|---|---|---|---|---|---|---|---|---|---|---|
| **Poe (Quora)** | `https://api.poe.com/v1` (OpenAI-compatible interface) | Bearer API key (Poe account) | Yes | Partial (proxies Anthropic models *through* OAI iface) | via OpenAI-style `/models` (100+ bots) | Aggregator: routes to GPT, Claude, Gemini, Grok + image/video/voice (Imagen 4, Veo 3, Flux, Kling, ElevenLabs) + community bots | Yes (per underlying model) | Yes (multimodal) | CONFIRMED (API launched 2025‑07‑31; point-based billing, not per-token) | [TechCrunch](https://techcrunch.com/2025/07/31/quoras-poe-is-releasing-an-api-for-developers-to-easily-access-a-boquet-of-models/) · [HyperAI](https://hyper.ai/en/stories/cd3cd6a39b8b109839aa301d2a150f76) |
| **Perplexity (Sonar)** | `https://api.perplexity.ai` | Bearer API key | Yes | No | (OAI client libs; Sonar models are fixed set) | `sonar`, `sonar-pro`, `sonar-reasoning-pro`, `sonar-deep-research` (web-grounded) | Yes (structured outputs) | Yes (image/file handling) | CONFIRMED | [docs.perplexity.ai](https://docs.perplexity.ai/docs/sonar/quickstart) · [API platform](https://www.perplexity.ai/api-platform) |
| **Sakana AI (Fugu)** | `https://api.sakana.ai/v1` (per-account base URL shown in console) | Bearer API key (console.sakana.ai) | Yes | No (native OAI; also supports Responses API) | `GET /v1/models` | `fugu`, `fugu-ultra` (`fugu-ultra-20260615`) — multi-agent orchestration exposed as a single model | Yes | UNCONFIRMED (multimodal not verified) | CONFIRMED (launched 2026‑06‑22) | [sakana.ai/fugu](https://sakana.ai/fugu/) · [console models](https://console.sakana.ai/models) · [get-started](https://console.sakana.ai/get-started) |
| **Xiaomi (MiMo)** | OAI: `https://api.xiaomimimo.com/v1` · Anthropic: `https://api.xiaomimimo.com/anthropic` | Bearer API key | Yes | Yes | OAI `/v1/models` | `mimo-v2.5-pro` (flagship), `MiMo-V2-Pro` (>1T total / 42B active, 1M ctx), `MiMo-V2-Flash` (309B/15B MoE), `MiMo-7B` | Yes | Yes (v2.5 pro) | CONFIRMED (open platform, ~$0.1/M input) | [mimo.mi.com](https://mimo.mi.com/) · [litellm/xiaomi_mimo](https://docs.litellm.ai/docs/providers/xiaomi_mimo) · [GitHub XiaomiMiMo/MiMo](https://github.com/xiaomimimo/mimo) |
| **Tencent Hunyuan** | `https://api.hunyuan.cloud.tencent.com/v1` | Tencent Cloud API key (Bearer) | Yes | No (OAI-only verified) | OAI `/v1/models` | `hunyuan-t1` (reasoning, TurboS Hybrid Transformer‑Mamba MoE), `hunyuan-t1-vision-20250916`, `hunyuan` general chat, HY‑2.0‑Instruct; also open 0.5B/1.8B/4B/7B | Yes | Yes (T1‑Vision) | CONFIRMED | [hunyuan.tencent.com](https://hunyuan.tencent.com/) · [Tencent Cloud AI Gateway doc](https://intl.cloud.tencent.com/ind/document/product/1290/79463) |

### Notes on Group A operator targets

- **Poe** is an **aggregator/gateway** (like OpenRouter), not a first-party model house. Billing is Poe **points** tied to subscription tiers (`$4.99`→`$249.99`/mo) + `$30`/1M extra points — NOT a per-token API key with usage billing. An adapter treats it as an OpenAI-compatible gateway; model discovery returns 100+ bot IDs.
- **Tencent Yuanbao** = the **consumer assistant app** (first released 2024‑05‑30) front-ending Hunyuan; it is **NOT a separate developer API**. The programmatic surface is the **Hunyuan** API above. A community "yuanbao-free-api" reverse-proxy exists ([chenwr727/yuanbao-free-api](https://github.com/chenwr727/yuanbao-free-api)) but is unofficial and MUST NOT be used as an official adapter target. **Adapter target = Hunyuan.**

---

## GROUP B — NEEDS-CLARIFICATION (operator name ambiguous / not a callable LLM API)

| Operator term | What it actually is (verified) | Callable LLM API? | Recommended action | Source |
|---|---|---|---|---|
| **Google "OKF"** | **Open Knowledge Format** — a Google Cloud **open specification** (markdown + YAML frontmatter) for representing agent context/knowledge, published **2026‑06‑12**. It is a *data/metadata format*, **NOT a model and NOT an LLM provider/API.** | **No** | **Do not scope a model adapter.** OKF is relevant to HelixAgent's *context/knowledge* layer, not the provider layer. If the operator meant a Google *model* API, the target is Gemini (already implemented) / Vertex (already implemented). Operator clarification required. | [Google Cloud Tech (X)](https://x.com/GoogleCloudTech/status/2067012903337664886) · [explainx OKF](https://www.explainx.ai/blog/google-open-knowledge-format-okf-ai-agents-2026) |
| **"GPT SOL"** | Most-probable match: **OpenAI GPT‑5.6 "Sol"** (Sol/Terra/Luna family, announced **2026‑06‑26**) — **limited preview to select partners only**, no broad/public API yet. (Alt reading "Solar" = Upstage Solar Pro/Mini, a *different* vendor with its own API.) | **Not yet public** (Sol) / Yes (if Upstage Solar) | Clarify which is meant. If **GPT‑5.6 Sol**: it will be served via the **existing OpenAI adapter** once GA — no new adapter, just a model-id add. If **Upstage Solar**: that is a distinct provider needing its own adapter. | [DataCamp GPT‑5.6 Sol](https://www.datacamp.com/blog/gpt-5-6-sol-luna-terra) · [Upstage Solar](https://www.upstage.ai/solar-llm) |
| **GOT family** | **GOT‑OCR2.0** — open-source **OCR** vision-text model (580M params, VitDet encoder + Qwen‑0.5B decoder, 8K ctx), Apache-style research release by Haoran Wei et al. | Self-host only (no first-party hosted API) | Adapter is feasible ONLY as a **self-hosted / HF-inference** target (like the existing huggingface/ollama path), and it is an **OCR** model, not a general chat LLM. Confirm the operator wants OCR coverage. | [HF GOT‑OCR2 docs](https://huggingface.co/docs/transformers/en/model_doc/got_ocr2) · [arXiv 2409.01704](https://arxiv.org/pdf/2409.01704) · [GitHub Ucas‑HaoranWei/GOT‑OCR2.0](https://github.com/Ucas-HaoranWei/GOT-OCR2.0) |

---

## GROUP C — CONFIRMED provider, but distribution model needs an operator decision

| Provider | What it is | API shape | Status | Source |
|---|---|---|---|---|
| **Subquadratic (SubQ)** | Miami frontier-AI startup (founded 2026, CEO Justin Dangel, CTO Alexander Whedon). Model **SubQ 1M‑Preview** — "fully subquadratic" SSA attention, 12M-token research context. | Three products in **private beta**: an API exposing the full context window, `SubQ Code` CLI, `SubQ Search`. Base URL / auth **NOT publicly documented** yet. | UNCONFIRMED (real company + model; **no public API endpoint verified** — private beta only; independent verification of claims still disputed) | [subq.ai/introducing-subq](https://subq.ai/introducing-subq) · [MIT Tech Review](https://www.technologyreview.com/2026/06/19/1139313/a-startup-claims-it-broke-through-a-bottleneck-thats-holding-back-llms/) · [VentureBeat](https://venturebeat.com/technology/miami-startup-subquadratic-claims-1-000x-ai-efficiency-gain-with-subq-model-researchers-demand-independent-proof) |

**Recommendation:** Subquadratic cannot be adapter-scoped until it exposes a public API with a documented base URL/auth. Track for GA; the API is expected OpenAI-shaped per the beta description but this is **UNCONFIRMED**.

---

## GROUP D — CONFIRMED single models to run via existing self-host paths (no new hosted adapter)

| Model | What it is | How to call | Status | Source |
|---|---|---|---|---|
| **Qwythos 9B** (`empero-ai/Qwythos-9B-Claude-Mythos-5-1M`) | Full-parameter reasoning model on a Qwen3.5‑9B base, 1M-token ctx, native Qwen function-calling, Apache‑2.0, released by **Empero AI** June 2026. **No first-party hosted API.** | Self-host via **vLLM / SGLang / Transformers**; GGUF quant runs on **llama.cpp / Ollama / LM Studio / Jan / KoboldCpp** → reachable through HelixAgent's **existing `ollama` / `lmstudio` / `huggingface` adapters.** | CONFIRMED (as a downloadable model; **not** a hosted provider) | [HF empero-ai/Qwythos‑9B](https://huggingface.co/empero-ai/Qwythos-9B-Claude-Mythos-5-1M) · [deployment guide](https://knightli.com/en/2026/06/24/qwythos-9b-claude-mythos-1m-context-guide/) |

**Recommendation:** No new adapter needed — register Qwythos-9B as a model behind the Ollama/LM Studio/HF-local providers already implemented. Confirm the "Claude-Mythos / uncensored" provenance is acceptable per policy before shipping.

---

## GROUP E — Highest-value ADDITIONAL providers to add (not in the ~45 list) — all CONFIRMED

| Provider | Base URL | Auth | OAI | Anth | Models endpoint | Flagship / latest | Tools | Vision | Status | Source |
|---|---|---|---|---|---|---|---|---|---|---|
| **xAI (Grok)** | `https://api.x.ai/v1` | Bearer API key | Yes | Yes (Anthropic SDK supported) | `GET /v1/models` | `grok-4.3` (1M ctx, agentic), `grok-4.1-fast` (2M ctx, cheap); older `grok-4*` slugs redirect | Yes (strong) | Yes | CONFIRMED — **top priority add** | [docs.x.ai/developers/models](https://docs.x.ai/developers/models) · [x.ai/api](https://x.ai/api) |
| **Moonshot AI (Kimi)** | `https://api.moonshot.ai/v1` | Bearer API key | Yes | Yes (OpenAI *and* Anthropic compatible) | OAI `/v1/models` | `kimi-k2.6` (latest), `kimi-k2.7-code`, `kimi-k2.5` (vision + thinking) | Yes | Yes (k2.5) | CONFIRMED — **top priority add** | [platform.moonshot.ai](https://platform.moonshot.ai/) · [GitHub MoonshotAI/Kimi-K2](https://github.com/moonshotai/kimi-k2) |
| **Zhipu AI (GLM / Z.ai)** | OAI: `https://open.bigmodel.cn/api/paas/v4/` · Anthropic: `https://open.bigmodel.cn/api/anthropic` (intl alias `https://api.z.ai/...`) | Bearer API key | Yes | Yes | OAI v4 models | `GLM-4.6`, `GLM-4.5` | Yes | Yes | CONFIRMED — **top priority add** (region-aware routing intl vs CN) | [zhipuai PyPI](https://pypi.org/project/zhipuai/) · [ai-sdk zhipu](https://ai-sdk.dev/providers/community-providers/zhipu) · [Roo Code Z.ai](https://docs.roocode.com/providers/zai) |
| **Fireworks AI** | `https://api.fireworks.ai/inference/v1` | Bearer API key | Yes | No | OAI `/v1/models` | Hosted open models (Llama, Qwen, DeepSeek, Kimi, GLM, GPT‑OSS); model ids like `accounts/fireworks/models/deepseek-v3p1-terminus` | Yes | Yes (per model) | CONFIRMED — high value (open-model serving) | [docs.fireworks.ai openai-compatibility](https://docs.fireworks.ai/tools-sdks/openai-compatibility) |
| **DeepInfra** | `https://api.deepinfra.com/v1/openai` | Bearer API key | Yes | No | OAI `/v1/openai/models` | Broad open-model catalog (Llama/Qwen/DeepSeek/…) pay-per-use | Yes | Yes (per model) | CONFIRMED | [deepinfra.com/docs/openai_api](https://deepinfra.com/docs/openai_api) |
| **Novita AI** | `https://api.novita.ai/v3/openai` (legacy `…/openai`) | Bearer API key | Yes | No | OAI models | Open-model catalog (Llama 3, DeepSeek, Qwen, …) | Yes | Yes (per model) | CONFIRMED | [novita.ai/docs/guides/llm-api](https://novita.ai/docs/guides/llm-api) |
| **AI21 Labs** | `https://api.ai21.com/studio/v1` (also on Bedrock/Azure Foundry) | Bearer API key | Partial (native REST + OAI-ish) | No | REST `/v1/*` | `jamba-large` (→ `jamba-large-1.7`), `jamba-mini` (→ `jamba-mini-2`) — long-context SSM/Transformer hybrid | Yes | No | CONFIRMED (native API shape; verify exact `/chat/completions` path) | [docs.ai21.com Jamba](https://docs.ai21.com/docs/jamba-foundation-models) · [litellm/ai21](https://docs.litellm.ai/docs/providers/ai21) |
| **Reka AI** | `https://api.reka.ai/v1` (v0 legacy `https://api.reka.ai/`) | Bearer API key | Yes (fully OAI-compatible) | No | OAI models | `reka-core`, `reka-flash`, `reka-edge`, `reka-flash-research` | Yes | Yes (multimodal) | CONFIRMED | [docs.reka.ai](https://docs.reka.ai/) · [quickstart](https://docs.reka.ai/quickstart) |
| **Hyperbolic** | `https://api.hyperbolic.xyz/v1` | Bearer API key | Yes | No | OAI models | Open-model GPU inference (Llama, Qwen, DeepSeek) | Yes | Yes (per model) | UNCONFIRMED (base URL from LiteLLM 3rd-party, not fetched from official docs this pass) | [litellm/hyperbolic](https://docs.litellm.ai/docs/providers/hyperbolic) |
| **Baseten** | Model-specific / dedicated-deploy endpoints (Model APIs `https://inference.baseten.co/v1` per model) | Bearer API key | Yes (OAI-compatible Model APIs) | No | per-deployment | Bring-your-own + hosted open models (Model APIs) | Yes | per model | UNCONFIRMED (per-model deploy model; treat like a self-host gateway — confirm exact base URL from official docs before adapter) | [apimart MAI on Baseten](https://apimart.ai/blog/mai-models-fireworks-ai-baseten-open-router) |

---

## Recommendation summary — what to build

1. **Add first (CONFIRMED, mainstream, OAI+Anth):** **xAI Grok**, **Moonshot Kimi**, **Zhipu GLM** — all three are dual OpenAI+Anthropic compatible with documented base URLs and are widely used. Highest ROI.
2. **Add next (CONFIRMED open-model gateways, OAI):** **Fireworks**, **DeepInfra**, **Novita** — thin OpenAI-compatible adapters, large catalogs, cheap.
3. **Add (CONFIRMED first-party model houses):** **AI21 Jamba**, **Reka** — smaller but distinctive (long-context SSM; multimodal).
4. **Operator-named CONFIRMED, adapter-ready:** **Perplexity Sonar**, **Xiaomi MiMo**, **Tencent Hunyuan**, **Sakana Fugu**, **Poe** (as an OAI gateway). Note Poe = points billing; Yuanbao = the Hunyuan API.
5. **Self-host path, no new hosted adapter:** **Qwythos 9B** (Ollama/LM Studio/HF), **GOT‑OCR2.0** (HF/self-host, OCR-specific).
6. **Verify base URL before coding:** **Hyperbolic**, **Baseten** (3rd-party sourced this pass).
7. **Blocked pending public API:** **Subquadratic SubQ** (private beta, no public endpoint).
8. **Not a provider — clarify with operator:** **Google "OKF"** (a knowledge *format*, not a model), **"GPT SOL"** (likely OpenAI GPT‑5.6 Sol → existing OpenAI adapter once GA; or Upstage Solar → separate adapter).

---

## Sources verified 2026-07-06

- Poe/Quora API: https://techcrunch.com/2025/07/31/quoras-poe-is-releasing-an-api-for-developers-to-easily-access-a-boquet-of-models/ , https://hyper.ai/en/stories/cd3cd6a39b8b109839aa301d2a150f76
- Perplexity Sonar: https://docs.perplexity.ai/docs/sonar/quickstart , https://www.perplexity.ai/api-platform
- Sakana Fugu: https://sakana.ai/fugu/ , https://console.sakana.ai/get-started , https://console.sakana.ai/models
- Xiaomi MiMo: https://mimo.mi.com/ , https://docs.litellm.ai/docs/providers/xiaomi_mimo , https://github.com/xiaomimimo/mimo
- Tencent Hunyuan / Yuanbao: https://hunyuan.tencent.com/ , https://intl.cloud.tencent.com/ind/document/product/1290/79463 , https://aiwiki.ai/wiki/tencent_yuanbao
- Google OKF: https://x.com/GoogleCloudTech/status/2067012903337664886 , https://www.explainx.ai/blog/google-open-knowledge-format-okf-ai-agents-2026
- GOT-OCR2.0: https://huggingface.co/docs/transformers/en/model_doc/got_ocr2 , https://arxiv.org/pdf/2409.01704 , https://github.com/Ucas-HaoranWei/GOT-OCR2.0
- "GPT SOL" (GPT-5.6 Sol / Upstage Solar): https://www.datacamp.com/blog/gpt-5-6-sol-luna-terra , https://www.upstage.ai/solar-llm
- Subquadratic SubQ: https://subq.ai/introducing-subq , https://www.technologyreview.com/2026/06/19/1139313/a-startup-claims-it-broke-through-a-bottleneck-thats-holding-back-llms/ , https://venturebeat.com/technology/miami-startup-subquadratic-claims-1-000x-ai-efficiency-gain-with-subq-model-researchers-demand-independent-proof
- Qwythos 9B: https://huggingface.co/empero-ai/Qwythos-9B-Claude-Mythos-5-1M , https://knightli.com/en/2026/06/24/qwythos-9b-claude-mythos-1m-context-guide/
- xAI Grok: https://docs.x.ai/developers/models , https://x.ai/api
- Moonshot Kimi: https://platform.moonshot.ai/ , https://github.com/moonshotai/kimi-k2
- Zhipu GLM / Z.ai: https://pypi.org/project/zhipuai/ , https://ai-sdk.dev/providers/community-providers/zhipu , https://docs.roocode.com/providers/zai
- Fireworks: https://docs.fireworks.ai/tools-sdks/openai-compatibility
- DeepInfra: https://deepinfra.com/docs/openai_api
- Novita: https://novita.ai/docs/guides/llm-api
- AI21 Jamba: https://docs.ai21.com/docs/jamba-foundation-models , https://docs.litellm.ai/docs/providers/ai21
- Reka: https://docs.reka.ai/ , https://docs.reka.ai/quickstart
- Hyperbolic: https://docs.litellm.ai/docs/providers/hyperbolic
- Baseten: https://apimart.ai/blog/mai-models-fireworks-ai-baseten-open-router
