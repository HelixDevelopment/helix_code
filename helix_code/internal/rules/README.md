# Rules Package

The `rules` package provides a hierarchical rule management engine for defining, matching, and applying project-specific coding guidelines, conventions, and best practices. Rules are automatically matched against files based on configurable patterns and applied with proper priority ordering.

## Overview

The rules package enables teams to codify their coding standards, architectural guidelines, security requirements, and other conventions into structured rules that can be:

- Automatically matched to relevant files using glob, regex, or exact patterns
- Organized into hierarchies with workspace, project, and file-specific levels
- Categorized by purpose (style, security, testing, etc.)
- Prioritized to control which rules take precedence
- Formatted for inclusion in AI prompts for context-aware code generation

### Key Features

- **Pattern-based matching**: Match rules to files using glob patterns (`*.go`), regular expressions (`.*_test\.go$`), or exact paths
- **Hierarchical inheritance**: Rules cascade from workspace to project to file-specific levels
- **Priority scoring**: Automatic scoring system determines which rules apply when multiple match
- **Thread-safe operations**: All manager operations are safe for concurrent access
- **Round-trip parsing**: Rules can be loaded from and saved to `.clinerules` files
- **Flexible categorization**: Organize rules by category and tags for easy querying

## Rule Types and Categories

### Pattern Types

The package supports four pattern types for matching rules against files:

| Pattern Type | Description | Example |
|--------------|-------------|---------|
| `PatternTypeGlob` | Standard glob pattern matching with `*`, `**`, and `?` | `*.go`, `src/**/*.ts` |
| `PatternTypeRegex` | Full regular expression matching | `.*_test\.go$` |
| `PatternTypeExact` | Exact file path match | `main.go`, `src/config.yaml` |
| `PatternTypeAny` | Matches any file (universal rule) | N/A |

```go
const (
    PatternTypeGlob  PatternType = "glob"   // Glob pattern (*.go, src/**/*.js)
    PatternTypeRegex PatternType = "regex"  // Regular expression
    PatternTypeExact PatternType = "exact"  // Exact file path match
    PatternTypeAny   PatternType = "any"    // Matches any file
)
```

### Rule Categories

Rules are organized into purpose-based categories:

| Category | Description | Use Cases |
|----------|-------------|-----------|
| `RuleCategoryStyle` | Code formatting and style | Naming conventions, formatting rules |
| `RuleCategoryArchitecture` | Architectural patterns | Layer boundaries, dependency rules |
| `RuleCategorySecurity` | Security requirements | Input validation, secret handling |
| `RuleCategoryTesting` | Testing standards | Coverage requirements, test patterns |
| `RuleCategoryDocumentation` | Documentation rules | Comment requirements, API docs |
| `RuleCategoryPerformance` | Performance guidelines | Optimization patterns, resource limits |
| `RuleCategoryGeneral` | General guidelines | Catch-all for uncategorized rules |

```go
const (
    RuleCategoryStyle         RuleCategory = "style"
    RuleCategoryArchitecture  RuleCategory = "architecture"
    RuleCategorySecurity      RuleCategory = "security"
    RuleCategoryTesting       RuleCategory = "testing"
    RuleCategoryDocumentation RuleCategory = "documentation"
    RuleCategoryPerformance   RuleCategory = "performance"
    RuleCategoryGeneral       RuleCategory = "general"
)
```

### Rule Scopes

Rules can be scoped to different levels of granularity:

| Scope | Description | Priority Bonus |
|-------|-------------|----------------|
| `RuleScopeGlobal` | Applies to entire project | +10 |
| `RuleScopeDirectory` | Applies to specific directory | +20 |
| `RuleScopeFile` | Applies to specific file | +30 |

```go
const (
    RuleScopeGlobal    RuleScope = "global"    // Applies to entire project
    RuleScopeDirectory RuleScope = "directory" // Applies to specific directory
    RuleScopeFile      RuleScope = "file"      // Applies to specific file
)
```

## Pattern Matching

### Glob Patterns

Glob patterns provide familiar file matching with wildcards:

```go
// Match all Go files
rule := &Rule{
    Pattern:     "*.go",
    PatternType: PatternTypeGlob,
}

// Match all TypeScript files in src/ and subdirectories
rule := &Rule{
    Pattern:     "src/**/*.ts",
    PatternType: PatternTypeGlob,
}

// Match files with single character variation
rule := &Rule{
    Pattern:     "test?.go",  // test1.go, testA.go, etc.
    PatternType: PatternTypeGlob,
}
```

