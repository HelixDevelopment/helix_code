#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-05 — Layer C: Kimi CLI reaches CodeGraph
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG12 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B), §11.4.3 topology-aware dispatch.
#
# WHAT IT PROVES:
#   Kimi CLI reaches CodeGraph. It drives `kimi --print --yolo -p` non-
#   interactively with a graph-only prompt. PASS-tier-1 if the answer
#   contains a real .go symbol path; otherwise it falls back to connect-only
#   + tool-discovery proof (Kimi's MCP loader enumerating the codegraph_*
#   tools is captured as the strongest available evidence when the agent's
#   own LLM quota blocks an end-to-end answer).
#
# ANTI-BLUFF:  A config entry alone is NOT a tier-1 PASS. The connect-only
#              fallback is reported HONESTLY and never claimed as end-to-end.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-05-kimi.sh
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
. "$SCRIPT_DIR/lib-agent-challenge.sh"

if ! command -v kimi >/dev/null 2>&1; then
  echo "PRECONDITION FAIL: kimi CLI not on PATH" >&2; exit 2
fi

run_agent_challenge CG-CHALLENGE-05 kimi \
  "$HOME/.kimi/mcp.json" "$CG" -- \
  timeout 220 kimi --print --yolo -p "$PROMPT"
