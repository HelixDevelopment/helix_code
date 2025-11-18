# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform built in Go that enables intelligent task division, work preservation, and cross-platform development workflows. It features:

- **Distributed Computing**: SSH-based worker pools with automatic management and health monitoring
- **Multi-Provider LLM Integration**: Support for local providers (Llama.cpp, Ollama, vLLM, LocalAI, LM Studio, Jan, GPT4All, etc.) and cloud APIs (OpenAI, Anthropic, Gemini, Vertex AI, Qwen, xAI, OpenRouter, Copilot, Bedrock, Azure, Groq)
- **Development Workflows**: Automated planning, building, testing, refactoring, debugging, and deployment workflows
- **Task Management**: Intelligent task division with dependency tracking, checkpointing, and automatic rollback
- **MCP Protocol**: Full Model Context Protocol implementation with multi-transport support
- **Multi-Client Architecture**: REST API, CLI, Terminal UI, Desktop, WebSocket, and mobile framework support
- **Memory Systems**: Integration with Mem0, Zep, Memonto, and BaseAI for long-term memory and context management

## Essential Build Commands

**IMPORTANT**: All build commands must be run from the `HelixCode/` subdirectory (not the repository root).

```bash
# Navigate to the Go module directory
cd HelixCode

# Build the main server binary
make build                    # Builds to bin/helixcode

# Testing
make test                     # Run all tests with go test -v ./...
go test -v ./internal/auth    # Test specific package
go test -cover ./...          # Run with coverage
./run_tests.sh                # Run unit tests via script
./run_all_tests.sh            # Run all tests (unit + integration + e2e)

# Code quality
make fmt                      # Format code with go fmt
make lint                     # Lint with golangci-lint (if installed)

# Development
make dev                      # Build and run (NOTE: config/dev/config.yaml doesn't exist, uses default config)
make clean                    # Clean build artifacts (bin/, dist/, coverage.out)

# Production builds (cross-platform)
make prod                     # Build for Linux, macOS, Windows

# Platform-specific builds
make aurora-os                # Build Aurora OS client to bin/aurora-os
make harmony-os               # Build Harmony OS client to bin/harmony-os

# Mobile builds (requires gomobile)
make mobile-init              # Initialize gomobile
make mobile-ios               # Build iOS framework (HelixCore.xcframework)
make mobile-android           # Build Android AAR (mobile.aar)
make mobile                   # Build all mobile bindings

# Assets and documentation
make logo-assets              # Generate logo assets from scripts/logo/generate_assets.go
make sync-manual              # Sync user manual to website
make manual-html              # Convert manual to HTML (requires pandoc)
make docs                     # Build all documentation
make release                  # Full release: clean, logo-assets, docs, build, test
```

## Architecture Overview

### Core Service Layers

**Application Entry Points** (`cmd/`):
- `cmd/server`: HTTP server with REST API and WebSocket support
- `cmd/cli`: Command-line interface client
- Additional applications in `applications/`: terminal-ui, desktop, aurora-os, harmony-os

**Service Layer** (`internal/`):
- `internal/auth`: JWT-based authentication with session management
- `internal/worker`: SSH-based distributed worker pool with auto-installation
- `internal/task`: Task management with checkpointing, dependencies, and queue
- `internal/llm`: Multi-provider LLM integration with unified Provider interface
- `internal/project`: Project lifecycle and session management
- `internal/workflow`: Workflow execution engine with step dependencies
- `internal/notification`: Multi-channel notifications (Slack, Discord, Email, Telegram)
- `internal/mcp`: Model Context Protocol implementation
- `internal/server`: HTTP server, routing, and API handlers
- `internal/memory`: Long-term memory integration (Mem0, Zep, Memonto, BaseAI)
- `internal/agent`: Multi-agent orchestration and coordination
- `internal/tools`: **Comprehensive tool ecosystem** - filesystem (read/write/edit/glob/grep), shell (exec/background), web (fetch/search), browser automation, codebase mapping, multi-file editing
- `internal/editor`: **Multi-format code editing** - supports Diff, Whole file, Search/Replace, and Line-based formats optimized for different LLM models (GPT-4, Claude, Gemini, Llama, etc.)
- `internal/context`: Context builder for AI conversations with fluent API
- `internal/session`: Session tracking and context management
- `internal/config`: Configuration management with Viper

