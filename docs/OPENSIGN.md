# OpenDesign — HelixCode Integration

## Overview

OpenDesign (https://github.com/nexu-io/open-design) is a local-first, open-source design workspace that turns coding agents into design engines. It provides design artifact generation, design systems, and design tokens.

**Constitution reference:** §11.4.162 (OpenDesign UI design system mandate)

## Components

1. **OD Daemon** — Express + SQLite local server (port 7456)
2. **open-design-mcp** — MCP stdio server bridging agents to the daemon
3. **Design systems** — 150+ DESIGN.md brand files
4. **Skills** — 100+ design prompts/templates

## Installation

### MCP Server (installed)
```bash
npm install -g open-design-mcp
# Version: 0.16.1
```

### Daemon (requires setup)
The daemon can be run via Docker or from source:

**Docker:**
```bash
git clone git@github.com:nexu-io/open-design.git /path/to/open-design
cd /path/to/open-design/deploy
cp .env.example .env
docker compose up -d
```

**From source:**
```bash
git clone git@github.com:nexu-io/open-design.git
cd open-design
corepack enable && pnpm install
pnpm tools-dev run web
```

## MCP Integration

Add to `.mcp.json`:
```json
{
  "mcpServers": {
    "open-design": {
      "command": "npx",
      "args": ["-y", "open-design-mcp"],
      "env": {
        "OD_DAEMON_URL": "http://localhost:7456"
      }
    }
  }
}
```

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `od_list_projects` | List all design projects |
| `od_get_project` | Fetch project + artifact files |
| `od_create_project` | Create new design project |
| `od_save_artifact` | Save HTML artifact to global store |
| `od_save_project_file` | Save file inside a project |
| `od_lint_artifact` | Lint HTML for issues |
| `od_compose_brief` | Format design prompt |
| `od_generate_design` | Generate design via BYOK (requires API key) |

## Design Token Integration (§11.4.162)

Per constitution §11.4.162:
- Use OpenDesign's design tokens/themes for color palette (light + dark)
- Typography scale and spacing
- Component-level design tokens
- Project brand colors from canonical assets (assets/Logo.jpeg)

## BYOK Configuration (for design generation)

To enable `od_generate_design`, add BYOK vars:
```json
{
  "env": {
    "OD_DAEMON_URL": "http://localhost:7456",
    "BYOK_BASE_URL": "https://your-ai-proxy.example.com/v1",
    "BYOK_API_KEY": "<provider-api-key>",
    "BYOK_MODEL": "open-design",
    "BYOK_PROVIDER": "openai"
  }
}
```

## Cross-references
- constitution/Constitution.md §11.4.162 (OpenDesign UI mandate)
- constitution/Constitution.md §11.4.74 (extend-don't-reimplement)
- constitution/Constitution.md §11.4.35 (project-specific configuration)
