#!/bin/bash
# Phase 1 Anti-Bluff Challenge: Verify LLM providers work for real
set -e

cd "$(dirname "$0")/../../.."  # Change to repo root

echo "=== Phase 1 Anti-Bluff Challenge: LLM Providers ==="
echo "Verifying real LLM integration (no simulations)"

# Test 1: Check CLI binary exists
echo "[1/5] Checking CLI binary..."
test -f helix_code/bin/cli || (echo "FAIL: CLI binary not found"; exit 1)
echo "  PASS: CLI binary exists"

# Test 2: Check LLM package builds
echo "[2/5] Checking LLM package builds..."
cd helix_code; go build ./internal/llm/... || (echo "FAIL: LLM package build failed"; exit 1)
echo "  PASS: LLM package builds"

# Test 3: Run LLM unit tests (no mocks) — exit-code based (CONST-035 anti-bluff)
# The prior form `grep -q "PASS"` passed as long as ANY line contained
# "PASS" — including the case where some tests PASS and others FAIL.
# Using the exit code captures the run-level result correctly.
echo "[3/5] Running LLM unit tests..."
go test ./internal/llm/ -timeout 30s -run "TestNewLlamaCPPProvider|TestLlamaCPPProvider_Close" -v || { echo "FAIL: LLM tests failed"; exit 1; }
echo "  PASS: LLM unit tests pass"

# Test 4: Verify no 'simulated' in production LLM code
echo "[4/5] Checking for bluff markers..."
count=$(grep -rn "simulated\|for now" internal/llm/*.go | grep -v "_test.go" | wc -l)
if [ "$count" -gt 0 ]; then
    echo "FAIL: Found $count bluff markers in LLM code"
    exit 1
fi
echo "  PASS: No bluff markers in LLM code"

# Test 5: Check provider interface implemented — exit-code based (CONST-035 anti-bluff)
echo "[5/5] Checking provider interface..."
go test ./internal/llm/ -timeout 10s -run "TestAlias" -v || { echo "FAIL: Provider interface test failed"; exit 1; }
echo "  PASS: Provider interface works"

echo ""
echo "=== ALL PHASE 1 CHALLENGES PASSED ==="
echo "Anti-Bluff Verification: COMPLETE"
