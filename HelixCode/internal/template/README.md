# Template Package

The `template` package provides project and code template management for the HelixCode platform.

## Overview

This package handles:
- Project templates
- Code snippet templates
- Template variable substitution
- Custom template creation
- Template library management

## Key Types

### TemplateManager

```go
type TemplateManager struct {
    templates map[string]*Template
    loader    *Loader
    renderer  *Renderer
}
```

### Template

```go
type Template struct {
    Name        string
    Description string
    Category    string
    Files       []*TemplateFile
    Variables   []*Variable
}
```

## Usage

### Loading Templates

```go
import "dev.helix.code/internal/template"

manager := template.NewManager(config)
err := manager.LoadTemplates(ctx, templatesPath)
```

### Rendering Templates

```go
// Render project template
err := manager.Render(ctx, "go-service", outputPath, map[string]interface{}{
    "ProjectName": "my-service",
    "Author":      "John Doe",
})
```

## Built-in Templates

- `go-service` - Go microservice
- `go-library` - Go library
- `python-api` - Python FastAPI
- `node-express` - Node.js Express
- `react-app` - React application

## Configuration

```yaml
template:
  templates_path: "~/.helixcode/templates"
  custom_templates: true
```

## Testing

```bash
go test -v ./internal/template/...
```
