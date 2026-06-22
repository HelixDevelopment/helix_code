#!/usr/bin/env bash
# ============================================================================
# codegraph_validate.sh — Validate CodeGraph index for HelixCode
# ============================================================================
# Constitution §11.4.78, §11.4.79
#
# Validates:
#   1. codegraph is installed and accessible
#   2. .codegraph/config.json exists and is valid
#   3. .codegraph/codegraph.db exists and is non-empty
#   4. Own-org submodules are indexed (not excluded)
#   5. Third-party submodules are excluded
#   6. Credential paths are excluded
#
# Usage: bash scripts/codegraph_validate.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CODEGRAPH_DIR="$PROJECT_ROOT/.codegraph"
CONFIG_FILE="$CODEGRAPH_DIR/config.json"
DB_FILE="$CODEGRAPH_DIR/codegraph.db"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  ✅ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ❌ $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  ⏭️  $1"; SKIP=$((SKIP + 1)); }

echo "=== CodeGraph Validation ==="
echo "Project root: $PROJECT_ROOT"

# 1. codegraph installed
echo ""
echo "--- Check: codegraph installed ---"
if command -v codegraph &>/dev/null; then
    pass "codegraph found at $(which codegraph)"
else
    fail "codegraph not found"
fi

# 2. Config exists
echo ""
echo "--- Check: config.json ---"
if [ -f "$CONFIG_FILE" ]; then
    pass "config.json exists"

    # Check it's valid JSON
    if jq empty "$CONFIG_FILE" 2>/dev/null; then
        pass "config.json is valid JSON"
    else
        fail "config.json is not valid JSON"
    fi
else
    fail "config.json not found"
fi

# 3. Database exists
echo ""
echo "--- Check: codegraph.db ---"
if [ -f "$DB_FILE" ]; then
    DB_SIZE=$(stat -f%z "$DB_FILE" 2>/dev/null || stat -c%s "$DB_FILE" 2>/dev/null || echo "0")
    if [ "$DB_SIZE" -gt 0 ]; then
        pass "codegraph.db exists ($(( DB_SIZE / 1024 / 1024 )) MB)"
    else
        fail "codegraph.db is empty"
    fi
else
    fail "codegraph.db not found (run: codegraph index)"
fi

# 4. Own-org submodules NOT excluded
echo ""
echo "--- Check: own-org submodules included ---"
if [ -f "$CONFIG_FILE" ]; then
    for sub in constitution submodules/challenges submodules/containers submodules/helix_qa submodules/security; do
        if jq -e '.exclude[] | select(. == "'"$sub"'/**" or . == "'"$sub"'/*")' "$CONFIG_FILE" >/dev/null 2>&1; then
            fail "$sub is excluded (should be included per §11.4.79)"
        else
            pass "$sub is not excluded"
        fi
    done
fi

# 5. Third-party submodules excluded
echo ""
echo "--- Check: third-party excluded ---"
if [ -f "$CONFIG_FILE" ]; then
    for pat in "cli_agents/**" "cli_agents_resources/**" "dependencies/LLama_CPP/**" "dependencies/Ollama/**"; do
        if jq -e '.exclude[] | select(. == "'"$pat"'")' "$CONFIG_FILE" >/dev/null 2>&1; then
            pass "$pat is excluded"
        else
            fail "$pat is NOT excluded (should be per §11.4.79)"
        fi
    done
fi

# 6. Credential paths excluded
echo ""
echo "--- Check: credentials excluded ---"
if [ -f "$CONFIG_FILE" ]; then
    for pat in "**/.env" "**/.env.*" "**/*.key" "**/*.pem" "**/secrets/**"; do
        if jq -e '.exclude[] | select(. == "'"$pat"'")' "$CONFIG_FILE" >/dev/null 2>&1; then
            pass "$pat is excluded"
        else
            fail "$pat is NOT excluded (should be per §11.4.10)"
        fi
    done
fi

# 7. LIVE INDEX AUDIT (§11.4.78/§11.4.79 — AUTHORITATIVE).
# CodeGraph v1.x is zero-config: it derives index scope from .gitignore, NOT from
# .codegraph/config.json (which is INERT/no-longer-read). Checks 4-6 above inspect
# config.json and are therefore advisory only — a config-only PASS that does not
# query the real index is a §11.4 PASS-bluff. This section queries the LIVE DB so
# the validator reports what is ACTUALLY indexed.
echo ""
echo "--- Check: LIVE index audit (authoritative — excluded paths MUST be absent from the DB) ---"
if [ -f "$DB_FILE" ] && command -v sqlite3 &>/dev/null; then
    # third-party / non-code trees MUST NOT be in the index (§11.4.79 + §11.4.10)
    for pat in "cli_agents/%" "cli_agents_resources/%" "github_pages_website/%" "dependencies/LLama_CPP/%" "dependencies/Ollama/%"; do
        cnt=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM files WHERE path LIKE '$pat';" 2>/dev/null || echo "ERR")
        base="${pat%/\%}"
        if [ "$cnt" = "0" ]; then
            pass "live DB: 0 indexed files under $base"
        elif [ "$cnt" = "ERR" ]; then
            skip "live DB query failed for $base"
        else
            fail "live DB: $cnt files under $base ARE indexed (exclusion NOT effective — fix root .gitignore + 'codegraph index'; config.json is INERT per §11.4.78)"
        fi
    done
    # own-org submodules MUST be present (§11.4.79 — an index that lacks them is a bluff)
    for inc in "submodules/helix_qa/%" "submodules/llm_provider/%" "constitution/%"; do
        cnt=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM files WHERE path LIKE '$inc';" 2>/dev/null || echo "ERR")
        base="${inc%/\%}"
        if [ "$cnt" != "0" ] && [ "$cnt" != "ERR" ]; then
            pass "live DB: own-org $base indexed ($cnt files, §11.4.79)"
        else
            fail "live DB: own-org $base NOT indexed (count=$cnt) — must be included per §11.4.79"
        fi
    done
    # credentials MUST never be indexed (§11.4.10)
    for sec in "%.env" "%.pem" "%.key"; do
        cnt=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM files WHERE path LIKE '$sec';" 2>/dev/null || echo "ERR")
        if [ "$cnt" = "0" ]; then
            pass "live DB: 0 indexed files matching $sec (§11.4.10)"
        elif [ "$cnt" = "ERR" ]; then
            skip "live DB query failed for $sec"
        else
            fail "live DB: $cnt credential-like files ($sec) ARE indexed — §11.4.10 leak risk"
        fi
    done
else
    skip "LIVE index audit (sqlite3 or codegraph.db missing)"
fi

# Summary
echo ""
echo "=== Validation Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  SKIP: $SKIP"

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "❌ VALIDATION FAILED"
    exit 1
else
    echo ""
    echo "✅ VALIDATION PASSED"
    exit 0
fi
