#!/bin/bash
# Run all HelixCode anti-bluff challenges

cd "$(dirname "$0")/../../.." # Change to repo root

# --- Governance cascade gate (CONST-035 / Article XI §11.9) ---
echo "[PRE-CHECK] Running governance cascade verification..."
if [[ -x "scripts/verify-governance-cascade.sh" ]]; then
  ./scripts/verify-governance-cascade.sh
  if [[ $? -ne 0 ]]; then
    echo "[BLOCKED] Governance cascade verification failed. Merge prohibited."
    exit 42
  fi
else
  echo "[BLOCKED] Governance verification script not found or not executable."
  exit 43
fi
echo "[PRE-CHECK] Governance cascade verification passed."
# --- End governance cascade gate ---

echo "=========================================="
echo "  HelixCode Anti-Bluff Challenge Runner"
echo "=========================================="
echo ""

PASSED=0
FAILED=0

for phase in 1 2 3 4 5 6 7 8; do
    echo "=== Running Phase $phase Challenge ==="
    SCRIPT=$(ls tests/e2e/challenges/phase${phase}_*challenge.sh 2>/dev/null | head -1)
    if [ -z "$SCRIPT" ]; then
        echo "❌ Phase $phase: Script not found"
        ((FAILED++))
    elif bash "$SCRIPT" 2>&1; then
        echo "✅ Phase $phase: PASSED"
        ((PASSED++))
    else
        echo "❌ Phase $phase: FAILED"
        ((FAILED++))
    fi
    echo ""
done

# gptme port anti-bluff challenges (subagent Role, verifier profile, cache coldness).
# Listed explicitly so the existing phase{N}_*.sh glob is undisturbed.
GPTME_SCRIPTS=(
    "tests/e2e/challenges/gptme_subagent_role.sh"
    "tests/e2e/challenges/gptme_verifier_profile.sh"
    "tests/e2e/challenges/gptme_cache_coldness.sh"
)
for SCRIPT in "${GPTME_SCRIPTS[@]}"; do
    NAME=$(basename "$SCRIPT" .sh)
    echo "=== Running $NAME Challenge ==="
    if [ ! -f "$SCRIPT" ]; then
        echo "❌ $NAME: Script not found"
        ((FAILED++))
    elif bash "$SCRIPT" 2>&1; then
        echo "✅ $NAME: PASSED"
        ((PASSED++))
    else
        echo "❌ $NAME: FAILED"
        ((FAILED++))
    fi
    echo ""
done

echo "=========================================="
echo "  Results: $PASSED passed, $FAILED failed"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
    exit 1
fi

echo "🎉 ALL CHALLENGES PASSED! Anti-Bluff Verification: COMPLETE"
