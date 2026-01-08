# HelixCode Aurora OS Client

A native client for Aurora OS (Open Mobile Platform), providing HelixCode development workflow capabilities on Aurora OS devices.

## Overview

Aurora OS (formerly Sailfish OS) is a mobile operating system developed by Open Mobile Platform. This client enables:
- Project management on Aurora OS devices
- Session management with multiple workflow modes
- Task monitoring and creation
- LLM integration for AI-powered development
- Worker status viewing and management
- Aurora OS-specific system diagnostics and optimization
- Security management with audit logging
- Push notifications for task updates

## Dependencies

### Go Dependencies

The Aurora OS client uses the following Go packages:

**Core Dependencies:**
- `fyne.io/fyne/v2` - Cross-platform GUI toolkit (v2.4+)
- `dev.helix.code/internal/*` - HelixCode internal packages

**Internal Package Dependencies:**
- `internal/config` - Configuration management (Viper-based)
- `internal/database` - PostgreSQL persistence
- `internal/redis` - Redis caching
- `internal/llm` - Multi-provider LLM integration
- `internal/project` - Project lifecycle management
- `internal/session` - Session management
- `internal/task` - Task management with checkpointing
- `internal/worker` - SSH-based worker pool management
- `internal/notification` - Multi-channel notifications
- `internal/server` - HTTP server and API handlers

### System Requirements

- Aurora OS SDK 4.0+
- Aurora OS device or emulator
- Go 1.24+ cross-compilation toolchain
- 512MB RAM minimum
- 100MB storage
- Network connectivity

## Build Options

The Aurora OS client supports two build modes controlled by Go build tags:

### GUI Mode (Default)

Builds the full graphical application using Fyne:

```bash
cd HelixCode

# Build for Aurora OS (ARM64) with GUI
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
  go build -ldflags="-s -w" \
  -o bin/aurora-os/helixcode-aurora \
  ./applications/aurora-os

# Build for current platform (development)
go build -o bin/helixcode-aurora ./applications/aurora-os

# Build using Makefile
make aurora-os
```

### CLI Mode (nogui)

Builds a lightweight command-line interface without GUI dependencies:

```bash
cd HelixCode

# Build CLI-only version for Aurora OS (ARM64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags nogui -ldflags="-s -w" \
  -o bin/aurora-os/helixcode-aurora-cli \
  ./applications/aurora-os

# Build CLI-only version for current platform
go build -tags nogui -o bin/helixcode-aurora-cli ./applications/aurora-os
```

The CLI mode is useful for:
- Headless servers or embedded devices
- SSH-based remote access
- Scripting and automation
- Reduced binary size and memory footprint

### Build Tag Reference

| Tag | Description | Dependencies |
|-----|-------------|--------------|
| (none) | Full GUI application | Fyne v2, OpenGL |
| `nogui` | CLI-only application | None (CGO_ENABLED=0 compatible) |

## Building

### Setup Aurora OS SDK

