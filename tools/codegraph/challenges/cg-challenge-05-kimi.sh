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
# CREDENTIALED PATH (HXC-010):  Kimi's bundled `managed:kimi-code` provider is
#   OAuth/quota-gated. When the operator exports HELIX_CG_OPENAI_API_KEY +
#   HELIX_CG_OPENAI_BASE_URL (an OpenAI-compatible router such as SiliconFlow
#   or OpenRouter), this script drives Kimi against an `openai_legacy` provider
#   so the LLM backend is no longer the limiter and a true tier-1 answer is
#   reachable. The key is read from the environment at runtime ONLY — it is
#   never written into any tracked file (CONST-042).
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

# Credentialed path: operator-supplied OpenAI-compatible router credentials.
# An `openai_legacy` Kimi provider whose base_url + api_key are env-overridden
# (kimi_cli/llm.py augment_provider_with_env_vars). The api_key is NEVER in a
# file: the config-file carries only a placeholder; OPENAI_API_KEY overrides it.
if [ -n "${HELIX_CG_OPENAI_API_KEY:-}" ] && [ -n "${HELIX_CG_OPENAI_BASE_URL:-}" ]; then
  KIMI_CG_CFG="$HOME/.kimi/config-codegraph-or.toml"
  if [ ! -f "$KIMI_CG_CFG" ]; then
    cat > "$KIMI_CG_CFG" <<'TOML'
default_model = "codegraph-router"
default_thinking = false
default_yolo = true

[models.codegraph-router]
provider = "openai-router"
model = "moonshotai/Kimi-K2.6"
max_context_size = 131072
capabilities = ["thinking"]
display_name = "Kimi via OpenAI-compatible router"

[providers.openai-router]
type = "openai_legacy"
base_url = "https://api.siliconflow.com/v1"
api_key = "env-override-placeholder"

[loop_control]
max_steps_per_turn = 30
max_retries_per_step = 3
TOML
  fi
  export OPENAI_API_KEY="$HELIX_CG_OPENAI_API_KEY"
  export OPENAI_BASE_URL="$HELIX_CG_OPENAI_BASE_URL"
  echo "  credentialed path: driving Kimi via openai_legacy router (key from env, never on disk)"
  run_agent_challenge CG-CHALLENGE-05 kimi \
    "$HOME/.kimi/mcp.json" "$CG" -- \
    timeout 280 kimi --print --yolo --config-file "$KIMI_CG_CFG" -p "$PROMPT"
else
  echo "  no HELIX_CG_OPENAI_* creds in env: using bundled provider (quota-gated)"
  run_agent_challenge CG-CHALLENGE-05 kimi \
    "$HOME/.kimi/mcp.json" "$CG" -- \
    timeout 220 kimi --print --yolo -p "$PROMPT"
fi
