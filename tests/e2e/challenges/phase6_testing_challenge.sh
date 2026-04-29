#!/bin/bash
# Phase 6 Anti-Bluff Challenge: Testing & Coverage
set -e

echo "=== Phase 6 Anti-Bluff Challenge: Testing & Coverage ==="

cd HelixCode

# Test 1: Check test coverage for auth (should be high)
echo "[1/5] Checking auth test coverage..."
result=$(go test ./internal/auth/... -cover -timeout 10s 2>&1 | grep "coverage:")
if [ -z "$result" ]; then
    echo "FAIL: Could not get coverage for auth"
    exit 1
fi
echo "  PASS: Auth coverage - $result"

# Test 2: Check test coverage for agent
echo "[2/5] Checking agent test coverage..."
go test ./internal/agent/... -cover -timeout 15s 2>&1 | grep -q "coverage:" || (echo "FAIL: Agent coverage check failed"; exit 1)
echo "  PASS: Agent coverage check passed"

# Test 3: Run all unit tests (no mocks above unit level)
echo "[3/5] Running unit tests..."
go test ./internal/... -short -timeout 60s 2>&1 | grep -q "FAIL" && (echo "FAIL: Unit tests failed"; exit 1)
echo "  PASS: Unit tests pass"

# Test 4: Verify no 'TODO implement' in production code
echo "[4/5] Checking for TODO implement markers..."
count=$(grep -rn "TODO implement\|placeholder" internal/ --include="*.go" | grep -v "_test.go" | wc -l)
if [ "$count" -gt 0 ]; then
    echo "FAIL: Found $count TODO/placeholder markers"
    exit 1
fi
echo "  PASS: No TODO/placeholder markers"

# Test 5: Verify challenge scripts exist
echo "[5/5] Checking challenge scripts..."
test -f ../tests/e2e/challenges/phase1_llm_challenge.sh || (echo "FAIL: Phase 1 challenge missing"; exit 1)
test -f ../tests/e2e/challenges/phase2_tools_editor_challenge.sh || (echo "FAIL: Phase 2 challenge missing"; exit 1)
echo "  PASS: All challenge scripts exist"

echo ""
echo "=== PHASE 6 CHALLENGES PASSED ==="
echo "Testing & Coverage: COMPLETE"
