#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-07 — Layer C: Qwen Code reaches CodeGraph
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG12 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B), §11.4.3 topology-aware dispatch.
#
# WHAT IT PROVES:
#   Qwen Code reaches CodeGraph. It drives `qwen -p` non-interactively with a
#   graph-only prompt. PASS-tier-1 if the answer contains a real .go symbol
#   path; otherwise it falls back to connect-only + tool-discovery proof
#   (Qwen Code's bundled OAuth free tier was discontinued upstream, so an
#   end-to-end answer may be blocked by missing LLM credentials — when that
#   happens the connect-only proof is the strongest honest evidence).
#
# ANTI-BLUFF:  A config entry alone is NOT a tier-1 PASS. Connect-only is
#              reported HONESTLY and never claimed as end-to-end.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-07-qwen.sh
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
. "$SCRIPT_DIR/lib-agent-challenge.sh"

if ! command -v qwen >/dev/null 2>&1; then
  echo "PRECONDITION FAIL: qwen CLI not on PATH" >&2; exit 2
fi

run_agent_challenge CG-CHALLENGE-07 qwen \
  "$REPO_ROOT/.qwen/settings.json" "$CG" -- \
  timeout 220 qwen -p "$PROMPT" --approval-mode yolo
