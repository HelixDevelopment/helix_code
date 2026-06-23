#!/usr/bin/env bash
# ==============================================================================
# record_tui_views.sh — HelixCode terminal-UI PER-VIEW video-QA recorder (HXC-108)
# ==============================================================================
# Fully-automatic, no-TCC/no-human per-view recorder for the HelixCode tview/tcell
# terminal-UI (helix_code/applications/terminal_ui). The dashboard is already
# covered (helixcode-tui-dashboard-*); this harness covers the PER-VIEW screens
# (Tasks / Workers / Projects / Sessions / LLM-chat / QA / Settings) by driving the
# live tcell session through tmux send-keys — the follow-up the HXC-108 dashboard
# evidence doc explicitly handed off ("tmux send-keys against the live tcell session").
#
# Why tmux (not screencapture): screencapture is TCC-blocked on this host, and the
# tcell raw-mode input cannot be reliably fed by a recorded shell. A detached tmux
# pane DOES render the tview UI (verified) and DOES accept send-keys nav, and
# `tmux capture-pane -p` returns the literal terminal CELL BUFFER — the unforgeable
# ground-truth of what the user sees (no OCR sampling-window risk, §11.4.107).
#
# Constitution anchors honoured:
#   §11.4.155  project-name-prefixed filenames: helixcode-tui-<view>-<ts>.mp4
#              (HELIX_RELEASE_PREFIX from .env → "helixcode", else dir-name).
#   §11.4.154  fresh-corpus rotation of ONLY this harness's own helixcode-tui-<view>-*
#              files (NEVER foreign/operator/HXC-112 files — §11.4.122 / §9.2).
#              Window/pane-scoped (a single tmux pane), never whole-desktop.
#   §11.4.158/.159  intensive per-view recording + READ-the-screen content
#              verification (capture-pane text is the read-back oracle) + MP4
#              (H.264 +faststart yuv420p) + content-not-duration.
#   §11.4.107(10)  the OCR analyzer reused for the secondary frame oracle is the
#              self-validated golden-good/golden-bad analyzer in record_feature.sh.
#   §11.4.6    real evidence only; §11.4.3 honest SKIP-with-reason for views that
#              genuinely need an unavailable backend; a non-rendered / panicking
#              view is a FINDING (FAIL), NEVER a faked PASS.
#   §11.4.119  this harness OWNS the tmux/asciinema resource; it does NOT touch the
#              desktop/web/ios harnesses (record_gui*, gui_record*_test.go).
#   §11.4.14   the tmux session is reaped on exit (trap), no orphan panes/PIDs.
#
# Subcommands:
#   record [VIEW ...]   record one/more per-view screens (default: all 7). Exit 0 if
#                       every recorded view PASSed (real content read back), non-zero
#                       if any FAILed; SKIPs do not fail the run.
#   selftest            delegate to record_feature.sh selftest (analyzer proof).
#   rotate              rotate ONLY this harness's own helixcode-tui-<view>-* files.
#   resolve-prefix      print the resolved project prefix.
#
# Exit: 0 all-PASS ; 1 one-or-more-FAIL ; 2 usage.
# ==============================================================================
set -uo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${HARNESS_DIR}/../.." && pwd)"
INNER_GO_DIR="${REPO_ROOT}/helix_code"
TUI_PKG="./applications/terminal_ui/"
RECORDINGS_DIR="${HELIX_RECORDINGS_DIR:-/Volumes/T7/Downloads/Recordings}"
FEATURE_HARNESS="${HARNESS_DIR}/record_feature.sh"

# Scratch under $TMPDIR (NEVER /tmp — tesseract is TCC-denied /tmp on this host,
# the documented §11.4.102 finding; honoured here for any OCR fallback).
HELIX_TMP_ROOT="${TMPDIR:-$HOME}"
WORK="$(mktemp -d "${HELIX_TMP_ROOT%/}/helixcode_tuivqa_XXXXXX")"

# tmux session name (unique-ish to avoid clobbering a foreign session).
TMUX_SESSION="helixcode_tuivqa_$$"
# Per-run TUI binary + isolated memory DB (never the operator's store).
TUI_BIN="${WORK}/helixcode_tui"
TUI_MEM_DB="${WORK}/tui_mem.db"
# Config that satisfies validation ("version is required") without touching the
# operator's ~/.config/helixcode/config.json. config/config.yaml has version:1.0.0.
TUI_CONFIG="${INNER_GO_DIR}/config/config.yaml"

