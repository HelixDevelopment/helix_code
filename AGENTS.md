# HelixCode Agent Guidelines

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform built in Go that enables intelligent task division, work preservation, and cross-platform development workflows. The project is **FULLY COMPLETE** with all 5 implementation phases successfully finished.

**Key Features:**
- **Distributed Computing**: SSH-based worker pools with auto-installation and health monitoring
- **Multi-Provider LLM Integration**: Support for 15+ providers including local (Llama.cpp, Ollama, vLLM) and cloud APIs (OpenAI, Anthropic Claude, Gemini, xAI, OpenRouter, GitHub Copilot, Azure Bedrock, AWS, VertexAI, Groq, Qwen, KoboldAI)
- **Development Workflows**: Automated planning, building, testing, refactoring, debugging, and deployment
- **Task Management**: Intelligent task division with dependency tracking, checkpointing, and rollback
- **MCP Protocol**: Full Model Context Protocol implementation with stdio and SSE transports
- **Multi-Client Architecture**: REST API, CLI, Terminal UI (tview), Desktop GUI (Fyne), WebSocket, iOS/Android mobile, Aurora OS, Harmony OS
- **Memory Systems**: Integration with 9 external memory providers (Mem0, Zep, Memonto, BaseAI, Character.AI, ChromaDB, FAISS, Pinecone, Qdrant, Weaviate)
- **Advanced Editor**: Multi-format code editing (Diff, Whole File, Search/Replace, Line-based) optimized per LLM model
- **Tools Ecosystem**: 9 comprehensive tool categories with 40+ individual tools
- **Notifications**: Multi-channel support (Slack, Discord, Email, Telegram, Webhooks)

## Project Status: FULLY COMPLETE ✅

All implementation phases have been successfully completed:
- **Phase 1**: Foundation (Database, Authentication, Worker Management, Task Management, REST API, Configuration)
- **Phase 2**: Core Services (Advanced Task Division, LLM Integration, Distributed Computing, MCP Protocol, Multi-Channel Notifications)
- **Phase 3**: Workflows (Project Management, Development Workflows, Session Management, Workflow Execution)
- **Phase 4**: LLM Integration (Hardware Detection, Model Management, Provider Architecture, CLI Interface)
- **Phase 5**: Advanced Features (SSH Worker Pool, Advanced LLM Tooling, Multi-Client Support, MCP Integration, Cross-Platform Support, Mobile Ready)

## Technology Stack

**Core Technologies:**
- **Language**: Go 1.24.0 with toolchain go1.24.9
- **Module**: `dev.helix.code`
- **Database**: PostgreSQL 15+ (optional, can be disabled)
- **Cache**: Redis 7+ (optional)
- **HTTP Framework**: Gin v1.11.0
- **Authentication**: JWT v4.5.2
- **Database Driver**: pgx/v5
- **Configuration**: Viper v1.21.0
- **CLI Framework**: Cobra v1.8.0
- **Testing**: Testify v1.11.1

**UI Technologies:**
- **Desktop**: Fyne v2.7.0
- **Terminal UI**: tview v0.42.0
- **Mobile**: gomobile bindings

**External Integrations:**
- **Browser Automation**: chromedp v0.14.2
- **Web Scraping**: goquery v1.10.3
- **Memory**: Zep Go v3.10.0
- **Tree-sitter**: go-tree-sitter for code analysis

## Essential Build Commands

**CRITICAL**: All commands must be run from the `HelixCode/` subdirectory (not repository root).

### Core Commands
- **Build**: `make build` (generates logo assets and builds to bin/helixcode)
- **Test all**: `make test` or `go test -v ./...`
- **Test single**: `go test -v -run TestName ./path/to/package`
- **Test comprehensive**: `./run_tests.sh` (full test suite with multiple test types)
- **Test all variants**: `./run_all_tests.sh` (comprehensive API key management tests)
- **Lint**: `make lint` or `golangci-lint run ./...`
- **Format**: `make fmt` or `go fmt ./...`
- **Clean**: `make clean` (removes bin/, dist/, coverage.out)

### Development Workflow
- **Dev server**: `make dev` (builds and runs with config/dev/config.yaml)
- **Logo assets**: `make logo-assets` (required before first build)
- **Setup deps**: `make setup-deps` or `go mod tidy`
- **Full dev setup**: `make dev-setup` (dependencies + logo processing)

### Specialized Builds
- **Production**: `make prod` (cross-platform builds for Linux, macOS, Windows)
- **Mobile**: `make mobile` (builds iOS framework and Android AAR)
- **Mobile individual**: `make mobile-ios`, `make mobile-android`
- **Aurora OS**: `make aurora-os` (Russian platform client)
- **Harmony OS**: `make harmony-os` (Chinese platform client)
- **Both specialized**: `make aurora-harmony` (both Aurora and Harmony OS)
- **Full release**: `make release` (clean + assets + docs + build + test)

