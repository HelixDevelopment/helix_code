# HelixCode - Distributed AI Development Platform

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
No feature/functionality/flow/use-case/edge-case/service/application on any supported platform of HelixCode is deliverable until covered by automation tests proving six invariants: anti-bluff posture, proof of working capability end-to-end, working implementation matching documented promise, no open issues/bugs, full documentation in sync, four-layer test floor. See constitution submodule `Constitution.md` §11.4.25 for the full mandate.

### CONST-049 — Constitution-Submodule Update Workflow Mandate (cascaded from constitution submodule §11.4.26)
Before any modification to `constitution/{Constitution,CLAUDE,AGENTS}.md`: fetch+pull first → apply with §11.4.17 classification → validate → commit+push to EVERY upstream → careful conflict resolution (no force-push) → cascade verification (CONST-047) → bump `.gitmodules` pointer in SAME commit. See constitution submodule `Constitution.md` §11.4.26 for the full mandate.

### CONST-050 — No-Fakes-Beyond-Unit-Tests + 100%-Test-Type-Coverage Mandate (cascaded from constitution submodule §11.4.27)
**(A)** Mocks/stubs/fakes/placeholders/TODOs/FIXMEs/"for now" patterns PERMITTED only in unit-test sources; non-unit tests MUST exercise the real, fully implemented system. Production code MUST NOT import mock paths. **(B)** 100% test-type coverage: unit + integration + E2E + full-automation + security + DDoS + scaling + chaos + stress + performance + benchmarking + UI + UX + Challenges (`./Challenges/`) + HelixQA (`./HelixQA/`, with full autonomous QA sessions). See constitution submodule `Constitution.md` §11.4.27 for the full mandate.

### CONST-051 — Submodules-As-Equal-Codebase + Decoupling + Dependency-Layout Mandate (cascaded from constitution submodule §11.4.28)
**(A)** Every owned-by-us submodule (orgs: vasic-digital, HelixDevelopment, red-elf, ATMOSphere1234321, Bear-Suite, BoatOS123456, Helix-Flow, Helix-Track, Server-Factory — dynamically discoverable via gh/glab) is an EQUAL part of HelixCode's codebase. Same engineering attention as main (analysis, tests, gap-fill, bug-fix, docs/diagrams/SQL/website materials). **(B)** Submodules MUST stay fully decoupled — NEVER inject HelixCode-specific context; use configuration injection when needed. **(C)** Dependencies of owned submodules MUST live at HelixCode root (`<root>/<name>/` or `<root>/submodules/<name>/`); nested own-org submodule chains FORBIDDEN. Third-party submodules exempt. See constitution submodule `Constitution.md` §11.4.28 for the full mandate.

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform that enables intelligent task division, work preservation, and cross-platform development workflows. Built with Go and designed for scalability, HelixCode provides a robust foundation for distributed computing with automatic checkpointing, rollback functionality, and real-time monitoring.

The project is fully completed with 5 implementation phases:
- **Phase 1**: Foundation (Database schema, authentication, worker management)
- **Phase 2**: Core Services (Task division, LLM integration, MCP protocol)
- **Phase 3**: Workflows (Project management, development workflows)
- **Phase 4**: LLM Integration (Hardware detection, model management, CLI)
- **Phase 5**: Advanced Features (SSH worker pool, advanced LLM tooling)

Key technologies include Go 1.24+, PostgreSQL, Redis, Gin framework, and multiple LLM providers integration (Llama.cpp, Ollama, OpenAI).

## Architecture

The platform consists of:
- **API Layer**: REST + WebSocket + MCP
- **Core Services**: Authentication, worker management, task management, LLM providers
- **Database Layer**: PostgreSQL + Redis
- **Distributed Workers**: Cross-platform support
- **Multi-Client Interfaces**: CLI, TUI, REST, Mobile

## Building and Running

### Prerequisites
- Go 1.24.0+
- PostgreSQL 15+
- Redis 7+ (optional)

