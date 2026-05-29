# HelixCode CLI Reference

## Overview

HelixCode provides a comprehensive command-line interface (CLI) for managing all aspects of the enterprise AI development platform. The CLI includes tools for server management, configuration, performance optimization, security scanning, and distributed computing.

## Main CLI Commands

### helix

The main HelixCode CLI command with comprehensive AI development platform management.

```bash
helix [command] [flags]
```

#### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | "" | Configuration file path |
| `--debug` | bool | false | Enable debug logging |
| `--log-level` | string | "info" | Log level (debug, info, warn, error) |

#### Commands

##### `helix start`

Start HelixCode with automated local LLM management.

```bash
helix start [flags]
```

**Flags:**
- `--auto`: Enable full automation (default: true)
- `--monitor`: Enable health monitoring (default: true)
- `--optimize`: Enable performance optimization (default: true)
- `--check-interval`: Health check interval (default: 30s)

**Description:**
Starts HelixCode with fully automated local LLM provider management. Automatically installs, configures, and manages 11+ local LLM providers with zero-touch operation.

**Example:**
```bash
# Start with default automation
helix start

# Start with custom check interval
helix start --check-interval=60s

# Start without optimization
helix start --optimize=false
```

##### `helix auto`

Fully automated local LLM management mode.

```bash
helix auto
```

**Description:**
Starts HelixCode in fully automated mode where all local LLM providers are automatically cloned, installed, configured, started as background services, and maintained without user intervention.

##### `helix server`

Start the HelixCode server.

```bash
helix server
```

**Description:**
Starts the HelixCode HTTP server with all configured providers and services on the default port (8080).

##### `helix version`

Show version information.

```bash
helix version
```

**Output:**
```
HelixCode Enterprise AI Development Platform
Version: 1.0.0
Build: 2025.01.20
AI Providers: 29 (18 cloud + 11 local)
Token Context: 2M
License: MIT
```

##### `helix generate`

Generate code/text with AI.

```bash
helix generate [prompt] [flags]
```

**Flags:**
- `--model`: LLM model to use (default: "llama-3-8b")
- `--max-tokens`: Maximum tokens to generate (default: 1000)
- `--temperature`: Generation temperature (default: 0.7)
- `--stream`: Stream the response (default: false)

**Example:**
```bash
# Generate code with default settings
helix generate "Write a Go function to reverse a string"

# Generate with specific model and settings
helix generate "Create a REST API in Python" --model=gpt-4 --max-tokens=2000 --temperature=0.5

# Stream the response
helix generate "Explain quantum computing" --stream
```

##### `helix test`

Run HelixCode test suite.

```bash
helix test [flags]
```

**Description:**
Runs comprehensive test suite including unit, integration, and E2E tests.

##### `helix worker`

Manage distributed workers.

```bash
helix worker [subcommand] [flags]
```

**Subcommands:**
- `add <host>`: Add a new worker
- `list`: List all workers
- `status`: Show worker status
- `remove <id>`: Remove a worker

**Examples:**
```bash
# List all workers
helix worker list

# Add a new worker
helix worker add worker1.example.com --user=helix --key=/path/to/key

# Show worker status
helix worker status

# Remove a worker
helix worker remove worker-uuid-123
```

##### `helix notify`

Send notifications.

```bash
helix notify [message] [flags]
```

**Flags:**
- `--type`: Notification type (info/warning/error/success/alert) (default: "info")
- `--priority`: Notification priority (low/medium/high/critical) (default: "medium")

**Examples:**
```bash
# Send info notification
helix notify "System started successfully"

# Send error notification
helix notify "Database connection failed" --type=error --priority=high

# Send alert
helix notify "High CPU usage detected" --type=alert --priority=critical
```

## Configuration Management CLI

### helix-config

Comprehensive configuration management CLI for HelixCode.

```bash
helix-config [command] [flags]
```

#### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | string | "" | Configuration file path |
| `-f, --format` | string | "auto" | Configuration format (json, yaml, toml) |
| `-o, --output` | string | "json" | Output format (json, yaml, table, pretty) |
| `--session-id` | string | "" | Session ID for tracking |
| `--user` | string | "" | User name for audit |
| `-v, --verbose` | bool | false | Verbose output |
| `--dry-run` | bool | false | Dry run without making changes |
| `-q, --quiet` | bool | false | Quiet mode (no output) |
| `--no-color` | bool | false | Disable color output |
| `-i, --interactive` | bool | false | Interactive mode |
| `-F, --force` | bool | false | Force operation without confirmation |
| `--backup` | bool | true | Create backup before making changes |
| `--timeout` | duration | 30s | Operation timeout |
| `--max-retries` | int | 3 | Maximum number of retries |
| `--show-secrets` | bool | false | Show sensitive configuration values |
| `--no-validate` | bool | false | Skip configuration validation |
| `--strict` | bool | false | Enable strict validation mode |
| `--pretty` | bool | true | Pretty print JSON output |
| `--sort-keys` | bool | true | Sort object keys in output |

#### Commands

##### `helix-config show`

Display current configuration.

```bash
helix-config show [flags]
```

**Flags:**
- `--section`: Show specific configuration section
- `--filter`: Filter configuration keys
- `--depth`: Maximum display depth

**Examples:**
```bash
# Show all configuration
helix-config show

# Show server configuration
helix-config show --section=server

# Show database configuration with filter
helix-config show --section=database --filter="host|port"

# Show in YAML format
helix-config show -o yaml
```

##### `helix-config get`

Get configuration value.

```bash
helix-config get <key> [flags]
```

**Examples:**
```bash
# Get server port
helix-config get server.port

# Get database host
helix-config get database.host

# Get LLM provider API key (with secrets shown)
helix-config get llm.providers.openai.api_key --show-secrets
```

##### `helix-config set`

Set configuration value.

```bash
helix-config set <key> <value> [flags]
```

**Examples:**
```bash
# Set server port
helix-config set server.port 9090

# Set database host
helix-config set database.host postgres.example.com

# Set LLM API key
helix-config set llm.providers.openai.api_key sk-your-api-key-here

# Dry run
helix-config set server.port 9090 --dry-run
```

##### `helix-config delete`

Delete configuration key.

```bash
helix-config delete <key> [flags]
```

**Examples:**
```bash
# Delete a configuration key
helix-config delete custom.setting

# Force delete without confirmation
helix-config delete custom.setting --force
```

##### `helix-config validate`

Validate configuration.

```bash
helix-config validate [flags]
```

**Flags:**
- `--schema`: Schema file for validation
- `--strict`: Enable strict validation

**Examples:**
```bash
# Validate current configuration
helix-config validate

# Validate with custom schema
helix-config validate --schema=custom-schema.json

# Strict validation
helix-config validate --strict
```

##### `helix-config export`

Export configuration.

```bash
helix-config export [filename] [flags]
```

**Examples:**
```bash
# Export to stdout
helix-config export

# Export to file
helix-config export config-backup.yaml -o yaml

# Export specific section
helix-config export server-config.json --section=server
```

##### `helix-config import`

Import configuration.

```bash
helix-config import <filename> [flags]
```

**Flags:**
- `-F, --force`: Force import even with validation errors
- `--conflict`: Conflict resolution (first, second, error)

**Examples:**
```bash
# Import configuration
helix-config import new-config.yaml

# Force import with conflicts resolved to 'first'
helix-config import new-config.yaml --conflict=first

# Dry run import
helix-config import new-config.yaml --dry-run
```

##### `helix-config backup`

Create configuration backup.

```bash
helix-config backup [filename] [flags]
```

**Examples:**
```bash
# Create timestamped backup
helix-config backup

# Create named backup
helix-config backup my-backup-2024.yaml
```

##### `helix-config restore`

Restore configuration from backup.

```bash
helix-config restore <filename> [flags]
```

**Examples:**
```bash
# Restore from backup
helix-config restore config-backup-2024-01-20.yaml

# Dry run restore
helix-config restore config-backup.yaml --dry-run
```

##### `helix-config reset`

Reset configuration to defaults.

```bash
helix-config reset [flags]
```

**Examples:**
```bash
# Reset all configuration
helix-config reset

# Reset specific section
helix-config reset --section=database
```

##### `helix-config reload`

Reload configuration from disk.

```bash
helix-config reload [flags]
```

