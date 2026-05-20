#!/usr/bin/env sh
# ============================================================================
# register-crush.sh   —  CodeGraph MCP registration for Crush      (task CG8)
# ----------------------------------------------------------------------------
# Purpose : Idempotently register the CodeGraph MCP stdio server with Crush by
#           merging an `mcp.codegraph` entry (type "stdio") into the project
#           `.crush.json`. A LITERAL absolute path is used — Crush evaluates
#           `$(...)` in its config at load time, so no shell expansion is
#           emitted. Re-running is safe — the entry is overwritten in place.
# Authority: Cascaded from HelixCode CLAUDE.md / CONSTITUTION.md. Subordinate
#           to constitution/ submodule (CONST-035, §11.4.18, §11.4.67).
# Plan    : docs/research/codegraph/incorporation-plan.md §4.4, §6 Phase B.
# Inputs  : none. Outputs: <repo-root>/.crush.json (merged).
# Exit    : 0 on success; non-zero on missing binary / Node / write failure.
# ============================================================================
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)
CG_BIN="$REPO_ROOT/tools/codegraph/node_modules/.bin/codegraph"

if [ ! -x "$CG_BIN" ] && [ ! -f "$CG_BIN" ]; then
    echo "ERROR: codegraph binary not found at $CG_BIN — run install.sh first." >&2
    exit 1
fi
if ! command -v node >/dev/null 2>&1; then
    echo "ERROR: node not found on PATH (required to merge JSON config)." >&2
    exit 1
fi

CRUSH_FILE="$REPO_ROOT/.crush.json"

CG_BIN="$CG_BIN" REPO_ROOT="$REPO_ROOT" CRUSH_FILE="$CRUSH_FILE" node <<'NODE'
const fs = require('fs');
const { CG_BIN, REPO_ROOT, CRUSH_FILE } = process.env;

let cfg = {};
if (fs.existsSync(CRUSH_FILE)) {
  const raw = fs.readFileSync(CRUSH_FILE, 'utf8').trim();
  if (raw) cfg = JSON.parse(raw);
}
cfg['$schema'] = cfg['$schema'] || 'https://charm.land/crush.json';
cfg.mcp = cfg.mcp || {};
cfg.mcp.codegraph = {
  type: 'stdio',
  command: CG_BIN,
  args: ['serve', '--mcp', '--path', REPO_ROOT],
};
fs.writeFileSync(CRUSH_FILE, JSON.stringify(cfg, null, 2) + '\n');
console.log('wrote ' + CRUSH_FILE);
NODE

echo "register-crush.sh: CodeGraph registered for Crush."
