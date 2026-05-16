# Template Package

The `template` package provides a comprehensive template management system for the HelixCode platform. It enables creation, validation, storage, and rendering of reusable templates for code generation, LLM prompts, documentation, workflows, and emails.

## Overview

The template system is built around two core components:

1. **Template**: Represents a single reusable template with content, variables, metadata, and tags
2. **Manager**: Coordinates template lifecycle operations including registration, storage, querying, and rendering

Key features include:
- Variable substitution with `{{variable}}` syntax
- Required and optional variables with default values
- Template type categorization for organization
- Thread-safe concurrent access
- File-based persistence (JSON format)
- Export/import for template sharing
- Lifecycle callbacks for integration
- Full-text search across templates
- Tag-based categorization and filtering
- Statistics and analytics

## Template Types

Templates are categorized by their intended purpose:

| Type | Constant | Description |
|------|----------|-------------|
| Code | `TypeCode` | Code generation templates (functions, classes, handlers) |
| Prompt | `TypePrompt` | LLM prompt templates for AI interactions |
| Workflow | `TypeWorkflow` | Workflow definition templates |
| Documentation | `TypeDocumentation` | Documentation templates (comments, READMEs) |
| Email | `TypeEmail` | Email templates for notifications |
| Custom | `TypeCustom` | User-defined template types |

```go
import "dev.helix.code/internal/template"

// Check type validity
if template.TypeCode.IsValid() {
    // Use the type
}

// Convert to string
typeStr := template.TypePrompt.String() // "prompt"
```

## Variable Substitution Syntax

Templates use double curly braces for variable placeholders:

```
{{variable_name}}
```

### Variable Naming Rules

- Must start with a letter or underscore
- Can contain letters, numbers, and underscores
- Case-sensitive
- Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`

### Examples

```go
// Simple variable
"Hello {{name}}"

// Multiple variables
"func {{function_name}}({{parameters}}) {{return_type}}"

