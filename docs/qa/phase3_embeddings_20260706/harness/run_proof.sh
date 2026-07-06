#!/usr/bin/env bash
# Phase-3 CPU embeddings END-TO-END proof (§11.4.108 runtime signature).
#
# Boots the HF Text Embeddings Inference (TEI) CPU container THROUGH the
# containers submodule compose.Orchestrator (§11.4.76), rootless podman
# (§11.4.161), NO GPU. POSTs a real sentence triple to /v1/embeddings, asserts
# the semantic-order cosine signature + non-zero-norm + dimension +
# determinism, runs the golden-good/golden-bad analyzer self-validation
# (§11.4.107(10)) and the RED-first zero-vector baseline (§11.4.115), then
# tears the container down (single-owner cleanup, §11.4.119) leaving the
# helixllm-coder container untouched.
#
# Reproducible: re-run against a clean host to regenerate every evidence file.
# All model/port/limit values are config-injected here (§CONST-045/046) — the
# compose file carries no literal.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"       # .../harness
EVID="$(cd "$HERE/.." && pwd)"              # .../phase3_embeddings_20260706
cd "$HERE"

# ---- config injection (no hardcoded host/port/model literal downstream) ----
export TEI_HOST_PORT="${TEI_HOST_PORT:-18435}"   # distinct from coder :18434
export TEI_MEM_LIMIT="${TEI_MEM_LIMIT:-8g}"
export TEI_CPUS="${TEI_CPUS:-8}"
PROJECT="phase3embed"
COMPOSE="compose.phase3embed.yml"
BASE="http://localhost:${TEI_HOST_PORT}"
MARGIN="${MARGIN:-0.15}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-300}"
# Model lane: primary = design default (nomic), fallback = bge-small.
PRIMARY_MODEL="nomic-ai/nomic-embed-text-v1.5"; PRIMARY_DIM=768
FALLBACK_MODEL="BAAI/bge-small-en-v1.5";        FALLBACK_DIM=384

BIN="$HERE/phase3embed.bin"

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

build_harness() {
  log "building harness (containers-submodule replace) ..."
  GOFLAGS=-mod=mod go build -o "$BIN" . || { echo "BUILD FAILED"; exit 3; }
}

# Each lane boots under its OWN compose project name so podman-compose
# re-renders the ${TEI_MODEL_ID} interpolation fresh per lane (a single reused
# project name caches the first lane's container spec — the defect that made a
# later lane silently serve the earlier lane's model).
CURRENT_CONTAINER=""   # "<project>_tei-embed_1" of the lane being polled
SERVED_PROJECT=""      # project of the lane that actually served

teardown_project() {
  local proj="$1" out="${2:-/dev/stdout}"
  log "teardown project=$proj (single-owner cleanup) ..."
  "$BIN" boot-down "$COMPOSE" "$proj" 2>&1 | tee "$out"
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
    # Fast-fail: if the container has already exited (e.g. a model TEI cannot
    # load), stop polling immediately instead of burning the whole timeout.
    st=$(podman inspect "$CURRENT_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || echo missing)
    if [ "$st" = "exited" ] || [ "$st" = "missing" ]; then
      log "container state=$st before healthy — abort poll (n=$n)"
      return 1
    fi
    sleep 3
  done
  log "health poll TIMED OUT after ${HEALTH_TIMEOUT}s"
  return 1
}

boot_model() {
  local model="$1" proj="$2"
  export TEI_MODEL_ID="$model"
  log "boot TEI lane model=$model port=$TEI_HOST_PORT project=$proj via containers submodule orchestrator"
  "$BIN" boot-up "$COMPOSE" "$proj"
}

# ---------- BUILD + PRE-CLEAN (clean-target integrity §11.4.108/§11.4.139) ----------
build_harness
# Guarantee a clean slate: remove any leftover tei-embed container/volume from a
# prior (possibly interrupted) run so the lane that boots is genuinely fresh.
for tag in primary fallback; do
  "$BIN" boot-down "$COMPOSE" "${PROJECT}_${tag}" >/dev/null 2>&1 || true
done

# ---------- PRE-FLIGHT ----------
{
  echo "### pre-flight (§11.4.119 single-owner, coder untouched)"
  echo "date_utc=$(date -u +%FT%TZ)"
  echo "uname_m=$(uname -m)  podman=$(podman --version)"
  echo "coder container (MUST remain running, untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Image}} {{.Status}}'
  echo "target host port ${TEI_HOST_PORT} listeners (expect none):"
  ss -ltn 2>/dev/null | grep ":${TEI_HOST_PORT} " || echo "  (free)"
  echo "TEI image:"
  podman images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep text-embeddings-inference || echo "  (image absent — pull required)"
} | tee "$EVID/00_preflight.txt"

# ---------- RED-FIRST BASELINE (§11.4.115) ----------
# The exact dim-1536 zero-vector gateway stub (openai.go:641-661) that this
# provider replaces. The analyzer MUST FAIL it (defect reproduced).
python3 - "$EVID/red_stub_response.json" <<'PY'
import json,sys
z=[0.0]*1536
json.dump({"object":"list","model":"stub-zero-vector-1536",
  "data":[{"object":"embedding","index":i,"embedding":z} for i in range(3)],
  "usage":{"prompt_tokens":0,"total_tokens":0}},open(sys.argv[1],"w"))
PY
{
  echo "### RED baseline (§11.4.115): analyzer vs the dim-1536 zero-vector stub"
  echo "RED_MODE=1 — expect the analyzer to FAIL (exit 1) = defect reproduced"
  "$BIN" cosine "$EVID/red_stub_response.json" 768 "$MARGIN"
  rc=$?
  echo "analyzer_exit=$rc"
  if [ "$rc" -ne 0 ]; then echo "RED-OK: stub correctly FAILED the runtime signature"
  else echo "RED-VIOLATION: stub PASSED — analyzer is a bluff gate"; fi
} 2>&1 | tee "$EVID/10_red_baseline.txt"

# ---------- BOOT + GREEN PROOF ----------
LANE=""
run_lane() {
  local model="$1" tag="$2"
  local proj="${PROJECT}_${tag}"
  CURRENT_CONTAINER="${proj}_tei-embed_1"
  boot_model "$model" "$proj" 2>&1 | tee "$EVID/20_boot_${tag}.txt"
  if poll_health 2>&1 | tee "$EVID/21_health_${tag}.txt"; then
    LANE="$model"; SERVED_PROJECT="$proj"; return 0
  fi
  log "lane $model did not become healthy; capturing logs + tearing down"
  podman logs "$CURRENT_CONTAINER" 2>&1 | tail -40 | tee "$EVID/22_teilogs_${tag}.txt" || true
  teardown_project "$proj" "$EVID/28_teardown_${tag}.txt"
  return 1
}

if run_lane "$PRIMARY_MODEL" "primary"; then
  :
elif run_lane "$FALLBACK_MODEL" "fallback"; then
  echo "SUBSTITUTION (§11.4.6): primary lane $PRIMARY_MODEL did not become healthy (TEI cpu-1.9 rejects its config.json — 'duplicate field max_position_embeddings'); fell back to $FALLBACK_MODEL (a design-listed TEI CPU model)" | tee "$EVID/23_substitution.txt"
else
  echo "BLOCKED: neither TEI lane became healthy — see 22_teilogs_*.txt" | tee "$EVID/90_blocked.txt"
  exit 4
fi

log "GREEN lane=$LANE project=$SERVED_PROJECT"

# container state evidence (§11.4.69 artifact)
podman ps --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}' \
  | tee "$EVID/24_container_state.txt"

