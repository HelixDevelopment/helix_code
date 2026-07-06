# Phase 3 (STT) — Local faster-whisper in rootless Podman + golden self-validation

**Date:** 2026-07-06 · **Host:** rootless podman 5.7.1 (uid 1000), CPU-only
**Grounding:** `docs/research/07.2026/07_stt_ocr_whisper_tesseract/…` (faster-whisper = recommended default)

Real speech-to-text of real audio, self-validated so the analyzer provably cannot
bluff (§11.4.6 / §11.4.107(10)). No metadata-only claims — every number below comes
from a captured run against the running container.

## What is deployed

- **Engine:** `faster-whisper==1.1.1` (CTranslate2 `4.8.1` backend), model `base`,
  `device=cpu`, `compute_type=int8`. GPU deliberately NOT used (reserved for the
  main-stream fleet-model proof).
- **Runtime:** rootless Podman (`Host.Security.Rootless=true`, uid 1000). Image
  `localhost/helix-stt:cpu` built from `Containerfile.whisper` (python:3.11-slim +
  ffmpeg + faster-whisper + FastAPI). §11.4.161 / §11.4.76.
- **API:** `POST /v1/audio/transcriptions` (OpenAI-compatible multipart) →
  `{text, raw_text, language, language_probability, duration, segments[]{start,end,
  text,avg_logprob,no_speech_prob,compression_ratio}, max_no_speech_prob,
  silence_guard{}}`. Plus `GET /health`.
- **Model weights:** downloaded at first run into the **gitignored** `./models`
  volume; re-obtain via `scripts/fetch_whisper_models.sh` (§11.4.77).

## Run

```bash
podman build -f Containerfile.whisper -t localhost/helix-stt:cpu .
podman run -d --name helix-stt -p 127.0.0.1:8123:8000 \
  -v "$PWD/models:/models:Z" -e WHISPER_MODEL=base \
  -e WHISPER_DEVICE=cpu -e WHISPER_COMPUTE_TYPE=int8 localhost/helix-stt:cpu
./validate.sh          # golden self-validation, 3 deterministic runs
```

## Golden self-validation (the anti-bluff core)

Fixtures generated locally & deterministically (`fixtures/`, 16 kHz mono s16 wav):
- `golden_good.wav` — `espeak-ng` TTS of **"The helix code test number forty two"**.
- `golden_bad_silence.wav` — 3 s digital silence.
- `golden_bad_noise.wav` — 3 s low-level white noise.

**golden-good (REAL transcript):** `"The helix code test number 42"`
(Whisper normalizes "forty two"→"42" — allowed), `language=en` (prob 0.8383),
`max_no_speech_prob=0.0337`. Evidence: `evidence/golden_good.json`.

**golden-bad:** both silence and noise return `text=""` — the known text is never
produced, so a "expected words present" gate cannot pass on nothing.
Evidence: `evidence/golden_bad_silence.json`, `evidence/golden_bad_noise.json`.

**Guard proof (VAD off, raw decoder) — `evidence/guard_demonstration.txt`:**
| clip | raw decoder emitted | no_speech_prob | guard verdict |
|---|---|---|---|
| golden_good | "The helix code test number 42" | **0.0337** | KEPT (real speech) |
| silence | " You" (hallucination) | **0.7441** | **NULLED** (≥ 0.6) |
| noise | (no segments; lang mis-detected `nn`) | — | empty |

This is the real Whisper silence-hallucination failure mode reproduced (silence →
" You") and caught by the guard.

**3/3 deterministic:** `evidence/validate_3runs.txt` — PASS=9 FAIL=0, 1 distinct
golden-good transcript across 3 runs.

## Calibrated threshold (§11.4.6 — measured, not guessed)

`WHISPER_NO_SPEECH_THRESHOLD = 0.6`. Measured separation on our own fixtures:
- real speech (golden_good): `no_speech_prob = 0.0337`
- hallucinated silence (VAD off): `no_speech_prob = 0.7441`

0.6 sits cleanly between the two (0.0337 ≪ 0.6 ≪ 0.7441). Two guards compose:
(1) Silero **VAD filter** on by default drops non-speech before the decoder can
invent text (why the HTTP golden-bad runs return empty with 0 segments); (2) the
**no_speech_prob ≥ 0.6** floor nulls any segment that survives VAD, so even with
VAD off the hallucination is rejected. Wrong-language is surfaced via
`language`/`language_probability` (noise → `nn`), lettings callers reject
off-language transcripts.

> Re-calibrate on the target corpus before wiring into a shipped HelixQA gate — this
> threshold is calibrated on TTS + synthetic silence/noise, not field audio.

## Honest deviations / UNCONFIRMED

- **TTS = espeak-ng 1.52.0** (piper not installed). Deterministic, so preferred per
  task. Robotic timbre; `language_probability` 0.8383 (not ~0.99) reflects synthetic
  speech, not a model fault — the transcript is exact.
- **CPU + `base` model** per task. Larger models / GPU would raise accuracy &
  language-prob but were out of scope here.
- **`model.bin` sha256** in `versions_and_provenance.txt` is a 4 K hf-xet pointer
  (weights live in the xet blob cache, ~142 MB total under `models/hf`); the real
  transcription is the proof the weights load, not the pointer hash.
- **HTTP golden-bad** returns empty via VAD (0 segments) so `silence_guard.triggered`
  reads `false` there; the no_speech_prob guard's effect is proven separately with
  VAD off in `guard_demonstration.txt`.

## Files

```
Containerfile.whisper           # rootless CPU faster-whisper image
app/app.py                      # FastAPI OpenAI-compatible /v1/audio/transcriptions
app/guard_demo.py               # VAD-off hallucination + guard demonstration
validate.sh                     # golden-good/golden-bad, 3 deterministic runs
scripts/fetch_whisper_models.sh # §11.4.77 model re-obtain
fixtures/*.wav                  # golden-good TTS + golden-bad silence/noise
evidence/                       # captured JSON transcripts, guard demo, versions
models/                         # gitignored model volume
```
