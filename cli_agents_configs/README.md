# HelixAgent CLI Agent Configurations

This directory contains exported configuration files for all 47 CLI agents that integrate with HelixAgent.

## Overview

Each CLI agent is configured to use HelixAgent as its primary backend, enabling:
- **Ensemble LLM responses** - Multiple providers aggregated
- **MCP tool access** - 45+ tools via Model Context Protocol
- **Debate orchestration** - Multi-agent discussions
- **Container orchestration** - Lazy NVIDIA RAG startup
- **Streaming responses** - Real-time token streaming

## Configuration Files

### Primary Agents (Fully Configured)

| Agent | Config | Features Tested |
|-------|--------|-----------------|
| Claude Code | `claude-code.yaml` | Terminal UI, MCP, Debate, Git |
| Aider | `aider.yaml` | Repo map, Architect mode, Multi-file |
| OpenHands | `openhands.yaml` | Docker, Sandbox, NVIDIA RAG |
| Codex | `codex.yaml` | Agent mode, Tool use |
| Cline | `cline.yaml` | IDE integration, Auto-approve |
| Continue | `continue.yaml` | Open source, Local LLM |
| Gemini CLI | `gemini.yaml` | Multi-modal, Vision |
| Amazon Q | `amazonq.yaml` | Enterprise, Security |
| Kiro | `kiro.yaml` | Context engine |
| Cursor | `cursor.yaml` | Editor, Tab completion |

### All 47 Agents

See `agents-list.txt` for complete list of supported agents.

## Usage

### Generate All Configs

```bash
./bin/helixagent --generate-all-agents --all-agents-output-dir=./cli_agents_configs
```

### Generate Single Agent Config

```bash
./bin/helixagent --generate-agent-config=claude_code
```

### Use Config with Agent

#### Claude Code
```bash
claude --config /etc/helixagent/agents/claude-code.yaml
```

#### Aider
```bash
aider --model helixagent/ensemble --config ~/.config/aider/helixagent.yaml
```

#### OpenHands
```bash
openhands --config ./cli_agents_configs/openhands.yaml
```

## Configuration Structure

Each config file contains:

```yaml
helixagent:
  endpoint: "http://localhost:7061"
  api_key: "your-api-key"
  
  primary_model:
    provider: "ensemble"
    model: "helixagent/ensemble"
    
  mcp:
    enabled: true
    tools: [...]
    
  debate:
    enabled: true
    default_topology: "mesh"

# Agent-specific settings
<agent_name>:
  # Agent-specific configuration
```

## Testing

### Run All CLI Agent Tests

```bash
make test-cli-agents
```

### Test Specific Agent

```bash
./challenges/scripts/cli_agent_test.sh claude_code
./challenges/scripts/cli_agent_test.sh aider
./challenges/scripts/cli_agent_test.sh openhands
```

### Validate Configuration

```bash
./bin/helixagent --validate-agent-config=./cli_agents_configs/claude-code.yaml
```

## Validation Results

See `../challenges/results/cli_agents/` for test results from each agent.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLI Agents (47 Total)                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ Claude   │ │  Aider   │ │OpenHands │ │  Codex   │ ...       │
│  │  Code    │ │          │ │          │ │          │           │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘           │
│       │            │            │            │                  │
│       └────────────┴────────────┴────────────┘                  │
│                         │                                       │
│              Exported Config Files                              │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────┼───────────────────────────────────────┐
│                         ▼                                       │
│                ┌─────────────────┐                              │
│                │   HelixAgent    │                              │
│                │   Ensemble      │                              │
│                │   LLM Service   │                              │
│                └────────┬────────┘                              │
│                         │                                       │
│    ┌────────────────────┼────────────────────┐                  │
│    ▼                    ▼                    ▼                  │
│ ┌────────┐        ┌──────────┐        ┌──────────┐             │
│ │  MCP   │        │ Debate   │        │ NVIDIA   │             │
│ │ Server │        │Orchestr. │        │   RAG    │             │
│ └────────┘        └──────────┘        └──────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

## License

SPDX-FileCopyrightText: 2026 Milos Vasic
SPDX-License-Identifier: Apache-2.0