**Data Layer**:
- `internal/database`: PostgreSQL for persistent storage
- `internal/redis`: Redis for caching and real-time state (optional, configurable)

**Cross-Platform**:
- `shared/mobile-core`: Shared code for mobile bindings (iOS, Android)

### Key Architecture Patterns

**Task Distribution**: Tasks are intelligently divided based on complexity and worker capabilities. The `task.Manager` maintains a queue with priority-based scheduling. Each task has:
- Type (planning, building, testing, refactoring, debugging, etc.)
- Priority levels (low, normal, high, critical)
- Status tracking (pending, assigned, running, completed, failed)
- Checkpoint system for work preservation
- Dependency resolution

**Worker Management**: The `worker.SSHWorkerPool` manages distributed workers over SSH. Features include:
- Automatic Helix CLI installation on new workers
- Health monitoring with configurable intervals
- Resource tracking (CPU, memory, GPU)
- Capability-based task assignment
- Connection pooling and retry logic

**LLM Provider Abstraction**: Unified `llm.Provider` interface supports multiple backends:
- Local inference servers: Llama.cpp, Ollama, vLLM, LocalAI, FastChat, text-generation-webui, LM Studio, Jan, GPT4All, KoboldAI, TabbyAPI, MLX, MistralRS
- Cloud APIs: OpenAI, Anthropic, Gemini, Vertex AI, Qwen, xAI, Groq
- Enterprise: Azure OpenAI, AWS Bedrock
- Aggregators: OpenRouter, GitHub Copilot
- All providers implement a common interface with Generate/GenerateStream methods
- Provider selection strategies: performance, cost, availability, round-robin with automatic fallback

**Workflow Execution**: Workflows consist of typed steps (analysis, generation, execution, validation) with actions (analyze_code, generate_code, run_tests, etc.). Steps can have dependencies, forming a DAG that executes in proper order.

**Tool System** (`internal/tools`): Comprehensive tool ecosystem providing AI agents with capabilities including:
- **Filesystem**: fs_read, fs_write, fs_edit, glob, grep with security boundaries and validation
- **Shell**: Command execution (foreground/background), output monitoring, process management
- **Web**: Web scraping, search, markdown parsing with rate limiting
- **Browser**: Chromium-based automation for screenshots, navigation, interaction
- **Codebase**: Mapping and analysis tools for understanding project structure
- **MultiEdit**: Transactional multi-file editing with preview and rollback
- All tools include schema validation, resource limits, timeout enforcement, and audit logging

**Code Editor** (`internal/editor`): Multi-format editing system optimized for different LLM models:
- **Diff Format**: Unix unified diff (best for GPT-4, Gemini Pro, Llama 70B+, DeepSeek)
- **Whole File**: Complete file replacement (best for Gemini Pro, Llama 3 8B, O1 models)
- **Search/Replace**: Pattern-based with regex (best for Claude, GPT-3.5, Mistral, Phi-3)
- **Line-Based**: Specific line range edits (best for GPT-4, Claude, Gemini, CodeLlama)
- Automatic format selection based on model capabilities and file size
- Built-in validation, backup support, and syntax checking for Go/JSON/YAML

**Context Builder** (`internal/context`): Fluent API for building AI conversation context with system roles, messages, metadata, and thread-safe operations. Integrates with memory system for conversation persistence.

## Configuration

Primary configuration at `HelixCode/config/config.yaml`. The system uses Viper for configuration management with environment variable overrides.

**Available Configuration Files**:
- `config/config.yaml`: Main production configuration
- `config/test-config.yaml`: Configuration for testing with simplified settings
- `config/working-config.yaml`: Working configuration for development
- `config/minimal-config.yaml`: Minimal configuration with essential settings only
- `config/fixed-config.yaml`: Fixed configuration for specific environments
- `config/azure_example.yaml`: Example Azure integration configuration
- `config/model-aliases.example.yaml`: Example model alias configuration

