#!/usr/bin/env bash
# Phase-3 CPU translation NLLB-200-CTranslate2 PRIMARY-lane END-TO-END proof
# (§11.4.108 runtime signature). EXTENDS the already-shipped LibreTranslate
# FALLBACK-lane proof (docs/qa/phase3_translation_20260707/).
#
# Builds the shim image (python:3.11-slim + ctranslate2 + transformers +
# sentencepiece; harness/shim/{Dockerfile,server.py}), boots it THROUGH the
# containers submodule compose.Orchestrator (§11.4.76), rootless podman
# (§11.4.161), NO GPU. POSTs two known sentences to /translate and asserts the
# task's UNFAKEABLE keyword-substring + not-identity runtime signature,
# determinism (§11.4.50), the golden-good/golden-bad analyzer self-validation
# (§11.4.107(10)), and the RED-first echo-stub baseline (§11.4.115). Tears the
# container down (single-owner cleanup, §11.4.119) leaving helixllm-coder
# untouched.
#
# Reproducible: re-run against a clean host to regenerate every evidence file.
# All model/port/limit values are config-injected here (§CONST-045/046) — the
# compose file carries no literal.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"       # .../harness
EVID="$(cd "$HERE/.." && pwd)"              # .../phase3_translation_nllb_20260707
cd "$HERE"

# ---- config injection (no hardcoded host/port/model literal downstream) ----
export NLLB_SHIM_IMAGE="${NLLB_SHIM_IMAGE:-localhost/helixllm-nllb-shim:latest}"
export NLLB_HOST_PORT="${NLLB_HOST_PORT:-18436}"     # distinct from coder :18434, embeddings :18435
export NLLB_MEM_LIMIT="${NLLB_MEM_LIMIT:-8g}"
export NLLB_CPUS="${NLLB_CPUS:-8}"
export NLLB_INTER_THREADS="${NLLB_INTER_THREADS:-1}"
export NLLB_INTRA_THREADS="${NLLB_INTRA_THREADS:-4}"
export NLLB_BLAS_THREADS="${NLLB_BLAS_THREADS:-4}"    # explicit BLAS cap (root-cause fix, see shim/server.py)
export NLLB_BEAM_SIZE="${NLLB_BEAM_SIZE:-1}"          # greedy => deterministic + cheap on CPU
PROJECT="phase3translatenllb"
COMPOSE="compose.phase3translatenllb.yml"
BASE="http://localhost:${NLLB_HOST_PORT}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-1800}"              # cold boot downloads a ~2.5GB model

# Model lane candidates: PRIMARY (design default NLLB-200-distilled-600M via
# CT2, verified full file listing incl. bundled tokenizer — §11.4.150) then a
# documented fallback (an alternative CT2 conversion of the SAME NLLB-200-
# distilled-600M model) if the primary fails to load (§11.4.6 honest
# substitution — never fake a PASS).
PRIMARY_REPO="entai2965/nllb-200-distilled-600M-ctranslate2"
FALLBACK_REPO="JustFrederik/nllb-200-distilled-600M-ct2-int8"

BIN="$HERE/phase3translatenllb.bin"
CURRENT_CONTAINER=""
SERVED_PROJECT=""
SERVED_REPO=""

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

build_harness() {
  log "building harness (containers-submodule replace) ..."
  GOFLAGS=-mod=mod go build -o "$BIN" . || { echo "BUILD FAILED"; exit 3; }
}

build_shim_image() {
  log "building shim image $NLLB_SHIM_IMAGE ..."
  podman build -t "$NLLB_SHIM_IMAGE" "$HERE/shim" > "$EVID/01_image_build.txt" 2>&1 \
    && echo "BUILD-OK $NLLB_SHIM_IMAGE" | tee -a "$EVID/01_image_build.txt" \
    || { echo "BUILD-FAILED $NLLB_SHIM_IMAGE — see 01_image_build.txt"; tail -30 "$EVID/01_image_build.txt"; exit 4; }
}

teardown_project() {
  local proj="$1" out="${2:-/dev/stdout}"
  log "teardown project=$proj (single-owner cleanup) ..."
  "$BIN" boot-down "$COMPOSE" "$proj" 2>&1 | tee "$out"
}

