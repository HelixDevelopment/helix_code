# Xiaomi MiMo Provider — Status Summary

**Status**: ✅ Implemented
**Date**: 2026-06-19
**Models**: 10 (text gen, ASR, TTS)
**Tests**: All passing (unit, integration, stress, chaos)
**Evidence**: Live API verification captured

## Sources verified 2026-06-22: helix_code/internal/llm/xiaomi_provider.go , helix_code/internal/llm/XIAOMI_PROVIDER.md , https://github.com/XiaomiMiMo/MiMo

Cross-referenced this summary against the in-repo provider source and the
external Xiaomi MiMo upstream (fetched/read 2026-06-22). Findings:
- **"10 models" confirmed from repo source.** `helix_code/internal/llm/xiaomi_provider.go`
  defines exactly 10 model entries — `mimo-v2.5-pro`, `mimo-v2.5`, `mimo-v2-pro`,
  `mimo-v2-omni`, `mimo-v2-flash`, `mimo-v2.5-asr`, `mimo-v2.5-tts`,
  `mimo-v2.5-tts-voiceclone`, `mimo-v2.5-tts-voicedesign`, `mimo-v2-tts` — with
  `TranscribeAudio()` (ASR) + `SynthesizeSpeech()` (TTS) endpoints against base
  URL `https://api.xiaomimimo.com/v1`. The "10 (text gen, ASR, TTS)" claim is
  accurate against the implementation (the implementation IS the source of truth
  for this provider per CONST-036/037 + §11.4.35).
- **Negative finding (external corroboration gap — important).** The public
  upstream `github.com/XiaomiMiMo/MiMo` documents the open-source **MiMo-7B**
  family (Base / RL-Zero / SFT / RL / RL-0530), explicitly math/code-reasoning
  and "NOT multimodal, ASR, or TTS." It does NOT corroborate the commercial
  **MiMo-V2.5** API lineup (omni-modal / ASR / TTS) this provider targets. The
  authoritative source for the V2.5 commercial endpoint is the key-gated
  `https://api.xiaomimimo.com/v1` service (a public GET of its root returns 404
  without an API key/path — expected), NOT the open-source MiMo-7B repo. Anyone
  validating these model IDs must do so against a live keyed call to the
  commercial endpoint, not the public GitHub repo.
- **External model-family is genuinely third-party.** Anthropic's models
  overview (platform.claude.com) lists no Xiaomi/MiMo models — confirming MiMo
  is an external provider integrated via HelixCode's OpenAI-compatible surface,
  not an Anthropic offering.
