#!/bin/bash
# drive_desktop_gui.sh — HXC-112: real input-driving + window-scoped recording
# of the HelixCode Fyne desktop GUI's LLM-chat feature.
#
# ROOT CAUSE this solves (confirmed by research, see HXC-112 + the harness
# header in find_window_id.swift): Fyne renders every widget onto a single
# OpenGL `GLFWContentView` NSView with NO per-widget Accessibility (AX) tree,
# and GLFW consumes input through the native NSEvent responder chain (the HID
# path). AppleScript `osascript`/"System Events" resolve targets via the AX
# hierarchy and post via the AX path — so they cannot find Fyne widgets and
# their keystrokes are dropped by the GL view. cliclick posts REAL CGEvents to
# kCGHIDEventTap (CGEventCreateMouseEvent/CGEventCreateKeyboardEvent +
# CGEventPost) — the same HID entry-point GLFW listens on — so it DOES reach
# the canvas. This script therefore drives input with cliclick, never osascript.
#
# REQUIREMENTS (all FACT, see research in HXC-112):
#   1. Run inside the logged-in user's Aqua GUI session (NOT an SSH/tmux
#      "Background" launchd session — a Fyne window started from Background
#      cannot attach to the WindowServer and renders no on-screen window;
#      proven: `launchctl managername` == Background from SSH; the helix-desktop
#      process stays alive but never appears in CGWindowListCopyWindowInfo).
#      Launch this script from a Terminal.app/iTerm window on the physical
#      console, OR via `launchctl asuser $(id -u) <this-script>` from a
#      privileged context, OR an Aqua LimitLoadToSessionType launchd agent.
#   2. The controlling terminal app (Terminal/iTerm) MUST be granted
#      Accessibility in System Settings > Privacy & Security > Accessibility
#      (cliclick posts via AXIsProcessTrusted-gated CGEvent; without it events
#      are silently dropped). Screen Recording permission is needed for
#      screencapture/ffmpeg window capture.
#   3. brew install cliclick ffmpeg  (cliclick at /opt/homebrew/bin/cliclick)
#
# USAGE: scripts/testing/drive_desktop_gui.sh [PROMPT]
#   PROMPT defaults to a deterministic arithmetic question so a real LLM
#   response is verifiable. Recording lands under $REC_DIR (default
#   /Volumes/T7/Downloads/Recordings) as helix_code-hxc112-<ts>.mp4 per
#   §11.4.155 project-prefix + §11.4.158 path.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_BIN="$(cd "$SCRIPT_DIR/../.." && pwd)/bin/helix-desktop"
WINDOW_MATCH="${WINDOW_MATCH:-helix}"          # owner/title substring
REC_DIR="${REC_DIR:-/Volumes/T7/Downloads/Recordings}"
PROMPT="${1:-What is 17 plus 25? Reply with the number only.}"
TS="$(date +%Y%m%d-%H%M%S)"
PREFIX="helix_code-hxc112"                       # §11.4.155 project-name prefix
MP4="$REC_DIR/${PREFIX}-${TS}.mp4"
CLICLICK="$(command -v cliclick || echo /opt/homebrew/bin/cliclick)"

log() { printf '[hxc112] %s\n' "$*"; }
fail() { printf '[hxc112][FAIL] %s\n' "$*" >&2; exit 1; }

# --- Preflight (honest, no-bluff): every prerequisite verified, not assumed ---
log "preflight: launchd session = $(launchctl managername 2>/dev/null)"
if [ "$(launchctl managername 2>/dev/null)" != "Aqua" ]; then
  fail "NOT in an Aqua GUI session (got '$(launchctl managername 2>/dev/null)'). A Fyne/GLFW window cannot attach to the WindowServer from a Background/SSH session. Run from a console Terminal or via 'launchctl asuser \$(id -u) $0'."
fi
[ -x "$DESKTOP_BIN" ] || fail "desktop binary missing: $DESKTOP_BIN (run: cd helix_code && make desktop)"
[ -x "$CLICLICK" ] || fail "cliclick missing — brew install cliclick"
command -v ffmpeg >/dev/null || fail "ffmpeg missing — brew install ffmpeg"
# cliclick prints a warning to STDERR when Accessibility is not trusted; treat as hard fail.
if "$CLICLICK" p:. 2>&1 | grep -qi "Accessibility privileges not enabled"; then
  fail "Terminal lacks Accessibility permission — cliclick CGEvents will be dropped. Grant it in System Settings > Privacy & Security > Accessibility."
fi
mkdir -p "$REC_DIR" || fail "cannot create $REC_DIR"

# §11.4.154(B) fresh-corpus rotation: remove ONLY our own prior hxc112 artefacts.
log "rotating prior ${PREFIX}-* recordings in $REC_DIR (our own only)"
rm -f "$REC_DIR/${PREFIX}-"*.mp4 "$REC_DIR/${PREFIX}-"*.png 2>/dev/null || true

# --- Launch the GUI into THIS (Aqua) session ---
log "launching $DESKTOP_BIN"
"$DESKTOP_BIN" > /tmp/hxc112-gui.log 2>&1 &
GUI_PID=$!
cleanup() { kill "$GUI_PID" 2>/dev/null; [ -n "${FF_PID:-}" ] && kill "$FF_PID" 2>/dev/null; }
trap cleanup EXIT