### Build Commands
```bash
cd HelixCode

# Setup dependencies
make setup-deps

# Generate logo assets
make logo-assets

# Build the application
make build

# Run all tests
make test

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean

# Build for production with cross-platform support
make prod

# Run development server
make dev
```

### Manual Build and Execution
```bash
# Build server
go build -o bin/helixcode ./cmd/server

# Build CLI
go build -o bin/helixcode-cli ./cmd/cli

# Run server
./bin/helixcode

# Run with specific config
./bin/helixcode --config config/prod/config.yaml
```

### Environment Variables
The application uses environment variables with `HELIX_` prefix:
- `HELIX_DATABASE_PASSWORD` - Database password
- `HELIX_REDIS_PASSWORD` - Redis password
- `HELIX_AUTH_JWT_SECRET` - JWT secret
- `HELIX_CONFIG` - Custom config file path

### CLI Usage
```bash
# Interactive mode
./bin/helixcode-cli

# List workers
./bin/helixcode-cli --list-workers

# Add a worker
./bin/helixcode-cli --worker worker-host --user helix --key ~/.ssh/id_rsa

# Generate with LLM
./bin/helixcode-cli --prompt "Hello world" --model llama-3-8b

# Health check
./bin/helixcode-cli --health
```

## Development Conventions

### Go Code Structure
The codebase follows Go best practices with a well-organized internal structure:

- `cmd/` - Main applications (server and CLI)
- `internal/` - Private application code organized by domain:
  - `auth/` - Authentication system
  - `config/` - Configuration management
  - `database/` - Database layer
  - `hardware/` - Hardware detection
  - `llm/` - LLM providers and reasoning
  - `logo/` - Logo processing & assets
  - `mcp/` - MCP protocol implementation
  - `notification/` - Multi-channel notifications
  - `project/` - Project management
  - `redis/` - Redis utilities
  - `server/` - HTTP server & API
  - `session/` - Session management
  - `task/` - Task management & checkpoints
  - `theme/` - Theme management
  - `worker/` - Worker pool management
  - `workflow/` - Workflow execution
- `pkg/` - Shared libraries (public)
- `shared/` - Shared code for mobile bindings
- `scripts/` - Build and utility scripts
- `test/` - Test-specific code

### Code Style
- Go idiomatic code with clear function and variable names
- Structured configuration using Viper for environment and file configuration
- Proper error handling with descriptive error messages
- Comprehensive logging with structured logging approach
- Dependency injection for better testability

### Testing Practices
- Test files follow Go convention with `_test.go` suffix
- Uses `testify` package for assertions and require functions
- Comprehensive test coverage with unit, integration, and end-to-end tests
- Test helpers for temporary directories and environment setup
- Table-driven tests for multiple scenarios
- Mock implementations where needed for testing

### Configuration Management
- Centralized configuration using Viper
- Environment variable support with `HELIX_` prefix
- YAML configuration files with validation
- Default values for all configuration options
- Secure defaults (e.g., requires non-default JWT secret)

### Database Schema
- PostgreSQL database with 11 core tables:
  - `users`: User accounts and authentication
  - `workers`: Distributed worker nodes with SSH config
  - `tasks`: Task management with checkpoints and dependencies
  - `projects`: Project lifecycle management
  - `sessions`: Development sessions and context
  - `llm_providers`: Configured LLM provider instances
  - `notifications`: Multi-channel notification management

### API Endpoints
- REST API with versioning (v1)
- Standard HTTP status codes
- JSON request/response format
- Authentication via JWT tokens
- Comprehensive error responses with error codes

## Mobile Support
- iOS framework generation using gomobile
- Android AAR generation using gomobile
- Shared code in `pkg/mobile-core` for cross-platform functionality

## OS Support
- Linux, macOS, Windows
- Aurora OS and Symphony OS clients
- Cross-platform SSH worker management

## Documentation Files
- Architecture Overview
- Development Guide  
- User Guide
- API Documentation
- Phase implementation summaries (2, 4, 5)

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
