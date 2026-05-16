# CRUSH.md - HelixCode Development Guide

This document contains essential information for agents working with the HelixCode distributed AI development platform.

## Constitutional anchors (cascaded from `CONSTITUTION.md`)

### Article XI §11.9 — Anti-Bluff Forensic Anchor
> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks. No false-success results are tolerable.

### Article XII §12.1 (CONST-042) — No-Secret-Leak
No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak is a release blocker until rotated and post-mortemed.

### Article XII §12.2 (CONST-043) — No-Force-Push
No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval per operation. Authorization for one push does not extend further. Bypassing hooks / signing / protected-branch rules also requires explicit approval.

### CONST-048 — Full-Automation-Coverage Mandate (cascaded from constitution submodule §11.4.25)
No feature/functionality/flow/use-case/edge-case/service/application on any supported platform of HelixCode is deliverable until covered by automation tests proving six invariants: (1) anti-bluff posture with captured runtime evidence; (2) proof of working capability end-to-end on target topology; (3) implementation matches documented promise; (4) no open issues/bugs surfaced; (5) full documentation in sync; (6) four-layer test floor. Coverage ledger regenerated at release-gate. See constitution submodule `Constitution.md` §11.4.25 for the full mandate.

### CONST-049 — Constitution-Submodule Update Workflow Mandate (cascaded from constitution submodule §11.4.26)
Before any modification to `constitution/{Constitution,CLAUDE,AGENTS}.md`: (1) fetch + pull first; (2) apply with §11.4.17 classification + verbatim mandate; (3) validate; (4) commit + push to EVERY configured upstream; (5) resolve conflicts preserving union — force-push forbidden; (6) cascade verification (CONST-047); (7) bump `.gitmodules` pointer in SAME commit. See constitution submodule `Constitution.md` §11.4.26 for the full mandate.

### CONST-050 — No-Fakes-Beyond-Unit-Tests + 100%-Test-Type-Coverage Mandate (cascaded from constitution submodule §11.4.27)
**(A)** Mocks/stubs/fakes/placeholders/TODOs/FIXMEs/"for now"/empty-implementation patterns PERMITTED only in unit-test sources; non-unit tests MUST exercise the real, fully implemented system. Production code MUST NOT import `helix_code/internal/mocks/`. **(B)** 100% test-type coverage: unit + integration + E2E + full-automation + security + DDoS + scaling + chaos + stress + performance + benchmarking + UI + UX + Challenges (vasic-digital/Challenges submodule at `./challenges/`) + helix_qa (HelixDevelopment/HelixQA submodule at `./helix_qa/`, with full autonomous QA sessions). See constitution submodule `Constitution.md` §11.4.27 for the full mandate.

### CONST-051 — Submodules-As-Equal-Codebase + Decoupling + Dependency-Layout Mandate (cascaded from constitution submodule §11.4.28)
**(A)** Every owned-by-us submodule (orgs: vasic-digital, HelixDevelopment, red-elf, ATMOSphere1234321, Bear-Suite, BoatOS123456, Helix-Flow, Helix-Track, Server-Factory — dynamically discoverable via gh/glab) is an EQUAL part of HelixCode's codebase. Same engineering attention as main (analysis, tests, gap-fill, bug-fix, docs/diagrams/SQL/website materials). **(B)** Submodules MUST stay fully decoupled — NEVER inject HelixCode-specific context; use configuration injection when needed. **(C)** Dependencies of owned submodules MUST live at HelixCode root (`<root>/<name>/` or `<root>/submodules/<name>/`); nested own-org submodule chains FORBIDDEN. Third-party submodules exempt. See constitution submodule `Constitution.md` §11.4.28 for the full mandate.

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform built in Go that enables intelligent task division, work preservation, and cross-platform development workflows. It features:

- **Distributed Computing**: SSH-based worker pools with automatic management and health monitoring
- **Multi-Provider LLM Integration**: Support for Llama.cpp, Ollama, OpenAI, Anthropic, Gemini, Qwen, xAI, OpenRouter, GitHub Copilot, and more
- **Development Workflows**: Automated planning, building, testing, refactoring, debugging, and deployment workflows
- **Task Management**: Intelligent task division with dependency tracking, checkpointing, and automatic rollback
- **MCP Protocol**: Full Model Context Protocol implementation with multi-transport support
- **Multi-Client Architecture**: REST API, CLI, Terminal UI, WebSocket, mobile, and specialized platform support

## Project Structure

```
helix_code/
├── cmd/                      # Application entry points
│   ├── server/              # Main HTTP server
│   ├── cli/                 # CLI client
│   ├── tui/                 # Terminal UI
│   └── desktop/             # Desktop client
├── internal/                # Internal packages (not importable externally)
│   ├── auth/                # Authentication & authorization
│   ├── worker/              # Worker pool & SSH management
│   ├── task/                # Task management & checkpoints
│   ├── llm/                 # LLM provider implementations
│   ├── mcp/                 # MCP protocol
│   ├── workflow/            # Workflow engine
│   ├── project/             # Project management
│   ├── session/             # Session tracking
│   ├── notification/        # Notification channels
│   ├── hardware/            # Hardware detection
│   ├── server/              # HTTP server & routes
│   ├── database/            # Database layer
│   ├── redis/               # Redis client
│   ├── config/              # Configuration management
│   ├── tools/               # Development tools
│   ├── editor/              # Code editing interfaces
│   ├── agent/               # AI agent coordination
│   ├── context/             # Context building
│   ├── focus/               # Focus management
│   ├── hooks/               # System hooks
│   ├── memory/              # Memory management
│   ├── persistence/         # Data persistence
│   ├── repomap/             # Repository mapping
│   ├── rules/               # Rule system
│   ├── template/            # Template system
│   ├── discovery/           # Service discovery
│   ├── event/               # Event bus
│   ├── commands/            # Command system
│   └── logo/                # Logo processing
├── applications/            # Platform-specific applications
│   ├── desktop/             # Desktop app (Fyne)
│   ├── terminal-ui/         # TUI app (tview)
│   ├── ios/                 # iOS app
│   ├── android/             # Android app
│   ├── aurora-os/           # Aurora OS client
│   └── harmony-os/          # Harmony OS client
├── config/                  # Configuration files
├── scripts/                 # Build and utility scripts
├── tests/                   # Test suites and frameworks
├── docker/                  # Docker configurations
├── docs/                    # Documentation
├── examples/                # Example usage
├── assets/                  # Static assets
├── shared/                  # Shared mobile code
└── mobile/                  # Mobile-specific code
```

## Essential Build Commands

### Development

```bash
cd HelixCode

# Build main application
make build                    # Builds to bin/helixcode

# Development server
make dev                      # Build and run with dev config

# Generate assets (logo, themes)
make logo-assets

# Setup development environment
make dev-setup                # Install dependencies and setup
```

### Testing

```bash
# Run all tests
make test

# Run comprehensive test suite
./scripts/run-tests.sh all

# Run specific test types
./scripts/run-tests.sh unit           # Unit tests only
./scripts/run-tests.sh integration    # Integration tests only
./scripts/run-tests.sh e2e           # End-to-end tests only
./scripts/run-tests.sh coverage       # Generate coverage report

# Performance tests
./scripts/run-tests.sh performance

# Test specific packages
go test -v ./internal/auth/
go test -v ./internal/worker/
go test -cover ./...
```

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean
```

### Production Builds

```bash
# Cross-platform builds
make prod                     # Build for Linux, macOS, Windows

# Mobile bindings
make mobile-ios               # Build iOS framework
make mobile-android           # Build Android AAR
make mobile                   # Build all mobile bindings

# Specialized platforms
make aurora-os                # Build Aurora OS client
make harmony-os               # Build Harmony OS client
make aurora-harmony           # Build both specialized platforms

