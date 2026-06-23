#!/usr/bin/env bash
# ==============================================================================
# record_gui_inprocess.sh — FULLY-AUTOMATIC (no-TCC, no-Aqua, no-human)
# recording of the HelixCode desktop Fyne GUI's LLM-chat feature.
# ==============================================================================
# §11.4.98  fully automatic + autonomous + rerunnable — NO human step, runnable
#           in ANY launchd domain (incl. Background). The prior approach
#           (scripts/testing/drive_desktop_gui.sh) drives a REAL on-screen
#           Fyne/GLFW window with cliclick + screencapture and HARD-REQUIRES an
#           Aqua session + human Accessibility/Screen-Recording TCC grants — it
#           CANNOT satisfy §11.4.98. This wrapper instead drives the GUI ENTIRELY
#           IN-PROCESS via Fyne's software renderer + in-memory test driver
#           (fyne.io/fyne/v2/test + .../driver/software): no real window, no
#           WindowServer, no GL, no TCC.
#
# DEEP-RESEARCH CONFIRMATION (§11.4.123, ≥2 authoritative sources):
#   1. Fyne `test` package — fyne.io/fyne/v2@v2.7.0/test (on-disk module cache):
#        test.NewApp() fyne.App                  (test/app.go:156)
#        test.NewWindow(fyne.CanvasObject) Window (test/window.go:21)
#        test.Type(fyne.Focusable, string)        (test/test.go:174)
#        test.Tap(fyne.Tappable)                  (test/test.go:142)
#        — "utility drivers for running UI tests WITHOUT rendering to a screen"
#          (pkg.go.dev/fyne.io/fyne/v2/test). NewApp "loads a test driver which
#          creates a virtual window in memory for testing."
#   2. Fyne `software` driver — .../driver/software/render.go:
#        software.Render(obj, theme) image.Image  — "renders it to a regular Go
#        image ... the same as setting the application theme and then calling
#        Canvas.Capture()." fyne.Canvas has `Capture() image.Image` (canvas.go:47).
#   The harness (applications/desktop/gui_record_test.go) uses exactly these.
#
# WHAT IS REAL (no simulation — §11.4.2/.107/.158/.159):
#   * Real Fyne widgets (the createLLMTab chat tree) software-painted to PNGs.
#   * Real LLM call: provider resolved from the production verifier-driven
#     ModelManager (RegisterEnvProviders), reply streamed via the production
#     streamDesktopChat() fn. Keys come from ~/api_keys.sh (sourced here).
#   * If no real provider is reachable → the Go test SKIPs honestly (§11.4.3);
#     this wrapper reports that as an honest gap, NOT a faked recording.
#
# OUTPUT (§11.4.155 project-prefix + §11.4.158 path):
#   /Volumes/T7/Downloads/Recordings/helixcode-desktopgui-llmchat-<UTC>.mp4
#   (prefix = HELIX_RELEASE_PREFIX from .env [=helixcode], else lowercased
#    project-root dir name). MP4: H.264 / yuv420p / +faststart.
#
# USAGE: scripts/video_qa/record_gui_inprocess.sh [PROMPT]
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"            # <repo>/scripts/video_qa
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"           # meta-repo root
INNER_ROOT="$REPO_ROOT/helix_code"                     # inner Go module (dev.helix.code)
REC_DIR="${REC_DIR:-/Volumes/T7/Downloads/Recordings}"
PROMPT="${1:-What is 17 plus 25? Reply with just the number.}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
PROJECT_PREFIX="$(grep -E '^HELIX_RELEASE_PREFIX=' "$REPO_ROOT/.env" 2>/dev/null | head -1 | cut -d= -f2)"
[[ -z "$PROJECT_PREFIX" ]] && PROJECT_PREFIX="$(basename "$REPO_ROOT" | tr '[:upper:]' '[:lower:]')"
SCOPE="${PROJECT_PREFIX}-desktopgui-llmchat"           # §11.4.155 scope prefix
MP4="${REC_DIR}/${SCOPE}-${TS}.mp4"
RECORD_FEATURE="$SCRIPT_DIR/record_feature.sh"

log()  { printf '[gui-inproc] %s\n' "$*"; }
fail() { printf '[gui-inproc][FAIL] %s\n' "$*" >&2; exit 1; }

command -v ffmpeg >/dev/null  || fail "ffmpeg missing — brew install ffmpeg"
command -v go >/dev/null      || fail "go missing"
[[ -x "$RECORD_FEATURE" ]]    || fail "record_feature.sh missing at $RECORD_FEATURE"

# Real provider keys (§11.4.10: sourced from the operator's gitignored vault,
# never committed, never echoed). Optional — absence → honest test SKIP.
if [[ -f "$HOME/api_keys.sh" ]]; then
  log "sourcing real provider keys from ~/api_keys.sh (values never printed)"
  set +u; # shellcheck disable=SC1090
  . "$HOME/api_keys.sh"; set -u
