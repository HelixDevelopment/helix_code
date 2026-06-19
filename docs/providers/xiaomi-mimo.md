# Xiaomi MiMo API Reference

## Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/v1/models` | GET | List available models |
| `/v1/chat/completions` | POST | Chat completions (text gen) |

## Authentication

Header: `Authorization: Bearer <api-key>` or `api-key: <api-key>`

Key types:
- Pay-as-you-go: `sk-xxxxx`
- Token Plan: `tp-xxxxx`

## Chat Completions Request

```json
{
  "model": "mimo-v2.5-pro",
  "messages": [
    {"role": "system", "content": "You are MiMo, an AI assistant developed by Xiaomi."},
    {"role": "user", "content": "Hello"}
  ],
  "max_completion_tokens": 1024,
  "temperature": 1.0,
  "top_p": 0.95,
  "stream": false,
  "tools": [],
  "tool_choice": "auto"
}
```

## Response with Reasoning

```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help you?",
      "reasoning_content": "The user is greeting me..."
    }
  }],
  "usage": {
    "prompt_tokens": 252,
    "completion_tokens": 10,
    "completion_tokens_details": {"reasoning_tokens": 9}
  }
}
```

## Model Parameters

| Model | Temperature | Top-P |
|---|---|---|
| mimo-v2.5-pro, mimo-v2.5 | 1.0 (range 0-1.5) | 0.95 (range 0.01-1.0) |
| mimo-v2-flash | 0.3 (range 0-1.5) | 0.95 (range 0.01-1.0) |

## Rate Limits

| Metric | Text Gen / TTS | ASR |
|---|---|---|
| RPM | 100 | 100 |
| TPM | 10M | 10K |

## Sources verified 2026-06-19
- https://platform.xiaomimimo.com/llms-full.txt