**Configuration File Locations** (searched in order):
1. Path specified via command-line flag
2. `./config/config.yaml` (relative to HelixCode/ directory)
3. `./config.yaml`
4. `$HOME/.config/helixcode/config.yaml`
5. `/etc/helixcode/config.yaml`

**Key Configuration Sections**:
- `server`: HTTP server settings (address, port, timeouts)
- `database`: PostgreSQL connection settings
- `redis`: Redis connection settings (optional, can be disabled via `enabled: false`)
- `auth`: JWT authentication settings
- `workers`: Worker pool health checks and concurrency
- `tasks`: Task retry and checkpoint intervals
- `llm`: LLM provider configuration and selection strategy
- `llm.providers`: Individual provider configurations (see config.yaml for full list)
- `notifications`: Multi-channel notification rules and channel configs
- `logging`: Log level, format, and output

**Critical Environment Variables** (override config file):
- `HELIX_AUTH_JWT_SECRET`: JWT signing secret (required for auth)
- `HELIX_DATABASE_PASSWORD`: PostgreSQL password
- `HELIX_DATABASE_HOST`: PostgreSQL host (default: localhost)
- `HELIX_DATABASE_PORT`: PostgreSQL port (default: 5432)
- `HELIX_DATABASE_USER`: PostgreSQL user (default: helixcode)
- `HELIX_DATABASE_NAME`: PostgreSQL database name (default: helixcode)
- `HELIX_REDIS_PASSWORD`: Redis password (if Redis enabled)
- `HELIX_REDIS_HOST`: Redis host (default: localhost)
- `HELIX_REDIS_PORT`: Redis port (default: 6379)

**Notification Channel Environment Variables** (optional):
- `HELIX_SLACK_WEBHOOK_URL`: Slack webhook for notifications
- `HELIX_TELEGRAM_BOT_TOKEN`, `HELIX_TELEGRAM_CHAT_ID`: Telegram bot configuration
- `HELIX_DISCORD_WEBHOOK_URL`: Discord webhook for notifications
- `HELIX_EMAIL_SMTP_SERVER`, `HELIX_EMAIL_SMTP_PORT`, `HELIX_EMAIL_USERNAME`, `HELIX_EMAIL_PASSWORD`, `HELIX_EMAIL_FROM`: SMTP email configuration

## Database Setup

```bash
# Create database and user
createdb helixcode
createuser helixcode

# Set password via environment variable
export HELIX_DATABASE_PASSWORD=your_password

# Schema is automatically created by the application
# Tables: users, workers, tasks, projects, sessions, llm_providers, notifications, etc.
```

**Note**: The database can be made optional for testing by:
1. Leaving the `database.host` empty in config: `host: ""`
2. Setting `database.enabled: false` (if supported in your config version)
3. Using in-memory storage (project persistence will be disabled)

## CLI Usage

The CLI client is at `cmd/cli/main.go`:

```bash
# Build CLI
cd HelixCode
go build -o bin/cli ./cmd/cli

# Interactive mode
./bin/cli

# List workers
./bin/cli --list-workers

# Add SSH worker (auto-installs Helix CLI)
./bin/cli --worker worker-host --user helix --key ~/.ssh/id_rsa

# Generate with LLM
./bin/cli --prompt "Hello world" --model llama-3-8b --max-tokens 1000

# Send notifications
./bin/cli --notify "Build complete" --notify-type "success"

# Health check
./bin/cli --health
```

## Repository Structure

**IMPORTANT**: This repository has a nested structure. The repository root contains documentation and example projects, while the main Go application is in the `HelixCode/` subdirectory.

