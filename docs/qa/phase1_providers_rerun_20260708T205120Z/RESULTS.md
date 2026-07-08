# Phase 1 Provider Re-Run Results

**Run ID**: `20260708T205120Z`
**Date**: 2026-07-08 20:51:45 UTC

## Summary

| Provider | Nonce Echo | Models | Streaming | Overall |
|----------|-----------|--------|-----------|---------|
| ✅ **groq** | PASS | 17 models | PASS | PASS |
| ✅ **mistral** | PASS | 70 models | PASS | PASS |
| ✅ **codestral** | PASS | N/A (no endpoint) | PASS | PASS |
| ✅ **cohere** | PASS | 429 rate-limited | PASS | PASS |
| ✅ **cerebras** | PASS | 3 models | PASS | PASS |

## Overall

- **PASS**: 5
- **FAIL**: 0
- **SKIP**: 0

## Notes

- **Groq, Mistral**: Also proven by the Go `TestProviderLiveProof` harness (`providerlive` build tag). Results here are consistent.
- **Codestral**: Uses the dedicated `https://codestral.mistral.ai/v1` endpoint with the `CODESTRAL_API_KEY` environment variable.
- **Cohere**: Uses the Cohere Chat v2 API (`api.cohere.com/v2/chat`) with the `COHERE_API_KEY` environment variable. The `/v2/models` endpoint returned HTTP 429 (rate-limited) — model count not enumerated in this run.
- **Cerebras**: OpenAI-compatible endpoint at `api.cerebras.ai/v1` with the `CEREBRAS_API_KEY` environment variable. Models: `gemma-4-31b`, `gpt-oss-120b`, `zai-glm-4.7`.
- Model-listing endpoints: Groq/Mistral/Cerebras expose `/models` under their base URL; Codestral's `codestral.mistral.ai` does not expose a `/models` endpoint; Cohere has a separate `/v2/models` endpoint (rate-limited this run).

## Captured Evidence

Each provider directory contains:
- `verdict.txt` — nonce-echo test outcome
- `nonce_request.json` / `nonce_response.json` — request/response transcripts
- `models_list.json` — model catalogue output
- `stream_output.txt` — streaming test output
