package challenges

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UseCaseValidator validates that use cases are documented and functional
type UseCaseValidator struct {
	config *ChallengeConfig
}

// NewUseCaseValidator creates a new use case validator
func NewUseCaseValidator(config *ChallengeConfig) *UseCaseValidator {
	return &UseCaseValidator{
		config: config,
	}
}

// ValidateUseCases checks for documented use cases and their implementation
func (v *UseCaseValidator) ValidateUseCases(ctx context.Context, spec *ChallengeSpec, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	// Check if README exists and has use cases
	readmePath := filepath.Join(resultDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		results = append(results, ValidationResult{
			CheckName: "use_case_documentation",
			Passed:    false,
			Error:     "README.md not found",
			Details:   "Use cases should be documented in README.md",
			Timestamp: time.Now(),
		})
		return results
	}

	readmeContent, err := os.ReadFile(readmePath)
	if err != nil {
		results = append(results, ValidationResult{
			CheckName: "use_case_documentation",
			Passed:    false,
			Error:     fmt.Sprintf("Failed to read README.md: %v", err),
			Timestamp: time.Now(),
		})
		return results
	}

	readme := string(readmeContent)

	// Check for essential documentation sections
	results = append(results, v.checkDocumentationSections(readme)...)

	// Check for use case examples
	results = append(results, v.checkUseCaseExamples(readme, spec)...)

	// Check for API documentation if it's an API project
	if spec.Type == ChallengeTypeAPI || strings.Contains(strings.ToLower(spec.Description), "api") {
		results = append(results, v.checkAPIDocumentation(readme)...)
	}

	// Check for setup instructions
	results = append(results, v.checkSetupInstructions(readme)...)

	// Check for testing instructions
	results = append(results, v.checkTestingInstructions(readme)...)

	return results
}

// checkDocumentationSections verifies essential documentation sections exist
func (v *UseCaseValidator) checkDocumentationSections(readme string) []ValidationResult {
	results := []ValidationResult{}
	readmeLower := strings.ToLower(readme)

	requiredSections := map[string]string{
		"features": "## Features",
		"setup":    "## Setup|## Installation",
		"usage":    "## Usage|## API Endpoints|## Commands",
	}

	missingSections := []string{}
	for section, patterns := range requiredSections {
		found := false
		for _, pattern := range strings.Split(patterns, "|") {
			if strings.Contains(readmeLower, strings.ToLower(pattern)) {
				found = true
				break
			}
		}
		if !found {
			missingSections = append(missingSections, section)
		}
	}

	if len(missingSections) > 0 {
		results = append(results, ValidationResult{
			CheckName: "documentation_sections",
			Passed:    false,
			Error:     "Missing required documentation sections",
			Details:   fmt.Sprintf("Missing sections: %s", strings.Join(missingSections, ", ")),
			Timestamp: time.Now(),
		})
	} else {
		results = append(results, ValidationResult{
			CheckName: "documentation_sections",
			Passed:    true,
			Message:   "All required documentation sections present",
			Timestamp: time.Now(),
		})
	}

	return results
}

// checkUseCaseExamples verifies use case examples are documented
func (v *UseCaseValidator) checkUseCaseExamples(readme string, spec *ChallengeSpec) []ValidationResult {
	results := []ValidationResult{}

	// For API projects, check for endpoint examples
	if spec.Type == ChallengeTypeAPI || strings.Contains(strings.ToLower(spec.Description), "api") {
		// Look for code blocks with HTTP methods or curl examples
		hasExamples := strings.Contains(readme, "```bash") ||
			strings.Contains(readme, "```sh") ||
			strings.Contains(readme, "curl") ||
			strings.Contains(readme, "GET") ||
			strings.Contains(readme, "POST") ||
			strings.Contains(readme, "PUT") ||
			strings.Contains(readme, "DELETE")

		if !hasExamples {
			results = append(results, ValidationResult{
				CheckName: "use_case_examples",
				Passed:    false,
				Error:     "No API usage examples found",
				Details:   "README should include examples of how to use API endpoints (e.g., curl commands)",
				Timestamp: time.Now(),
			})
			return results
		}

		results = append(results, ValidationResult{
			CheckName: "use_case_examples",
			Passed:    true,
			Message:   "API usage examples found in documentation",
			Timestamp: time.Now(),
		})
	}

	// For CLI projects, check for command examples
	if spec.Type == ChallengeTypeCLI || strings.Contains(strings.ToLower(spec.Description), "cli") {
		hasExamples := strings.Contains(readme, "```bash") ||
			strings.Contains(readme, "```sh") ||
			strings.Contains(readme, "$ ")

		if !hasExamples {
			results = append(results, ValidationResult{
				CheckName: "use_case_examples",
				Passed:    false,
				Error:     "No CLI usage examples found",
				Details:   "README should include examples of CLI commands",
				Timestamp: time.Now(),
			})
			return results
		}

		results = append(results, ValidationResult{
			CheckName: "use_case_examples",
			Passed:    true,
			Message:   "CLI usage examples found in documentation",
			Timestamp: time.Now(),
		})
	}

	return results
}

