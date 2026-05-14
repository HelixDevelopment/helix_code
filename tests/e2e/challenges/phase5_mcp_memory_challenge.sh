#!/bin/bash
# Phase 5 Anti-Bluff Challenge: MCP, Memory & Notifications
set -e

echo "=== Phase 5 Anti-Bluff Challenge: MCP, Memory, Notifications ==="

cd HelixCode

# Test 1: MCP package builds
echo "[1/5] Checking MCP package..."
go build ./internal/mcp/... || (echo "FAIL: MCP build failed"; exit 1)
echo "  PASS: MCP package builds"

# Test 2: Memory package builds
echo "[2/5] Checking memory package..."
go build ./internal/memory/... || (echo "FAIL: Memory build failed"; exit 1)
echo "  PASS: Memory package builds"

# Test 3: Notifications package builds
echo "[3/5] Checking notification package..."
go build ./internal/notification/... || (echo "FAIL: Notification build failed"; exit 1)
echo "  PASS: Notification package builds"

# Test 4: Run memory tests (skip e2e) — exit-code based (CONST-035 anti-bluff)
echo "[4/5] Running memory tests..."
go test ./internal/memory/... -timeout 90s -short || { echo "FAIL: Memory tests failed"; exit 1; }
echo "  PASS: Memory tests pass"

# Test 5: Run notification tests
echo "[5/5] Running notification tests..."
go test ./internal/notification/... -timeout 15s || { echo "FAIL: Notification tests failed"; exit 1; }
echo "  PASS: Notification tests pass"

echo ""
echo "=== PHASE 5 CHALLENGES PASSED ==="