### Testing Variations
- **Unit tests**: `./run_tests.sh --unit`
- **Integration**: `./run_tests.sh --integration`
- **E2E**: `./run_tests.sh --e2e`
- **Coverage**: `./run_tests.sh --coverage` (generates HTML report)
- **Benchmarks**: `./run_tests.sh --benchmarks`
- **Security**: `./run_tests.sh --security`
- **Hardware automation**: `./run_tests.sh --automation`
- **Challenge tests**: `cd tests/e2e/challenges && go run cmd/runner/main.go`
- **Specific timeout**: `export TEST_TIMEOUT=30s && ./run_tests.sh --unit`
- **Skip expensive**: `./run_tests.sh --skip-expensive`
- **Skip hardware**: `./run_tests.sh --skip-hardware`

## Architecture & Code Organization

### Core Structure
```
HelixCode/
├── cmd/                    # Application entry points
│   ├── server/            # Main HTTP server
│   ├── cli/               # CLI client (with root.go commands)
│   ├── security-test/     # Security testing tools
│   ├── performance-optimization/ # Performance optimization tools
│   └── [... other tools]
├── internal/              # Internal packages (not importable externally)
│   ├── auth/              # JWT authentication with session management
│   ├── worker/            # SSH-based distributed worker pool
│   ├── task/              # Task management with checkpointing
│   ├── llm/               # Multi-provider LLM integration (15+ providers)
│   ├── mcp/               # Model Context Protocol implementation
│   ├── workflow/          # Workflow execution engine
│   ├── project/           # Project lifecycle and session management
│   ├── server/            # HTTP server, routing, and API handlers
│   ├── database/          # PostgreSQL layer (optional)
│   ├── redis/             # Redis client (optional)
│   ├── config/            # Configuration management with Viper
│   ├── tools/             # Comprehensive tool ecosystem (40+ tools)
│   ├── editor/            # Multi-format code editing system
│   ├── memory/            # Long-term memory integration (9 providers)
│   ├── notification/      # Multi-channel notifications
│   ├── context/           # Context building with mentions
│   ├── agent/             # AI agent coordination
│   ├── commands/          # Built-in command system
│   ├── discovery/         # Service discovery
│   ├── deployment/        # Production deployment
│   ├── event/             # Event bus
│   ├── focus/             # Focus management
│   ├── hardware/          # Hardware detection
│   ├── hooks/             # System hooks
│   ├── logging/           # Logging system
│   ├── monitoring/        # Monitoring and metrics
│   ├── performance/       # Performance optimization
│   ├── persistence/       # Data persistence
│   ├── providers/         # AI and vector providers
│   ├── repomap/           # Repository mapping
│   ├── rules/             # Rule system
│   ├── security/          # Security management
│   ├── session/           # Session management
│   ├── template/          # Template system
│   ├── version/           # Version management
│   └── [... other services]
├── applications/          # Platform-specific apps
│   ├── desktop/           # Desktop GUI (Fyne-based)
│   ├── terminal-ui/       # Terminal UI (tview)
│   ├── ios/               # iOS application (Swift bindings)
│   ├── android/           # Android application (Kotlin)
│   ├── aurora-os/         # Aurora OS client
│   └── harmony-os/        # Harmony OS client
├── shared/                # Shared mobile code
│   └── mobile-core/       # Gomobile bindings
├── config/               # Configuration files
├── tests/                # Test suites and frameworks
│   ├── e2e/challenges/   # Challenge testing framework
│   ├── integration/       # Integration tests
│   ├── unit/              # Unit tests
│   ├── security/          # Security tests
│   ├── automation/        # Hardware automation tests
│   └── performance/       # Performance benchmarks
├── scripts/              # Build and utility scripts
├── docker/               # Docker configurations
├── assets/               # Static assets (logos, themes)
└── go.mod                # Go module definition (dev.helix.code)
```

### Key Patterns

**Interface-Driven Design:**
- Core interfaces define contracts (Provider, Repository, Manager)
- Multiple implementations per interface (various LLM providers)
- Factory pattern for provider creation
- Easy mocking for unit tests

**Manager Pattern:**
- Centralized managers (TaskManager, WorkerManager, ProviderManager)
- Thread-safe operations with `sync.RWMutex`
- Encapsulate complex business logic

**Repository Pattern:**
- Data access abstraction via interfaces
- Database-agnostic implementations
- Redis caching integrated transparently

## Code Style & Conventions (Go 1.24.0, module: dev.helix.code)

### Import Organization
1. Standard library imports
2. Third-party imports  
3. Internal imports (dev.helix.code/internal/...)
4. Blank line between groups

