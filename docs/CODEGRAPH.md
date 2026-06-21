# CodeGraph — HelixCode Integration

## Overview

CodeGraph (`@colbymchenry/codegraph`) is a local SQLite semantic code-knowledge-graph exposed to agents over MCP. It provides 100% local, zero-cloud code intelligence for all CLI agents working on HelixCode.

**Constitution reference:** §11.4.78, §11.4.79, §11.4.80

## Installation

CodeGraph is installed globally:
```bash
npm install -g @colbymchenry/codegraph
# Binary: /Users/milosvasic/.local/bin/codegraph
```

## Configuration

Configuration file: `.codegraph/config.json`

**Included paths** (own-org submodules per §11.4.79):
- `submodules/*` — all own-org submodules
- `constitution/` — constitution submodule
- `helix_code/` — inner Go application

**Excluded paths** (per §11.4.10 + §11.4.79):
- `cli_agents/*`, `cli_agents_resources/*` — third-party
- `dependencies/LLama_CPP`, `dependencies/Ollama`, `dependencies/HuggingFace_Hub` — third-party
- `**/.env`, `**/.env.*`, `**/*.key`, `**/*.pem`, `**/secrets/**` — credentials
- Standard build artifacts, caches, vendor directories

## Usage

```bash
# Index the project
codegraph index

# Check status
codegraph status

# Start MCP server (for CLI agents)
codegraph serve --mcp --path /Volumes/T7/Projects/helix_code
```

## MCP Integration

Configured in `.mcp.json`:
```json
{
  "mcpServers": {
    "codegraph": {
      "type": "stdio",
      "command": "/Users/milosvasic/.local/bin/codegraph",
      "args": ["serve", "--mcp", "--path", "/Volumes/T7/Projects/helix_code"]
    }
  }
}
```

## Regeneration Mechanism (§11.4.77)

The `.codegraph/codegraph.db` file is gitignored. Regeneration:
```bash
codegraph index
```

## Verification

```bash
codegraph status
# Should show: indexed files, total symbols, database size
```

## Cross-references
- constitution/Constitution.md §11.4.78 (CodeGraph mandate)
- constitution/Constitution.md §11.4.79 (own-org submodules included)
- constitution/Constitution.md §11.4.80 (regular update + sync)
- constitution/Constitution.md §11.4.77 (regeneration mechanism)
