#!/usr/bin/env bash
# Phase-3 CPU OCR (Tesseract) END-TO-END proof (§11.4.108 runtime signature).
#
# Boots the helix-ocr container (submodules/helix_llm/services/ocr/) THROUGH
# the containers submodule compose.Orchestrator (§11.4.76), rootless podman
# (§11.4.161), NO GPU. Renders KNOWN-text fixtures via the service's own
# /v1/render endpoint (so font/version drift can never fake a result), OCRs
# them via /v1/ocr, and asserts the normalized-token + mean-confidence
# runtime signature for TWO distinct strings, runs the golden-good/golden-bad
# analyzer self-validation (§11.4.107(10): blank, noise, wrong-content all
# MUST FAIL) and the RED-first stub baseline (§11.4.115), then tears the
# container down (single-owner cleanup, §11.4.119) leaving the
# helixllm-coder container untouched.
#
# Reproducible: re-run against a clean host to regenerate every evidence file.
# All host-port/resource values are config-injected here (§CONST-045/046) —
# the compose file carries no literal.
set -uo pipefail

# §11.4.102 root-caused resource-contention mitigation (self-scoped ONLY —
# never touches any other process, §11.4.174): the first attempt at this
# proof hit `pthread_create failed: Resource temporarily unavailable` /
# `fork/exec /usr/bin/crun: resource temporarily unavailable` during the
# container build — investigation showed the host's per-UID RLIMIT_NPROC
# soft limit (4096) was ~95% consumed (3904-3900+ live processes/threads for
# this uid, dominated by long-running unrelated MCP-server processes from
# other sessions). Raising THIS session's own soft limit to its hard cap
# gives this script's own process tree (podman-compose -> crun) headroom to
# fork without touching, inspecting the ownership of, or killing any other
# process.
ulimit -u "$(ulimit -Hu)" 2>/dev/null || true

HERE="$(cd "$(dirname "$0")" && pwd)"       # .../harness
EVID="$(cd "$HERE/.." && pwd)"              # .../phase3_tesseract_ocr_20260707
cd "$HERE"

# ---- config injection (no hardcoded host/port/limit literal downstream) ----
export OCR_HOST_PORT="${OCR_HOST_PORT:-18438}"   # distinct from coder:18434, embed:18435, xlate:18436, whisper:18437
export OCR_MEM_LIMIT="${OCR_MEM_LIMIT:-2g}"
export OCR_CPUS="${OCR_CPUS:-4}"
PROJECT="phase3ocr"
COMPOSE="compose.phase3ocr.yml"
BASE="http://localhost:${OCR_HOST_PORT}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-300}"
# Confidence floor: CALIBRATED from this project's OWN observed output
# (§11.4.6 / §11.4.107(13)) — never hardcoded from literature. First run
# observed: golden-good mean_conf = 95.99 / 94.77 (two distinct rendered
# strings); golden-bad-noise mean_conf = 11.97 (Tesseract hallucinating a few
# low-confidence glyphs from pure noise); golden-bad-blank mean_conf = 0
# (no words at all). 60 sits comfortably below the observed good cluster
# (~95-96) while decisively above the highest observed bad-fixture value
# (11.97) — the same separation the prior local proof
# (docs/qa/p3_tesseract_ocr/README.md) found with its own floor of 80 on the
# same class of rendered label text (94-96 good cluster).
CONF_FLOOR="${CONF_FLOOR:-60}"

BIN="$HERE/phase3ocr.bin"
OCR_SVC_DIR="$(cd "$HERE/../../../../../submodules/helix_llm/services/ocr" && pwd)"

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

build_harness() {
  log "building harness (containers-submodule replace) ..."
  GOFLAGS=-mod=mod go build -o "$BIN" . || { echo "BUILD FAILED"; exit 3; }
}