**Examples:**
```bash
# Reload configuration
helix-config reload

# Reload with validation
helix-config reload --validate
```

##### `helix-config watch`

Watch configuration changes.

```bash
helix-config watch [flags]
```

**Flags:**
- `--interval`: Watch interval (default: 5s)
- `--command`: Command to run on changes

**Examples:**
```bash
# Watch for configuration changes
helix-config watch

# Watch and run command on changes
helix-config watch --command="systemctl restart helixcode"

# Watch with custom interval
helix-config watch --interval=10s
```

##### `helix-config migrate`

Migrate configuration between versions.

```bash
helix-config migrate [flags]
```

**Flags:**
- `--from`: Source version
- `--to`: Target version
- `--backup`: Create backup before migration

**Examples:**
```bash
# Migrate to latest version
helix-config migrate

# Migrate from specific version
helix-config migrate --from=1.0.0 --to=1.1.0
```

##### `helix-config benchmark`

Benchmark configuration operations.

```bash
helix-config benchmark [flags]
```

**Examples:**
```bash
# Run configuration benchmarks
helix-config benchmark

# Benchmark with verbose output
helix-config benchmark --verbose
```

##### `helix-config template`

Manage configuration templates.

```bash
helix-config template [subcommand] [flags]
```

**Subcommands:**
- `list`: List available templates
- `apply <template>`: Apply configuration template
- `create <name>`: Create new template
- `delete <name>`: Delete template

**Examples:**
```bash
# List templates
helix-config template list

# Apply production template
helix-config template apply production

# Create custom template
helix-config template create my-template
```

##### `helix-config history`

View configuration change history.

```bash
helix-config history [flags]
```

**Flags:**
- `--limit`: Maximum number of entries (default: 50)
- `--since`: Show changes since date
- `--user`: Filter by user

**Examples:**
```bash
# Show recent changes
helix-config history

# Show changes by specific user
helix-config history --user=admin

# Show changes since date
helix-config history --since=2024-01-01
```

##### `helix-config schema`

Manage configuration schemas.

```bash
helix-config schema [subcommand] [flags]
```

**Subcommands:**
- `show`: Show current schema
- `validate`: Validate against schema
- `update`: Update schema

**Examples:**
```bash
# Show current schema
helix-config schema show

# Validate configuration against schema
helix-config schema validate

# Update schema
helix-config schema update
```

##### `helix-config completion`

Generate shell completion scripts.

```bash
helix-config completion [shell]
```

**Supported shells:** bash, zsh, fish, powershell

**Examples:**
```bash
# Generate bash completion
helix-config completion bash > /etc/bash_completion.d/helix-config

# Generate zsh completion
helix-config completion zsh > /usr/local/share/zsh/site-functions/_helix-config
```

##### `helix-config version`

Show version information.

```bash
helix-config version
```

## Standalone CLI Tools

### Performance Optimization CLI

```bash
./bin/helixcode-performance-optimization [flags]
```

**Flags:**
- `--config`: Configuration file path
- `--baseline`: Create performance baseline
- `--optimize`: Run optimization
- `--report`: Generate report
- `--continuous`: Continuous monitoring mode

**Examples:**
```bash
# Create performance baseline
./bin/helixcode-performance-optimization --baseline

# Run optimization
./bin/helixcode-performance-optimization --optimize

# Generate performance report
./bin/helixcode-performance-optimization --report
```

### Security Fix CLI

```bash
./bin/helixcode-security-fix [flags]
```

**Flags:**
- `--scan`: Run security scan
- `--fix`: Apply security fixes
- `--report`: Generate security report
- `--auto`: Automatic mode

**Examples:**
```bash
# Run security scan
./bin/helixcode-security-fix --scan

# Apply automatic fixes
./bin/helixcode-security-fix --fix --auto

# Generate security report
./bin/helixcode-security-fix --report
```

### Local LLM CLI

```bash
./bin/helixcode-local-llm [command] [flags]
```

**Commands:**
- `discover`: Discover available local models
- `recommend`: Get model recommendations
- `analytics`: Show usage analytics
- `report`: Generate performance report
- `insights`: Get usage insights

