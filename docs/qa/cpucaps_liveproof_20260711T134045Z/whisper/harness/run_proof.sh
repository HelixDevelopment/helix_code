#!/usr/bin/env bash
# Phase-3 CPU Whisper STT END-TO-END proof (§11.4.108 runtime signature).
#
# Boots the HelixLLM faster-whisper CPU service (built from
# submodules/helix_llm/container/Containerfile.whisper) THROUGH the
# containers submodule compose.Orchestrator (§11.4.76), rootless podman
# (§11.4.161), NO GPU. Synthesizes two KNOWN utterances with espeak-ng
# (deterministic, §11.4.107 unfakeable proof), POSTs each to
# /v1/audio/transcriptions, asserts the recovered transcript contains every
# expected key content word, runs the golden-good/golden-bad analyzer
# self-validation (§11.4.107(10)) against REAL silence + REAL white-noise
# transcriptions plus in-memory wrong-content/empty derivations, and the
# RED-first stub baseline (§11.4.115), then tears the container down
# (single-owner cleanup, §11.4.119) leaving helixllm-coder untouched.
#
# Reproducible: re-run against a clean host to regenerate every evidence file.
# All model/port/limit values are config-injected here (§CONST-045/046) — the
# compose file carries no literal.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"       # .../harness
EVID="$(cd "$HERE/.." && pwd)"              # .../phase3_whisper_stt_20260707
cd "$HERE"

# ---- config injection (no hardcoded host/port/model literal downstream) ----
export STT_HOST_PORT="${STT_HOST_PORT:-18437}"   # distinct from coder:18434, embeddings:18435, translation:18436
export WHISPER_MODEL="${WHISPER_MODEL:-base}"
export WHISPER_COMPUTE_TYPE="${WHISPER_COMPUTE_TYPE:-int8}"
export WHISPER_NO_SPEECH_THRESHOLD="${WHISPER_NO_SPEECH_THRESHOLD:-0.6}"
export STT_MEM_LIMIT="${STT_MEM_LIMIT:-4g}"
export STT_CPUS="${STT_CPUS:-4}"
PROJECT="phase3whisperstt_cpu"
COMPOSE="compose.phase3whisper.yml"
BASE="http://localhost:${STT_HOST_PORT}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-300}"

BIN="$HERE/phase3whisper.bin"

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

build_harness() {
  log "building harness (containers-submodule replace) ..."
  GOFLAGS=-mod=mod go build -o "$BIN" . || { echo "BUILD FAILED"; exit 3; }
}

CURRENT_CONTAINER="${PROJECT}_helixllm-stt_1"

teardown_project() {
  local out="${1:-/dev/stdout}"
  log "teardown project=$PROJECT (single-owner cleanup) ..."
  "$BIN" boot-down "$COMPOSE" "$PROJECT" 2>&1 | tee "$out"
}

