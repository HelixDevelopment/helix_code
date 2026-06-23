#!/usr/bin/env bash
# ==============================================================================
# record_gui_features_inprocess.sh — FULLY-AUTOMATIC (no-TCC, no-Aqua, no-human)
# recording of the REST of the HelixCode desktop Fyne GUI's features (the tabs
# BEYOND LLM-chat, which record_gui_inprocess.sh already covers).
# ==============================================================================
# §11.4.158 fully automatic + autonomous + rerunnable — NO human step, runnable
#           in ANY launchd domain (incl. Background). Drives the GUI ENTIRELY
#           IN-PROCESS via Fyne's software renderer + in-memory test driver
#           (fyne.io/fyne/v2/test + .../driver/software): no real window, no
#           WindowServer, no GL, no TCC. Identical mechanism to the LLM-chat
#           recorder (record_gui_inprocess.sh) — deep-research-confirmed there.
#
# WHAT IS REAL (no simulation — §11.4.2/.107/.158/.159):
#   * Each tab is built by the SAME production create<Tab>() main.go uses, then
#     software-painted to PNGs by the in-process harness
#     (applications/desktop/gui_record_features_test.go).
#   * REAL managers + REAL bundle-backed i18n translator wire the DesktopApp;
#     REAL domain data is seeded through the REAL manager APIs and the REAL list
#     selection / button taps / theme switch are driven. Nothing is fabricated.
#   * No LLM provider is required (these tabs render local domain state) — so
#     this wrapper runs even when no API key is present.
#
# OUTPUT (§11.4.155 project-prefix + §11.4.158 path): one MP4 per feature at
#   /Volumes/T7/Downloads/Recordings/helixcode-desktopgui-<feature>-<UTC>.mp4
#   (H.264 / yuv420p / +faststart). <feature> ∈
#   {dashboard,tasks,workers,projects,sessions,settings}.
#
# USAGE: scripts/video_qa/record_gui_features_inprocess.sh
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"            # <repo>/scripts/video_qa
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"           # meta-repo root
INNER_ROOT="$REPO_ROOT/helix_code"                     # inner Go module
REC_DIR="${REC_DIR:-/Volumes/T7/Downloads/Recordings}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
PROJECT_PREFIX="$(grep -E '^HELIX_RELEASE_PREFIX=' "$REPO_ROOT/.env" 2>/dev/null | head -1 | cut -d= -f2)"
[[ -z "$PROJECT_PREFIX" ]] && PROJECT_PREFIX="$(basename "$REPO_ROOT" | tr '[:upper:]' '[:lower:]')"
RECORD_FEATURE="$SCRIPT_DIR/record_feature.sh"

log()  { printf '[gui-feat] %s\n' "$*"; }
fail() { printf '[gui-feat][FAIL] %s\n' "$*" >&2; exit 1; }

command -v ffmpeg >/dev/null  || fail "ffmpeg missing — brew install ffmpeg"
command -v go >/dev/null      || fail "go missing"
[[ -x "$RECORD_FEATURE" ]]    || fail "record_feature.sh missing at $RECORD_FEATURE"

mkdir -p "$REC_DIR" || fail "cannot create $REC_DIR"

# Per-feature expected OCR content (ALL-must-match; real prose / real seeded
# data the production tab genuinely renders — see the OCR-confirmed text in the
# evidence doc). Tokens chosen to be OCR-robust (avoid garbled glyph runs).
feature_expect() {
  case "$1" in
    dashboard) echo "HelixCode|Workers|Quick Actions";;
    tasks)     echo "Tasks|Recorded task|Create Task";;
    workers)   echo "Workers|worker-seed-1|Add Worker";;
    projects)  echo "Projects|HelixCode Recorder|Project Details";;
    sessions)  echo "Sessions|Recorder Session|Session Details";;
    settings)  echo "Current Theme|Primary|Server Connection";;
    *) echo "";;
  esac
}
FEATURES="dashboard tasks workers projects sessions settings"

# §11.4.154(B) fresh-corpus rotation: remove ONLY our own scope-prefixed prior
# artefacts (never foreign/operator files — §11.4.122/§9.2).
for feat in $FEATURES; do
  scope="${PROJECT_PREFIX}-desktopgui-${feat}"
  log "rotating prior ${scope}-* recordings in $REC_DIR (our own scope only)"
  rm -f "${REC_DIR}/${scope}-"*.mp4 "${REC_DIR}/${scope}-"*.evidence_frame.png 2>/dev/null || true
done

FRAMES_BASE="$(mktemp -d "${TMPDIR:-/tmp}/helix_gui_feat_frames.XXXXXX")"
cleanup() { rm -rf "$FRAMES_BASE"; }
trap cleanup EXIT
log "frames base: $FRAMES_BASE"

# ---- 1) Run the in-process Go harness (renders REAL widgets to PNG frames) ----
log "running in-process Fyne software-render harness (no window, no TCC)…"
TEST_LOG="$FRAMES_BASE/gotest.log"
set +e
( cd "$INNER_ROOT" && \
  HELIX_GUI_FEATURES_FRAMES_DIR="$FRAMES_BASE" \
  go test -count=1 -timeout 300s \
    -run 'TestRecordDesktopGUI(Dashboard|Tasks|Workers|Projects|Sessions|Settings)' \
    -v ./applications/desktop/ ) >"$TEST_LOG" 2>&1