### Naming Conventions
- **Types**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase, descriptive names
- **Constants**: PascalCase, grouped by functionality
- **Interfaces**: Simple capability names (Provider, Repository, Manager)

### Error Handling
- Return errors with context: `fmt.Errorf("failed to X: %v", err)`
- Check errors immediately after operations
- Use package-level error variables: `ErrInvalidCredentials`
- Structured error responses in HTTP handlers

### Testing Patterns
- **Test files**: Alongside source files (`*_test.go`) or in `tests/` directory
- **Assertions**: Use `github.com/stretchr/testify` - `require.NoError` for critical, `assert.Equal` for comparisons
- **Mocks**: Interface-based mocking using `github.com/stretchr/testify/mock`
- **Test structure**: Table-driven tests with subtests using `t.Run()`
- **Test categories**: Unit, Integration, E2E, Security, Performance, Automation

## Configuration Management

### Primary Configuration
Main config at `config/config.yaml` with environment variable overrides:

```yaml
server:
  address: "0.0.0.0"
  port: 8080

database:
  host: ""  # Empty to disable for testing
  port: 5432
  user: "helix"
  # Password via HELIX_DATABASE_PASSWORD
  dbname: "helixcode_prod"
  sslmode: "disable"

redis:
  host: "redis"
  port: 6379
  password: "redispass"
  enabled: true

auth:
  jwt_secret: "QBHQ2paeBWWnOgniSQLqh1Dsd+pumKOcUTZbTXB+N0g="
  # Or via HELIX_AUTH_JWT_SECRET
  token_expiry: 86400
  session_expiry: 604800
  bcrypt_cost: 12
  
workers:
  health_check_interval: 30
  health_ttl: 120
  max_concurrent_tasks: 10
  
tasks:
  max_retries: 3
  checkpoint_interval: 300
  cleanup_interval: 3600
  
llm:
  default_provider: "local"
  max_tokens: 4096
  temperature: 0.7
  timeout: 30
  max_retries: 3
  
  providers:
    # Local providers: ollama, llamacpp, vllm
    # Cloud providers: openai, anthropic, gemini, xai, openrouter, etc.
```

### Environment Variables
**Required for Production:**
- `HELIX_DATABASE_PASSWORD`: PostgreSQL password
- `HELIX_AUTH_JWT_SECRET`: JWT signing secret
- `HELIX_REDIS_PASSWORD`: Redis password (if enabled)

**LLM Provider Keys:**
```bash
# Free providers (optional for higher limits)
export GITHUB_TOKEN="ghp_your_github_token"         # GitHub Copilot
export OPENROUTER_API_KEY="sk-or-your-key"          # OpenRouter
export XAI_API_KEY="xai-your-key"                    # XAI/Grok

# Premium providers
export ANTHROPIC_API_KEY="sk-ant-your-key"          # Anthropic Claude
export GEMINI_API_KEY="your-gemini-key"              # Google Gemini
export OPENAI_API_KEY="sk-your-openai-key"           # OpenAI
export AWS_ACCESS_KEY_ID="your-access-key"            # AWS Bedrock
export AWS_SECRET_ACCESS_KEY="your-secret-key"        # AWS Bedrock
export AZURE_CLIENT_ID="your-client-id"               # Azure
export AZURE_CLIENT_SECRET="your-client-secret"       # Azure
export AZURE_TENANT_ID="your-tenant-id"             # Azure
```

**Notification Channels:**
```bash
export HELIX_SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
export HELIX_TELEGRAM_BOT_TOKEN="your_bot_token"
export HELIX_TELEGRAM_CHAT_ID="your_chat_id"
export HELIX_EMAIL_SMTP_SERVER="smtp.gmail.com"
export HELIX_EMAIL_USERNAME="your_email"
export HELIX_EMAIL_PASSWORD="your_app_password"
```

### Database Setup
```bash
# Optional - can be disabled for testing
createdb helixcode_prod
createuser helix
export HELIX_DATABASE_PASSWORD=your_password
# Schema auto-created by application
```

**Database is Optional**: Can be disabled for testing by setting `database.host: ""`

## Testing Approach

### Test Categories
- **Unit tests**: Alongside source files (`*_test.go`) or in `tests/unit/`
- **Integration tests**: In `tests/integration/` directory  
- **E2E tests**: Full workflow testing in `tests/e2e/`
- **Security tests**: OWASP compliance in `tests/security/`
- **Performance tests**: Benchmarking in `tests/performance/`
- **Automation tests**: Hardware automation in `tests/automation/`
- **Challenge tests**: Comprehensive project generation validation in `tests/e2e/challenges/`

### Key Test Files
- `run_tests.sh`: Comprehensive test runner with multiple test types and reporting
- `run_all_tests.sh`: API key management test suite
- `tests/e2e/challenges/`: Challenge testing framework for real project generation
- Test helpers: `internal/mocks/memory_mocks.go`, `internal/notification/testutil/`

