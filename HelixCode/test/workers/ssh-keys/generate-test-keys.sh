#!/usr/bin/env bash
# HelixCode/test/workers/ssh-keys/generate-test-keys.sh
# Generates fresh ephemeral SSH keys for test workers. The previous tracked
# keys (id_rsa, id_rsa.pub) were a CONST-041 violation discovered in P0-08
# and remediated in P0-T08.5 (2026-05-04); their material lives forever in
# git history and must be considered compromised — they are deliberately NOT
# regenerated here (different algorithm: ed25519 vs the original RSA).
#
# Any production or CI system that erroneously trusted the leaked public key
# MUST reject it and replace it with the output of this script.
#
# Run before any test that needs SSH keys; idempotent (skips if keys exist).
# Usage: bash HelixCode/test/workers/ssh-keys/generate-test-keys.sh

set -euo pipefail
cd "$(dirname "$0")"

if [ -f id_rsa ] && [ -f id_rsa.pub ]; then
  echo "OK: ephemeral SSH keys already exist at $(pwd)"
  exit 0
fi

ssh-keygen -t ed25519 -f id_rsa -N "" -C "helixcode-test-ephemeral-$(date -Iseconds)"
chmod 600 id_rsa
chmod 644 id_rsa.pub
cp id_rsa.pub authorized_keys

echo "OK: generated ephemeral test SSH keys at $(pwd)"
echo "These keys are gitignored. NEVER commit them. CONST-041 absolute."