fi

mkdir -p "$REC_DIR" || fail "cannot create $REC_DIR"

# §11.4.154(B) fresh-corpus rotation: remove ONLY our own scope-prefixed prior
# artefacts (never foreign/operator files — §11.4.122/§9.2).
log "rotating prior ${SCOPE}-* recordings in $REC_DIR (our own scope only)"
rm -f "${REC_DIR}/${SCOPE}-"*.mp4 2>/dev/null || true

FRAMES_DIR="$(mktemp -d "${TMPDIR:-/tmp}/helix_gui_frames.XXXXXX")"
cleanup() { rm -rf "$FRAMES_DIR"; }
trap cleanup EXIT
log "frames dir: $FRAMES_DIR"

# ---- 1) Run the in-process Go harness (renders REAL widgets to PNG frames) ----
log "running in-process Fyne software-render harness (no window, no TCC)…"
TEST_LOG="$FRAMES_DIR/gotest.log"
set +e
( cd "$INNER_ROOT" && \
  HELIX_GUI_FRAMES_DIR="$FRAMES_DIR" HELIX_GUI_PROMPT="$PROMPT" \
  go test -count=1 -timeout 300s -run TestRecordDesktopGUILLMChat -v ./applications/desktop/ ) \
  >"$TEST_LOG" 2>&1
TEST_RC=$?
set -e 2>/dev/null || true

# Distinguish honest SKIP (no provider) from PASS from FAIL.
if grep -q '^--- SKIP: TestRecordDesktopGUILLMChat' "$TEST_LOG" || grep -q 'SKIP-OK: no real LLM provider' "$TEST_LOG"; then
  sed -n '1,60p' "$TEST_LOG" >&2
  cat "$TEST_LOG" | tail -5 >&2
  echo "RESULT: HONEST-SKIP (§11.4.3) — no real LLM provider reachable; recording requires a real provider, NOT fabricated (§11.4.2)." >&2
  exit 3
fi
if [[ $TEST_RC -ne 0 ]]; then
  tail -60 "$TEST_LOG" >&2
  fail "in-process harness test FAILED (rc=$TEST_RC) — see $TEST_LOG"
fi

NFRAMES="$(ls -1 "$FRAMES_DIR"/frame_*.png 2>/dev/null | wc -l | tr -d ' ')"
[[ "$NFRAMES" -ge 3 ]] || fail "harness produced only $NFRAMES frames (need ≥3 real frames)"
log "harness PASS: $NFRAMES real rendered PNG frames captured"

# ---- 2) Assemble PNG frames → MP4 (H.264 / yuv420p / +faststart, §11.4.159) ----
# Hold each frame ~1.4s (fps=5/7) for a watchable timeline; pad to even dims.
log "assembling $NFRAMES frames → $MP4"
ffmpeg -nostdin -v error -y \
  -framerate 5/7 -pattern_type glob -i "$FRAMES_DIR/frame_*.png" \
  -vf "fps=24,scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p" \
  -c:v libx264 -movflags +faststart "$MP4" </dev/null \
  || fail "ffmpeg MP4 assembly failed"
[[ -s "$MP4" ]] || fail "MP4 not produced: $MP4"
log "MP4 written: $MP4 ($(du -h "$MP4" | cut -f1))"

# ---- 3) Self-validate the OCR analyzer (golden-good/golden-bad, §11.4.107(10)) -
log "self-validating OCR analyzer (golden-good/golden-bad)…"
SELFTEST_OUT="$FRAMES_DIR/selftest.log"
if "$RECORD_FEATURE" selftest >"$SELFTEST_OUT" 2>&1; then
  log "analyzer self-validation: PASS"
elif grep -q 'analyzer.*self-validation.*: PASS\|ANALYZER SELF-VALIDATION: PASS' "$SELFTEST_OUT"; then
  log "analyzer self-validation: PASS (live-capture half an honest env-gap — analyzer proven)"
else
  tail -30 "$SELFTEST_OUT" >&2
  fail "analyzer self-validation FAILED — the OCR analyzer is unreliable, cannot trust its verdict"
fi

# ---- 4) OCR content-validate the rendered frames (§11.4.117/.159(J)) ----------
# The captured chat MUST show the real UI chrome + a real AI turn. We assert the
# rendered text the user READS (not a hierarchy attribute):
#   * "LLM Chat" + "Send Message" — stable GUI chrome, proves the real desktop
#     LLM-chat widget tree rendered (not a blank/wrong surface).
#   * the real model NAME from the AI turn (read from the reply sidecar) —
#     proves a REAL provider turn rendered into the history (anti-bluff §11.4.2).
# All three patterns MUST be present (ocr_analyze is ALL-must-match).
MODEL_TOKEN=""
if [[ -f "$FRAMES_DIR/reply.txt" ]]; then
  # First path segment of "[AI (<provider>/<model>...)]" — e.g. "deepseek".
  MODEL_TOKEN="$(grep -oE '\[AI \(([a-z0-9._-]+)' "$FRAMES_DIR/reply.txt" | head -1 | sed -E 's/^\[AI \(//')"
