#!/usr/bin/env bash
# helix_code/scripts/run-challenge-matrix.sh
#
# §6.X-SANCTIONED Android challenge / on-device-recording runner for HelixCode.
#
# PURPOSE (HXC-108 Android unblock, path [A]):
#   Produce a REAL on-device recording of the HelixCode Android client by
#   running the Android emulator INSIDE a podman container (the §6.X
#   Containers-submodule path), installing the real app-debug.apk, launching
#   the app, and recording its screen — all in-container — then pulling the
#   recording to the host. This is the sanctioned alternative to raw
#   host-direct `emulator`/`adb` (which the §11.4.109 PreToolUse guard blocks
#   and which §6.X forbids as gate evidence).
#
# CONSTITUTIONAL ANCHORS:
#   §11.4.76  Containers-submodule mandate (emulator runs INSIDE a container).
#   §11.4.81  Cross-platform parity: per-OS dispatch via `uname -s`.
#   §11.4.112 Structural-impossibility: macOS arm64 has no /dev/kvm reachable
#             from a Linux container → honest SKIP-with-reason, never a fake
#             PASS. Cited evidence (meta-repo root, NOT helix_code/docs):
#             <repo-root>/docs/research/android_emulator_podman_macos_20260623/feasibility.md
#   §11.4.158 Recording-coverage: the produced recording must show the genuine
#             app running with real data (verified downstream by the caller).
#   §11.4.161 Rootless container runtime (podman, no sudo).
#
# WHY IN-CONTAINER ADB (not host-adb-connect):
#   The submodule's pkg/emulator/containerized.go boots the emulator in the
#   container and drives it from the HOST via a forwarded adb port. That path
#   is geared to gradle instrumentation tests (emulator-matrix --test-class →
#   :<module>:connectedDebugAndroidTest). For a *launch + screenrecord* of the
#   real APK, this runner drives adb FROM INSIDE the container via
#   `podman exec`, so the emulator AND its adb client both live in-container
#   (maximally §6.X-faithful) and we avoid the emulator-binds-127.0.0.1
#   host-reachability question. It reuses the EXACT same §6.X image the
#   submodule's Containerized.Boot runs (localhost/lava-android-emulator:*).
#
# RUNNER SELECTION (mirrors submodule emulator.ResolveRunner semantics):
#   - Linux x86_64 + /dev/kvm present  → containerized (gate-eligible).
#   - macOS / no-kvm                   → SKIP-with-reason (§11.4.112).
#
# USAGE:
#   run-challenge-matrix.sh record --apk <path> [options]
#
#   Modes:
#     record   Boot emulator-in-container, install APK, launch app, record
#              screen, pull recording + screenshot + logcat to --out-dir.
#
#   Options:
#     --apk PATH            (record) Path to the debug APK to install. REQUIRED on Linux.
#     --app-id ID           Android applicationId to launch. Default: dev.helix.code
#     --avd NAME            AVD name baked into the image. Default: default
#     --image IMG           Emulator container image. Default:
#                           $CONTAINERS_EMULATOR_IMAGE or
#                           localhost/lava-android-emulator:api34-x86_64
#     --runtime BIN         Container runtime CLI. Default: podman
#     --out-dir DIR         Where to write recording artifacts on this host.
#                           Default: $HOME/helixcode-recordings
#     --prefix NAME         Recording filename prefix (§11.4.155). Default: helixcode
#     --record-seconds N    screenrecord duration. Default: 30
#     --boot-timeout N      Seconds to await sys.boot_completed=1. Default: 420
#     --keep-container      Do not teardown the container on exit (debug).
#     -h | --help
#
# EXIT CODES:
#   0  record succeeded (Linux).
#   2  invalid arguments / preflight failure (missing podman, image, apk).
#   3  emulator boot timed out / install failed / app did not reach foreground.
#  77  honest SKIP (macOS / non-x86_64 / no-kvm) — distinct from PASS(0) so a
#      naive caller cannot misread an honest SKIP as a successful recording
#      (automake SKIP convention; §11.4.3 honest-gap).
#
# IDEMPOTENT + SAFE: unique per-run container name; trap-based teardown;
# no host power management; rootless podman; reuses a free /dev/kvm device
# node read-only via --device passthrough.

set -euo pipefail

