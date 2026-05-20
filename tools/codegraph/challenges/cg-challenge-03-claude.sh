#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-03 — Layer C: Claude Code reaches CodeGraph
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG12 (Phase C — anti-bluff verification). PRIMARY agent.
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B), §11.4.3 topology-aware dispatch.
#
# WHAT IT PROVES:
#   Claude Code (the primary HelixCode agent) actually invokes a codegraph_*
#   MCP tool and returns REAL HelixCode graph data. It drives `claude -p`
#   non-interactively with --allowedTools mcp__codegraph__codegraph_search
#   and a prompt only answerable from the indexed graph, then asserts the
#   answer contains a real .go symbol path.
#
# ANTI-BLUFF:  FAILS if the answer is empty, errors, contains a simulated/
#              placeholder value, or has no real symbol path. A config entry
#              alone is NOT a PASS — the agent must invoke the tool and
#              return real graph data.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-03-claude.sh
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
. "$SCRIPT_DIR/lib-agent-challenge.sh"

if ! command -v claude >/dev/null 2>&1; then
  echo "PRECONDITION FAIL: claude CLI not on PATH" >&2; exit 2
fi

run_agent_challenge CG-CHALLENGE-03 claude \
  "$REPO_ROOT/.mcp.json" "$CG" -- \
  timeout 220 claude -p "$PROMPT" \
    --allowedTools "mcp__codegraph__codegraph_search" \
    --permission-mode acceptEdits