**Glob Wildcards:**
- `*` - Matches any characters except `/`
- `**` - Matches any directory depth (zero or more levels)
- `?` - Matches any single character

### Regular Expressions

For complex matching requirements, use regex patterns:

```go
// Match Go test files
rule := &Rule{
    Pattern:     `.*_test\.go$`,
    PatternType: PatternTypeRegex,
}

// Match files starting with "api_" and ending with ".go"
rule := &Rule{
    Pattern:     `^api_.*\.go$`,
    PatternType: PatternTypeRegex,
}

// Match any handler file
rule := &Rule{
    Pattern:     `.*/handlers?/.*\.go$`,
    PatternType: PatternTypeRegex,
}
```

### Exact Matching

For rules that apply to specific files only:

```go
rule := &Rule{
    Pattern:     "main.go",
    PatternType: PatternTypeExact,
}

rule := &Rule{
    Pattern:     "internal/config/config.go",
    PatternType: PatternTypeExact,
}
```

### Universal Rules

For rules that apply to all files:

```go
rule := &Rule{
    PatternType: PatternTypeAny,
    Content:     "All code must include copyright headers",
}
```

## Rule Hierarchies

The rules package implements a three-level hierarchy with automatic inheritance:

```
.helix/.clinerules        <- Workspace rules (Priority: +1000)
.clinerules               <- Project rules   (Priority: +2000)
src/.clinerules           <- File rules      (Priority: +3000)
internal/auth/.clinerules <- File rules      (Priority: +3000)
```

### Priority Resolution

When multiple rules match a file, they are sorted by score (descending). The score is calculated as:

```
Score = (Priority * 100) + PatternTypeBonus + ScopeBonus + HierarchyBonus
```

**Pattern Type Bonuses:**
- Exact match: +50
- Glob (no wildcards): +40
- Glob (single `*`): +30
- Glob (multiple `*`): +20
- Regex: +25
- Any: +0

**Scope Bonuses:**
- File: +30
- Directory: +20
- Global: +10

**Hierarchy Bonuses:**
- File-specific rules: +3000
- Project rules: +2000
- Workspace rules: +1000

### Loading Rules from Directory

```go
manager := rules.NewManager()

// Automatically discovers and loads rules from:
// - .helix/.clinerules (workspace)
// - .clinerules (project)
// - **/*.clinerules (file-specific)
err := manager.LoadFromDirectory("/path/to/project")
if err != nil {
    log.Fatal(err)
}
```

## Complete API Reference

### Rule Structure

```go
type Rule struct {
    ID          string            // Unique identifier
    Name        string            // Human-readable name
    Description string            // Rule description
    Pattern     string            // File pattern (glob or regex)
    PatternType PatternType       // Type of pattern
    Content     string            // Rule content/guidelines
    Priority    int               // Priority (higher = more important)
    Category    RuleCategory      // Rule category
    Scope       RuleScope         // Where rule applies
    Tags        []string          // Tags for categorization
    Metadata    map[string]string // Additional metadata
}
```

### Rule Methods

```go
// Check if rule matches a file path
func (r *Rule) Matches(filePath string) bool

// Calculate match score for prioritization
func (r *Rule) MatchScore(filePath string) int

// Validate rule configuration
func (r *Rule) Validate() error

// Create a deep copy of the rule
func (r *Rule) Clone() *Rule

// String representation
func (r *Rule) String() string
```

### RuleSet Structure

```go
type RuleSet struct {
    Name        string            // Name of the rule set
    Description string            // Description
    Rules       []*Rule           // List of rules
    Metadata    map[string]string // Additional metadata
}
```

### RuleSet Methods

```go
// Add a rule (validates and checks for duplicates)
func (rs *RuleSet) AddRule(rule *Rule) error

// Remove a rule by ID
func (rs *RuleSet) RemoveRule(id string) bool

// Get a rule by ID
func (rs *RuleSet) GetRule(id string) *Rule

// Get all rules matching a file path (sorted by score)
func (rs *RuleSet) GetMatchingRules(filePath string) []*MatchResult

// Get rules by category
func (rs *RuleSet) GetRulesByCategory(category RuleCategory) []*Rule

// Get rules by tag
func (rs *RuleSet) GetRulesByTag(tag string) []*Rule

// Get rule count
func (rs *RuleSet) Count() int

// Clear all rules
func (rs *RuleSet) Clear()
```