```
/ (repository root)
├── HelixCode/                    # Main Go application (go.mod is here)
│   ├── cmd/                      # Application entry points
│   │   ├── server/               # Main HTTP server
│   │   └── cli/                  # CLI client
│   ├── applications/             # Platform-specific apps
│   │   ├── terminal-ui/          # Terminal UI (TUI)
│   │   ├── desktop/              # Desktop GUI (Fyne-based)
│   │   ├── android/              # Android application
│   │   ├── ios/                  # iOS application
│   │   ├── aurora-os/            # Aurora OS client (Russian platform)
│   │   └── harmony-os/           # Harmony OS client (Chinese platform)
│   ├── internal/                 # Internal packages (not importable externally)
│   │   ├── auth/                 # Authentication & authorization
│   │   ├── worker/               # Worker pool & SSH management
│   │   ├── task/                 # Task management & checkpoints
│   │   ├── llm/                  # LLM provider implementations
│   │   ├── mcp/                  # MCP protocol
│   │   ├── workflow/             # Workflow engine
│   │   ├── project/              # Project management
│   │   ├── session/              # Session tracking
│   │   ├── memory/               # Long-term memory systems
│   │   ├── agent/                # Multi-agent coordination
│   │   ├── tools/                # Tool calling capabilities
│   │   ├── notification/         # Notification channels
│   │   ├── server/               # HTTP server & routes
│   │   ├── database/             # Database layer
│   │   ├── redis/                # Redis client
│   │   ├── config/               # Configuration management
│   │   └── [... other services]
│   ├── shared/                   # Shared code
│   │   └── mobile-core/          # Mobile platform bindings
│   ├── config/                   # Configuration files
│   ├── scripts/                  # Build and utility scripts
│   ├── docs/                     # Technical documentation
│   ├── tests/                    # Integration and E2E tests
│   ├── go.mod                    # Go module definition
│   ├── Makefile                  # Build system
│   └── README.md                 # HelixCode-specific README
├── Example_Projects/             # Reference implementations
├── Dependencies/                 # Git submodules (LLama_CPP, etc.)
├── Documentation/                # Project-wide documentation
├── Specification/                # Technical specifications
├── Implementation_Guide/         # Implementation guides
├── Design/                       # Design assets
├── README.md                     # Repository overview
└── CLAUDE.md                     # This file
```

**Working Directory**: All Go commands and make targets must be executed from `HelixCode/` subdirectory, not the repository root.

## Development Workflows

The system implements automated workflows for different development phases:

**Planning Mode**: Analyzes requirements, creates technical specifications, breaks down into tasks
**Building Mode**: Code generation, dependency management, integration
**Testing Mode**: Unit tests, integration tests, test execution
**Refactoring Mode**: Code analysis, optimization, restructuring
**Debugging Mode**: Error analysis, root cause identification, fixes
**Deployment Mode**: Build, package, deploy to targets

Each workflow is defined with typed steps and dependencies in `internal/workflow`.

## Testing Patterns

- Unit tests are alongside source files (e.g., `manager_test.go` next to `manager.go`)
- Use testify for assertions: `github.com/stretchr/testify`
- Mock interfaces for database and external services
- Test with `go test -v ./...` from the `HelixCode/` directory

## Key Package Documentation

Several internal packages have detailed README files with comprehensive documentation:
- `internal/editor/README.md`: Complete guide to the multi-format code editing system (276+ tests)
- `internal/tools/README.md`: Tool ecosystem documentation with examples and security guidelines
- `internal/context/README.md`: Context builder API reference and usage patterns

These README files contain important implementation details, usage examples, and best practices that supplement this CLAUDE.md file.

## Module and Dependencies

**Module name**: `dev.helix.code`
**Go version**: 1.24.0

**Core dependencies** (check `go.mod` for complete list):
- `github.com/gin-gonic/gin`: HTTP web framework
- `github.com/google/uuid`: UUID generation
- `github.com/spf13/viper`: Configuration management
- `github.com/golang-jwt/jwt/v4`: JWT authentication
- `github.com/gorilla/websocket`: WebSocket support
- `github.com/jackc/pgx/v5`, `github.com/lib/pq`: PostgreSQL drivers
- `github.com/go-redis/redis/v8`: Redis client
- `github.com/stretchr/testify`: Testing framework
- `github.com/chromedp/chromedp`: Browser automation
- `golang.org/x/crypto`, `golang.org/x/net`: Crypto and networking
- AWS, Azure, Google Cloud SDKs for cloud provider integrations

