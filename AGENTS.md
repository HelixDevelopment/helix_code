# HelixCode Agent Guidelines

## Project Overview

HelixCode is a distributed AI development platform built in Go that enables intelligent task division, work preservation, and cross-platform development workflows.

**Key Features:**
- **Distributed Computing**: SSH-based worker pools with auto-installation and health monitoring
- **Multi-Provider LLM Integration**: Support for local providers (Llama.cpp, Ollama, vLLM) and cloud APIs (OpenAI, Anthropic, Gemini, xAI, Groq, etc.)
- **Development Workflows**: Automated planning, building, testing, refactoring, debugging, and deployment
- **Task Management**: Intelligent task division with dependency tracking, checkpointing, and rollback
- **MCP Protocol**: Full Model Context Protocol implementation
- **Multi-Client Architecture**: REST API, CLI, Terminal UI, Desktop, WebSocket, and mobile framework support
- **Memory Systems**: Integration with Mem0, Zep, Memonto, and BaseAI for long-term memory

## Essential Build Commands

**IMPORTANT**: All commands must be run from the `HelixCode/` subdirectory (not repository root).

### Core Commands
- **Build**: `make build` (generates logo assets and builds to bin/helixcode)
- **Test all**: `make test` or `go test -v ./...`
- **Test single**: `go test -v -run TestName ./path/to/package`
- **Test comprehensive**: `./run_tests.sh` (full test suite with multiple test types)
- **Lint**: `make lint` or `golangci-lint run ./...`
- **Format**: `make fmt` or `go fmt ./...`
- **Clean**: `make clean` (removes bin/, dist/, coverage.out)

### Development Workflow
- **Dev server**: `make dev` (builds and runs with config/dev/config.yaml)
- **Logo assets**: `make logo-assets` (required before first build)
- **Setup deps**: `make setup-deps` or `go mod tidy`

### Specialized Builds
- **Production**: `make prod` (cross-platform builds for Linux, macOS, Windows)
- **Mobile**: `make mobile` (builds iOS framework and Android AAR)
- **Aurora OS**: `make aurora-os` (Russian platform client)
- **Harmony OS**: `make harmony-os` (Chinese platform client)
- **Full release**: `make release` (clean + assets + docs + build + test)

### Testing Variations
- **Unit tests**: `./run_tests.sh --unit`
- **Integration**: `./run_tests.sh --integration`
- **E2E**: `./run_tests.sh --e2e`
- **Coverage**: `./run_tests.sh --coverage` (generates HTML report)
- **Benchmarks**: `./run_tests.sh --benchmarks`
- **Security**: `./run_tests.sh --security`
- **Challenge tests**: `cd tests/e2e/challenges && go run cmd/runner/main.go`

## Architecture & Code Organization

### Core Structure
```
HelixCode/
├── cmd/                    # Application entry points
│   ├── server/            # Main HTTP server
│   └── cli/               # CLI client
├── internal/              # Internal packages (not importable externally)
│   ├── auth/              # JWT authentication with session management
│   ├── worker/            # SSH-based distributed worker pool
│   ├── task/              # Task management with checkpointing
│   ├── llm/               # Multi-provider LLM integration
│   ├── mcp/               # Model Context Protocol implementation
│   ├── workflow/          # Workflow execution engine
│   ├── project/           # Project lifecycle and session management
│   ├── server/            # HTTP server, routing, and API handlers
│   ├── database/          # PostgreSQL layer (optional)
│   ├── redis/             # Redis client (optional)
│   ├── config/            # Configuration management with Viper
│   ├── tools/             # Comprehensive tool ecosystem
│   ├── editor/            # Multi-format code editing system
│   ├── memory/            # Long-term memory integration
│   └── [... other services]
├── applications/          # Platform-specific apps
│   ├── desktop/           # Desktop GUI (Fyne-based)
│   ├── terminal-ui/       # Terminal UI (tview)
│   ├── ios/               # iOS application
│   ├── android/           # Android application
│   ├── aurora-os/         # Aurora OS client
│   └── harmony-os/        # Harmony OS client
├── config/               # Configuration files
├── tests/                # Test suites and frameworks
├── external/             # Git submodules (memory providers)
└── go.mod                # Go module definition
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

### Dependencies
Core dependencies: `github.com/gin-gonic/gin`, `github.com/jackc/pgx/v5`, `github.com/golang-jwt/jwt/v4`, `github.com/spf13/viper`, `github.com/stretchr/testify`

## Configuration Management

### Primary Configuration
Main config at `config/config.yaml` with environment variable overrides:

```yaml
server:
  address: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  # Password via HELIX_DATABASE_PASSWORD

auth:
  # JWT secret via HELIX_AUTH_JWT_SECRET
  
workers:
  health_check_interval: 30
  
llm:
  default_provider: "local"
  max_tokens: 4096
  temperature: 0.7