poll_health() {
  local deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  local n=0
  while [ "$(date +%s)" -lt "$deadline" ]; do
    n=$((n+1))
    code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/health" 2>/dev/null || echo 000)
    if [ "$code" = "200" ]; then
      log "health OK after $n polls"
      return 0
    fi
    # Fast-fail: if the container has already EXITED (e.g. the image failed
    # to build/start), stop polling immediately instead of burning the whole
    # timeout. Only "exited" aborts — "created"/"missing" can appear
    # transiently while the pod/container is still starting.
    st=$(podman inspect "$CURRENT_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || echo starting)
    if [ "$st" = "exited" ]; then
      log "container state=exited before healthy — abort poll (n=$n)"
      return 1
    fi
    sleep 3
  done
  log "health poll TIMED OUT after ${HEALTH_TIMEOUT}s"
  return 1
}

# ---------- BUILD + PRE-CLEAN (clean-target integrity §11.4.108/§11.4.139) ----------
build_harness
"$BIN" boot-down "$COMPOSE" "$PROJECT" >/dev/null 2>&1 || true
# Persistent external HF-model cache (survives teardown; external volumes are
# exempt from `compose down -v`). First cold run downloads weights into it;
# subsequent runs load from cache (no network).
podman volume create helixllm-whisper-cache >/dev/null 2>&1 || true

# ---------- PRE-FLIGHT ----------
{
  echo "### pre-flight (§11.4.119 single-owner, coder + siblings untouched)"
  echo "date_utc=$(date -u +%FT%TZ)"
  echo "uname_m=$(uname -m)  podman=$(podman --version)  espeak-ng=$(espeak-ng --version 2>&1 | head -1)  ffmpeg=$(ffmpeg -hide_banner -version 2>&1 | head -1)"
  echo "coder container (MUST remain running, untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Image}} {{.Status}}'
  echo "sibling capability containers (embeddings/translation — MUST NOT be touched, expect none currently running):"
  podman ps -a --format '{{.Names}}' | grep -E 'phase3embed|phase3translate' || echo "  (none running)"
  echo "target host port ${STT_HOST_PORT} listeners (expect none):"
  ss -ltn 2>/dev/null | grep ":${STT_HOST_PORT} " || echo "  (free)"
  echo "helixllm-stt image (expect absent on first run — built fresh via --build):"
  podman images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep helixllm-stt || echo "  (image absent — will be built)"
} | tee "$EVID/00_preflight.txt"

# ---------- SYNTHESIZE KNOWN AUDIO (§11.4.107 unfakeable proof) ----------
FOX_WAV="$HERE/fox.wav"
HW_WAV="$HERE/helloworld.wav"
SIL_WAV="$HERE/silence.wav"
NOI_WAV="$HERE/noise.wav"
{
  echo "### synthesize deterministic KNOWN audio (espeak-ng TTS + ffmpeg silence/noise)"
  "$BIN" synth 0 "$FOX_WAV"
  "$BIN" synth 1 "$HW_WAV"
  "$BIN" synth-silence 3 "$SIL_WAV"
  "$BIN" synth-noise 3 "$NOI_WAV"
  ls -la "$FOX_WAV" "$HW_WAV" "$SIL_WAV" "$NOI_WAV"
} | tee "$EVID/01_synth.txt"

# ---------- RED-FIRST BASELINE (§11.4.115) ----------
# A stub that returns "" (the shape a broken/stubbed provider would emit).
# The analyzer MUST FAIL it (defect reproduced) using the IDENTICAL `analyze`
# subcommand the GREEN lane below uses — no separate red-only code path.
python3 - "$EVID/red_stub_empty.json" <<'PY'
import json,sys
json.dump({"text": "", "raw_text": "", "language": "en", "language_probability": 0.0,
  "duration": 0.0, "segments": [], "max_no_speech_prob": 0.0,
  "silence_guard": {"triggered": True, "threshold": 0.6, "reason": "stub"}},
  open(sys.argv[1], "w"))
PY
python3 - "$EVID/red_stub_wrong.json" <<'PY'
import json,sys
json.dump({"text": "completely unrelated filler content about nothing relevant",
  "raw_text": "completely unrelated filler content about nothing relevant",
  "language": "en", "language_probability": 0.9, "duration": 1.0, "segments": [],
  "max_no_speech_prob": 0.05, "silence_guard": {"triggered": False, "threshold": 0.6, "reason": "stub"}},
  open(sys.argv[1], "w"))
PY
{
  echo "### RED baseline (§11.4.115): analyzer vs canned stub responses (fixture 0 = fox)"
  echo "RED_MODE=1 -- expect the analyzer to FAIL (exit 1) = defect reproduced"
  echo "-- stub: empty text --"
  "$BIN" analyze "$EVID/red_stub_empty.json" 0
  rc1=$?
  echo "analyzer_exit=$rc1"
  echo "-- stub: canned wrong text --"
  "$BIN" analyze "$EVID/red_stub_wrong.json" 0
  rc2=$?
  echo "analyzer_exit=$rc2"
  if [ "$rc1" -ne 0 ] && [ "$rc2" -ne 0 ]; then
    echo "RED-OK: both stubs correctly FAILED the runtime signature"
  else
    echo "RED-VIOLATION: a stub PASSED — analyzer is a bluff gate"
  fi
} 2>&1 | tee "$EVID/10_red_baseline.txt"

# ---------- BOOT + GREEN PROOF ----------
log "boot helixllm-stt model=$WHISPER_MODEL device=cpu compute=$WHISPER_COMPUTE_TYPE port=$STT_HOST_PORT project=$PROJECT via containers submodule orchestrator (--build)"
"$BIN" boot-up "$COMPOSE" "$PROJECT" 2>&1 | tee "$EVID/20_boot.txt"

BOOT_OK=1
if ! poll_health 2>&1 | tee "$EVID/21_health.txt"; then
  BOOT_OK=0
  log "helixllm-stt did not become healthy; capturing logs"
  podman logs "$CURRENT_CONTAINER" 2>&1 | tail -80 | tee "$EVID/22_stt_logs.txt" || true
fi

if [ "$BOOT_OK" -ne 1 ]; then
  echo "BLOCKED: helixllm-stt lane did not become healthy — see 22_stt_logs.txt" | tee "$EVID/90_blocked.txt"
  teardown_project "$EVID/29_teardown.txt"
  exit 4
fi

log "GREEN lane healthy: project=$PROJECT"

# container state evidence (§11.4.69 artifact)
podman ps --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}' \
  | tee "$EVID/24_container_state.txt"

