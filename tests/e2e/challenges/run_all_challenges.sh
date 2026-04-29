#!/bin/bash
# Run all HelixCode anti-bluff challenges

cd "$(dirname "$0")/../../.." # Change to repo root

echo "=========================================="
echo "  HelixCode Anti-Bluff Challenge Runner"
echo "=========================================="
echo ""

PASSED=0
FAILED=0

for phase in 1 2 3 4 5 6 7; do
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

echo "=========================================="
echo "  Results: $PASSED passed, $FAILED failed"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
    exit 1
fi

echo "🎉 ALL CHALLENGES PASSED! Anti-Bluff Verification: COMPLETE"
