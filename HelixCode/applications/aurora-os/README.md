# HelixCode Aurora OS Client

A native client for Aurora OS (Open Mobile Platform), providing HelixCode development workflow capabilities on Aurora OS devices.

## Overview

Aurora OS (formerly Sailfish OS) is a mobile operating system developed by Open Mobile Platform. This client enables:
- Project management on Aurora OS devices
- Task monitoring and creation
- Workflow execution triggering
- Worker status viewing
- Push notifications for task updates

## Requirements

- Aurora OS SDK 4.0+
- Aurora OS device or emulator
- Go cross-compilation toolchain

## Building

### Setup Aurora OS SDK

1. Download Aurora OS SDK from [Open Mobile Platform](https://developer.auroraos.ru/)
2. Install the SDK following official instructions
3. Set up the build environment

### Cross-Compile Go Binary

```bash
cd HelixCode

# Build for Aurora OS (ARM64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" \
  -o bin/aurora-os/helixcode-aurora \
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

### Project Browser

- List connected projects
- View project details
- Quick project switching

### Task Dashboard

- View task list with status
- Create new tasks
- Update task status
- Filter by status/priority

### Workflow Trigger

- Trigger planning workflow
- Trigger building workflow
- Trigger testing workflow
- Trigger refactoring workflow

### Worker Monitor

- View worker status
- Health indicators
- Resource usage

### Notifications

- Task completion alerts
- Task failure notifications
- Worker status changes

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
    "theme": "auto"
  }
}
```

## UI Components

### Cover Page

When app is minimized:
- Active task count
- Worker status indicator
- Quick action button

### Pulley Menu

Pull down for quick actions:
- New task
- Refresh
- Settings

### Pages

1. **Projects** - Project list and details
2. **Tasks** - Task management
3. **Workers** - Worker monitoring
4. **Settings** - App configuration

## Offline Support

The client supports limited offline functionality:
- View cached project data
- View cached task list
- Queue task updates for sync
- Automatic sync when online

## Push Notifications

Enable push notifications for:
- Task status changes
- Worker alerts
- Workflow completions

Requires:
- Aurora Push Service configured
- Server webhook integration

## Development

### Project Structure

```
applications/aurora-os/
├── main.go           # Entry point
├── ui/               # UI components
├── api/              # Server communication
├── storage/          # Local storage
├── qml/              # QML UI files
├── icons/            # App icons
└── helixcode.spec    # RPM spec file
```

### Building for Development

```bash
# Build and deploy to emulator
aurora-sb2 -t AuroraOS-4.0.2.x86_64 go build ./applications/aurora-os
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

## Troubleshooting

### Connection Issues

1. Verify network connectivity
2. Check server URL (HTTPS required for production)
3. Verify SSL certificate is valid
4. Check firewall settings

### Performance

For better performance on older devices:
- Reduce sync interval
- Disable animations
- Limit cached data

### Installation Failures

1. Check available storage space
2. Verify RPM signature
3. Check Aurora OS version compatibility

## Requirements

- Aurora OS 4.0+
- 512MB RAM minimum
- 100MB storage
- Network connectivity

## Related Documentation

- [Aurora OS SDK Documentation](https://developer.auroraos.ru/doc)
- [Server Documentation](../../cmd/server/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
