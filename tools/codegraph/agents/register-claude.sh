#!/usr/bin/env sh
# ============================================================================
# register-claude.sh  —  CodeGraph MCP registration for Claude Code  (task CG5)
# ----------------------------------------------------------------------------
# Purpose : Idempotently register the CodeGraph MCP stdio server with Claude
#           Code by merging an `mcpServers.codegraph` entry into the project
#           `.mcp.json` and an `mcp__codegraph__*` permission into
#           `.claude/settings.json`. Re-running is safe — the entry is
#           overwritten in place, no other keys are touched.
# Authority: Cascaded from HelixCode CLAUDE.md / CONSTITUTION.md. Subordinate
#           to constitution/ submodule (CONST-035 anti-bluff, §11.4.18
#           script-doc-block mandate, §11.4.67 sh -n cleanliness).
# Plan    : docs/research/codegraph/incorporation-plan.md §4.1, §6 Phase B.
# Inputs  : none (paths derived relative to this script).
# Outputs : <repo-root>/.mcp.json, <repo-root>/.claude/settings.json (merged).
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

MCP_FILE="$REPO_ROOT/.mcp.json"
SETTINGS_FILE="$REPO_ROOT/.claude/settings.json"
mkdir -p "$REPO_ROOT/.claude"

CG_BIN="$CG_BIN" REPO_ROOT="$REPO_ROOT" MCP_FILE="$MCP_FILE" \
SETTINGS_FILE="$SETTINGS_FILE" node <<'NODE'
const fs = require('fs');
const { CG_BIN, REPO_ROOT, MCP_FILE, SETTINGS_FILE } = process.env;

function readJson(path) {
  if (!fs.existsSync(path)) return {};
  const raw = fs.readFileSync(path, 'utf8').trim();
  if (!raw) return {};
  return JSON.parse(raw);
}

// --- .mcp.json : mcpServers.codegraph ---
const mcp = readJson(MCP_FILE);
mcp.mcpServers = mcp.mcpServers || {};
mcp.mcpServers.codegraph = {
  type: 'stdio',
  command: CG_BIN,
  args: ['serve', '--mcp', '--path', REPO_ROOT],
};
fs.writeFileSync(MCP_FILE, JSON.stringify(mcp, null, 2) + '\n');
console.log('wrote ' + MCP_FILE);

// --- .claude/settings.json : permissions.allow += mcp__codegraph__* ---
const settings = readJson(SETTINGS_FILE);
settings.permissions = settings.permissions || {};
settings.permissions.allow = settings.permissions.allow || [];
if (!settings.permissions.allow.includes('mcp__codegraph__*')) {
  settings.permissions.allow.push('mcp__codegraph__*');
}
fs.writeFileSync(SETTINGS_FILE, JSON.stringify(settings, null, 2) + '\n');
console.log('wrote ' + SETTINGS_FILE);
NODE

echo "register-claude.sh: CodeGraph registered for Claude Code."
