#!/usr/bin/env bash
# mistborn-down.sh — tear down the mistborn distribution stack
# Drops SSH tunnels, stops + removes containers on mistborn.local.
set -euo pipefail
REMOTE_USER="milosvasic"
REMOTE_HOST="mistborn.local"

echo "=== mistborn-down: stopping local SSH tunnels"
pkill -f "ssh.*${REMOTE_HOST}.*-L 15432" 2>/dev/null && echo "    tunnel killed" || echo "    no tunnel found"

echo "=== mistborn-down: stopping remote containers"
ssh "${REMOTE_USER}@${REMOTE_HOST}" \
    "export PATH=/opt/homebrew/bin:\$PATH && cd /tmp && podman-compose -f docker-compose.mistborn.yml down 2>&1 | tail -3"

echo "=== mistborn-down: done"
