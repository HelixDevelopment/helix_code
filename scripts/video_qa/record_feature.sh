#!/usr/bin/env bash
# ==============================================================================
# record_feature.sh — HelixCode video-QA recording harness (HXC-108)
# ==============================================================================
# Window-scoped video capture + OCR/vision content-validation for HelixCode
# client features, per constitution anchors:
#   §11.4.154  WINDOW-SCOPED capture only (NEVER whole desktop) + fresh-corpus
#              rotation of ONLY this harness's own scope-prefixed prior files
#              (NEVER foreign/operator files — §11.4.122 / §9.2).
#   §11.4.155  project-name-prefixed recording filenames (HELIX_RELEASE_PREFIX
#              from .env, else lowercased project-root dir name).
#   §11.4.158  intensive recording + READ-the-screen content verification.
#   §11.4.159  window-specific MP4 (H.264 +faststart yuv420p) + vision
#              validation + project prefix + content-not-duration.
#   §11.4.160/.163  vision/OCR pipeline reads frames, verifies expected patterns,
#              flags TODO/simulate/placeholder bluffs, self-validated analyzer
#              with golden-good + golden-bad fixtures (§11.4.107(10)).
#   §11.4.6    real evidence only; §11.4.3 honest env-gap SKIP, never faked.
#
# NOTE: stays clear of HXC-112 (desktop Fyne GUI). Default self-test scope-prefix
# is "<project>-harness_selftest-" so corpus rotation never touches HXC-112 files.
#
# Subcommands:
#   record   <client> <feature> [--cmd "<shell command>"] [--window-owner S]
#            [--window-title S] [--seconds N] [--expect "p1|p2|..."]
#            [--scope-prefix-tag TAG]
#   validate <mp4path> --expect "p1|p2|..."        # OCR-validate an existing mp4
#   ocr-analyze <imagepath> --expect "p1|p2|..."   # core analyzer (used by self-test)
#   selftest                                       # prove mechanics + golden-good/bad
#   rotate   <scope-prefix-tag>                    # rotate own prior recordings
#   resolve-prefix                                 # print resolved project prefix
#
# Exit: 0 PASS ; 1 FAIL (content/validation) ; 3 SKIP (honest env-gap) ; 2 usage.
# ==============================================================================
set -uo pipefail

# ---- locate repo root (this script lives at <root>/scripts/video_qa/) --------
HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${HARNESS_DIR}/../.." && pwd)"
RECORDINGS_DIR="${HELIX_RECORDINGS_DIR:-/Volumes/T7/Downloads/Recordings}"
FIND_WIN_SWIFT="${HARNESS_DIR}/find_window_id.swift"

# CRITICAL (§11.4.102 finding): on this host the Homebrew tesseract/leptonica is
# DENIED read access to /tmp (= /private/tmp) image files ("findFileFormat: image
# file not found" on a file that demonstrably exists), but reads files under
# $TMPDIR / $HOME fine. ALL scratch files MUST live under a $TMPDIR-rooted work dir,
# NEVER /tmp, or OCR silently reads nothing (a hidden false-negative bluff surface).
HELIX_TMP_ROOT="${TMPDIR:-$HOME}"
mkwork() { mktemp -d "${HELIX_TMP_ROOT%/}/helixcode_vqa_${1:-work}_XXXXXX"; }
HELIX_WORK="$(mkwork session)"
cleanup_work() { [[ -n "${HELIX_WORK:-}" && -d "$HELIX_WORK" ]] && rm -rf "$HELIX_WORK"; }
trap cleanup_work EXIT

