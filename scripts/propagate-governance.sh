#!/usr/bin/env bash
# ============================================================================
# Governance Propagation Script
# ============================================================================

cd "$(git rev-parse --show-toplevel)"

REPO_ROOT="$(pwd)"
GOV_FILES=("CONSTITUTION.md" "CLAUDE.md" "AGENTS.md")
PUSHED=0
FAILED=0
SKIPPED=0
TIMEOUT=10

echo "=== Governance Propagation ==="
echo "Repo: $REPO_ROOT"
echo ""

while IFS= read -r line; do
    path=$(echo "$line" | awk '{print $2}')

    if [ ! -e "$path/.git" ]; then
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    cd "$path"

    copied=0
    for file in "${GOV_FILES[@]}"; do
        src="$REPO_ROOT/$file"
        if [ -f "$src" ]; then
            cp "$src" .
            git add "$file" > /dev/null 2>&1 || true
            copied=$((copied + 1))
        fi
    done

    if [ "$copied" -eq 0 ]; then
        SKIPPED=$((SKIPPED + 1))
        cd "$REPO_ROOT"
        continue
    fi

    if git diff --cached --quiet; then
        SKIPPED=$((SKIPPED + 1))
        cd "$REPO_ROOT"
        continue
    fi

    if ! git commit -m "chore(governance): propagate Constitution, CLAUDE.md, AGENTS.md" > /dev/null 2>&1; then
        SKIPPED=$((SKIPPED + 1))
        cd "$REPO_ROOT"
        continue
    fi

    if timeout "$TIMEOUT" git push > /dev/null 2>&1; then
        echo "✅ PUSHED: $path"
        PUSHED=$((PUSHED + 1))
    else
        echo "❌ PUSH_FAIL: $path (no write access or timeout)"
        FAILED=$((FAILED + 1))
    fi

    cd "$REPO_ROOT"
done < <(git submodule status)

echo ""
echo "Summary: Pushed=$PUSHED Failed=$FAILED Skipped=$SKIPPED"

echo "→ Pushing main repository..."
git push origin main && git push gitlab main 2>/dev/null
echo "✅ Main repository pushed"
