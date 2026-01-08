# HelixCode Desktop Application

A cross-platform desktop application for HelixCode, built with Go and Fyne. Provides a graphical interface for managing AI-powered development workflows.

## Overview

The Desktop application offers:
- Native look and feel on Windows, macOS, and Linux
- Project management with visual file browser
- Task dashboard with drag-and-drop
- Workflow execution with progress visualization
- Worker management interface
- LLM provider configuration GUI
- System tray integration

## Requirements

### Build Requirements

- Go 1.24+
- C compiler (gcc on Linux/Windows, clang on macOS)
- OpenGL development libraries

#### Linux (Debian/Ubuntu)
```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev
```

#### Linux (Fedora)
```bash
sudo dnf install gcc mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel
```

#### macOS
```bash
xcode-select --install
```

#### Windows
- Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)
- Or use MSYS2 with `pacman -S mingw-w64-x86_64-gcc`

## Building

```bash
cd HelixCode

# Build for current platform
go build -o bin/helix-desktop ./applications/desktop

# Cross-compile for all platforms
make desktop-all
```

### Platform-Specific Builds

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/helix-desktop-linux ./applications/desktop

# macOS
GOOS=darwin GOARCH=amd64 go build -o bin/helix-desktop-macos ./applications/desktop

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/helix-desktop.exe ./applications/desktop
```

## Running

```bash
./bin/helix-desktop
```

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `--server` | HelixCode server URL | http://localhost:8080 |
| `--config` | Configuration file path | ~/.config/helixcode/desktop.yaml |
| `--theme` | UI theme (light/dark) | system |
| `--scale` | UI scaling factor | 1.0 |

## Features

### Dashboard

The main dashboard provides:
- Quick stats overview (projects, tasks, workers)
- Recent activity feed
- Quick action buttons
- System status indicators

### Project Manager

- Visual project browser
- File tree navigation
- Project creation wizard
- Metadata editor
- Git integration status

### Task Center

- Kanban-style task board
- Task creation dialog
- Priority and status filtering
- Drag-and-drop task management
- Task detail view

### Workflow Runner

- One-click workflow execution
- Progress visualization
- Step-by-step status
- Error reporting
- Workflow history

### Worker Manager

- Worker list with health status
- Resource utilization graphs
- Worker configuration
- Task assignment view

### Settings

- Server connection configuration
- Theme customization
- Keyboard shortcuts
- Notification preferences
- LLM provider configuration

## Configuration

Create `~/.config/helixcode/desktop.yaml`:

```yaml
server:
  url: "http://localhost:8080"
  timeout: 30s
  auto_reconnect: true

appearance:
  theme: "dark"
  scale: 1.0
  font_size: 12
  animations: true

behavior:
  start_minimized: false
  minimize_to_tray: true
  auto_update: true
  confirm_exit: true

notifications:
  enabled: true
  sound: true
  task_complete: true
  task_failed: true
  worker_offline: true

keyboard:
  new_project: "Ctrl+N"
  new_task: "Ctrl+T"
  settings: "Ctrl+,"
  quit: "Ctrl+Q"
```

## Keyboard Shortcuts

### Global

| Shortcut | Action |
|----------|--------|
| `Ctrl+N` | New project |
| `Ctrl+T` | New task |
| `Ctrl+R` | Refresh |
| `Ctrl+,` | Open settings |
| `Ctrl+Q` | Quit |
| `F1` | Help |
| `F11` | Toggle fullscreen |

### Navigation

| Shortcut | Action |
|----------|--------|
| `Ctrl+1` | Dashboard |
| `Ctrl+2` | Projects |
| `Ctrl+3` | Tasks |
| `Ctrl+4` | Workers |
| `Ctrl+5` | Settings |

### Actions

| Shortcut | Action |
|----------|--------|
| `Enter` | Open/Select |
| `Delete` | Delete item |
| `Ctrl+S` | Save |
| `Escape` | Cancel/Close |

## System Tray

When minimize to tray is enabled:
- Left-click: Show/hide window
- Right-click: Context menu
  - Show window
  - Quick actions
  - Settings
  - Quit

## Themes

### Built-in Themes

- **Light**: Light background with dark text
- **Dark**: Dark background with light text
- **System**: Follow system preference

### Custom Themes

Create custom themes in `~/.config/helixcode/themes/`:

```yaml
name: "My Theme"
colors:
  background: "#1E1E1E"
  foreground: "#FFFFFF"
  primary: "#00ADD8"
  secondary: "#5DC9E2"
  success: "#4CAF50"
  warning: "#FF9800"
  error: "#F44336"
```

## Troubleshooting

### Application won't start

1. Check OpenGL support:
   ```bash
   glxinfo | grep "OpenGL version"
   ```

2. Try software rendering:
   ```bash
   FYNE_RENDER_SOFTWARE=1 ./bin/helix-desktop
   ```

### High DPI Issues

Adjust scaling:
```bash
./bin/helix-desktop --scale 1.5
```

Or set environment variable:
```bash
export FYNE_SCALE=1.5
./bin/helix-desktop
```

### Connection Issues

- Verify server URL in settings
- Check firewall settings
- Ensure server is running

### Performance

For better performance:
- Disable animations in settings
- Reduce refresh rate
- Use dark theme (less GPU usage)

## Dependencies

- [Fyne](https://fyne.io/) - Cross-platform GUI toolkit
- Go 1.24+
- OpenGL 2.0+

## Related Documentation

- [CLI Documentation](../../cmd/cli/README.md)
- [Terminal UI](../terminal-ui/README.md)
- [Server Documentation](../../cmd/server/README.md)