### Manager Structure

```go
type Manager struct {
    workspaceRules *RuleSet            // Workspace-level rules
    projectRules   *RuleSet            // Project-level rules
    fileRules      map[string]*RuleSet // File-specific rules
    mu             sync.RWMutex        // Thread-safety
}
```

### Manager Methods

```go
// Create a new manager
func NewManager() *Manager

// Load rules from standard directory locations
func (m *Manager) LoadFromDirectory(dir string) error

// Load workspace rules from specific file
func (m *Manager) LoadWorkspaceRules(path string) error

// Load project rules from specific file
func (m *Manager) LoadProjectRules(path string) error

// Load file-specific rules
func (m *Manager) LoadFileRules(fileKey, path string) error

// Get all rules matching a file (with inheritance)
func (m *Manager) GetRulesForFile(filePath string) []*MatchResult

// Get all rules across all levels
func (m *Manager) GetAllRules() []*Rule

// Get rules by category from all levels
func (m *Manager) GetRulesByCategory(category RuleCategory) []*Rule

// Get rules by tag from all levels
func (m *Manager) GetRulesByTag(tag string) []*Rule

// Format rules as markdown for AI prompts
func (m *Manager) FormatRulesForFile(filePath string) string

// Add rule to workspace level
func (m *Manager) AddWorkspaceRule(rule *Rule) error

// Add rule to project level
func (m *Manager) AddProjectRule(rule *Rule) error

// Remove rule by ID from any level
func (m *Manager) RemoveRule(id string) bool

// Clear all rules
func (m *Manager) Clear()

// Get total rule count
func (m *Manager) Count() int

// Export all rules as single RuleSet
func (m *Manager) Export() *RuleSet

// Save workspace rules to file
func (m *Manager) SaveWorkspaceRules(path string) error

// Save project rules to file
func (m *Manager) SaveProjectRules(path string) error
```

### Parser Functions

```go
// Create parser for a file
func NewParser(filePath string) *Parser

// Parse file into RuleSet
func (p *Parser) Parse() (*RuleSet, error)

// Parse string content into RuleSet
func ParseString(content string) (*RuleSet, error)

// Format RuleSet back to .clinerules format
func Format(ruleSet *RuleSet) string
```

### MatchResult Structure

```go
type MatchResult struct {
    Matches  bool   // Whether the rule matches
    Score    int    // Match score (for prioritization)
    Rule     *Rule  // The matched rule
    FilePath string // The file path that was matched
}
```

## Creating Custom Rules

### Programmatic Rule Creation

```go
import "dev.helix.code/internal/rules"

manager := rules.NewManager()

// Create a Go error handling rule
errorRule := &rules.Rule{
    ID:          "go-error-handling",
    Name:        "Go Error Handling",
    Description: "Best practices for error handling in Go",
    Pattern:     "*.go",
    PatternType: rules.PatternTypeGlob,
    Content: `When handling errors in Go:
- Always check error returns
- Wrap errors with context using fmt.Errorf("...: %w", err)
- Never use _ to discard errors
- Return errors to callers for handling
- Use errors.Is() and errors.As() for error checking`,
    Priority:    10,
    Category:    rules.RuleCategoryStyle,
    Scope:       rules.RuleScopeGlobal,
    Tags:        []string{"go", "errors", "best-practices"},
}

if err := manager.AddProjectRule(errorRule); err != nil {
    log.Fatal(err)
}

// Create a security rule for authentication handlers
authRule := &rules.Rule{
    ID:          "auth-security",
    Name:        "Authentication Security",
    Description: "Security requirements for auth handlers",
    Pattern:     "internal/auth/**/*.go",
    PatternType: rules.PatternTypeGlob,
    Content: `Authentication handlers must:
- Always validate and sanitize input
- Use constant-time comparison for secrets
- Implement rate limiting
- Log all authentication attempts
- Never expose internal errors to clients`,
    Priority:    20,
    Category:    rules.RuleCategorySecurity,
    Scope:       rules.RuleScopeDirectory,
    Tags:        []string{"security", "auth", "critical"},
}

if err := manager.AddProjectRule(authRule); err != nil {
    log.Fatal(err)
}
```

### File-Based Rule Creation

Create a `.clinerules` file with the following format:

```
# Project Rules

[Go Error Handling]
pattern: *.go
description: Best practices for error handling in Go
priority: 10
category: style
tags: go, errors, best-practices

When handling errors in Go:
- Always check error returns
- Wrap errors with context using fmt.Errorf("...: %w", err)
- Never use _ to discard errors
- Return errors to callers for handling

[Test Coverage Requirements]
pattern: /*_test\.go$/
description: Testing standards for Go test files
priority: 15
category: testing
tags: testing, go, coverage

All test files must:
- Include table-driven tests where applicable
- Test both success and error paths
- Achieve minimum 80% coverage
- Use testify/assert for assertions

[Security - Authentication]
pattern: internal/auth/**/*.go
description: Security requirements for auth code
priority: 20
category: security
scope: directory
tags: security, auth, critical

Authentication code must:
- Validate all input parameters
- Use bcrypt for password hashing
- Implement proper session management
- Log security-relevant events
```

## Rule Validation

Rules are validated before being added to a RuleSet or Manager. Validation checks:

```go
func (r *Rule) Validate() error {
    // Name is required
    if r.Name == "" {
        return fmt.Errorf("rule name cannot be empty")
    }

    // Pattern is required unless PatternTypeAny
    if r.Pattern == "" && r.PatternType != PatternTypeAny {
        return fmt.Errorf("rule pattern cannot be empty")
    }

    // Content is required
    if r.Content == "" {
        return fmt.Errorf("rule content cannot be empty")
    }

    // Regex patterns must compile
    if r.PatternType == PatternTypeRegex {
        if _, err := regexp.Compile(r.Pattern); err != nil {
            return fmt.Errorf("invalid regex pattern: %w", err)
        }
    }

    // Glob patterns must be valid
    if r.PatternType == PatternTypeGlob {
        if strings.Contains(r.Pattern, "***") {
            return fmt.Errorf("invalid glob pattern: *** is not valid")
        }
    }

    return nil
}
```

### Validation Example

```go
rule := &rules.Rule{
    Name:        "",  // Empty - will fail validation
    Pattern:     "*.go",
    PatternType: rules.PatternTypeGlob,
    Content:     "Some content",
}

if err := rule.Validate(); err != nil {
    // err: "rule name cannot be empty"
    log.Printf("Invalid rule: %v", err)
}

// Invalid regex
rule = &rules.Rule{
    Name:        "Test",
    Pattern:     "[invalid(regex",
    PatternType: rules.PatternTypeRegex,
    Content:     "Content",
}

if err := rule.Validate(); err != nil {
    // err: "invalid regex pattern: ..."
    log.Printf("Invalid rule: %v", err)
}
```

## Usage Examples

### Basic Usage

```go
import "dev.helix.code/internal/rules"

// Create manager and load rules
manager := rules.NewManager()
if err := manager.LoadFromDirectory("/path/to/project"); err != nil {
    log.Fatal(err)
}

// Get rules for a specific file
matches := manager.GetRulesForFile("internal/auth/handler.go")
for _, match := range matches {
    fmt.Printf("Rule: %s (score: %d)\n", match.Rule.Name, match.Score)
    fmt.Println(match.Rule.Content)
}
```

### Formatting Rules for AI Prompts

```go
// Get formatted rules for inclusion in AI prompt
formattedRules := manager.FormatRulesForFile("internal/handler.go")

// Output format:
// # Project Rules
//
// Please follow these rules when making changes:
//
// ## Rule 1: Go Error Handling
// **Description:** Best practices for error handling
// **Applies to:** `*.go`
//
// Always check error returns...
//
// ---
```

### Querying Rules

```go
// Get all security rules
securityRules := manager.GetRulesByCategory(rules.RuleCategorySecurity)
for _, rule := range securityRules {
    fmt.Printf("Security rule: %s\n", rule.Name)
}

// Get all rules tagged with "critical"
criticalRules := manager.GetRulesByTag("critical")

// Get all rules across all levels
allRules := manager.GetAllRules()
fmt.Printf("Total rules: %d\n", len(allRules))
```

### Saving Rules

```go
// Save project rules to file
if err := manager.SaveProjectRules("/path/to/.clinerules"); err != nil {
    log.Fatal(err)
}

// Export all rules to single set
combined := manager.Export()
content := rules.Format(combined)
os.WriteFile("all-rules.clinerules", []byte(content), 0644)
```

### Custom Parser Usage

```go
// Parse rules from string
content := `
[My Rule]
pattern: *.go
priority: 5

