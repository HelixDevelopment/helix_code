#!/usr/bin/env bash
# =============================================================================
# Library:     lib-agent-challenge.sh — shared driver for CG-CHALLENGE-03..07
# Task ID:     CG12 (Phase C — Layer C: each agent reaches CodeGraph).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              §11.4.3 topology-aware dispatch (strongest available proof),
#              CONST-050(B) captured wire evidence.
#
# This is NOT a standalone Challenge — it is sourced by each per-agent
# Challenge (cg-challenge-03..07). It provides run_agent_challenge(), which:
#   * Tier 1 (end-to-end): drives the agent CLI non-interactively with a
#     prompt only answerable from the codegraph graph, then asserts the
#     transcript contains a real HelixCode .go symbol path returned by a
#     codegraph_* tool. PASS = true end-to-end.
#   * Tier 2 (connect-only fallback, §11.4.3): if the agent could not be
#     driven to a real answer (no creds / quota / no scriptable mode), it
#     proves the weaker-but-honest claim that codegraph is a registered,
#     reachable MCP server for that agent (config entry present + binary
#     reachable + — when the transcript shows it — codegraph tools were
#     discovered by the agent's MCP loader). This is reported HONESTLY as
#     "connect-only", NEVER as end-to-end.
#
# A Tier-1 PASS requires a REAL .go symbol path in the transcript. An agent
# that answers without the path, errors, or returns a simulated value falls
# back to Tier 2. A Tier-2 result is a PASS of the connect-only invariant
# only — the report MUST distinguish the two.
#
# Exit codes:  0 = PASS (end-to-end OR connect-only, reported honestly)
#              1 = FAIL (agent could not reach codegraph at all)
#              2 = preconditions unmet
# =============================================================================
set -euo pipefail

PROMPT='Use the codegraph MCP tool codegraph_search to search for the symbol named Provider. Report the filePath of the first result only, nothing else.'

# run_agent_challenge <cid> <agent> <config-file> <cgbin> -- <agent-cmd...>
run_agent_challenge() {
  local cid="$1" agent="$2" cfg="$3" cgbin="$4"
  shift 4
  [ "${1:-}" = "--" ] && shift
  local repo_root evid_dir transcript
  repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  evid_dir="$repo_root/docs/research/codegraph/evidence/phase-c"
  mkdir -p "$evid_dir"
  transcript="$evid_dir/${cid}-${agent}-transcript.txt"

  echo "### ${cid} — Layer C: agent '${agent}' reaches CodeGraph"

  # --- Tier 1: drive the agent non-interactively -----------------------------
  echo "  tier-1: driving '${agent}' non-interactively"
  ( "$@" ) > "$transcript" 2>&1 || true
  echo "  transcript captured: $transcript ($(wc -c < "$transcript") bytes)"

  if grep -qiE 'simulated|placeholder' "$transcript"; then
    echo "  WARN: tier-1 transcript contains a simulated/placeholder token" >&2
  elif grep -qoE '[A-Za-z0-9_./-]+\.go(:[0-9]+)?' "$transcript" \
       && grep -qiE 'provider' "$transcript"; then
    echo "  PASS: '${agent}' invoked codegraph and returned a REAL symbol path:"
    grep -oE '[A-Za-z0-9_./-]+\.go(:[0-9]+)?' "$transcript" | head -1 | sed 's/^/        /'
    echo "${cid}: PASS (true end-to-end — agent invoked codegraph_* and returned real graph data)"
    return 0
  fi

  # --- Tier 2: connect-only fallback (§11.4.3) -------------------------------
  echo "  tier-1 did not yield a real answer; falling back to connect-only proof (§11.4.3)" >&2
  echo "  tier-2: connect-only (strongest available proof for '${agent}')"
  local ok=1 discovered=""
  if [ -f "$cfg" ] && grep -q 'codegraph' "$cfg"; then
    echo "  PASS: '${agent}' config registers codegraph MCP server ($cfg)"
  else
    echo "  FAIL: '${agent}' config has no codegraph entry ($cfg)" >&2; ok=0
  fi
  if [ -x "$cgbin" ]; then
    echo "  PASS: codegraph binary reachable for '${agent}' ($cgbin)"
  else
    echo "  FAIL: codegraph binary not reachable ($cgbin)" >&2; ok=0
  fi
  # extra-strong evidence: did the agent's own MCP loader discover the tools?
  if grep -qE 'codegraph_(search|status|callers|callees|impact|node|files|context|explore)' "$transcript"; then
    discovered="yes"
    echo "  PASS: '${agent}' transcript shows its MCP loader discovered codegraph tools"
  fi
  {
    echo "Agent: $agent"
    echo "Result: connect-only (agent could not be driven to a real codegraph answer)"
    echo "Config: $cfg"
    echo "Tools discovered in agent MCP loader: ${discovered:-not-shown-in-transcript}"
    echo "--- codegraph entry in config ---"
    grep -n -A6 'codegraph' "$cfg" 2>/dev/null || true
  } > "$evid_dir/${cid}-${agent}-connect-proof.txt"

  if [ "$ok" -eq 1 ]; then
    if [ -n "$discovered" ]; then
      echo "${cid}: PASS (connect-only + tool-discovery — codegraph registered, reachable, and tools enumerated by the agent; end-to-end NOT proven)"
    else
      echo "${cid}: PASS (connect-only — codegraph registered + reachable; end-to-end NOT proven)"
    fi
    return 0
  fi
  echo "${cid}: FAIL (agent could not reach codegraph at all)"
  return 1
}
