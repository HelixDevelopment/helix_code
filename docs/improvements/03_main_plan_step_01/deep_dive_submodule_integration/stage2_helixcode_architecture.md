# HelixCode Internal Architecture - Comprehensive Technical Analysis

## Executive Summary

HelixCode is a **Go-based enterprise AI development platform** built around a sophisticated multi-agent system with 29+ LLM provider integrations, distributed computing capabilities, and a zero-tolerance security posture. The architecture follows a **layered, modular design** with clear separation between CLI entry points, core internal packages, application frontends, and API services.

**Key Architectural Characteristics:**
- **Language**: Go 1.26 with ~60+ internal packages
- **Module**: `dev.helix.code`
- **CLI Framework**: Cobra + Viper for configuration
- **HTTP Framework**: Gin with JWT auth, CORS, security middleware
- **Database**: PostgreSQL (pgx) with Redis caching
- **External Dependencies**: Fyne (desktop), tview (terminal), chromedp (browser), tree-sitter (parsing)

---

## 1. CMD/ Directory - CLI Entry Points & Commands

### 1.1 Architecture Pattern
**Pattern**: Cobra Command Pattern with subcommand hierarchy

### 1.2 Key Files & Structure
```
cmd/
├── cli/main.go          - CLI client entry point
├── root.go              - Root cobra command definition
├── server/main.go       - HTTP server entry point
├── server/main_test.go  - Server tests
├── security-fix/        - Standalone security fix utility
├── security-fix-standalone/
├── security-test/       - Security testing utility
```

### 1.3 Key Types & Interfaces

**Root Command** (`cmd/root.go`):
```go
var rootCmd = &cobra.Command{
    Use:   "helix",
    Short: "HelixCode - Enterprise AI Development Platform",
    Long:  "HelixCode is the world's most advanced enterprise AI development platform...",
    Version: "1.0.0",
}
```

- **Global Flags**: `--config`, `--debug`, `--log-level`
- **Config Integration**: Viper with automatic env binding (`HELIX_*` prefix)
- **Config Search Paths**: `$HOME/.helix.yaml`, `./config/`, `/etc/helixcode/`

### 1.4 CLI Agent Features Implemented
| Feature | Status | Implementation |
|---------|--------|----------------|
| Root command with version | ✅ | Cobra rootCmd |
| Global config flag | ✅ | `--config` with viper |
| Debug/logging flags | ✅ | `--debug`, `--log-level` |
| Server mode | ✅ | `cmd/server/main.go` |
| Security utilities | ✅ | `cmd/security_fix/` |

### 1.5 What's Missing vs State-of-the-Art
- **Missing**: Interactive chat REPL loop (only basic command structure)
- **Missing**: Auto-completion for file paths in CLI
- **Missing**: Rich TUI dashboard in CLI mode (only separate terminal-ui app)
- **Missing**: Inline editing commands (`/edit`, `/add`, `/drop`)

---

## 2. INTERNAL/ Directory - Core Packages

### 2.1 AGENT/ - Multi-Agent Orchestration System

**Architecture Pattern**: **Actor Model + Registry Pattern + Circuit Breaker**

**Key Types**:
```go
// Core Agent Interface
interface Agent {
    ID() string; Type() AgentType; Name() string
    Capabilities() []Capability
    CanHandle(task *task.Task) bool
    Execute(ctx context.Context, task *task.Task) (*task.Result, error)
    Collaborate(ctx context.Context, agents []Agent, task *task.Task) (*CollaborationResult, error)
    Initialize(ctx context.Context, config *AgentConfig) error
    Shutdown(ctx context.Context) error
    Status() AgentStatus; Health() *HealthCheck
}

// Agent Types
AgentTypePlanning | AgentTypeCoding | AgentTypeTesting | AgentTypeDebugging
AgentTypeReview | AgentTypeRefactoring | AgentTypeDocumentation | AgentTypeCoordinator

// Collaboration System
CollaborationResult { Success, Results, Consensus, Conflicts, Messages }
CollaborationMessage { ID, From, To, Type, Content, Timestamp }
Conflict { ID, Agents, Issue, Proposals, Resolution }
```

**BaseAgent Implementation** (`internal/agent/base_agent.go`):
- **Task Queue**: Buffered channel (100 tasks)
- **Result Channel**: Buffered channel (100 results)
- **Concurrency**: Configurable maxConcurrency (default: 1)
- **Timeout**: Configurable (default: 30s)
- **Retry**: Configurable retryCount (default: 3)
- **Statistics**: tasksProcessed, tasksSucceeded, tasksFailed, totalDuration
- **LLM Integration**: Optional llm.Provider + tools.ToolRegistry
- **Collaboration Logic**: Built-in inter-agent collaboration with type-based routing:
  - CodingAgent → ReviewAgent (code review)
  - CodingAgent → TestingAgent (test generation)
  - DebuggingAgent → TestingAgent (verification)
  - ReviewAgent → RefactoringAgent (issue addressing)
  - PlanningAgent → PlanningAgent (consensus)

