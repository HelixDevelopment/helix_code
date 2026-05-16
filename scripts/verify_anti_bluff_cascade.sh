#!/usr/bin/env bash
# scripts/verify_anti_bluff_cascade.sh
# Asserts the anti-bluff anchor (CONST-035) is present in every Helix* repo's
# CONSTITUTION.md / CLAUDE.md / AGENTS.md.
#
# Exits non-zero on any gap. Used as a Phase 1.5 invariant + future CI gate.

set -euo pipefail

ANCHOR='We had been in position that all tests do execute with success'

TARGETS=(
    "."
    "HelixAgent"
    "HelixQA"
    "Dependencies/HelixDevelopment/LLMsVerifier"
    "Dependencies/HelixDevelopment/DocProcessor"
    "Dependencies/HelixDevelopment/LLMOrchestrator"
    "Dependencies/HelixDevelopment/LLMProvider"
    "Dependencies/HelixDevelopment/VisionEngine"
    "Containers"
    "Security"
    "helix_agent/HelixLLM"
    "helix_agent/HelixSpecifier"
    "helix_agent/HelixMemory"
)

GAPS=0
TOTAL=0
for T in "${TARGETS[@]}"; do
    if [ ! -d "$T" ]; then
        echo "WARN: target dir missing: $T (skipped)"
        continue
    fi
    for FILE in CONSTITUTION.md CLAUDE.md AGENTS.md; do
        TOTAL=$((TOTAL+1))
        if [ ! -f "$T/$FILE" ]; then
            echo "GAP: $T/$FILE missing"
            GAPS=$((GAPS+1))
        elif ! grep -q "$ANCHOR" "$T/$FILE"; then
            echo "GAP: $T/$FILE lacks anti-bluff anchor"
            GAPS=$((GAPS+1))
        fi
    done
done

if [ "$GAPS" -gt 0 ]; then
    echo ""
    echo "FAIL: $GAPS gap(s) out of $TOTAL checks"
    exit 1
fi

echo "OK: anti-bluff anchor present in all $TOTAL files across $((${#TARGETS[@]})) repos"
