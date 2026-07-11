#!/usr/bin/env bash
# Phase-3 CPU translation (NMT) END-TO-END proof (§11.4.108 runtime signature).
#
# Boots a stock LibreTranslate (Argos/OPUS-MT on CTranslate2) CPU container
# THROUGH the containers submodule compose.Orchestrator (§11.4.76), rootless
# podman (§11.4.161), NO GPU. Drives the LibreTranslate /translate + /detect
# routes for a known golden reference set and asserts the CX-05 anti-gaming
# TRIPLE (not-identity+detected-target, forward chrF-vs-golden, back-translation
# metamorphic) + determinism (§11.4.50), runs the golden-good/golden-bad
# analyzer self-validation (§11.4.107(10)) and the RED-first identity-passthrough
# baseline (§11.4.115), then tears the container down (single-owner cleanup,
# §11.4.119) leaving the helixllm-coder container untouched.
#
# Reproducible: re-run against a clean host to regenerate every evidence file.
# All model/port/limit/language values are config-injected here (§CONST-045/046)
# — the compose file carries no literal.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"       # .../harness
EVID="$(cd "$HERE/.." && pwd)"              # .../phase3_translation_20260707
cd "$HERE"

# ---- config injection (no hardcoded host/port/model/lang literal downstream) ----
# Fully-qualified image name: rootless podman enforces short-name resolution and
# cannot prompt for a registry without a TTY, so the docker.io/ prefix is required.
export LT_IMAGE="${LT_IMAGE:-docker.io/libretranslate/libretranslate:latest}"
export LT_HOST_PORT="${LT_HOST_PORT:-18436}"   # distinct from coder :18434, embeddings :18435
export LT_LOAD_ONLY="${LT_LOAD_ONLY:-en,fr,de}"
export LT_MEM_LIMIT="${LT_MEM_LIMIT:-6g}"
export LT_CPUS="${LT_CPUS:-8}"
PROJECT="phase3translate_lt"
COMPOSE="compose.phase3translate.yml"
BASE="http://localhost:${LT_HOST_PORT}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-900}"        # cold boot downloads Argos packages
# Anti-gaming thresholds — calibrated on THIS project's own fixtures
# (§11.4.107(13)); real output clears them with margin, every golden-bad fails.
CHRF_FLOOR="${CHRF_FLOOR:-0.30}"
BACK_MARGIN="${BACK_MARGIN:-0.40}"

BIN="$HERE/phase3translate.bin"
CURRENT_CONTAINER="${PROJECT}_libretranslate_1"

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

build_harness() {
  log "building harness (containers-submodule replace) ..."
  GOFLAGS=-mod=mod go build -o "$BIN" . || { echo "BUILD FAILED"; exit 3; }
}

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
    # LibreTranslate is ready once /languages returns 200 with a non-empty list
    # (models loaded before the server binds). §11.4.6 — proven by a real 200.
    code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/languages" 2>/dev/null || echo 000)
    if [ "$code" = "200" ]; then
      log "health OK (/languages 200) after $n polls"
      return 0
    fi
    # Fast-fail: if the container has already EXITED (bad arg, model failure),
    # stop polling immediately. Only "exited" aborts — "created"/"missing" are
    # transient while the pod/container is still starting, so they keep polling.
    st=$(podman inspect "$CURRENT_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || echo starting)
    if [ "$st" = "exited" ]; then
      log "container state=exited before healthy — abort poll (n=$n)"
      return 1
    fi
    sleep 4
  done
  log "health poll TIMED OUT after ${HEALTH_TIMEOUT}s"
  return 1
}

# ---------- BUILD + PRE-CLEAN (clean-target integrity §11.4.108/§11.4.139) ----------
build_harness
# Guarantee a clean slate: remove any leftover container/volume from a prior
# (possibly interrupted) run so the container that boots is genuinely fresh.
"$BIN" boot-down "$COMPOSE" "$PROJECT" >/dev/null 2>&1 || true
# Persistent external Argos-model cache (design §3.1). Created once; survives
# teardown (external volumes are exempt from `compose down -v`). First cold run
# downloads the en/fr/de packages into it; subsequent runs load from cache.
podman volume create helixllm-libretranslate-cache >/dev/null 2>&1 || true
# Pre-pull the image (fully-qualified) so `compose up` starts a container rather
# than racing a first-time registry pull under the no-TTY podman-compose shim.
log "pre-pulling $LT_IMAGE (cold pull may take a few minutes) ..."
podman pull "$LT_IMAGE" > "$EVID/01_image_pull.txt" 2>&1 \
  && echo "PULL-OK $LT_IMAGE" | tee -a "$EVID/01_image_pull.txt" \
  || { echo "PULL-FAILED $LT_IMAGE — see 01_image_pull.txt"; tail -5 "$EVID/01_image_pull.txt"; exit 4; }