TEST_RC=$?
set -e 2>/dev/null || true
if [[ $TEST_RC -ne 0 ]]; then
  tail -80 "$TEST_LOG" >&2
  fail "in-process feature harness FAILED (rc=$TEST_RC) — see $TEST_LOG"
fi
grep -E 'RECORD-OK|--- (PASS|FAIL)' "$TEST_LOG" | sed 's/^/    /' >&2

# ---- 2) Self-validate the OCR analyzer ONCE (golden-good/golden-bad, §11.4.107(10)) ----
log "self-validating OCR analyzer (golden-good/golden-bad)…"
SELFTEST_OUT="$FRAMES_BASE/selftest.log"
if "$RECORD_FEATURE" selftest >"$SELFTEST_OUT" 2>&1; then
  log "analyzer self-validation: PASS"
elif grep -q 'ANALYZER SELF-VALIDATION: PASS\|analyzer.*self-validation.*: PASS' "$SELFTEST_OUT"; then
  log "analyzer self-validation: PASS (live-capture half an honest env-gap — analyzer proven)"
else
  tail -30 "$SELFTEST_OUT" >&2
  fail "analyzer self-validation FAILED — cannot trust OCR verdicts"
fi

# ---- 3) Per feature: assemble MP4 + OCR content-validate -----------------------
declare -a PASS_FEATS=() FAIL_FEATS=() SKIP_FEATS=()
for feat in $FEATURES; do
  fdir="$FRAMES_BASE/$feat"
  scope="${PROJECT_PREFIX}-desktopgui-${feat}"
  mp4="${REC_DIR}/${scope}-${TS}.mp4"
  expect="$(feature_expect "$feat")"

  if [[ ! -d "$fdir" ]]; then
    log "[$feat] no frames dir — SKIP (harness did not produce it)"
    SKIP_FEATS+=("$feat"); continue
  fi
  nframes="$(ls -1 "$fdir"/frame_*.png 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$nframes" -lt 3 ]]; then
    log "[$feat] only $nframes frames (<3) — FAIL"
    FAIL_FEATS+=("$feat"); continue
  fi

  # Assemble PNG frames → MP4 (H.264 / yuv420p / +faststart, §11.4.159(B)).
  log "[$feat] assembling $nframes frames → $mp4"
  if ! ffmpeg -nostdin -v error -y \
      -framerate 5/7 -pattern_type glob -i "$fdir/frame_*.png" \
      -vf "fps=24,scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p" \
      -c:v libx264 -movflags +faststart "$mp4" </dev/null; then
    log "[$feat] ffmpeg assembly FAILED"
    FAIL_FEATS+=("$feat"); continue
  fi
  [[ -s "$mp4" ]] || { log "[$feat] MP4 not produced"; FAIL_FEATS+=("$feat"); continue; }

  # OCR content-validation (§11.4.117/.159(J)): the real tab content the user
  # READS must be present. Try per-frame first, then MP4-level sampled frames.
  log "[$feat] OCR content-validate: expect '$expect'"
  OCR_LOG="$fdir/ocr.log"; : > "$OCR_LOG"; ocr_pass=0
  for png in "$fdir"/frame_*.png; do
    if "$RECORD_FEATURE" ocr-analyze "$png" --expect "$expect" >>"$OCR_LOG" 2>&1; then
      ocr_pass=1; cp "$png" "${mp4%.mp4}.evidence_frame.png" 2>/dev/null || true
      log "[$feat] OCR PASS on $(basename "$png")"; break
    fi
  done
  if [[ "$ocr_pass" -ne 1 ]]; then
    log "[$feat] per-frame OCR no-match; trying MP4-level validate…"
    if "$RECORD_FEATURE" validate "$mp4" --expect "$expect" >>"$OCR_LOG" 2>&1; then
      ocr_pass=1; log "[$feat] OCR PASS at MP4 level"
    fi
  fi
  if [[ "$ocr_pass" -ne 1 ]]; then
    echo "---- [$feat] OCR log tail ----" >&2; tail -25 "$OCR_LOG" >&2
    log "[$feat] OCR content-validation FAILED (blank/wrong render?) — FAIL"
    FAIL_FEATS+=("$feat"); continue
  fi
  log "[$feat] DONE: $mp4 ($(du -h "$mp4" | cut -f1))"
  PASS_FEATS+=("$feat")
done

# ---- 4) Summary ---------------------------------------------------------------
echo "============================================================" >&2
echo "GUI-FEATURES RECORD SUMMARY (fully automatic, no-TCC, no-Aqua, no-human):" >&2
echo "  PASS (recorded + OCR-validated): ${PASS_FEATS[*]:-<none>}" >&2
echo "  FAIL: ${FAIL_FEATS[*]:-<none>}" >&2
echo "  SKIP: ${SKIP_FEATS[*]:-<none>}" >&2
echo "  output dir: $REC_DIR (prefix ${PROJECT_PREFIX}-desktopgui-<feature>-)" >&2
echo "============================================================" >&2

[[ ${#FAIL_FEATS[@]} -eq 0 ]] || exit 1
exit 0