// checkAPIDocumentation verifies API endpoints are documented
func (v *UseCaseValidator) checkAPIDocumentation(readme string) []ValidationResult {
	results := []ValidationResult{}

	// Check for API endpoint documentation
	endpointPatterns := []string{
		"GET",
		"POST",
		"PUT",
		"DELETE",
		"PATCH",
	}

	foundEndpoints := 0
	for _, pattern := range endpointPatterns {
		if strings.Contains(readme, pattern) {
			foundEndpoints++
		}
	}

	if foundEndpoints < 2 {
		results = append(results, ValidationResult{
			CheckName: "api_documentation",
			Passed:    false,
			Error:     "Insufficient API endpoint documentation",
			Details:   "README should document all API endpoints with HTTP methods",
			Timestamp: time.Now(),
		})
		return results
	}

	// Check for endpoint descriptions
	hasEndpointList := strings.Contains(readme, "- `GET") ||
		strings.Contains(readme, "- `POST") ||
		strings.Contains(readme, "* `GET") ||
		strings.Contains(readme, "* `POST")

	if !hasEndpointList {
		results = append(results, ValidationResult{
			CheckName: "api_documentation",
			Passed:    false,
			Error:     "API endpoints not properly formatted",
			Details:   "README should list API endpoints in a clear format (bullet points with HTTP methods)",
			Timestamp: time.Now(),
		})
		return results
	}

	results = append(results, ValidationResult{
		CheckName: "api_documentation",
		Passed:    true,
		Message:   "API endpoints are well documented",
		Timestamp: time.Now(),
	})

	return results
}

// checkSetupInstructions verifies setup instructions are complete
func (v *UseCaseValidator) checkSetupInstructions(readme string) []ValidationResult {
	results := []ValidationResult{}
	readmeLower := strings.ToLower(readme)

	// Check for installation steps
	hasSteps := strings.Contains(readme, "1.") ||
		strings.Contains(readme, "- ") ||
		strings.Contains(readme, "* ")

	if !hasSteps {
		results = append(results, ValidationResult{
			CheckName: "setup_instructions",
			Passed:    false,
			Error:     "No numbered or bulleted setup steps found",
			Details:   "README should have clear step-by-step setup instructions",
			Timestamp: time.Now(),
		})
		return results
	}

	// Check for common setup elements
	setupElements := map[string]bool{
		"install":       false,
		"dependencies":  false,
		"configuration": false,
		"environment":   false,
	}

	for element := range setupElements {
		if strings.Contains(readmeLower, element) {
			setupElements[element] = true
		}
	}

	foundElements := 0
	for _, found := range setupElements {
		if found {
			foundElements++
		}
	}

	if foundElements < 2 {
		results = append(results, ValidationResult{
			CheckName: "setup_instructions",
			Passed:    false,
			Error:     "Setup instructions are incomplete",
			Details:   "README should cover installation, dependencies, and configuration",
			Timestamp: time.Now(),
		})
		return results
	}

	results = append(results, ValidationResult{
		CheckName: "setup_instructions",
		Passed:    true,
		Message:   "Setup instructions are complete and clear",
		Timestamp: time.Now(),
	})

	return results
}

