#!/bin/bash
# Phase 2 Anti-Bluff Challenge: Tools & Editor
set -e

echo "=== Phase 2 Anti-Bluff Challenge: Tools & Editor ==="

# Test 1: Tools package builds
echo "[1/4] Checking tools package..."
cd helix_code; go build ./internal/tools/... || (echo "FAIL: Tools build failed"; exit 1)
echo "  PASS: Tools package builds"

# Test 2: Editor package builds
echo "[2/4] Checking editor package..."
go build ./internal/editor/... || (echo "FAIL: Editor build failed"; exit 1)
echo "  PASS: Editor package builds"

# Test 3: Run tools tests
# Timeout bumped from 30s to 180s: internal/tools/browser tests serialize
# real chromium launches via an inter-process flock (see
# chromium_serialize_test.go) so the chromium phase of the suite is
# inherently sequential. Five chromium-launching tests at ~6 s each =
# ~30 s in the lock, plus per-test fixed costs. Verified locally:
# `go test ./internal/tools/browser/` exits 0 in ~43 s with the flock
# in place. The earlier 30 s budget killed the test binary mid-chromium,
# producing a false anti-bluff FAIL when in reality the run-time race
# is environmental, not a defect.
echo "[3/4] Running tools tests..."
# Anti-bluff (CONST-035 / §11.9): the prior form was
#   `go test ... 2>&1 | grep -q "FAIL" && (echo "FAIL: ..."; exit 1)`
# which silently PASSed on:
#   - "no test files" output (zero coverage masquerading as success)
#   - compilation errors (output doesn't contain literal "FAIL")
#   - `panic: test timed out` (different output token)
#   - any error path Go test runner takes that doesn't print "FAIL"
# Using the `go test` exit code (non-zero on any failure incl. compile
# error / panic / timeout) is the actual contract.
go test ./internal/tools/... -timeout 180s || { echo "FAIL: Tools tests failed"; exit 1; }
echo "  PASS: Tools tests pass"

# Test 4: Run editor tests
echo "[4/4] Running editor tests..."
go test ./internal/editor/... -timeout 30s || { echo "FAIL: Editor tests failed"; exit 1; }
echo "  PASS: Editor tests pass"

echo ""
echo "=== PHASE 2 CHALLENGES PASSED ==="