**Examples:**
```bash
# Discover local models
./bin/helixcode-local-llm discover

# Get model recommendations
./bin/helixcode-local-llm recommend --use-case=code-generation

# Show analytics
./bin/helixcode-local-llm analytics

# Generate insights report
./bin/helixcode-local-llm insights --period=7d
```

## Docker CLI Commands

### Docker Compose Commands

```bash
# Start all services
docker-compose -f docker/docker-compose.yml up -d

# Start with security scanning
docker-compose -f scripts/security_scan/docker-compose.security.yml up -d

# View service status
docker-compose ps

# View logs
docker-compose logs -f helixcode

# Stop all services
docker-compose down

# Rebuild and restart
docker-compose up -d --build
```

### Security / performance / config tooling

These tools are NOT distributed as the `helixcode-*` Docker images previously listed
here (no such images exist; §11.4.99 correction 2026-05-29) and direct `docker` runs are
not a supported workflow (Rule 4). Use the real entry points:

```bash
# Run security scanners (root Makefile)
make scan-all          # all scanners; or scan-gosec / scan-trivy / scan-secrets

# Run performance optimization (inner Go cmd)
cd helix_code && go run ./cmd/performance_optimization

# Run configuration validation (inner Go cmd)
cd helix_code && go run ./cmd/config_test
```

## Script Commands

### Test Scripts

```bash
# Run all tests
./scripts/run-tests.sh

# Run unit tests only
./scripts/run-tests.sh --unit

# Run with coverage
./scripts/run-tests.sh --coverage

# Run integration tests
./scripts/run-tests.sh --integration
```

### Build Scripts

```bash
# Build all components
make build

# Build with assets
make build-assets

# Build for production
make prod

# Build mobile apps
make mobile
```

### Utility Scripts

```bash
# Setup development environment
./scripts/setup-dev.sh

# Clean build artifacts
./scripts/clean.sh

# Generate documentation
./scripts/generate-docs.sh

# Run linting
./scripts/lint.sh
```

## Configuration Examples

### Basic Configuration

```yaml
# config/config.yaml
server:
  address: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  user: "helix"
  dbname: "helixcode_prod"

redis:
  host: "localhost"
  port: 6379

llm:
  default_provider: "local"
  providers:
    local:
      enabled: true
    openai:
      api_key: "${OPENAI_API_KEY}"
```

### Production Configuration

```yaml
# Production configuration
server:
  address: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/helixcode.crt"
    key_file: "/etc/ssl/private/helixcode.key"

database:
  host: "postgres"
  port: 5432
  user: "helix"
  password: "${HELIX_DATABASE_PASSWORD}"
  sslmode: "require"
  max_connections: 50

redis:
  host: "redis"
  port: 6379
  password: "${HELIX_REDIS_PASSWORD}"

performance:
  cpu_optimization: true
  memory_optimization: true
  garbage_collection: true
  target_throughput: 10000
  target_latency: "50ms"

security:
  scanning:
    enabled: true
    zero_tolerance: true
  headers:
    enabled: true
    csp: "default-src 'self'"
    hsts: true
```

### Development Configuration

```yaml
# Development configuration
server:
  address: "127.0.0.1"
  port: 8080

database:
  host: ""  # Disabled for testing

redis:
  host: ""  # Disabled for testing

logging:
  level: "debug"
  format: "text"

performance:
  cpu_optimization: false
  memory_optimization: false
  garbage_collection: false
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Network error |
| 4 | Authentication error |
| 5 | Permission denied |
| 6 | Resource not found |
| 7 | Validation error |
| 8 | Timeout error |
| 9 | Security violation |

## Environment Variables

### Required Variables

```bash
# Authentication
HELIX_AUTH_JWT_SECRET=your-super-secure-jwt-secret

# Database
HELIX_DATABASE_PASSWORD=your-secure-database-password

# Redis
HELIX_REDIS_PASSWORD=your-secure-redis-password
```

### Optional Variables

```bash
# LLM Providers
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key
GEMINI_API_KEY=your-gemini-key

# Notifications
HELIX_SLACK_WEBHOOK_URL=https://hooks.slack.com/...
HELIX_EMAIL_SMTP_SERVER=smtp.gmail.com

# Security
HELIX_SECURITY_SCAN_INTERVAL=24h
HELIX_SECURITY_ZERO_TOLERANCE=true