### Running Tests
```bash
# Basic
make test
go test -v ./...

# Comprehensive with coverage and reporting
./run_tests.sh --all

# Specific types
./run_tests.sh --unit --integration --coverage
./run_tests.sh --security --automation

# Challenge tests (validates complete project generation)
cd tests/e2e/challenges
go run cmd/runner/main.go -list
go run cmd/runner/main.go -challenge notes-project-001 -interfaces cli -providers ollama

# Custom timeout
export TEST_TIMEOUT=30s
./run_tests.sh --unit

# Skip expensive tests
./run_tests.sh --skip-expensive --skip-hardware
```

### Test Results
- Reports saved to `test-results/` directory with timestamps
- Coverage reports: HTML and text formats
- Comprehensive test reports with hardware information
- Parallel execution support with configurable job count

## Key Subsystems

### Tools Package (`internal/tools/`)
Comprehensive tool ecosystem for AI agents with security boundaries:
- **Filesystem** (`internal/tools/filesystem/`): fs_read, fs_write, fs_edit, glob, grep
- **Shell** (`internal/tools/shell/`): shell, shell_background, shell_output, shell_kill with sandbox
- **Web** (`internal/tools/web/`): web_fetch, web_search with rate limiting and caching
- **Browser** (`internal/tools/browser/`): browser_launch, browser_navigate, browser_screenshot via chromedp
- **MultiEdit** (`internal/tools/multiedit/`): Transactional multi-file editing with backup
- **Mapping** (`internal/tools/mapping/`): Codebase analysis with treesitter support
- **Git** (`internal/tools/git/`): Git automation, attribution, and smart commits
- **Voice** (`internal/tools/voice/`): Voice input and transcription
- **Confirmation** (`internal/tools/confirmation/`): User interaction with audit trails
- **Notebook**: Jupyter notebook integration

### Editor Package (`internal/editor/`)
Multi-format editing system optimized for different LLM models:
- **Diff Format**: Unix unified diff (best for GPT-4, Gemini Pro, DeepSeek Coder)
- **Whole File**: Complete file replacement (best for Claude, O1 models, Llama 3 8B)
- **Search/Replace**: Pattern-based with regex (best for Claude, GPT-3.5, Mistral)
- **Line-Based**: Specific line range edits (best for GPT-4, Claude, Gemini)
- **Automatic format selection** based on model capabilities
- **Thread-safe** concurrent editing with mutex protection
- **Built-in validation** and backup support

### LLM Package (`internal/llm/`)
Extensive multi-provider integration:
- **Providers**: 15+ providers including OpenAI, Anthropic Claude, Gemini, xAI/Grok, OpenRouter, GitHub Copilot, Qwen, Ollama, Llama.cpp, vLLM, KoboldAI, Azure Bedrock, AWS, VertexAI, Groq
- **Features**: Vision mode switching, cross-provider registry, health monitoring, compression, token budgeting, reasoning modes
- **Free providers**: XAI (Grok), OpenRouter (free models), GitHub Copilot (with subscription), Qwen (2K/day)
- **Advanced**: Anthropic Claude with extended thinking (200K context, 50K output), Gemini with 2M tokens

### Challenge Testing Framework
Located at `tests/e2e/challenges/`:
- Validates ability to generate complete working projects from prompts
- Tests across multiple interfaces (CLI, TUI, REST, WebSocket, Desktop)
- Supports distributed worker testing (2, 5, 10+ workers)
- Comprehensive validation (no placeholders, compiles, tests pass, runs correctly)
- 6-layer validation: Directory structure, compilation, functionality, tests, README, Dockerfile
- Challenge definitions in JSON format with metadata and requirements
- Batch execution and detailed result reporting
- Support for multiple providers and models in matrix testing

## Important Gotchas

### Critical Requirements
- **Always work from HelixCode/ subdirectory** - not repository root
- **Generate logo assets before first build** with `make logo-assets` 
- **Database/Redis are optional** - can be disabled for testing by setting `database.host: ""`
- **Environment variables override config file** - set in shell or `.env`
- **Go version**: Requires Go 1.24.0 with toolchain go1.24.9

### SSH Worker Auto-Install
- When adding workers via SSH, the system automatically installs Helix CLI on remote machines
- Requires SSH key-based authentication (passwordless)
- Workers are health-checked every 30s by default
- Workers can run on any platform with SSH access and Go installed

### Task Checkpointing
- Long-running tasks automatically checkpoint at intervals (default 300s)
- Enables work preservation and recovery from failures
- Checkpoints stored in PostgreSQL for persistence
- Task state includes dependencies, progress, and partial results

### Provider Fallback
- LLM requests can fall back to alternative providers if primary fails
- Configurable provider priority and retry logic (performance, cost, availability, round-robin)
- Automatic rate limiting and quota management
- Cross-provider request sharing and result caching

