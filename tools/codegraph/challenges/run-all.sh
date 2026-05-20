#!/usr/bin/env bash
# =============================================================================
# Script:    tools/codegraph/challenges/run-all.sh
# Purpose:   Run all 7 CodeGraph Challenges (CG-CHALLENGE-01..07) in order and
#            print a per-Challenge PASS/FAIL summary with evidence pointers.
# Task ID:   CG13 (Phase C — registers/executes the CodeGraph Challenge bank).
# Authority: HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#            CONST-050(B) captured wire evidence per check.
#
# Each Challenge captures its own runtime evidence under
# docs/research/codegraph/evidence/phase-c/. This runner is the
# bank-execution entry point referenced by the helix_qa bank
# helix_qa/banks/codegraph-integration.yaml.
#
# Exit codes: 0 = every Challenge PASSed   1 = at least one Challenge FAILed
# Usage:      tools/codegraph/challenges/run-all.sh [summary-file]
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
EVID_DIR="$REPO_ROOT/docs/research/codegraph/evidence/phase-c"
SUMMARY="${1:-$EVID_DIR/cg-challenge-run-summary.txt}"
mkdir -p "$EVID_DIR"

CHALLENGES="
cg-challenge-01-layer-a.sh
cg-challenge-02-layer-b.sh
cg-challenge-03-claude.sh
cg-challenge-04-opencode.sh
cg-challenge-05-kimi.sh
cg-challenge-06-crush.sh
cg-challenge-07-qwen.sh
"

total=0; passed=0; failed=0
{
  echo "=== CodeGraph Challenge bank — run $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
  echo
  for c in $CHALLENGES; do
    [ -z "$c" ] && continue
    total=$((total+1))
    log="$EVID_DIR/${c%.sh}-run.log"
    echo "--- running $c ---"
    if bash "$SCRIPT_DIR/$c" > "$log" 2>&1; then
      verdict="$(grep -oE 'CG-CHALLENGE-0[0-9]: PASS[^\n]*' "$log" | tail -1)"
      echo "  PASS  $c  ${verdict:-PASS}"
      passed=$((passed+1))
    else
      verdict="$(grep -oE 'CG-CHALLENGE-0[0-9]: FAIL[^\n]*' "$log" | tail -1)"
      echo "  FAIL  $c  ${verdict:-FAIL}  (see $log)"
      failed=$((failed+1))
    fi
  done
  echo
  echo "=== summary: $passed/$total PASS, $failed FAIL ==="
} | tee "$SUMMARY"

[ "$failed" -eq 0 ]