**Coordinator** (`internal/agent/coordinator.go`):
- **Registry**: AgentRegistry with mutex-protected map
- **Task Assignment**: findSuitableAgent() - first idle agent with matching capabilities
- **Resilience**: CircuitBreakerManager + RetryPolicy
- **Workflow Execution**: WorkflowExecutor integration
- **Health Monitoring**: Per-agent circuit breaker state tracking

**Task System** (`internal/agent/task/`):
```go
type Task struct {
    ID, Title, Description string
    Type TaskType // planning, code_generation, code_edit, testing, debugging, review, refactoring, documentation, analysis
    Priority Priority // 1-4
    Input map[string]interface{}
    RequiredCapabilities []string
    Dependencies []string
}
```

**Agent Types Directory** (`internal/agent/types/`):
- `coding_agent.go` - Code generation and editing
- `debugging_agent.go` - Error analysis and fixing
- `planning_agent.go` - Technical planning and architecture
- `review_agent.go` - Code review and quality assessment
- `testing_agent.go` - Test generation and execution

### 2.2 LLM/ - LLM Provider Integration (29+ Providers)

**Architecture Pattern**: **Factory Pattern + Strategy Pattern + Registry Pattern**

**Key Types**:
```go
// Provider Interface
interface Provider {
    GetType() ProviderType
    GetName() string
    GetModels() []ModelInfo
    Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error)
    GetHealth(ctx context.Context) (*ProviderHealth, error)
}

// LLMRequest/Response
LLMRequest { Model, Messages, MaxTokens, Temperature, Tools }
LLMResponse { Content, Usage, Model, FinishReason }

// Model Selection Criteria
ModelSelectionCriteria {
    TaskType, RequiredCapabilities, MaxTokens, Budget, LatencyRequirement, QualityPreference
}
```

**Supported Providers** (from `internal/llm/factory.go`):

| Cloud Providers | Local Providers |
|----------------|-----------------|
| OpenAI | Ollama |
| Anthropic | LlamaCpp |
| Gemini | VLLM |
| Azure | LocalAI |
| AWS Bedrock | FastChat |
| Vertex AI | TextGen |
| Groq | LMStudio |
| XAI (Grok) | Jan |
| OpenRouter | GPT4All |
| Copilot | TabbyAPI |
| Qwen | MLX |
| | MistralRS |
| | KoboldAI |

**Model Manager** (`internal/llm/model_manager.go`):
- **Scoring Algorithm**: Multi-criteria model scoring (capability match, token fit, budget, latency, quality)
- **Hardware Integration**: Hardware detector for local model selection
- **Verifier Integration**: Score-augmented selection via verifier.Adapter
- **Health Checks**: Per-provider health monitoring

**AutoLLM Manager** (`internal/llm/auto_llm_manager.go`):
- **Zero-Touch Configuration**: Auto-discover, auto-install, auto-configure, auto-start, auto-monitor, auto-update
- **Health Monitoring**: Auto-recovery with configurable retries
- **Performance Optimization**: Auto-optimize, load balance, cache responses, predict scaling
- **Security**: Auto-sandbox, min privileges, network isolation, resource limits

**Load Balancer** (`internal/llm/load_balancer.go`):
- **Strategies**: round_robin, least_connections, response_time, weighted, performance_based
- **Statistics Collection**: Per-provider request counts, response times, error rates
- **Optimal Provider Detection**: Continuous performance-based selection

**Compression System** (`internal/llm/compression/`):
- Context compression with configurable strategies
- Retention policies for important context
- Token budget management

**Vision Support** (`internal/llm/vision/`):
- Vision model detection and switching
- Config-based vision provider management
- Verifier synchronization

**Cross-Provider Registry** (`internal/llm/cross_provider_registry.go`):
- Unified model naming across providers
- Provider-agnostic model resolution
- Fallback model chains

### 2.3 TOOLS/ - Tool Framework

**Architecture Pattern**: **Plugin Registry + Unified Tool Interface**

**Key Types**:
```go
interface Tool {
    Name() string
    Description() string
    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
    Schema() ToolSchema
    Category() ToolCategory
    Validate(params map[string]interface{}) error
}

type ToolRegistry struct {
    tools   map[string]Tool
    aliases map[string]string
    // Component instances
    filesystem   *filesystem.FileSystemTools
    shell        *shell.ShellExecutor
    web          *web.WebTools
    browser      *browser.BrowserTools
    mapper       mapping.Mapper
    multiEdit    *multiedit.MultiFileEditor
    confirmation *confirmation.ConfirmationCoordinator
}
```