# ---------- PRE-FLIGHT ----------
{
  echo "### pre-flight (§11.4.119 single-owner, coder untouched)"
  echo "date_utc=$(date -u +%FT%TZ)"
  echo "uname_m=$(uname -m)  podman=$(podman --version)"
  echo "lane=LibreTranslate (stock, Argos/OPUS-MT on CTranslate2, CPU) image=$LT_IMAGE"
  echo "load_only=$LT_LOAD_ONLY  host_port=$LT_HOST_PORT  chrf_floor=$CHRF_FLOOR  back_margin=$BACK_MARGIN"
  echo "coder container (MUST remain running, untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Image}} {{.Status}}'
  echo "target host port ${LT_HOST_PORT} listeners (expect none):"
  ss -ltn 2>/dev/null | grep ":${LT_HOST_PORT} " || echo "  (free)"
  echo "LibreTranslate image present?"
  podman images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep -i libretranslate || echo "  (image absent — pull required)"
} | tee "$EVID/00_preflight.txt"

# ---------- SUBSTITUTION NOTE (honest §11.4.6) ----------
{
  echo "### lane decision / substitution (§11.4.6)"
  echo "DESIGN PRIMARY  : NLLB-200-distilled-600M CT2 int8 behind a custom"
  echo "                  LibreTranslate-shaped /translate FastAPI shim (TRANSLATION_PROVIDER.md §1.2)."
  echo "LANE PROVEN HERE: stock LibreTranslate (Argos/OPUS-MT on CTranslate2) — the design's"
  echo "                  DOCUMENTED FALLBACK lane (TRANSLATION_PROVIDER.md §1.2/§3.1): identical"
  echo "                  LibreTranslate /translate contract, same CTranslate2 engine, but a single"
  echo "                  pre-built CPU image (no custom shim + no CT2 model-conversion step)."
  echo "REASON          : reliable zero-new-shim CPU boot for this proof. The NLLB-CT2 primary needs"
  echo "                  a bespoke shim image build (python+ctranslate2+sentencepiece+FastAPI+lang-detect)"
  echo "                  plus an offline HF->CT2 int8 conversion — deferred as its own work item."
  echo "HONEST          : this is a REAL NMT service (real Argos/CTranslate2 translation), the anti-gaming"
  echo "                  triple below proves genuine translation, NOT an identity/passthrough bluff."
} | tee "$EVID/23_substitution.txt"

# ---------- RED-FIRST BASELINE (§11.4.115) ----------
# The exact untranslated-passthrough "warming" bluff (design §2.5): the gateway
# echoes q unchanged. The analyzer MUST FAIL it (defect reproduced).
python3 - "$EVID/red_identity_record.json" <<'PY'
import json,sys
rec={
 "pair":"en->fr","source":"The book is on the table.","source_lang":"en",
 "target":"fr","golden":"Le livre est sur la table.",
 "forward":"The book is on the table.",   # identity passthrough == source
 "forward_detected":"en","forward_detected_confidence":99.0,
 "back":"The book is on the table."
}
json.dump(rec,open(sys.argv[1],"w"),indent=2)
PY
{
  echo "### RED baseline (§11.4.115): analyzer vs the identity-passthrough 'warming' bluff"
  echo "RED_MODE=1 — expect the analyzer to FAIL (exit 1) = defect reproduced"
  "$BIN" analyze "$EVID/red_identity_record.json" "$CHRF_FLOOR" "$BACK_MARGIN"
  rc=$?
  echo "analyzer_exit=$rc"
  if [ "$rc" -ne 0 ]; then echo "RED-OK: identity passthrough correctly FAILED the runtime signature"
  else echo "RED-VIOLATION: identity passthrough PASSED — analyzer is a bluff gate"; fi
} 2>&1 | tee "$EVID/10_red_baseline.txt"

