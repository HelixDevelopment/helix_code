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

Upstream now documents a one-line installer as the **preferred** path; a
from-source / Docker path remains available. Both are described below.

### Preferred: one-line curl installer (upstream-recommended)
```bash
# <agent> = claude | codex | cursor | copilot | openclaw | antigravity |
#           gemini | pi | vibe | hermes | cline | trae | opencode | ...
curl -fsSL https://open-design.ai/install.sh | sh -s <agent>
```
This wires Open Design (daemon + MCP adapter) for the named coding agent.

### MCP Server (npm package — already installed in this project)
```bash
npm install -g open-design-mcp
# Version: 0.16.1 (npm `latest`, verified 2026-06-22)
```

### Daemon (manual setup — Docker or from source)
The daemon can be run via Docker or from source:

**Docker:**
```bash
git clone git@github.com:nexu-io/open-design.git /path/to/open-design
cd /path/to/open-design/deploy
cp .env.example .env
docker compose up -d
```

**From source** (requires Node.js ~24 and pnpm 10.33.x per upstream quickstart):
```bash
git clone https://github.com/nexu-io/open-design.git
cd open-design
corepack enable && pnpm install   # pnpm 10.33.x; Node.js ~24
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

Reconciled 2026-06-22: install section updated to add the upstream-preferred
**curl one-line installer** (`curl -fsSL https://open-design.ai/install.sh | sh -s
<agent>`) and the from-source **toolchain pins (Node.js ~24 + pnpm 10.33.x)**; the
existing `npm install -g open-design-mcp` (0.16.1) and Docker paths are retained.
Both the curl-installer and the from-source path are now documented per the
fetched upstream README/quickstart.

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
- **Resolved (install method drift).** The upstream now documents a one-line
  installer — `curl -fsSL https://open-design.ai/install.sh | sh -s <agent>` —
  and from-source guidance of **Node ~24 + pnpm 10.33.x**. The Installation
  section above has been reconciled to lead with the curl-installer (the
  upstream-preferred path) and to pin the from-source toolchain (Node ~24 / pnpm
  10.33.x). The `npm install -g open-design-mcp` path is retained because it
  still yields the published 0.16.1 MCP server.
