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
	// TODO: Implement CLI task manager common features validation
	return []ValidationResult{
		{
			CheckName: "common_sense_features",
			Passed:    false,
			Message:   "CLI task manager common features validation not yet implemented",
			Timestamp: time.Now(),
		},
	}
}
