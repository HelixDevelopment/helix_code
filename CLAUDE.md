# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform built in Go. Key capabilities:
- SSH-based distributed worker pools with auto-management
- Multi-provider LLM integration (local: Ollama, Llama.cpp, vLLM; cloud: OpenAI, Anthropic, Gemini, xAI, Bedrock, Azure)
- Automated development workflows (planning, building, testing, refactoring, debugging)
- Task management with checkpointing and dependency tracking
- MCP (Model Context Protocol) with stdio/SSE transports
- Multi-client architecture: REST API, CLI, TUI, Desktop, WebSocket, mobile

## Essential Commands

**CRITICAL**: All commands must run from `HelixCode/` subdirectory, not the repository root.

```bash
cd HelixCode

# Build
make build                    # Build server to bin/helixcode
make prod                     # Cross-platform builds (Linux, macOS, Windows)

# Test
make test                     # Run all tests (go test -v ./...)
go test -v ./internal/auth    # Test single package
go test -v ./internal/auth -run TestSpecific  # Run single test
go test -cover ./...          # With coverage
make test-coverage            # Coverage analysis with report
make test-benchmark           # Run benchmarks
./run_tests.sh                # Unit tests only
./run_all_tests.sh            # All tests (unit + integration + e2e)

# Code quality
make fmt                      # Format with go fmt
make lint                     # Lint with golangci-lint

# Development
make clean                    # Clean bin/, dist/, coverage.out
make dev                      # Build and run development server
make setup-deps               # Download and tidy Go dependencies

# Mobile/Platform
make mobile                   # iOS + Android bindings
make aurora-os                # Aurora OS client
make harmony-os               # Harmony OS client
```

## Architecture Overview

### Repository Structure

```
/ (repository root)
├── HelixCode/              # Main Go application (go.mod is here)
│   ├── cmd/server/         # HTTP server entry point
│   ├── cmd/cli/            # CLI client entry point
│   ├── applications/       # Platform apps (terminal-ui, desktop, aurora-os, harmony-os)
│   ├── internal/           # Internal packages (40+ packages)
│   ├── shared/mobile-core/ # Mobile platform bindings
│   ├── config/             # Configuration files
│   └── tests/              # Integration and E2E tests
├── Example_Projects/       # Reference implementations
├── Dependencies/           # Git submodules (LLama_CPP, etc.)
├── Specification/          # Technical specifications
└── Implementation_Guide/   # Implementation guides
```

### Key Internal Packages

**Core Services** (`internal/`):
- `auth`: JWT authentication with session management
- `worker`: SSH-based distributed worker pool with auto-installation
- `task`: Task management with checkpointing, dependencies, priority queue
- `llm`: Multi-provider LLM integration with unified `Provider` interface
- `project`: Project lifecycle management
- `workflow`: Workflow execution engine with step DAG dependencies
- `server`: HTTP server, routing, API handlers
- `mcp`: Model Context Protocol (stdio + SSE transports)

**AI & Tools**:
- `agent`: Multi-agent orchestration and coordination
- `tools`: Tool ecosystem - filesystem, shell, web, browser automation, codebase mapping, multi-file editing (see `internal/tools/README.md`)
- `editor`: Multi-format code editing (Diff/Whole/Search-Replace/Line-based) optimized per LLM model (see `internal/editor/README.md`)
- `context`: Fluent API for AI conversation context building
- `memory`: Long-term memory (Mem0, Zep, Memonto integration)

**Infrastructure**:
- `database`: PostgreSQL persistence
- `redis`: Optional caching and real-time state
- `config`: Viper-based configuration with environment variable overrides
- `notification`: Multi-channel (Slack, Discord, Email, Telegram)

**Additional Packages**: `cognee`, `commands`, `deployment`, `discovery`, `event`, `focus`, `hardware`, `hooks`, `logging`, `monitoring`, `performance`, `persistence`, `provider`, `repomap`, `rules`, `security`, `session`, `template`

### Key Architecture Patterns

**Task Distribution**: `task.Manager` handles priority-based scheduling with:
- Types: planning, building, testing, refactoring, debugging
- Priority levels: low, normal, high, critical
- Automatic checkpointing for work preservation
- Dependency resolution between tasks

**Worker Management**: `worker.SSHWorkerPool` manages distributed workers:
- Auto-installs Helix CLI on remote machines via SSH
- Health monitoring (default 30s intervals)
- Resource tracking (CPU, memory, GPU)
- Capability-based task assignment

**LLM Provider Interface**: All providers implement unified `llm.Provider`:
```go
type Provider interface {
    Generate(ctx, *LLMRequest) (*LLMResponse, error)
    GenerateStream(ctx, *LLMRequest, chan<- LLMResponse) error
    GetModels() []ModelInfo
    IsAvailable(ctx) bool
}
```
- Selection strategies: performance, cost, availability, round-robin
- Automatic fallback when primary provider fails