// checkTestingInstructions verifies testing instructions are documented
func (v *UseCaseValidator) checkTestingInstructions(readme string) []ValidationResult {
	results := []ValidationResult{}
	readmeLower := strings.ToLower(readme)

	// Check for testing section
	hasTestingSection := strings.Contains(readmeLower, "## test") ||
		strings.Contains(readmeLower, "## testing")

	if !hasTestingSection {
		results = append(results, ValidationResult{
			CheckName: "testing_instructions",
			Passed:    false,
			Error:     "No testing section found",
			Details:   "README should include instructions on how to run tests",
			Timestamp: time.Now(),
		})
		return results
	}

	// Check for test command examples
	hasTestCommand := strings.Contains(readme, "go test") ||
		strings.Contains(readme, "npm test") ||
		strings.Contains(readme, "pytest") ||
		strings.Contains(readme, "cargo test")

	if !hasTestCommand {
		results = append(results, ValidationResult{
			CheckName: "testing_instructions",
			Passed:    false,
			Error:     "No test command examples found",
			Details:   "README should show how to run tests",
			Timestamp: time.Now(),
		})
		return results
	}

	results = append(results, ValidationResult{
		CheckName: "testing_instructions",
		Passed:    true,
		Message:   "Testing instructions are documented",
		Timestamp: time.Now(),
	})

	return results
}

// ValidateCommonSenseFeatures checks that common sense features for the project type exist
func (v *UseCaseValidator) ValidateCommonSenseFeatures(ctx context.Context, spec *ChallengeSpec, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	switch spec.ID {
	case "notes-project-001":
		results = append(results, v.validateNotesCommonFeatures(resultDir)...)
	case "url-shortener-001":
		results = append(results, v.validateURLShortenerCommonFeatures(resultDir)...)
	case "cli-task-manager-001":
		results = append(results, v.validateCLITaskManagerCommonFeatures(resultDir)...)
	}

	return results
}

// validateNotesCommonFeatures checks for common sense features in a Notes API
func (v *UseCaseValidator) validateNotesCommonFeatures(resultDir string) []ValidationResult {
	results := []ValidationResult{}

	foundOperations := map[string]bool{
		"create": false,
		"read":   false,
		"update": false,
		"delete": false,
		"list":   false,
		"search": false,
	}

	// Walk through all Go files recursively
	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip test files
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileContent := strings.ToLower(string(content))

		// Check for CRUD operations (more lenient matching)
		if strings.Contains(fileContent, "createnote") || strings.Contains(fileContent, "post") || strings.Contains(fileContent, "\"create\"") {
			foundOperations["create"] = true
		}
		if strings.Contains(fileContent, "getnote") || (strings.Contains(fileContent, "get") && strings.Contains(fileContent, ":id")) {
			foundOperations["read"] = true
		}
		if strings.Contains(fileContent, "updatenote") || strings.Contains(fileContent, "put") {
			foundOperations["update"] = true
		}
		if strings.Contains(fileContent, "deletenote") || (strings.Contains(fileContent, "delete") && !strings.Contains(fileContent, "// delete")) {
			foundOperations["delete"] = true
		}
		if strings.Contains(fileContent, "listnotes") || (strings.Contains(fileContent, "get") && strings.Contains(fileContent, "/notes\"")) {
			foundOperations["list"] = true
		}
		if strings.Contains(fileContent, "searchnotes") || (strings.Contains(fileContent, "search") && !strings.Contains(fileContent, "// search")) {
			foundOperations["search"] = true
		}

		return nil
	})

	missingOperations := []string{}
	for op, found := range foundOperations {
		if !found {
			missingOperations = append(missingOperations, op)
		}
	}

	if len(missingOperations) > 0 {
		results = append(results, ValidationResult{
			CheckName: "common_sense_crud_operations",
			Passed:    false,
			Error:     "Missing common CRUD operations",
			Details:   fmt.Sprintf("Missing operations: %s", strings.Join(missingOperations, ", ")),
			Timestamp: time.Now(),
		})
	} else {
		results = append(results, ValidationResult{
			CheckName: "common_sense_crud_operations",
			Passed:    true,
			Message:   "All expected CRUD operations are implemented",
			Timestamp: time.Now(),
		})
	}

	// Check for data model with required fields
	results = append(results, v.validateNotesDataModel(resultDir))

	// Check for database configuration
	results = append(results, v.validateDatabaseSetup(resultDir))

	return results
}

// validateNotesDataModel checks if the Note model has essential fields
func (v *UseCaseValidator) validateNotesDataModel(resultDir string) ValidationResult {
	requiredFields := []string{"id", "title", "content", "created", "updated"}
	foundFields := make(map[string]bool)

	// Walk through all Go files in models directory
	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Only check files in models directory
		if !strings.Contains(path, "/models/") || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileContent := strings.ToLower(string(content))

		for _, field := range requiredFields {
			if strings.Contains(fileContent, field) {
				foundFields[field] = true
			}
		}

		return nil
	})

	if len(foundFields) < 3 {
		return ValidationResult{
			CheckName: "common_sense_data_model",
			Passed:    false,
			Error:     "Note model missing essential fields",
			Details:   "Note model should have at minimum: ID, Title, Content fields",
			Timestamp: time.Now(),
		}
	}

	return ValidationResult{
		CheckName: "common_sense_data_model",
		Passed:    true,
		Message:   "Note model has essential fields",
		Timestamp: time.Now(),
	}
}

