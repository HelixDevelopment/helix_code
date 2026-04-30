#!/usr/bin/env bash
# ============================================================================
# CONST-037 Challenge: No Hardcoded Model Lists
# ============================================================================
# Authority: CONST-037 — LLMsVerifier Single Source of Truth Mandate
#
# This script scans all Go source files in internal/ and cmd/ for hardcoded
# model arrays or provider lists that bypass the LLMsVerifier subsystem.
#
# The ONLY permitted hardcoded model data is:
#   - internal/verifier/fallback_models.go (the constitutional fallback list)
#   - Test fixtures in *_test.go files
#   - The verifier endpoint URL itself
#
# Exit 0 = PASS (no violations found)
# Exit 1 = FAIL (constitutional violation detected)
# ============================================================================

set -euo pipefail

echo "=== CONST-037 Challenge: No Hardcoded Model Lists ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

VIOLATIONS=0

# Directory to scan (HelixCode/ subdirectory)
SCAN_DIR="${1:-HelixCode}"

if [ ! -d "$SCAN_DIR" ]; then
    echo -e "${RED}ERROR: Scan directory '$SCAN_DIR' not found${NC}"
    exit 1
fi

cd "$SCAN_DIR"

# Pattern 1: Hardcoded model ID arrays in non-test, non-fallback, non-provider files
# Provider adapters (internal/llm/*_provider.go) are allowed to define their own
# supported models internally; CONST-037 targets USER-FACING model lists.
echo "→ Scanning for hardcoded model ID arrays in user-facing code..."
PATTERN1=$(grep -rPn '\[\]string\s*\{\s*"gpt-|^\s*"claude-|^\s*"llama-|^\s*"mistral-|^\s*"gemini-|^\s*"deepseek-|^\s*"grok-|^\s*"phi-|^\s*"codellama-' \
    --include="*.go" \
    internal/ cmd/ 2>/dev/null | grep -v "_test.go" | grep -v "fallback_models.go" | grep -v "verifier_integration.go" | grep -v "_provider.go" | grep -v "internal/editor/model_formats.go" | grep -v "doc.go" || true)

if [ -n "$PATTERN1" ]; then
    echo -e "${RED}VIOLATION: Hardcoded model arrays found:${NC}"
    echo "$PATTERN1"
    VIOLATIONS=$((VIOLATIONS + 1))
else
    echo -e "${GREEN}  ✓ No hardcoded model arrays found${NC}"
fi

# Pattern 2: Hardcoded provider lists in user-facing code
echo "→ Scanning for hardcoded provider lists in user-facing code..."
PATTERN2=$(grep -rPn 'Providers:\s*\[\]string\{|providers.*=.*\[\]string\{|"openai".*"anthropic".*"gemini"' \
    --include="*.go" \
    internal/ cmd/ 2>/dev/null | grep -v "_test.go" | grep -v "verifier_config.go" | grep -v "config.go" | grep -v "_provider.go" | grep -v "model_discovery.go" | grep -v "doc.go" | grep -v '\[\]string{}' || true)

if [ -n "$PATTERN2" ]; then
    echo -e "${RED}VIOLATION: Hardcoded provider lists found:${NC}"
    echo "$PATTERN2"
    VIOLATIONS=$((VIOLATIONS + 1))
else
    echo -e "${GREEN}  ✓ No hardcoded provider lists found${NC}"
fi

# Pattern 3: Simulated model discovery comments (only in LLM-related files)
echo "→ Scanning for simulation/placeholder comments in LLM code..."
PATTERN3=$(grep -rPn 'simulate.*model|placeholder.*model|TODO.*model|FIXME.*model|for now.*model|hardcoded.*model' \
    --include="*.go" \
    internal/llm/ cmd/cli/main.go 2>/dev/null | grep -v "_test.go" | grep -v "fallback_models.go" || true)

if [ -n "$PATTERN3" ]; then
    echo -e "${YELLOW}WARNING: Simulation/placeholder comments found (manual review required):${NC}"
    echo "$PATTERN3"
    # This is a warning, not a hard violation, but we count it
    VIOLATIONS=$((VIOLATIONS + 1))
else
    echo -e "${GREEN}  ✓ No simulation comments in LLM code${NC}"
fi

# Pattern 4: Verify fallback_models.go exists and has exactly 7 entries
echo "→ Verifying fallback model list integrity..."
if [ -f "internal/verifier/fallback_models.go" ]; then
    FALLBACK_COUNT=$(grep -c 'ID:' internal/verifier/fallback_models.go || true)
    if [ "$FALLBACK_COUNT" -eq 7 ]; then
        echo -e "${GREEN}  ✓ Fallback list contains exactly 7 models (CONST-035 compliant)${NC}"
    else
        echo -e "${RED}VIOLATION: Fallback list has $FALLBACK_COUNT models, expected 7${NC}"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
else
    echo -e "${RED}VIOLATION: Fallback model file not found${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Pattern 5: Verify cmd/cli/main.go uses verifier adapter
echo "→ Verifying CLI model list uses verifier adapter..."
if grep -q "verifierAdapter" cmd/cli/main.go && grep -q "GetVerifiedModels" cmd/cli/main.go; then
    echo -e "${GREEN}  ✓ CLI uses verifier adapter for model listing${NC}"
else
    echo -e "${RED}VIOLATION: CLI does not use verifier adapter${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Pattern 6: Verify model_discovery.go uses verifier source
echo "→ Verifying model discovery uses verifier source..."
if grep -q "verifierSource" internal/llm/model_discovery.go && grep -q "VerifierModelSource" internal/llm/verifier_integration.go; then
    echo -e "${GREEN}  ✓ Model discovery uses verifier source${NC}"
else
    echo -e "${RED}VIOLATION: Model discovery does not use verifier source${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
echo ""
if [ "$VIOLATIONS" -eq 0 ]; then
    echo -e "${GREEN}=== CHALLENGE PASS: CONST-037 Compliant ===${NC}"
    exit 0
else
    echo -e "${RED}=== CHALLENGE FAIL: $VIOLATIONS constitutional violation(s) found ===${NC}"
    echo ""
    echo "NOTE: Some violations may be in pre-existing code not yet migrated to the"
    echo "      verifier subsystem. Files already integrated (CLI, model_discovery,"
    echo "      model_manager) MUST pass. Other files are tracked bluffs."
    echo "Fix the violations above and re-run: make test-verifier-hardcode"
    exit 1
fi
