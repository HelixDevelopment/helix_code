# HelixCode Harmony OS Client

A native client for Huawei Harmony OS (HarmonyOS NEXT), providing HelixCode development workflow capabilities on Harmony OS devices.

## Overview

Harmony OS is Huawei's distributed operating system. This client enables:
- Project management on Harmony OS devices
- Task monitoring and creation with distributed scheduling
- Workflow execution across device ecosystem
- Worker status monitoring
- Distributed device collaboration (Super Device)
- Push notifications
- Real-time system metrics monitoring

## Features

### GUI Mode (Default)
- Full graphical interface using Fyne toolkit
- 10 tabs: Dashboard, Tasks, Workers, Projects, Sessions, LLM, Harmony System, Distributed Services, Resource Management, Settings
- Real-time data updates from backend API
- Harmony OS-specific themes and styling

### CLI Mode (nogui)
- Lightweight command-line interface
- All core functionality without GUI dependencies
- Suitable for headless/embedded Harmony devices
- Interactive shell mode

## Requirements

### GUI Mode
- Go 1.24.0+ (with toolchain go1.24.9)
- DevEco Studio 4.0+ (for Harmony OS deployment)
- HarmonyOS SDK (API 9+)
- Fyne toolkit dependencies:
  - Linux: `libgl1-mesa-dev`, `xorg-dev`
  - macOS: Xcode command line tools
  - Windows: TDM-GCC or similar

### CLI Mode (nogui)
- Go 1.24.0+ only
- No GUI dependencies required

## Dependencies

### Core Dependencies
```
dev.helix.code/internal/config      - Configuration management
dev.helix.code/internal/database    - PostgreSQL persistence
dev.helix.code/internal/hardware    - Hardware detection
dev.helix.code/internal/llm         - LLM provider integration
dev.helix.code/internal/monitoring  - System monitoring
dev.helix.code/internal/notification - Push notifications
dev.helix.code/internal/project     - Project management
dev.helix.code/internal/redis       - Caching
dev.helix.code/internal/server      - API server
dev.helix.code/internal/session     - Session management
dev.helix.code/internal/task        - Task management
dev.helix.code/internal/worker      - Worker pool management
```

### GUI Dependencies (main.go, theme.go)
```
fyne.io/fyne/v2           - Cross-platform GUI toolkit
fyne.io/fyne/v2/app       - Application management
fyne.io/fyne/v2/container - UI containers
fyne.io/fyne/v2/dialog    - Dialog boxes
fyne.io/fyne/v2/layout    - Layout management
fyne.io/fyne/v2/theme     - Theming support
fyne.io/fyne/v2/widget    - UI widgets
```

## Building

### GUI Mode (Default)

```bash
cd HelixCode

# Build for current platform
go build -o bin/harmony-os/helixcode-harmony ./applications/harmony-os

# Build for Harmony OS (ARM64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
  go build -ldflags="-s -w" \
  -o bin/harmony-os/helixcode-harmony \
  ./applications/harmony-os

# Using Makefile
make harmony-os
```

### CLI Mode (nogui)

```bash
cd HelixCode

# Build CLI-only version (no GUI dependencies)
go build -tags nogui \
  -o bin/harmony-os/helixcode-harmony-cli \
  ./applications/harmony-os

# Cross-compile for ARM64 (Harmony OS)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags nogui -ldflags="-s -w" \
  -o bin/harmony-os/helixcode-harmony-cli \
  ./applications/harmony-os
```

### Package as HAP

```bash
cd applications/harmony-os
hdc build release
```

## Installation

### From HAP Package

```bash
# Install via HDC
hdc install helixcode-harmony.hap
```

### From AppGallery

(Coming soon)

## Usage

### GUI Mode

Simply run the application:
```bash
./helixcode-harmony
```

The application will:
1. Load configuration from standard locations
2. Connect to backend API (configurable in Settings)
3. Initialize Harmony OS distributed features
4. Display the main window with all tabs

### CLI Mode

```bash
# Show help
./helixcode-harmony-cli help

# Show system status
./helixcode-harmony-cli status

# Show Harmony OS system info
./helixcode-harmony-cli system

# Manage projects
./helixcode-harmony-cli projects list
./helixcode-harmony-cli projects create --name "MyApp" --path "/path/to/app" --type go

# Manage sessions
./helixcode-harmony-cli sessions list
./helixcode-harmony-cli sessions create --name "DevSession" --project "proj_123"

# Manage tasks
./helixcode-harmony-cli tasks list
./helixcode-harmony-cli tasks create --type building --desc "Build the app" --priority high

# Manage workers
./helixcode-harmony-cli workers list
./helixcode-harmony-cli workers add --host 192.168.1.100 --user developer

# LLM operations
./helixcode-harmony-cli llm providers
./helixcode-harmony-cli llm models

# Distributed operations
./helixcode-harmony-cli distributed status
./helixcode-harmony-cli distributed discover

# Interactive mode
./helixcode-harmony-cli interactive
```