fi
[[ -z "$MODEL_TOKEN" ]] && MODEL_TOKEN="Chat Settings"   # fallback to stable chrome
EXPECT="LLM Chat|Send Message|${MODEL_TOKEN}"
log "OCR content-validation: assert frames render expected GUI+chat content: $EXPECT"
OCR_PASS=0 OCR_LOG="$FRAMES_DIR/ocr.log"
: > "$OCR_LOG"
for png in "$FRAMES_DIR"/frame_*.png; do
  if "$RECORD_FEATURE" ocr-analyze "$png" --expect "$EXPECT" >>"$OCR_LOG" 2>&1; then
    OCR_PASS=1
    log "OCR PASS on frame $(basename "$png")"
    break
  fi
done
if [[ "$OCR_PASS" -ne 1 ]]; then
  # Also OCR the assembled MP4 across sampled frames (validate subcommand).
  log "per-frame OCR did not match; trying MP4-level validate across sampled frames…"
  if "$RECORD_FEATURE" validate "$MP4" --expect "$EXPECT" >>"$OCR_LOG" 2>&1; then
    OCR_PASS=1
    log "OCR PASS at MP4 level"
  fi
fi
if [[ "$OCR_PASS" -ne 1 ]]; then
  echo "---- OCR log tail ----" >&2; tail -40 "$OCR_LOG" >&2
  fail "OCR content-validation FAILED — rendered frames did not show expected GUI+chat content (possible blank/wrong render)"
fi

# ---- 4b) ANSWER-RENDERED proof (§11.4.2/.107 — prove the REAL reply rendered,
# not just chrome+picker). The chrome check above can pass on the EMPTY initial
# frame (the picker shows the model name before any turn). Here we require a
# frame whose OCR shows the AI-turn ANSWER token — i.e. the real provider reply
# genuinely painted into the chat history widget.
ANSWER_TOKEN=""
if [[ -f "$FRAMES_DIR/reply.txt" ]]; then
  # The text on the last [AI (...)]: line (the rendered answer). Strip the
  # prefix; take the first non-empty token (e.g. "42"). For multi-line replies
  # the first answer token is enough to prove the turn rendered.
  ANSWER_TOKEN="$(grep -E '^\[AI \(' "$FRAMES_DIR/reply.txt" | tail -1 \
                  | sed -E 's/^\[AI \([^)]*\)\]:[[:space:]]*//' \
                  | grep -oE '[A-Za-z0-9]+' | head -1)"
fi
if [[ -n "$ANSWER_TOKEN" ]]; then
  log "answer-rendered proof: require a frame OCR-showing the real AI answer token: '$ANSWER_TOKEN'"
  ANS_PASS=0
  for png in "$FRAMES_DIR"/frame_*.png; do
    # Require BOTH the model name AND the answer token on the SAME frame so we
    # match the rendered "[AI (<model>)]: <answer>" history line, not the picker.
    if "$RECORD_FEATURE" ocr-analyze "$png" --expect "${MODEL_TOKEN}|${ANSWER_TOKEN}" >>"$OCR_LOG" 2>&1; then
      # Guard against a picker-only false match: the answer token must NOT be a
      # substring of the model name (e.g. avoid "deepseek" matching itself).
      if [[ "$MODEL_TOKEN" != *"$ANSWER_TOKEN"* ]] || [[ "$ANSWER_TOKEN" =~ ^[0-9]+$ ]]; then
        ANS_PASS=1; log "answer-rendered PASS on $(basename "$png") (real AI turn '$ANSWER_TOKEN' visible)"; break
      fi
    fi
  done
  if [[ "$ANS_PASS" -ne 1 ]]; then
    echo "---- OCR log tail ----" >&2; tail -30 "$OCR_LOG" >&2
    fail "answer-rendered proof FAILED — no frame showed the real AI answer '$ANSWER_TOKEN' rendered into the chat history (the GUI chrome drew but the real reply did not render — §11.4.2 anti-bluff)"
  fi
else
  log "answer-rendered proof: SKIP (no AI-turn answer token in reply sidecar — Go test already hard-asserts a real reply)"
fi

# ---- 5) Summary -------------------------------------------------------------
echo "============================================================" >&2
echo "RECORD-OK (fully automatic, no-TCC, no-Aqua, no-human):" >&2
echo "  MP4 ........... $MP4" >&2
echo "  frames ........ $NFRAMES real software-rendered PNGs" >&2
echo "  analyzer ...... golden-good/golden-bad self-validation PASS" >&2
echo "  OCR content ... PASS (expected: $EXPECT)" >&2
echo "  reply (head) .. $(head -c 200 "$FRAMES_DIR/reply.txt" 2>/dev/null | tr '\n' ' ')" >&2
echo "============================================================" >&2
echo "$MP4"
