#!/bin/bash
# Phase 3 Anti-Bluff Challenge: Worker & Distributed Computing
set -e

echo "=== Phase 3 Anti-Bluff Challenge: Worker & Distributed ==="

cd HelixCode

# Test 1: Worker package builds
echo "[1/4] Checking worker package..."
go build ./internal/worker/... || (echo "FAIL: Worker build failed"; exit 1)
echo "  PASS: Worker package builds"

# Test 2: Run worker tests
echo "[2/4] Running worker tests..."
go test ./internal/worker/... -timeout 15s 2>&1 | grep -q "FAIL" && (echo "FAIL: Worker tests failed"; exit 1)
echo "  PASS: Worker tests pass"

# Test 3: Check no bluff markers
echo "[3/4] Checking for bluff markers..."
count=$(grep -rn "simulated\|for now\|TODO implement" internal/worker/*.go | grep -v "_test.go" | wc -l)
if [ "$count" -gt 0 ]; then
    echo "FAIL: Found $count bluff markers in worker code"
    exit 1
fi
echo "  PASS: No bluff markers in worker code"

# Test 4: Server builds (distributed computing)
echo "[4/4] Checking server builds..."
go build ./cmd/server/... || (echo "FAIL: Server build failed"; exit 1)
echo "  PASS: Server builds"

echo ""
echo "=== PHASE 3 CHALLENGES PASSED ==="