```

### Environment Variables
**Required:**
- `HELIX_DATABASE_PASSWORD`: PostgreSQL password
- `HELIX_AUTH_JWT_SECRET`: JWT signing secret
- `HELIX_REDIS_PASSWORD`: Redis password (if enabled)

**Optional for LLM providers:**
- `ANTHROPIC_API_KEY`: Claude
- `GEMINI_API_KEY`: Google Gemini  
- `OPENAI_API_KEY`: OpenAI
- `XAI_API_KEY`: xAI/Grok

### Database Setup
```bash
createdb helixcode
createuser helixcode
export HELIX_DATABASE_PASSWORD=your_password
# Schema auto-created by application
```

**Database is Optional**: Can be disabled for testing by leaving `database.host` empty or setting `database.enabled: false`.

## Testing Approach

### Test Categories
- **Unit tests**: Alongside source files (`*_test.go`)
- **Integration tests**: In `tests/` directory
- **E2E tests**: Full workflow testing
- **Challenge tests**: Comprehensive project generation validation

### Key Test Files
- `run_tests.sh`: Comprehensive test runner with multiple test types
- `test_runner.go`: Test execution engine
- `tests/e2e/challenges/`: Challenge testing framework

### Running Tests
```bash
# Basic
make test

# Comprehensive with coverage
./run_tests.sh --all

# Specific types
./run_tests.sh --unit --integration --coverage

# Challenge tests (validates complete project generation)
cd tests/e2e/challenges
go run cmd/runner/main.go -list
go run cmd/runner/main.go -challenge notes-project-001
```

## Key Subsystems

### Tools Package (`internal/tools/`)
Comprehensive tool ecosystem for AI agents:
- **Filesystem**: fs_read, fs_write, fs_edit, glob, grep
- **Shell**: shell, shell_background, shell_output, shell_kill
- **Web**: web_fetch, web_search
- **Browser**: browser_launch, browser_navigate, browser_screenshot
- **MultiEdit**: Transactional multi-file editing
- All tools include security boundaries and validation

### Editor Package (`internal/editor/`)
Multi-format editing system optimized for different LLM models:
- **Diff Format**: Unix unified diff (best for GPT-4, Gemini Pro)
- **Whole File**: Complete file replacement (best for Claude, O1 models)
- **Search/Replace**: Pattern-based with regex (best for Claude, Mistral)
- **Line-Based**: Specific line range edits
- Automatic format selection based on model capabilities

### Challenge Testing Framework
Located at `tests/e2e/challenges/`:
- Validates ability to generate complete working projects from prompts
- Tests across multiple interfaces (CLI, TUI, REST, WebSocket, Desktop)
- Supports distributed worker testing (2, 5, 10 workers)
- Comprehensive validation (no placeholders, compiles, tests pass)
- Detailed result organization and logging

## Important Gotchas

### Critical Requirements
- **Always work from HelixCode/ subdirectory** - not repository root
- **Generate logo assets before first build** with `make logo-assets`
- **Database/Redis are optional** - can be disabled for testing
- **Environment variables override config file**

### SSH Worker Auto-Install
- When adding workers via SSH, the system automatically installs Helix CLI
- Requires SSH key-based authentication
- Workers are health-checked every 30s by default

### Task Checkpointing
- Long-running tasks automatically checkpoint at intervals (default 300s)
- Enables work preservation and recovery from failures
- Checkpoints stored in PostgreSQL for persistence

### Provider Fallback
- LLM requests can fall back to alternative providers if primary fails
- Configurable provider priority and retry logic
- Automatic rate limiting and quota management

### Session Context
- Development sessions maintain context across interactions
- Context stored in Redis for fast access
- Automatic cleanup of expired sessions

## Free AI Providers (No API Keys Required)

The system includes multiple free providers out-of-the-box:
- **XAI (Grok)**: grok-3-fast-beta, grok-3-mini-fast-beta
- **OpenRouter**: deepseek-r1-free, meta-llama/llama-3.2-3b-instruct:free
- **GitHub Copilot**: gpt-4o, claude-3.5-sonnet (free with GitHub subscription)
- **Qwen**: 2,000 requests/day free tier with OAuth2

## Memory Integration

External memory providers are included as git submodules in `external/memory/`:
- **Mem0**: Advanced memory management with embeddings
- **Zep**: Long-term conversational memory
- **Memonto**: Knowledge graph-based memory
- **BaseAI**: Comprehensive memory platform with tools

## Docker Deployment

### Quick Start with Docker Compose

```bash
# Clone repository
git clone https://github.com/your-org/helixcode.git
cd helixcode/HelixCode

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

## Common Development Workflows

### Adding New LLM Provider

1. Implement the `Provider` interface in `internal/llm/`:
```go
type Provider interface {
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan *GenerateChunk, error)
    GetCapabilities() *Capabilities
    GetModels() []Model
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