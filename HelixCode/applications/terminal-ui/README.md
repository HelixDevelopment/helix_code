# HelixCode Terminal UI

A terminal-based user interface for HelixCode, providing a rich interactive experience for managing projects, tasks, and workflows directly from your terminal.

## Overview

The Terminal UI (TUI) offers:
- Project browser and management
- Task creation and monitoring
- Real-time workflow execution visualization
- Worker status monitoring
- LLM provider configuration
- Split-pane layouts for multitasking

## Building

```bash
cd HelixCode
go build -o bin/helix-tui ./applications/terminal-ui
```

## Running

```bash
./bin/helix-tui --server http://localhost:8080
```

Or connect to a remote server:

```bash
./bin/helix-tui --server https://helixcode.example.com
```

## Key Bindings

### Global

| Key | Action |
|-----|--------|
| `q` | Quit application |
| `?` | Show help |
| `Tab` | Switch focus between panels |
| `Ctrl+C` | Cancel current operation |
| `Esc` | Close modal/go back |

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` / `←` | Move left / collapse |
| `l` / `→` | Move right / expand |
| `g` | Go to top |
| `G` | Go to bottom |
| `/` | Search |

### Actions

| Key | Action |
|-----|--------|
| `Enter` | Select / Confirm |
| `n` | New item |
| `d` | Delete item |
| `e` | Edit item |
| `r` | Refresh |

### Panels

| Key | Action |
|-----|--------|
| `1` | Projects panel |
| `2` | Tasks panel |
| `3` | Workers panel |
| `4` | Logs panel |

## Features

### Project Browser

Browse and manage projects with:
- Tree view of project files
- Quick project switching
- Project creation wizard
- Metadata editing

### Task Manager

Monitor and manage tasks:
- Task list with status indicators
- Task creation with type selection
- Real-time status updates
- Task filtering and search

### Workflow Execution

Execute and monitor workflows:
- Planning workflow
- Building workflow
- Testing workflow
- Refactoring workflow
- Step-by-step progress visualization

### Worker Monitor

View worker status:
- Health indicators
- Resource utilization
- Capability listing
- Task assignments

### Log Viewer

Real-time log streaming:
- Filterable by level
- Searchable
- Auto-scroll
- Export capability

## Configuration

Create `~/.config/helixcode/tui.yaml`:

```yaml
server:
  url: "http://localhost:8080"
  timeout: 30s

theme:
  primary: "#00ADD8"
  secondary: "#5DC9E2"
  background: "#1E1E1E"
  foreground: "#FFFFFF"

keybindings:
  quit: "q"
  help: "?"
  search: "/"

display:
  refresh_interval: 1s
  max_log_lines: 1000
```

## Themes

### Built-in Themes

- `default` - Dark theme with Go blue accent
- `light` - Light theme
- `gruvbox` - Gruvbox color scheme
- `dracula` - Dracula color scheme

Set theme via command line:
```bash
./bin/helix-tui --theme dracula
```

Or in config:
```yaml
theme:
  name: "dracula"
```

## Authentication

### Interactive Login

On first run, you'll be prompted for credentials:
```
Server: http://localhost:8080
Username: admin
Password: ********
```

### Token-based

Use a pre-configured token:
```bash
./bin/helix-tui --server http://localhost:8080 --token "your-jwt-token"
```

### Environment Variables

```bash
export HELIX_TUI_SERVER="http://localhost:8080"
export HELIX_TUI_TOKEN="your-jwt-token"
./bin/helix-tui
```

## Troubleshooting

### Display Issues

If the TUI doesn't render correctly:
- Ensure terminal supports 256 colors
- Set `TERM=xterm-256color`
- Try a different terminal emulator

### Connection Issues

- Verify server URL is correct
- Check network connectivity
- Ensure server is running

### Performance

For large projects or many tasks:
- Increase refresh interval
- Reduce max log lines
- Use filtering to limit displayed items

## Requirements

- Terminal with 256-color support
- Minimum 80x24 terminal size (100x30 recommended)
- Go 1.24+ (for building)
- HelixCode server running

## Dependencies

- [tview](https://github.com/rivo/tview) - Terminal UI framework
- [tcell](https://github.com/gdamore/tcell) - Terminal cell library

## Related Documentation

- [CLI Documentation](../../cmd/cli/README.md)
- [Server Documentation](../../cmd/server/README.md)
- [Desktop Application](../desktop/README.md)