**Tool Categories**:
| Category | Tools | Description |
|----------|-------|-------------|
| Filesystem | FSRead, FSWrite, FSEdit, Glob, Grep | File operations |
| Shell | Shell, ShellBackground, ShellOutput, ShellKill | Command execution |
| Web | WebFetch, WebSearch | Web scraping and search |
| Browser | BrowserLaunch, BrowserNavigate, BrowserScreenshot, BrowserClose | ChromeDP-based |
| Mapping | CodebaseMap, FileDefinitions | Code analysis via tree-sitter |
| MultiEdit | MultiEditBegin, MultiEditAdd, MultiEditPreview, MultiEditCommit | Atomic multi-file edits |
| Interactive | AskUser, TaskTracker | User interaction |
| Notebook | NotebookRead, NotebookEdit | Jupyter-like notebooks |

**Key Innovations**:
- **MultiEdit**: Atomic multi-file editing with transaction support
- **Confirmation System**: Danger-level policies with audit trails (`internal/tools/confirmation/`)
- **Browser Automation**: ChromeDP-based with screenshot capture
- **Code Mapping**: Tree-sitter based code analysis for accurate edits

### 2.4 COMMANDS/ - Slash Command System

**Architecture Pattern**: **Command Pattern + Registry**

**Key Types**:
```go
interface Command {
    Name() string
    Aliases() []string
    Description() string
    Usage() string
    Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error)
}

type CommandContext struct {
    UserID, SessionID, ProjectID string
    Args []string; Flags map[string]string
    RawInput string; ChatHistory []ChatMessage
    WorkingDir string; Metadata map[string]interface{}
}

type CommandResult struct {
    Success bool; Message string; Data interface{}
    Actions []Action; ShouldReply bool
}
```

**Built-in Commands** (`internal/commands/builtin/`):
- `condense.go` - Context condensation
- `deepplanning.go` - Deep planning mode
- `newrule.go` - Rule creation
- `newtask.go` - Task creation
- `reportbug.go` - Bug reporting
- `workflows.go` - Workflow management

### 2.5 CONTEXT/ - Context Management

**Architecture Pattern**: **Hierarchical Context Store + TTL Expiration**

**Key Types**:
```go
type ContextManager struct {
    items    map[string]*ContextItem
    sessions map[string]*SessionContext
    projects map[string]*ProjectContext
    global   *GlobalContext
}

type ContextItem struct {
    ID, Type, Key string
    Value interface{}
    Metadata map[string]interface{}
    Timestamp time.Time
    TTL *time.Duration
    Source string
    Priority int
}
```

**Features**:
- **Hierarchical Scopes**: File → Session → Project → Global
- **TTL Expiration**: Automatic cleanup of expired items (5-minute ticker)
- **Search**: Key pattern matching within context types
- **Version Control**: Context item versioning for conflict resolution

**Context Builder** (`internal/context/builder.go`):
- Template-based context assembly
- Source aggregation (files, git, URLs, terminal output)
- Mention system for file/folder/git/problem/URL/terminal references

### 2.6 MEMORY/ - Memory & Conversation System

**Architecture Pattern**: **Conversation Manager + Callback Events**

**Key Types**:
```go
type Manager struct {
    conversations    map[string]*Conversation
    activeConv       *Conversation
    maxMessages      int    // 1000 default
    maxTokens        int    // 100000 default (~25K words)
    maxConversations int    // 100 default
    onCreate  []ConversationCallback
    onMessage []MessageCallback
    onClear   []ConversationCallback
    onDelete  []ConversationCallback
}
```

**Features**:
- **Conversation Lifecycle**: Create, get, set active, delete, clear, import, export
- **Message Management**: Add with automatic limit enforcement
- **Token Management**: Automatic truncation at 75% of maxTokens
- **Search**: Full-text search across conversations and messages
- **Version Control**: Optimistic concurrency with conflict resolution (overwrite/merge strategies)
- **Statistics**: Per-conversation and aggregate metrics
- **Snapshot Support**: Export/import conversation snapshots

**Memory Providers** (`internal/memory/providers/`):
| Provider | Type |
|----------|------|
| Mem0 | Memory layer |
| Zep | Long-term memory |
| Memonto | Memory management |
| BaseAI | AI-native memory |
| ChromaDB | Vector store |
| FAISS | Vector search |
| Pinecone | Cloud vector DB |
| Qdrant | Vector DB |
| Weaviate | Vector search engine |
| Anima | Character memory |
| CharacterAI | Personality memory |

### 2.7 DATABASE/ - Data Persistence

**Architecture Pattern**: **Interface + PostgreSQL Implementation + Mock Layer**

**Key Types**:
```go
interface DatabaseInterface {
    // Connection management
    Connect(ctx context.Context) error
    Close() error
    HealthCheck() error
    // CRUD operations
    Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
    Execute(ctx context.Context, sql string, args ...interface{}) (Result, error)
    // Transactions
    BeginTx(ctx context.Context) (Transaction, error)
}
```

**Mock System**:
- `mock_database.go` - Full database mock
- `mock_row.go`, `mock_rows.go` - Row mocking
- `mock_helpers.go` - Test utilities

### 2.8 AUTH/ - Authentication

**Architecture Pattern**: **JWT-based Auth Service + Database-backed**

