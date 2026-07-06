#!/usr/bin/env bash
# §11.4.77 re-obtain mechanism for the gitignored faster-whisper model volume.
# Runs the container once so faster-whisper downloads the pinned model into ./models.
set -euo pipefail
DIR="$(cd "$(dirname "$0")/.." && pwd)"
MODEL="${WHISPER_MODEL:-base}"
mkdir -p "$DIR/models/hf"
podman run --rm -v "$DIR/models:/models:Z" -e HF_HOME=/models/hf \
  -e WHISPER_MODEL="$MODEL" localhost/helix-stt:cpu \
  python -c "from faster_whisper import WhisperModel; WhisperModel('$MODEL', device='cpu', compute_type='int8'); print('model $MODEL cached')"
