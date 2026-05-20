#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-06 — Layer C: Crush reaches CodeGraph
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG12 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B), §11.4.3 topology-aware dispatch.
#
# WHAT IT PROVES:
#   Crush reaches CodeGraph. It drives `crush run` non-interactively with a
#   graph-only prompt. PASS-tier-1 if the answer contains a real .go symbol
#   path; otherwise it falls back to connect-only + tool-discovery proof
#   (Crush's `run` subcommand does not accept the global -y/--yolo auto-
#   approve flag, so an MCP tool call may stall on a permission prompt in
#   non-interactive mode — when that happens the connect-only proof is the
#   strongest honest evidence available).
#
# ANTI-BLUFF:  A config entry alone is NOT a tier-1 PASS. Connect-only is
#              reported HONESTLY and never claimed as end-to-end.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-06-crush.sh
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
. "$SCRIPT_DIR/lib-agent-challenge.sh"

if ! command -v crush >/dev/null 2>&1; then
  echo "PRECONDITION FAIL: crush CLI not on PATH" >&2; exit 2
fi

run_agent_challenge CG-CHALLENGE-06 crush \
  "$REPO_ROOT/.crush.json" "$CG" -- \
  timeout 220 crush run "$PROMPT"
