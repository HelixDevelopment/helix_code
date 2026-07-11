# Phase 1 — Provider Live Probe: Cohere + Gemini

**Date:** 2026-07-08T15:41Z  
**Scope:** Probe Cohere chat endpoint and Gemini key validity with real API calls  
**Status:** READ-ONLY INVESTIGATION (no code changed)

---

## Cohere

### Current code location
`helix_code/internal/llm/providers/cohere/client.go`

### Current base URL
`https://api.cohere.com/v1/chat` (constant `CohereBaseURL`)

### Result: DEAD — 404
All models (`command-r-plus`, `command`, `command-r`) return HTTP 404:
> `model 'command-r' was removed on September 15, 2025.`

### Working endpoint
`https://api.cohere.com/v2/chat` with `messages` array (OpenAI-compatible format) and model `command-r-08-2024`.

### Live nonce echo proof
```
Nonce: COHERE-PROBE-20260708
Request:  POST https://api.cohere.com/v2/chat
Response: {"id":"7543355b-...","message":{"role":"assistant","content":[{"type":"text","text":"COHERE-PROBE-20260708"}]},"finish_reason":"COMPLETE"}
HTTP 200
```

### Required fixes
1. Base URL: `https://api.cohere.com/v1/chat` → `https://api.cohere.com/v2/chat`
2. Request format: legacy `{"message":...,"chat_history":[...]}` → OpenAI-compat `{"messages":[{"role":"user","content":"..."}]}`
3. Response parsing: old `{"text":"...","finish_reason":"..."}` → new `{"message":{"role":"assistant","content":[{"type":"text","text":"..."}]},"finish_reason":"..."}`
4. Default model: `command-r-plus` (removed Sep 2025) → `command-r-08-2024`

---

## Gemini

### Current code location
`helix_code/internal/llm/gemini_provider.go`

### API key
`GEMINI_API_KEY=<REDACTED-GEMINI-KEY-CONST-042-rotate-required-see-Issues>` (from `.env` + env var)

### Result: KEY INVALID — 400
Both native and OpenAI-compatible endpoints reject:
```
"error": {"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}
"reason": "API_KEY_INVALID"
```

The key is not recognized by Google's API infrastructure — it was either never activated or has been revoked/deleted from Google AI Studio or Google Cloud console.

### Required fixes (blocked on key)
1. **Operator action**: obtain a new valid Gemini API key from https://aistudio.google.com/app/apikey
2. Code: update `GEMINI_API_KEY` in `.env` with the new key
3. Alternatively, if the desired route is the OpenAI-compatible endpoint:
   - Base URL: `https://generativelanguage.googleapis.com/v1beta` → may need adjusting per Google docs
   - Endpoint: `https://generativelanguage.googleapis.com/v1beta/openai/chat/completions`
   - Model mapping: Gemini model IDs in OpenAI-compat mode
   - Auth: `Authorization: Bearer <key>` or `x-goog-api-key` header

---

## Evidence captured
- Cohere v2 chat with nonce echo: HTTP 200, text `COHERE-PROBE-20260708` returned
- Cohere v1/chat: HTTP 404 (dead endpoint)
- Gemini native API: HTTP 400 `API_KEY_INVALID`
- Gemini OpenAI-compat API: HTTP 400 `API_KEY_INVALID`