**Note**: Run `go mod tidy` to ensure all dependencies are properly installed.

## Code Generation

Logo assets are auto-generated before build:
```bash
make logo-assets    # Generates from scripts/logo/generate_assets.go
```

This extracts colors and creates themed variations of the logo.

## Cross-Platform Support

The platform supports multiple deployment targets:
- **Standard Desktop/Server**: Linux, macOS, Windows (via `make prod`)
- **Mobile**: iOS (xcframework) and Android (AAR) via gomobile bindings (`make mobile`)
- **Specialized Platforms**:
  - Aurora OS (Russian mobile platform, via `make aurora-os`)
  - Harmony OS (Chinese ecosystem, via `make harmony-os`)
- **Applications**: Terminal UI, Desktop GUI, CLI client (all built from `applications/` and `cmd/`)

## Important Implementation Notes

- **Nested Repository Structure**: The main Go application is in `HelixCode/` subdirectory. Always `cd HelixCode` before running build/test commands.
- **SSH Worker Auto-Install**: When adding workers via SSH, the system automatically installs the Helix CLI binary on remote machines
- **Task Checkpointing**: Long-running tasks automatically checkpoint at configured intervals (default 300s) for work preservation
- **Provider Fallback**: LLM requests can fall back to alternative providers if the primary fails (configurable via `llm.selection.fallback_enabled`)
- **Health Monitoring**: Workers are health-checked every 30s by default; unhealthy workers are removed from the active pool
- **Session Context**: Development sessions maintain context across interactions for continuity
- **MCP Protocol**: Supports both stdio and SSE transports for Model Context Protocol
- **Database Schema Auto-Init**: The server automatically creates database schema on startup via `db.InitializeSchema()`
- **Database is Optional**: Database can be disabled for testing by leaving `database.host` empty or setting `database.enabled: false`
- **Redis is Optional**: Redis can be disabled by setting `redis.enabled: false` in config; the system will function without it
- **Environment Variables Override Config**: All `HELIX_*` environment variables take precedence over config file values
- **Provider Selection Strategy**: Configurable via `llm.selection.strategy` (performance, cost, availability, round-robin)
- **Editor Format Selection**: The code editor automatically selects the best edit format (Diff/Whole/Search-Replace/Line-based) based on the LLM model being used
- **Tool Security**: All tools implement security boundaries - path validation, command blocklists, resource limits, and audit logging
- **Multi-Edit Transactions**: Use the multiedit tools for atomic multi-file changes with preview and rollback capabilities

## Testing Infrastructure

The project includes multiple test levels:
- **Unit tests**: Alongside source files (`*_test.go`), run with `go test -v ./internal/<package>`
- **Integration tests**: In `tests/` directory
- **E2E tests**: In `test/e2e/` directory
- **Challenge tests**: In `tests/e2e/challenges/` directory (comprehensive system testing)
- **Test helpers**: Mock implementations in `internal/mocks/`
- **Coverage**: Generate with `go test -cover ./...` or check `coverage.out`

**Test Scripts** (run from `HelixCode/` directory):
- `./run_tests.sh`: Run unit tests
- `./run_integration_tests.sh`: Run integration tests
- `./run_all_tests.sh`: Run all tests (unit + integration + e2e)
- `./scripts/run-tests.sh`: Alternative test runner in scripts directory
- `./scripts/run-docker-tests.sh`: Run tests in Docker containers
- `./scripts/run-all-tests.sh`: Comprehensive test suite including Docker tests

**Important Test Configurations**:
- Use `config/test-config.yaml` for test environments
- Use `config/minimal-config.yaml` for minimal testing setups
- Database can be disabled in config for testing (set `database.enabled: false` or leave host empty)
- Redis can be disabled with `redis.enabled: false` in config

## Challenge Testing Framework

**Purpose**: The Challenge Testing Framework validates HelixCode's ability to generate complete, working software projects from prompts. Each challenge represents a real-world project that HelixCode must implement end-to-end.

**Location**: `tests/e2e/challenges/`

### Key Features