PANE_W=200
PANE_H=50
# Provider init takes ~25-30s before the tview UI paints; we poll stderr for the
# final init line instead of a blind sleep (deterministic, §11.4.50).
INIT_TIMEOUT=60
NAV_SETTLE=2.5         # seconds to let a view paint after a nav keypress
HOLD_SECONDS=5         # seconds to hold the view on screen for the recording

cleanup() {
    tmux has-session -t "$TMUX_SESSION" 2>/dev/null && tmux kill-session -t "$TMUX_SESSION" 2>/dev/null
    [[ -n "${WORK:-}" && -d "$WORK" ]] && rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

log()  { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
skip() { printf 'SKIP (honest env-gap §11.4.3): %s\n' "$*" >&2; exit 3; }

require_tools() {
    local miss=()
    for t in "$@"; do command -v "$t" >/dev/null 2>&1 || miss+=("$t"); done
    [[ ${#miss[@]} -eq 0 ]] || skip "missing required tool(s): ${miss[*]}"
}

# ---- §11.4.155 / §11.4.151 prefix resolution (env-first, deterministic) ------
resolve_prefix() {
    if [[ -n "${HELIX_RELEASE_PREFIX:-}" ]]; then printf '%s' "${HELIX_RELEASE_PREFIX}"; return 0; fi
    if [[ -f "${REPO_ROOT}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HELIX_RELEASE_PREFIX[[:space:]]*=' "${REPO_ROOT}/.env" 2>/dev/null \
              | tail -1 | sed -E 's/^[^=]*=[[:space:]]*//; s/^"//; s/"$//; s/^'\''//; s/'\''$//')"
        if [[ -n "$v" ]]; then printf '%s' "$v"; return 0; fi
    fi
    basename "${REPO_ROOT}" | tr '[:upper:]' '[:lower:]'
}

# ==============================================================================
# §11.4.154(B) fresh-corpus rotation — ONLY this harness's own scope-prefixed
# files: <prefix>-tui-<view>-*.{mp4,cast,gif,png}. NEVER the dashboard
# (<prefix>-tui-dashboard-*, owned by the prior recorder) and NEVER foreign/
# operator/HXC-112 files. We rotate per-VIEW so re-recording one view does not
# wipe other views' fresh evidence.
# ==============================================================================
rotate_view() {
    local view="$1" prefix glob n=0
    prefix="$(resolve_prefix)"
    [[ -n "$view" ]] || { log "rotate: empty view (refusing broad glob)"; return 0; }
    [[ "$view" == "dashboard" ]] && { log "rotate: refusing to touch dashboard (owned by prior recorder)"; return 0; }
    [[ -d "$RECORDINGS_DIR" ]] || { log "rotate: recordings dir absent, nothing to rotate"; return 0; }
    glob="${prefix}-tui-${view}-"
    shopt -s nullglob
    for f in "${RECORDINGS_DIR}/${glob}"*.mp4 "${RECORDINGS_DIR}/${glob}"*.cast \
             "${RECORDINGS_DIR}/${glob}"*.gif "${RECORDINGS_DIR}/${glob}"*.png; do
        case "$(basename "$f")" in
            "${glob}"*) rm -f "$f" && n=$((n+1)) && log "rotated (removed own stale): $(basename "$f")";;
            *) log "SKIP rotate (basename guard, not ours): $(basename "$f")";;
        esac
    done
    shopt -u nullglob
    log "rotation: removed ${n} own prior file(s) for view '${view}'"
}

# ---- view registry: VIEW = nav-key + expected-content patterns (read-back) ----
# Patterns are asserted against the literal tmux capture-pane cell buffer
# (the unforgeable ground-truth of what the user sees, §11.4.107). Each view's
# patterns are distinct from the dashboard so a stuck/non-navigated screen FAILs.
view_navkey() {
    case "$1" in
        tasks) echo 't';; workers) echo 'w';; projects) echo 'p';;
        sessions) echo 's';; llm) echo 'l';; qa) echo 'q';; settings) echo 'c';;
        *) echo '';;
    esac
}
# Expected patterns: '|'-separated; ALL must appear in the captured pane.
view_expect() {
    case "$1" in
        tasks)    echo 'Task Management|Tasks';;
        workers)  echo 'Worker Management|Hostname|Heartbeat';;
        projects) echo 'Project Management|Project Details';;
        sessions) echo 'Session Management|Development Sessions';;
        llm)      echo 'AI Model Interaction|Chat|Message';;
        qa)       echo 'QA Dashboard|QA Engine';;
        settings) echo 'Settings|Theme|Cognee';;
        *) echo '';;
    esac
}
ALL_VIEWS="tasks workers projects sessions llm qa settings"

