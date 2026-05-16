// Package rules provides hierarchical project rule management with pattern-based matching.
//
// The rules package enables definition and management of coding guidelines,
// style rules, and project conventions that can be automatically applied
// based on file patterns. Rules support inheritance from workspace to project
// to file-specific levels, with pattern matching via glob, regex, or exact paths.
//
// # Key Components
//
// Manager coordinates rules across multiple levels with inheritance:
//
//	manager := rules.NewManager()
//
//	// Load rules from standard locations
//	err := manager.LoadFromDirectory("/path/to/project")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get rules applicable to a specific file
//	matches := manager.GetRulesForFile("internal/auth/handler.go")
//	for _, match := range matches {
//	    fmt.Printf("Rule: %s (score: %d)\n", match.Rule.Name, match.Score)
//	    fmt.Println(match.Rule.Content)
//	}
//
// # Rule Hierarchy
//
// Rules are organized in three levels with increasing priority:
//
//  1. Workspace rules (.helix/.clinerules) - Lowest priority
//  2. Project rules (.clinerules in project root) - Medium priority
//  3. File-specific rules (*.clinerules files) - Highest priority
//
// Higher priority rules override lower priority ones when they match the same file.
//
// # Rule Definition
//
// Rule defines an individual coding guideline:
//
//	rule := &rules.Rule{
//	    ID:          "go-error-handling",
//	    Name:        "Go Error Handling",
//	    Description: "Guidelines for handling errors in Go code",
//	    Pattern:     "*.go",
//	    PatternType: rules.PatternTypeGlob,
//	    Content:     `Always check error returns and handle appropriately...`,
//	    Priority:    10,
//	    Category:    rules.RuleCategoryStyle,
//	    Scope:       rules.RuleScopeGlobal,
//	    Tags:        []string{"go", "errors", "best-practices"},
//	}
//
//	err := manager.AddProjectRule(rule)
//
// # Pattern Types
//
// Rules support multiple pattern types for file matching:
//
//	// Glob patterns
//	rule.PatternType = rules.PatternTypeGlob
//	rule.Pattern = "src/**/*.ts"    // Matches TypeScript files in src/
//
//	// Regular expressions
//	rule.PatternType = rules.PatternTypeRegex
//	rule.Pattern = `.*_test\.go$`   // Matches Go test files
//
//	// Exact path
//	rule.PatternType = rules.PatternTypeExact
//	rule.Pattern = "main.go"        // Matches only main.go
//
//	// Any file
//	rule.PatternType = rules.PatternTypeAny   // Matches all files
//
// # Rule Categories
//
// Rules are categorized by purpose:
//
//	RuleCategoryStyle         // Code style rules
//	RuleCategoryArchitecture  // Architecture guidelines
//	RuleCategorySecurity      // Security requirements
//	RuleCategoryTesting       // Testing guidelines
//	RuleCategoryDocumentation // Documentation rules
//	RuleCategoryPerformance   // Performance guidelines
//	RuleCategoryGeneral       // General guidelines
//
// # Rule Parsing
//
// The Parser component reads rules from .clinerules files:
//
//	parser := rules.NewParser("/path/to/.clinerules")
//	ruleSet, err := parser.Parse()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, rule := range ruleSet.Rules {
//	    fmt.Printf("Loaded rule: %s\n", rule.Name)
//	}
//
// # Rule Formatting
//
// Rules can be formatted for prompts to AI models:
//
//	formattedRules := manager.FormatRulesForFile("internal/handler.go")
//	// Returns markdown-formatted rules for inclusion in prompts
//
// # Querying Rules
//
// Rules can be queried by various criteria:
//
//	// By category
//	securityRules := manager.GetRulesByCategory(rules.RuleCategorySecurity)
//
//	// By tag
//	goRules := manager.GetRulesByTag("go")
//
//	// All rules
//	allRules := manager.GetAllRules()
//
// # Rule Persistence
//
// Rules can be saved to files:
//
//	err := manager.SaveProjectRules("/path/to/.clinerules")
//	err = manager.SaveWorkspaceRules("/path/to/.helix/.clinerules")
//
// # Thread Safety
//
// All Manager operations are thread-safe, allowing concurrent access from
// multiple goroutines.
//
// # Example .clinerules File Format
//
// The rules file format uses markdown-like syntax:
//
//	# Rule: Go Error Handling
//	Pattern: *.go
//	Category: style
//	Priority: 10
//
//	Always check error returns and handle them explicitly:
//	- Never use _ to discard errors
//	- Wrap errors with context using fmt.Errorf
//	- Return errors to callers for handling
//
//	---
//
//	# Rule: Test Coverage
//	Pattern: *_test.go
//	Category: testing
//
//	All test files should:
//	- Include table-driven tests where applicable
//	- Test error conditions
//	- Use testify for assertions
package rules
