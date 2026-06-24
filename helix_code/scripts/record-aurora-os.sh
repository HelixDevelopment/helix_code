#!/usr/bin/env bash
# helix_code/scripts/record-aurora-os.sh
#
# §11.4.81 cross-platform-parity recording runner for the HelixCode Aurora OS client.
# Companion to scripts/run-challenge-matrix.sh (Android). Per-OS/SDK dispatch:
#   - Aurora OS SDK present (sfdk/mb2) → build the native RPM, deploy to the Aurora
#     emulator/device, launch, and record the client running on the REAL Aurora target.
#   - SDK absent → honest SKIP-with-reason (exit 77, §11.4.3), NEVER a fake recording
#     and NEVER a Fyne-on-desktop proxy (§11.4.143 — a desktop build is NOT the Aurora
#     target). Cited evidence: docs/qa/HXC-108_remaining_platforms_evidence.md +
#     docs/research/aurora_harmony_recording_paths_20260623/feasibility.md.
#
# RATIONALE (§11.4.98(B) honest bootstrap): no host currently has the Aurora SDK, so this
# runner SKIPs everywhere today — but it is the sanctioned, ready entry point the moment an
# operator installs Aurora OS SDK 4.0+ (sfdk/mb2 + the Aurora emulator) per
# applications/aurora_os/README.md. This mechanizes the honest gap instead of leaving it
# undocumented in prose only.
#
# EXIT CODES:
#   0   recording produced (Aurora target).
#   2   invalid arguments / preflight failure.
#   3   build/deploy/record failure on a present-SDK host.
#   77  honest SKIP (Aurora SDK absent) — distinct from PASS so a caller cannot misread it.
#
# USAGE: record-aurora-os.sh [--out-dir DIR] [--prefix NAME] [-h|--help]

set -euo pipefail

OUT_DIR="${HOME}/helixcode-recordings"
PREFIX="${HELIX_RELEASE_PREFIX:-helixcode}"

log()  { printf '[record-aurora-os] %s\n' "$*" >&2; }
die()  { printf '[record-aurora-os] ERROR: %s\n' "$*" >&2; exit "${2:-2}"; }
skip() { printf '[record-aurora-os] SKIP: %s\n' "$*" >&2; exit 77; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out-dir) OUT_DIR="${2:?}"; shift 2 ;;
    --prefix)  PREFIX="${2:?}"; shift 2 ;;
    -h|--help) grep -E '^#' "$0" | sed -E 's/^# ?//' | sed -n '1,40p'; exit 0 ;;
    *) die "unknown option '$1'" ;;
  esac
done

# §11.4.81 SDK detection — Aurora/Sailfish toolchain.
missing=""
for tool in sfdk mb2; do
  command -v "$tool" >/dev/null 2>&1 || missing="${missing} ${tool}"
done
if [[ -n "$missing" ]]; then
  skip "Aurora OS SDK not installed (missing:${missing}). Recording the client on the \
real Aurora target requires Aurora OS SDK 4.0+ (sfdk/mb2 + the Aurora emulator) per \
applications/aurora_os/README.md — §11.4.98(B) operator bootstrap. A Fyne desktop build \
is NOT a valid Aurora proxy (§11.4.143). Cited: \
<repo-root>/docs/research/aurora_harmony_recording_paths_20260623/feasibility.md."
fi

# --- SDK present: build the native RPM + record on the Aurora target. ---
log "Aurora OS SDK detected; building + recording the client on the Aurora target ..."
mkdir -p "$OUT_DIR"
TS="$(date +%Y%m%d-%H%M%S)"
REC_HOST="${OUT_DIR}/${PREFIX}-aurora-client-${TS}.mp4"

APP_DIR="$(cd "$(dirname "$0")/../applications/aurora_os" && pwd)"
[[ -d "$APP_DIR" ]] || die "aurora_os application dir not found at $APP_DIR" 2

# mb2 native build (armv7hl) per the Aurora SDK toolchain.
( cd "$APP_DIR" && mb2 -t AuroraOS-armv7hl build ) || die "mb2 Aurora build failed" 3
# Deploy + launch + record via the Aurora emulator/device tooling (sfdk).
# (The concrete sfdk deploy/run/record incantation is environment-specific; surfaced as a
#  hard failure rather than a fake success if it does not produce a real recording.)
log "deploying + recording on the Aurora target ..."
sfdk device exec true >/dev/null 2>&1 || die "no Aurora device/emulator reachable via sfdk" 3
# A real implementation records the on-target screen here; absence of a produced file is a
# failure, never a faked PASS (§11.4 anti-bluff).
[[ -f "$REC_HOST" ]] || die "Aurora recording was not produced (no fake PASS — §11.4)" 3
log "DONE"
printf 'RECORDING=%s\n' "$REC_HOST"
exit 0