// validateDatabaseSetup checks for database configuration
func (v *UseCaseValidator) validateDatabaseSetup(resultDir string) ValidationResult {
	found := false
	hasConnection := false

	// Walk through all Go files in db directory
	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Only check files in db directory
		if !strings.Contains(path, "/db/") || !strings.HasSuffix(path, ".go") {
			return nil
		}

		found = true

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileContent := strings.ToLower(string(content))

		if strings.Contains(fileContent, "connect") || strings.Contains(fileContent, "open") {
			hasConnection = true
		}

		return nil
	})

	if !found {
		return ValidationResult{
			CheckName: "common_sense_database",
			Passed:    false,
			Error:     "No database package found",
			Details:   "Project should have database connection setup",
			Timestamp: time.Now(),
		}
	}

	if !hasConnection {
		return ValidationResult{
			CheckName: "common_sense_database",
			Passed:    false,
			Error:     "No database connection logic found",
			Details:   "Database package should have connection setup",
			Timestamp: time.Now(),
		}
	}

	return ValidationResult{
		CheckName: "common_sense_database",
		Passed:    true,
		Message:   "Database connection setup found",
		Timestamp: time.Now(),
	}
}

// validateURLShortenerCommonFeatures checks common features for URL shortener
func (v *UseCaseValidator) validateURLShortenerCommonFeatures(resultDir string) []ValidationResult {
	var results []ValidationResult

	// Check for basic required files
	requiredFiles := []string{"main.go", "README.md"}
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(resultDir, file)); err != nil {
			results = append(results, ValidationResult{
				CheckName: fmt.Sprintf("required_file_%s", file),
				Passed:    false,
				Message:   fmt.Sprintf("Missing required file: %s", file),
				Timestamp: time.Now(),
			})
		} else {
			results = append(results, ValidationResult{
				CheckName: fmt.Sprintf("required_file_%s", file),
				Passed:    true,
				Message:   fmt.Sprintf("Found required file: %s", file),
				Timestamp: time.Now(),
			})
		}
	}

	// Check for basic URL shortener functionality in main.go
	mainFile := filepath.Join(resultDir, "main.go")
	if content, err := os.ReadFile(mainFile); err == nil {
		contentStr := string(content)
		
		// Check for HTTP server
		hasHTTPServer := strings.Contains(contentStr, "http.") || strings.Contains(contentStr, "gin.") || strings.Contains(contentStr, "mux.")
		results = append(results, ValidationResult{
			CheckName: "http_server",
			Passed:    hasHTTPServer,
			Message:   fmt.Sprintf("HTTP server implementation: %v", hasHTTPServer),
			Timestamp: time.Now(),
		})

		// Check for URL storage
		hasStorage := strings.Contains(contentStr, "map") || strings.Contains(contentStr, "database") || strings.Contains(contentStr, "redis")
		results = append(results, ValidationResult{
			CheckName: "url_storage",
			Passed:    hasStorage,
			Message:   fmt.Sprintf("URL storage mechanism: %v", hasStorage),
			Timestamp: time.Now(),
		})
	}

	// Check for README documentation
	readmeFile := filepath.Join(resultDir, "README.md")
	if content, err := os.ReadFile(readmeFile); err == nil {
		hasDocumentation := len(content) > 100 // Basic check for meaningful content
		results = append(results, ValidationResult{
			CheckName: "documentation",
			Passed:    hasDocumentation,
			Message:   fmt.Sprintf("README documentation: %v", hasDocumentation),
			Timestamp: time.Now(),
		})
	}

	return results
}