### Session Context
- Development sessions maintain context across interactions for continuity
- Context stored in Redis for fast access with TTL
- Automatic cleanup of expired sessions
- Context includes file mentions, search results, and conversation history

### MCP Protocol Implementation
- Supports both stdio and SSE transports for Model Context Protocol
- Tool integration and execution through standardized interface
- Real-time bidirectional communication
- Used by AI agents for tool execution and workflow management

### Mobile & Specialized Platforms
- **Gomobile**: iOS framework (.xcframework) and Android AAR from `shared/mobile-core/`
- **Aurora OS**: Russian platform client with specialized UI
- **Harmony OS**: Chinese platform client with native components
- All mobile platforms use shared Go core with platform-specific UI

## Free AI Providers (No API Keys Required)

The system includes multiple free providers out-of-the-box:
- **XAI (Grok)**: grok-3-fast-beta, grok-3-mini-fast-beta, grok-3-beta - Fast and capable
- **OpenRouter**: deepseek-r1-free, meta-llama/llama-3.2-3b-instruct:free - Free models from various providers  
- **GitHub Copilot**: gpt-4o, claude-3.5-sonnet, claude-3.7-sonnet, o1, gemini-2.0-flash - Free with GitHub subscription
- **Qwen**: 2,000 requests/day free tier with OAuth2 authentication

## Premium AI Providers (Advanced Features)

### Anthropic Claude ⭐ Industry-Leading
- **Models**: Claude 4 Sonnet/Opus, Claude 3.7 Sonnet, Claude 3.5 Sonnet/Haiku, Claude 3 Opus/Sonnet/Haiku
- **Context**: 200K tokens (all models)
- **Max Output**: Up to 50K tokens (Claude 4/3.7)
- **Advanced Features**: Extended thinking, prompt caching, 50K output, 200K context

### Google Gemini (2M Token Context)  
- **Models**: Gemini 2.5 Pro, Gemini 2.0 Flash, Gemini 1.5 Pro/Flash
- **Context**: Up to 2M tokens (largest available)
- **Features**: Multimodal, function calling, code execution

### Other Premium Providers
- **OpenAI**: GPT-4o, GPT-4 Turbo, O1 models
- **Azure**: Enterprise-grade OpenAI models with Microsoft infrastructure
- **AWS Bedrock**: Claude, Titan, Jurassic models via AWS
- **VertexAI**: Google's enterprise models
- **Groq**: Ultra-fast inference with Llama and Mixtral

## Memory Integration

External memory providers are integrated via `internal/memory/`:
- **Mem0**: Advanced memory management with embeddings and semantic search
- **Zep**: Long-term conversational memory with message compression
- **Memonto**: Knowledge graph-based memory with relationship mapping
- **BaseAI**: Comprehensive memory platform with tools and analytics
- **ChromaDB**: Vector database for similarity search
- **FAISS**: Facebook AI's vector similarity library
- **Pinecone**: Managed vector database service
- **Qdrant**: Vector database for embeddings
- **Weaviate**: Knowledge graph with vector search
- **Character.AI**: Character-based memory system

## Docker Deployment

### Quick Start with Docker Compose

```bash
# Clone repository  
git clone https://github.com/your-org/helixcode.git
cd HelixCode  # Important: work from HelixCode/ subdirectory

# Configure environment
cp .env.example .env
# Edit .env with your secure passwords

# Start all services
docker-compose up -d

# Check deployment
docker-compose ps
curl http://localhost/health
```

### Services Included
- **helixcode-server**: Main application (ports 8080, 2222) 
- **postgres**: PostgreSQL database (port 5432)
- **redis**: Redis cache (port 6379)
- **nginx**: Reverse proxy (ports 80, 443)
- **prometheus**: Monitoring (port 9090, optional)
- **grafana**: Dashboards (port 3000, optional)

### Production Environment Variables
Required in `.env`:
```bash
HELIX_AUTH_JWT_SECRET=your-super-secure-jwt-secret
HELIX_DATABASE_PASSWORD=your-secure-database-password  
HELIX_REDIS_PASSWORD=your-secure-redis-password
GRAFANA_ADMIN_PASSWORD=your-grafana-password
```

### Docker Features
- **Multi-stage build**: Builder stage with Go 1.24-alpine, production stage with minimal Alpine
- **Health checks**: All services include health endpoints and monitoring
- **Volume mounts**: Persistent data for PostgreSQL, Redis, logs, and SSH keys
- **Non-root user**: Application runs as helixcode (uid: 1001) for security
- **Asset generation**: Logo assets generated during build process

## Common Development Workflows

### Adding New LLM Provider

