#!/usr/bin/env bash
# scripts/boot_coder_cdi.sh
#
# Boot the rootless `helixllm-coder` container after regenerating the NVIDIA CDI
# spec.  RUN AS THE CONTAINER OWNER (NO root / su / sudo needed):
#     bash scripts/boot_coder_cdi.sh
#
# Root cause it fixes (§11.4.111, diagnosed 2026-07-11): the coder container binds
# the GPU via the CDI name `nvidia.com/gpu=all`. Its CDI spec was pinned to the
# stale DRM index /dev/dri/card0, but that node re-enumerated as card1 on the last
# reboot AND no CDI spec was generated (`nvidia-ctk cdi list` = 0). We regenerate
# the spec to a USER-writable dir (~/.config/cdi) — resolve-by-identity, current
# devices — and point podman at it with CDI_SPEC_DIRS, so no /etc/cdi write (root)
# is ever required. Fully rootless (§11.4.161); idempotent; §11.4.133-safe (only
# writes a CDI descriptor + starts an existing container — no image rebuild, no
# destructive op, no host power op).
set -euo pipefail

CDI_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/cdi"
CONTAINER="${1:-helixllm-coder}"
HEALTH_URL="${2:-http://localhost:18434/v1/models}"

echo "[1/4] regenerating NVIDIA CDI spec (rootless) -> $CDI_DIR/nvidia.yaml"
mkdir -p "$CDI_DIR"
nvidia-ctk cdi generate --output="$CDI_DIR/nvidia.yaml"
# Sanity: the fresh spec must reference the CURRENT DRM node, not a stale card0.
if grep -q "/dev/dri/card0" "$CDI_DIR/nvidia.yaml" && ! [ -e /dev/dri/card0 ]; then
  echo "WARN: fresh CDI spec still names /dev/dri/card0 which is absent — check /dev/dri" >&2
fi

echo "[2/4] starting rootless container '$CONTAINER' with CDI_SPEC_DIRS=$CDI_DIR"
CDI_SPEC_DIRS="$CDI_DIR" podman start "$CONTAINER"

echo "[3/4] container status:"
podman ps --format '  {{.Names}} {{.Status}}' | grep -F "$CONTAINER" || true

echo "[4/4] waiting for readiness at $HEALTH_URL (up to 600s)..."
for i in $(seq 1 60); do
  if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    echo "OK: '$CONTAINER' is UP and serving (after ~$((i*10))s)"
    curl -s "$HEALTH_URL" 2>/dev/null | head -c 300; echo
    exit 0
  fi
  sleep 10
done
echo "WARN: not healthy within 600s. Inspect: podman logs --tail 40 $CONTAINER" >&2
exit 1
