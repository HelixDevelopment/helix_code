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

# Only propagate into HelixCode-owned submodules; skip third-party repos.
is_owned() {
    local p="$1"
    case "$p" in
        Challenges|Security|Containers|HelixQA) return 0 ;;
        HelixAgent|helix_agent/HelixLLM|helix_agent/HelixMemory|helix_agent/HelixSpecifier) return 0 ;;
        submodules/*) return 0 ;;
        *) return 1 ;;
    esac
}

echo "=== Governance Propagation ==="
echo "Repo: $REPO_ROOT"
echo ""

while IFS= read -r line; do
    path=$(echo "$line" | awk '{print $2}')

    if ! is_owned "$path"; then
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

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

    branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
    if [ "$branch" = "HEAD" ] || [ -z "$branch" ]; then
        echo "⚠ DETACHED_HEAD: $path — skipping push (checkout main first)"
        FAILED=$((FAILED + 1))
        cd "$REPO_ROOT"
        continue
    fi
    if timeout "$TIMEOUT" git push origin "$branch" > /dev/null 2>&1; then
        echo "✅ PUSHED: $path ($branch)"
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