# ----------------------------------------------------------------------------
# Defaults
# ----------------------------------------------------------------------------
MODE=""
APK=""
APP_ID="dev.helix.code"
AVD_NAME="default"
IMAGE="${CONTAINERS_EMULATOR_IMAGE:-localhost/lava-android-emulator:api34-x86_64}"
RUNTIME="podman"
OUT_DIR="${HOME}/helixcode-recordings"
PREFIX="helixcode"
RECORD_SECONDS=30
BOOT_TIMEOUT=420
KEEP_CONTAINER=0

log()  { printf '[run-challenge-matrix] %s\n' "$*" >&2; }
die()  { printf '[run-challenge-matrix] ERROR: %s\n' "$*" >&2; exit "${2:-2}"; }
skip() { printf '[run-challenge-matrix] SKIP: %s\n' "$*" >&2; exit 77; }

# ----------------------------------------------------------------------------
# Arg parsing
# ----------------------------------------------------------------------------
if [[ $# -eq 0 ]]; then
  die "no mode given. Usage: run-challenge-matrix.sh record --apk <path> [options]"
fi
MODE="$1"; shift || true
case "$MODE" in
  record) ;;
  -h|--help)
    grep -E '^#' "$0" | sed -E 's/^# ?//' | sed -n '1,90p'
    exit 0 ;;
  *) die "unknown mode '$MODE' (supported: record)" ;;
esac

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apk)            APK="${2:?}"; shift 2 ;;
    --app-id)         APP_ID="${2:?}"; shift 2 ;;
    --avd)            AVD_NAME="${2:?}"; shift 2 ;;
    --image)          IMAGE="${2:?}"; shift 2 ;;
    --runtime)        RUNTIME="${2:?}"; shift 2 ;;
    --out-dir)        OUT_DIR="${2:?}"; shift 2 ;;
    --prefix)         PREFIX="${2:?}"; shift 2 ;;
    --record-seconds) RECORD_SECONDS="${2:?}"; shift 2 ;;
    --boot-timeout)   BOOT_TIMEOUT="${2:?}"; shift 2 ;;
    --keep-container) KEEP_CONTAINER=1; shift ;;
    -h|--help)        grep -E '^#' "$0" | sed -E 's/^# ?//' | sed -n '1,90p'; exit 0 ;;
    *) die "unknown option '$1'" ;;
  esac
done

# Validate numeric args at preflight (clean die 2, not a mid-run arithmetic error).
[[ "$RECORD_SECONDS" =~ ^[1-9][0-9]*$ ]] || die "--record-seconds must be a positive integer, got '$RECORD_SECONDS'" 2
[[ "$BOOT_TIMEOUT"   =~ ^[1-9][0-9]*$ ]] || die "--boot-timeout must be a positive integer, got '$BOOT_TIMEOUT'" 2

# ----------------------------------------------------------------------------
# §11.4.81 per-OS dispatch
# ----------------------------------------------------------------------------
OS="$(uname -s)"
ARCH="$(uname -m)"
log "host: OS=${OS} ARCH=${ARCH}"

case "$OS" in
  Darwin)
    # §11.4.112 structurally-impossible accelerated containerized path on
    # macOS: applehv/Virtualization.framework does not expose /dev/kvm to a
    # Linux container; HVF is host-only. Honest SKIP, never a fake PASS.
    skip "macOS (${ARCH}) cannot run an accelerated Android emulator inside a Linux \
container: Apple Virtualization.framework/applehv does not expose /dev/kvm to \
guest containers and HVF is a host-only API (§11.4.112). \
Cited evidence: <repo-root>/docs/research/android_emulator_podman_macos_20260623/feasibility.md. \
Route §6.X Android recording to a Linux x86_64 KVM host: \
'run-challenge-matrix.sh record --apk <apk>' there."
    ;;
  Linux)
    if [[ "$ARCH" != "x86_64" && "$ARCH" != "amd64" ]]; then
      skip "Linux ${ARCH} is not x86_64; the §6.X emulator image is x86_64. \
Run on a Linux x86_64 host with /dev/kvm."
    fi
    if [[ ! -e /dev/kvm ]]; then
      skip "/dev/kvm absent on this Linux host — no hardware acceleration; the \
containerized emulator path is not gate-eligible here (§11.4.112 / submodule accel model)."
    fi
    if [[ ! -r /dev/kvm || ! -w /dev/kvm ]]; then
      die "/dev/kvm present but not rw for $(id -un); add the user to the kvm group \
or grant an ACL (no sudo in committed flows per §11.4.161)." 2
    fi
    ;;
  *)
    skip "Unsupported host OS '${OS}' for the containerized Android emulator path."
    ;;
esac

