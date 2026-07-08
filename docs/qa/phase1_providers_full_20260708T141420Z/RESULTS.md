# Provider Phase 1 — Full Live Test Results

**Date:** 2026-07-08T17:41:38Z (second pass)

**Scope:** All providers with `API_KEY` env vars — nonce echo (NONCE-{uuid} echo), streaming (SSE chunk count), embeddings (dimension verification), tool-calling (function declaration round-trip).

**Root scope.** Coder untouched (cloud APIs only). NO push.

## Legend

| Column | What it tests |
|--------|---------------|
| Nonce Echo | Send `Reply with EXACTLY this nonce and nothing else: NONCE-XXXX` — PASS if nonce appears in response |
| Streaming | POST with `stream: true`, count SSE `data:` chunks |
| Embeddings | POST to `/embeddings` — PASS if returned vector dim > 0 |
| Tool Calling | POST with `tools: [{function: get_weather}]` — PASS if response contains `tool_calls` |

Failure reasons:
- **402** = insufficient credits/balance (account-level, not a code bug)
- **401** = bad/expired API key
- **404** = model name wrong or no access
- **429** = rate limit or credit exhausted

## Summary

| Provider | Nonce Echo | Streaming | Embeddings | Tool Calling | Status |
|----------|-----------|-----------|------------|--------------|--------|
| mistral | PASS | PASS | PASS | PASS | ALL PASS |
| codestral | PASS | PASS | N/A | N/A | ALL PASS |
| groq | PASS | PASS | N/A | PASS | ALL PASS |
| cohere | PASS | FAIL | PASS | N/A | PARTIAL |
| cerebras | FAIL | FAIL | N/A | FAIL | PARTIAL |
| sambanova | FAIL | FAIL | N/A | FAIL | PARTIAL |
| siliconflow | FAIL | FAIL | FAIL | FAIL | PARTIAL |
| zai | FAIL | FAIL | FAIL | FAIL | PARTIAL |
| deepseek | PASS | PASS | N/A | PASS | ALL PASS |
| fireworks | FAIL | FAIL | N/A | FAIL | PARTIAL |
| gemini | FAIL | FAIL | FAIL | FAIL | PARTIAL |
| github_models | FAIL | FAIL | N/A | FAIL | PARTIAL |
| huggingface | FAIL | FAIL | N/A | N/A | PARTIAL |
| hyperbolic | FAIL | FAIL | N/A | FAIL | PARTIAL |
| openrouter | FAIL | FAIL | N/A | FAIL | PARTIAL |
| novita | FAIL | FAIL | N/A | FAIL | PARTIAL |
| nvidia | PASS | PASS | FAIL | N/A | PARTIAL |
| nvidia_nv | FAIL | FAIL | N/A | N/A | PARTIAL |
| replicate | FAIL | FAIL | N/A | N/A | PARTIAL |
| upstage | FAIL | FAIL | FAIL | FAIL | PARTIAL |
| chutes | FAIL | FAIL | N/A | N/A | PARTIAL |
| venice | FAIL | FAIL | N/A | FAIL | PARTIAL |
| poe | FAIL | N/A | N/A | N/A | PARTIAL |
| tencentcloud | FAIL | FAIL | N/A | FAIL | PARTIAL |
| publicai | FAIL | FAIL | N/A | N/A | PARTIAL |
| zhipu | FAIL | FAIL | FAIL | FAIL | PARTIAL |
| zen | FAIL | FAIL | N/A | N/A | PARTIAL |

*\* nonce present but not exact — model added extra text*

## Detailed Results

### mistral

- **nonce_echo:** PASS
  - nonce: `NONCE-9E73A9CC6671431F`
  - response: `NONCE-9E73A9CC6671431F`
  - exact: `True`
- **streaming:** PASS
  - chunks: `6`
  - text_len: `13`
  - elapsed_s: `0.42`
- **embeddings:** PASS
  - dim: `1024`
  - tokens: `9`
- **tool_calling:** PASS
  - tool_call: `get_weather`

### codestral

- **nonce_echo:** PASS
  - nonce: `NONCE-37A17977E48F46CE`
  - response: `NONCE-37A17977E48F46CE`
  - exact: `True`
- **streaming:** PASS
  - chunks: `22`
  - text_len: `41`
  - elapsed_s: `2.06`

### groq

- **nonce_echo:** PASS
  - nonce: `NONCE-141785DF2E3D4886`
  - response: `NONCE-141785DF2E3D4886`
  - exact: `True`
- **streaming:** PASS
  - chunks: `16`
  - text_len: `14`
  - elapsed_s: `0.16`
- **tool_calling:** PASS
  - tool_call: `get_weather`

### cohere

- **nonce_echo:** PASS
  - nonce: `NONCE-E567D338CF7540E7`
  - response: `NONCE-E567D338CF7540E7`
  - exact: `True`
- **streaming:** FAIL
  - error: `405`
