# Project Package

The `project` package provides project lifecycle and session management for the HelixCode platform.

## Overview

This package handles:
- Project creation and configuration
- Project lifecycle management
- Session context tracking
- File and directory management
- Project templates

## Key Types

### Project

```go
type Project struct {
    ID          string
    Name        string
    Description string
    Path        string
    Type        ProjectType
    Language    string
    Status      Status
    Settings    *Settings
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### ProjectType

```go
type ProjectType string

const (
    TypeApplication ProjectType = "application"
    TypeLibrary     ProjectType = "library"
    TypeService     ProjectType = "service"
    TypePlugin      ProjectType = "plugin"
)
```

### Settings

```go
type Settings struct {
    BuildCommand   string
    TestCommand    string
    LintCommand    string
    WatchPaths     []string
    IgnorePaths    []string
    Environment    map[string]string
}
```

## Usage

### Creating the Project Manager

```go
import "dev.helix.code/internal/project"

manager := project.NewManager(db, config)
```

### Creating Projects

```go
// Create new project
proj := &project.Project{
    Name:        "my-service",
    Description: "A microservice for user management",
    Path:        "/path/to/project",
    Type:        project.TypeService,
    Language:    "go",
    Settings: &project.Settings{
        BuildCommand: "go build ./...",
        TestCommand:  "go test ./...",
        LintCommand:  "golangci-lint run",
    },
}

err := manager.CreateProject(ctx, proj)
```

### Project Templates

```go
// Create from template
proj, err := manager.CreateFromTemplate(ctx, "go-service", &project.CreateOptions{
    Name:        "user-service",
    Path:        "/path/to/project",
    Description: "User management service",
})
```

### Project Operations

```go
// Get project
proj, err := manager.GetProject(ctx, projectID)

// Update project
proj.Description = "Updated description"
err := manager.UpdateProject(ctx, proj)

// Delete project
err := manager.DeleteProject(ctx, projectID)

// List projects
projects, err := manager.ListProjects(ctx, &project.ListOptions{
    Language: "go",
    Status:   project.StatusActive,
})
```

### Session Management

```go
// Start session
session, err := manager.StartSession(ctx, projectID)

// Get current session
session, err := manager.GetCurrentSession(ctx, projectID)

// End session
err := manager.EndSession(ctx, sessionID)

// Session maintains context across interactions
session.AddContext("recent_files", []string{"main.go", "handler.go"})
session.AddContext("last_task", taskID)
```

### File Operations

```go
// List project files
files, err := manager.ListFiles(ctx, projectID, "/src")

// Read file
content, err := manager.ReadFile(ctx, projectID, "main.go")

// Write file
err := manager.WriteFile(ctx, projectID, "main.go", content)

// Watch for changes
changes := manager.Watch(ctx, projectID, []string{"*.go"})
for change := range changes {
    log.Info("File changed: %s", change.Path)
}
```

## Project Status

| Status | Description |
|--------|-------------|
| `active` | Project is active |
| `inactive` | Project is inactive |
| `archived` | Project is archived |
| `error` | Project has errors |

## Configuration

```yaml
project:
  default_path: "~/projects"
  templates_path: "~/.helixcode/templates"
  max_projects: 100
  session_timeout: 1h
  auto_save_interval: 5m
```

## Built-in Templates

- `go-service` - Go microservice
- `go-library` - Go library
- `python-api` - Python FastAPI service
- `node-express` - Node.js Express app
- `react-app` - React application

## Project Discovery

```go
// Discover projects in directory
projects, err := manager.DiscoverProjects(ctx, "/path/to/workspace")

// Auto-detect project type
projType, language, err := manager.DetectProjectType(ctx, "/path/to/project")
```

## Testing

```bash
go test -v ./internal/project/...
```

## Notes

- Sessions maintain context for continuity
- Use templates for consistent project setup
- Configure watch paths for file monitoring
- Archive projects instead of deleting