**Key Types**:
```go
type AuthService struct {
    config AuthConfig
    db     *AuthDB
}

type AuthConfig struct {
    JWTSecret     string
    TokenExpiry   time.Duration
    SessionExpiry time.Duration
    BcryptCost    int
}
```

**Features**:
- JWT token generation and verification
- Session management with expiry
- Bcrypt password hashing
- Database-backed user storage

### 2.9 SERVER/ - HTTP API Server

**Architecture Pattern**: **Gin Router + Dependency Injection + Middleware Chain**

**Key Types**:
```go
type Server struct {
    config         *config.Config
    db             *database.Database
    redis          *redis.Client
    auth           *auth.AuthService
    mcp            *mcp.MCPServer
    notification   *notification.NotificationEngine
    taskManager    *task.DatabaseManager
    workerManager  *worker.DatabaseManager
    projectManager *project.DatabaseManager
    sessionManager *session.Manager
    verifierResult *verifier.BootstrapResult
    qaEngine       *helixqa.Engine
    router         *gin.Engine
}
```

**Route Structure**:
```
/health                          - Health check (public)
/api/v1/
  /auth/register, /login, /logout, /refresh
  /users/me
  /workers/ (CRUD + heartbeat + metrics)
  /tasks/ (CRUD + assign + start + complete + fail + checkpoint + retry)
  /projects/ (CRUD + sessions + workflows)
  /sessions/ (CRUD)
  /llm/providers, /llm/models
  /memory/systems, /memory/stats
  /qa/session (QA testing)
  /screenshot/:platform
  /system/stats, /system/status
  /server/info, /metrics
/ws                              - WebSocket for MCP
/static                          - Static web frontend
```

**Middleware Stack**:
1. Gin Logger
2. Gin Recovery
3. CORS Middleware
4. Security Middleware (X-Content-Type-Options, X-Frame-Options, HSTS)
5. JWT Auth Middleware (Bearer token verification)

### 2.10 WORKER/ - Distributed Computing

**Architecture Pattern**: **SSH-based Worker Pool + Consensus**

**Key Types**:
```go
type Manager struct {
    workers      map[string]*Worker
    sshPool      *SSHPool
    consensus    *ConsensusEngine
    isolation    *IsolationManager
    memoryRepo   *MemoryRepository
}

type Worker struct {
    ID, Hostname string
    Status WorkerStatus // active, inactive, maintenance, failed, offline
    HealthStatus HealthStatus
    Resources WorkerResources
    SSHConfig SSHConfig
    Capabilities []string
    MaxConcurrentTasks int
}
```

**Features**:
- **SSH Pool**: Connection pooling to remote workers
- **Health Monitoring**: Heartbeat-based with metrics
- **Consensus**: Distributed consensus engine
- **Isolation**: Task isolation between workers
- **Database Manager**: Persistent worker state in PostgreSQL

### 2.11 WORKFLOW/ - Development Workflows

**Architecture Pattern**: **State Machine + Step Executor**

**Key Types**:
```go
type Workflow struct {
    ID, Name, Description, Mode string
    Steps []Step
    Status WorkflowStatus
}

type Step struct {
    ID, Name, Description string
    Type StepType   // analysis, generation, execution, validation
    Action StepAction // analyze_code, generate_code, execute_command, run_tests, lint_code, build_project
    Dependencies []string
    Status StepStatus
}
```

**Workflow Modes**:
- `autonomy/` - Autonomous execution with guardrails, escalation, permission system
- `planmode/` - Planning mode with executor, planner, mode controller
- `snapshots/` - Workflow snapshots with comparison, metadata, restore

### 2.12 EDITOR/ - Editor Integration

**Architecture Pattern**: **Format Registry + Diff Engine**

**Supported Edit Formats** (`internal/editor/formats/`):
| Format | File |
|--------|------|
| Unified Diff | `udiff_format.go` |
| Search/Replace | `search_replace_format.go` |
| Whole File | `whole_format.go` |
| Line Numbers | `line_number_format.go` |
| Architect | `architect_format.go` |
| Ask | `ask_format.go` |
| Diff | `diff_format.go` |

**Editor Implementations**:
- `diff_editor.go` - Diff-based editing
- `line_editor.go` - Line-number based editing
- `search_replace_editor.go` - Search/replace editing
- `whole_editor.go` - Whole file replacement

### 2.13 EVENT/ - Event System

**Architecture Pattern**: **Pub/Sub Bus + Instance Management**

**Key Types**:
```go
type EventBus struct {
    subscribers map[EventType][]EventHandler
    // ...
}
```

**Features**:
- Benchmark-tested event dispatch
- Instance-based event routing
- Typed event handlers

### 2.14 FIX/ - Auto-Fix Capabilities

**Architecture Pattern**: **Rule-based Fix Engine**

**Key Types**:
```go
type FixEngine struct {
    // Auto-fix capabilities for common issues
}
```

**Features**:
- Automated error detection and correction
- Test-driven fix validation

### 2.15 FOCUS/ - Focus Chain Management

**Architecture Pattern**: **Chain of Focus Contexts**

