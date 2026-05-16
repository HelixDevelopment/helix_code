#!/usr/bin/env bash
# scripts/verify-llmsverifier-pin-parity.sh
#
# Historical note (P1.5-WP2): the duplicate transitive submodule
# helix_agent/LLMsVerifier was eliminated in WP2 — there is now exactly
# ONE LLMsVerifier checkout (dependencies/HelixDevelopment/LLMsVerifier).
# The pin-parity gate is therefore degenerate. We retain this script as
# a no-op pass so the make ci-validate-all wiring stays intact and any
# future re-introduction of a duplicate is immediately visible.
#
# Wired into make ci-validate-all.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

CANONICAL_PATH="dependencies/HelixDevelopment/LLMsVerifier"
LEGACY_TRANSITIVE_PATH="helix_agent/LLMsVerifier"

if [ ! -d "$CANONICAL_PATH/.git" ] && [ ! -f "$CANONICAL_PATH/.git" ]; then
  echo "ERROR: $CANONICAL_PATH not initialised. Run: git submodule update --init --recursive" >&2
  exit 2
fi

if [ -e "$LEGACY_TRANSITIVE_PATH" ]; then
  echo "FAIL: legacy duplicate $LEGACY_TRANSITIVE_PATH re-appeared" >&2
  echo "  WP2 removed this submodule. Re-introduction requires a parity check restoration." >&2
  exit 1
fi

CANONICAL_SHA=$(git -C "$CANONICAL_PATH" rev-parse HEAD)
echo "OK: LLMsVerifier single canonical pin at $CANONICAL_SHA (transitive duplicate eliminated in P1.5-WP2)"
exit 0