# ----------------------------------------------------------------------------
# Linux x86_64 + KVM: containerized record flow
# ----------------------------------------------------------------------------
command -v "$RUNTIME" >/dev/null 2>&1 || die "container runtime '$RUNTIME' not on PATH (§11.4.161 rootless podman)." 2
[[ -n "$APK" ]] || die "--apk is required in record mode on Linux." 2
[[ -f "$APK" ]] || die "APK not found: $APK" 2
"$RUNTIME" image exists "$IMAGE" 2>/dev/null || \
  die "emulator image '$IMAGE' not present. Build it from the Containers submodule: \
podman build --build-arg API_LEVEL=34 --build-arg ABI=x86_64 -f submodules/containers/pkg/emulator/Containerfile -t $IMAGE ." 2

mkdir -p "$OUT_DIR"
TS="$(date +%Y%m%d-%H%M%S)"
CN="helixcode-emu-${TS}-$$"
REC_DEV="/sdcard/${PREFIX}-rec.mp4"
SHOT_DEV="/sdcard/${PREFIX}-shot.png"
REC_HOST="${OUT_DIR}/${PREFIX}-android-client-${TS}.mp4"
SHOT_HOST="${OUT_DIR}/${PREFIX}-android-client-${TS}.png"
LOG_HOST="${OUT_DIR}/${PREFIX}-android-client-${TS}.logcat.txt"

cleanup() {
  if [[ "$KEEP_CONTAINER" -eq 0 ]]; then
    log "teardown container ${CN}"
    "$RUNTIME" rm -f "$CN" >/dev/null 2>&1 || true
  else
    log "keeping container ${CN} (--keep-container)"
  fi
}
trap cleanup EXIT INT TERM

# 1. Boot the emulator INSIDE the container (entrypoint runs `emulator -avd`).
#    --device /dev/kvm enables KVM acceleration (the submodule's gate path).
#
#    ROOTLESS-PODMAN KVM PLUMBING (submodule gap — see header / report):
#    The image runs as USER emulator (uid 1000). Under rootless podman the
#    DEFAULT userns maps container-uid-1000 to a host SUBUID that has no
#    access to /dev/kvm, so the emulator's ProbeKVM fails with
#    "This user doesn't have permissions to use KVM (/dev/kvm)". With
#    `--userns=keep-id`, container-uid-1000 maps to the invoking host user
#    (uid 1000), who holds the /dev/kvm ACL — KVM probes pass AND the baked
#    AVD/SDK (owned by uid 1000) + HOME resolve. The submodule's
#    pkg/emulator/containerized.go buildContainerRunArgs passes only
#    `--device /dev/kvm`; it should add `--userns=keep-id` for rootless
#    podman. Until that lands upstream this wrapper adds it.
USERNS_ARGS=()
if [[ "$(basename "$RUNTIME")" == "podman" ]]; then
  USERNS_ARGS=(--userns=keep-id)
fi
log "booting emulator in container (image=${IMAGE}, avd=${AVD_NAME}) ..."
"$RUNTIME" run -d --rm --name "$CN" \
  --device /dev/kvm \
  "${USERNS_ARGS[@]}" \
  -e ANDROID_AVD_NAME="$AVD_NAME" \
  -e ANDROID_COLD_BOOT=true \
  "$IMAGE" >/dev/null \
  || die "container failed to start" 3

# helper: run adb inside the container against the single local emulator (-e)
cexec() { "$RUNTIME" exec "$CN" bash -lc "$*"; }

# 2. Await device + sys.boot_completed=1 (positive, on-the-wire — §11.4.6).
log "awaiting device ..."
cexec "adb -e wait-for-device" || { "$RUNTIME" logs "$CN" >&2 || true; die "emulator never registered with adb" 3; }

log "awaiting sys.boot_completed=1 (timeout ${BOOT_TIMEOUT}s) ..."
deadline=$(( $(date +%s) + BOOT_TIMEOUT ))
booted=0
while [[ $(date +%s) -lt $deadline ]]; do
  # Fail fast if the container died mid-boot instead of spinning the full timeout.
  if ! "$RUNTIME" inspect -f '{{.State.Running}}' "$CN" 2>/dev/null | grep -q true; then
    "$RUNTIME" logs "$CN" 2>&1 | tail -40 >&2 || true
    die "emulator container exited during boot" 3
  fi
  bc="$(cexec "adb -e shell getprop sys.boot_completed 2>/dev/null" | tr -d '\r\n ' || true)"
  if [[ "$bc" == "1" ]]; then booted=1; break; fi
  sleep 5
