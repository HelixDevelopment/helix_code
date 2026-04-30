#!/usr/bin/env bash
# ============================================================================
# Governance Propagation Script
# ============================================================================
# Copies Constitution, CLAUDE.md, and AGENTS.md to all submodules,
# commits, and attempts to push. Third-party submodules will fail to
# push (expected) — failures are logged but do not stop the script.
#
# Usage: ./scripts/propagate-governance.sh
# ============================================================================

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

GOV_FILES=("CONSTITUTION.md" "CLAUDE.md" "AGENTS.md")
PUSHED=0
FAILED=0
SKIPPED=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Governance Propagation ==="
echo "Source: $(pwd)"
echo "Files: ${GOV_FILES[*]}"
echo ""

# Update all submodules first
git submodule update --init --recursive > /dev/null 2>&1 || true

# Iterate over all submodules
while IFS= read -r line; do
    # Parse submodule path from "git submodule status" output
    # Format: <commit> <path> (<branch>)
    path=$(echo "$line" | awk '{print $2}')

    if [ ! -d "$path/.git" ]; then
        echo -e "${YELLOW}SKIP: $path (not initialized)${NC}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    echo "→ Processing: $path"
    cd "$path"

    # Copy governance files
    COPIED=0
    for file in "${GOV_FILES[@]}"; do
        src="../../$file"
        if [ -f "$src" ]; then
            cp "$src" .
            git add "$file" > /dev/null 2>&1 || true
            COPIED=$((COPIED + 1))
        fi
    done

    if [ "$COPIED" -eq 0 ]; then
        echo -e "${YELLOW}  Nothing to copy${NC}"
        SKIPPED=$((SKIPPED + 1))
        cd - > /dev/null
        continue
    fi

    # Check if there are changes to commit
    if git diff --cached --quiet; then
        echo -e "${YELLOW}  No changes (files identical)${NC}"
        SKIPPED=$((SKIPPED + 1))
        cd - > /dev/null
        continue
    fi

    # Commit
    git commit -m "chore(governance): propagate Constitution, CLAUDE.md, AGENTS.md

Propagated from main HelixCode repository.
Authority: CONST-036 through CONST-041 — LLMsVerifier integration mandates.
" > /dev/null 2>&1 || {
        echo -e "${YELLOW}  Commit failed (possibly no changes)${NC}"
        SKIPPED=$((SKIPPED + 1))
        cd - > /dev/null
        continue
    }

    # Push (will fail for third-party submodules without write access)
    if git push > /dev/null 2>&1; then
        echo -e "${GREEN}  ✅ Pushed${NC}"
        PUSHED=$((PUSHED + 1))
    else
        echo -e "${RED}  ❌ Push failed (no write access — expected for third-party)${NC}"
        FAILED=$((FAILED + 1))
    fi

    cd - > /dev/null
done < <(git submodule status)

echo ""
echo "=== Summary ==="
echo -e "${GREEN}Pushed: $PUSHED${NC}"
echo -e "${RED}Failed to push: $FAILED${NC} (expected for third-party)"
echo -e "${YELLOW}Skipped: $SKIPPED${NC}"
echo ""

# Push main repo
echo "→ Pushing main repository..."
if git push origin main && git push gitlab main 2>/dev/null; then
    echo -e "${GREEN}✅ Main repository pushed${NC}"
else
    echo -e "${RED}❌ Main repository push failed${NC}"
    exit 1
fi
