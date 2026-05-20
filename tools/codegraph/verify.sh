#!/usr/bin/env bash
# =============================================================================
# Script:   tools/codegraph/verify.sh
# Purpose:  Anti-bluff end-to-end proof that CodeGraph genuinely works against
#           the real HelixCode repository (CONST-035 / Article XI §11.9).
#           This is the Layer-A verification body for Challenge CG-CHALLENGE-01.
# Task ID:  CG10 (Phase C — anti-bluff verification).
# Authority: Cascaded from HelixCode root CLAUDE.md / CONSTITUTION.md.
#
# WHAT IT DOES (every step captures real runtime output — no metadata-only PASS):
#   1. `codegraph status . --json` must report files>0 AND nodes>0 AND edges>0.
#      A zero-count graph is an explicit FAIL (config-presence is NOT a PASS).
#   2. `codegraph query Provider --json` must return a non-empty result set
#      containing at least one real HelixCode Go symbol path.
#   3. `codegraph context "add a new LLM provider"` must reference real
#      HelixCode files.
#   4. Anti-bluff guard: none of the captured output may contain a
#      `simulated` / `placeholder` result value.
#
# Exit codes: 0 = all layer-A checks PASS with captured evidence
#             1 = at least one check FAILED (codegraph broken / empty / error)
#             2 = preconditions unmet (binary missing, graph not initialized)
# Usage:      tools/codegraph/verify.sh [evidence-dir]
#             evidence-dir defaults to docs/research/codegraph/evidence/phase-c
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CG="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"
EVID_DIR="${1:-$REPO_ROOT/docs/research/codegraph/evidence/phase-c}"
mkdir -p "$EVID_DIR"

PASS=0
FAIL=0
note_pass() { echo "  PASS: $1"; PASS=$((PASS+1)); }
note_fail() { echo "  FAIL: $1" >&2; FAIL=$((FAIL+1)); }

echo "=== CodeGraph Layer-A verification (CG-CHALLENGE-01) ==="
echo "repo-root: $REPO_ROOT"
echo "binary:    $CG"
echo

if [ ! -x "$CG" ]; then
  echo "PRECONDITION FAIL: codegraph binary not found/executable at $CG" >&2
  exit 2
fi
if [ ! -f "$REPO_ROOT/.codegraph/codegraph.db" ]; then
  echo "PRECONDITION FAIL: .codegraph/codegraph.db missing — run codegraph init -i" >&2
  exit 2
fi

# --- Step 1: status must report non-zero counts ------------------------------
echo "[1/3] codegraph status . --json"
STATUS_JSON="$EVID_DIR/cg-challenge-01-status.json"
if ! "$CG" status "$REPO_ROOT" --json > "$STATUS_JSON" 2>/dev/null; then
  note_fail "codegraph status exited non-zero"
  FILES=0; NODES=0; EDGES=0
else
  COUNTS="$(python3 - "$STATUS_JSON" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
def pick(*keys):
    for k in keys:
        v = d.get(k)
        if isinstance(v, (int, float)):
            return int(v)
        if isinstance(v, dict):
            for kk in ('files', 'nodes', 'edges', 'count'):
                if isinstance(v.get(kk), (int, float)):
                    return int(v[kk])
    return 0
files = pick('fileCount', 'files', 'stats', 'statistics', 'counts')
nodes = pick('nodeCount', 'nodes')
edges = pick('edgeCount', 'edges')
print(files, nodes, edges)
PY
)"
  FILES="${COUNTS%% *}"; rest="${COUNTS#* }"; NODES="${rest%% *}"; EDGES="${rest##* }"
  echo "  files=$FILES nodes=$NODES edges=$EDGES"
  if [ "${FILES:-0}" -gt 0 ] && [ "${NODES:-0}" -gt 0 ] && [ "${EDGES:-0}" -gt 0 ]; then
    note_pass "status reports non-zero counts (files=$FILES nodes=$NODES edges=$EDGES)"
  else
    note_fail "status reports zero/empty graph — codegraph scan produced no data"
  fi
fi
echo

# --- Step 2: query must return a real HelixCode Go symbol --------------------
echo "[2/3] codegraph query Provider --json --limit 10"
QUERY_JSON="$EVID_DIR/cg-challenge-01-query-provider.json"
if ! "$CG" query Provider --json --limit 10 > "$QUERY_JSON" 2>/dev/null; then
  note_fail "codegraph query exited non-zero"
else
  HIT="$(python3 - "$QUERY_JSON" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
results = d if isinstance(d, list) else d.get('results', [])
real = 0
for r in results:
    node = r.get('node', r) if isinstance(r, dict) else {}
    fp = node.get('filePath', '')
    lang = node.get('language', '')
    if fp and (fp.endswith('.go') or lang == 'go'):
        real += 1
print(real, len(results))
PY
)"
  REAL_HITS="${HIT%% *}"
  TOTAL_HITS="${HIT##* }"
  echo "  go-symbol hits=$REAL_HITS of total=$TOTAL_HITS"
  if [ "${TOTAL_HITS:-0}" -gt 0 ] && [ "${REAL_HITS:-0}" -gt 0 ]; then
    note_pass "query returned $REAL_HITS real Go 'Provider' symbol(s)"
  else
    note_fail "query returned empty or no real Go symbols"
  fi
fi
echo

# --- Step 3: context must reference real HelixCode files ---------------------
echo "[3/3] codegraph context \"add a new LLM provider\""
CTX_OUT="$EVID_DIR/cg-challenge-01-context.md"
if ! "$CG" context "add a new LLM provider" -p "$REPO_ROOT" > "$CTX_OUT" 2>/dev/null; then
  note_fail "codegraph context exited non-zero"
else
  CTX_BYTES="$(wc -c < "$CTX_OUT")"
  if [ "$CTX_BYTES" -gt 50 ] && grep -qE '\.(go|ts|py|md)' "$CTX_OUT"; then
    note_pass "context built ($CTX_BYTES bytes) referencing real repo files"
  else
    note_fail "context empty or references no real files ($CTX_BYTES bytes)"
  fi
fi
echo

# --- anti-bluff guard: no simulated/placeholder result values ----------------
echo "[guard] anti-bluff token scan of captured evidence"
if grep -liE '"(simulated|placeholder)"' "$STATUS_JSON" "$QUERY_JSON" 2>/dev/null; then
  note_fail "captured output contains a simulated/placeholder result value"
else
  note_pass "no simulated/placeholder result tokens in captured evidence"
fi
echo

echo "=== Layer-A summary: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
  echo "CG-CHALLENGE-01: FAIL"
  exit 1
fi
echo "CG-CHALLENGE-01: PASS — evidence in $EVID_DIR"
exit 0
