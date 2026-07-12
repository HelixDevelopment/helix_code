# Xiaomi MiMo Provider

## Overview

Xiaomi MiMo is a first-class LLM provider in HelixCode, supporting text generation,
speech recognition (ASR), speech synthesis (TTS), multimodal understanding, tool calling,
and web search.

## Configuration

### Environment Variables

| Variable | Description |
|---|---|
| `XIAOMI_MIMO_API_KEY` | Primary API key (format: `sk-xxxxx`) |
| `ApiKey_Xiaomi_MiMo` | Legacy alias (from `~/api_keys.sh`) |

### API

- **Base URL**: `https://api.xiaomimimo.com/v1`
- **Auth**: `Authorization: Bearer <key>` or `api-key: <key>`
- **Format**: OpenAI-compatible

## Models

| Model | Context | Max Output | Use Case |
|---|---|---|---|
| `mimo-v2.5-pro` | 1M | 128K | Flagship text gen, deep thinking |
| `mimo-v2.5` | 1M | 128K | Omni-modal (text/image/video/audio) |
| `mimo-v2-pro` | 1M | 128K | Text gen (deprecated 2026-06-30) |
| `mimo-v2-omni` | 256K | 128K | Multimodal (deprecated 2026-06-30) |
| `mimo-v2.5-pro` | 256K | 64K | Fast text gen |
| `mimo-v2.5-asr` | 8K | 2K | Speech recognition |
| `mimo-v2.5-tts` | 8K | 8K | Speech synthesis |
| `mimo-v2.5-tts-voiceclone` | 8K | 8K | Voice cloning |
| `mimo-v2.5-tts-voicedesign` | 8K | 8K | Voice design |
| `mimo-v2-tts` | 8K | 8K | TTS (deprecated 2026-06-30) |

## Capabilities

- Text generation
- Code generation & analysis
- Reasoning (deep thinking with `reasoning_content` field)
- Tool calling (function calling)
- Web search (built-in, billed separately)
- Multimodal understanding (image/video/audio)
- ASR (speech-to-text)
- TTS (text-to-speech with voice cloning/design)

## Usage

```go
config := llm.ProviderConfigEntry{
    Type:    llm.ProviderTypeXiaomi,
    APIKey:  os.Getenv("XIAOMI_MIMO_API_KEY"),
    Enabled: true,
}
provider, err := llm.NewProvider(config)
```

## Rate Limits

- RPM: 100
- TPM: 10M (text gen), 10K (ASR)

## Sources verified 2026-06-19
- https://platform.xiaomimimo.com/llms-full.txt
- https://github.com/XiaomiMiMo
- Live API verification: HTTP 200 on /v1/models and /v1/chat/completions
