#!/usr/bin/env bash
# scripts/test-scan-secrets.sh
# Tests scan-secrets.sh: passes on a clean tree, fails on a planted secret,
# and respects the allowlist mechanism.

set -uo pipefail
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

CLEAN_DIR=$(mktemp -d)
PLANT_DIR=$(mktemp -d)
ALLOW_DIR=$(mktemp -d)
trap 'rm -rf "$CLEAN_DIR" "$PLANT_DIR" "$ALLOW_DIR"' EXIT

# Put a benign file in the clean dir to make sure the scanner traverses
printf "FOO=bar\n" > "$CLEAN_DIR/safe.txt"

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
printf "OPENAI_API_KEY=sk-FAKE0123456789abcdefghijklmnopqrstuvwxyz\n" > "$PLANT_DIR/leak-test.txt"
if scripts/scan-secrets.sh "$PLANT_DIR" > /tmp/scan2.out 2>&1; then
  echo "  FAIL — scanner did not detect the planted secret"
  sed 's/^/    /' /tmp/scan2.out
  FAIL=$((FAIL+1))
else
  echo "  PASS — scanner detected the planted secret"
  PASS=$((PASS+1))
fi

# --- Test 3: secret in an allowlisted path should PASS (exit 0) ---
echo "TEST 3: secret in allowlisted path → expect exit 0"
# Plant a secret in the allow dir
printf "TOKEN=sk-FAKE0123456789abcdefghijklmnopqrstuvwxyz\n" > "$ALLOW_DIR/allowlisted-doc.txt"
# Temporarily install an allowlist that covers the planted file
ORIGINAL_ALLOW="$REPO_ROOT/.scan-secrets-allow"
BACKUP_ALLOW="$REPO_ROOT/.scan-secrets-allow.bak.$$"
if [ -f "$ORIGINAL_ALLOW" ]; then
  cp "$ORIGINAL_ALLOW" "$BACKUP_ALLOW"
fi
# The allowlist path must match what grep outputs: relative path from scan target.
# Since we scan $ALLOW_DIR directly, the file appears as "allowlisted-doc.txt".
RESTORE_ALLOW() {
  if [ -f "$BACKUP_ALLOW" ]; then
    mv "$BACKUP_ALLOW" "$ORIGINAL_ALLOW"
  else
    rm -f "$ORIGINAL_ALLOW"
  fi
}
trap 'rm -rf "$CLEAN_DIR" "$PLANT_DIR" "$ALLOW_DIR"; RESTORE_ALLOW' EXIT

printf "# test allowlist\nallowlisted-doc.txt\n" > "$ORIGINAL_ALLOW"
if scripts/scan-secrets.sh "$ALLOW_DIR" > /tmp/scan3.out 2>&1; then
  echo "  PASS — allowlisted path correctly skipped"
  PASS=$((PASS+1))
else
  echo "  FAIL — scanner flagged an allowlisted path"
  sed 's/^/    /' /tmp/scan3.out
  FAIL=$((FAIL+1))
fi
RESTORE_ALLOW
trap 'rm -rf "$CLEAN_DIR" "$PLANT_DIR" "$ALLOW_DIR"' EXIT

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
