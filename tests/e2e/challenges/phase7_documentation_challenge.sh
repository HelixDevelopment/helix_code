#!/bin/bash
# Phase 7 Anti-Bluff Challenge: Documentation & Deployment
set -e

echo "=== Phase 7 Anti-Bluff Challenge: Documentation & Deployment ==="

# Test 1: Verify governance files exist
echo "[1/6] Checking governance files..."
test -f CONSTITUTION.md || (echo "FAIL: CONSTITUTION.md missing"; exit 1)
test -f CLAUDE.md || (echo "FAIL: CLAUDE.md missing"; exit 1)
test -f AGENTS.md || (echo "FAIL: AGENTS.md missing"; exit 1)
echo "  PASS: All governance files exist"

# Test 2: Verify docker config exists
echo "[2/6] Checking Docker configuration..."
test -f helix_code/docker-compose.yml || (echo "FAIL: docker-compose.yml missing"; exit 1)
test -f helix_code/Dockerfile || (echo "FAIL: Dockerfile missing"; exit 1)
test -f docker/docker-entrypoint.sh || (echo "FAIL: docker-entrypoint.sh missing"; exit 1)
echo "  PASS: Docker config exists"

# Test 3: Verify deployment package builds
echo "[3/6] Checking deployment package..."
cd helix_code; go build ./internal/deployment/... || (echo "FAIL: Deployment build failed"; exit 1)
echo "  PASS: Deployment package builds"

# Test 4: Run deployment tests — exit-code based (CONST-035 anti-bluff)
echo "[4/6] Running deployment tests..."
go test ./internal/deployment/... -timeout 30s || { echo "FAIL: Deployment tests failed"; exit 1; }
echo "  PASS: Deployment tests pass"

# Test 5: Verify master action plan exists
echo "[5/6] Checking master action plan..."
test -f ../docs/bluff_proofing/MASTER_ACTION_PLAN.md || (echo "FAIL: MASTER_ACTION_PLAN.md missing"; exit 1)
echo "  PASS: Master action plan exists"

# Test 6: Verify all challenge scripts
echo "[6/6] Checking all challenge scripts..."
for phase in 1 2 3 4 5 6; do
    test -f ../tests/e2e/challenges/phase${phase}_*challenge.sh || (echo "FAIL: Phase $phase challenge missing"; exit 1)
done
echo "  PASS: All challenge scripts exist"

echo ""
echo "=== PHASE 7 CHALLENGES PASSED ==="
echo "Documentation & Deployment: COMPLETE"
echo ""
echo "=== PHASE 7 CHALLENGE COMPLETE ==="