# Full release build
make release                  # Clean + assets + docs + build + test
```

## Configuration Management

### Primary Configuration

Main configuration at `config/config.yaml` with environment variable overrides:

```yaml
server:
  address: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  dbname: "helixcode"
  # Password via HELIX_DATABASE_PASSWORD

auth:
  # JWT secret via HELIX_AUTH_JWT_SECRET
  token_expiry: 86400
  session_expiry: 604800

workers:
  health_check_interval: 30
  max_concurrent_tasks: 10

tasks:
  max_retries: 3
  checkpoint_interval: 300

llm:
  default_provider: "local"
  max_tokens: 4096
  temperature: 0.7
```

### Environment Variables

**Required for Production:**
```bash
export HELIX_DATABASE_PASSWORD="your_secure_password"
export HELIX_AUTH_JWT_SECRET="your_jwt_secret"
export HELIX_REDIS_PASSWORD="your_redis_password"  # if Redis enabled
```

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

## Code Style & Patterns

### Go Conventions

**Language Version:** Go 1.24.0 (toolchain go1.24.9)
**Module:** `dev.helix.code`

**Naming Conventions:**
- **Types**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase, descriptive names
- **Constants**: PascalCase, grouped by functionality
- **Interfaces**: Simple capability names (Provider, Repository, Manager)

**Import Organization:**
1. Standard library imports
2. Third-party imports
3. Internal imports (dev.helix.code/internal/...)
4. Blank line between groups

**Error Handling:**
- Return errors with context using `fmt.Errorf("failed to X: %v", err)`
- Check errors immediately after operations
- Use package-level error variables: `ErrInvalidCredentials`
- Structured error responses in HTTP handlers

### Architectural Patterns

**Interface-Driven Design:**
- Core interfaces define contracts (Provider, Repository, Manager)
- Multiple implementations per interface (various LLM providers)
- Factory pattern for provider creation
- Easy mocking for unit tests

**Dependency Injection:**
- Constructor injection: `NewService(config Config, repo Repository)`
- Configuration-based service initialization
- Clean separation between layers

**Manager Pattern:**
- Centralized managers (TaskManager, WorkerManager, ProviderManager)
- Thread-safe operations with `sync.RWMutex`
- Encapsulate complex business logic

**Repository Pattern:**
- Data access abstraction via interfaces
- Database-agnostic implementations
- Redis caching integrated transparently

## Testing Patterns

### Test Structure

**File Organization:**
- Test files alongside source: `auth_test.go` next to `auth.go`
- Comprehensive test coverage required
- Separate test packages for complex scenarios

**Test Patterns:**
- Table-driven tests for multiple scenarios
- Subtests with `t.Run("name", func(t *testing.T) {...})`
- Setup/teardown with helper functions
- Mock implementations using `testify/mock`

**Assertions:**
```go
require.NoError(t, err)              # Critical operations
assert.Equal(t, expected, actual)     # Value comparisons
assert.Contains(t, result, substring) # String contains
```

**Mock Usage:**
- Interface-based mocking for isolation
- `mock.Mock` for behavior verification
- Test doubles for external dependencies

### Test Categories

**Unit Tests:**
- Fast, isolated tests for individual functions
- Mock all external dependencies
- Tagged with `unit` build tag

**Integration Tests:**
- Test interaction between components
- Use real databases/Redis in Docker
- Tagged with `integration` build tag

**End-to-End Tests:**
- Full workflow testing
- Docker Compose test environment
- Mock external services (LLM providers)
- Tagged with `e2e` build tag

## Database Setup

### PostgreSQL

```bash
# Create database and user
createdb helixcode
createuser helixcode

# Set password via environment variable
export HELIX_DATABASE_PASSWORD=your_password

# Schema is automatically created by the application
```

### Key Tables
- `users` - User authentication and profiles
- `workers` - Worker node registration and status
- `tasks` - Task management and state
- `projects` - Project metadata and sessions
- `sessions` - User session tracking
- `llm_providers` - LLM provider configurations
- `notifications` - Notification rules and history

## CLI Usage

### Build CLI

```bash
cd HelixCode
go build -o bin/cli ./cmd/cli
```

### Common Commands

```bash
# Interactive mode
./bin/cli