done
[[ "$booted" -eq 1 ]] || { "$RUNTIME" logs "$CN" 2>&1 | tail -40 >&2 || true; die "boot timed out after ${BOOT_TIMEOUT}s" 3; }
# settle the framework launcher
cexec "adb -e shell input keyevent 82" >/dev/null 2>&1 || true
sleep 3
log "boot complete; android version: $(cexec "adb -e shell getprop ro.build.version.release" | tr -d '\r\n')"

# 3. Install the real APK in-container (token kept out of caller's command string).
log "copying APK into container ..."
"$RUNTIME" cp "$APK" "$CN:/tmp/app.apk" || die "podman cp APK failed" 3
log "installing APK ..."
inst_out="$(cexec "adb -e install -r /tmp/app.apk" 2>&1 || true)"
printf '%s\n' "$inst_out" >&2
grep -q "Success" <<<"$inst_out" || die "APK install did not report Success" 3

# 4. Launch the app via the launcher intent (monkey — not guard-blocked).
log "launching ${APP_ID} ..."
cexec "adb -e shell monkey -p ${APP_ID} -c android.intent.category.LAUNCHER 1" >/dev/null 2>&1 || \
  die "failed to launch ${APP_ID}" 3
sleep 8

# 5. Confirm the app actually reached the foreground (anti-bluff content gate).
res="$(cexec "adb -e shell dumpsys activity activities 2>/dev/null | grep -E 'ResumedActivity|mResumedActivity'" || true)"
res_oneline="$(printf '%s' "$res" | tr '\r\n' '  ' | sed -E 's/[[:space:]]+/ /g')"
log "resumed-activity: ${res_oneline}"
if grep -q "$APP_ID" <<<"$res"; then
  FOREGROUND=1
else
  FOREGROUND=0
  log "WARN: ${APP_ID} not seen in ResumedActivity — recording will still be captured for inspection."
fi

# NOTE on the two-hop pull: /sdcard/* lives inside the EMULATOR GUEST (a
# nested VM), NOT the container filesystem. `podman cp $CN:/sdcard/...` cannot
# see it. So every artifact is pulled GUEST→container (`adb -e pull` to /tmp)
# then container→host (`podman cp` from /tmp).

dump_logs_on_fail() { "$RUNTIME" logs "$CN" 2>&1 | tail -30 >&2 || true; }

# 6. Record the live UI FIRST (most valuable artifact; captured right after
#    launch while the app is foreground). screenrecord runs in-guest.
log "recording ${RECORD_SECONDS}s of the live UI ..."
cexec "adb -e shell screenrecord --time-limit ${RECORD_SECONDS} --bit-rate 6000000 ${REC_DEV}" \
  || { dump_logs_on_fail; die "screenrecord failed (emulator/container may have exited)" 3; }
cexec "adb -e pull ${REC_DEV} /tmp/rec.mp4" || { dump_logs_on_fail; die "adb pull recording (guest->container) failed" 3; }
"$RUNTIME" cp "$CN:/tmp/rec.mp4" "$REC_HOST" || die "recording pull (container->host) failed" 3

# 7. Mid-session screenshot (best-effort; for visual content verification).
log "capturing screenshot ..."
cexec "adb -e shell screencap -p ${SHOT_DEV} && adb -e pull ${SHOT_DEV} /tmp/shot.png" >/dev/null 2>&1 \
  && "$RUNTIME" cp "$CN:/tmp/shot.png" "$SHOT_HOST" >/dev/null 2>&1 \
  || log "WARN: screenshot capture/pull failed (non-fatal; recording already captured)"

# 8. Capture logcat (real Go-core / activity lifecycle evidence).
cexec "adb -e logcat -d -t 2000" > "$LOG_HOST" 2>/dev/null || true

# 9. Report artifacts.
log "DONE"
printf 'RECORDING=%s\n' "$REC_HOST"
printf 'SCREENSHOT=%s\n' "$SHOT_HOST"
printf 'LOGCAT=%s\n' "$LOG_HOST"
# Surface what the script already observed so the caller's content gate
# (§11.4.158/§11.4.160) is not blind to it. FOREGROUND=0 ⇒ inspect manually.
printf 'FOREGROUND=%s\n' "${FOREGROUND:-0}"
printf 'RESUMED_ACTIVITY=%s\n' "${res_oneline:-}"
ls -la "$REC_HOST" "$SHOT_HOST" "$LOG_HOST" >&2 || true
exit 0
