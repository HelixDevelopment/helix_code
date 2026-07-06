#!/usr/bin/env bash
# run.sh — full reproducible Tesseract-OCR-in-rootless-podman + golden self-validation.
# Rootless (no sudo). CPU-only. Produces real OCR evidence under evidence/.
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="localhost/helix-ocr:tesseract5"
CTR="helix-ocr-run"
HOST_PORT="${HOST_PORT:-18080}"

echo "== 1. build image (rootless) =="
podman build -f Containerfile.tesseract -t "$IMAGE" .

echo "== 2. capture tesseract --version from inside the container =="
podman run --rm "$IMAGE" sh -c 'id; tesseract --version; tesseract --list-langs' \
    | tee evidence/container_tesseract_version.txt

echo "== 3. generate golden fixtures INSIDE the container (pinned DejaVu font + PIL) =="
# --user 0 => container-root, which maps to the invoking host UID under rootless
# podman, so it can write the host-owned fixtures/ dir. Still unprivileged on host.
podman run --rm --user 0 -v "$PWD":/work:z -w /work "$IMAGE" python3 gen_fixtures.py

echo "== 4. boot the /v1/ocr service (detached, rootless) =="
podman rm -f "$CTR" >/dev/null 2>&1 || true
podman run -d --name "$CTR" -p "${HOST_PORT}:8080" "$IMAGE"

echo "== 5. golden self-validation (golden-good must read tokens; golden-bad must not) =="
OCR_BASE_URL="http://127.0.0.1:${HOST_PORT}" python3 validate.py
rc=$?

echo "== 6. teardown =="
podman rm -f "$CTR" >/dev/null 2>&1 || true
exit $rc