1. Implement the `Provider` interface in `internal/llm/`:
```go
type Provider interface {
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan *GenerateChunk, error)
    GetCapabilities() *Capabilities
    GetModels() []Model
    ValidateConfig(config map[string]interface{}) error
}
```

2. Add provider configuration in `config/config.yaml`
3. Register provider in `internal/llm/manager.go`
4. Add tests in `internal/llm/provider_test.go`

### Creating New Workflow

1. Define workflow steps in `internal/workflow/`:
```go
type Step struct {
    ID          string
    Type        StepType
    Action      string
    Dependencies []string
    Config      map[string]interface{}
}
```

2. Implement step actions in `internal/workflow/actions.go`
3. Register workflow in `internal/workflow/executor.go`
4. Add workflow tests in `internal/workflow/workflow_test.go`

### Adding New Tool

1. Implement `Tool` interface in `internal/tools/`:
```go
type Tool interface {
    Name() string
    Description() string
    Category() Category
    Schema() *Schema
    Validate(params map[string]interface{}) error
    Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
}
```

2. Register in `internal/tools/registry.go`
3. Add comprehensive tests in `internal/tools/*_test.go`

## Debugging Tips

### Common Issues

1. **Build fails with missing logo assets**:
   ```bash
   make logo-assets
   make build
   ```

2. **Database connection errors**:
   - Check `HELIX_DATABASE_PASSWORD` environment variable
   - Verify PostgreSQL is running on configured port
   - Database can be disabled by leaving `database.host` empty in config

3. **Worker SSH connection issues**:
   - Ensure SSH keys are properly configured
   - Check firewall settings on worker machines
   - Verify worker has Go installed (or enable auto-install)

4. **Test failures related to timing**:
   ```bash
   export TEST_TIMEOUT=30s  # Increase timeout
   ./run_tests.sh --unit
   ```

### Debug Commands

```bash
# Check server health
curl http://localhost:8080/health

# View worker status
./bin/helixcode --list-workers

# Test LLM provider directly
./bin/helixcode --prompt "test" --model llama-3-8b

# Run with verbose logging
HELIX_LOG_LEVEL=debug ./bin/helixcode
```

### Profile & Monitor

```bash
# CPU profiling
go tool pprof http://localhost:8080/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine debugging
go tool trace trace.out
```

## Security Considerations

### Authentication
- JWT tokens for API authentication
- Session management with Redis
- SSH key-based worker authentication

### Input Validation
- Path validation in file operations
- Command blocklist for shell execution
- Schema validation for all tools

### Environment Variables
- Never commit secrets to repository
- Use `.env.example` as template
- All sensitive config via environment variables

### Network Security
- Rate limiting on API endpoints
- CORS configuration for web clients
- TLS enforcement in production

## Additional Important Details

### Configuration File Locations
Configuration files are searched in order:
1. Path specified via command-line flag
2. `./config/config.yaml` (relative to HelixCode/ directory)
3. `./config.yaml`
4. `$HOME/.config/helixcode/config.yaml`
5. `/etc/helixcode/config.yaml`

### Available Configuration Files
- `config/config.yaml`: Main production configuration
- `config/test-config.yaml`: Configuration for testing
- `config/minimal-config.yaml`: Minimal configuration for quick start
- `config/working-config.yaml`: Development configuration
- `config/azure_example.yaml`: Example Azure integration

### Model Aliases
Create custom model aliases in `config/model-aliases.example.yaml`:
```yaml
aliases:
  "gpt4": "openai:gpt-4"
  "claude": "anthropic:claude-3-sonnet"
  "local-llm": "local:llama-3-8b"
```

### Platform-Specific Applications

#### Desktop Application (Fyne-based)
- Location: `applications/desktop/main.go`
- UI Framework: Fyne v2
- Build: `make desktop` or directly with go build
- Features: Complete GUI interface, system tray integration

#### Terminal UI (TUI)
- Location: `applications/terminal-ui/`
- UI Framework: tview
- Build: `make terminal-ui`
- Features: Rich terminal interface with keyboard shortcuts

#### Mobile Applications
- Location: `applications/ios/` and `applications/android/`
- Framework: gomobile bindings from `shared/mobile-core/`
- Build: `make mobile` (generates .xcframework and .aar)

#### Specialized Platforms
- **Aurora OS** (Russian platform): `applications/aurora-os/main.go`
- **Harmony OS** (Chinese platform): `applications/harmony-os/main.go`

### Docker Configuration

#### Multi-stage Build
- Builder stage: Golang 1.24-alpine with build dependencies
- Production stage: Minimal Alpine with runtime dependencies
- Non-root user: helixcode (uid: 1001)
- Assets generated during build process

#### Production Features
- Health checks for all services
- Volume mounts for persistent data
- Reverse proxy configuration via nginx
- Optional monitoring (Prometheus + Grafana)

## Performance & Optimization

