#!/usr/bin/env bash
# mistborn-up.sh — boot the distributed-service stack on mistborn.local
# and establish SSH tunnels so the developer host can use the services
# as if they were local.
#
# Per operator mandate 2026-05-13: every container EXCEPT HelixAgent +
# HelixCode runs on mistborn.local.
#
# Prerequisites:
#   - Passwordless SSH to milosvasic@mistborn.local (id_ed25519 in agent).
#   - Podman 5.x + podman-compose on mistborn (verified 5.8.2 / applehv).
#   - No process bound to local ports 15432 / 16379 / 11434 / 16333 /
#     18000 / 18087 / 19090 / 13001 (alternate ports leave space for any
#     local Podman containers already on the canonical ports).
#
# Usage: bash scripts/mistborn-up.sh
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REMOTE_USER="milosvasic"
REMOTE_HOST="mistborn.local"
REMOTE_TMP="/tmp/docker-compose.mistborn.yml"
LOCAL_COMPOSE="${REPO_ROOT}/docs/distribution/docker-compose.mistborn.yml"

echo "=== mistborn-up: distributing services to ${REMOTE_HOST} ==="
echo "[1/4] SCP compose file"
scp -q "${LOCAL_COMPOSE}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_TMP}"

echo "[2/4] podman-compose up -d on remote"
ssh "${REMOTE_USER}@${REMOTE_HOST}" \
    "export PATH=/opt/homebrew/bin:\$PATH && cd /tmp && podman-compose -f docker-compose.mistborn.yml up -d 2>&1 | tail -5"

echo "[3/4] Wait for healthchecks (max 60 s)"
for _ in $(seq 1 12); do
    unhealthy=$(ssh "${REMOTE_USER}@${REMOTE_HOST}" \
        "export PATH=/opt/homebrew/bin:\$PATH && podman ps --format '{{.Names}}\t{{.Status}}' | grep -E '(starting|unhealthy)' | wc -l | tr -d ' '")
    if [[ "${unhealthy}" == "0" ]]; then
        echo "    all containers healthy"
        break
    fi
    echo "    ${unhealthy} container(s) still starting; sleeping 5 s ..."
    sleep 5
done

echo "[4/4] Establishing SSH tunnels (background, ServerAliveInterval=30)"
pkill -f "ssh.*${REMOTE_HOST}.*-L 15432" 2>/dev/null || true
sleep 1
ssh -fN -o ServerAliveInterval=30 \
    -L 15432:127.0.0.1:5432 \
    -L 16379:127.0.0.1:6379 \
    -L 16333:127.0.0.1:6333 \
    -L 16334:127.0.0.1:6334 \
    -L 18000:127.0.0.1:8000 \
    -L 11434:127.0.0.1:11434 \
    -L 19090:127.0.0.1:9090 \
    -L 13000:127.0.0.1:3000 \
    "${REMOTE_USER}@${REMOTE_HOST}"

# Verify tunnel sockets bound
ss -tlnp 2>/dev/null | grep -qE "127\\.0\\.0\\.1:(15432|16379)" || {
    echo "FAIL: tunnel sockets not bound"
    exit 1
}

echo
echo "=== mistborn-up: ALL SERVICES UP AND TUNNELLED ==="
echo "  Local 127.0.0.1:15432 -> mistborn-postgres"
echo "  Local 127.0.0.1:16379 -> mistborn-redis"
echo "  Local 127.0.0.1:16333 -> mistborn-qdrant (HTTP)"
echo "  Local 127.0.0.1:16334 -> mistborn-qdrant (gRPC)"
echo "  Local 127.0.0.1:18000 -> mistborn-chromadb"
echo "  Local 127.0.0.1:11434 -> mistborn-ollama"
echo "  Local 127.0.0.1:19090 -> mistborn-prometheus"
echo "  Local 127.0.0.1:13000 -> mistborn-grafana"
echo
echo "Local HelixCode / HelixAgent connect to 127.0.0.1:<tunneled-port>"
echo "as if these services were running locally."