- **embeddings:** PASS
  - dim: `1024`
  - tokens: `7`

### cerebras

- **nonce_echo:** FAIL
  - error: `404: {"message":"Model llama3.1-8b does not exist or you do not have access to it.","type":"not_found_error","param":"model","code":"model_not_found"}`
  - nonce: `NONCE-70026141D4134214`
- **streaming:** FAIL
  - error: `404`
- **tool_calling:** FAIL
  - error: `404: {"message":"Model llama3.1-8b does not exist or you do not have access to it.","type":"not_found_error","param":"model","code":"model_not_found"}`

### sambanova

- **nonce_echo:** FAIL
  - error: `402: {"error":{"balance_units":0,"billing_portal_url":"https://fast-snova-ai-prod-1-ui.cloud.snova.ai/plans/billing","code":"PAYMENT_METHOD_REQUIRED","message":"A payment method is required. Add one at htt`
  - nonce: `NONCE-85EC8E2A4B6D4809`
- **streaming:** FAIL
  - error: `402`
- **tool_calling:** FAIL
  - error: `402: {"error":{"balance_units":0,"billing_portal_url":"https://cloud.sambanova.ai/plans/billing","code":"PAYMENT_METHOD_REQUIRED","message":"A payment method is required. Add one at https://cloud.sambanova`

### siliconflow

- **nonce_echo:** FAIL
  - error: `401: "Api key is invalid"`
  - nonce: `NONCE-B55262F66B4F4829`
- **streaming:** FAIL
  - error: `401`
- **embeddings:** FAIL
  - error: `401: "Api key is invalid"`
- **tool_calling:** FAIL
  - error: `401: "Api key is invalid"`

### zai

- **nonce_echo:** FAIL
  - error: `429: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`
  - nonce: `NONCE-A6251080EAF0445B`
- **streaming:** FAIL
  - error: `429`
- **embeddings:** FAIL
  - error: `400: {"error":{"code":"1211","message":"模型不存在，请检查模型代码。"}}`
- **tool_calling:** FAIL
  - error: `429: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`

### deepseek

- **nonce_echo:** PASS
  - nonce: `NONCE-8BBD938E8E014321`
  - response: `NONCE-8BBD938E8E014321`
  - exact: `True`
- **streaming:** PASS
  - chunks: `16`
  - text_len: `14`
  - elapsed_s: `1.05`
- **tool_calling:** PASS
  - tool_call: `get_weather`

### fireworks

- **nonce_echo:** FAIL
  - error: `404: {"error":{"message":"Model not found, inaccessible, and/or not deployed","param":"model","code":"NOT_FOUND","type":"error"},"request_id":"chatcmpl-6f275285b34a4e8e8edfa03476fcf65b"}`
  - nonce: `NONCE-442B31065B774D54`
- **streaming:** FAIL
  - error: `404`
- **tool_calling:** FAIL
  - error: `404: {"error":{"message":"Model not found, inaccessible, and/or not deployed","param":"model","code":"NOT_FOUND","type":"error"},"request_id":"chatcmpl-a9815992da4548238b03a4899b76fe73"}`

### gemini

- **nonce_echo:** FAIL
  - error: `400: {
  "error": {
    "code": 400,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/googl`
  - nonce: `NONCE-3D581838C0084B5E`
- **streaming:** FAIL
  - error: `400`
- **embeddings:** FAIL
  - error: `404: `
- **tool_calling:** FAIL
  - error: `400: {
  "error": {
    "code": 400,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/googl`

### github_models

- **nonce_echo:** FAIL
  - error: `401: {"error":{"code":"unauthorized","message":"Bad credentials","details":"Bad credentials"}}`
  - nonce: `NONCE-F35F2A579C9D4900`
- **streaming:** FAIL
  - error: `401`
- **tool_calling:** FAIL
  - error: `401: {"error":{"code":"unauthorized","message":"Bad credentials","details":"Bad credentials"}}`

### huggingface

- **nonce_echo:** FAIL
  - error: `[Errno -2] Name or service not known`
  - nonce: `NONCE-D7EF13E607034372`
- **streaming:** FAIL
  - error: `[Errno -2] Name or service not known`

### hyperbolic

- **nonce_echo:** FAIL
  - error: `402: {"detail":"Insufficient funds, please see https://docs.hyperbolic.xyz/docs/hyperbolic-pricing"}`
  - nonce: `NONCE-888EA313DF9B46B6`
- **streaming:** FAIL
  - error: `402`
- **tool_calling:** FAIL
  - error: `402: {"detail":"Insufficient funds, please see https://docs.hyperbolic.xyz/docs/hyperbolic-pricing"}`

### openrouter

- **nonce_echo:** FAIL
  - error: `402: {"error":{"message":"Insufficient credits. Add more using https://openrouter.ai/settings/credits","code":402}}`
  - nonce: `NONCE-2688651E34E243A5`
- **streaming:** FAIL
  - error: `402`
