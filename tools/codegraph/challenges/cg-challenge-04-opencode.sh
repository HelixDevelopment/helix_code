#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-04 — Layer C: OpenCode reaches CodeGraph
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG12 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B), §11.4.3 topology-aware dispatch.
#
# WHAT IT PROVES:
#   OpenCode actually invokes a codegraph_* MCP tool and returns REAL
#   HelixCode graph data. It drives `opencode run` non-interactively with a
#   prompt only answerable from the indexed graph, then asserts the answer
#   contains a real .go symbol path.
#
# ANTI-BLUFF:  FAILS / falls back if the answer is empty, errors, or has no
#              real symbol path. Config presence alone is NOT end-to-end.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-04-opencode.sh
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
. "$SCRIPT_DIR/lib-agent-challenge.sh"

if ! command -v opencode >/dev/null 2>&1; then
  echo "PRECONDITION FAIL: opencode CLI not on PATH" >&2; exit 2
fi

run_agent_challenge CG-CHALLENGE-04 opencode \
  "$REPO_ROOT/opencode.jsonc" "$CG" -- \
  timeout 220 opencode run "$PROMPT"