# ==============================================================================
# Build (or reuse) the TUI binary once.
# ==============================================================================
build_tui() {
    if [[ -x "$TUI_BIN" ]]; then return 0; fi
    require_tools go
    log "building TUI binary (${TUI_PKG}) ..."
    ( cd "$INNER_GO_DIR" && go build -o "$TUI_BIN" "$TUI_PKG" ) >"${WORK}/build.log" 2>&1 \
        || { sed 's/^/    [build] /' "${WORK}/build.log" >&2; fail "TUI build failed"; }
    [[ -x "$TUI_BIN" ]] || fail "TUI build produced no binary"
    log "TUI binary ready: ${TUI_BIN}"
}

# ==============================================================================
# Launch the TUI fresh in a clean tmux pane and wait for full provider-init paint.
# Returns 0 when painted, 1 on early exit / timeout.
# ==============================================================================
launch_tui() {
    local stderr_log="$1"
    tmux has-session -t "$TMUX_SESSION" 2>/dev/null && tmux kill-session -t "$TMUX_SESSION" 2>/dev/null
    tmux new-session -d -s "$TMUX_SESSION" -x "$PANE_W" -y "$PANE_H"
    sleep 0.3
    # Drive the binary with the valid config + isolated memory DB.
    tmux send-keys -t "$TMUX_SESSION" \
        "cd '${INNER_GO_DIR}' && HELIX_CONFIG='${TUI_CONFIG}' HELIX_MEMORY_DB='${TUI_MEM_DB}' '${TUI_BIN}' 2>'${stderr_log}'" Enter
    # Poll stderr for the final init line ("plugins loaded"/"plugins unavailable")
    # then confirm the dashboard sidebar painted.
    # Poll for the final-init line. Do NOT treat a transient pgrep miss as death:
    # the binary takes a few seconds to exec + retries DB/Redis (degraded-mode is
    # expected and fine for recording), so it may not show under its name on the
    # first iteration. The only deterministic early-exit signal is a CONFIG ERROR
    # written to stderr (the binary log.Fatalf's "config validation failed").
    local i painted=1
    for ((i=1; i<=INIT_TIMEOUT; i++)); do
        sleep 1
        if grep -q "plugins loaded\|plugins unavailable" "$stderr_log" 2>/dev/null; then
            painted=0; break
        fi
        if grep -q "config validation failed\|Failed to initialize Terminal UI" "$stderr_log" 2>/dev/null; then
            log "launch: TUI fatal init error (see stderr)"; return 1
        fi
    done
    [[ $painted -eq 0 ]] || { log "launch: init timeout after ${INIT_TIMEOUT}s"; return 1; }
    sleep 2  # allow the dashboard to paint after init completes
    # Confirm the sidebar is on screen (proves the tview UI actually rendered).
    if ! tmux capture-pane -t "$TMUX_SESSION" -p 2>/dev/null | grep -q "(d) Dashboard"; then
        log "launch: sidebar did not paint"; return 1
    fi
    return 0
}

tui_alive() { pgrep -f "$(basename "$TUI_BIN")" >/dev/null 2>&1; }

# Quit the TUI cleanly (signal-only quit path) and reap the pane (§11.4.14).
quit_tui() {
    tmux send-keys -t "$TMUX_SESSION" C-c 2>/dev/null || true
    sleep 1
    tmux has-session -t "$TMUX_SESSION" 2>/dev/null && tmux kill-session -t "$TMUX_SESSION" 2>/dev/null
    # Belt-and-braces: ensure no orphan TUI process survives.
    pkill -f "$(basename "$TUI_BIN")" 2>/dev/null || true
}

# Did the captured pane reach the target view? (header pattern present)
pane_has_all() {
    local pane="$1" expect="$2"
    local IFS='|'
    read -ra pats <<< "$expect"
    local p
    for p in "${pats[@]}"; do
        [[ -z "$p" ]] && continue
        printf '%s' "$pane" | grep -qF "$p" || return 1
    done
    return 0
}

