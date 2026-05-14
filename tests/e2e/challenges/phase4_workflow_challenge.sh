#!/bin/bash
# Phase 4 Anti-Bluff Challenge: Workflow & Session
set -e

echo "=== Phase 4 Anti-Bluff Challenge: Workflow & Session ==="

cd HelixCode

# Test 1: Workflow package builds
echo "[1/4] Checking workflow package..."
go build ./internal/workflow/... || (echo "FAIL: Workflow build failed"; exit 1)
echo "  PASS: Workflow package builds"

# Test 2: Session package builds
echo "[2/4] Checking session package..."
go build ./internal/session/... || (echo "FAIL: Session build failed"; exit 1)
echo "  PASS: Session package builds"

# Test 3: Run workflow tests — exit-code based (CONST-035 anti-bluff: see phase2 for rationale)
echo "[3/4] Running workflow tests..."
go test ./internal/workflow/... -timeout 30s || { echo "FAIL: Workflow tests failed"; exit 1; }
echo "  PASS: Workflow tests pass"

# Test 4: Run session tests
echo "[4/4] Running session tests..."
go test ./internal/session/... -timeout 15s || { echo "FAIL: Session tests failed"; exit 1; }
echo "  PASS: Session tests pass"

echo ""
echo "=== PHASE 4 CHALLENGES PASSED ==="