# Worker management
./bin/cli --list-workers
./bin/cli --worker worker-host --user helix --key ~/.ssh/id_rsa

# AI-powered generation
./bin/cli --prompt "Hello world" --model llama-3-8b --max-tokens 1000

# Notifications
./bin/cli --notify "Build complete" --notify-type "success"

# Health check
./bin/cli --health
```

## AI Provider Integration

### Free Providers (No API Key Required)

**XAI (Grok):**
```bash
helixcode llm provider set xai
# Models: grok-3-fast-beta, grok-3-mini-fast-beta, grok-3-beta
```

**OpenRouter:**
```bash
helixcode llm provider set openrouter
# Models: deepseek-r1-free, meta-llama/llama-3.2-3b-instruct:free
```

**GitHub Copilot:**
```bash
export GITHUB_TOKEN="ghp_your_github_token"
helixcode llm provider set copilot
# Models: gpt-4o, claude-3.5-sonnet, claude-3.7-sonnet, o1, gemini-2.0-flash
```

### Premium Providers

**Anthropic Claude (Extended Thinking & Caching):**
```bash
export ANTHROPIC_API_KEY="sk-ant-your-key"
helixcode llm provider set anthropic --model claude-4-sonnet
# Features: Extended thinking, prompt caching, 200K context, 50K output
```

**Google Gemini (2M Token Context):**
```bash
export GEMINI_API_KEY="your-gemini-key"
helixcode llm provider set gemini --model gemini-2.5-pro
# Features: 2M token context, multimodal, function calling
```

## Important Gotchas & Patterns

### SSH Worker Auto-Install
- When adding workers via SSH, the system automatically installs the Helix CLI binary on remote machines
- Requires SSH key-based authentication
- Workers are health-checked every 30s by default

### Task Checkpointing
- Long-running tasks automatically checkpoint at configured intervals (default 300s)
- Enables work preservation and recovery from failures
- Checkpoints stored in PostgreSQL for persistence

### Provider Fallback
- LLM requests can fall back to alternative providers if the primary fails
- Configurable provider priority and retry logic
- Automatic rate limiting and quota management

### Session Context
- Development sessions maintain context across interactions for continuity
- Context stored in Redis for fast access
- Automatic cleanup of expired sessions

### MCP Protocol
- Supports both stdio and SSE transports for Model Context Protocol
- Tool integration and execution through standardized interface
- Real-time bidirectional communication

### Mobile & Specialized Platforms
- Gomobile bindings for iOS/Android applications
- Aurora OS (Russian platform) and Harmony OS (Chinese platform) support
- Cross-platform UI with Fyne framework

## Development Workflow

### Before Making Changes
1. Read existing code in the relevant package
2. Check for similar patterns in other packages
3. Run tests to ensure baseline functionality
4. Make incremental changes with tests

### After Changes
1. Run `make test` to ensure all tests pass
2. Run `make fmt` and `make lint` for code quality
3. Test specific functionality with targeted tests
4. Update documentation if needed

### Commit Process
1. Use conventional commit messages
2. Include tests for new functionality
3. Update relevant documentation
4. Ensure CI/CD pipeline would pass

## Dependencies

### Core Dependencies
- `github.com/gin-gonic/gin`: HTTP framework
- `github.com/jackc/pgx/v5`: PostgreSQL driver
- `github.com/golang-jwt/jwt/v4`: JWT authentication
- `github.com/spf13/viper`: Configuration management
- `github.com/gorilla/websocket`: WebSocket support
- `golang.org/x/crypto/ssh`: SSH client for workers
- `github.com/google/uuid`: UUID generation
- `github.com/stretchr/testify`: Testing framework

### UI Frameworks
- `fyne.io/fyne/v2`: Desktop GUI framework
- `github.com/rivo/tview`: Terminal UI framework

### External Integrations
- AWS SDK v2: Bedrock integration
- Azure SDK: Azure AI services
- Chromedp: Browser automation
- Go-Redis: Redis client

## Cross-Platform Support

**Standard Platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

**Mobile Platforms:**
- iOS (via gomobile)
- Android (via gomobile)

**Specialized Platforms:**
- Aurora OS (Russian platform)
- Harmony OS (Chinese platform)

## Code Generation & Assets

### Logo Assets

```bash
# Generate logo assets and themes
make logo-assets