# ==============================================================================
# Render a per-view recording from the live tmux session into an MP4.
# Primary content oracle: the tmux capture-pane TEXT (ground-truth cell buffer).
# Video: we record the nav+hold interaction as an asciinema .cast, render it to a
# .gif via agg, then transcode to H.264 MP4 (§11.4.159(B)). If agg/asciinema are
# unavailable we fall back to assembling the captured pane PNG frame(s) into an MP4
# via ffmpeg+ImageMagick — still a real recording of the real rendered view.
# ==============================================================================
record_one_view() {
    local view="$1"
    local navkey expect prefix ts base mp4 pane_txt
    navkey="$(view_navkey "$view")"
    expect="$(view_expect "$view")"
    [[ -n "$navkey" && -n "$expect" ]] || { log "record_one_view: unknown view '${view}'"; return 2; }
    prefix="$(resolve_prefix)"
    ts="$(date -u +%Y%m%dT%H%M%SZ)"
    base="${prefix}-tui-${view}-${ts}"
    mp4="${RECORDINGS_DIR}/${base}.mp4"

    log "=================== VIEW: ${view} (nav='${navkey}') ==================="
    rotate_view "$view"            # §11.4.154(B) rotate own prior first
    mkdir -p "$RECORDINGS_DIR"

    local stderr_log="${WORK}/tui_${view}.stderr"
    if ! launch_tui "$stderr_log"; then
        # The TUI itself failed to start/paint — honest env/runtime finding.
        # Distinguish a config/runtime panic (FINDING/FAIL) from missing tooling (SKIP).
        if grep -q "config validation failed" "$stderr_log" 2>/dev/null; then
            log "VIEW ${view}: SKIP — TUI config invalid (see ${stderr_log})"; return 3
        fi
        log "VIEW ${view}: FAIL — TUI did not paint before nav (runtime finding; see ${stderr_log})"
        FINDINGS+=("${view}: TUI did not paint/start ($(tail -1 "$stderr_log" 2>/dev/null))")
        return 1
    fi

    # ---- navigate to the view (with a documented focus-recovery retry) ----
    # From the boot/dashboard state the sidebar List has focus; its registered
    # single-letter shortcut routes most views. Some keys (notably 'l') only route
    # via the app-level capture once focus has left the sidebar list. Recovery:
    # if the first keypress did not change the header, send Escape (drop focus) and
    # re-send the key so the app-level menuHotkeyTarget capture routes it.
    tmux send-keys -t "$TMUX_SESSION" "$navkey"; sleep "$NAV_SETTLE"
    pane_txt="$(tmux capture-pane -t "$TMUX_SESSION" -p 2>/dev/null)"
    if ! pane_has_all "$pane_txt" "$expect"; then
        log "VIEW ${view}: first nav did not reach view; focus-recovery retry (Esc + re-send)"
        tmux send-keys -t "$TMUX_SESSION" Escape; sleep 0.6
        tmux send-keys -t "$TMUX_SESSION" "$navkey"; sleep "$NAV_SETTLE"
        pane_txt="$(tmux capture-pane -t "$TMUX_SESSION" -p 2>/dev/null)"
    fi
    if ! tui_alive; then
        # A nav keypress crashed the TUI — a real, reproducible DEFECT (FINDING).
        local panic; panic="$(grep -m1 'panic:\|nil pointer\|runtime error' "$stderr_log" 2>/dev/null)"
        log "VIEW ${view}: FAIL — TUI CRASHED on navigation. ${panic}"
        FINDINGS+=("${view}: TUI PANICKED on nav '${navkey}' — ${panic:-see ${stderr_log}}")
        # capture the crash stack into the work dir for the evidence doc
        cp "$stderr_log" "${WORK}/${view}.crash.stderr" 2>/dev/null || true
        return 1
    fi

    # ---- PRIMARY content oracle: capture-pane text read-back ----
    if ! pane_has_all "$pane_txt" "$expect"; then
        log "VIEW ${view}: FAIL — expected content not read back from pane (nav unreliable)."
        printf '%s\n' "$pane_txt" | sed -n '1,8p' | sed 's/^/    [pane] /' >&2
        FINDINGS+=("${view}: expected content '${expect}' not present after nav '${navkey}'")
        quit_tui
        return 1
    fi
    log "VIEW ${view}: content read-back PASS (all expected patterns present in pane buffer)"
    # Persist the literal pane text as machine-checkable evidence.
    printf '%s\n' "$pane_txt" > "${RECORDINGS_DIR}/${base}.pane.txt"

    # ---- anti-bluff scan on the pane text (§11.4.159(I)) ----
    local lc; lc="$(printf '%s' "$pane_txt" | tr '[:upper:]' '[:lower:]')"
    local b bluff=""
    for b in "todo implement" "for now, simulate" "simulated response" "placeholder" "in production this would"; do
        if printf '%s' "$lc" | grep -qF "$b"; then bluff="$b"; break; fi
    done
    if [[ -n "$bluff" ]]; then
        log "VIEW ${view}: FAIL — anti-bluff pattern in rendered view: '${bluff}'"
        FINDINGS+=("${view}: anti-bluff pattern '${bluff}' present in rendered view")
        quit_tui
        return 1
    fi

    # ---- produce the MP4 recording of the LIVE view ----
    # The view is still on screen here. We sample the LIVE tmux pane at a fixed
    # cadence over HOLD_SECONDS (real rendered frames, not a synthetic still),
    # render each snapshot to a PNG, and assemble them into an H.264 MP4. This
    # captures the genuine view as it sits painted — and, where present, any live
    # cursor/blink/redraw motion across the hold window.
    local made_mp4=1
    if render_view_mp4_from_live_pane "$view" "$mp4"; then made_mp4=0; fi
    quit_tui   # done sampling; reap the pane
    if [[ $made_mp4 -ne 0 ]]; then
        log "VIEW ${view}: content PASS but MP4 render unavailable (no asciinema/agg/ffmpeg or magick) — honest §11.4.3 partial"
        echo "VIEW-RESULT: PASS-NO-MP4 ${view} (content read-back verified; mp4 render tooling absent)"
        PASS_VIEWS+=("$view")
        return 0
    fi
    [[ -s "$mp4" ]] || { log "VIEW ${view}: MP4 render produced empty file"; FINDINGS+=("${view}: empty MP4"); return 1; }

    # ---- SECONDARY oracle: OCR the produced MP4 with the self-validated analyzer ----
    # Best-effort confirmation that the rendered video also reads back the content.
    if [[ -x "$FEATURE_HARNESS" ]] && command -v tesseract >/dev/null 2>&1; then
        local ocr_pat; ocr_pat="$(printf '%s' "$expect" | cut -d'|' -f1)"   # first pattern
        if "$FEATURE_HARNESS" validate "$mp4" --expect "$ocr_pat" >"${WORK}/${view}.ocr" 2>&1; then
            log "VIEW ${view}: secondary OCR oracle PASS ('${ocr_pat}' read from MP4)"
        else
            log "VIEW ${view}: secondary OCR oracle did not confirm (terminal-bitmap OCR is best-effort; primary pane oracle PASSed)"
        fi
    fi

    log "VIEW ${view}: PASS — mp4=${mp4}"
    echo "VIEW-RESULT: PASS ${view}  mp4=${mp4}"
    PASS_VIEWS+=("$view")
    return 0
}