**Key Types**:
```go
type Chain struct {
    // Focus chain for maintaining context across operations
}
```

**Features**:
- Focus preservation across operations
- Context switching management

### 2.16 HARDWARE/ - Hardware Abstraction

**Architecture Pattern**: **Detector Pattern**

**Key Types**:
```go
type Detector struct {
    // Hardware capability detection
}
```

**Features**:
- GPU detection (count, model, memory)
- CPU detection (count, capabilities)
- Memory detection
- Used for local LLM model selection

### 2.17 HOOKS/ - Hook System

**Architecture Pattern**: **Hook Registry + Executor**

**Key Types**:
```go
type Hook struct {
    Name, Description string
    Event string
    Command string
    // ...
}

type Manager struct {
    hooks map[string]*Hook
    executor *Executor
}
```

**Features**:
- Pre/post event hooks
- Configurable hook execution
- Hook registry management

### 2.18 REPOMAP/ - Repository Mapping

**Architecture Pattern**: **Tree-sitter Based Code Analysis**

**Key Types**:
```go
type RepoMap struct {
    cache *Cache
    fileRanker *FileRanker
    tagExtractor *TagExtractor
    treeSitter *TreeSitter
}
```

**Features**:
- File ranking based on importance
- Tag extraction for symbols
- Tree-sitter parsing for language support
- Caching for performance

### 2.19 SESSION/ - Session Management

**Architecture Pattern**: **Manager + Session Store**

**Key Types**:
```go
type Manager struct {
    sessions map[string]*Session
}

type Session struct {
    ID, ProjectID, Name, Mode, Status string
    FocusChainID string
    Context map[string]interface{}
    // ...
}
```

### 2.20 TASK/ - Task Management

**Architecture Pattern**: **Manager + Queue + Checkpoints + Dependencies**

**Key Types**:
```go
type Manager struct {
    tasks map[string]*Task
    queue *Queue
    checkpoints map[string][]*Checkpoint
    dependencies *DependencyGraph
}

type Task struct {
    ID, Name, Description, Type, Status string
    Priority int; Criticality string
    AssignedWorker string
    Dependencies []string
    RetryCount, MaxRetries int
    // ...
}
```

**Features**:
- Task queue with priority
- Checkpoint system for resumable tasks
- Dependency graph for task ordering
- Database-backed persistence

### 2.21 NOTIFICATION/ - Notification System

**Architecture Pattern**: **Engine + Integrations + Queue + Rate Limiting**

**Integrations**:
- Discord
- Slack
- Telegram
- Email
- Webhooks

**Features**:
- Event-based notification triggers
- Rate limiting
- Retry logic with backoff
- Queue-based delivery
- Metrics collection

### 2.22 VERIFIER/ - LLM Verifier Subsystem

**Architecture Pattern**: **Adapter + Bootstrap + Poller**

**Key Types**:
```go
type Adapter struct {
    // Score-augmented model selection
}

type BootstrapResult struct {
    // Verifier initialization result
}
```

**Features**:
- Model score verification
- Health monitoring
- Cache management
- Embedded server mode
- Fallback model chains

### 2.23 MCP/ - Model Context Protocol

**Architecture Pattern**: **Server + WebSocket Handler**

**Key Types**:
```go
type MCPServer struct {
    // Model Context Protocol server
}
```

**Features**:
- WebSocket-based MCP communication
- Mock connection support for testing

---

## 3. APPLICATIONS/ - Multi-Platform UI

### 3.1 Architecture Pattern
**Pattern**: Shared core (`shared/mobile-core/`) with platform-specific frontends

### 3.2 Supported Platforms
| Platform | Framework | Entry Point |
|----------|-----------|-------------|
| Terminal UI | tview (Go TUI) | `applications/terminal_ui/main.go` |
| Desktop | Fyne (Go GUI) | `applications/desktop/main.go` |
| Android | Kotlin | `applications/android/app/src/main/java/.../MainActivity.kt` |
| iOS | Swift | `applications/ios/HelixCode/ViewController.swift` |
| Aurora OS | Go | `applications/aurora_os/main.go` |
| Harmony OS | Go | `applications/harmony_os/main.go` |

### 3.3 Terminal UI
- Uses `github.com/rivo/tview` for terminal UI components
- Theme support with configurable colors
- Component-based layout

### 3.4 Desktop
- Uses `fyne.io/fyne/v2` for cross-platform GUI
- Theme system with light/dark modes
- No-GUI build option (`main_nogui.go`)

### 3.5 Mobile
- **Android**: Kotlin with `MobileCore.kt` shared logic
- **iOS**: Swift with `MobileCore.swift` shared logic
- Shared Go core in `shared/mobile-core/mobile.go`

---

## 4. API/OPENAPI.YAML - API Specification

### 4.1 Architecture Pattern
**OpenAPI 3.0.3** with comprehensive REST API design

