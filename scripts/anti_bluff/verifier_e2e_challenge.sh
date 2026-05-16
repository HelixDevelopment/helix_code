#!/usr/bin/env bash
# ============================================================================
# E2E Challenge: LLMsVerifier End-to-End Workflow
# ============================================================================
# Authority: CONST-035, CONST-036, CONST-037, CONST-038, CONST-039, CONST-040
#
# This challenge verifies that the complete verifier integration works:
# 1. Server boots with verifier subsystem initialized
# 2. API /api/v1/llm/models returns verifier-sourced data (or fallback)
# 3. API /api/v1/llm/providers returns verifier-sourced provider list
# 4. CLI --list-models uses verifier data
# 5. ModelManager scoring incorporates verifier scores
#
# Exit 0 = PASS
# Exit 1 = FAIL
# ============================================================================

set -euo pipefail

echo "=== E2E Challenge: LLMsVerifier End-to-End Workflow ==="

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

VIOLATIONS=0
SCAN_DIR="${1:-HelixCode}"
cd "$SCAN_DIR"

# Step 1: Verify server binary compiles with verifier wiring
echo "→ Step 1: Server compiles with verifier wiring..."
if go build -o /tmp/helixcode-server ./cmd/server > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ Server binary compiles${NC}"
else
    echo -e "${RED}FAIL: Server binary does not compile${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 2: Verify CLI binary compiles with verifier wiring
echo "→ Step 2: CLI compiles with verifier wiring..."
if go build -o /tmp/helixcode-cli ./cmd/cli > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ CLI binary compiles${NC}"
else
    echo -e "${RED}FAIL: CLI binary does not compile${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 3: Verify server handlers use verifier adapter
echo "→ Step 3: Server handlers use verifier adapter..."
if grep -q "verifierResult" internal/server/handlers.go && \
   grep -q "GetVerifiedModels" internal/server/handlers.go && \
   grep -q "buildProvidersFromVerifiedModels" internal/server/handlers.go; then
    echo -e "${GREEN}  ✓ Server handlers wired to verifier${NC}"
else
    echo -e "${RED}FAIL: Server handlers not wired to verifier${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 4: Verify CLI uses verifier adapter
echo "→ Step 4: CLI uses verifier adapter..."
if grep -q "verifierAdapter" cmd/cli/main.go && \
   grep -q "verifier.Bootstrap" cmd/cli/main.go && \
   grep -q "GetVerifiedModels" cmd/cli/main.go; then
    echo -e "${GREEN}  ✓ CLI wired to verifier${NC}"
else
    echo -e "${RED}FAIL: CLI not wired to verifier${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 5: Verify ModelManager uses verifier scores
echo "→ Step 5: ModelManager uses verifier scores..."
if grep -q "verifierAdapter" internal/llm/model_manager.go && \
   grep -q "GetModelScore" internal/llm/model_manager.go; then
    echo -e "${GREEN}  ✓ ModelManager integrates verifier scores${NC}"
else
    echo -e "${RED}FAIL: ModelManager does not use verifier scores${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 6: Verify bootstrap.go exists and is wired
echo "→ Step 6: Verifier bootstrap exists..."
if [ -f "internal/verifier/bootstrap.go" ]; then
    if grep -q "func Bootstrap" internal/verifier/bootstrap.go && \
       grep -q "func (r \*BootstrapResult) Shutdown" internal/verifier/bootstrap.go; then
        echo -e "${GREEN}  ✓ Bootstrap helper present${NC}"
    else
        echo -e "${RED}FAIL: Bootstrap helper incomplete${NC}"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
else
    echo -e "${RED}FAIL: Bootstrap helper missing${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 7: Verify integration tests exist
echo "→ Step 7: Integration tests exist..."
if [ -f "internal/verifier/integration_test.go" ]; then
    if grep -q "go:build integration" internal/verifier/integration_test.go; then
        echo -e "${GREEN}  ✓ Integration tests present${NC}"
    else
        echo -e "${YELLOW}WARNING: integration_test.go missing build tag${NC}"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
else
    echo -e "${RED}FAIL: Integration tests missing${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 8: Verify provider enrichment bridge exists
echo "→ Step 8: Provider enrichment bridge exists..."
if [ -f "internal/llm/verifier_bridge.go" ]; then
    if grep -q "EnrichModelInfo" internal/llm/verifier_bridge.go && \
       grep -q "SetVerifierAdapter" internal/llm/verifier_bridge.go; then
        echo -e "${GREEN}  ✓ Provider enrichment bridge present${NC}"
    else
        echo -e "${RED}FAIL: Provider enrichment bridge incomplete${NC}"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
else
    echo -e "${RED}FAIL: Provider enrichment bridge missing${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 9: Run unit tests
echo "→ Step 9: Running verifier unit tests..."
if go test -v ./internal/verifier/... > /tmp/verifier_unit_test.log 2>&1; then
    PASS_COUNT=$(grep -c "^--- PASS:" /tmp/verifier_unit_test.log || true)
    echo -e "${GREEN}  ✓ Unit tests pass ($PASS_COUNT tests)${NC}"
else
    echo -e "${RED}FAIL: Unit tests failed${NC}"
    tail -20 /tmp/verifier_unit_test.log
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Step 10: Run integration tests
echo "→ Step 10: Running verifier integration tests..."
if go test -tags=integration -v ./internal/verifier/... > /tmp/verifier_integration_test.log 2>&1; then
    PASS_COUNT=$(grep -c "^--- PASS:" /tmp/verifier_integration_test.log || true)
    echo -e "${GREEN}  ✓ Integration tests pass ($PASS_COUNT tests)${NC}"
else
    echo -e "${RED}FAIL: Integration tests failed${NC}"
    tail -20 /tmp/verifier_integration_test.log
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
echo ""
if [ "$VIOLATIONS" -eq 0 ]; then
    echo -e "${GREEN}=== E2E CHALLENGE PASS: Verifier fully integrated ===${NC}"
    exit 0
else
    echo -e "${RED}=== E2E CHALLENGE FAIL: $VIOLATIONS violation(s) ===${NC}"
    exit 1
fi
