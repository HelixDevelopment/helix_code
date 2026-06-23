#!/usr/bin/env bash
# =============================================================================
# run_gui_recordings_aqua.sh — operator-run GUI feature recorder (HXC-108 GUI slice)
# -----------------------------------------------------------------------------
# SUPERSEDED for automation (2026-06-23, §11.4.98): the FULLY-AUTOMATIC headless
# recorder `scripts/video_qa/record_gui_inprocess.sh` (Fyne software-renderer,
# no TCC / no Aqua / no human) is the primary GUI-recording path and needs no
# operator step. This script is retained ONLY as a manual fallback for capturing
# exact on-screen WindowServer pixels, which still requires an Aqua session +
# the TCC grants below. Prefer record_gui_inprocess.sh.
# -----------------------------------------------------------------------------
# WHY THIS EXISTS: the agent's session is in the launchd *Background* domain, so
# it cannot attach a Fyne/GLFW window to the WindowServer (no on-screen capture).
# The desktop-GUI feature recordings must run from a logged-in *Aqua* session.
# This script is that runner.
#
# HOW TO RUN (from a logged-in GUI Terminal, as YOUR normal user):
#     /Volumes/T7/Projects/helix_code/scripts/video_qa/run_gui_recordings_aqua.sh
#
#   Do NOT prefix it with a privilege-escalation command. Two reasons:
#     1. §6.U forbids privilege escalation in our scripts.
#     2. It would break the recording — screen/window capture must run in YOUR
#        Aqua session (root captures the wrong session), and the two required
#        permissions (Accessibility + Screen Recording) are per-app System
#        Settings TOGGLES that escalation cannot set on macOS Sequoia anyway.
#   This script detects those two permissions; if either is missing it OPENS the
#   right System Settings pane and WAITS (polls) for you to toggle it ON for your
#   Terminal, then proceeds automatically.
#
# WHAT THE AGENT TRACKS (§11.4.116 sync channel):
#     /Volumes/T7/Downloads/Recordings/gui_qa/status.json     (atomic snapshot)
#     /Volumes/T7/Downloads/Recordings/gui_qa/events.jsonl     (append-only log)
#     /Volumes/T7/Downloads/Recordings/gui_qa/DONE  or  FAILED (terminal marker)
# When DONE appears, the agent reads the channel + the recording, reviews, and
# commits the curated evidence.
# =============================================================================
set -uo pipefail

REPO_ROOT="/Volumes/T7/Projects/helix_code"
HARNESS="$REPO_ROOT/helix_code/scripts/testing/drive_desktop_gui.sh"
OUT="/Volumes/T7/Downloads/Recordings/gui_qa"
mkdir -p "$OUT"
EVENTS="$OUT/events.jsonl"; STATUS="$OUT/status.json"
: > "$EVENTS"; rm -f "$OUT/DONE" "$OUT/FAILED"

ts() { date -u '+%Y-%m-%dT%H:%M:%SZ'; }
emit() { # emit <phase> <state> <detail>
  printf '{"t":"%s","phase":"%s","state":"%s","detail":"%s"}\n' "$(ts)" "$1" "$2" "${3//\"/\'}" >> "$EVENTS"
  # atomic snapshot (write-temp-then-rename — a reader never sees a torn write)
  printf '{"updated":"%s","phase":"%s","state":"%s","detail":"%s"}\n' "$(ts)" "$1" "$2" "${3//\"/\'}" > "$STATUS.tmp" && mv "$STATUS.tmp" "$STATUS"
  echo "[gui-qa][$1][$2] $3"
}
finish_ok()  { emit "complete" "DONE"   "$1"; : > "$OUT/DONE"; exit 0; }
finish_bad() { emit "complete" "FAILED" "$1"; : > "$OUT/FAILED"; exit 1; }

emit "start" "RUNNING" "GUI feature recorder; user=$(id -un)"

# ---- guard: never run as root (recording must be your Aqua session) ----------
if [ "$(id -u)" = "0" ]; then
  finish_bad "Do NOT run this as root / with escalation (§6.U + capture would target the wrong session). Re-run it as your normal logged-in user."
fi

# ---- 1. Aqua-session preflight ----------------------------------------------
DOMAIN="$(launchctl managername 2>/dev/null || echo unknown)"
emit "preflight" "INFO" "launchd domain=$DOMAIN"
if [ "$DOMAIN" != "Aqua" ]; then
  finish_bad "NOT an Aqua session (domain=$DOMAIN). Run this from a logged-in GUI Terminal (Terminal.app / iTerm), not over SSH/tmux."
fi
[ -x "$HARNESS" ] || finish_bad "harness missing/non-exec: $HARNESS"
command -v cliclick >/dev/null 2>&1 || finish_bad "cliclick missing — run: brew install cliclick"

# ---- 2. TCC grants: Accessibility (cliclick) + Screen Recording (capture) ----
# These are per-app System Settings toggles. TEST → if missing, OPEN the pane →
# WAIT (poll up to 5 min) for you to toggle it ON for your Terminal.
acc_ok() { ! cliclick p:. 2>&1 | grep -qi "Accessibility privileges not enabled"; }
if ! acc_ok; then
  emit "tcc" "WAITING" "Accessibility NOT granted — opening pane; enable your Terminal, then it resumes"
  open "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility" 2>/dev/null || true
  i=0; until acc_ok || [ "$i" -ge 150 ]; do sleep 2; i=$((i+1)); done
  acc_ok || finish_bad "Accessibility still not granted after 5 min."
fi
emit "tcc" "OK" "Accessibility granted"

sr_ok() { local t="/tmp/_srtest_$$.png"; screencapture -x -t png "$t" >/dev/null 2>&1 && [ -s "$t" ]; local r=$?; rm -f "$t"; return $r; }
if ! sr_ok; then
  emit "tcc" "WAITING" "Screen Recording NOT granted — opening pane; enable your Terminal, then it resumes"
  open "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture" 2>/dev/null || true
  i=0; until sr_ok || [ "$i" -ge 150 ]; do sleep 2; i=$((i+1)); done
  sr_ok || finish_bad "Screen Recording still not granted after 5 min."
fi
emit "tcc" "OK" "Accessibility + Screen Recording granted"

# ---- 3. Drive + record the GUI features (window-scoped, OCR-validated) -------
# drive_desktop_gui.sh launches the Fyne GUI, drives the LLM-chat via cliclick,
# window-records to /Volumes/T7/Downloads/Recordings, and OCR-validates.
emit "record" "RUNNING" "invoking drive_desktop_gui.sh (LLM-chat feature)"
LOG="$OUT/drive_desktop_gui.log"
if bash "$HARNESS" >"$LOG" 2>&1; then
  REC="$(ls -t /Volumes/T7/Downloads/Recordings/helix*-hxc112-*.mp4 2>/dev/null | head -1)"
  finish_ok "GUI recording complete. recording=${REC:-?} log=$LOG — agent: read events.jsonl + the recording, review + commit."
else
  rc=$?
  tail_err="$(tail -3 "$LOG" 2>/dev/null | tr '\n' ' ')"
  finish_bad "drive_desktop_gui.sh exit=$rc. last log: $tail_err (full: $LOG)"
fi