### 4.2 API Endpoints Summary
| Category | Endpoints | Auth |
|----------|-----------|------|
| Health | GET /health | Public |
| Auth | POST /auth/{register,login,logout,refresh} | Mixed |
| Users | GET/PUT/DELETE /users/me | JWT |
| Projects | CRUD /projects | Mixed |
| Workflows | POST /projects/{id}/workflows/{planning,building,testing,refactoring} | Public |
| Tasks | CRUD + assign/start/complete/fail/checkpoint/retry | JWT |
| Workers | CRUD + heartbeat + metrics | JWT |
| Sessions | CRUD /sessions | JWT |
| LLM | GET /llm/providers, /llm/models | Public |
| Memory | GET /memory/systems, /memory/stats | Public |
| QA | POST/GET/DELETE /qa/session* | JWT |
| Screenshot | GET /screenshot/:platform | JWT |
| System | GET /system/stats, /system/status | JWT |
| Server | GET /server/info, /metrics | Public |
| WebSocket | GET /ws | Public |

### 4.3 Key Schemas
- Comprehensive schema definitions for all entities
- Task lifecycle with 7 status states
- Worker resources (CPU, memory, disk, GPU)
- Session modes (planning, building, testing, refactoring, debugging, deployment)

---

## 5. CONFIG/ - Configuration System

### 5.1 Architecture Pattern
**Viper-based + Environment Variables + Validation + Migration**

### 5.2 Key Features
- **Sources**: File (YAML), Environment (`HELIX_*`), Defaults
- **Hot Reload**: Via fsnotify (implied by dependency)
- **Validation**: Strict validation with custom rules
- **Migration**: Version-based config migration (1.0.0 → 1.1.0 → 1.2.0)
- **Templates**: Environment-specific templates (dev, prod, test)
- **Secure Fields**: JWT secret, DB password via env vars only

### 5.3 Config Structure
```yaml
version: "1.0.0"
application: { name, version, environment, workspace, session }
server: { address, port, timeouts }
database: { host, port, user, password, dbname }
redis: { enabled, host, port }
auth: { jwt_secret, token_expiry, session_expiry, bcrypt_cost }
workers: { health_check_interval, max_concurrent_tasks }
tasks: { max_retries, checkpoint_interval }
llm: { default_provider, default_model, max_tokens, temperature }
logging: { level, format, output }
verifier: { enabled, mode, endpoint, scoring_weights }
qa: { enabled, banks_dir, platforms, coverage_target }
```

---

## 6. TEST/ - Testing Infrastructure

### 6.1 Architecture Pattern
**Multi-layer Testing with Automation**

### 6.2 Test Categories
| Category | Location | Description |
|----------|----------|-------------|
| Unit Tests | `*_test.go` (per package) | Package-level unit tests |
| Integration | `test/integration/` | Discord, Slack, Telegram integration |
| E2E | `test/e2e/` | Comprehensive end-to-end |
| Load | `test/load/` | Notification load testing |
| Automation | `test/automation/` | Provider automation (Anthropic, Gemini, OpenRouter, Qwen, XAI) |
| Workers | `test/workers/` | SSH key management for worker tests |

### 6.3 Standalone Tests
- `standalone_tests/cli_test.go` - CLI standalone tests
- `standalone_tests/test_suite.go` - Test suite runner

---

## 7. BENCHMARKS/ - Performance Benchmarking

### 7.1 Architecture Pattern
**Go Benchmark Suite**

### 7.2 Key Files
- `benchmarks/performance_bench_test.go` - Performance benchmarks
- `benchmark-reports/` - Generated benchmark reports with timestamps

---

## 8. Module Dependencies & Relationships

### 8.1 Dependency Graph (Key Flows)
```
main.go
  └── cmd/root.go (Cobra)
        ├── cmd/cli/main.go → internal/* (CLI mode)
        └── cmd/server/main.go → internal/server (Server mode)

internal/server/server.go
  ├── internal/auth (JWT)
  ├── internal/database (PostgreSQL)
  ├── internal/redis (Caching)
  ├── internal/llm (LLM providers)
  ├── internal/mcp (MCP protocol)
  ├── internal/notification (Notifications)
  ├── internal/task (Task management)
  ├── internal/worker (Distributed workers)
  ├── internal/project (Project management)
  ├── internal/session (Session tracking)
  ├── internal/helixqa (QA engine)
  └── internal/verifier (Model verification)

internal/agent/coordinator.go
  ├── internal/agent/base_agent.go
  ├── internal/agent/task (Task definitions)
  ├── internal/llm (LLM provider)
  └── internal/tools (Tool registry)

internal/tools/registry.go
  ├── internal/tools/filesystem
  ├── internal/tools/shell
  ├── internal/tools/web
  ├── internal/tools/browser (ChromeDP)
  ├── internal/tools/mapping (Tree-sitter)
  ├── internal/tools/multiedit
  └── internal/tools/confirmation

internal/llm/factory.go
  ├── 29+ provider implementations
  ├── internal/llm/model_manager.go
  ├── internal/llm/auto_llm_manager.go
  ├── internal/llm/load_balancer.go
  └── internal/hardware (Hardware detection)
```