# §11.4.77 regeneration mechanism: the ocr-shim binary is compiled OUTSIDE the
# container (host-compiled, static) — see the Containerfile's honest-
# substitution comment for why (a golang-toolchain in-container build stage
# reproducibly hit host RLIMIT_NPROC contention, §11.4.174).
build_ocr_shim() {
  log "building ocr-shim (static linux/amd64) in $OCR_SVC_DIR ..."
  ( cd "$OCR_SVC_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ocr-shim . ) \
    || { echo "OCR-SHIM BUILD FAILED"; exit 3; }
}

teardown_project() {
  local proj="$1" out="${2:-/dev/stdout}"
  log "teardown project=$proj (single-owner cleanup) ..."
  "$BIN" boot-down "$COMPOSE" "$proj" 2>&1 | tee "$out"
}

CURRENT_CONTAINER=""

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
    st=$(podman inspect "$CURRENT_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || echo starting)
    if [ "$st" = "exited" ]; then
      log "container state=exited before healthy — abort poll (n=$n)"
      return 1
    fi
    sleep 2
  done
  log "health poll TIMED OUT after ${HEALTH_TIMEOUT}s"
  return 1
}

# ---------- BUILD + PRE-CLEAN (clean-target integrity §11.4.108/§11.4.139) ----------
build_harness
build_ocr_shim
"$BIN" boot-down "$COMPOSE" "$PROJECT" >/dev/null 2>&1 || true

# ---------- PRE-FLIGHT ----------
{
  echo "### pre-flight (§11.4.119 single-owner, coder + sibling capabilities untouched)"
  echo "date_utc=$(date -u +%FT%TZ)"
  echo "uname_m=$(uname -m)  podman=$(podman --version)"
  echo "coder container (MUST remain running, untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Image}} {{.Status}}'
  echo "sibling Phase-3 containers currently present (must remain untouched, informational):"
  podman ps -a --format '{{.Names}} {{.Status}}' | grep -E 'phase3(embed|translate|whisper|ocr)' || echo "  (none currently present)"
  echo "target host port ${OCR_HOST_PORT} listeners (expect none):"
  ss -ltn 2>/dev/null | grep ":${OCR_HOST_PORT} " || echo "  (free)"
} | tee "$EVID/00_preflight.txt"

# ---------- §11.4.150 deep multi-angle research summary (full citations in RESULTS.md) ----------
{
  echo "### deep multi-angle research (§11.4.150) — see RESULTS.md for full citations"
  echo "angle 1: Debian bookworm tesseract-ocr package version = 5.3.0-2 (packages.debian.org, verified 2026-07-07)"
  echo "angle 2: tessdoc Command-Line-Usage — TSV column layout + --oem/--psm syntax (tesseract-ocr.github.io, verified 2026-07-07)"
  echo "angle 3: tesseract TSV conf column semantics (0-100 word-level, -1 layout rows) — tomrochette.com / GitHub issue #2746 (verified 2026-07-07)"
  echo "angle 4: prior local empirical proof docs/qa/p3_tesseract_ocr/README.md — same-class rendered-label OCR observed conf 94-96 (2026-07-xx, this repo)"
} | tee "$EVID/01_research_summary.txt"

# ---------- RED-FIRST BASELINE (§11.4.115) ----------
# A stub response with full_text="" / mean_conf=0 — what a broken/absent OCR
# engine (or a dead stub) would return. The analyzer MUST FAIL it BEFORE the
# real lane is even booted (defect-shape reproduced).
EXP_TOKENS_1="HELIX,OCR,2026,QUICK,BROWN,FOX"
cat > "$EVID/red_stub_response.json" <<'JSON'
{"engine":"stub-empty","config":"","words":[],"full_text":"","mean_conf":0}
JSON
{
  "$BIN" analyze "$EVID/red_stub_response.json" "$EXP_TOKENS_1" "$CONF_FLOOR"
} 2>&1 | tee "$EVID/10_red_baseline.txt"
RED_RC=${PIPESTATUS[0]}
{
  echo "RED_MODE=1 — expect the analyzer to FAIL (exit != 0) = defect-shape reproduced"
  echo "analyzer_exit=$RED_RC"
  if [ "$RED_RC" -ne 0 ]; then echo "RED-OK: empty stub correctly FAILED the runtime signature"
  else echo "RED-VIOLATION: stub PASSED — analyzer is a bluff gate"; fi
} | tee -a "$EVID/10_red_baseline.txt"

# ---------- BOOT (build + up) via containers submodule orchestrator ----------
CURRENT_CONTAINER="${PROJECT}_ocr_1"
"$BIN" boot-up "$COMPOSE" "$PROJECT" 2>&1 | tee "$EVID/20_boot.txt"
BOOT_RC=${PIPESTATUS[0]}
if [ "$BOOT_RC" -ne 0 ]; then
  echo "BLOCKED: compose up (with --build) failed — see 20_boot.txt" | tee "$EVID/90_blocked.txt"
  exit 4
fi

if ! poll_health 2>&1 | tee "$EVID/21_health.txt"; then
  podman logs "$CURRENT_CONTAINER" 2>&1 | tail -60 | tee "$EVID/22_ocrlogs.txt" || true
  teardown_project "$PROJECT" "$EVID/28_teardown_failed.txt"
  echo "BLOCKED: helix-ocr never became healthy — see 21_health.txt / 22_ocrlogs.txt" | tee "$EVID/90_blocked.txt"
  exit 4
fi

log "GREEN: helix-ocr healthy on ${BASE}"
podman ps --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}' | tee "$EVID/24_container_state.txt"
curl -s "$BASE/health" | tee "$EVID/25_health_response.json"; echo

# ---------- FIXTURES (rendered SERVER-SIDE via /v1/render — §11.4.6 honest, non-fakeable) ----------
TEXT_1="HELIX OCR 2026 quick brown fox"
TEXT_2="PHASE THREE TESSERACT PROOF SEVEN"
TEXT_WRONG="BANANA SPACESHIP GALAXY EIGHT"
EXP_TOKENS_2="PHASE,THREE,TESSERACT,PROOF,SEVEN"

