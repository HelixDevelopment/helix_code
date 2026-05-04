#!/usr/bin/env bash
# scripts/test-scan-secrets.sh
# Tests scan-secrets.sh: passes on a clean tree, fails on a planted secret.

set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

CLEAN_DIR=$(mktemp -d)
PLANT_DIR=$(mktemp -d)
trap 'rm -rf "$CLEAN_DIR" "$PLANT_DIR"' EXIT

# Put a benign file in the clean dir to make sure the scanner traverses
echo "FOO=bar" > "$CLEAN_DIR/safe.txt"

PASS=0
FAIL=0

# --- Test 1: clean directory should PASS (exit 0) ---
echo "TEST 1: clean directory → expect exit 0"
if scripts/scan-secrets.sh "$CLEAN_DIR" > /tmp/scan1.out 2>&1; then
  echo "  PASS"
  PASS=$((PASS+1))
else
  rc=$?
  echo "  FAIL (exit $rc)"
  sed 's/^/    /' /tmp/scan1.out
  FAIL=$((FAIL+1))
fi

# --- Test 2: planted secret should FAIL (exit non-zero) ---
echo "TEST 2: planted secret → expect non-zero exit"
echo "OPENAI_API_KEY=sk-FAKE0123456789abcdefghijklmnopqrstuvwxyz" > "$PLANT_DIR/leak-test.txt"
if scripts/scan-secrets.sh "$PLANT_DIR" > /tmp/scan2.out 2>&1; then
  echo "  FAIL — scanner did not detect the planted secret"
  sed 's/^/    /' /tmp/scan2.out
  FAIL=$((FAIL+1))
else
  echo "  PASS — scanner detected the planted secret"
  PASS=$((PASS+1))
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
