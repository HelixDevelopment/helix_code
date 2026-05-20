#!/usr/bin/env sh
# ============================================================================
# register-qwen.sh    —  CodeGraph MCP registration for Qwen Code  (task CG9)
# ----------------------------------------------------------------------------
# Purpose : Idempotently register the CodeGraph MCP stdio server with Qwen
#           Code by merging an `mcpServers.codegraph` entry into the project
#           `.qwen/settings.json`. Re-running is safe — the entry is
#           overwritten in place, no other settings are touched.
# Authority: Cascaded from HelixCode CLAUDE.md / CONSTITUTION.md. Subordinate
#           to constitution/ submodule (CONST-035, §11.4.18, §11.4.67).
# Plan    : docs/research/codegraph/incorporation-plan.md §4.5, §6 Phase B.
# Inputs  : none. Outputs: <repo-root>/.qwen/settings.json (merged).
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

QWEN_DIR="$REPO_ROOT/.qwen"
QWEN_FILE="$QWEN_DIR/settings.json"
mkdir -p "$QWEN_DIR"

CG_BIN="$CG_BIN" REPO_ROOT="$REPO_ROOT" QWEN_FILE="$QWEN_FILE" node <<'NODE'
const fs = require('fs');
const { CG_BIN, REPO_ROOT, QWEN_FILE } = process.env;

let cfg = {};
if (fs.existsSync(QWEN_FILE)) {
  const raw = fs.readFileSync(QWEN_FILE, 'utf8').trim();
  if (raw) cfg = JSON.parse(raw);
}
cfg.mcpServers = cfg.mcpServers || {};
cfg.mcpServers.codegraph = {
  command: CG_BIN,
  args: ['serve', '--mcp', '--path', REPO_ROOT],
  timeout: 15000,
};
fs.writeFileSync(QWEN_FILE, JSON.stringify(cfg, null, 2) + '\n');
console.log('wrote ' + QWEN_FILE);
NODE

echo "register-qwen.sh: CodeGraph registered for Qwen Code."