1. Download Aurora OS SDK from [Open Mobile Platform](https://developer.auroraos.ru/)
2. Install the SDK following official instructions
3. Set up the build environment

### Cross-Compile Go Binary

```bash
cd HelixCode

# Build for Aurora OS (ARM64) - GUI version
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
  go build -ldflags="-s -w" \
  -o bin/aurora-os/helixcode-aurora \
  ./applications/aurora-os

# Build for Aurora OS (ARM64) - CLI version (no GUI dependencies)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags nogui -ldflags="-s -w" \
  -o bin/aurora-os/helixcode-aurora-cli \
  ./applications/aurora-os

# Build using Makefile
make aurora-os
```

### Package for Aurora OS

```bash
# Create RPM package
cd applications/aurora-os
rpmbuild -ba helixcode.spec
```

## Installation

### From RPM Package

```bash
# Install on device via SSH
scp helixcode-aurora-1.0.0.arm64.rpm nemo@device:/tmp/
ssh nemo@device "pkcon install-local /tmp/helixcode-aurora-1.0.0.arm64.rpm"
```

### From Aurora App Store

(Coming soon)

## Features

### Aurora Dashboard

- Real-time system statistics (CPU, memory, disk)
- Worker pool overview
- Task statistics
- Quick action buttons
- Activity log

### Project Management

- List and browse projects
- Create new projects with type selection (Go, Node, Python, Rust, Generic)
- View project details (path, build/test commands, metadata)
- Set active project

### Session Management

- Create development sessions with different modes:
  - Planning, Building, Testing, Refactoring, Debugging, Deployment
- Start, pause, resume, and complete sessions
- View session duration and status
- Track session history

### Task Dashboard

- View task list with status and priority
- Create new tasks with type and priority selection
- Task types: Planning, Building, Testing, Refactoring, Debugging
- Priority levels: Low, Normal, High, Critical

### LLM Integration

- View available AI models
- Model details (provider, context size, capabilities)
- Chat interface with provider selection
- Provider health monitoring
- Support for: Ollama, OpenAI, Anthropic, Gemini, Local providers

### Worker Monitor

- View worker status and health
- Add new SSH workers
- Worker configuration (host, port, user)
- Health indicators

### Aurora OS System Features

- **System Diagnostics**: Comprehensive system health checks
  - CPU/memory/disk status
  - Database connectivity
  - Component initialization status
  - Performance mode status

- **Performance Optimization**:
  - Toggle performance mode
  - Garbage collection
  - GOMAXPROCS optimization
  - Memory management

- **Security Management**:
  - Encryption configuration (AES-256-GCM, AES-256-CBC, ChaCha20-Poly1305)
  - Access control roles (admin, developer, viewer)
  - Security scanning
  - Comprehensive audit logging

### CLI Mode Commands

When built with `-tags nogui`, the following commands are available:

```bash
# Show help
helix-aurora help

# Show system status
helix-aurora status

# Project management
helix-aurora projects list
helix-aurora projects create --name "MyProject" --path "/path/to/project" --type go
helix-aurora projects set-active <project-id>
helix-aurora projects delete <project-id>

# Session management
helix-aurora sessions list
helix-aurora sessions create --name "Dev Session" --project <id> --mode building
helix-aurora sessions start <session-id>
helix-aurora sessions pause <session-id>
helix-aurora sessions complete <session-id>

# Task management
helix-aurora tasks list
helix-aurora tasks create --type building --desc "Build the project" --priority high
helix-aurora tasks cancel <task-id>

# Worker management
helix-aurora workers list
helix-aurora workers add --host 192.168.1.100 --port 22 --user deploy
helix-aurora workers remove <worker-id>

# LLM operations
helix-aurora llm providers
helix-aurora llm models

# Aurora OS specific
helix-aurora aurora info
helix-aurora aurora diagnostics
helix-aurora aurora performance
helix-aurora aurora optimize

# Security
helix-aurora security status
helix-aurora security audit
helix-aurora security encryption enable
helix-aurora security access list

# Interactive mode
helix-aurora interactive
```

## Configuration

### Server Connection

On first launch, configure the server connection:
1. Open Settings
2. Enter server URL
3. Enter credentials or API token
4. Test connection
5. Save

### Configuration File

Configuration is stored in:
```
~/.config/helixcode-aurora/config.json
```

```json
{
  "server": {
    "url": "https://helixcode.example.com",
    "token": "your-jwt-token"
  },
  "sync": {
    "interval": 30,
    "notifications": true
  },
  "display": {
    "theme": "aurora"
  },
  "aurora": {
    "performance_mode": false,
    "encryption_enabled": true,
    "encryption_algorithm": "AES-256-GCM"
  }
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIX_THEME` | UI theme (dark, light, helix, aurora) | aurora |
| `HELIX_LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `HELIX_DATABASE_HOST` | PostgreSQL host | localhost |
| `HELIX_DATABASE_PORT` | PostgreSQL port | 5432 |
| `HELIX_REDIS_HOST` | Redis host | localhost |
| `HELIX_AUTH_JWT_SECRET` | JWT secret for authentication | - |

## UI Components

### Available Themes

- **Aurora** (default) - Dark theme optimized for Aurora OS
- **Dark** - Standard dark theme
- **Light** - Light theme
- **Helix** - HelixCode brand theme

### Pages

1. **Aurora Dashboard** - System overview and quick actions
2. **Tasks** - Task management
3. **Workers** - Worker monitoring
4. **Aurora System** - System resources and native services
5. **Security** - Security management and audit log
6. **Projects** - Project browser and creation
7. **Sessions** - Session management
8. **LLM** - AI model interaction
9. **Settings** - Theme, server, database, LLM configuration

## Development

### Project Structure

```
applications/aurora-os/
├── main.go           # GUI entry point (build tag: !nogui)
├── main_nogui.go     # CLI entry point (build tag: nogui)
├── theme.go          # Theme definitions (build tag: !nogui)
├── main_test.go      # Tests
├── README.md         # This file
├── icons/            # App icons
└── helixcode.spec    # RPM spec file
```

### Building for Development

```bash
# Build GUI version for local development
go build -o bin/helixcode-aurora ./applications/aurora-os

# Build CLI version for local development
go build -tags nogui -o bin/helixcode-aurora-cli ./applications/aurora-os

# Build and deploy to emulator
aurora-sb2 -t AuroraOS-4.0.2.x86_64 go build ./applications/aurora-os
```

### Running Tests

```bash
cd HelixCode
go test -v ./applications/aurora-os/...
```

### Debugging

Enable debug logging:
```bash
HELIX_LOG_LEVEL=debug ./helixcode-aurora
```

## Known Limitations

- Camera integration not available (Aurora OS limitation)
- Background sync limited by Aurora OS power management
- Some UI animations may be limited on older devices
- LLM chat requires running provider (Ollama, OpenAI, etc.)
- Database/Redis optional but recommended for persistence

## Troubleshooting

### Connection Issues

1. Verify network connectivity
2. Check server URL (HTTPS required for production)
3. Verify SSL certificate is valid
4. Check firewall settings

### Performance

For better performance on older devices:
- Enable performance mode in Aurora Settings
- Reduce sync interval
- Use CLI mode for resource-constrained environments
- Limit cached data

### Installation Failures

1. Check available storage space
2. Verify RPM signature
3. Check Aurora OS version compatibility

### Database/Redis Not Available

The application can run without database or Redis:
- Data will be stored in memory only
- Sessions/projects won't persist across restarts
- Suitable for standalone usage

## Related Documentation

- [Aurora OS SDK Documentation](https://developer.auroraos.ru/doc)
- [Server Documentation](../../cmd/server/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
- [Desktop Application](../desktop/README.md) - Similar patterns for desktop
