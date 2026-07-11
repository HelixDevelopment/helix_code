#!/usr/bin/env bash
# scripts/boot_coder_cdi.sh
#
# Boot the rootless `helixllm-coder` container after regenerating the NVIDIA CDI
# spec. RUN AS ROOT:  sudo bash scripts/boot_coder_cdi.sh   [rootless-user]
#
# Root cause it fixes (§11.4.111, diagnosed 2026-07-11): the coder container's
# CDI GPU passthrough is pinned to the stale device index /dev/dri/card0, but the
# DRM node re-enumerated as card1 on the last reboot AND no CDI spec was generated
# (`nvidia-ctk cdi list` = 0 devices; /etc/cdi absent). Regenerating the CDI spec
# maps nvidia.com/gpu=all to the CURRENT devices (resolve-by-identity, not stale
# index), so `podman start` stops failing on the missing card0.
#
# Safety (§11.4.133 / §11.4.161): only regenerates a CDI descriptor file (root,
# reversible) and starts an EXISTING rootless container as its owner — no image
# rebuild, no destructive op, no host power op. Idempotent.
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: must run as root:  sudo bash $0 [rootless-user]" >&2
  exit 1
fi

# Rootless container owner: arg 1, else the sudo invoker, else 'milos'.
RUSER="${1:-${SUDO_USER:-milos}}"
if ! id "$RUSER" >/dev/null 2>&1; then
  echo "ERROR: rootless user '$RUSER' does not exist; pass it as arg 1." >&2
  exit 1
fi
RUID="$(id -u "$RUSER")"
echo "[1/5] rootless container owner = $RUSER (uid $RUID)"

echo "[2/5] regenerating NVIDIA CDI spec -> /etc/cdi/nvidia.yaml"
mkdir -p /etc/cdi
nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml

echo "[3/5] CDI devices now registered:"
nvidia-ctk cdi list || true

echo "[4/5] starting helixllm-coder as $RUSER (rootless podman)"
if ! sudo -u "$RUSER" \
      XDG_RUNTIME_DIR="/run/user/$RUID" \
      DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$RUID/bus" \
      podman start helixllm-coder; then
  echo "ERROR: podman start failed. If it STILL cites /dev/dri/card0, the container" >&2
  echo "carries a stale DIRECT device binding (not the CDI name) and must be recreated" >&2
  echo "from its Containerfile against nvidia.com/gpu=all. Check:" >&2
  echo "  sudo -u $RUSER podman inspect helixllm-coder --format '{{json .HostConfig.Devices}}'" >&2
  exit 1
fi

echo "[5/5] waiting for the coder to load the 30B model on :18434 (up to 600s)..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:18434/v1/models >/dev/null 2>&1; then
    echo "OK: coder is UP at :18434 (after ~$((i*10))s)"
    curl -s http://localhost:18434/v1/models 2>/dev/null | head -c 400; echo
    exit 0
  fi
  sleep 10
done
echo "WARN: coder not healthy on :18434 within 600s. Inspect: sudo -u $RUSER podman logs --tail 40 helixllm-coder" >&2
exit 1