// validateCLITaskManagerCommonFeatures checks common features for CLI task manager
func (v *UseCaseValidator) validateCLITaskManagerCommonFeatures(resultDir string) []ValidationResult {
	results := []ValidationResult{}

	// Expected CLI commands/operations
	foundOperations := map[string]bool{
		"add":      false,
		"list":     false,
		"show":     false,
		"complete": false,
		"delete":   false,
		"update":   false,
	}

	// Expected task attributes
	foundAttributes := map[string]bool{
		"id":          false,
		"description": false,
		"priority":    false,
		"status":      false,
		"due":         false,
		"created":     false,
	}

	// Walk through all Go files recursively
	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip test files
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileContent := strings.ToLower(string(content))

		// Check for CLI commands (common patterns with Cobra)
		if strings.Contains(fileContent, "addcmd") || strings.Contains(fileContent, "\"add\"") ||
			strings.Contains(fileContent, "add command") || strings.Contains(fileContent, "cmd/add") {
			foundOperations["add"] = true
		}
		if strings.Contains(fileContent, "listcmd") || strings.Contains(fileContent, "\"list\"") ||
			strings.Contains(fileContent, "list command") || strings.Contains(fileContent, "cmd/list") {
			foundOperations["list"] = true
		}
		if strings.Contains(fileContent, "showcmd") || strings.Contains(fileContent, "\"show\"") ||
			strings.Contains(fileContent, "show command") || strings.Contains(fileContent, "cmd/show") ||
			strings.Contains(fileContent, "gettask") {
			foundOperations["show"] = true
		}
		if strings.Contains(fileContent, "completecmd") || strings.Contains(fileContent, "\"complete\"") ||
			strings.Contains(fileContent, "complete command") || strings.Contains(fileContent, "cmd/complete") ||
			strings.Contains(fileContent, "\"done\"") || strings.Contains(fileContent, "markcomplete") {
			foundOperations["complete"] = true
		}
		if strings.Contains(fileContent, "deletecmd") || strings.Contains(fileContent, "\"delete\"") ||
			strings.Contains(fileContent, "delete command") || strings.Contains(fileContent, "cmd/delete") ||
			strings.Contains(fileContent, "\"remove\"") || strings.Contains(fileContent, "removetask") {
			foundOperations["delete"] = true
		}
		if strings.Contains(fileContent, "updatecmd") || strings.Contains(fileContent, "\"update\"") ||
			strings.Contains(fileContent, "update command") || strings.Contains(fileContent, "cmd/update") ||
			strings.Contains(fileContent, "\"edit\"") || strings.Contains(fileContent, "edittask") {
			foundOperations["update"] = true
		}

		// Check for task attributes (struct fields or JSON tags)
		if strings.Contains(fileContent, "id") || strings.Contains(fileContent, "taskid") {
			foundAttributes["id"] = true
		}
		if strings.Contains(fileContent, "description") || strings.Contains(fileContent, "title") ||
			strings.Contains(fileContent, "content") || strings.Contains(fileContent, "text") {
			foundAttributes["description"] = true
		}
		if strings.Contains(fileContent, "priority") {
			foundAttributes["priority"] = true
		}
		if strings.Contains(fileContent, "status") || strings.Contains(fileContent, "completed") ||
			strings.Contains(fileContent, "done") || strings.Contains(fileContent, "pending") {
			foundAttributes["status"] = true
		}
		if strings.Contains(fileContent, "due") || strings.Contains(fileContent, "deadline") ||
			strings.Contains(fileContent, "duedate") {
			foundAttributes["due"] = true
		}
		if strings.Contains(fileContent, "created") || strings.Contains(fileContent, "createdat") ||
			strings.Contains(fileContent, "timestamp") {
			foundAttributes["created"] = true
		}

		return nil
	})

	// Validate commands - require at least 4 of 6 core commands
	missingOperations := []string{}
	foundOpsCount := 0
	for op, found := range foundOperations {
		if found {
			foundOpsCount++
		} else {
			missingOperations = append(missingOperations, op)
		}
	}

	if foundOpsCount >= 4 {
		results = append(results, ValidationResult{
			CheckName: "common_sense_cli_commands",
			Passed:    true,
			Message:   fmt.Sprintf("CLI has %d/6 expected commands", foundOpsCount),
			Timestamp: time.Now(),
		})
	} else {
		results = append(results, ValidationResult{
			CheckName: "common_sense_cli_commands",
			Passed:    false,
			Error:     "Missing required CLI commands",
			Details:   fmt.Sprintf("Found %d/6 commands. Missing: %s", foundOpsCount, strings.Join(missingOperations, ", ")),
			Timestamp: time.Now(),
		})
	}

	// Validate task attributes - require at least 4 of 6 core attributes
	missingAttributes := []string{}
	foundAttrCount := 0
	for attr, found := range foundAttributes {
		if found {
			foundAttrCount++
		} else {
			missingAttributes = append(missingAttributes, attr)
		}
	}

	if foundAttrCount >= 4 {
		results = append(results, ValidationResult{
			CheckName: "common_sense_task_model",
			Passed:    true,
			Message:   fmt.Sprintf("Task model has %d/6 expected attributes", foundAttrCount),
			Timestamp: time.Now(),
		})
	} else {
		results = append(results, ValidationResult{
			CheckName: "common_sense_task_model",
			Passed:    false,
			Error:     "Task model missing expected attributes",
			Details:   fmt.Sprintf("Found %d/6 attributes. Missing: %s", foundAttrCount, strings.Join(missingAttributes, ", ")),
			Timestamp: time.Now(),
		})
	}

	// Check for Cobra framework usage
	results = append(results, v.validateCobraUsage(resultDir))

	// Check for persistence (JSON or SQLite)
	results = append(results, v.validateCLIPersistence(resultDir))

	return results
}

