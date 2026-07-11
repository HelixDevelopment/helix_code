#!/usr/bin/env bash
# §11.4.78 / §11.4.83 durable dual-challenge evidence capture for OpenDesign.
#
# Re-runnable, self-contained script proving TWO independent unforgeable
# facts in one pass:
#   (1) OpenDesign daemon fact  — GET /api/health + GET /api/projects on the
#       live daemon (:7456) return the ACTUAL seeded 'helixcode-brand'
#       project. This is obtainable ONLY by calling the running OpenDesign
#       daemon — no other component in the repo can produce this string.
#   (2) CodeGraph cross-fact    — `codegraph status` against this repo's
#       live SQLite index (.codegraph/codegraph.db) returns a REAL node
#       count. This is obtainable ONLY by querying the actual built index —
#       no static source inspection produces this number.
#
# Neither fact can be faked by grepping source or reading config; both
# require the respective live service/index to be up and queried for real.
#
# Usage: scripts/opendesign/capture_durable_evidence.sh <output_dir>
#   <output_dir> — where to write od_health.json, od_projects.json,
#                   codegraph_status.txt, and manifest.txt (durable, e.g.
#                   under docs/qa/, per §11.4.83 — never scratchpad).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

OUT_DIR="${1:?usage: $0 <output_dir>}"
mkdir -p "$OUT_DIR"

OD_HOST="${OD_HOST:-127.0.0.1}"
OD_PORT="${OD_PORT:-7456}"
HEALTH_URL="http://${OD_HOST}:${OD_PORT}/api/health"
PROJECTS_URL="http://${OD_HOST}:${OD_PORT}/api/projects"

TS_UTC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
rc=0

echo "[$TS_UTC] capture_durable_evidence: start (out_dir=$OUT_DIR)"

# --- Challenge 1: OpenDesign daemon fact ---------------------------------
echo "[$TS_UTC] challenge 1/2: OpenDesign daemon (GET $HEALTH_URL, GET $PROJECTS_URL)"
if ! curl -sS -m 5 "$HEALTH_URL" >"$OUT_DIR/od_health.json" 2>"$OUT_DIR/od_health.err"; then
  echo "  FAIL: health probe unreachable — see od_health.err"
  rc=1
fi
if ! curl -sS -m 5 "$PROJECTS_URL" >"$OUT_DIR/od_projects.json" 2>"$OUT_DIR/od_projects.err"; then
  echo "  FAIL: projects list unreachable — see od_projects.err"
  rc=1
fi
if [ "$rc" -eq 0 ]; then
  if command -v jq >/dev/null 2>&1; then
    ok="$(jq -r '.ok // empty' "$OUT_DIR/od_health.json" 2>/dev/null)"
    ver="$(jq -r '.version // empty' "$OUT_DIR/od_health.json" 2>/dev/null)"
    seeded="$(jq -r '.projects[]? | select(.id=="helixcode-brand") | .id' "$OUT_DIR/od_projects.json" 2>/dev/null)"
    echo "  health: ok=$ok version=$ver"
    echo "  seeded project present: ${seeded:-<NOT FOUND>}"
    if [ "$ok" != "true" ] || [ "$seeded" != "helixcode-brand" ]; then
      echo "  FAIL: daemon reachable but expected facts not confirmed"
      rc=1
    fi
  else
    echo "  jq not available — raw JSON captured, manual inspection required"
  fi
fi

# --- Challenge 2: CodeGraph cross-fact ------------------------------------
echo "[$TS_UTC] challenge 2/2: codegraph status (live index at $REPO_ROOT/.codegraph)"
if command -v codegraph >/dev/null 2>&1; then
  # Strip ANSI color codes so the committed evidence file is plain-text
  # readable (§11.4.168-style export hygiene); the underlying data is
  # unmodified real tool output.
  if ! codegraph status 2>&1 | sed -E 's/\x1b\[[0-9;]*m//g' >"$OUT_DIR/codegraph_status.txt"; then
    echo "  FAIL: codegraph status returned non-zero — see codegraph_status.txt"
    rc=1
  else
    node_count="$(grep -oP 'Nodes:\s*\K[0-9,]+' "$OUT_DIR/codegraph_status.txt" | head -1)"
    echo "  codegraph node count: ${node_count:-<NOT FOUND>}"
    if [ -z "${node_count:-}" ]; then
      echo "  FAIL: could not parse node count from codegraph status output"
      rc=1
    fi
  fi
else
  echo "  FAIL: codegraph CLI not found on PATH"
  rc=1
fi

{
  echo "captured_at_utc=$TS_UTC"
  echo "host=$(hostname)"
  echo "repo_root=$REPO_ROOT"
  echo "od_health_url=$HEALTH_URL"
  echo "od_projects_url=$PROJECTS_URL"
  echo "exit_code=$rc"
} >"$OUT_DIR/manifest.txt"

echo "[$TS_UTC] capture_durable_evidence: done rc=$rc (see $OUT_DIR)"
exit "$rc"
