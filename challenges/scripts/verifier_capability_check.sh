#!/usr/bin/env bash
# ============================================================================
# CONST-041 Challenge: No Hardcoded Capability Flags
# ============================================================================
# Authority: CONST-041 — MCP/LSP/ACP/Embedding/RAG/Skills/Plugins Integration Mandate
#
# This script verifies that model capability flags are sourced from the
# verifier's VerificationResult, not hardcoded in provider adapters.
#
# Exit 0 = PASS
# Exit 1 = FAIL
# ============================================================================

set -euo pipefail

echo "=== CONST-041 Challenge: Capability Flags Source Verification ==="

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

VIOLATIONS=0
SCAN_DIR="${1:-HelixCode}"

cd "$SCAN_DIR"

# Pattern 1: Hardcoded SupportsToolUse / SupportsVision in non-test, non-verifier files
echo "→ Scanning for hardcoded capability assignments..."
PATTERN1=$(grep -rPn 'SupportsToolUse:\s*(true|false)|SupportsVision:\s*(true|false)|SupportsStreaming:\s*(true|false)|SupportsReasoning:\s*(true|false)' \
    --include="*.go" \
    internal/ cmd/ 2>/dev/null | grep -v "_test.go" | grep -v "verifier_integration.go" | grep -v "fallback_models.go" | grep -v "doc.go" | grep -v "internal/llm/vision" || true)

if [ -n "$PATTERN1" ]; then
    echo -e "${RED}VIOLATION: Hardcoded capability flags found:${NC}"
    echo "$PATTERN1"
    VIOLATIONS=$((VIOLATIONS + 1))
else
    echo -e "${GREEN}  ✓ No hardcoded capability flags in production code${NC}"
fi

# Pattern 2: Hardcoded capability slices
echo "→ Scanning for hardcoded capability slices..."
PATTERN2=$(grep -rPn 'Capabilities:\s*\[\]\w*Capability\{|Capabilities:\s*\[\]string\{' \
    --include="*.go" \
    internal/ cmd/ 2>/dev/null | grep -v "_test.go" | grep -v "verifier_integration.go" | grep -v "fallback_models.go" | grep -v "doc.go" | grep -v "internal/llm/vision" || true)

if [ -n "$PATTERN2" ]; then
    echo -e "${YELLOW}WARNING: Hardcoded capability slices found (review required):${NC}"
    echo "$PATTERN2"
    VIOLATIONS=$((VIOLATIONS + 1))
else
    echo -e "${GREEN}  ✓ No hardcoded capability slices${NC}"
fi

# Pattern 3: Verify verifier_integration.go maps from VerifiedModel capabilities
echo "→ Verifying capability mapping in verifier_integration.go..."
if grep -q "v.SupportsCode" internal/llm/verifier_integration.go && \
   grep -q "v.SupportsStreaming" internal/llm/verifier_integration.go && \
   grep -q "v.SupportsTools" internal/llm/verifier_integration.go; then
    echo -e "${GREEN}  ✓ Capabilities mapped from verifier VerifiedModel${NC}"
else
    echo -e "${RED}VIOLATION: verifier_integration.go does not map verifier capabilities${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Pattern 4: Verify model_manager uses verifier scores
echo "→ Verifying ModelManager uses verifier adapter for scoring..."
if grep -q "verifierAdapter" internal/llm/model_manager.go && \
   grep -q "GetModelScore" internal/llm/model_manager.go; then
    echo -e "${GREEN}  ✓ ModelManager integrates verifier scores${NC}"
else
    echo -e "${RED}VIOLATION: ModelManager does not use verifier adapter${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
echo ""
if [ "$VIOLATIONS" -eq 0 ]; then
    echo -e "${GREEN}=== CHALLENGE PASS: CONST-041 Compliant ===${NC}"
    exit 0
else
    echo -e "${RED}=== CHALLENGE FAIL: $VIOLATIONS violation(s) found ===${NC}"
    exit 1
fi