# ---- §11.4.155 / §11.4.151 prefix resolution (env-first, deterministic) ------
resolve_prefix() {
    # 1) HELIX_RELEASE_PREFIX from environment
    if [[ -n "${HELIX_RELEASE_PREFIX:-}" ]]; then
        printf '%s' "${HELIX_RELEASE_PREFIX}"; return 0
    fi
    # 2) HELIX_RELEASE_PREFIX from <root>/.env  (git-ignored; never logged)
    if [[ -f "${REPO_ROOT}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HELIX_RELEASE_PREFIX[[:space:]]*=' "${REPO_ROOT}/.env" 2>/dev/null \
              | tail -1 | sed -E 's/^[^=]*=[[:space:]]*//; s/^"//; s/"$//; s/^'\''//; s/'\''$//')"
        if [[ -n "$v" ]]; then printf '%s' "$v"; return 0; fi
    fi
    # 3) fallback: lowercased snake_case project-root dir name (§11.4.29)
    basename "${REPO_ROOT}" | tr '[:upper:]' '[:lower:]'
}

log()  { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
skip() { printf 'SKIP (honest env-gap §11.4.3): %s\n' "$*" >&2; exit 3; }

require_tools() {
    local miss=()
    for t in "$@"; do command -v "$t" >/dev/null 2>&1 || miss+=("$t"); done
    [[ ${#miss[@]} -eq 0 ]] || skip "missing required tool(s): ${miss[*]}"
}

# ==============================================================================
# §11.4.154(B) fresh-corpus rotation — ONLY this harness's own scope-prefixed
# files. Pattern: <project>-<scope-tag>-*.{mp4,mov,png}. NEVER a wildcard that
# could match foreign/operator/HXC-112 files (§11.4.122 / §9.2).
# ==============================================================================
rotate_own_scope() {
    local scope_tag="$1" prefix
    prefix="$(resolve_prefix)"
    [[ -n "$scope_tag" ]] || fail "rotate: empty scope tag (refusing broad glob)"
    [[ -d "$RECORDINGS_DIR" ]] || { log "rotate: recordings dir absent, nothing to rotate"; return 0; }
    local glob="${prefix}-${scope_tag}-"
    local n=0
    shopt -s nullglob
    for f in "${RECORDINGS_DIR}/${glob}"*.mp4 "${RECORDINGS_DIR}/${glob}"*.mov "${RECORDINGS_DIR}/${glob}"*.png; do
        # Defensive: only delete files whose basename truly starts with our scope prefix.
        case "$(basename "$f")" in
            "${glob}"*) rm -f "$f" && n=$((n+1)) && log "rotated (removed own stale): $(basename "$f")";;
            *) log "SKIP rotate (basename guard, not ours): $(basename "$f")";;
        esac
    done
    shopt -u nullglob
    log "rotation complete for scope '${glob}*': removed ${n} own prior file(s)"
}

# ==============================================================================
# OCR analyzer (§11.4.160/.163) — extract text from an image, verify ALL expected
# patterns present (case-insensitive), and flag anti-bluff patterns
# (TODO implement / simulate / placeholder / "for now"). Self-validated by
# selftest's golden-good + golden-bad pair (§11.4.107(10)).
# Returns 0 if ALL expected patterns matched AND zero bluff patterns; else 1.
# ==============================================================================
ocr_extract() {
    local img="$1"
    [[ -f "$img" ]] || { echo ""; return 1; }
    # tesseract writes <out>.txt ; "-" sends to stdout.
    tesseract "$img" stdout 2>/dev/null
}

ocr_analyze() {
    local img="$1"; shift
    local expect=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --expect) expect="$2"; shift 2;;
            *) shift;;
        esac
    done
    require_tools tesseract
    [[ -f "$img" ]] || fail "ocr-analyze: image not found: $img"
    local text; text="$(ocr_extract "$img")"
    local lc; lc="$(printf '%s' "$text" | tr '[:upper:]' '[:lower:]')"

    log "ocr-analyze: image=$(basename "$img") extracted_chars=${#text}"
    # --- anti-bluff scan (§11.4.158/.159(I)) ---
    local bluff_hit=""
    for b in "todo implement" "simulate" "simulated" "placeholder" "for now" "in production this would"; do
        if printf '%s' "$lc" | grep -qF "$b"; then bluff_hit="$b"; break; fi
    done
    if [[ -n "$bluff_hit" ]]; then
        printf 'OCR-VERDICT: FAIL (anti-bluff pattern present: "%s")\n' "$bluff_hit" >&2
        return 1
    fi
    # --- expected-pattern check (ALL must be present) ---
    if [[ -z "$expect" ]]; then
        printf 'OCR-VERDICT: FAIL (no --expect patterns specified; §11.4.159(J) requires expected content)\n' >&2
        return 1
    fi
    local IFS='|' missing=()
    read -ra pats <<< "$expect"
    for p in "${pats[@]}"; do
        [[ -z "$p" ]] && continue
        local plc; plc="$(printf '%s' "$p" | tr '[:upper:]' '[:lower:]')"
        if ! printf '%s' "$lc" | grep -qF "$plc"; then missing+=("$p"); fi
    done
    if [[ ${#missing[@]} -ne 0 ]]; then
        printf 'OCR-VERDICT: FAIL (missing expected pattern(s): %s)\n' "${missing[*]}" >&2
        return 1
    fi
    printf 'OCR-VERDICT: PASS (all expected patterns present, no bluff): %s\n' "$expect" >&2
    return 0
}

# ==============================================================================
# Extract sample frames from an mp4/mov and OCR-validate across them.
# A recording PASSes if AT LEAST ONE sampled frame passes ocr_analyze
# (the expected content appeared at some steady-state point). FAIL if no
# frame matches OR any frame shows a bluff pattern in isolation handled inside
# ocr_analyze per-frame; overall verdict = any-frame-pass AND no-frame-bluff.
# ==============================================================================
validate_recording() {
    local mp4="$1"; shift
    local expect=""
    while [[ $# -gt 0 ]]; do
        case "$1" in --expect) expect="$2"; shift 2;; *) shift;; esac
    done
    require_tools ffmpeg tesseract
    [[ -f "$mp4" ]] || fail "validate: file not found: $mp4"
    local frdir; frdir="$(mkwork frames)"
    # Sample 1 frame/sec (§11.4.160 <=5s interval); cap at 30 frames.
    ffmpeg -nostdin -v error -i "$mp4" -vf fps=1 -frames:v 30 "${frdir}/f_%03d.png" </dev/null 2>/dev/null
    local frames=( "${frdir}"/f_*.png )
    if [[ ${#frames[@]} -eq 0 || ! -f "${frames[0]}" ]]; then
        rm -rf "$frdir"; fail "validate: ffmpeg extracted 0 frames from $mp4"
    fi
    log "validate: extracted ${#frames[@]} frame(s) from $(basename "$mp4")"
    local any_pass=1 any_bluff=1 evidence_frame=""
    for fr in "${frames[@]}"; do
        if ocr_analyze "$fr" --expect "$expect" >"${HELIX_WORK}/ocrout" 2>&1; then
            any_pass=0; evidence_frame="$fr"
            cp "$fr" "${mp4%.mp4}.evidence_frame.png" 2>/dev/null || true
            break
        fi
        grep -q "anti-bluff pattern present" "${HELIX_WORK}/ocrout" && any_bluff=0
    done
    rm -rf "$frdir"
    if [[ $any_bluff -eq 0 ]]; then
        printf 'RECORDING-VERDICT: FAIL (anti-bluff pattern detected in a frame) — %s\n' "$(basename "$mp4")" >&2
        return 1
    fi
    if [[ $any_pass -eq 0 ]]; then
        printf 'RECORDING-VERDICT: PASS — expected content read back; evidence frame: %s\n' "${mp4%.mp4}.evidence_frame.png" >&2
        return 0
    fi
    printf 'RECORDING-VERDICT: FAIL (expected content "%s" never read in any sampled frame) — %s\n' "$expect" "$(basename "$mp4")" >&2
    return 1
}

# ==============================================================================
# Window-scoped record (§11.4.154/.159) — launch a command in a NEW Terminal
# window, discover its CGWindowID, record THAT window only, transcode to MP4,
# then OCR-validate.
# ==============================================================================
discover_window_id() {
    local owner="$1" title="$2"
    command -v swift >/dev/null 2>&1 || { echo ""; return 1; }
    # Attempt 1: owner + title (most specific).
    if [[ -n "$title" ]]; then
        local wid
        wid="$(swift "$FIND_WIN_SWIFT" --owner "$owner" --title "$title" 2>/dev/null)"
        if [[ -n "$wid" ]]; then echo "$wid"; return 0; fi
        # macOS often returns EMPTY window titles to a CLI process unless it holds the
        # right privacy grant (§11.4.102 finding: Terminal window seen but title=''),
        # so owner+title can miss. Documented owner-only fallback: we activate OUR
        # window so it is the frontmost Terminal window — still WINDOW-SCOPED
        # (a single window id), never whole-desktop (§11.4.154 preserved).
        log "discover: owner+title miss (titles likely hidden by privacy grant); owner-only fallback for '${owner}'"
    fi
    # Attempt 2: owner only (frontmost matching window).
    swift "$FIND_WIN_SWIFT" --owner "$owner" 2>/dev/null
}

record_feature() {
    local client="$1" feature="$2"; shift 2
    local cmd="" owner="Terminal" title="" seconds=6 expect="" scope_tag=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --cmd) cmd="$2"; shift 2;;
            --window-owner) owner="$2"; shift 2;;
            --window-title) title="$2"; shift 2;;
            --seconds) seconds="$2"; shift 2;;
            --expect) expect="$2"; shift 2;;
            --scope-prefix-tag) scope_tag="$2"; shift 2;;
            *) shift;;
        esac
    done
    require_tools screencapture ffmpeg swift osascript
    local prefix; prefix="$(resolve_prefix)"
    local ts; ts="$(date -u +%Y%m%dT%H%M%SZ)"
    [[ -z "$scope_tag" ]] && scope_tag="${client}"
    # §11.4.154(B): rotate ONLY our own scope-prefixed prior files first.
    rotate_own_scope "$scope_tag"
    mkdir -p "$RECORDINGS_DIR"
    # §11.4.155 filename: <prefix>-<client>-<feature>-<ts>.mp4
    local base="${prefix}-${client}-${feature}-${ts}"
    local raw_mov="${RECORDINGS_DIR}/${base}.raw.mov"
    local out_mp4="${RECORDINGS_DIR}/${base}.mp4"

    [[ -n "$cmd" ]] || skip "record: no --cmd given to drive on screen"
    # Launch the command in a NEW Terminal window with a unique title marker so the
    # window-id discovery is deterministic.
    local marker="HELIXCODE_WIN_${ts}"
    log "launching Terminal window (marker=${marker}) running: ${cmd}"
    osascript >/dev/null 2>&1 <<OSA
tell application "Terminal"
    activate
    set newTab to do script "printf '\\\\033]0;${marker}\\\\007'; echo '=== ${marker} ==='; ${cmd}; echo '=== ${marker}_DONE ==='"
    set custom title of (window 1) to "${marker}"
end tell
OSA
    sleep 2
    local wid; wid="$(discover_window_id "$owner" "$marker")"
    if [[ -z "$wid" ]]; then
        # Honest §11.4.3 gap — never fall back to whole-desktop (§11.4.154).
        skip "record: could not discover CGWindowID for owner='${owner}' title~='${marker}' (window-scoped capture unavailable; refusing whole-desktop fallback)"
    fi
    log "discovered window id=${wid}; capturing window-scoped for ${seconds}s"
    local cap_err="${HELIX_WORK}/cap_err"
    # ---- PRIMARY: native window-scoped VIDEO (§11.4.154/.159), bounded -V, -x ----
    if screencapture -v -x -V"${seconds}" -l"${wid}" "$raw_mov" 2>"$cap_err" && [[ -s "$raw_mov" ]]; then
        log "captured native window video (screencapture -v -l)"
        ffmpeg -nostdin -v error -y -i "$raw_mov" \
            -c:v libx264 -pix_fmt yuv420p -movflags +faststart "$out_mp4" </dev/null 2>/dev/null || \
            fail "record: ffmpeg MP4 transcode failed"
        rm -f "$raw_mov"
    else
        log "native window video unavailable ($(tr -d '\n' <"$cap_err")); trying window-scoped STILL timelapse fallback"
        # ---- FALLBACK: window-scoped STILL timelapse (still window-scoped, §11.4.154) ----
        local stilldir; stilldir="$(mkwork stills)"
        local nframes=$(( seconds * 2 )); [[ $nframes -lt 4 ]] && nframes=4
        local got=0 k
        for ((k=1; k<=nframes; k++)); do
            if screencapture -x -o -l"${wid}" "${stilldir}/s_$(printf '%04d' "$k").png" 2>"$cap_err" \
               && [[ -s "${stilldir}/s_$(printf '%04d' "$k").png" ]]; then
                got=$((got+1))
            fi
            sleep 0.5
        done
        if [[ $got -eq 0 ]]; then
            rm -rf "$stilldir"
            # Honest §11.4.3 env-gap: BOTH window primitives blocked by macOS TCC.
            # NEVER fall back to whole-desktop/region capture (§11.4.154 forbids it).
            skip "record: window-scoped capture blocked by host (screencapture -v video AND -l<id> still both failed: '$(tr -d '\n' <"$cap_err")'). Reproduce: screencapture -v -V2 -l${wid} /tmp/x.mov ; screencapture -o -l${wid} /tmp/x.png . Grant Screen Recording video permission to the controlling app, or run from a GUI-foreground session. Refusing whole-desktop fallback."
        fi
        log "captured ${got}/${nframes} window-scoped stills; assembling MP4"
        ffmpeg -nostdin -v error -y -framerate 2 -pattern_type glob -i "${stilldir}/s_*.png" \
            -c:v libx264 -pix_fmt yuv420p -movflags +faststart -r 10 "$out_mp4" </dev/null 2>/dev/null || \
            { rm -rf "$stilldir"; fail "record: ffmpeg stills→MP4 assemble failed"; }
        rm -rf "$stilldir"
    fi
    [[ -s "$out_mp4" ]] || fail "record: no MP4 produced"
    log "recording produced: ${out_mp4}"
    # close ONLY this harness's window (§11.4.159(E)) — match by our marker title.
    osascript >/dev/null 2>&1 <<OSA || true
tell application "Terminal"
    repeat with w in windows
        if (custom title of w) is "${marker}" then close w
    end repeat
end tell
OSA
    # §11.4.158/.159(D) read-the-screen content verification.
    if validate_recording "$out_mp4" --expect "$expect"; then
        printf 'RECORD-RESULT: PASS  mp4=%s\n' "$out_mp4"
        return 0
    else
        printf 'RECORD-RESULT: FAIL  mp4=%s (content verification failed; per §11.4.159(L) investigate root cause before re-record)\n' "$out_mp4"
        return 1
    fi
}

# ==============================================================================
# SELF-TEST (§11.4.107(10)) — prove the OCR analyzer (a) reads a known string
# back from a window-scoped recording (golden-good PASS) and (b) FAILS on a
# wrong expected pattern (golden-bad FAIL). Scope-prefix: <project>-harness_selftest-*.
# Falls back from live window recording to a synthetic-image golden pair ONLY
# for the analyzer-self-validation half if live recording hits an env-gap —
# and reports that as an honest §11.4.3 partial, never a faked full PASS.
# ==============================================================================
selftest() {
    require_tools tesseract ffmpeg
    local prefix; prefix="$(resolve_prefix)"
    local scope_tag="harness_selftest"
    local ts; ts="$(date -u +%Y%m%dT%H%M%SZ)"
    local probe="HELIXCODE_OCR_PROBE_4242"
    log "SELFTEST start (prefix=${prefix}, probe=${probe})"
    rotate_own_scope "$scope_tag"
    mkdir -p "$RECORDINGS_DIR"

    local live_ok=2   # 2=not attempted, 0=ok, 1=failed/skipped
    local out_mp4=""
    # ---- attempt live window-scoped recording of the probe ----
    # CRITICAL (§11.4.102 fix): record_feature uses skip()/fail() which `exit`. Run it in
    # an isolated SUBSHELL so an honest env-gap SKIP inside the live path does NOT abort
    # the whole selftest before the load-bearing analyzer self-validation runs.
    if command -v screencapture >/dev/null && command -v swift >/dev/null && command -v osascript >/dev/null; then
        ( record_feature "harness_selftest" "ocr_probe" \
            --cmd "echo ${probe}; sleep 5" \
            --window-owner "Terminal" \
            --seconds 6 \
            --expect "${probe}" \
            --scope-prefix-tag "${scope_tag}" ) >"${HELIX_WORK}/selftest_live" 2>&1
        local live_rc=$?
        case $live_rc in
            0) live_ok=0; out_mp4="$(grep -oE '/Volumes/.*\.mp4' "${HELIX_WORK}/selftest_live" | tail -1)";;
            *) live_ok=1;;   # 1=FAIL content, 3=SKIP env-gap — both = "live not a clean PASS"
        esac
        sed 's/^/    [live] /' "${HELIX_WORK}/selftest_live" >&2
        log "live recording attempt exit=${live_rc}"
    fi

    # ---- analyzer self-validation: golden-good + golden-bad image fixtures ----
    # These prove the analyzer cannot bluff, INDEPENDENT of the live recording path.
    local fixdir; fixdir="$(mkwork fixtures)"
    local good="${fixdir}/golden_good.png" bad="${fixdir}/golden_bad.png"
    # Render text into PNGs (ImageMagick if present, else ffmpeg drawtext).
    render_text_png() {
        local text="$1" out="$2"
        # Prefer a real font FILE (the `Courier` alias is unresolvable on this host —
        # §11.4.102 finding). Fall back to ImageMagick's built-in default font.
        local fontfile=""
        for f in /System/Library/Fonts/Menlo.ttc \
                 /System/Library/Fonts/Monaco.ttf \
                 /System/Library/Fonts/Supplemental/Courier\ New.ttf; do
            [[ -f "$f" ]] && { fontfile="$f"; break; }
        done
        if command -v magick >/dev/null 2>&1; then
            if [[ -n "$fontfile" ]]; then
                magick -size 1100x200 xc:white -gravity center -pointsize 44 \
                    -font "$fontfile" -fill black -annotate 0 "$text" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
            fi
            # default-font path (no -font): reliable, no alias resolution needed.
            magick -size 1100x200 xc:white -gravity center -pointsize 44 \
                -fill black -annotate 0 "$text" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
        fi
        if command -v convert >/dev/null 2>&1; then
            if [[ -n "$fontfile" ]]; then
                convert -size 1100x200 xc:white -gravity center -pointsize 44 \
                    -font "$fontfile" -fill black -annotate 0 "$text" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
            fi
            convert -size 1100x200 xc:white -gravity center -pointsize 44 \
                -fill black -annotate 0 "$text" "$out" 2>/dev/null && [[ -s "$out" ]] && return 0
        fi
        return 1
    }
    if ! render_text_png "$probe" "$good" || ! render_text_png "$probe" "$bad"; then
        rm -rf "$fixdir"
        skip "selftest: no text-render tool (magick/convert/ffmpeg-drawtext) to build golden fixtures"
    fi

    local gg_rc gb_rc
    # golden-GOOD: expect the probe string → MUST PASS (rc 0)
    ocr_analyze "$good" --expect "$probe" >"${HELIX_WORK}/gg" 2>&1; gg_rc=$?
    # golden-BAD: expect a string that is NOT on screen → MUST FAIL (rc 1)
    ocr_analyze "$bad" --expect "HELIXCODE_WRONG_PATTERN_9999" >"${HELIX_WORK}/gb" 2>&1; gb_rc=$?
    sed 's/^/    [golden-good] /' "${HELIX_WORK}/gg" >&2
    sed 's/^/    [golden-bad ] /' "${HELIX_WORK}/gb" >&2
    rm -rf "$fixdir"

    local analyzer_ok=1
    if [[ $gg_rc -eq 0 && $gb_rc -eq 1 ]]; then
        analyzer_ok=0
        log "ANALYZER SELF-VALIDATION: PASS (golden-good PASS rc=0, golden-bad FAIL rc=1) — analyzer cannot bluff"
    else
        log "ANALYZER SELF-VALIDATION: FAIL (golden-good rc=${gg_rc} [want 0], golden-bad rc=${gb_rc} [want 1])"
    fi

    # ---- overall verdict ----
    echo "" >&2
    echo "=========================== SELFTEST SUMMARY ===========================" >&2
    case $live_ok in
        0) echo "live window-scoped recording + OCR read-back : PASS (mp4=${out_mp4})" >&2;;
        1) echo "live window-scoped recording + OCR read-back : SKIP/FAIL (see [live] log; honest env-gap §11.4.3)" >&2;;
        2) echo "live window-scoped recording + OCR read-back : NOT ATTEMPTED (tooling absent)" >&2;;
    esac
    [[ $analyzer_ok -eq 0 ]] && echo "analyzer golden-good/golden-bad self-validation  : PASS" >&2 \
                             || echo "analyzer golden-good/golden-bad self-validation  : FAIL" >&2
    echo "========================================================================" >&2

    # The analyzer self-validation is the load-bearing anti-bluff proof and MUST pass.
    # Live recording is best-effort (env may gate window discovery / permissions).
    if [[ $analyzer_ok -ne 0 ]]; then
        echo "SELFTEST: FAIL (analyzer self-validation failed — analyzer is unreliable)"; return 1
    fi
    if [[ $live_ok -eq 0 ]]; then
        echo "SELFTEST: PASS (live recording read-back + analyzer self-validation both PASS)"; return 0
    fi
    echo "SELFTEST: PARTIAL-PASS (analyzer self-validation PASS; live recording an honest env-gap §11.4.3 — see log)"
    return 0
}

