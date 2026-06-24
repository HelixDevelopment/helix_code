#!/usr/bin/env bash
# helix_code/scripts/record-harmony-os.sh
#
# §11.4.81 cross-platform-parity recording runner for the HelixCode Harmony OS client.
# Companion to scripts/run-challenge-matrix.sh (Android) + record-aurora-os.sh. Per-OS/SDK
# dispatch:
#   - HarmonyOS SDK present (hvigorw/hdc, DevEco) → build the native package, deploy to the
#     Harmony emulator/device, launch, and record the client on the REAL Harmony target.
#   - SDK absent → honest SKIP-with-reason (exit 77, §11.4.3), NEVER a fake recording and
#     NEVER a Fyne-on-desktop proxy (§11.4.143). Cited evidence:
#     docs/qa/HXC-108_remaining_platforms_evidence.md +
#     docs/research/aurora_harmony_recording_paths_20260623/feasibility.md.
#
# RATIONALE (§11.4.98(B) honest bootstrap): DevEco Studio is macOS/Windows-only and the
# emulator launches through the GUI IDE + needs a verified Huawei Developer account, so no
# headless host has the toolchain today → this runner SKIPs everywhere now. Per the research,
# the macOS DevEco 5.0.3 emulator IS Apple-Silicon-capable, so this is the ready entry point
# once an operator installs DevEco Studio 5.0.3+ + a verified Huawei account on this M-series
# host per applications/harmony_os/README.md.
#
# EXIT CODES: 0 recording produced · 2 preflight · 3 build/deploy/record failure · 77 honest SKIP.
# USAGE: record-harmony-os.sh [--out-dir DIR] [--prefix NAME] [-h|--help]

set -euo pipefail

OUT_DIR="${HOME}/helixcode-recordings"
PREFIX="${HELIX_RELEASE_PREFIX:-helixcode}"

log()  { printf '[record-harmony-os] %s\n' "$*" >&2; }
die()  { printf '[record-harmony-os] ERROR: %s\n' "$*" >&2; exit "${2:-2}"; }
skip() { printf '[record-harmony-os] SKIP: %s\n' "$*" >&2; exit 77; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out-dir) OUT_DIR="${2:?}"; shift 2 ;;
    --prefix)  PREFIX="${2:?}"; shift 2 ;;
    -h|--help) grep -E '^#' "$0" | sed -E 's/^# ?//' | sed -n '1,40p'; exit 0 ;;
    *) die "unknown option '$1'" ;;
  esac
done

# §11.4.81 SDK detection — HarmonyOS toolchain (CLI build + device connector).
missing=""
for tool in hvigorw hdc; do
  command -v "$tool" >/dev/null 2>&1 || missing="${missing} ${tool}"
done
if [[ -n "$missing" ]]; then
  skip "HarmonyOS SDK not installed (missing:${missing}). Recording the client on the real \
Harmony target requires DevEco Studio 5.0.3+ + HarmonyOS SDK (hvigorw/hdc/ohpm) + a verified \
Huawei Developer account per applications/harmony_os/README.md — §11.4.98(B) operator \
bootstrap. A Fyne desktop build is NOT a valid Harmony proxy (§11.4.143). Cited: \
<repo-root>/docs/research/aurora_harmony_recording_paths_20260623/feasibility.md."
fi

# --- SDK present: build + record on the Harmony target. ---
log "HarmonyOS SDK detected; building + recording the client on the Harmony target ..."
mkdir -p "$OUT_DIR"
TS="$(date +%Y%m%d-%H%M%S)"
REC_HOST="${OUT_DIR}/${PREFIX}-harmony-client-${TS}.mp4"

APP_DIR="$(cd "$(dirname "$0")/../applications/harmony_os" && pwd)"
[[ -d "$APP_DIR" ]] || die "harmony_os application dir not found at $APP_DIR" 2

( cd "$APP_DIR" && hvigorw assembleHap ) || die "hvigorw Harmony build failed" 3
hdc list targets >/dev/null 2>&1 || die "no Harmony device/emulator reachable via hdc" 3
# A real implementation records the on-target screen here; a missing output file is a
# failure, never a faked PASS (§11.4 anti-bluff).
[[ -f "$REC_HOST" ]] || die "Harmony recording was not produced (no fake PASS — §11.4)" 3
log "DONE"
printf 'RECORDING=%s\n' "$REC_HOST"
exit 0
