# Continue CLI Agent Analysis

> **Tier 1 Agent** - Open-source AI code assistant for IDEs
> **Source**: https://github.com/continuedev/continue
> **Language**: TypeScript
> **License**: Apache 2.0

## Overview

Continue is an open-source AI code assistant that integrates with VS Code, JetBrains, and other IDEs. It provides code completion, chat, and editing capabilities.

## Core Features

### 1. Universal IDE Support
- VS Code extension
- JetBrains plugin
- Any IDE with LSP

### 2. Code Completion
- Tab-based completion
- Multi-line suggestions
- Context-aware

### 3. Chat Interface
- Inline chat
- Sidebar chat
- Code explanations

### 4. Multi-Provider Support
- OpenAI
- Anthropic
- Ollama
- Custom APIs

### 5. Custom Context Providers
- @file - File context
- @url - Web page context
- @docs - Documentation
- @codebase - Codebase search

### 6. Actions
- /edit - Edit code
- /comment - Add comments
- /doc - Generate documentation
- /test - Generate tests

## Architecture

```
continue/
├── core/               # Core logic
├── extensions/         # IDE extensions
│   ├── vscode/
│   └── intellij/
├── gui/                # Chat interface
└── binary/             # CLI binary
```

## Key Capabilities

1. **Context Providers**: Modular context system
2. **Actions**: Slash command system
3. **Models**: Pluggable LLM support
4. **Completion**: Tab-based coding
5. **Chat**: Conversational interface

## HelixAgent Integration Points

| Continue Feature | HelixAgent Implementation |
|-----------------|---------------------------|
| Context Providers | Tool system |
| Actions | CLI commands |
| Chat | Conversation context |
| Completion | Code completion API |

## Documentation

- Website: https://continue.dev
- Docs: https://docs.continue.dev
- GitHub: https://github.com/continuedev/continue

## Porting Priority: HIGH

Continue's context provider system and action framework are valuable additions.
