# Tier 3-5 CLI Agents Analysis

> **Date**: 2026-04-04
> **Total Agents**: 35+ additional agents

## Tier 3: Notable/Niche Agents

### Amazon Q
- **Source**: AWS official CLI agent
- **Language**: Rust
- **Features**: AWS integration, cloud-native
- **Priority**: MEDIUM

### Claude Plugins
- **Source**: Claude Code plugin system
- **Language**: TypeScript
- **Features**: Plugin architecture, extensibility
- **Priority**: MEDIUM

### Claude Squad
- **Source**: Multi-agent coordination
- **Language**: TypeScript
- **Features**: Agent swarms, team coordination
- **Priority**: MEDIUM

### Open Interpreter
- **Source**: OpenAI interpreter alternative
- **Language**: Python
- **Features**: Code execution, data analysis
- **Priority**: MEDIUM

### Plandex
- **Source**: Task planning agent
- **Language**: Go
- **Features**: Task decomposition, planning
- **Priority**: MEDIUM

### Ollama Code
- **Source**: Ollama integration
- **Language**: Python
- **Features**: Local model support
- **Priority**: LOW (HelixAgent already supports Ollama)

## Tier 4: Specialized Agents

### Git MCP
- **Source**: Git Model Context Protocol
- **Language**: TypeScript
- **Features**: Git operations via MCP
- **Priority**: LOW

### Postgres MCP
- **Source**: PostgreSQL MCP server
- **Language**: TypeScript
- **Features**: Database operations
- **Priority**: LOW

### Mistral Code
- **Source**: Mistral AI integration
- **Language**: Python
- **Features**: Mistral model support
- **Priority**: LOW

### Qwen Code
- **Source**: Alibaba Qwen integration
- **Language**: Python
- **Features**: Qwen model support
- **Priority**: LOW

### Octogen
- **Source**: Code generation agent
- **Language**: Python
- **Features**: Multi-agent code generation
- **Priority**: LOW

### Nanocoder
- **Language**: TypeScript
- **Features**: Lightweight coding
- **Priority**: LOW

### Taskweaver
- **Source**: Microsoft agent framework
- **Language**: Python
- **Features**: Data analytics focus
- **Priority**: LOW

### Opencode CLI
- **Language**: TypeScript
- **Features**: VS Code extension + CLI
- **Priority**: LOW

### Fauxpilot
- **Source**: GitHub Copilot alternative
- **Language**: Python
- **Features**: Self-hosted completion
- **Priority**: LOW

## Tier 5: Experimental/Minimal

- AIChat, AIAgent, AIChat LLM Functions
- Codai, Codename Goose, Codex Skills
- Conduit, Copilot CLI, Crush
- DeepSeek CLI Youkpan, Get Shit Done
- Mobile Agent, Multiagent Coding
- Noi, Shai, Snow CLI, Superset
- UI/UX Pro Max, Warp, X-CMD, Xela CLI

## Summary

- **Tier 3**: 10 agents (Notable features)
- **Tier 4**: 10 agents (Specialized use cases)
- **Tier 5**: 15+ agents (Experimental/minimal)

## Porting Priority

1. **HIGH**: Tier 1 + Tier 2 agents (already analyzed)
2. **MEDIUM**: Tier 3 agents (amazon-q, claude-plugins, open-interpreter)
3. **LOW**: Tier 4 agents (specialized tools)
4. **VERY LOW**: Tier 5 agents (minimal overlap)

## Recommendation

Focus on **Tier 1 and Tier 2** features first (already done or in progress). Tier 3-5 agents have significant overlap with features already identified from Tier 1-2 agents.