### 8.2 External Module Replacements (go.mod)
```
digital.vasic.containers => ../Containers
digital.vasic.helixqa => ../HelixQA
digital.vasic.docprocessor => ../Dependencies/HelixDevelopment/DocProcessor
digital.vasic.llmorchestrator => ../Dependencies/HelixDevelopment/LLMOrchestrator
digital.vasic.visionengine => ../Dependencies/HelixDevelopment/VisionEngine
digital.vasic.challenges => ../Challenges
digital.vasic.security => ../Security
```

---

## 9. Power Features Analysis

### 9.1 Implemented Power Features

| Feature | Implementation | Rating |
|---------|---------------|--------|
| **29+ LLM Providers** | Factory pattern with 18 cloud + 11 local | ⭐⭐⭐⭐⭐ |
| **Auto LLM Management** | Zero-touch discovery/install/configure/monitor | ⭐⭐⭐⭐⭐ |
| **Load Balancing** | 5 strategies (performance-based default) | ⭐⭐⭐⭐⭐ |
| **Multi-Agent System** | 8 agent types with collaboration | ⭐⭐⭐⭐⭐ |
| **Circuit Breaker** | Per-agent resilience with retries | ⭐⭐⭐⭐⭐ |
| **Tool Framework** | 20+ tools across 8 categories | ⭐⭐⭐⭐⭐ |
| **Multi-Edit** | Atomic multi-file transactions | ⭐⭐⭐⭐⭐ |
| **Context Builder** | Mentions, templates, sources | ⭐⭐⭐⭐ |
| **Memory System** | 11+ providers, conversation management | ⭐⭐⭐⭐⭐ |
| **Distributed Workers** | SSH-based worker pools | ⭐⭐⭐⭐ |
| **Workflow Engine** | Planning/building/testing/refactoring | ⭐⭐⭐⭐ |
| **QA Integration** | Screenshot-based testing | ⭐⭐⭐⭐ |
| **Edit Formats** | 7 editor formats | ⭐⭐⭐⭐ |
| **Code Mapping** | Tree-sitter based analysis | ⭐⭐⭐⭐⭐ |
| **Browser Automation** | ChromeDP screenshot/console | ⭐⭐⭐⭐ |
| **Notification System** | Discord/Slack/Telegram/Email/Webhook | ⭐⭐⭐⭐ |
| **Verifier Subsystem** | Score-augmented model selection | ⭐⭐⭐⭐⭐ |
| **Cross-Platform UI** | Terminal/Desktop/Android/iOS/Specialized OS | ⭐⭐⭐⭐⭐ |
| **API Server** | Full REST API with OpenAPI spec | ⭐⭐⭐⭐⭐ |
| **Config System** | Viper + env + validation + migration | ⭐⭐⭐⭐⭐ |
| **Security** | JWT, Bcrypt, confirmation policies, sandbox | ⭐⭐⭐⭐⭐ |

### 9.2 Unique Innovations

1. **AutoLLMManager**: Fully automated local LLM lifecycle - discovers, installs, configures, starts, monitors, and updates local LLM providers without manual intervention
2. **Cross-Provider Registry**: Unified model naming across 29+ providers with fallback chains
3. **Verifier Bridge**: Score-augmented model selection that verifies model capabilities before selection
4. **Multi-Agent Collaboration**: Built-in collaboration patterns (coding→review, coding→testing, debugging→verification)
5. **Confirmation Audit System**: Danger-level policies with JSONL audit trails
6. **HelixQA Engine**: Integrated quality assurance with screenshot capture across platforms
7. **Specialized OS Support**: Aurora OS and Harmony OS native applications

---

## 10. Missing Features vs State-of-the-Art CLI Agents

### 10.1 Compared to Aider, Claude Code, Cline, Continue

| Feature | HelixCode Status | Gap Analysis |
|---------|-----------------|--------------|
| **Interactive Chat REPL** | ⚠️ Basic | Missing rich inline chat with syntax highlighting |
| **Git Integration** | ⚠️ Partial | Has git tools but missing deep git-aware context |
| **Repository Map (Aider-style)** | ✅ Strong | Tree-sitter based with file ranking |
| **Voice Input** | ⚠️ Partial | Voice tools exist but not integrated in CLI |
| **Cost Tracking** | ⚠️ Basic | Budget in model selection but no per-session cost |
| **Lint/Test Integration** | ⚠️ Partial | Workflow exists but not auto-triggered |
| **Undo/Redo Stack** | ❌ Missing | No explicit undo/redo for edits |
| **File Watching** | ⚠️ Partial | fsnotify in deps but not integrated for auto-reload |
| **Multi-line Input** | ❌ Missing | No multi-line prompt support in CLI |
| **Inline Diff Preview** | ⚠️ Partial | Editor formats exist but no TUI diff preview |
| **Auto-commit Messages** | ✅ Present | `internal/tools/git/message_generator.go` |
| **Shell Command Suggestions** | ⚠️ Partial | Shell tool exists but no smart suggestions |
| **Model Aliases** | ✅ Present | `internal/llm/aliases.go` |
| **Token Counting** | ✅ Present | Conversation token management |
| **Context Compression** | ✅ Present | `internal/llm/compression/` |
| **Reasoning Models** | ✅ Present | `internal/llm/reasoning.go` |
| **Vision Support** | ✅ Present | `internal/llm/vision/` |
| **Caching** | ✅ Present | Response caching in AutoLLMManager |
| **Streaming Responses** | ⚠️ Partial | Streaming supported in providers but not fully integrated |
| **MCP Protocol** | ✅ Present | Full MCP server with WebSocket |
| **Checkpoint/Restore** | ✅ Present | Task checkpoint system |
| **Dependency Management** | ✅ Present | Task dependency graph |

