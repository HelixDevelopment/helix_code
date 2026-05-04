#!/usr/bin/env bash
# scripts/verify-llmsverifier-pin-parity.sh
# Fail if Dependencies/HelixDevelopment/LLMsVerifier and HelixAgent/LLMsVerifier
# point at different SHAs. Wired into make ci-validate-all.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

CANONICAL_PATH="Dependencies/HelixDevelopment/LLMsVerifier"
TRANSITIVE_PATH="HelixAgent/LLMsVerifier"

if [ ! -d "$CANONICAL_PATH/.git" ] && [ ! -f "$CANONICAL_PATH/.git" ]; then
  echo "ERROR: $CANONICAL_PATH not initialised. Run: git submodule update --init --recursive" >&2
  exit 2
fi

if [ ! -d "$TRANSITIVE_PATH/.git" ] && [ ! -f "$TRANSITIVE_PATH/.git" ]; then
  echo "ERROR: $TRANSITIVE_PATH not initialised. Did P0-03 (add HelixAgent) run?" >&2
  exit 2
fi

CANONICAL_SHA=$(git -C "$CANONICAL_PATH" rev-parse HEAD)
TRANSITIVE_SHA=$(git -C "$TRANSITIVE_PATH" rev-parse HEAD)

if [ "$CANONICAL_SHA" = "$TRANSITIVE_SHA" ]; then
  echo "OK: LLMsVerifier pin parity — both at $CANONICAL_SHA"
  exit 0
fi

echo "FAIL: LLMsVerifier pin divergence" >&2
echo "  $CANONICAL_PATH  → $CANONICAL_SHA" >&2
echo "  $TRANSITIVE_PATH → $TRANSITIVE_SHA" >&2
echo "" >&2
echo "Resolution: pick the canonical SHA, bump the other to match, commit, push." >&2
exit 1
