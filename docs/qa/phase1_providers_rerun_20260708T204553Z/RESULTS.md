# Phase 1 Provider Re-Run Results

**Run ID**: `20260708T204553Z`
**Date**: 2026-07-08 20:46:25 UTC

## Summary

| Provider | Nonce Echo | Models | Streaming | Overall |
|----------|-----------|--------|-----------|---------|
| ✅ **groq** | PASS | models_count: 17 | PASS | PASS |
| ✅ **mistral** | PASS | models_count: 70 | PASS | PASS |
| ✅ **codestral** | PASS | models_status: HTTP 404 | PASS | PASS |
| ✅ **cohere** | PASS | models_status: HTTP 429 | PASS | PASS |
| ❌ **cerebras** | FAIL | models_count: 3 | FAIL | FAIL |

## Overall

- **PASS**: 4
- **FAIL**: 1
- **SKIP**: 0

## Notes

- **Groq, Mistral**: Also proven by the Go `TestProviderLiveProof` harness (`providerlive` build tag). Results here are consistent.
- **Codestral**: Uses the dedicated `https://codestral.mistral.ai/v1` endpoint with the `CODESTRAL_API_KEY` environment variable.
- **Cohere**: Uses the Cohere Chat v2 API (`api.cohere.com/v2/chat`) with the `COHERE_API_KEY` environment variable.
- **Cerebras**: OpenAI-compatible endpoint at `api.cerebras.ai/v1` with the `CEREBRAS_API_KEY` environment variable.
- Model-listing endpoints vary: Groq/Mistral/Cerebras expose `/models` under their base URL; Codestral proxies through Mistral's model list; Cohere has a separate `/v2/models` endpoint.

## Captured Evidence

Each provider directory contains:
- `verdict.txt` — nonce-echo test outcome
- `nonce_request.json` / `nonce_response.json` — request/response transcripts
- `models_list.json` — model catalogue output
- `stream_output.txt` — streaming test output