# Performance
HELIX_PERFORMANCE_OPTIMIZATION_INTERVAL=1h
HELIX_PERFORMANCE_TARGET_THROUGHPUT=10000
```

## Command Chaining

### Pipeline Usage

```bash
# Chain configuration commands
helix-config get server.port | xargs helix-config set server.port

# Use with other tools
helix-config export | jq '.server.port' | xargs echo "Port is:"
```

### Scripting Examples

```bash
#!/bin/bash
# Backup and update configuration
helix-config backup pre-update-$(date +%Y%m%d)
helix-config set server.port 9090
helix-config validate
if [ $? -eq 0 ]; then
    echo "Configuration updated successfully"
    helix-config reload
else
    echo "Configuration validation failed"
    exit 1
fi
```

## Troubleshooting CLI Issues

### Common Problems

#### Command Not Found
```bash
# Check if binary exists
ls -la bin/helixcode

# Add to PATH
export PATH=$PATH:/path/to/helixcode/bin

# Check permissions
chmod +x bin/helixcode
```

#### Configuration Not Loaded
```bash
# Check configuration file
cat config/config.yaml

# Validate syntax
yamllint config/config.yaml

# Check file permissions
ls -la config/config.yaml
```

#### Connection Refused
```bash
# Check if server is running
curl http://localhost:8080/health

# Check port binding
netstat -tlnp | grep 8080

# Check firewall
iptables -L | grep 8080
```

#### Permission Denied
```bash
# Check user permissions
id

# Check file permissions
ls -la /etc/helixcode/

# Run with sudo if necessary
sudo helixcode server
```

### Debug Mode

```bash
# Enable debug logging
helixcode --debug server

# Verbose configuration output
helix-config --verbose show

# Dry run mode
helix-config set server.port 9090 --dry-run
```

### Log Analysis

```bash
# View recent logs
tail -f /var/log/helixcode/server.log

# Search for errors
grep "ERROR" /var/log/helixcode/*.log

# Count error types
grep "ERROR" /var/log/helixcode/*.log | cut -d' ' -f4 | sort | uniq -c
```

## CodeGraph integration (MCP code-graph server)

HelixCode incorporates **CodeGraph** (`@colbymchenry/codegraph`, MIT) as a
pinned, vendored third-party tool under `tools/codegraph/`. CodeGraph parses
the codebase into a local SQLite knowledge graph of symbols and edges and
serves it to CLI coding agents over the **Model Context Protocol (MCP)**, so
an agent queries the graph instantly instead of running grep/glob/Read scans.

It is not part of the `helix` binary — it is a developer-tooling layer wired
into five CLI agents (Claude Code, OpenCode, Kimi CLI, Crush, Qwen Code).

### Install + initialize + scan

```bash
# Install the pinned CodeGraph runtime (idempotent).
tools/codegraph/install.sh

# Initialize + scan (re-runnable any time).
tools/codegraph/node_modules/.bin/codegraph init -i        # repo root
cd helix_code && ../tools/codegraph/node_modules/.bin/codegraph init -i

# Inspect the graph — non-zero node/edge/file counts prove the scan worked.
tools/codegraph/node_modules/.bin/codegraph status .
tools/codegraph/node_modules/.bin/codegraph query Provider
```

### Per-agent registration

```bash
tools/codegraph/agents/register-claude.sh    # .mcp.json + mcp__codegraph__* permission
tools/codegraph/agents/register-opencode.sh  # opencode.jsonc
tools/codegraph/agents/register-kimi.sh      # ~/.kimi/mcp.json
tools/codegraph/agents/register-crush.sh     # .crush.json
tools/codegraph/agents/register-qwen.sh      # .qwen/settings.json
```

Each agent then exposes eight `codegraph_*` MCP tools (`codegraph_search`,
`codegraph_context`, `codegraph_callers`, `codegraph_callees`,
`codegraph_impact`, `codegraph_node`, `codegraph_files`, `codegraph_status`).

Full detail: `tools/codegraph/README.md` and the incorporation plan at
`docs/research/codegraph/incorporation-plan.md`.

---

This CLI reference provides comprehensive coverage of all HelixCode command-line tools and their usage patterns. For additional help, use `--help` flag with any command or refer to the specific documentation sections.