#!/usr/bin/env sh
# ============================================================================
# register-kimi.sh    —  CodeGraph MCP registration for Kimi CLI    (task CG7)
# ----------------------------------------------------------------------------
# Purpose : Idempotently register the CodeGraph MCP stdio server with Kimi CLI
#           by merging an `mcpServers.codegraph` entry into ~/.kimi/mcp.json
#           (Claude-Desktop-compatible shape). NOTE: ~/.kimi/mcp.json lives in
#           the operator HOME directory, OUTSIDE this repo — it is therefore
#           NOT version-controlled; this script IS the committed deliverable
#           that reproduces it. Re-running is safe — entry overwritten in place.
# Authority: Cascaded from HelixCode CLAUDE.md / CONSTITUTION.md. Subordinate
#           to constitution/ submodule (CONST-035, §11.4.18, §11.4.67).
# Plan    : docs/research/codegraph/incorporation-plan.md §4.3, §6 Phase B.
# Inputs  : none. Outputs: $HOME/.kimi/mcp.json (merged, OUTSIDE repo).
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

KIMI_DIR="${HOME}/.kimi"
KIMI_FILE="$KIMI_DIR/mcp.json"
mkdir -p "$KIMI_DIR"

CG_BIN="$CG_BIN" REPO_ROOT="$REPO_ROOT" KIMI_FILE="$KIMI_FILE" node <<'NODE'
const fs = require('fs');
const { CG_BIN, REPO_ROOT, KIMI_FILE } = process.env;

let cfg = {};
if (fs.existsSync(KIMI_FILE)) {
  const raw = fs.readFileSync(KIMI_FILE, 'utf8').trim();
  if (raw) cfg = JSON.parse(raw);
}
cfg.mcpServers = cfg.mcpServers || {};
cfg.mcpServers.codegraph = {
  command: CG_BIN,
  args: ['serve', '--mcp', '--path', REPO_ROOT],
};
fs.writeFileSync(KIMI_FILE, JSON.stringify(cfg, null, 2) + '\n');
console.log('wrote ' + KIMI_FILE);
NODE

echo "register-kimi.sh: CodeGraph registered for Kimi CLI (~/.kimi/mcp.json)."
