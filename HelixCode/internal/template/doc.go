// Package template provides reusable template management with variable substitution and validation.
//
// The template package enables creation, management, and rendering of templates
// for various purposes including code generation, prompts, documentation, and
// emails. Templates support variable placeholders with validation, default values,
// and type checking.
//
// # Key Components
//
// Manager coordinates template operations:
//
//	manager := template.NewManager()
//
//	// Register built-in templates
//	err := manager.RegisterBuiltinTemplates()
//
//	// Create and register custom template
//	tpl := template.NewTemplate("API Handler", "Generate API handler", template.TypeCode)
//	tpl.SetContent(`func {{handler_name}}(c *gin.Context) {
//	    {{body}}
//	}`)
//	tpl.AddVariable(template.Variable{Name: "handler_name", Required: true, Type: "string"})
//	tpl.AddVariable(template.Variable{Name: "body", Required: true, Type: "string"})
//
//	err = manager.Register(tpl)
//
// # Template Types
//
// Templates are categorized by purpose:
//
//	template.TypeCode          // Code generation templates
//	template.TypePrompt        // LLM prompt templates
//	template.TypeWorkflow      // Workflow definition templates
//	template.TypeDocumentation // Documentation templates
//	template.TypeEmail         // Email templates
//	template.TypeCustom        // Custom purpose templates
//
// # Variable Definition
//
// Variables define expected inputs for templates:
//
//	variable := template.Variable{
//	    Name:         "function_name",
//	    Description:  "Name of the function to generate",
//	    Required:     true,
//	    DefaultValue: "",
//	    Type:         "string",  // string, int, bool, etc.
//	}
//
//	tpl.AddVariable(variable)
//
// # Template Rendering
//
// Render templates with variable values:
//
//	// Using Manager
//	result, err := manager.Render(templateID, map[string]interface{}{
//	    "function_name": "HandleLogin",
//	    "parameters":    "ctx *gin.Context",
//	    "body":          "// TODO: implement",
//	})
//
//	// Using Template directly
//	result, err := tpl.Render(map[string]interface{}{
//	    "handler_name": "GetUsers",
//	    "body":         "users := db.GetAll()\nc.JSON(200, users)",
//	})
//
//	// Render by name
//	result, err = manager.RenderByName("API Handler", vars)
//
// # Variable Sets
//
// Use VariableSet for building render inputs:
//
//	vars := template.NewVariableSet()
//	vars.Set("name", "UserService")
//	vars.Set("methods", []string{"Create", "Read", "Update", "Delete"})
//
//	// Merge variable sets
//	defaults := template.NewVariableSet()
//	defaults.Set("author", "HelixCode")
//	vars.Merge(defaults)
//
// # Template Querying
//
// Find templates by various criteria:
//
//	// Get by ID
//	tpl, err := manager.Get(templateID)
//
//	// Get by name
//	tpl, err = manager.GetByName("Code Review")
//
//	// Get by type
//	codeTemplates := manager.GetByType(template.TypeCode)
//
//	// Get by tag
//	goTemplates := manager.GetByTag("go")
//
//	// Search across name, description, and tags
//	results := manager.Search("authentication")
//
//	// Get all templates
//	all := manager.GetAll()
//
// # Template Lifecycle
//
// Manage templates through their lifecycle:
//
//	// Update a template
//	err := manager.Update(templateID, func(tpl *template.Template) {
//	    tpl.SetContent(newContent)
//	    tpl.AddTag("updated")
//	})
//
//	// Delete a template
//	err = manager.Delete(templateID)
//
//	// Clear all templates
//	manager.Clear()
//
// # Persistence
//
// Templates can be loaded from and saved to files:
//
//	// Load from file
//	err := manager.LoadFromFile("templates/handler.json")
//
//	// Load from directory
//	count, err := manager.LoadFromDirectory("templates/")
//	fmt.Printf("Loaded %d templates\n", count)
//
//	// Save to file
//	err = manager.SaveToFile(templateID, "templates/handler.json")
//
// # Template Export/Import
//
// Export and import templates for sharing:
//
//	// Export template
//	snapshot, err := manager.Export(templateID)
//
//	// Import template
//	err = manager.Import(snapshot)
//
// # Lifecycle Callbacks
//
// Register callbacks for template events:
//
//	manager.OnCreate(func(tpl *template.Template) {
//	    log.Printf("Template created: %s", tpl.Name)
//	})
//
//	manager.OnUpdate(func(tpl *template.Template) {
//	    invalidateCache(tpl)
//	})
//
//	manager.OnDelete(func(tpl *template.Template) {
//	    log.Printf("Template deleted: %s", tpl.Name)
//	})
//
// # Variable Extraction
//
// Automatically extract variables from template content:
//
//	tpl := template.NewTemplate("Dynamic", "", template.TypeCode)
//	tpl.SetContent("Hello {{name}}, welcome to {{place}}!")
//
//	// Extract variables from content
//	variables := tpl.ExtractVariables()
//	// Returns: ["name", "place"]
//
// # Simple Rendering
//
// For one-off rendering without template registration:
//
//	result := template.RenderSimple(
//	    "Hello {{name}}!",
//	    map[string]interface{}{"name": "World"},
//	)
//	// Returns: "Hello World!"
//
// # Template Validation
//
// Templates are validated before registration:
//
//	err := tpl.Validate()
//	// Checks:
//	// - Name is not empty
//	// - Content is not empty
//	// - Type is valid
//	// - Declared variables exist in content
//
// # Statistics
//
// Get statistics about templates:
//
//	stats := manager.GetStatistics()
//
//	fmt.Printf("Total: %d\n", stats.TotalTemplates)
//	fmt.Printf("By Type: %v\n", stats.ByType)
//	fmt.Printf("Tag Cloud: %v\n", stats.TagCloud)
//
// # Built-in Templates
//
// The package includes built-in templates:
//   - Function: Go function template
//   - Code Review: Code review prompt
//   - Bug Fix: Bug fix assistance prompt
//   - Function Documentation: Function documentation template
//   - Status Update Email: Project status email template
//
// # Thread Safety
//
// All Manager operations are thread-safe through internal mutex protection,
// allowing concurrent access from multiple goroutines.
//
// # Metadata and Tags
//
// Templates support metadata and tags for organization:
//
//	tpl.SetMetadata("version", "2.0")
//	tpl.SetMetadata("author", "Team Alpha")
//
//	tpl.AddTag("production")
//	tpl.AddTag("approved")
//
//	if tpl.HasTag("approved") {
//	    deploy(tpl)
//	}
package template