# Two identical requests (determinism §11.4.50).
"$BIN" embed "$BASE" "$EVID/green_response_1.json" 2>&1 | tee "$EVID/30_embed_1.txt"
"$BIN" embed "$BASE" "$EVID/green_response_2.json" 2>&1 | tee -a "$EVID/30_embed_1.txt"

# Served model + dim are read back from the REAL response (§11.4.6 — the served
# model reports its own name and dimension; we do not pin a guessed dim).
SERVED_MODEL=$(python3 -c "import json;print(json.load(open('$EVID/green_response_1.json')).get('model',''))")
SERVED_DIM=$(python3 -c "import json;print(len(json.load(open('$EVID/green_response_1.json'))['data'][0]['embedding']))")

# GREEN runtime signature. expDim=0 => the check does NOT pin a hardcoded
# dimension; it requires all vectors equal-dim, >0, non-zero-norm, and the
# semantic-order margin. The dimension is captured, not asserted against a guess.
{
  echo "### GREEN runtime signature (§11.4.108) served_model=$SERVED_MODEL served_dim=$SERVED_DIM margin=$MARGIN"
  "$BIN" cosine "$EVID/green_response_1.json" 0 "$MARGIN"; sig=$?
  echo "signature_exit=$sig"
  "$BIN" determinism "$EVID/green_response_1.json" "$EVID/green_response_2.json"; det=$?
  echo "determinism_exit=$det"
  [ "$sig" -eq 0 ] && [ "$det" -eq 0 ] && echo "GREEN-OK" || echo "GREEN-FAIL"
} 2>&1 | tee "$EVID/11_green_proof.txt"

# Analyzer self-validation (§11.4.107(10)) using the REAL captured golden-good.
# expDim for the golden-bad wrong-dim fixture is DERIVED from the good response.
{
  echo "### analyzer self-validation (§11.4.107(10)) golden-good = real captured response (dim=$SERVED_DIM)"
  "$BIN" selfvalidate "$EVID/green_response_1.json" "$MARGIN"; sv=$?
  echo "selfvalidate_exit=$sv"
} 2>&1 | tee "$EVID/12_self_validation.txt"

# ---------- TEARDOWN + coder-untouched proof ----------
teardown_project "$SERVED_PROJECT" "$EVID/29_teardown.txt"
{
  echo "### post-teardown state"
  echo "tei-embed containers (expect none):"
  podman ps -a --format '{{.Names}}' | grep "${PROJECT}_" || echo "  (none — removed)"
  echo "coder still running (untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Status}}'
} | tee "$EVID/29b_post_teardown.txt"

log "DONE. Evidence in $EVID"
