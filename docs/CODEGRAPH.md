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

> **IMPORTANT (verified 2026-06-22):** CodeGraph v1.0.1 is **zero-config** and
> derives its exclusion scope **only from `.gitignore`** (+ `.git/info/exclude`),
> **NOT** from `.codegraph/config.json`. The legacy `config.json` in this repo is
> **inert** — it is no longer read by the tool. Editing it has no effect on what
> gets indexed. The single source of truth for exclusion is the ignore files.
> (Ref: https://colbymchenry.github.io/codegraph/getting-started/configuration/)

**Exclusion source of truth:** `.gitignore` (repo-tracked) and/or
`.git/info/exclude` (local, non-tracking — preferred for paths that must stay
git-tracked, e.g. `cli_agents/`).

**Must be excluded** (per §11.4.10 + §11.4.79) — verify each is actually absent
from the index, NOT merely listed in the inert config:
- `cli_agents/`, `cli_agents_resources/`, `github_pages_website/` — third-party
- nested `submodules/**/submodules/**`, `submodules/**/cli_agents/**` — vendored trees
- generated `**/*.gen.ts`, `**/worker-configuration.d.ts`
- `**/.env`, `**/.env.*`, `**/*.key`, `**/*.pem`, `**/secrets/**` — credentials

**Must be included** (own-org per §11.4.79): `submodules/*`, `constitution/`,
`helix_code/` — confirmed present via live `codegraph_explore`.

> **Anti-bluff note (§11.4):** `codegraph_validate.sh` historically validated
> `config.json`'s lists — which the tool ignores — so a 18/18 PASS did NOT prove
> third-party paths were excluded (36k `cli_agents` files were in fact indexed,
> 4.38 GB DB → #850 watchdog kills). The validator MUST instead query the live DB
> (`SELECT COUNT(*) FROM files WHERE path LIKE '<excluded>/%'` MUST be 0). See
> `docs/research/codegraph_daemon_stability_20260622/findings.md`.

## Tool selection — `codegraph_search` vs `codegraph_explore`

`codegraph_search` is a SQLite **FTS5 implicit-AND symbol-NAME lookup** (matches a
single symbol whose name/qualified-name/signature/docstring contains **all** the
query tokens). A natural-language phrase like `"provider Generate LLM"` correctly
returns **"No results found"** — no single symbol is named that. This is
working-as-designed, **not** a broken index.

- **Name a symbol → `codegraph_search`** (e.g. `OllamaProvider`, `NewOllamaProvider`).
- **Ask a question / explore an area / multi-concept → `codegraph_explore`** (NL surface).

(Ref: `docs/research/codegraph_search_rootcause_20260622/findings.md`.)

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

## Sources verified 2026-06-22: https://registry.npmjs.org/@colbymchenry/codegraph/latest , https://colbymchenry.github.io/codegraph/getting-started/configuration/

Cross-referenced this doc against the latest official CodeGraph sources (fetched
2026-06-22). Findings:
- **Version current.** npm `@colbymchenry/codegraph` latest = **1.0.1** (registry
  `latest` tag; description "Local-first code intelligence for AI agents (MCP).
  Self-contained — bundles its own runtime."). This doc's "CodeGraph v1.0.1"
  claim is accurate.
- **Exclusion source-of-truth confirmed.** The official configuration page
  confirms CodeGraph is "zero-config" with "no config file to write or keep in
  sync" — exclusions are driven by `.gitignore` (honored via git in git repos,
  read directly root+nested in non-git projects) plus built-in defaults
  (`node_modules`, `vendor`, `dist`, `build`, `target`, `.venv`, `Pods`,
  `.next`, files > 1 MB), with `!negation` overrides in `.gitignore`. This
  corroborates the doc's "config.json is inert; `.gitignore`/`.git/info/exclude`
  are the single source of truth" claim — NO contradiction.
- **Negative finding.** The fetched configuration page does not document a
  `codegraph sync` subcommand under that exact name; the §11.4.80 sync flow this
  doc references comes from the constitution-submodule `codegraph_*.sh` wrappers,
  not the upstream CLI surface. Treat the upstream CLI contract as authoritative
  for raw `codegraph` invocations.
