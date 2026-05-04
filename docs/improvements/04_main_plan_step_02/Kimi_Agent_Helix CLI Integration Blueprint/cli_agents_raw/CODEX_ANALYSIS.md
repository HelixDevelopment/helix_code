# OpenAI Codex CLI Agent Analysis

> **Tier 1 Agent** - OpenAI's official coding agent
> **Source**: https://github.com/openai/codex
> **Language**: TypeScript (CLI) + Rust (Core)
> **License**: Apache 2.0

## Overview

Codex is OpenAI's official coding agent that runs locally on your machine. It integrates with ChatGPT plans and provides both CLI and desktop app experiences.

## Architecture

```
codex/
├── codex-cli/          # TypeScript CLI frontend
├── codex-rs/           # Rust core implementation
│   ├── core/           # Core logic
│   ├── tui/            # Terminal UI (ratatui)
│   ├── app-server-protocol/  # Protocol definitions
│   └── ...
└── sdk/                # SDK for integrations
```

## Core Features

### 1. Sandboxed Execution
- Uses macOS Seatbelt (`/usr/bin/sandbox-exec`) for sandboxing
- Network-disabled by default in sandbox
- Secure command execution environment

### 2. Multi-Modal Input
- Natural language commands
- File attachments
- Image context support

### 3. Approval System
- Configurable approval policies
- Command execution approval
- Patch application approval

### 4. Git Integration
- Automatic git commits
- Branch management
- Diff viewing

### 5. Collaboration Modes
- Full autonomy mode
- Interactive mode
- Approval-required mode

### 6. TUI (Terminal UI)
- Built with ratatui (Rust)
- Rich interactive interface
- Chat-style interaction
- File browser integration

### 7. Protocol-Based Communication
- JSON-RPC Lite protocol
- Structured request/response
- Thread history management
- Conversation summaries

## Key Files

| File | Lines | Purpose |
|------|-------|---------|
| AGENTS.md | 16,538 | Development guidelines |
| codex-rs/core | ~5000 | Core Rust implementation |
| codex-rs/tui | ~3000 | Terminal UI |
| codex-cli | ~2000 | TypeScript CLI |

## Security Features

1. **Sandboxing**: All commands run in sandbox
2. **Network Control**: Can disable network access
3. **Approval Gates**: Multi-level approval system
4. **Audit Logging**: Complete operation logs

## HelixAgent Integration Points

| Codex Feature | HelixAgent Implementation |
|--------------|---------------------------|
| Sandboxed Execution | ToolExecutor with permission modes |
| Approval System | PermissionMode + rules |
| TUI | Web UI + Terminal UI |
| Git Integration | ToolGit with auto-commit |
| Protocol | API endpoints |
| Sandboxing | Container-based isolation |

## Documentation

- Docs: https://developers.openai.com/codex
- IDE Integration: https://developers.openai.com/codex/ide
- Auth: https://developers.openai.com/codex/auth

## Porting Priority: HIGH

Codex's sandboxed execution model, approval system, and protocol architecture are valuable additions to HelixAgent.
