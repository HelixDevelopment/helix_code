# Aider CLI Agent Analysis

> **Tier 1 Agent** - Git-native AI pair programming tool
> **Source**: https://github.com/Aider-AI/aider
> **Language**: Python (~20,270 LOC)
> **License**: Apache 2.0

## Overview

Aider is the most popular open-source AI pair programming tool. It allows developers to collaborate with LLMs directly in their terminal, with deep Git integration and support for multiple LLM providers.

**Key Stats:**
- 5.7M+ PyPI installs
- 15B+ tokens processed weekly
- 88% "Singularity" (self-written code)
- Top 20 on OpenRouter

## Core Features

### 1. Repository Mapping (`repomap.py` - 867 lines)
- Maps entire codebase for context
- Tree-sitter based language parsing
- Supports 100+ programming languages
- Intelligent file context selection

### 2. Multiple Coder Modes (`coders/` - ~3000 lines)
- **EditBlock Coder**: Surgical code edits
- **Architect Coder**: High-level design
- **Ask Coder**: Q&A mode
- **Context Coder**: Context-aware coding
- **Patch Coder**: Diff-based editing
- **Search/Replace**: Direct text replacement

### 3. Git Integration (`repo.py` - 622 lines)
- Automatic commits with sensible messages
- Branch management
- Diff viewing
- Conflict resolution
- Git history awareness

### 4. Multi-Provider Support (`models.py` - 1323 lines)
- Claude (Anthropic)
- GPT-4o, o1, o3-mini (OpenAI)
- DeepSeek R1 & Chat
- Local models via Ollama
- 100+ models via OpenRouter

### 5. Slash Commands (`commands.py` - 1712 lines)
- `/add` - Add files to context
- `/drop` - Remove files from context
- `/commit` - Commit changes
- `/diff` - Show diffs
- `/undo` - Undo last change
- `/lint` - Run linters
- `/test` - Run tests
- `/help` - Show help
- `/voice` - Voice input
- `/model` - Switch models
- `/settings` - Configure settings

### 6. Voice-to-Code (`voice.py` - 187 lines)
- Speech recognition
- Voice commands
- Hands-free coding

### 7. Linting & Testing (`linter.py` - 304 lines)
- Automatic linting
- Test execution
- Error fixing loops

### 8. IDE Integration (`watch.py` - 318 lines)
- File watching
- IDE/editor integration
- Comment-driven development

### 9. Web Scraping (`scrape.py` - 284 lines)
- URL content extraction
- Documentation scraping
- Image/web page context

## Architecture

```
aider/
├── main.py           # Entry point (1274 lines)
├── models.py         # LLM abstraction (1323 lines)
├── repo.py           # Git integration (622 lines)
├── repomap.py        # Codebase mapping (867 lines)
├── commands.py       # Slash commands (1712 lines)
├── io.py             # I/O handling (1191 lines)
├── coders/           # Coder implementations
│   ├── base_coder.py
│   ├── editblock_coder.py
│   ├── architect_coder.py
│   └── ...
└── ...
```

## Key Capabilities to Port to HelixAgent

### 1. Repository Mapping
- Tree-sitter integration for code parsing
- Repository structure analysis
- Intelligent context selection

### 2. Edit Block Format
- Surgical code modifications
- Search/replace blocks
- Minimal diff generation

### 3. Git-Native Workflow
- Automatic commit messages
- Branch-aware operations
- Diff-based reviews

### 4. Multi-Modal Support
- Voice commands
- Image context
- URL/web content

### 5. Lint/Test Integration
- Automatic linting
- Test execution
- Fix iteration loops

## Integration Points

| Aider Feature | HelixAgent Implementation |
|--------------|---------------------------|
| Repository Map | Extend ToolSchema with TreeView, Symbols |
| Edit Blocks | ToolEdit with search/replace |
| Git Integration | ToolGit with auto-commit |
| Voice Commands | KAIROS voice mode extension |
| Lint/Test | ToolLint, ToolTest integration |
| Slash Commands | CLI command system |

## Documentation

- Website: https://aider.chat/
- Docs: https://aider.chat/docs/
- LLM Setup: https://aider.chat/docs/llms.html
- Repomap: https://aider.chat/docs/repomap.html

## Porting Priority: HIGH

Aider's git-native workflow, repository mapping, and edit block format are essential features to port to HelixAgent.
