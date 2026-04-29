#!/bin/bash
# Phase 2 Anti-Bluff Challenge: Tools & Editor
set -e

echo "=== Phase 2 Anti-Bluff Challenge: Tools & Editor ==="

# Test 1: Tools package builds
echo "[1/4] Checking tools package..."
cd HelixCode; go build ./internal/tools/... || (echo "FAIL: Tools build failed"; exit 1)
echo "  PASS: Tools package builds"

# Test 2: Editor package builds
echo "[2/4] Checking editor package..."
go build ./internal/editor/... || (echo "FAIL: Editor build failed"; exit 1)
echo "  PASS: Editor package builds"

# Test 3: Run tools tests
echo "[3/4] Running tools tests..."
go test ./internal/tools/... -timeout 30s 2>&1 | grep -q "FAIL" && (echo "FAIL: Tools tests failed"; exit 1)
echo "  PASS: Tools tests pass"

# Test 4: Run editor tests
echo "[4/4] Running editor tests..."
go test ./internal/editor/... -timeout 30s 2>&1 | grep -q "FAIL" && (echo "FAIL: Editor tests failed"; exit 1)
echo "  PASS: Editor tests pass"

echo ""
echo "=== PHASE 2 CHALLENGES PASSED ==="