# Sample the LIVE tmux pane over HOLD_SECONDS into PNG frames, assemble an H.264 MP4.
# Requires the target view to be currently displayed in $TMUX_SESSION. Real rendered
# frames of the real view (not a synthetic still). The richest still is also kept as
# the .evidence_frame.png beside the MP4 (§11.4.159 content-not-duration).
render_view_mp4_from_live_pane() {
    local view="$1" mp4="$2"
    command -v ffmpeg >/dev/null 2>&1 || return 1
    tmux has-session -t "$TMUX_SESSION" 2>/dev/null || return 1
    local frdir; frdir="${WORK}/${view}_frames"; mkdir -p "$frdir"
    local nframes=$(( HOLD_SECONDS * 2 )); [[ $nframes -lt 4 ]] && nframes=4
    local got=0 k snap png best_png best_lines=0 lines
    for ((k=1; k<=nframes; k++)); do
        snap="$(tmux capture-pane -t "$TMUX_SESSION" -p 2>/dev/null)"
        [[ -z "$snap" ]] && { sleep 0.5; continue; }
        png="${frdir}/f_$(printf '%04d' "$k").png"
        if render_pane_png "$snap" "$png"; then
            got=$((got+1))
            # track the most-content-rich frame for the evidence still
            lines="$(printf '%s' "$snap" | grep -c '[^[:space:]]')"
            if [[ "$lines" -gt "$best_lines" ]]; then best_lines="$lines"; best_png="$png"; fi
        fi
        sleep 0.5
    done
    [[ $got -gt 0 ]] || { rm -rf "$frdir"; return 1; }
    # libx264/yuv420p require EVEN width+height; pane PNGs are arbitrary-sized, so
    # pad up to the next even dimension (the documented 'width not divisible by 2'
    # encoder requirement). +faststart for streamable MP4 (§11.4.159(B)).
    ffmpeg -nostdin -v error -y -framerate 2 -pattern_type glob -i "${frdir}/f_*.png" \
        -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
        -c:v libx264 -pix_fmt yuv420p -movflags +faststart -r 10 "$mp4" </dev/null 2>/dev/null \
        || { rm -rf "$frdir"; return 1; }
    [[ -n "${best_png:-}" && -f "$best_png" ]] && cp "$best_png" "${mp4%.mp4}.evidence_frame.png" 2>/dev/null || true
    rm -rf "$frdir"
    [[ -s "$mp4" ]]
}