### 10.2 Critical Gaps

1. **No Streaming TUI**: The terminal-ui app exists but doesn't show real-time streaming LLM responses
2. **Limited Inline Editing**: Edit formats exist but no interactive inline editing experience
3. **No File Watch Auto-Reload**: fsnotify is a dependency but not used for automatic context refresh
4. **No Smart Completion**: No shell-like tab completion for file paths or commands
5. **No Session Persistence in CLI**: Sessions exist in server mode but not in CLI mode
6. **Missing Rich Error Recovery**: Limited automatic error recovery beyond circuit breakers
7. **No Plugin System**: No external plugin/extension mechanism beyond built-in tools

---

## 11. Performance Characteristics

### 11.1 Scalability Design
- **Worker Pool**: SSH-based horizontal scaling
- **Load Balancing**: Performance-based provider selection
- **Caching**: Redis + in-memory provider response caching
- **Circuit Breakers**: Automatic failure isolation
- **Context Compression**: Token budget management

### 11.2 Concurrency Patterns
- **Agent Task Queue**: Buffered channels with configurable concurrency
- **Context Manager**: RWMutex for read-heavy operations
- **Tool Registry**: RWMutex for safe concurrent access
- **Memory Manager**: RWMutex for conversation operations
- **Server**: Gin's goroutine-per-request model

### 11.3 Memory Management
- **Conversation Limits**: Configurable max messages (1000) and tokens (100000)
- **Auto-Truncation**: Keeps last 50% of messages when limit exceeded
- **Context TTL**: Automatic expiration with cleanup routine
- **Redis Offloading**: Optional Redis for distributed caching

---

## 12. Security Architecture

### 12.1 Security Layers
| Layer | Implementation |
|-------|---------------|
| Auth | JWT with bcrypt, configurable expiry |
| Transport | HSTS, CORS, security headers |
| Confirmation | Danger-level policies with audit trails |
| Sandbox | Auto-sandbox for LLM providers |
| Privileges | Minimum privilege mode |
| Network Isolation | Optional network isolation |

### 12.2 Security Tools
- `cmd/security_fix/` - Automated security fix utility
- `cmd/security_test/` - Security testing utility
- `internal/security/` - Core security package
- `internal/tools/confirmation/` - Confirmation with audit logging

---

## 13. Deployment & Operations

### 13.1 Deployment Targets
- **Docker**: `docker-compose.yml`, `docker-compose.full-test.yml`
- **Specialized Platforms**: Aurora OS, Harmony OS
- **Cloud**: Azure provider integration
- **Local**: Ollama, LMStudio, and 9 other local providers

### 13.2 Monitoring
- **Health Checks**: `/health` endpoint with DB and Redis checks
- **Metrics**: `/metrics` endpoint
- **Worker Metrics**: CPU, memory, disk, network, temperature
- **LLM Health**: Per-provider health monitoring
- **Notification**: Multi-channel alerting

---

## 14. Summary of Architectural Strengths

1. **Modularity**: 60+ well-separated internal packages with clear interfaces
2. **Extensibility**: Factory pattern for LLM providers, tool registry for tools, command registry for commands
3. **Resilience**: Circuit breakers, retry policies, health monitoring, graceful degradation
4. **Scale**: Distributed worker pools, load balancing, Redis caching
5. **Cross-Platform**: 6 platform targets with shared core
6. **Security**: Multi-layer security with audit trails
7. **Testing**: Comprehensive test suite with automation, E2E, load testing
8. **Configuration**: Mature config system with validation, migration, templates

## 15. Summary of Architectural Weaknesses

1. **Complexity**: Very large codebase with many interdependencies
2. **Documentation Gap**: Many packages have minimal documentation beyond doc.go
3. **Mock Dependency**: Heavy reliance on mocks for testing (may mask integration issues)
4. **CLI Experience**: Basic CLI compared to rich TUI applications
5. **External Modules**: Hardcoded relative paths for external modules (`../Containers`, `../HelixQA`)
6. **Version Mismatch**: Claims Go 1.26 but uses features that may not be stable

---

*Report generated from analysis of HelixCode repository at https://github.com/HelixDevelopment/HelixCode*
*Focus Path: `HelixCode/` directory*
