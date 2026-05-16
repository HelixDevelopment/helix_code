# HelixCode Applications

This directory contains all client applications for HelixCode platform.

## Applications Overview

| Application | Type | Framework | Description |
|-------------|------|-----------|-------------|
| `terminal-ui/` | TUI | tview | Terminal-based user interface |
| `desktop/` | GUI | Fyne | Cross-platform desktop application |
| `aurora-os/` | GUI | Fyne | Aurora OS client application |
| `harmony-os/` | GUI | Fyne | Harmony OS client application |
| `ios/` | Mobile | gomobile | iOS framework bindings |
| `android/` | Mobile | gomobile | Android AAR bindings |

## Build Dependencies

### Terminal UI (No Special Dependencies)

The terminal-ui application uses `tview` and `tcell` which are pure Go libraries with no external dependencies.

```bash
make build-terminal-ui
# or
go build -o bin/helix-tui ./applications/terminal_ui
```

### Desktop GUI Applications

Desktop, Aurora OS, and Harmony OS applications use Fyne v2, which requires OpenGL and X11 development headers.

#### Linux (Debian/Ubuntu)

```bash
sudo apt-get update
sudo apt-get install -y \
    libgl1-mesa-dev \
    libxrandr-dev \
    libxcursor-dev \
    libxinerama-dev \
    libxi-dev \
    libxxf86vm-dev \
    xorg-dev
```

#### Linux (Fedora/RHEL/CentOS)

```bash
sudo dnf install -y \
    mesa-libGL-devel \
    libXrandr-devel \
    libXcursor-devel \
    libXinerama-devel \
    libXi-devel \
    libXxf86vm-devel \
    xorg-x11-server-devel
```

#### Linux (Arch Linux)

```bash
sudo pacman -S mesa libxrandr libxcursor libxinerama libxi
```

#### Linux (ALT Linux)

```bash
sudo apt-get install -y \
    libGL-devel \
    libXrandr-devel \
    libXcursor-devel \
    libXinerama-devel \
    libXi-devel
```

#### macOS

macOS includes the required dependencies by default. Ensure Xcode Command Line Tools are installed:

```bash
xcode-select --install
```

#### Windows

Windows builds require MinGW-w64 or TDM-GCC. For cross-compilation from Linux:

```bash
# Install cross-compilers
sudo apt-get install gcc-mingw-w64

# Build for Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
    go build -o bin/helix-desktop.exe ./applications/desktop
```

### Mobile Applications

Mobile builds require gomobile and platform-specific SDKs.

#### iOS

```bash
# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# Requires Xcode and iOS SDK
make mobile-ios
```

#### Android

```bash
# Set Android SDK and NDK paths
export ANDROID_HOME=$HOME/Android/Sdk
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/25.2.9519653

# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

make mobile-android
```

## Build Commands

### Individual Applications

```bash
# Terminal UI
go build -o bin/helix-tui ./applications/terminal_ui

# Desktop GUI
make desktop
# or: go build -o bin/helix-desktop ./applications/desktop

# Desktop CLI (no GUI dependencies)
make desktop-nogui
# or: go build -tags nogui -o bin/helix-desktop-cli ./applications/desktop

# Aurora OS
make aurora-os
# or: go build -o bin/aurora-os ./applications/aurora_os

# Harmony OS
make harmony-os
# or: go build -o bin/harmony-os ./applications/harmony_os
```

### Cross-Platform Builds

```bash
# Desktop for all platforms
make desktop-all

# Desktop for specific platforms
make desktop-linux
make desktop-macos
make desktop-windows

# Mobile bindings
make mobile         # Both iOS and Android
make mobile-ios     # iOS only
make mobile-android # Android only

# Aurora and Harmony OS
make aurora-harmony
```

## Application Features

### Terminal UI

- Full TUI interface using tview
- Task management dashboard
- Worker monitoring
- LLM provider configuration
- Real-time notifications
- Theme customization

### Desktop GUI

- Cross-platform Fyne-based interface
- Project management
- Workflow execution
- Worker pool visualization
- Task queue management
- Notification center
- Settings configuration

### Aurora OS / Harmony OS

- Platform-specific UI adaptations
- Native look and feel
- Full feature parity with desktop
- Optimized for touch and stylus input

### Mobile (iOS/Android)

- Shared core library bindings
- Native platform integration
- Background task monitoring
- Push notifications support

## Troubleshooting

### Linux: OpenGL errors

If you see errors like `fatal error: GL/gl.h: No such file or directory`:

```bash
# Ensure all dependencies are installed
sudo apt-get install libgl1-mesa-dev
```

### Linux: X11 errors

If you see errors about X11 headers:

```bash
# Install X11 development packages
sudo apt-get install xorg-dev
```

### macOS: CGO errors

Ensure Xcode Command Line Tools are properly installed:

```bash
xcode-select --install
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
```

### Building without GUI

Use the `nogui` build tag to build the desktop CLI without GUI dependencies:

```bash
go build -tags nogui -o bin/helix-desktop-cli ./applications/desktop
```

### Headless server environments

For CI/CD or headless servers, use either:
1. The `nogui` build tag
2. Xvfb for virtual display:

```bash
# Install Xvfb
sudo apt-get install xvfb

# Run build with virtual display
xvfb-run -a make desktop
```

## Testing

Applications include unit tests for non-GUI components:

```bash
# Test terminal-ui
go test -v ./applications/terminal_ui/...

# Test desktop (non-GUI components)
go test -v -tags nogui ./applications/desktop/...
```

## Configuration

All applications share configuration from `config/config.yaml`. Key settings:

```yaml
server:
  port: 8080
  address: localhost

database:
  enabled: true
  host: localhost
  port: 5432

llm:
  provider: ollama
  endpoint: http://localhost:11434
```

See `config/README.md` for complete configuration options.