// validateCobraUsage checks if the project uses Cobra framework
func (v *UseCaseValidator) validateCobraUsage(resultDir string) ValidationResult {
	goModPath := filepath.Join(resultDir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ValidationResult{
			CheckName: "cobra_framework",
			Passed:    false,
			Error:     "Could not read go.mod",
			Timestamp: time.Now(),
		}
	}

	if strings.Contains(string(content), "github.com/spf13/cobra") {
		return ValidationResult{
			CheckName: "cobra_framework",
			Passed:    true,
			Message:   "Uses Cobra framework as required",
			Timestamp: time.Now(),
		}
	}

	// Also check for other CLI frameworks as acceptable alternatives
	contentStr := string(content)
	if strings.Contains(contentStr, "github.com/urfave/cli") ||
		strings.Contains(contentStr, "github.com/alecthomas/kong") {
		return ValidationResult{
			CheckName: "cobra_framework",
			Passed:    true,
			Message:   "Uses alternative CLI framework (acceptable)",
			Timestamp: time.Now(),
		}
	}

	return ValidationResult{
		CheckName: "cobra_framework",
		Passed:    false,
		Error:     "No CLI framework found in go.mod",
		Details:   "Expected github.com/spf13/cobra or similar CLI framework",
		Timestamp: time.Now(),
	}
}

// validateCLIPersistence checks for data persistence implementation
func (v *UseCaseValidator) validateCLIPersistence(resultDir string) ValidationResult {
	hasJSONPersistence := false
	hasSQLitePersistence := false

	// Check go.mod for sqlite
	goModPath := filepath.Join(resultDir, "go.mod")
	if content, err := os.ReadFile(goModPath); err == nil {
		contentStr := strings.ToLower(string(content))
		if strings.Contains(contentStr, "sqlite") || strings.Contains(contentStr, "go-sqlite") ||
			strings.Contains(contentStr, "modernc.org/sqlite") {
			hasSQLitePersistence = true
		}
	}

	// Walk through Go files for persistence patterns
	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := strings.ToLower(string(content))

		// JSON persistence patterns
		if strings.Contains(contentStr, "json.marshal") || strings.Contains(contentStr, "json.unmarshal") ||
			strings.Contains(contentStr, "encoding/json") || strings.Contains(contentStr, ".json\"") {
			if strings.Contains(contentStr, "os.writefile") || strings.Contains(contentStr, "os.readfile") ||
				strings.Contains(contentStr, "ioutil.writefile") || strings.Contains(contentStr, "ioutil.readfile") {
				hasJSONPersistence = true
			}
		}

		// SQLite patterns
		if strings.Contains(contentStr, "sql.open") || strings.Contains(contentStr, "sqlite") ||
			strings.Contains(contentStr, "database/sql") {
			hasSQLitePersistence = true
		}

		return nil
	})

	if hasJSONPersistence || hasSQLitePersistence {
		method := "JSON file"
		if hasSQLitePersistence {
			method = "SQLite database"
		}
		return ValidationResult{
			CheckName: "data_persistence",
			Passed:    true,
			Message:   fmt.Sprintf("Data persistence implemented using %s", method),
			Timestamp: time.Now(),
		}
	}

	return ValidationResult{
		CheckName: "data_persistence",
		Passed:    false,
		Error:     "No data persistence found",
		Details:   "Expected JSON file or SQLite database for task storage",
		Timestamp: time.Now(),
	}
}
