# HelixCode CLI

The HelixCode Command Line Interface (CLI) provides a powerful terminal-based client for interacting with HelixCode servers and managing distributed AI development workflows.

## Overview

The CLI client enables:
- Managing projects, tasks, and workers
- Executing development workflows (planning, building, testing, refactoring)
- Interacting with LLM providers
- Monitoring system status and metrics
- SSH-based worker management

## Installation

### From Source

```bash
cd HelixCode
make build
# Binary is at bin/helixcode
```

### Cross-Platform Builds

```bash
make prod
# Creates binaries for Linux, macOS, and Windows
```

## Configuration

The CLI reads configuration from multiple sources (in order of precedence):
1. Command-line flags
2. Environment variables (prefixed with `HELIX_`)
3. Config file locations:
   - `./config/config.yaml`
   - `./config.yaml`
   - `~/.config/helixcode/config.yaml`
   - `/etc/helixcode/config.yaml`

### Minimal Configuration

```yaml
server:
  address: "127.0.0.1"
  port: 8080

auth:
  jwt_secret: "your-secret-key"

logging:
  level: "info"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIX_SERVER_ADDRESS` | Server bind address | 127.0.0.1 |
| `HELIX_SERVER_PORT` | Server port | 8080 |
| `HELIX_AUTH_JWT_SECRET` | JWT signing secret | (required) |
| `HELIX_DATABASE_HOST` | PostgreSQL host | localhost |
| `HELIX_DATABASE_PASSWORD` | Database password | - |
| `HELIX_REDIS_HOST` | Redis host | localhost |

## Commands

### Server Management

```bash
# Start the server
helixcode server start

# Check server health
helixcode server health

# View server info
helixcode server info
```

### Project Management

```bash
# Create a new project
helixcode project create --name "my-project" --path /path/to/project

# List projects
helixcode project list

# Get project details
helixcode project get <project-id>
```

### Task Management

```bash
# Create a task
helixcode task create --name "Build feature" --type building --priority high

# List tasks
helixcode task list

# Update task status
helixcode task update <task-id> --status running

# View task details
helixcode task get <task-id>
```

### Worker Management

```bash
# List workers
helixcode worker list

# Add a worker via SSH
helixcode worker add --host worker.example.com --user deploy

# View worker details
helixcode worker get <worker-id>
```

### Workflow Execution

```bash
# Execute planning workflow
helixcode workflow planning --project <project-id> --requirements "Add user authentication"

# Execute building workflow
helixcode workflow building --project <project-id>

# Execute testing workflow
helixcode workflow testing --project <project-id>

# Execute refactoring workflow
helixcode workflow refactoring --project <project-id> --scope "auth module"
```

### LLM Provider Management

```bash
# List available LLM providers
helixcode llm providers

# List available models
helixcode llm models

# Test provider connectivity
helixcode llm test --provider ollama
```

## Usage Examples

### Complete Development Workflow

```bash
# 1. Create a project
helixcode project create --name "new-feature" --path ./projects/new-feature

# 2. Plan the implementation
helixcode workflow planning --project <project-id> \
  --requirements "Implement user profile page with avatar upload"

# 3. Build the feature
helixcode workflow building --project <project-id>

# 4. Run tests
helixcode workflow testing --project <project-id>

# 5. Refactor if needed
helixcode workflow refactoring --project <project-id> --scope "user profile"
```

### Distributed Worker Setup

```bash
# Add multiple workers
helixcode worker add --host worker1.local --user deploy --capabilities build,test
helixcode worker add --host worker2.local --user deploy --capabilities build,test,gpu

# Monitor workers
helixcode worker list --verbose
```

## Output Formats

The CLI supports multiple output formats:

```bash
# JSON output (default for scripting)
helixcode task list --output json

# Table output (human readable)
helixcode task list --output table

# YAML output
helixcode task list --output yaml
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Authentication error |
| 4 | Connection error |
| 5 | Resource not found |

## Troubleshooting

### Common Issues

**Connection refused**
```
Error: connection refused to localhost:8080
```
Solution: Ensure the HelixCode server is running.

**Authentication failed**
```
Error: invalid or expired token
```
Solution: Re-authenticate or check JWT secret configuration.

**Database connection error**
```
Error: failed to connect to database
```
Solution: Verify database credentials and connectivity.

### Debug Mode

Enable verbose logging:
```bash
helixcode --log-level debug <command>
```

### Configuration Validation

Validate your configuration:
```bash
helixcode config validate
```

## Related Documentation

- [Server Documentation](../server/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
- [Configuration Guide](../../docs/CONFIGURATION.md)
- [CLI Reference](../../docs/COMPLETE_CLI_REFERENCE.md)