# poll_health returns 0 once /health reports 200. It fast-fails (returns 1)
# immediately if the container has exited, OR /health reports a genuine load
# error (HTTP 500), OR the container is STUCK in "created" (never actually
# started — the rootlessport-fork-failure class observed on this shared host,
# §11.4.174: other concurrent tracks' work can transiently exhaust host
# RLIMIT_NPROC) for too many consecutive polls — never burning the whole
# timeout on a condition that will never resolve itself (§11.4.6). 503/000/
# connection-refused (still downloading/loading) and a FRESH "created" (still
# starting) keep polling.
poll_health() {
  local deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  local n=0 created_streak=0
  while [ "$(date +%s)" -lt "$deadline" ]; do
    n=$((n+1))
    resp=$(curl -s -w '\n%{http_code}' "$BASE/health" 2>/dev/null || echo -e "\n000")
    code=$(echo "$resp" | tail -1)
    bodyline=$(echo "$resp" | head -n -1)
    if [ "$code" = "200" ]; then
      log "health OK after $n polls: $bodyline"
      return 0
    fi
    if [ "$code" = "500" ]; then
      log "health reports LOAD ERROR after $n polls: $bodyline"
      return 1
    fi
    st=$(podman inspect "$CURRENT_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || echo starting)
    if [ "$st" = "exited" ]; then
      log "container state=exited before healthy — abort poll (n=$n)"
      return 1
    fi
    if [ "$st" = "created" ]; then
      created_streak=$((created_streak+1))
      if [ "$created_streak" -ge 10 ]; then
        log "container STUCK in state=created for $created_streak consecutive polls (host fork/resource pressure, §11.4.174) — abort poll, caller should retry (n=$n)"
        return 1
      fi
    else
      created_streak=0
    fi
    if [ $((n % 15)) -eq 0 ]; then
      log "still polling (n=$n, code=$code) ... container state=$st"
    fi
    sleep 4
  done
  log "health poll TIMED OUT after ${HEALTH_TIMEOUT}s"
  return 1
}

# ---------- BUILD + PRE-CLEAN (clean-target integrity §11.4.108/§11.4.139) ----------
build_harness
build_shim_image
for tag in primary fallback; do
  "$BIN" boot-down "$COMPOSE" "${PROJECT}_${tag}" >/dev/null 2>&1 || true
done
# Persistent external HF-model cache (§11.4.77). Created once; survives
# teardown (external volumes are exempt from `compose down -v`). First cold
# run downloads weights into it; subsequent runs load from cache.
podman volume create helixllm-nllb-cache >/dev/null 2>&1 || true

# ---------- PRE-FLIGHT ----------
{
  echo "### pre-flight (§11.4.119 single-owner, coder untouched)"
  echo "date_utc=$(date -u +%FT%TZ)"
  echo "uname_m=$(uname -m)  podman=$(podman --version)"
  echo "lane=NLLB-200-distilled-600M via CTranslate2 (PRIMARY design-default lane), CPU, shim=$NLLB_SHIM_IMAGE"
  echo "primary_repo=$PRIMARY_REPO  fallback_repo=$FALLBACK_REPO  host_port=$NLLB_HOST_PORT"
  echo "coder container (MUST remain running, untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Image}} {{.Status}}'
  echo "target host port ${NLLB_HOST_PORT} listeners (expect none):"
  ss -ltn 2>/dev/null | grep ":${NLLB_HOST_PORT} " || echo "  (free)"
} | tee "$EVID/00_preflight.txt"

# ---------- RED-FIRST BASELINE (§11.4.115) ----------
# The exact untranslated-passthrough "warming"/echo-stub bluff (the defect
# this provider must never exhibit): a stub server that just echoes q back as
# translatedText. "Pointing the harness at a stub" = feeding the analyzer the
# record such a stub would produce (same methodology as the already-shipped
# embeddings + LibreTranslate proofs: 10_red_baseline.txt in both).
python3 - "$EVID/red_echo_stub_en_de.json" <<'PY'
import json,sys
rec={"pair":"en->de","source":"The house is blue.","source_lang":"eng_Latn",
     "target":"deu_Latn","forward":"The house is blue."}  # echo stub: forward == source
json.dump(rec, open(sys.argv[1],"w"), indent=2)
PY
python3 - "$EVID/red_echo_stub_en_fr.json" <<'PY'
import json,sys
rec={"pair":"en->fr","source":"The cat sleeps.","source_lang":"eng_Latn",
     "target":"fra_Latn","forward":"The cat sleeps."}  # echo stub: forward == source
json.dump(rec, open(sys.argv[1],"w"), indent=2)
PY
{
  echo "### RED baseline (§11.4.115): analyzer vs an echo-stub that returns q unchanged"
  echo "RED_MODE=1 — expect the analyzer to FAIL (exit 1) = defect reproduced, for BOTH pairs"
  echo "--- en->de ---"
  "$BIN" analyze "$EVID/red_echo_stub_en_de.json"; rc1=$?
  echo "analyzer_exit(en->de)=$rc1"
  echo "--- en->fr ---"
  "$BIN" analyze "$EVID/red_echo_stub_en_fr.json"; rc2=$?
  echo "analyzer_exit(en->fr)=$rc2"
  if [ "$rc1" -ne 0 ] && [ "$rc2" -ne 0 ]; then
    echo "RED-OK: echo-stub correctly FAILED the runtime signature for both pairs"
  else
    echo "RED-VIOLATION: echo-stub PASSED at least one pair — analyzer is a bluff gate"
  fi
} 2>&1 | tee "$EVID/10_red_baseline.txt"

# ---------- BOOT + GREEN PROOF (lane retry, §11.4.6 honest substitution) ----------
# run_lane_once performs exactly one boot+health-poll attempt for a lane.
run_lane_once() {
  local repo="$1" tag="$2" attempt="$3"
  local proj="${PROJECT}_${tag}"
  CURRENT_CONTAINER="${proj}_nllb-shim_1"
  export NLLB_MODEL_REPO="$repo"
  log "boot NLLB shim lane repo=$repo port=$NLLB_HOST_PORT project=$proj attempt=$attempt via containers submodule orchestrator"
  "$BIN" boot-up "$COMPOSE" "$proj" 2>&1 | tee -a "$EVID/20_boot_${tag}.txt"
  if poll_health 2>&1 | tee -a "$EVID/21_health_${tag}.txt"; then
    # Served-model cross-check (§11.4.108/§11.4.139 clean-artifact integrity):
    # confirm the container actually reports serving THIS lane's repo, not a
    # stale-cache shadow of a different lane (the exact bug the repo-keyed
    # cache directory in shim/server.py fixes — verify it, don't just assume).
    reported=$(curl -s "$BASE/health" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("model",""))' 2>/dev/null || echo "")
    echo "served-model cross-check: requested=$repo reported=$reported" | tee "$EVID/21b_served_model_${tag}.txt"
    if [ "$reported" != "$repo" ]; then
      echo "CROSS-CHECK VIOLATION: reported model != requested repo — stale/mismatched artifact, not a genuine health" | tee -a "$EVID/21b_served_model_${tag}.txt"
      podman logs "$CURRENT_CONTAINER" 2>&1 | tail -60 | tee "$EVID/22_shimlogs_${tag}.txt" || true
      teardown_project "$proj" "$EVID/28_teardown_${tag}.txt"
      return 1
    fi
    SERVED_PROJECT="$proj"; SERVED_REPO="$repo"; return 0
  fi
  log "lane $repo did not become healthy; capturing logs + tearing down"
  podman logs "$CURRENT_CONTAINER" 2>&1 | tail -60 | tee "$EVID/22_shimlogs_${tag}.txt" || true
  curl -s "$BASE/health" | tee "$EVID/22_health_body_${tag}.json" >/dev/null 2>&1 || true
  teardown_project "$proj" "$EVID/28_teardown_${tag}.txt"
  return 1
}

# run_lane retries run_lane_once up to N times with backoff. §11.4.102 root
# cause established for THIS run (captured evidence, not a guess): the shared
# host is under transient process/thread pressure from concurrent parallel
# tracks (§11.4.174/§11.4.176 — verified via /proc/loadavg "3/5012" total
# threads + `ulimit -u`=4096 for this user, and other tracks' podman/HF/build
# activity observed concurrently), which has twice produced HOST-LEVEL
# transient faults distinct from a genuine model/CT2 incompatibility:
#   (a) OpenBLAS pthread_create failure during ctranslate2.Translator() init
#       (fixed at the source via OPENBLAS_NUM_THREADS et al., see shim/server.py)
#   (b) `rootlessport fork/exec: resource temporarily unavailable` at
#       `podman-compose up` time itself (a container-runtime-level fork, not
#       fixable from inside the container) — the container is left stuck in
#       state=created and never starts.
# A bounded retry-with-backoff is the honest, precedented response to a
# TRANSIENT host-load fault (mirrors the embeddings harness's
# run_lane_retry for transient HF-download flakiness) — it is NOT applied to
# mask a genuine, deterministic model/CT2 incompatibility, which would fail
# identically on every retry and correctly exhaust the retry budget.
run_lane() {
  local repo="$1" tag="$2" tries="${3:-4}" k
  for k in $(seq 1 "$tries"); do
    log "lane $tag attempt $k/$tries"
    if run_lane_once "$repo" "$tag" "$k"; then return 0; fi
    if [ "$k" -lt "$tries" ]; then
      log "lane $tag attempt $k/$tries failed — backing off before retry (transient host-load fault, §11.4.174)"
      sleep "$((10 * k))"
    fi
  done
  return 1
}

if run_lane "$PRIMARY_REPO" "primary"; then
  :
elif run_lane "$FALLBACK_REPO" "fallback"; then
  echo "SUBSTITUTION (§11.4.6): primary repo $PRIMARY_REPO did not become healthy (see 22_shimlogs_primary.txt / 22_health_body_primary.json); fell back to $FALLBACK_REPO (an alternative CT2 conversion of the SAME facebook/nllb-200-distilled-600M model)" | tee "$EVID/23_substitution.txt"
else
  echo "BLOCKED: neither NLLB-200-distilled-600M CT2 lane became healthy — see 22_shimlogs_*.txt / 22_health_body_*.json" | tee "$EVID/90_blocked.txt"
  exit 5
fi

log "GREEN lane repo=$SERVED_REPO project=$SERVED_PROJECT"

# container state evidence (§11.4.69 artifact)
podman ps --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}' \
  | tee "$EVID/24_container_state.txt"

