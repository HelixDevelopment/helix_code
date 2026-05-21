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
# CREDENTIALED PATH (HXC-010):  Qwen Code supports `--auth-type openai` with an
#   OpenAI-compatible provider. When the operator exports HELIX_CG_OPENAI_API_KEY
#   + HELIX_CG_OPENAI_BASE_URL (+ optional HELIX_CG_QWEN_MODEL), this script
#   drives Qwen against that router. The key is supplied via the OPENAI_API_KEY
#   environment variable ONLY — it is NEVER written into the tracked
#   .qwen/settings.json (CONST-042).
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

# Credentialed path: operator-supplied OpenAI-compatible router credentials.
# Qwen Code reads OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL from the
# environment; the key never touches the tracked .qwen/settings.json.
if [ -n "${HELIX_CG_OPENAI_API_KEY:-}" ] && [ -n "${HELIX_CG_OPENAI_BASE_URL:-}" ]; then
  export OPENAI_API_KEY="$HELIX_CG_OPENAI_API_KEY"
  export OPENAI_BASE_URL="$HELIX_CG_OPENAI_BASE_URL"
  QWEN_CG_MODEL="${HELIX_CG_QWEN_MODEL:-Qwen/Qwen3-Coder-30B-A3B-Instruct}"
  export OPENAI_MODEL="$QWEN_CG_MODEL"
  echo "  credentialed path: driving Qwen via --auth-type openai (key from env, never on disk)"
  run_agent_challenge CG-CHALLENGE-07 qwen \
    "$REPO_ROOT/.qwen/settings.json" "$CG" -- \
    timeout 280 qwen --auth-type openai -m "$QWEN_CG_MODEL" --approval-mode yolo --debug -o stream-json -p "$PROMPT"
else
  echo "  no HELIX_CG_OPENAI_* creds in env: using bundled OAuth (discontinued free tier)"
  run_agent_challenge CG-CHALLENGE-07 qwen \
    "$REPO_ROOT/.qwen/settings.json" "$CG" -- \
    timeout 220 qwen -p "$PROMPT" --approval-mode yolo
fi