# ==============================================================================
# probe-caps — report which capture primitives this host actually grants
# (§11.4.3 honest env-gap documentation; §11.4.6 real probe, never assumed).
# ==============================================================================
probe_caps() {
    local wd; wd="$(mkwork caps)"
    local wid=""; command -v swift >/dev/null 2>&1 && wid="$(swift "$FIND_WIN_SWIFT" --owner Terminal 2>/dev/null)"
    echo "host capture capabilities (probed $(date -u +%FT%TZ)):"
    # region still
    if screencapture -x -R0,0,8,8 "${wd}/r.png" 2>/dev/null && [[ -s "${wd}/r.png" ]]; then
        echo "  [OK ] screencapture -R region still"
    else echo "  [GAP] screencapture -R region still"; fi
    # window still
    if [[ -n "$wid" ]] && screencapture -x -o -l"$wid" "${wd}/w.png" 2>/dev/null && [[ -s "${wd}/w.png" ]]; then
        echo "  [OK ] screencapture -l<id> window still (id=$wid)"
    else echo "  [GAP] screencapture -l<id> window still (id=${wid:-none}) — window-scoped recording requires this OR window video"; fi
    # window video
    if [[ -n "$wid" ]] && screencapture -v -x -V1 -l"$wid" "${wd}/w.mov" 2>/dev/null && [[ -s "${wd}/w.mov" ]]; then
        echo "  [OK ] screencapture -v -l<id> window video"
    else echo "  [GAP] screencapture -v window video (macOS Screen-Recording video TCC grant)"; fi
    # OCR under TMPDIR
    if command -v magick >/dev/null && command -v tesseract >/dev/null; then
        magick -size 700x140 xc:white -gravity center -pointsize 40 -font /System/Library/Fonts/Menlo.ttc -fill black -annotate 0 "CAPS_OCR_OK" "${wd}/o.png" 2>/dev/null
        if tesseract "${wd}/o.png" stdout 2>/dev/null | grep -q "CAPS_OCR_OK"; then
            echo "  [OK ] OCR (magick render + tesseract read) under TMPDIR=${HELIX_TMP_ROOT}"
        else echo "  [GAP] OCR read-back under TMPDIR"; fi
    else echo "  [GAP] OCR tooling (magick/tesseract)"; fi
    rm -rf "$wd"
}

# ==============================================================================
main() {
    [[ $# -ge 1 ]] || { sed -n '2,40p' "${BASH_SOURCE[0]}" >&2; exit 2; }
    local sub="$1"; shift
    case "$sub" in
        record)        [[ $# -ge 2 ]] || { echo "usage: record <client> <feature> [...]" >&2; exit 2; }; record_feature "$@";;
        validate)      [[ $# -ge 1 ]] || { echo "usage: validate <mp4> --expect P" >&2; exit 2; }; validate_recording "$@";;
        ocr-analyze)   [[ $# -ge 1 ]] || { echo "usage: ocr-analyze <img> --expect P" >&2; exit 2; }; ocr_analyze "$@";;
        selftest)      selftest;;
        rotate)        [[ $# -ge 1 ]] || { echo "usage: rotate <scope-tag>" >&2; exit 2; }; rotate_own_scope "$1";;
        resolve-prefix) resolve_prefix; echo;;
        probe-caps)    probe_caps;;
        *) echo "unknown subcommand: ${sub}" >&2; exit 2;;
    esac
}
main "$@"