**Workflow Engine**: DAG-based workflow execution with step types (analysis, generation, execution, validation) and actions (analyze_code, generate_code, run_tests).

## Configuration

Primary config: `HelixCode/config/config.yaml` (Viper-based with env var overrides).

**Config Files**:
- `config/config.yaml`: Production
- `config/test-config.yaml`: Testing (simplified)
- `config/minimal-config.yaml`: Minimal setup
- `config/working-config.yaml`: Development

**Config Search Order**: CLI flag → `./config/config.yaml` → `./config.yaml` → `~/.config/helixcode/config.yaml` → `/etc/helixcode/config.yaml`

**Critical Environment Variables** (override config):
```bash
HELIX_AUTH_JWT_SECRET        # Required for auth
HELIX_DATABASE_PASSWORD      # PostgreSQL password
HELIX_DATABASE_HOST          # Default: localhost
HELIX_DATABASE_PORT          # Default: 5432
HELIX_REDIS_PASSWORD         # If Redis enabled
```

**Database**: Optional for testing. Disable by setting `database.enabled: false` or leaving `host` empty.
**Redis**: Optional. Disable with `redis.enabled: false`.

## Testing

```bash
# Unit tests (alongside source: manager_test.go next to manager.go)
go test -v ./internal/auth

# All tests
./run_all_tests.sh

# Integration tests
./run_integration_tests.sh
```

**Test configurations**: Use `config/test-config.yaml` or `config/minimal-config.yaml`.

**Testing framework**: `github.com/stretchr/testify` for assertions; mock interfaces in `internal/mocks/`.

## Module Info

- **Module**: `dev.helix.code`
- **Go version**: 1.24.0 (toolchain go1.24.9)

**Key dependencies**: gin (HTTP), viper (config), pgx/pq (PostgreSQL), redis, jwt, websocket, chromedp (browser), testify, cobra (CLI), fyne (desktop UI), tview (TUI), tree-sitter (parsing).

## Important Notes

- **Nested repo**: Main Go code in `HelixCode/` subdirectory - always `cd HelixCode` first
- **Database auto-init**: Schema created on startup via `db.InitializeSchema()`
- **Task checkpointing**: Auto-checkpoint every 300s (configurable)
- **Worker health**: Checked every 30s; unhealthy workers removed
- **LLM fallback**: Enabled via `llm.selection.fallback_enabled`
- **Editor format**: Auto-selects best format (Diff/Whole/Search-Replace/Line) per LLM model
- **Tool security**: Path validation, command blocklists, resource limits, audit logging

## Package Documentation

Detailed READMEs for complex packages:
- `internal/editor/README.md`: Multi-format code editing (Diff/Whole/Search-Replace/Line) with 276+ tests
- `internal/tools/README.md`: Tool ecosystem (filesystem, shell, web, browser, mapping, multiedit)
- `internal/context/README.md`: Fluent API for building AI conversation context
- `internal/llm/README.md`: LLM provider integration and selection strategies
- `internal/llm/LOCAL_PROVIDERS.md`: Local provider setup (Ollama, Llama.cpp, vLLM)

## Challenge Testing Framework

E2E challenge tests in `tests/e2e/challenges/` validate HelixCode's ability to generate complete working projects.

```bash
cd tests/e2e/challenges
go run cmd/runner/main.go -list                              # List challenges
go run cmd/runner/main.go -challenge notes-project-001       # Run single challenge
```

See `tests/e2e/challenges/README.md` for full documentation on multi-provider testing, distributed workers, and creating new challenges.

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change. Coverage and passing suites measure the LLM's model of the product, not
the product.

1. **No self-certification.** *Verified, tested, working, complete, fixed, passing*
   are forbidden in commits, PRs, and agent replies without accompanying pasted
   output from a same-session real-system run.
2. **Demo before code.** Every task begins with the runnable acceptance demo below.
3. **Real system.** Demos run against real artifacts — built binaries, live
   databases, instrumented devices — not mocks/stubs/in-memory fakes.
4. **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `it.skip` without a trailing
   `SKIP-OK: #<ticket>` annotation fails `make ci-validate-all`.
5. **Contract tests on every seam.** Any change touching a module↔module boundary
   runs one roundtrip test asserting the wire format on both sides.
6. **Evidence in the PR.** PR body contains a fenced `## Demo` block with exact
   command(s) + output.

### Acceptance demo for this module

```bash
# TODO — replace with a 10-line real-system demo. See examples in
# HelixAgent/docs/development/dod-dropin/templates/CLAUDE_md_clause.md
```

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
