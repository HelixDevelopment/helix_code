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
- Project brand colors from canonical assets (assets/Logo.png)

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

## Sources verified 2026-06-22: https://github.com/nexu-io/open-design , https://registry.npmjs.org/open-design-mcp/latest

Cross-referenced this doc against the latest official OpenDesign sources (fetched
2026-06-22). Findings:
- **MCP package version current.** npm `open-design-mcp` latest = **0.16.1**
  (registry `latest` tag; description "MCP stdio server bridging coding agents to
  Open Design daemon (BYOK flow with full systemPrompt fidelity)."). This doc's
  "open-design-mcp … Version: 0.16.1" claim is accurate.
- **Components + daemon port confirmed.** The upstream README confirms the two
  components this doc lists: an Express + SQLite **daemon** on port **7456**
  (`http://localhost:7456`) and a **stdio MCP server**. It also corroborates the
  150+ design-system `DESIGN.md` brand files and 100+ skills this doc cites
  (upstream states "100+ Skills", "150+ Design Systems", "261 Plugins").
- **Negative finding (install method drift).** The upstream now documents a
  one-line installer — `curl -fsSL https://open-design.ai/install.sh | sh -s
  <agent>` — and from-source guidance of **Node ~24 + pnpm 10.33.x** (this doc
  says `pnpm install`). The doc's `npm install -g open-design-mcp` still yields
  the published 0.16.1 MCP server, so it is not wrong, but the upstream
  curl-installer is the now-preferred path and the from-source toolchain pins
  (Node 24 / pnpm 10.33.x) should be reflected on the next revision.
