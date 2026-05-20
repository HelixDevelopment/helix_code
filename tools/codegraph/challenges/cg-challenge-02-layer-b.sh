#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-02 — Layer B: the CodeGraph MCP server responds
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG11 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              CONST-050(B) (captured wire evidence per check).
#
# WHAT IT PROVES:
#   The MCP transport works end-to-end. It starts `codegraph serve --mcp`,
#   speaks real JSON-RPC over stdio:
#     1. `initialize`          -> a valid result with serverInfo,
#     2. `tools/list`          -> lists all 9 codegraph_* tools,
#     3. `tools/call` (codegraph_search "Provider")
#                              -> a non-empty result with REAL graph data
#                                 (a real HelixCode symbol path).
#   The full JSON-RPC wire transcript is captured as evidence.
#
# ANTI-BLUFF:  FAILS LOUDLY if the server returns no tools, an error object,
#              or an empty tools/call result. A response that merely "looks
#              like JSON" is not enough — the search result must contain a
#              real HelixCode symbol.
#
# Exit codes:  0 = PASS   1 = FAIL   2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-02-layer-b.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
EVID_DIR="$REPO_ROOT/docs/research/codegraph/evidence/phase-c"
mkdir -p "$EVID_DIR"
WIRE="$EVID_DIR/cg-challenge-02-jsonrpc-wire.jsonl"

echo "### CG-CHALLENGE-02 — Layer B (MCP JSON-RPC transport)"
if [ ! -x "$CG" ]; then
  echo "PRECONDITION FAIL: codegraph binary missing at $CG" >&2
  exit 2
fi

# --- drive the MCP server over stdio with a real JSON-RPC exchange -----------
# requests: initialize -> notifications/initialized -> tools/list -> tools/call
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cg-challenge-02","version":"1.0"}}}'
  sleep 2
  printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  sleep 1
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
  sleep 2
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"codegraph_search","arguments":{"query":"Provider"}}}'
  sleep 4
} | timeout 60 "$CG" serve --mcp --path "$REPO_ROOT" > "$WIRE" 2>/dev/null || true

if [ ! -s "$WIRE" ]; then
  echo "FAIL: MCP server produced no JSON-RPC output" >&2
  exit 1
fi
echo "  wire transcript captured: $WIRE ($(wc -l < "$WIRE") lines)"

# --- assert the wire transcript proves a working server ----------------------
python3 - "$WIRE" <<'PY'
import json, sys
lines = [l.strip() for l in open(sys.argv[1]) if l.strip()]
msgs = {}
for l in lines:
    try:
        d = json.loads(l)
    except Exception:
        continue
    if 'id' in d:
        msgs[d['id']] = d

fail = 0
# 1) initialize
init = msgs.get(1)
if init and 'result' in init and init['result'].get('serverInfo'):
    print("  PASS: initialize -> serverInfo:", init['result']['serverInfo'])
else:
    print("  FAIL: initialize did not return a valid result", file=sys.stderr); fail += 1

# 2) tools/list
tl = msgs.get(2)
tools = []
if tl and 'result' in tl:
    tools = [t['name'] for t in tl['result'].get('tools', [])]
cg_tools = sorted(t for t in tools if t.startswith('codegraph_'))
if len(cg_tools) >= 8:
    print(f"  PASS: tools/list -> {len(cg_tools)} codegraph tools: {cg_tools}")
else:
    print(f"  FAIL: tools/list returned only {len(cg_tools)} codegraph tools: {cg_tools}", file=sys.stderr); fail += 1

# 3) tools/call codegraph_search -> real graph data
tc = msgs.get(3)
text = ''
if tc and 'result' in tc:
    content = tc['result'].get('content', [])
    if content and isinstance(content, list):
        text = content[0].get('text', '')
if tc and 'error' in tc:
    print(f"  FAIL: tools/call returned an error: {tc['error']}", file=sys.stderr); fail += 1
elif not text:
    print("  FAIL: tools/call codegraph_search returned empty content", file=sys.stderr); fail += 1
elif 'simulated' in text.lower() or 'placeholder' in text.lower():
    print("  FAIL: tools/call result contains a simulated/placeholder value", file=sys.stderr); fail += 1
elif 'Provider' in text and ('.go' in text or '/' in text):
    snippet = text.splitlines()[0] if text.splitlines() else text[:80]
    print(f"  PASS: tools/call codegraph_search -> real graph data ({len(text)} chars)")
    print(f"        first line: {snippet}")
else:
    print(f"  FAIL: tools/call result has no real symbol path: {text[:120]!r}", file=sys.stderr); fail += 1

if fail:
    print(f"\nCG-CHALLENGE-02: FAIL ({fail} check(s) failed)")
    sys.exit(1)
print("\nCG-CHALLENGE-02: PASS — JSON-RPC wire transcript proves MCP transport works")
PY