## Distributed Capabilities

### Distributed Engine

The HarmonyDistributedEngine manages:
- Device discovery across Harmony OS ecosystem
- Task scheduling based on device capabilities
- Scheduling policies: balanced, performance, power_efficient
- Cross-device data synchronization

### Cross-Device Access

Access HelixCode from any Harmony OS device:
1. Enable distributed mode in Settings
2. Discover nearby devices
3. Transfer session to another device

### Device Collaboration

Multiple devices can collaborate:
- Phone for quick task creation
- Tablet for project browsing
- PC for detailed workflow monitoring

### Task Scheduling

Tasks are automatically scheduled across available devices based on:
- CPU usage and availability
- Memory resources
- GPU capabilities (for AI tasks)
- Network connectivity
- Power efficiency requirements

## Configuration

### Server Setup

1. Open Settings tab (GUI) or edit config file (CLI)
2. Enter server URL (default: http://localhost:8080)
3. Configure authentication token if required
4. Test connection

### Configuration File

Stored in standard config locations:
- `./config/config.yaml`
- `~/.config/helixcode/config.yaml`
- `/etc/helixcode/config.yaml`

Example configuration:
```yaml
server:
  host: localhost
  port: 8080

database:
  host: localhost
  port: 5432
  name: helixcode
  enabled: true

redis:
  host: localhost
  port: 6379
  enabled: true

harmony:
  distributed:
    enabled: true
    discovery: true
    sync_interval: 30s
```

### Environment Variables

```bash
HELIX_AUTH_JWT_SECRET        # JWT authentication secret
HELIX_DATABASE_PASSWORD      # PostgreSQL password
HELIX_DATABASE_HOST          # Database host
HELIX_REDIS_PASSWORD         # Redis password
```

## UI Tabs (GUI Mode)

1. **Dashboard** - Overview, quick stats, and actions
2. **Tasks** - Task management with distributed scheduling
3. **Workers** - Worker pool management and device discovery
4. **Projects** - Project browsing and creation
5. **Sessions** - Development session management
6. **LLM** - AI model interaction and provider health
7. **Harmony System** - System metrics and capabilities
8. **Distributed Services** - Device network and sync status
9. **Resource Management** - Policy configuration
10. **Settings** - Theme, server, and configuration

## API Integration

The client connects to the HelixCode backend API for:
- Task retrieval and creation
- Worker status updates
- Project and session management
- LLM provider health checks

When the API is unavailable, the client operates in standalone mode with local data management.

## Theming

Four built-in themes:
- **Dark** - Standard dark theme
- **Light** - Standard light theme
- **Helix** - HelixCode branded purple theme
- **Harmony** - Warm orange Harmony OS-inspired theme (default)

## Offline Support

Limited offline functionality:
- View cached data
- Queue operations for later sync
- Auto-sync when connectivity restored

## Performance Optimization

### Memory Management
- Efficient caching strategy with automatic cleanup
- Background data refresh with configurable intervals
- Memory-aware loading

### Battery Optimization
- Adaptive sync intervals based on power state
- Background task limits following system policies
- Power-aware networking

## Troubleshooting

### Connection Issues
1. Check network connectivity
2. Verify server URL in Settings
3. Check SSL certificates
4. Verify authentication token

### Distributed Issues
1. Ensure devices on same network
2. Check HUAWEI ID login
3. Enable distributed capabilities in system settings
4. Check device permissions

### Build Issues

For GUI build errors (missing Fyne dependencies):
```bash
# Ubuntu/Debian
sudo apt-get install libgl1-mesa-dev xorg-dev

# Fedora
sudo dnf install mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel

# macOS
xcode-select --install
```

For nogui build, ensure `-tags nogui` is specified to exclude GUI dependencies.

## System Requirements

- Harmony OS 3.0+ (API 9+) or compatible Linux system
- 1GB RAM minimum (2GB recommended)
- 200MB storage
- Network connectivity for API features
- HUAWEI ID (for distributed features)

## Related Documentation

- [Harmony OS Documentation](https://developer.harmonyos.com/en/docs/)
- [HelixCode Server Documentation](../../cmd/server/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
- [Desktop Client](../desktop/README.md)
- [Aurora OS Client](../aurora-os/README.md)

## License

Part of the HelixCode project. See main LICENSE file for details.