# ---------- GREEN PROOF: per-pair probe (x2 for determinism) + analyze ----------
GREEN_ALL=1
# NOTE (§11.4.102 root cause, fixed here): unquoted "en->de"/"en->fr" contain a
# literal ">" which bash parses as a redirection operator ANYWHERE it appears
# in an unquoted word — an earlier revision's `PAIRS=(en->de en->fr)` was a
# silent bash syntax error that skipped the entire probe loop. Fixed by
# quoting every array element.
PAIRS=("en->de" "en->fr")
PAIRNAMES=("en_de" "en_fr")
for i in "${!PAIRS[@]}"; do
  pair="${PAIRS[$i]}"; tag="${PAIRNAMES[$i]}"
  "$BIN" probe "$BASE" "$pair" "$EVID/green_record_${tag}_1.json" 2>&1 | tee "$EVID/30_probe_${tag}.txt"
  "$BIN" probe "$BASE" "$pair" "$EVID/green_record_${tag}_2.json" 2>&1 | tee -a "$EVID/30_probe_${tag}.txt"
  # NOTE (§11.4.102 root cause, fixed here): `{ block; } | tee file` runs the
  # LEFT side of a pipe in a SUBSHELL in bash — `GREEN_ALL=0` assigned inside
  # such a block is LOST when read in the parent shell afterward (verified
  # empirically before this fix). Using plain output redirection (no pipe)
  # keeps the block in the current shell so the assignment is real; `cat` the
  # file afterward for the same console visibility `tee` gave.
  {
    echo "### GREEN runtime signature (§11.4.108) pair=$pair repo=$SERVED_REPO"
    "$BIN" analyze "$EVID/green_record_${tag}_1.json"; sig=$?
    echo "signature_exit=$sig"
    "$BIN" determinism "$EVID/green_record_${tag}_1.json" "$EVID/green_record_${tag}_2.json"; det=$?
    echo "determinism_exit=$det"
    if [ "$sig" -eq 0 ] && [ "$det" -eq 0 ]; then echo "GREEN-OK ($pair)"; else echo "GREEN-FAIL ($pair)"; GREEN_ALL=0; fi
  } > "$EVID/11_green_proof_${tag}.txt" 2>&1
  cat "$EVID/11_green_proof_${tag}.txt"
