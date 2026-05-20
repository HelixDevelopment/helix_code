#!/usr/bin/env sh
# ============================================================================
# register-opencode.sh — CodeGraph MCP registration for OpenCode   (task CG6)
# ----------------------------------------------------------------------------
# Purpose : Idempotently register the CodeGraph MCP stdio server with OpenCode
#           by merging an `mcp.codegraph` entry into the project
#           `opencode.jsonc`. OpenCode collapses binary+args into a single
#           `command` array and gates the server with an `enabled` flag.
#           Re-running is safe — the entry is overwritten in place.
# Authority: Cascaded from HelixCode CLAUDE.md / CONSTITUTION.md. Subordinate
#           to constitution/ submodule (CONST-035, §11.4.18, §11.4.67).
# Plan    : docs/research/codegraph/incorporation-plan.md §4.2, §6 Phase B.
# Inputs  : none. Outputs: <repo-root>/opencode.jsonc (merged).
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
    echo "ERROR: node not found on PATH (required to merge JSONC config)." >&2
    exit 1
fi

OC_FILE="$REPO_ROOT/opencode.jsonc"

CG_BIN="$CG_BIN" REPO_ROOT="$REPO_ROOT" OC_FILE="$OC_FILE" node <<'NODE'
const fs = require('fs');
const { CG_BIN, REPO_ROOT, OC_FILE } = process.env;

// JSONC: strip // and /* */ comments before parse (best-effort, string-safe).
function parseJsonc(raw) {
  let out = '', inStr = false, q = '', i = 0;
  while (i < raw.length) {
    const c = raw[i], n = raw[i + 1];
    if (inStr) {
      out += c;
      if (c === '\\') { out += raw[i + 1] || ''; i += 2; continue; }
      if (c === q) inStr = false;
      i++; continue;
    }
    if (c === '"' || c === "'") { inStr = true; q = c; out += c; i++; continue; }
    if (c === '/' && n === '/') { while (i < raw.length && raw[i] !== '\n') i++; continue; }
    if (c === '/' && n === '*') { i += 2; while (i < raw.length && !(raw[i] === '*' && raw[i + 1] === '/')) i++; i += 2; continue; }
    out += c; i++;
  }
  return JSON.parse(out);
}

let cfg = {};
if (fs.existsSync(OC_FILE)) {
  const raw = fs.readFileSync(OC_FILE, 'utf8').trim();
  if (raw) cfg = parseJsonc(raw);
}
cfg['$schema'] = cfg['$schema'] || 'https://opencode.ai/config.json';
cfg.mcp = cfg.mcp || {};
cfg.mcp.codegraph = {
  type: 'local',
  command: [CG_BIN, 'serve', '--mcp', '--path', REPO_ROOT],
  enabled: true,
};
fs.writeFileSync(OC_FILE, JSON.stringify(cfg, null, 2) + '\n');
console.log('wrote ' + OC_FILE);
NODE

echo "register-opencode.sh: CodeGraph registered for OpenCode."