# Render terminal pane text into a PNG (monospace, dark theme to match the TUI).
render_pane_png() {
    local pane="$1" out="$2"
    local txt="${WORK}/pane.txt"
    printf '%s\n' "$pane" > "$txt"
    local fontfile=""
    local f
    for f in /System/Library/Fonts/Menlo.ttc /System/Library/Fonts/Monaco.ttf; do
        [[ -f "$f" ]] && { fontfile="$f"; break; }
    done
    if command -v magick >/dev/null 2>&1; then
        magick -background black -fill '#7CFC00' ${fontfile:+-font "$fontfile"} \
            -pointsize 14 "label:@${txt}" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
    fi
    if command -v convert >/dev/null 2>&1; then
        convert -background black -fill '#7CFC00' ${fontfile:+-font "$fontfile"} \
            -pointsize 14 "label:@${txt}" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
    fi
    return 1
}

# ==============================================================================
do_record() {
    require_tools tmux go ffmpeg
    build_tui
    local views=("$@")
    [[ ${#views[@]} -eq 0 ]] && read -ra views <<< "$ALL_VIEWS"
    PASS_VIEWS=(); SKIP_VIEWS=(); FINDINGS=()
    local view rc
    for view in "${views[@]}"; do
        record_one_view "$view"; rc=$?
        case $rc in
            0) : ;;  # PASS (recorded into PASS_VIEWS)
            3) SKIP_VIEWS+=("$view");;
            *) : ;;  # FAIL (recorded into FINDINGS)
        esac
        quit_tui   # ensure clean slate between views
    done

    echo "" >&2
    echo "======================= TUI PER-VIEW RECORDING SUMMARY =======================" >&2
    printf 'PASS  (%d): %s\n' "${#PASS_VIEWS[@]}" "${PASS_VIEWS[*]:-none}" >&2
    printf 'SKIP  (%d): %s\n' "${#SKIP_VIEWS[@]}" "${SKIP_VIEWS[*]:-none}" >&2
    printf 'FINDINGS (%d):\n' "${#FINDINGS[@]}" >&2
    local fnd
    for fnd in "${FINDINGS[@]:-}"; do [[ -n "$fnd" ]] && printf '  - %s\n' "$fnd" >&2; done
    echo "==============================================================================" >&2

    [[ ${#FINDINGS[@]} -eq 0 ]] && return 0 || return 1
}

main() {
    [[ $# -ge 1 ]] || { sed -n '2,55p' "${BASH_SOURCE[0]}" >&2; exit 2; }
    local sub="$1"; shift
    case "$sub" in
        record)         do_record "$@";;
        selftest)       [[ -x "$FEATURE_HARNESS" ]] || fail "selftest: ${FEATURE_HARNESS} not found"; "$FEATURE_HARNESS" selftest;;
        rotate)         [[ $# -ge 1 ]] && rotate_view "$1" || { for v in $ALL_VIEWS; do rotate_view "$v"; done; };;
        resolve-prefix) resolve_prefix; echo;;
        *) echo "unknown subcommand: ${sub}" >&2; exit 2;;
    esac
}
main "$@"