- **Comprehensive Validation**: Checks for no placeholders, successful compilation, passing tests, and working applications
- **Multi-Interface Testing**: Tests via CLI, TUI, REST API, WebSocket, and Desktop interfaces
- **Multi-Provider Support**: Tests with all LLM providers (Ollama, OpenAI, Anthropic, Gemini, etc.)
- **Distributed Testing**: Supports single instance and distributed worker configurations (2, 5, 10 workers)
- **Full Logging**: Records all requests, responses, and execution details
- **Result Organization**: Structured storage of all generated code and iterations
- **Batch Execution**: Run multiple challenges across all combinations of interfaces/providers/models

### Challenge Definitions

Three example challenges are included:
- **notes-project-001**: Simple Notes Application (CRUD, PostgreSQL, REST API)
- **url-shortener-001**: URL Shortener Service (Redis, PostgreSQL, Analytics)
- **cli-task-manager-001**: CLI Task Manager (Cobra, Local Storage)

Add challenges in `tests/e2e/challenges/definitions/` as JSON files.

### Running Challenges

```bash
cd tests/e2e/challenges

# List available challenges
go run cmd/runner/main.go -list

# Run single challenge
go run cmd/runner/main.go \
  -challenge notes-project-001 \
  -interfaces cli \
  -providers ollama \
  -models llama2

# Run full test suite
go run cmd/runner/main.go \
  -interfaces cli,tui,rest \
  -distributions single,worker_2,worker_5 \
  -providers ollama,openai,anthropic \
  -models llama2,gpt-4,claude-3-sonnet \
  -export-report ./results/full-report.json
```

### Distributed Worker Testing

Start distributed workers with Docker Compose:

```bash
# 2 workers
docker-compose -f tests/e2e/challenges/docker-compose-workers.yml up -d \
  --scale helixcode-worker=2

# 5 workers
docker-compose -f tests/e2e/challenges/docker-compose-workers.yml up -d \
  --scale helixcode-worker=5

# 10 workers
docker-compose -f tests/e2e/challenges/docker-compose-workers.yml up -d \
  --scale helixcode-worker=10

# Run distributed tests
cd tests/e2e/challenges
go run cmd/runner/main.go -distributions worker_2,worker_5,worker_10
```

### Validation Checks

Each challenge execution is validated for:

1. **Directory Structure**: Result directory exists with required files/directories
2. **Code Quality**: No TODO/FIXME/placeholders, no empty implementations
3. **Compilation**: Code compiles successfully without errors
4. **Testing**: Tests exist and pass with adequate coverage
5. **Functionality**: Application starts and runs correctly
6. **Documentation**: README, Dockerfile, and other required files present

### Results and Logging

All challenge executions are logged and organized:

```
test-results/
├── challenges/{challenge-id}/{interface}_{provider}_{model}_{timestamp}_{id}/
│   ├── [Generated project files]
│   └── execution-metadata.json
├── logs/{execution-id}/
│   ├── execution.log      # Main execution log
│   ├── requests.log        # All LLM requests/responses
│   └── validation.log      # Validation results
└── state/
    ├── challenges.json     # All challenge definitions
    ├── executions.json     # All execution records
    └── batches.json        # Batch execution records
```

### Key Implementation Files

- `tests/e2e/challenges/types.go`: Core type definitions and enums
- `tests/e2e/challenges/validator.go`: Code validation logic (placeholder detection, compilation, tests)
- `tests/e2e/challenges/executor.go`: Challenge execution engine
- `tests/e2e/challenges/manager.go`: Challenge and batch management
- `tests/e2e/challenges/cmd/runner/main.go`: CLI test runner
- `tests/e2e/challenges/docker-compose-workers.yml`: Distributed worker setup
- `tests/e2e/challenges/definitions/*.json`: Challenge specifications

### Creating New Challenges

1. Create JSON definition in `tests/e2e/challenges/definitions/`
2. Specify requirements (compilation, tests, files, etc.)
3. Define detailed prompt
4. Run with: `go run cmd/runner/main.go -challenge your-challenge-id`

See `tests/e2e/challenges/README.md` for complete documentation.