# ---------- BOOT + HEALTH ----------
"$BIN" boot-up "$COMPOSE" "$PROJECT" 2>&1 | tee "$EVID/20_boot.txt"
if ! poll_health 2>&1 | tee "$EVID/21_health.txt"; then
  log "LibreTranslate did not become healthy; capturing logs + tearing down"
  podman logs "$CURRENT_CONTAINER" 2>&1 | tail -60 | tee "$EVID/22_ltlogs.txt" || true
  teardown_project "$PROJECT" "$EVID/28_teardown.txt"
  echo "BLOCKED: LibreTranslate lane did not become healthy — see 22_ltlogs.txt" | tee "$EVID/90_blocked.txt"
  exit 4
fi

# container + languages evidence (§11.4.69 artifact)
podman ps --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}' | tee "$EVID/24_container_state.txt"
curl -s "$BASE/languages" | tee "$EVID/25_languages.json" >/dev/null
echo "" >> "$EVID/25_languages.json"

# ---------- GREEN PROOF: anti-gaming triple per pair + determinism ----------
GREEN_ALL=1
PAIRS=(0 1)         # 0=en->fr, 1=en->de
PAIRNAMES=(en_fr en_de)
for i in "${PAIRS[@]}"; do
  tag="${PAIRNAMES[$i]}"
  # Two identical probes (determinism §11.4.50) — each drives real forward+detect+back.
  "$BIN" probe "$BASE" "$i" "$EVID/green_record_${tag}_1.json" 2>&1 | tee "$EVID/30_probe_${tag}.txt"
  "$BIN" probe "$BASE" "$i" "$EVID/green_record_${tag}_2.json" 2>&1 | tee -a "$EVID/30_probe_${tag}.txt"
  {
    echo "### GREEN runtime signature (§11.4.108) pair=$tag floor=$CHRF_FLOOR margin=$BACK_MARGIN"
    "$BIN" analyze "$EVID/green_record_${tag}_1.json" "$CHRF_FLOOR" "$BACK_MARGIN"; sig=$?
    echo "signature_exit=$sig"
    "$BIN" determinism "$EVID/green_record_${tag}_1.json" "$EVID/green_record_${tag}_2.json"; det=$?
    echo "determinism_exit=$det"
    if [ "$sig" -eq 0 ] && [ "$det" -eq 0 ]; then echo "GREEN-OK ($tag)"; else echo "GREEN-FAIL ($tag)"; GREEN_ALL=0; fi
  } 2>&1 | tee "$EVID/11_green_proof_${tag}.txt"
done

# Analyzer self-validation (§11.4.107(10)) using the REAL captured golden-good
# (the en->fr pair). golden-bad identity/wrong-lang/garbage/empty each MUST FAIL.
{
  echo "### analyzer self-validation (§11.4.107(10)) golden-good = real captured en->fr record"
  "$BIN" selfvalidate "$EVID/green_record_en_fr_1.json" "$CHRF_FLOOR" "$BACK_MARGIN"; sv=$?
  echo "selfvalidate_exit=$sv"
  [ "$sv" -ne 0 ] && GREEN_ALL=0
} 2>&1 | tee "$EVID/12_self_validation.txt"

{
  echo "### overall verdict"
  if [ "$GREEN_ALL" -eq 1 ]; then echo "ALL-GREEN: anti-gaming triple + determinism + self-validation PASS"
  else echo "NOT-ALL-GREEN: see per-pair proofs"; fi
} | tee "$EVID/13_verdict.txt"

# ---------- TEARDOWN + coder-untouched proof ----------
teardown_project "$PROJECT" "$EVID/29_teardown.txt"
{
  echo "### post-teardown state"
  echo "libretranslate containers (expect none):"
  podman ps -a --format '{{.Names}}' | grep "${PROJECT}" || echo "  (none — removed)"
  echo "coder still running (untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Status}}'
} | tee "$EVID/29b_post_teardown.txt"

log "DONE. Evidence in $EVID"
[ "$GREEN_ALL" -eq 1 ] || exit 5