- **tool_calling:** FAIL
  - error: `402: {"error":{"message":"Insufficient credits. Add more using https://openrouter.ai/settings/credits","code":402}}`

### novita

- **nonce_echo:** FAIL
  - error: `401: {"code":401,"reason":"FAILED_TO_AUTH","message":"failed to authenticate API key","metadata":{}}`
  - nonce: `NONCE-BD19C120E7864118`
- **streaming:** FAIL
  - error: `401`
- **tool_calling:** FAIL
  - error: `401: {"code":401,"reason":"FAILED_TO_AUTH","message":"failed to authenticate API key","metadata":{}}`

### nvidia

- **nonce_echo:** PASS
  - nonce: `NONCE-A87317B243824DA1`
  - response: `NONCE-A87317B243824DA1`
  - exact: `True`
- **streaming:** PASS
  - chunks: `15`
  - text_len: `14`
  - elapsed_s: `1.06`
- **embeddings:** FAIL
  - error: `404: 404 page not found
`

### nvidia_nv

- **nonce_echo:** FAIL
  - error: `404: {"status":404,"title":"Not Found","detail":"Function 'cd89bd68-13e3-47a9-861e-9a62e6e14b05': Not found for account 'GdVPNjXQ2-wJkJswpqGTxPOeYFSDssOI_ypxdMykpGA'"}`
  - nonce: `NONCE-2AA4A969D2DB4652`
- **streaming:** FAIL
  - error: `404`

### replicate

- **nonce_echo:** FAIL
  - error: `401: {"title":"Unauthenticated","detail":"You did not pass a valid authentication token","status":401}
`
  - nonce: `NONCE-F325A3C413FC4723`
- **streaming:** FAIL
  - error: `401`

### upstage

- **nonce_echo:** FAIL
  - error: `403: {"error":{"message":"API key suspended due to insufficient credit. Register your payment method at https://console.upstage.ai/billing to continue.","type":"invalid_request_error","param":"","code":"ap`
  - nonce: `NONCE-F7396460DE224366`
- **streaming:** FAIL
  - error: `403`
- **embeddings:** FAIL
  - error: `403: {"error":{"message":"API key suspended due to insufficient credit. Register your payment method at https://console.upstage.ai/billing to continue.","type":"invalid_request_error","param":"","code":"ap`
- **tool_calling:** FAIL
  - error: `403: {"error":{"message":"API key suspended due to insufficient credit. Register your payment method at https://console.upstage.ai/billing to continue.","type":"invalid_request_error","param":"","code":"ap`

### chutes

- **nonce_echo:** FAIL
  - error: `404: {"detail":"No matching chute found!"}`
  - nonce: `NONCE-A9C18E23DDCF4583`
- **streaming:** FAIL
  - error: `404`

### venice

- **nonce_echo:** FAIL
  - error: `402: {"error":"Insufficient USD or Diem balance to complete request. Visit https://venice.ai/settings/api to add credits."}`
  - nonce: `NONCE-FBB45B68978842F1`
- **streaming:** FAIL
  - error: `402`
- **tool_calling:** FAIL
  - error: `402: {"error":"Insufficient USD or Diem balance to complete request. Visit https://venice.ai/settings/api to add credits."}`

### poe

- **nonce_echo:** FAIL
  - error: `404: 404: Not Found`
  - nonce: `NONCE-CAD175666F014CE3`

### tencentcloud

- **nonce_echo:** FAIL
  - error: `401: {"id":"07d3c799795fad81f05fe601d6311644","error":{"message":"not authorized","type":"not_authorized_error","param":null,"code":"not_authorized"}}`
  - nonce: `NONCE-3239107C0D9D4346`
- **streaming:** FAIL
  - error: `401`
- **tool_calling:** FAIL
  - error: `401: {"id":"92f8520ac02baa1cff65f6f624ce982a","error":{"message":"not authorized","type":"not_authorized_error","param":null,"code":"not_authorized"}}`

### publicai

- **nonce_echo:** FAIL
  - error: `401: {"message":"Session check failed"}`
  - nonce: `NONCE-02FEECFB1C5742D1`
- **streaming:** FAIL
  - error: `401`

### zhipu

- **nonce_echo:** FAIL
  - error: `429: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`
  - nonce: `NONCE-57DB979C840B4061`
- **streaming:** FAIL
  - error: `429`
- **embeddings:** FAIL
  - error: `400: {"error":{"code":"1211","message":"模型不存在，请检查模型代码。"}}`
- **tool_calling:** FAIL
  - error: `429: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`

### zen

- **nonce_echo:** FAIL
  - error: `404: {"message":"no Route matched with those values"}`
  - nonce: `NONCE-DE4657F481054CA3`
- **streaming:** FAIL
  - error: `404`

## Evidence

- Test script: `test_providers.py`
- Captured at: 2026-07-08T17:41:38Z
- Git range: `gpt-4o-mini` — full provider matrix