My rule content here
`

ruleSet, err := rules.ParseString(content)
if err != nil {
    log.Fatal(err)
}

for _, rule := range ruleSet.Rules {
    fmt.Printf("Loaded: %s -> %s\n", rule.Name, rule.Pattern)
}
```

## Best Practices

### Rule Organization

1. **Use descriptive names**: Rule names should clearly indicate their purpose
2. **Write actionable content**: Rule content should provide specific, actionable guidance
3. **Set appropriate priorities**: Higher priorities for more critical rules (security > style)
4. **Use categories**: Categorize rules for easier filtering and organization
5. **Tag extensively**: Tags enable flexible querying across rule sets

### Pattern Selection

1. **Prefer glob patterns**: Globs are easier to read and maintain
2. **Use regex for complex matching**: When globs are insufficient
3. **Be specific**: More specific patterns get higher priority scores
4. **Avoid over-matching**: Don't use `PatternTypeAny` unless necessary

### Hierarchy Usage

1. **Workspace rules**: Organization-wide standards
2. **Project rules**: Project-specific conventions
3. **File rules**: Exceptions or highly specific requirements

### Performance Considerations

1. **Limit regex complexity**: Complex regex patterns impact matching performance
2. **Use exact matches**: When a rule applies to a single file
3. **Minimize file rules**: Directory-level rules are more efficient

## Integration Patterns

### Integration with AI Context

```go
import (
    "dev.helix.code/internal/context"
    "dev.helix.code/internal/rules"
)

func buildContext(filePath string) *context.Context {
    manager := rules.NewManager()
    manager.LoadFromDirectory(projectRoot)

    // Get formatted rules for the file
    rulesContent := manager.FormatRulesForFile(filePath)

    // Include in AI context
    ctx := context.NewContext()
    ctx.AddSystemMessage(rulesContent)

    return ctx
}
```

### Integration with Editor

```go
import (
    "dev.helix.code/internal/editor"
    "dev.helix.code/internal/rules"
)

func applyRulesBeforeEdit(filePath string, edit *editor.Edit) {
    manager := rules.NewManager()
    manager.LoadFromDirectory(projectRoot)

    matches := manager.GetRulesForFile(filePath)

    // Check if edit violates any rules
    for _, match := range matches {
        if match.Rule.Category == rules.RuleCategorySecurity {
            validateSecurityEdit(match.Rule, edit)
        }
    }
}
```

### Integration with Workflow Engine

```go
import (
    "dev.helix.code/internal/rules"
    "dev.helix.code/internal/workflow"
)

func createWorkflowStep(filePath string) *workflow.Step {
    manager := rules.NewManager()
    manager.LoadFromDirectory(projectRoot)

    // Get applicable rules for context
    rulesContent := manager.FormatRulesForFile(filePath)

    return &workflow.Step{
        Name:    "code-generation",
        Context: rulesContent,
    }
}
```

### Custom Rule Loader

```go
func loadRulesFromDatabase(db *sql.DB) (*rules.Manager, error) {
    manager := rules.NewManager()

    rows, err := db.Query("SELECT id, name, pattern, content FROM rules")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var id, name, pattern, content string
        rows.Scan(&id, &name, &pattern, &content)

        rule := &rules.Rule{
            ID:          id,
            Name:        name,
            Pattern:     pattern,
            PatternType: rules.PatternTypeGlob,
            Content:     content,
        }

        manager.AddProjectRule(rule)
    }

    return manager, nil
}
```

## Testing

Run tests for the rules package:

```bash
cd HelixCode
go test -v ./internal/rules/...
```

Run with coverage:

```bash
go test -cover ./internal/rules/...
```

Run specific test:

```bash
go test -v ./internal/rules -run TestRuleMatching
```

## File Format Reference

### .clinerules File Format

```
# Comments start with #

[Rule Name]
pattern: glob/regex/exact pattern
description: Optional description
priority: 1-100 (default: 1)
category: style|architecture|security|testing|documentation|performance|general
scope: global|directory|file
tags: comma, separated, tags
custom_key: custom metadata value

Rule content goes here.
Can span multiple lines.
Supports markdown formatting.

[Another Rule]
pattern: /regex_pattern/
...
```

### Pattern Type Detection

The parser automatically detects pattern types:

| Pattern | Detected Type |
|---------|---------------|
| `*` or `**` | `PatternTypeAny` |
| `/pattern/` | `PatternTypeRegex` |
| Contains `*` or `?` | `PatternTypeGlob` |
| Other | `PatternTypeExact` |