# Fox transcribed TWICE (determinism §11.4.50), helloworld/silence/noise once each.
"$BIN" transcribe "$BASE" "$FOX_WAV" "$EVID/green_fox_1.json" 2>&1 | tee "$EVID/30_transcribe.txt"
"$BIN" transcribe "$BASE" "$FOX_WAV" "$EVID/green_fox_2.json" 2>&1 | tee -a "$EVID/30_transcribe.txt"
"$BIN" transcribe "$BASE" "$HW_WAV" "$EVID/green_helloworld_1.json" 2>&1 | tee -a "$EVID/30_transcribe.txt"
"$BIN" transcribe "$BASE" "$SIL_WAV" "$EVID/silence_response.json" 2>&1 | tee -a "$EVID/30_transcribe.txt"
"$BIN" transcribe "$BASE" "$NOI_WAV" "$EVID/noise_response.json" 2>&1 | tee -a "$EVID/30_transcribe.txt"

# GREEN runtime signature per utterance.
{
  echo "### GREEN runtime signature (§11.4.108) fixture=fox"
  "$BIN" analyze "$EVID/green_fox_1.json" 0; sig_fox=$?
  echo "signature_exit=$sig_fox"
  [ "$sig_fox" -eq 0 ] && echo "GREEN-OK(fox)" || echo "GREEN-FAIL(fox)"
} 2>&1 | tee "$EVID/11_green_proof_fox.txt"

{
  echo "### GREEN runtime signature (§11.4.108) fixture=helloworld"
  "$BIN" analyze "$EVID/green_helloworld_1.json" 1; sig_hw=$?
  echo "signature_exit=$sig_hw"
  [ "$sig_hw" -eq 0 ] && echo "GREEN-OK(helloworld)" || echo "GREEN-FAIL(helloworld)"
} 2>&1 | tee "$EVID/11_green_proof_helloworld.txt"

# Determinism (§11.4.50): fox transcribed twice must normalize identically.
{
  echo "### determinism (fox x2)"
  "$BIN" determinism "$EVID/green_fox_1.json" "$EVID/green_fox_2.json"; det=$?
  echo "determinism_exit=$det"
} 2>&1 | tee "$EVID/13_determinism.txt"

# Analyzer self-validation (§11.4.107(10)) using the REAL captured responses.
{
  echo "### analyzer self-validation (§11.4.107(10)): 2 real golden-good + real silence + real noise + in-memory wrong-content/empty"
  "$BIN" selfvalidate "$EVID/green_fox_1.json" "$EVID/green_helloworld_1.json" "$EVID/silence_response.json" "$EVID/noise_response.json"; sv=$?
  echo "selfvalidate_exit=$sv"
} 2>&1 | tee "$EVID/12_self_validation.txt"

# ---------- TEARDOWN + coder/siblings-untouched proof ----------
teardown_project "$EVID/29_teardown.txt"
{
  echo "### post-teardown state"
  echo "helixllm-stt containers (expect none):"
  podman ps -a --format '{{.Names}}' | grep "${PROJECT}" || echo "  (none — removed)"
  echo "coder still running (untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Status}}'
} | tee "$EVID/29b_post_teardown.txt"

log "DONE. Evidence in $EVID"