"$BIN" render "$BASE" "$EVID/fixture_good1.png" label "$TEXT_1" | tee "$EVID/30_render_good1.txt"
"$BIN" render "$BASE" "$EVID/fixture_good2.png" label "$TEXT_2" | tee "$EVID/30_render_good2.txt"
"$BIN" render "$BASE" "$EVID/fixture_blank.png" blank ""        | tee "$EVID/30_render_blank.txt"
"$BIN" render "$BASE" "$EVID/fixture_noise.png" noise ""        | tee "$EVID/30_render_noise.txt"
"$BIN" render "$BASE" "$EVID/fixture_wrong.png" label "$TEXT_WRONG" | tee "$EVID/30_render_wrong.txt"

# ---------- GREEN PROOF #1 (string 1) ----------
"$BIN" ocr "$BASE" "$EVID/fixture_good1.png" "$EVID/green_response_good1.json" | tee "$EVID/31_ocr_good1.txt"
"$BIN" ocr "$BASE" "$EVID/fixture_good1.png" "$EVID/green_response_good1_rep.json" | tee -a "$EVID/31_ocr_good1.txt"
{
  echo "### GREEN runtime signature #1 (§11.4.108) text=\"$TEXT_1\""
  "$BIN" analyze "$EVID/green_response_good1.json" "$EXP_TOKENS_1" "$CONF_FLOOR"; sig1=$?
  echo "signature_exit=$sig1"
  "$BIN" determinism "$EVID/green_response_good1.json" "$EVID/green_response_good1_rep.json"; det1=$?
  echo "determinism_exit=$det1"
  [ "$sig1" -eq 0 ] && [ "$det1" -eq 0 ] && echo "GREEN-OK-1" || echo "GREEN-FAIL-1"
} 2>&1 | tee "$EVID/11_green_proof_1.txt"

# ---------- GREEN PROOF #2 (string 2, distinct — not a one-image fluke) ----------
"$BIN" ocr "$BASE" "$EVID/fixture_good2.png" "$EVID/green_response_good2.json" | tee "$EVID/32_ocr_good2.txt"
{
  echo "### GREEN runtime signature #2 (§11.4.108) text=\"$TEXT_2\""
  "$BIN" analyze "$EVID/green_response_good2.json" "$EXP_TOKENS_2" "$CONF_FLOOR"; sig2=$?
  echo "signature_exit=$sig2"
  [ "$sig2" -eq 0 ] && echo "GREEN-OK-2" || echo "GREEN-FAIL-2"
} 2>&1 | tee "$EVID/12_green_proof_2.txt"

# ---------- GOLDEN-BAD OCR (blank / noise / wrong-content) ----------
"$BIN" ocr "$BASE" "$EVID/fixture_blank.png" "$EVID/bad_response_blank.json" | tee "$EVID/33_ocr_blank.txt"
"$BIN" ocr "$BASE" "$EVID/fixture_noise.png" "$EVID/bad_response_noise.json" | tee "$EVID/34_ocr_noise.txt"
"$BIN" ocr "$BASE" "$EVID/fixture_wrong.png" "$EVID/bad_response_wrong.json" | tee "$EVID/35_ocr_wrong.txt"

# ---------- ANALYZER SELF-VALIDATION (§11.4.107(10)) ----------
{
  echo "### analyzer self-validation (§11.4.107(10)) golden-good = real captured response (text=\"$TEXT_1\")"
  "$BIN" selfvalidate "$EVID/green_response_good1.json" "$EXP_TOKENS_1" "$CONF_FLOOR" \
    "$EVID/bad_response_blank.json" "$EVID/bad_response_noise.json" "$EVID/bad_response_wrong.json"
  sv=$?
  echo "selfvalidate_exit=$sv"
} 2>&1 | tee "$EVID/13_self_validation.txt"

# ---------- TEARDOWN + coder/sibling-untouched proof (§11.4.119 / §11.4.174) ----------
teardown_project "$PROJECT" "$EVID/29_teardown.txt"
{
  echo "### post-teardown state"
  echo "ocr containers (expect none):"
  podman ps -a --format '{{.Names}}' | grep "${PROJECT}_" || echo "  (none — removed)"
  echo "coder still running (untouched):"
  podman ps --filter name=helixllm-coder --format '{{.Names}} {{.Status}}'
  echo "sibling Phase-3 containers (untouched — none of THESE were started/stopped by this run):"
  podman ps -a --format '{{.Names}} {{.Status}}' | grep -E 'phase3(embed|translate|whisper)' || echo "  (none currently present)"
} | tee "$EVID/29b_post_teardown.txt"

log "DONE. Evidence in $EVID"