### Worker Scaling
- **Single instance**: Direct execution on server
- **Distributed**: SSH-based worker pools (2, 5, 10+ workers)
- **Auto-scaling**: Workers added based on task queue length
- **Load balancing**: Tasks distributed based on worker capabilities

### LLM Provider Selection
Strategies available in `config.yaml`:
- `performance`: Fastest response time
- `cost`: Lowest cost per token
- `availability`: Most reliable uptime
- `round-robin`: Rotate through providers
- `fallback`: Automatic failover

### Caching Strategy
- **Redis**: Session state, task status, user preferences
- **Response cache**: LLM responses with TTL
- **File cache**: Asset and template caching
- **Database pool**: Connection pooling with pgx

## Extension Points

### Custom LLM Providers
Implement `Provider` interface:
```go
type Provider interface {
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan *GenerateChunk, error)
    GetCapabilities() *Capabilities
    GetModels() []Model
    ValidateConfig(config map[string]interface{}) error
}
```

### Custom Notification Channels
Implement in `internal/notification/`:
```go
type Channel interface {
    Name() string
    Send(ctx context.Context, msg *Message) error
    Validate(config map[string]interface{}) error
}
```

### Custom Workflow Actions
Register actions in `internal/workflow/actions.go`:
```go
type Action interface {
    Name() string
    Execute(ctx context.Context, params map[string]interface{}) (*ActionResult, error)
    Validate(params map[string]interface{}) error
}
```

### Custom Tools
Extend the tool registry in `internal/tools/`:
```go
type Tool interface {
    Name() string
    Description() string
    Category() Category
    Schema() *Schema
    Validate(params map[string]interface{}) error
    Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
}
```

## Troubleshooting Guide

### Common Build Issues
1. **Logo assets not found**: Run `make logo-assets` before building
2. **Go version mismatch**: Requires Go 1.24.0 with toolchain go1.24.9
3. **Dependency errors**: Run `go mod tidy` to clean up dependencies
4. **Cross-platform build failures**: Check CGO_ENABLED setting for target platform

### Runtime Issues
1. **Database connection refused**: Check if PostgreSQL is running and accessible
2. **Redis connection timeouts**: Verify Redis is running and password is correct
3. **Worker SSH failures**: Check SSH key authentication and network connectivity
4. **LLM provider timeouts**: Increase timeout in config or check provider status

### Performance Issues
1. **Slow LLM responses**: Consider switching providers or using local models
2. **High memory usage**: Monitor task queues and implement cleanup
3. **Database slowness**: Check indexes and connection pool settings
4. **Worker overload**: Add more workers or implement task priorities

## Monitoring & Observability

### Health Endpoints
- `/health`: Basic application health
- `/metrics`: Prometheus metrics
- `/workers`: Worker pool status
- `/tasks`: Task queue status

### Key Metrics
- Task execution time and success rate
- Worker health and performance
- LLM provider response times
- Database connection pool status
- Memory and CPU usage

### Logging Levels
- `debug`: Detailed execution information
- `info`: General operational information
- `warn`: Warning messages that don't stop operation
- `error`: Error messages that may affect functionality
- `fatal`: Critical errors causing application exit

## Conclusion

HelixCode is a sophisticated distributed AI development platform with comprehensive features for building, testing, and deploying AI-powered applications. The modular architecture allows for easy extension and customization, while the robust testing framework ensures reliability at scale.

Key strengths:
- **Multi-platform support** with native clients for all major platforms
- **Distributed architecture** with automatic worker management
- **Flexible LLM integration** with both free and premium providers
- **Comprehensive tooling** for AI-powered development workflows
- **Production-ready** with Docker deployment and monitoring

When working with this codebase, always test thoroughly, follow the established patterns, and leverage the comprehensive testing and validation frameworks included.
---

## Universal Mandatory Constraints

> Cascaded from the HelixAgent root `CLAUDE.md` via `/tmp/UNIVERSAL_MANDATORY_RULES.md`.
> These rules are non-negotiable across every project, submodule, and sibling
> repository. Project-specific addenda are welcome but cannot weaken or
> override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either. All builds and tests run manually or via
   Makefile/script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates. Including for public repos. SSH keys are configured on every
   service.