# Manual generation
cd scripts/logo && go run generate_assets.go
```

This extracts colors and creates themed variations of the logo for different platforms and use cases.

## Monitoring & Observability

### Health Checks
- Application health endpoint: `/health`
- Worker health monitoring
- Database connectivity checks
- External service availability

### Metrics
- Prometheus metrics endpoint: `/metrics`
- Task execution statistics
- Worker performance metrics
- LLM provider response times
- Resource usage tracking

### Logging
- Structured logging with levels
- Contextual information in logs
- Integration with external logging systems
- Security event logging

## Security Considerations

- JWT-based authentication with configurable expiry
- Password hashing with bcrypt
- SSH key-based worker authentication
- Environment variable configuration (no secrets in code)
- Rate limiting on API endpoints
- Input validation and sanitization
- HTTPS enforcement in production

## Performance Optimization

### Database
- Connection pooling with pgx
- Query optimization and indexing
- Prepared statements for repeated queries
- Read replica support for scaling

### Caching
- Redis for session and task state
- LLM response caching with TTL
- Static asset caching
- Database query result caching

### Concurrency
- Goroutine pools for task execution
- Channel-based communication
- Mutex protection for shared state
- Context-based cancellation

This guide provides the essential information needed for effective development in the HelixCode codebase. Follow these patterns and conventions to maintain consistency and quality across the platform.

### CONST-052 — Lowercase-Snake_Case-Naming Mandate (cascaded from constitution submodule §11.4.29)
Every directory/submodule/file MUST use lowercase snake_case names. Existing non-compliant names MUST be renamed atomically with updates to all references (configs, docs, source-code imports, governance files). Common-sense exceptions: language-mandated case (Java/Kotlin/Android/Apple/C#/Swift) inside language-root, vendor third-party submodules, build artefacts. `Upstreams/` → `upstreams/` transition: `install_upstreams.sh` supports BOTH directory names during migration. Phased execution; each rename batch ships with (i) reference-resolution regression test, (ii) full CONST-050(B) test-type matrix run, (iii) anti-bluff wire-evidence. See root `CONSTITUTION.md` §CONST-052 and constitution submodule `Constitution.md` §11.4.29 for the full mandate.


## CONST-053: .gitignore + No-Versioned-Build-Artifacts Mandate (cascaded from constitution submodule §11.4.30)

> Verbatim user mandate (2026-05-15): *"every project module, every Submodule, every servcie and apolication MUST HAVE proper .gitignore file! We MUST NOT git version build artifacts, cache files, tmp files, main .env file(s) or any files containing sensitive data, API keys or token! Any build derivate which we can recreate by executing proper mechanism for generating MUST NOT be versioned! We MUST pay attention what is going to be commited every time we are preparing to execute commit! If any violetion is detected it MUST be fixed before commit is executed!"*

Every project module, owned-by-us submodule, service, and application MUST ship a proper `.gitignore`. Forbidden-from-version-control classes:

1. **Build artefacts**: `/bin/`, `/build/`, `/dist/`, `/out/`, `target/`, `*.exe`, `*.dll`, `*.so`, `*.dylib`, `*.a`, `*.o`, `*.class`, `*.pyc`, generator-produced files when the generator is committed.
2. **Cache files**: `__pycache__/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `node_modules/`, `.next/`, `.cache/`, `.gradle/`, `.terraform/`, language-server caches.
3. **Temp files**: `*.tmp`, `*.swp`, `*~`, `.DS_Store`, `Thumbs.db`, `*.orig`, `*.rej`.
4. **Sensitive-data files**: `.env`, `.env.*` (allow `.env.example` placeholder only — no real secrets even as examples), `*.pem`, `*.key`, `*.crt`, `id_rsa*`, `id_ed25519*`, `.netrc`, `secrets/`, `api_keys.sh`.
5. **Generated reports/logs**: `*.log`, `coverage.out`, `htmlcov/`, runtime captures unless reference assets.
6. **OS/IDE personal state**: `.idea/`, `.history/`, `.vscode/` (except shared settings).