done

# Analyzer self-validation (§11.4.107(10)) using the REAL captured golden-good
# (en->de) + the REAL captured en->fr record as the wrong-language bad source.
{
  echo "### analyzer self-validation (§11.4.107(10)) golden-good = real captured en->de record"
  "$BIN" selfvalidate "$EVID/green_record_en_de_1.json" "$EVID/green_record_en_fr_1.json"; sv=$?
  echo "selfvalidate_exit=$sv"
  [ "$sv" -ne 0 ] && GREEN_ALL=0
} > "$EVID/12_self_validation.txt" 2>&1
cat "$EVID/12_self_validation.txt"

{
  echo "### overall verdict"
  if [ "$GREEN_ALL" -eq 1 ]; then echo "ALL-GREEN: runtime signature + determinism + self-validation PASS (lane=$SERVED_REPO)"
  else echo "NOT-ALL-GREEN: see per-pair proofs"; fi
} | tee "$EVID/13_verdict.txt"

# ---------- TEARDOWN + coder-untouched proof ----------
teardown_project "$SERVED_PROJECT" "$EVID/29_teardown.txt"
{
  echo "### post-teardown state"
  echo "nllb-shim containers (expect none):"
  podman ps -a --format '{{.Names}}' | grep "${PROJECT}_" || echo "  (none — removed)"
  echo "coder still running (untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Status}}'
} | tee "$EVID/29b_post_teardown.txt"

log "DONE. Evidence in $EVID"
[ "$GREEN_ALL" -eq 1 ] || exit 6