# Wait for the window to appear on screen (up to 20s).
WIN_LINE=""
for _ in $(seq 1 40); do
  WIN_LINE="$(swift "$SCRIPT_DIR/find_window_id.swift" "$WINDOW_MATCH" 2>/dev/null)" && [ -n "$WIN_LINE" ] && break
  sleep 0.5
done
[ -n "$WIN_LINE" ] || fail "GUI window never appeared (see /tmp/hxc112-gui.log). Process alive=$(ps -p $GUI_PID >/dev/null && echo yes || echo no)."
log "window: $WIN_LINE"
# Parse bounds.
eval "$(echo "$WIN_LINE" | tr ' ' '\n' | grep -E '^(id|x|y|w|h)=')"
WIN_ID="$id"; WX="$x"; WY="$y"; WW="$w"; WH="$h"
log "window id=$WIN_ID bounds=${WX},${WY} ${WW}x${WH}"

# --- Start window-scoped recording (ffmpeg avfoundation screen + crop to window rect) ---
# ScreenCaptureKit gives true single-window video but needs a Swift helper;
# ffmpeg avfoundation captures the screen and we crop to the window's CGWindow
# bounds — window-scoped output, never the whole desktop (§11.4.154(A)).
log "starting window-scoped recording -> $MP4"
ffmpeg -y -f avfoundation -capture_cursor 1 -framerate 15 -i "Capture screen 0:none" \
  -vf "crop=${WW}:${WH}:${WX}:${WY}" -t 30 -pix_fmt yuv420p -movflags +faststart \
  -c:v libx264 -preset ultrafast "$MP4" > /tmp/hxc112-ffmpeg.log 2>&1 &
FF_PID=$!
sleep 2   # let the recorder spin up

# --- Bring window forward + navigate to LLM Chat tab ---
# Click the dashboard "LLM Chat" quick-action (top-left action row). We click by
# coordinate within the window because there is no AX tree to target by name.
# Coordinates are window-relative offsets converted to absolute screen points.
clickrel() { "$CLICLICK" "m:$((WX+$1)),$((WY+$2))" "c:$((WX+$1)),$((WY+$2))"; }

# Raise the window first (click its title bar area).
"$CLICLICK" "c:$((WX+WW/2)),$((WY+12))"; sleep 0.5

# Navigate to the LLM tab. The window is a container.AppTabs (main.go:485) with
# tab labels Dashboard|Tasks|Workers|Projects|Sessions|LLM|Settings along the
# TOP tab bar (just under the title bar). The "LLM" tab is the 6th label. We
# click it directly in the tab bar — more robust than the dashboard "LLM Chat"
# quick-action button (which calls da.tabs.SelectIndex(5), main.go:589). The
# tab bar sits ~30px below the window top; tabs are laid out left-to-right.
# Default offsets calibrated for the default window width; override via env.
LLMTAB_X="${LLMTAB_X:-360}"; LLMTAB_Y="${LLMTAB_Y:-40}"
log "navigating to LLM tab in tab bar (rel ${LLMTAB_X},${LLMTAB_Y})"
clickrel "$LLMTAB_X" "$LLMTAB_Y"; sleep 1.0

# --- Focus chat input, type prompt, click Send ---
# The chat input (MultiLineEntry, placeholder "Type your message here...") sits
# along the bottom border of the LLM tab; Send Message button is to its right.
INPUT_X="${INPUT_X:-300}"; INPUT_Y="${INPUT_Y:-620}"
SEND_X="${SEND_X:-820}";   SEND_Y="${SEND_Y:-620}"
log "focusing chat input (rel ${INPUT_X},${INPUT_Y}) and typing prompt"
clickrel "$INPUT_X" "$INPUT_Y"; sleep 0.4
"$CLICLICK" "t:$PROMPT"; sleep 0.6
log "clicking Send Message (rel ${SEND_X},${SEND_Y})"
clickrel "$SEND_X" "$SEND_Y"

# Let the LLM stream a real response into the chat history.
log "waiting for LLM streamed response (recording continues)"
sleep 18

# --- Stop recording ---
kill -INT "$FF_PID" 2>/dev/null; wait "$FF_PID" 2>/dev/null
FF_PID=""
[ -s "$MP4" ] || fail "recording empty: $MP4 (see /tmp/hxc112-ffmpeg.log)"
log "recording done: $MP4 ($(stat -f%z "$MP4") bytes)"

# --- Read-back (§11.4.159(D)/§11.4.160): OCR the recording to confirm the
# typed prompt + the LLM reply actually appear in-GUI. ---
if command -v "$SCRIPT_DIR/ocr_recording.sh" >/dev/null 2>&1 || [ -x "$SCRIPT_DIR/ocr_recording.sh" ]; then
  log "OCR read-back:"
  "$SCRIPT_DIR/ocr_recording.sh" "$MP4" "$PROMPT"
fi

echo "HXC112_RESULT mp4=$MP4 prompt=\"$PROMPT\" window_id=$WIN_ID"
log "DONE"
