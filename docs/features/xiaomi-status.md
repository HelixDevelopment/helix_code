# Xiaomi MiMo Provider — Feature Status

**Date**: 2026-06-19
**Status**: Implemented (→ Fixed.md)
**Revision**: 1

## Component Overview

| Component | Status | Evidence |
|---|---|---|
| Provider implementation | ✅ Implemented | `helix_code/internal/llm/xiaomi_provider.go` |
| ASR endpoint | ✅ Implemented | `TranscribeAudio()` method |
| TTS endpoint | ✅ Implemented | `SynthesizeSpeech()` method |
| Factory registration | ✅ Implemented | `case ProviderTypeXiaomi` in factory.go |
| Key recognition | ✅ Implemented | `XIAOMI_MIMO_API_KEY`, `ApiKey_Xiaomi_MiMo` |
| Hosted catalogue | ✅ Implemented | Entry in `openai_compatible_catalogue.go` |
| Verifier natively wired | ✅ Implemented | `dynamicNativelyWiredProviders["xiaomi"]` |
| reasoning_content | ✅ Implemented | Captured in `ProviderMetadata` |
| Dev config | ✅ Implemented | `config/dev/config.yaml` entry |
| Unit tests | ✅ Passing | 8 test files |
| Integration tests | ✅ Passing | 4/4 with live API evidence |
| Stress tests | ✅ Passing | 9/9 with rate limit evidence |
| Chaos tests | ✅ Passing | 6/6 |
| Documentation | ✅ Complete | Provider README + API reference |

## Models

| Model | Context | Max Output | Capabilities |
|---|---|---|---|
| mimo-v2.5-pro | 1M | 128K | Text gen, deep thinking, tool calling, web search |
| mimo-v2.5 | 1M | 128K | Omni-modal (text/image/video/audio) |
| mimo-v2-flash | 256K | 64K | Fast text gen, reasoning |
| mimo-v2.5-asr | 8K | 2K | Speech recognition |
| mimo-v2.5-tts | 8K | 8K | Speech synthesis |
| mimo-v2.5-tts-voiceclone | 8K | 8K | Voice cloning |
| mimo-v2.5-tts-voicedesign | 8K | 8K | Voice design |

## Evidence

- Live API verification: HTTP 200 on `/v1/models` and `/v1/chat/completions`
- Chat completion: 260 prompt + 20 completion tokens via mimo-v2-flash
- Streaming: 18 chunks received
- Tool calling: 1 tool call `get_weather(location:Tokyo)`
- Stress: 8/20 sequential successes (rate limit at iteration 8)
- Chaos: 401 invalid key, 400 invalid model, context cancellation handled

## Recordings

| Recording | Content |
|---|---|
| helixcode-tui-xiaomi-verify-20260619-234112.cast | API verification |
| helixcode-tui-xiaomi-tests-20260619-234151.cast | Test suite execution |

## Sources verified 2026-06-19
- https://platform.xiaomimimo.com/llms-full.txt
- https://github.com/XiaomiMiMo
- Live API: https://api.xiaomimimo.com/v1
