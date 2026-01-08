# HelixCode Harmony OS Client

A native client for Huawei Harmony OS (HarmonyOS NEXT), providing HelixCode development workflow capabilities on Harmony OS devices.

## Overview

Harmony OS is Huawei's distributed operating system. This client enables:
- Project management on Harmony OS devices
- Task monitoring and creation
- Workflow execution
- Worker status monitoring
- Distributed device collaboration
- Push notifications

## Requirements

- DevEco Studio 4.0+
- HarmonyOS SDK (API 9+)
- Go cross-compilation toolchain
- Harmony OS device or emulator

## Building

### Setup Development Environment

1. Download [DevEco Studio](https://developer.harmonyos.com/en/develop/deveco-studio/)
2. Install HarmonyOS SDK
3. Configure Go cross-compilation

### Build Go Backend

```bash
cd HelixCode

# Build for Harmony OS (ARM64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" \
  -o bin/harmony-os/helixcode-harmony \
  ./applications/harmony-os

# Using Makefile
make harmony-os
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

## Features

### Distributed Capabilities

Leverage Harmony OS distributed features:
- Cross-device project access
- Distributed task execution
- Device collaboration
- Seamless handoff

### Project Management

- Browse and manage projects
- View project files
- Quick project switching
- Metadata editing

### Task Dashboard

- Task list with status indicators
- Task creation with priority
- Real-time status updates
- Task filtering and search

### Workflow Execution

- One-tap workflow launch
- Progress visualization
- Step status tracking
- Error reporting

### Worker Monitor

- Worker health status
- Resource utilization
- Capability listing
- Task assignments

### Notifications

- Task completion alerts
- Task failure notifications
- Worker status changes
- Workflow updates

## Configuration

### Server Setup

1. Open app settings
2. Enter server URL
3. Configure authentication
4. Test connection

### Configuration File

Stored in app sandbox:
```
/data/app/com.helixcode.harmony/config.json
```

```json
{
  "server": {
    "url": "https://helixcode.example.com",
    "token": "your-jwt-token"
  },
  "sync": {
    "interval": 30,
    "background": true
  },
  "distributed": {
    "enabled": true,
    "discovery": true
  }
}
```

## UI Components

### Main Tabs

1. **Dashboard** - Overview and quick actions
2. **Projects** - Project browser
3. **Tasks** - Task management
4. **Workers** - Worker status
5. **Settings** - Configuration

### ArkUI Components

Built with ArkUI declarative framework:
- List components for data display
- Card layouts for details
- Navigation for routing
- Dialog for confirmations

## Distributed Features

### Cross-Device Access

Access HelixCode from any Harmony OS device:
1. Enable distributed mode
2. Discover nearby devices
3. Transfer session to another device

### Device Collaboration

Multiple devices can collaborate:
- Phone for quick task creation
- Tablet for project browsing
- PC for detailed workflow monitoring

### Seamless Handoff

Continue work across devices:
- Start on phone, continue on tablet
- Automatic state synchronization
- No re-authentication required

## Offline Support

Limited offline functionality:
- View cached data
- Queue operations
- Auto-sync when online

## Push Notifications

Supports Harmony OS push service:
- Task updates
- Workflow completions
- Worker alerts

## Development

### Project Structure

```
applications/harmony-os/
├── main.go              # Go backend
├── entry/               # ArkTS entry
│   ├── src/
│   │   └── main/
│   │       ├── ets/     # ArkTS code
│   │       └── resources/
│   └── build-profile.json5
├── AppScope/
│   └── app.json5
└── build-profile.json5
```

### Building for Development

```bash
# Build and deploy to emulator
hdc shell aa start -a MainAbility -b com.helixcode.harmony
```

### Debugging

```bash
# View logs
hdc hilog | grep HelixCode
```

## Performance Optimization

### Memory Management

- Efficient caching strategy
- Background data cleanup
- Memory-aware loading

### Battery Optimization

- Adaptive sync intervals
- Background task limits
- Power-aware networking

## Known Limitations

- Some features require API 10+
- Distributed features need device network
- Background sync limited by system policies

## Troubleshooting

### Connection Issues

1. Check network connectivity
2. Verify server URL
3. Check SSL certificates
4. Verify authentication

### Distributed Issues

1. Ensure devices on same network
2. Check HUAWEI ID login
3. Enable distributed capabilities
4. Check device permissions

### Performance

For better performance:
- Enable hardware acceleration
- Reduce sync frequency
- Clear cache periodically

## Requirements

- Harmony OS 3.0+ (API 9+)
- 1GB RAM recommended
- 200MB storage
- Network connectivity
- HUAWEI ID (for distributed features)

## Related Documentation

- [Harmony OS Documentation](https://developer.harmonyos.com/en/docs/)
- [Server Documentation](../../cmd/server/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
- [Aurora OS Client](../aurora-os/README.md)