// Multiline content
`func {{function_name}}({{parameters}}) {{return_type}} {
    {{body}}
}`
```

## API Reference

### Template

The `Template` struct represents a single reusable template:

```go
type Template struct {
    ID          string            // Unique identifier (auto-generated)
    Name        string            // Human-readable name
    Description string            // Template description
    Type        Type              // Template type (code, prompt, etc.)
    Content     string            // Template content with placeholders
    Variables   []Variable        // Declared variables
    Metadata    map[string]string // Additional metadata
    Tags        []string          // Tags for categorization
    Author      string            // Template author
    Version     string            // Template version
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

#### Template Methods

| Method | Description |
|--------|-------------|
| `NewTemplate(name, description string, templateType Type) *Template` | Create a new template |
| `SetContent(content string)` | Set template content |
| `AddVariable(variable Variable)` | Add a variable definition |
| `Render(vars map[string]interface{}) (string, error)` | Render with variables |
| `ValidateVariables(vars map[string]interface{}) error` | Validate variable values |
| `ExtractVariables() []string` | Extract variable names from content |
| `Validate() error` | Validate template structure |
| `Clone() *Template` | Create a copy of the template |
| `SetMetadata(key, value string)` | Set metadata value |
| `GetMetadata(key string) (string, bool)` | Get metadata value |
| `AddTag(tag string)` | Add a tag (deduplicates) |
| `HasTag(tag string) bool` | Check if tag exists |
| `String() string` | String representation |

### Variable

The `Variable` struct defines a template variable:

```go
type Variable struct {
    Name         string // Variable name (matches placeholder)
    Description  string // Human-readable description
    Required     bool   // Whether variable must be provided
    DefaultValue string // Default if not provided
    Type         string // Expected type (string, int, bool)
}
```

### Manager

The `Manager` handles template lifecycle operations:

```go
type Manager struct {
    // Internal fields - thread-safe via mutex
}
```

#### Manager Methods

| Method | Description |
|--------|-------------|
| `NewManager() *Manager` | Create a new manager |
| `Register(template *Template) error` | Register a template |
| `Get(id string) (*Template, error)` | Get template by ID |
| `GetByName(name string) (*Template, error)` | Get template by name |
| `GetByType(templateType Type) []*Template` | Get templates by type |
| `GetByTag(tag string) []*Template` | Get templates with tag |
| `GetAll() []*Template` | Get all templates |
| `Search(query string) []*Template` | Search templates |
| `Update(id string, updater func(*Template)) error` | Update a template |
| `Delete(id string) error` | Delete a template |
| `Render(templateID string, vars map[string]interface{}) (string, error)` | Render by ID |
| `RenderByName(name string, vars map[string]interface{}) (string, error)` | Render by name |
| `Count() int` | Total template count |
| `CountByType() map[Type]int` | Count per type |
| `Clear()` | Remove all templates |
| `LoadFromFile(filename string) error` | Load from JSON file |
| `LoadFromDirectory(dirPath string) (int, error)` | Load all from directory |
| `SaveToFile(templateID, filename string) error` | Save to JSON file |
| `Export(templateID string) (*TemplateSnapshot, error)` | Export template |
| `Import(snapshot *TemplateSnapshot) error` | Import template |
| `RegisterBuiltinTemplates() error` | Load built-in templates |
| `OnCreate(callback TemplateCallback)` | Register create callback |
| `OnUpdate(callback TemplateCallback)` | Register update callback |
| `OnDelete(callback TemplateCallback)` | Register delete callback |
| `GetStatistics() *ManagerStatistics` | Get statistics |

### Helper Types

```go
// VariableSet for building render inputs
type VariableSet map[string]interface{}

func NewVariableSet() VariableSet
func (vs VariableSet) Set(name string, value interface{})
func (vs VariableSet) Get(name string) (interface{}, bool)
func (vs VariableSet) Merge(other VariableSet)

// TemplateSnapshot for export/import
type TemplateSnapshot struct {
    Template   *Template
    ExportedAt time.Time
}

// ManagerStatistics for analytics
type ManagerStatistics struct {
    TotalTemplates int
    ByType         map[Type]int
    TagCloud       map[string]int
}

// TemplateCallback for lifecycle events
type TemplateCallback func(*Template)
```

### Utility Functions

```go
// Parse template from string (auto-detects variables)
func ParseTemplate(name string, content string, templateType Type) (*Template, error)

// Simple one-off rendering without template registration
func RenderSimple(content string, vars map[string]interface{}) string
```

## Built-in Templates Catalog

The package includes several built-in templates ready for use:

### Function (TypeCode)

Generates Go function definitions.

```go
// Content
`func {{function_name}}({{parameters}}) {{return_type}} {
    {{body}}
}`

// Variables
// - function_name (required): Name of the function
// - parameters (optional, default: ""): Function parameters
// - return_type (optional, default: ""): Return type
// - body (required): Function body

// Tags: code, go
```

### Code Review (TypePrompt)

LLM prompt for code review.

```go
// Content
`Review the following {{language}} code and provide feedback on:
- Code quality
- Best practices
- Potential issues
- Suggestions for improvement

Code:
{{code}}

Focus on: {{focus_areas}}`

// Variables
// - language (required): Programming language
// - code (required): Code to review
// - focus_areas (optional, default: "all aspects"): Areas to focus on

// Tags: prompt, review
```

### Bug Fix (TypePrompt)

LLM prompt for debugging assistance.

```go
// Variables
// - language (required): Programming language
// - error_message (required): Error message
// - code (required): Problematic code
// - expected_behavior (required): What should happen
// - actual_behavior (required): What is happening

// Tags: prompt, debug
```

### Function Documentation (TypeDocumentation)

Generates function documentation comments.

```go
// Content
`// {{function_name}} {{description}}
//
// Parameters:
{{parameters_doc}}
//
// Returns:
{{returns_doc}}
//
// Example:
//   {{example}}`

// Variables
// - function_name (required)
// - description (required)
// - parameters_doc (optional, default: "//   None")
// - returns_doc (optional, default: "//   None")
// - example (optional, default: "N/A")

// Tags: documentation, function
```

### Status Update Email (TypeEmail)

Project status update email template.

```go
// Variables
// - project_name (required)
// - recipient_name (required)
// - progress (required, type: int): Progress percentage
// - status (required)
// - completed_items (required)
// - in_progress_items (required)
// - next_steps (required)
// - sender_name (required)

// Tags: email, status
```

## Creating Custom Templates

### Basic Template Creation

```go
import "dev.helix.code/internal/template"

// Create manager
manager := template.NewManager()

// Create template
tpl := template.NewTemplate(
    "API Handler",           // Name
    "Generate REST API handler", // Description
    template.TypeCode,       // Type
)

// Set content with placeholders
tpl.SetContent(`func {{handler_name}}Handler(c *gin.Context) {
    // {{description}}
    {{body}}
}`)

// Define variables
tpl.AddVariable(template.Variable{
    Name:        "handler_name",
    Description: "Handler function name",
    Required:    true,
    Type:        "string",
})

tpl.AddVariable(template.Variable{
    Name:         "description",
    Description:  "Handler description comment",
    Required:     false,
    DefaultValue: "TODO: add description",
    Type:         "string",
})

tpl.AddVariable(template.Variable{
    Name:        "body",
    Description: "Handler implementation",
    Required:    true,
    Type:        "string",
})

// Add metadata and tags
tpl.Author = "DevTeam"
tpl.Version = "1.0.0"
tpl.SetMetadata("framework", "gin")
tpl.AddTag("api")
tpl.AddTag("go")
tpl.AddTag("http")

// Register with manager
err := manager.Register(tpl)
if err != nil {
    log.Fatalf("Failed to register template: %v", err)
}
```

### Using ParseTemplate for Quick Creation

```go
// Auto-detect variables from content
content := `package {{package_name}}

import "{{import_path}}"

type {{struct_name}} struct {
    {{fields}}
}`

tpl, err := template.ParseTemplate("Struct Definition", content, template.TypeCode)
if err != nil {
    log.Fatal(err)
}

// Variables are auto-detected as required
// - package_name
// - import_path
// - struct_name
// - fields
```

## Template Validation

Templates are validated before registration and rendering:

### Validation Checks

1. **Name validation**: Template name cannot be empty
2. **Content validation**: Template content cannot be empty
3. **Type validation**: Template type must be valid
4. **Variable consistency**: Declared variables must exist in content

```go
// Manual validation
tpl := template.NewTemplate("Test", "Test template", template.TypeCode)
tpl.SetContent("Hello {{name}}")
tpl.AddVariable(template.Variable{Name: "name", Required: true})

err := tpl.Validate()
if err != nil {
    log.Printf("Validation failed: %v", err)
}
```

### Render-time Validation

```go
// Required variable missing
_, err := tpl.Render(map[string]interface{}{})
// Error: required variable 'name' is missing

// Unreplaced placeholder
tpl.SetContent("Hello {{name}}, {{unknown}}")
_, err = tpl.Render(map[string]interface{}{"name": "World"})
// Error: template has unreplaced placeholders
```

## Usage Examples

### Example 1: Code Generation Pipeline

```go
package main

import (
    "fmt"
    "log"

    "dev.helix.code/internal/template"
)

func main() {
    manager := template.NewManager()

    // Create CRUD handler template
    crudTpl := template.NewTemplate("CRUD Handler", "Generate CRUD handlers", template.TypeCode)
    crudTpl.SetContent(`// {{resource}}Handler handles {{resource}} CRUD operations
type {{resource}}Handler struct {
    service *{{resource}}Service
}

func New{{resource}}Handler(service *{{resource}}Service) *{{resource}}Handler {
    return &{{resource}}Handler{service: service}
}

func (h *{{resource}}Handler) Create(c *gin.Context) {
    var req Create{{resource}}Request
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    result, err := h.service.Create(c.Request.Context(), &req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, result)
}

func (h *{{resource}}Handler) Get(c *gin.Context) {
    id := c.Param("id")
    result, err := h.service.Get(c.Request.Context(), id)
    if err != nil {
        c.JSON(404, gin.H{"error": "not found"})
        return
    }
    c.JSON(200, result)
}`)

    crudTpl.AddVariable(template.Variable{Name: "resource", Required: true, Type: "string"})
    crudTpl.AddTag("crud")
    crudTpl.AddTag("handler")

    manager.Register(crudTpl)

    // Generate handler for User resource
    code, err := manager.RenderByName("CRUD Handler", map[string]interface{}{
        "resource": "User",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(code)
}
```

### Example 2: LLM Prompt Templates

```go
package main

import (
    "fmt"

    "dev.helix.code/internal/template"
)

func main() {
    manager := template.NewManager()
    manager.RegisterBuiltinTemplates()

    // Use built-in Code Review template
    prompt, _ := manager.RenderByName("Code Review", map[string]interface{}{
        "language": "Go",
        "code": `func processData(data []byte) error {
    if data == nil {
        return nil
    }
    // Process...
    return nil
}`,
        "focus_areas": "error handling and edge cases",
    })

    fmt.Println(prompt)

    // Create custom analysis prompt
    analysisTpl := template.NewTemplate("Architecture Analysis", "Analyze code architecture", template.TypePrompt)
    analysisTpl.SetContent(`Analyze the following {{language}} codebase architecture:

Project Structure:
{{structure}}

Key Files:
{{key_files}}

Please provide:
1. Overall architecture assessment
2. Design pattern identification
3. Potential improvements
4. Scalability considerations

Focus specifically on: {{focus}}`)

    analysisTpl.AddVariable(template.Variable{Name: "language", Required: true})
    analysisTpl.AddVariable(template.Variable{Name: "structure", Required: true})
    analysisTpl.AddVariable(template.Variable{Name: "key_files", Required: true})
    analysisTpl.AddVariable(template.Variable{Name: "focus", Required: false, DefaultValue: "overall quality"})

    manager.Register(analysisTpl)
}
```

### Example 3: Template Persistence

```go
package main

import (
    "log"
    "os"
    "path/filepath"

    "dev.helix.code/internal/template"
)

func main() {
    manager := template.NewManager()

    // Create template
    tpl := template.NewTemplate("Service", "Microservice template", template.TypeCode)
    tpl.SetContent(`package {{package}}

type {{name}}Service struct {
    db *Database
}

func New{{name}}Service(db *Database) *{{name}}Service {
    return &{{name}}Service{db: db}
}`)
    tpl.AddVariable(template.Variable{Name: "package", Required: true})
    tpl.AddVariable(template.Variable{Name: "name", Required: true})

    manager.Register(tpl)

    // Save to file
    templatesDir := filepath.Join(os.Getenv("HOME"), ".helixcode", "templates")
    os.MkdirAll(templatesDir, 0755)

    err := manager.SaveToFile(tpl.ID, filepath.Join(templatesDir, "service.json"))
    if err != nil {
        log.Fatal(err)
    }

    // Load templates on startup
    newManager := template.NewManager()
    count, err := newManager.LoadFromDirectory(templatesDir)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Loaded %d templates", count)
}
```

### Example 4: Template Lifecycle Callbacks

```go
package main

import (
    "log"

    "dev.helix.code/internal/template"
)

func main() {
    manager := template.NewManager()

    // Register callbacks
    manager.OnCreate(func(tpl *template.Template) {
        log.Printf("Template created: %s (type: %s)", tpl.Name, tpl.Type)
        // Send to analytics
        // Invalidate caches
    })

    manager.OnUpdate(func(tpl *template.Template) {
        log.Printf("Template updated: %s (version: %s)", tpl.Name, tpl.Version)
        // Clear template cache
        // Notify dependent services
    })

    manager.OnDelete(func(tpl *template.Template) {
        log.Printf("Template deleted: %s", tpl.Name)
        // Cleanup references
        // Archive for audit
    })

    // Operations now trigger callbacks
    tpl := template.NewTemplate("Test", "Test template", template.TypeCode)
    tpl.SetContent("Hello {{name}}")
    tpl.AddVariable(template.Variable{Name: "name", Required: true})

    manager.Register(tpl) // Triggers OnCreate

    manager.Update(tpl.ID, func(t *template.Template) {
        t.Version = "1.1.0"
    }) // Triggers OnUpdate

    manager.Delete(tpl.ID) // Triggers OnDelete
}
```

### Example 5: Template Search and Discovery

```go
package main

import (
    "fmt"

    "dev.helix.code/internal/template"
)

func main() {
    manager := template.NewManager()
    manager.RegisterBuiltinTemplates()

    // Add more templates
    addCustomTemplates(manager)

    // Search by query (searches name, description, tags)
    results := manager.Search("code")
    fmt.Printf("Found %d templates matching 'code'\n", len(results))

    // Get by type
    prompts := manager.GetByType(template.TypePrompt)
    fmt.Printf("Found %d prompt templates\n", len(prompts))

    // Get by tag
    goTemplates := manager.GetByTag("go")
    fmt.Printf("Found %d Go templates\n", len(goTemplates))

    // Get statistics
    stats := manager.GetStatistics()
    fmt.Printf("Total templates: %d\n", stats.TotalTemplates)
    fmt.Printf("By type: %v\n", stats.ByType)
    fmt.Printf("Tag cloud: %v\n", stats.TagCloud)
}

func addCustomTemplates(manager *template.Manager) {
    // Add your custom templates here
}
```

## Best Practices

### 1. Use Descriptive Variable Names

```go
// Good
tpl.SetContent("func {{function_name}}({{input_parameters}}) {{return_type}}")

// Avoid
tpl.SetContent("func {{fn}}({{p}}) {{r}}")
```

### 2. Provide Default Values for Optional Variables

```go
tpl.AddVariable(template.Variable{
    Name:         "license",
    Description:  "License header to include",
    Required:     false,
    DefaultValue: "MIT License",
    Type:         "string",
})
```

### 3. Use Tags for Organization

```go
// Multiple relevant tags
tpl.AddTag("go")
tpl.AddTag("http")
tpl.AddTag("api")
tpl.AddTag("production")
```

### 4. Version Your Templates

```go
tpl.Version = "2.1.0"
tpl.SetMetadata("changelog", "Added error handling section")
```

### 5. Validate Before Registration

```go
if err := tpl.Validate(); err != nil {
    log.Printf("Template validation failed: %v", err)
    return err
}
manager.Register(tpl)
```

### 6. Use VariableSet for Complex Inputs

```go
vars := template.NewVariableSet()
vars.Set("name", "UserService")
vars.Set("methods", formatMethods(methods))

// Merge common defaults
vars.Merge(getProjectDefaults())

result, _ := tpl.Render(vars)
```

### 7. Handle Errors Appropriately

```go
result, err := manager.Render(templateID, vars)
if err != nil {
    if strings.Contains(err.Error(), "required variable") {
        // Handle missing variable
    } else if strings.Contains(err.Error(), "template not found") {
        // Handle missing template
    }
    return err
}
```

## Integration Patterns

### Integration with LLM Package

```go
import (
    "dev.helix.code/internal/llm"
    "dev.helix.code/internal/template"
)

func generateWithLLM(manager *template.Manager, provider llm.Provider) {
    // Render prompt template
    prompt, _ := manager.RenderByName("Code Review", map[string]interface{}{
        "language": "Go",
        "code":     sourceCode,
    })

    // Send to LLM
    response, _ := provider.Generate(ctx, &llm.LLMRequest{
        Prompt: prompt,
    })
}
```

### Integration with Workflow Package

```go
import (
    "dev.helix.code/internal/template"
    "dev.helix.code/internal/workflow"
)

func createWorkflowStep(manager *template.Manager) *workflow.Step {
    // Use workflow template
    definition, _ := manager.RenderByName("Build Step", map[string]interface{}{
        "project":   "my-service",
        "buildCmd":  "go build ./...",
        "testCmd":   "go test ./...",
    })

    return workflow.ParseStep(definition)
}
```

### Integration with Notification Package

```go
import (
    "dev.helix.code/internal/notification"
    "dev.helix.code/internal/template"
)

func sendStatusUpdate(manager *template.Manager, notifier *notification.Service) {
    // Render email template
    body, _ := manager.RenderByName("Status Update Email", map[string]interface{}{
        "project_name":      "HelixCode",
        "recipient_name":    "Team",
        "progress":          75,
        "status":            "On Track",
        "completed_items":   "- Feature A\n- Feature B",
        "in_progress_items": "- Feature C",
        "next_steps":        "- Testing\n- Documentation",
        "sender_name":       "PM Bot",
    })

    notifier.SendEmail(ctx, recipients, "Status Update", body)
}
```

## Configuration

Templates can be configured via the main config file:

```yaml
template:
  templates_path: "~/.helixcode/templates"  # Custom templates directory
  custom_templates: true                     # Enable custom templates
  auto_load: true                           # Auto-load on startup
  validation:
    strict: true                            # Strict validation mode
    require_description: false              # Require template descriptions
```

## Testing

```bash
# Run all template tests
go test -v ./internal/template/...

# Run specific test
go test -v ./internal/template -run TestRender

# With coverage
go test -cover ./internal/template/...

# Run benchmarks
go test -bench=. ./internal/template/...
```

## Thread Safety

All `Manager` operations are thread-safe:

- Internal mutex (`sync.RWMutex`) protects all state
- Read operations use read locks
- Write operations use write locks
- Safe for concurrent access from multiple goroutines

```go
// Safe concurrent usage
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(idx int) {
        defer wg.Done()
        manager.Render(templateID, vars)
    }(i)
}
wg.Wait()
```

## Related Packages

- `internal/llm`: LLM provider integration (uses prompt templates)
- `internal/workflow`: Workflow engine (uses workflow templates)
- `internal/context`: Context building for AI conversations
- `internal/notification`: Notification system (uses email templates)
- `internal/editor`: Code editing (may use code templates)