**Anti-bluff invariant**: `.gitignore` line alone is not sufficient — no file matching the forbidden patterns may be CURRENTLY TRACKED. A tracked `*.log` despite the ignore-line is a violation of equal severity to no ignore-line at all.

**Pre-commit attention**: every commit author (human OR agent) MUST inspect `git diff --staged` + `git status` BEFORE executing the commit. Forbidden-class hits abort the commit until fixed (un-stage, add to `.gitignore`, scrub if already-tracked). Gate `CM-GITIGNORE-PRECOMMIT-AUDIT` + paired mutation.

**Secret-leak intersection (CONST-042 / §11.4.10):** a `.env` leak is BOTH a CONST-053 and a CONST-042 violation; rotation + post-mortem required.

**Recreatable-content test**: if a documented mechanism regenerates the file from sources, it is a build derivative and MUST be ignored. The committed sources MUST include the generator.

**Cascade requirement:** This anchor (verbatim or by `CONST-053` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a §11.4 PASS-bluff at the repository-hygiene layer. See constitution submodule `Constitution.md` §11.4.30 for the full mandate.


## CONST-054: Submodule-Dependency-Manifest Mandate (cascaded from constitution submodule §11.4.31)

> Verbatim user mandate (2026-05-15): *"We MUST HAVE mechanism for each Submodule to determine / know what are its Submodule dependencies so new projects or palces we are incorporate them can add these Submodules to the project root and make them available! Suggested idea is configuration file with expected Submodules Git ssh urls perhaps? New project can read it, and recursively add each Submodule to the root of the project and install / expose it to veryone."*

Every owned-by-us submodule MUST ship `helix-deps.yaml` at its root declaring its own-org dependencies. Schema: `schema_version`, `deps: [{name, ssh_url, ref, why, layout: flat|grouped}]`, `transitive_handling.{recursive,conflict_resolution}`, `language_specific_subtree`. Tooling: `incorporate-submodule <ssh-url>` adds the submodule at the parent project's canonical path (CONST-051(C)), reads `helix-deps.yaml`, recurses for each declared dep, aborts on conflicting refs, emits `<root>/.helix-manifest.yaml` audit record.

Anti-bluff guarantee: every manifest paired with a Challenge that bootstraps a throwaway consuming project, runs `incorporate-submodule`, asserts produced layout matches the manifest, runs the submodule's own tests against the bootstrapped layout, captures wire evidence per §11.4.2. A manifest without this proof is a CONST-054 violation.

§11.4.31 / CONST-054 is the **operational complement** of CONST-051(C): nested own-org submodule chains are FORBIDDEN — manifests are the bridge that lets consumers reconstruct the dependency graph at the parent root.

**Cascade requirement:** This anchor (verbatim or by `CONST-054` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to §11.4 PASS-bluff at the dependency-graph layer. See constitution submodule `Constitution.md` §11.4.31 for the full mandate.

## CONST-055: Post-Constitution-Pull Validation Mandate (cascaded from constitution submodule §11.4.32)

> Verbatim user mandate (2026-05-15): *"Every time we fetch and pull new changes on constitution Submodule we MUST process the whole project and all Submodule (deep recursively) for validation and verification taht every single rule or mandatory constraint is followed and respected! If it is not, IT MUST BE!"*

Whenever a project's constitution submodule is fetched + pulled with any content change, the project MUST run `scripts/verify-all-constitution-rules.sh` BEFORE the new constitution HEAD is treated as canonical for any other work. The sweep re-runs the governance-cascade verifier AND every implementable rule gate (CONST-053 `.gitignore` audit, CONST-051(C) nested-own-org-chain audit, CONST-052 case audit, CONST-050(A) mock-from-production audit, CONST-035 anti-bluff smoke, etc.) against the post-pull tree. Failures populate the project's Issues tracker per §11.4.15 (Status: `Reopened`, Type: `Bug`); closure requires positive-evidence per §11.4.

Pull-time invocation: `git submodule update --remote constitution` triggers the sweep automatically (post-update hook OR commit-wrapper invocation). Operator-explicit manual invocation also available.

Anti-bluff: the sweep's own meta-test (paired mutation per §1.1) plants a known violation of each enforced gate and asserts the sweep reports FAIL for the planted gate. A sweep that exits PASS without running every implementable gate is a CONST-055 violation.

CONST-055 is the **enforcement engine** for every other §11.4.x and CONST-NNN rule — without it, new rules cascade as anchors but never get enforced.

**Cascade requirement:** This anchor (verbatim or by `CONST-055` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to §11.4 PASS-bluff at the constitutional-enforcement layer. See constitution submodule `Constitution.md` §11.4.32 for the full mandate.


## CONST-056: Mandatory install_upstreams on clone/add Mandate (cascaded from constitution submodule §11.4.36)

> Verbatim user mandate (2026-05-15): *"Every Submodule or Git repository we add or clone MUST BE upstreams installed using Upstreamable utility which MUST BE available through exported paths of the host system (in .bashrc or .zhrc) using install_upstreams command executed from the root of the cloned (added) repository - only if in it is Upstreams or upstreams directory present with bash script files (recipes) for all repository's upstreams!"*

Every clone / add of a Git repository under HelixCode MUST be followed by `install_upstreams` invocation from the repository's root IF its tree contains `upstreams/` (or legacy `Upstreams/` per CONST-052 transition) populated with `*.sh` recipe files. The utility (installed on operator's `PATH` via `.bashrc`/`.zshrc`; implementation in the constitution submodule's `install_upstreams.sh` — already supports BOTH directory names since constitution commit `45d3678`) reads the recipe files, configures every declared upstream as a named git remote, and fans out `origin` push URLs.

Skipping the invocation when `upstreams/` is present silently breaks §2.1 (multi-upstream push is the norm) — the next push lands on only one upstream. Gate `CM-INSTALL-UPSTREAMS-ON-CLONE` + paired mutation. Automation: the future `incorporate-submodule` per CONST-054 auto-invokes; manual invocation supported. Pre-commit check: `git remote -v | grep -c push` reports expected count.

**Cascade requirement:** This anchor (verbatim or by `CONST-056` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.36 for the full mandate.


## CONST-057: Type-aware Closure-Status Vocabulary (cascaded from constitution submodule §11.4.33)

Every project tracking work items by Type per §11.4.16 MUST close them with the Type-appropriate terminal `**Status:**` value, drawn from this 3-element closed map:

| Item `**Type:**` | Closure `**Status:**` value     |
|------------------|---------------------------------|
| `Bug`            | `Fixed (→ Fixed.md)`            |
| `Feature`        | `Implemented (→ Fixed.md)`      |
| `Task`           | `Completed (→ Fixed.md)`        |

The `(→ Fixed.md)` suffix is preserved across all three so the existing migration-discipline tooling (atomic Issues.md → Fixed.md move per §11.4.19) keeps working without per-Type branching. Generators (`generate_issues_summary.sh`, `generate_fixed_summary.sh`, the §11.4.23 colorizer) MUST treat the three terminal values as semantically equivalent (all "closed, positive evidence captured") while preserving the literal in the emitted document.

Closing a `Feature` with `Fixed (→ Fixed.md)` or a `Task` with `Implemented (→ Fixed.md)` is a CONST-057 violation. Gate `CM-CLOSURE-VOCAB-TYPE-AWARE` walks every Fixed.md heading + every Issues.md heading whose `**Status:**` is one of the three terminal values and asserts the Status-Type match. Composes with §11.4.15 / §11.4.16 / §11.4.19 / §11.4.23.

**Cascade requirement:** This anchor (verbatim or by `CONST-057` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.33 for the full mandate.

## CONST-058: Reopened-Source Attribution Mandate (cascaded from constitution submodule §11.4.34)

Every Issues.md (or equivalent project tracker) heading whose `**Status:**` is `Reopened` MUST carry, within 8 non-blank lines of the heading, a `**Reopened-Details:**` line capturing four sub-facts:

- **By:** `AI` or `User` (source-of-truth observer who flipped the status). `AI` covers in-loop reopens (test failure, gate regression, captured-evidence retrospect). `User` covers operator-side observations (manual testing, end-user report, design reconsideration).
- **On:** ISO date (`YYYY-MM-DD`).
- **Reason:** one-line cause classification — chosen from the closed vocabulary `{ test-failed | manual-testing-detected | captured-evidence-contradicts | end-user-report | cycle-re-discovered | design-reconsidered }`. Other values permitted with explicit `Reason: <free text>` annotation but the closed list MUST be tried first.
- **Evidence:** path to or short description of the captured artefact justifying the reopen — log file, recording, gate failure ID, operator quote, etc. Reopens without evidence are §11.4.6 / §11.4.7 violations (demotion from Fixed requires captured evidence under the conditions that re-exposed the defect).

The Issues_Summary.md Status column MUST distinguish the four `Reopened` sub-states by source so a sweep query for "reopens by AI in the last 30 days" is mechanically possible. Suggested column rendering: `Reopened (AI: test-failed)` vs `Reopened (User: manual-testing)`. Gate `CM-ITEM-REOPENED-DETAILS` mirrors `CM-ITEM-OPERATOR-BLOCKED-DETAILS` (§11.4.21 walk pattern). Composes with §11.4.6 / §11.4.7 / §11.4.15 / §11.4.21.

**Cascade requirement:** This anchor (verbatim or by `CONST-058` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.34 for the full mandate.

## CONST-059: Canonical-Root Inheritance Clarity (cascaded from constitution submodule §11.4.35)

The **constitution submodule's** three files (`constitution/Constitution.md`, `constitution/CLAUDE.md`, `constitution/AGENTS.md`) ARE the **canonical root** (also called the **parent** files). They contain only universal rules per §11.4.17.

The consuming project's **repository-root files** (`<project-root>/CLAUDE.md`, `<project-root>/AGENTS.md`, optionally `<project-root>/Constitution.md`) are **consumer extensions**. They MUST start with the inheritance pointer (either the Claude-Code native `@constitution/CLAUDE.md` import or the portable `## INHERITED FROM constitution/CLAUDE.md` heading). They contain only project-specific rules per §11.4.17.

**When in doubt about which file to edit:** universal rule → constitution submodule's file; project-specific rule → consumer's file. Default consumer-side when uncertain (§11.4.17 — narrower scope is cheap to widen).

**Terminology:** "the parent CLAUDE.md" / "the root Constitution" → constitution-submodule file at `constitution/<filename>`; "the project CLAUDE.md" / "this project's AGENTS.md" → consumer-side file at `<project-root>/<filename>`.

**No silent demotion or silent promotion.** Moving a rule between layers MUST be a visible commit — `git mv` of a section if it's a clean clone, or explicit `Lifted from <project> to constitution per §11.4.35` / `Demoted from constitution to <project> per §11.4.35` commit-message annotation.

Gate `CM-CANONICAL-ROOT-CLARITY` verifies (a) consumer's `CLAUDE.md` opens with the inheritance pointer, (b) constitution submodule's three files are present at the expected path, (c) no `## INHERITED FROM` block in the constitution submodule's own files (those ARE the source-of-truth, not consumers). Composes with §11.4.17.

**Cascade requirement:** This anchor (verbatim or by `CONST-059` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.35 for the full mandate.