3. **NO manual container commands.** Container orchestration is owned by
   the project's binary/orchestrator (e.g. `make build` → `./bin/<app>`).
   Direct `docker`/`podman start|stop|rm` and `docker-compose up|down`
   are prohibited as workflows. The orchestrator reads its configured
   `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration,
   E2E, automation, security/penetration, and benchmark tests. No false
   positives. Mocks/stubs ONLY in unit tests; all other test types use
   real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts
   (`./challenges/scripts/`) validating real-life use cases. No false
   success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API
   calls, real databases, live services. No simulated success. Fallback
   chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health
   endpoints. Circuit breakers for all external dependencies.
   Prometheus / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and
   relevant docs alongside code changes. Pass language-appropriate
   format/lint/security gates. Conventional Commits:
   `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation
   suite (`make ci-validate-all`-equivalent) plus all challenges
   (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes,
   placeholder classes, TODO implementations are STRICTLY FORBIDDEN in
   production code. All production code is fully functional with real
   integrations. Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all
   angles: runtime testing (actual HTTP requests / real CLI
   invocations), compile verification, code structure checks,
   dependency existence checks, backward compatibility, and no false
   positives in tests or challenges. Grep-only validation is NEVER
   sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and
   challenge execution MUST be strictly limited to 30-40% of host
   system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`,
   `-p 1` for `go test`. Container limits required. The host runs
   mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in
    `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with
    root cause analysis, affected files, fix description, and a link to
    the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/
    placeholders MAY be used ONLY in unit tests (files ending
    `_test.go` run under `go test -short`, equivalent for other
    languages). ALL other test types — integration, E2E, functional,
    security, stress, chaos, challenge, benchmark, runtime
    verification — MUST execute against the REAL running system with
    REAL containers, REAL databases, REAL services, and REAL HTTP
    calls. Non-unit tests that cannot connect to real services MUST
    skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported
    error, defect, or unexpected behavior MUST be reproduced by a
    Challenge script BEFORE any fix is attempted. Sequence:
    (1) Write the Challenge first. (2) Run it; confirm fail (it
    reproduces the bug). (3) Then write the fix. (4) Re-run; confirm
    pass. (5) Commit Challenge + fix together. The Challenge becomes
    the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any
    struct field that is a mutable collection (map, slice) accessed
    concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe` (or the project's equivalent
    primitives). Bare `sync.Mutex + map/slice` combinations are
    prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done"
requires pasted terminal output from a real run, produced in the same
session as the change.

- **No self-certification.** Words like *verified, tested, working,
  complete, fixed, passing* are forbidden in commits/PRs/replies unless
  accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable
  acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip`
  without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo`
  block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->


<!-- BEGIN const035-strengthening-2026-04-29 -->

## CONST-035 — End-User Usability Mandate (2026-04-29 strengthening)

A test or Challenge that PASSES is a CLAIM that the tested behavior
**works for the end user of the product**. The HelixAgent project
has repeatedly hit the failure mode where every test ran green AND
every Challenge reported PASS, yet most product features did not
actually work — buggy challenge wrappers masked failed assertions,
scripts checked file existence without executing the file,
"reachability" tests tolerated timeouts, contracts were honest in
advertising but broken in dispatch. **This MUST NOT recur.**

Every PASS result MUST guarantee:

a. **Quality** — the feature behaves correctly under inputs an end
   user will send, including malformed input, edge cases, and
   concurrency that real workloads produce.
b. **Completion** — the feature is wired end-to-end from public
   API surface down to backing infrastructure, with no stub /
   placeholder / "wired lazily later" gaps that silently 503.
c. **Full usability** — a CLI agent / SDK consumer / direct curl
   client following the documented model IDs, request shapes, and
   endpoints SUCCEEDS without having to know which of N internal
   aliases the dispatcher actually accepts.

A passing test that doesn't certify all three is a **bluff** and
MUST be tightened, or marked `t.Skip("...SKIP-OK: #<ticket>")`
so absence of coverage is loud rather than silent.

### Bluff taxonomy (each pattern observed in HelixAgent and now forbidden)

- **Wrapper bluff** — assertions PASS but the wrapper's exit-code
  logic is buggy, marking the run FAILED (or the inverse: assertions
  FAIL but the wrapper swallows them). Every aggregating wrapper MUST
  use a robust counter (`! grep -qs "|FAILED|" "$LOG"` style) —
  never inline arithmetic on a command that prints AND exits
  non-zero.
- **Contract bluff** — the system advertises a capability but
  rejects it in dispatch. Every advertised capability MUST be
  exercised by a test or Challenge that actually invokes it.
- **Structural bluff** — `check_file_exists "foo_test.go"` passes
  if the file is present but doesn't run the test or assert anything
  about its content. File-existence checks MUST be paired with at
  least one functional assertion.
- **Comment bluff** — a code comment promises a behavior the code
  doesn't actually have. Documentation written before / about code
  MUST be re-verified against the code on every change touching the
  documented function.
- **Skip bluff** — `t.Skip("not running yet")` without a
  `SKIP-OK: #<ticket>` marker silently passes. Every skip needs the
  marker; CI fails on bare skips.

The taxonomy is illustrative, not exhaustive. Every Challenge or
test added going forward MUST pass an honest self-review against
this taxonomy before being committed.

<!-- END const035-strengthening-2026-04-29 -->
