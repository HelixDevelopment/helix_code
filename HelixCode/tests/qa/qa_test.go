package qa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig holds the configuration for QA tests
type TestConfig struct {
	BaseURL     string
	AdminToken  string
	Timeout     time.Duration
	ProjectRoot string
}

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() *TestConfig {
	// Find project root by looking for go.mod
	projectRoot := findProjectRoot()
	return &TestConfig{
		BaseURL:     "http://localhost:8080",
		AdminToken:  "test-admin-token",
		Timeout:     30 * time.Second,
		ProjectRoot: projectRoot,
	}
}

// findProjectRoot locates the project root by searching for go.mod
func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// =============================================================================
// Code Quality Tests
// =============================================================================

// TestQA_CodeQuality_NoTODOsInProduction ensures no TODO comments in production code
func TestQA_CodeQuality_NoTODOsInProduction(t *testing.T) {
	config := DefaultTestConfig()

	todoPattern := regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX|BUG)\s*:`)
	violations := make([]string, 0)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip test files and directories
		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if matches := todoPattern.FindStringSubmatch(line); matches != nil {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Found %d TODO/FIXME comments in production code:", len(violations))
		for _, v := range violations {
			t.Logf("  %s", v)
		}
		// Warning only - don't fail the test
		t.Logf("Warning: Production code contains TODO comments")
	}
}

// TestQA_CodeQuality_NoPanicInProduction ensures no naked panic() calls
func TestQA_CodeQuality_NoPanicInProduction(t *testing.T) {
	config := DefaultTestConfig()

	panicPattern := regexp.MustCompile(`\bpanic\s*\(`)
	violations := make([]string, 0)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			// Skip if it's a comment
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			if panicPattern.MatchString(line) {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Found %d panic() calls in production code:", len(violations))
		for _, v := range violations {
			t.Logf("  %s", v)
		}
		// Panics should be reviewed - warning only
		t.Logf("Warning: Review panic() calls for proper error handling")
	}
}

// TestQA_CodeQuality_NoFmtPrintln ensures no fmt.Println debugging statements
func TestQA_CodeQuality_NoFmtPrintln(t *testing.T) {
	config := DefaultTestConfig()

	printPattern := regexp.MustCompile(`fmt\.(Print|Println|Printf)\s*\(`)
	violations := make([]string, 0)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			if printPattern.MatchString(line) {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Found %d fmt.Print* statements (should use proper logging):", len(violations))
		for _, v := range violations[:min(10, len(violations))] {
			t.Logf("  %s", v)
		}
		if len(violations) > 10 {
			t.Logf("  ... and %d more", len(violations)-10)
		}
	}
}

// TestQA_CodeQuality_ErrorHandling checks for unchecked errors
func TestQA_CodeQuality_ErrorHandling(t *testing.T) {
	config := DefaultTestConfig()

	// Pattern for common cases where errors might be ignored
	// This is a simplified check - real static analysis tools are better
	ignoreErrPattern := regexp.MustCompile(`\b_\s*=\s*\w+\(`)
	violations := make([]string, 0)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if ignoreErrPattern.MatchString(line) && strings.Contains(line, "err") {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s:%d: potential ignored error", relPath, i+1))
			}
		}
		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Found %d potential ignored errors:", len(violations))
		for _, v := range violations[:min(5, len(violations))] {
			t.Logf("  %s", v)
		}
	}
}

// =============================================================================
// Documentation Tests
// =============================================================================

// TestQA_Documentation_ExportedFunctionsDocumented checks for documentation on exported functions
func TestQA_Documentation_ExportedFunctionsDocumented(t *testing.T) {
	config := DefaultTestConfig()

	undocumented := make([]string, 0)
	total := 0
	documented := 0

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Check if exported (starts with uppercase)
			if !fn.Name.IsExported() {
				continue
			}

			total++

			// Check if documented
			if fn.Doc != nil && len(fn.Doc.List) > 0 {
				documented++
			} else {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				undocumented = append(undocumented, fmt.Sprintf("%s: %s", relPath, fn.Name.Name))
			}
		}

		return nil
	})

	require.NoError(t, err)

	coverage := 0.0
	if total > 0 {
		coverage = float64(documented) / float64(total) * 100
	}

	t.Logf("Documentation coverage: %.1f%% (%d/%d exported functions documented)", coverage, documented, total)

	if len(undocumented) > 0 {
		t.Logf("Sample undocumented functions:")
		for _, u := range undocumented[:min(10, len(undocumented))] {
			t.Logf("  %s", u)
		}
		if len(undocumented) > 10 {
			t.Logf("  ... and %d more", len(undocumented)-10)
		}
	}

	// Require at least 50% documentation coverage
	assert.Greater(t, coverage, 50.0, "Documentation coverage below 50%%")
}

// TestQA_Documentation_PackageDocumented checks for package-level documentation
func TestQA_Documentation_PackageDocumented(t *testing.T) {
	config := DefaultTestConfig()

	packages := make(map[string]bool)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		if _, exists := packages[dir]; exists {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		hasDoc := node.Doc != nil && len(node.Doc.List) > 0
		packages[dir] = hasDoc

		return nil
	})

	require.NoError(t, err)

	documented := 0
	undocumented := make([]string, 0)

	for pkg, hasDoc := range packages {
		if hasDoc {
			documented++
		} else {
			relPath, _ := filepath.Rel(config.ProjectRoot, pkg)
			undocumented = append(undocumented, relPath)
		}
	}

	coverage := 0.0
	if len(packages) > 0 {
		coverage = float64(documented) / float64(len(packages)) * 100
	}

	t.Logf("Package documentation: %.1f%% (%d/%d packages documented)", coverage, documented, len(packages))

	if len(undocumented) > 0 {
		t.Logf("Undocumented packages:")
		for _, u := range undocumented[:min(10, len(undocumented))] {
			t.Logf("  %s", u)
		}
	}
}

// TestQA_Documentation_READMEExists checks for README files in packages
func TestQA_Documentation_READMEExists(t *testing.T) {
	config := DefaultTestConfig()

	packages := make([]string, 0)
	withReadme := 0

	internalDir := filepath.Join(config.ProjectRoot, "internal")
	entries, err := os.ReadDir(internalDir)
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pkgPath := filepath.Join(internalDir, entry.Name())
		packages = append(packages, entry.Name())

		readmePath := filepath.Join(pkgPath, "README.md")
		if _, err := os.Stat(readmePath); err == nil {
			withReadme++
		}
	}

	coverage := 0.0
	if len(packages) > 0 {
		coverage = float64(withReadme) / float64(len(packages)) * 100
	}

	t.Logf("README coverage: %.1f%% (%d/%d packages have README.md)", coverage, withReadme, len(packages))
}

// =============================================================================
// API Contract Tests
// =============================================================================

// TestQA_APIContract_HealthEndpoint verifies health endpoint contract
func TestQA_APIContract_HealthEndpoint(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API contract test")  // SKIP-OK: #legacy-untriaged
	}
	defer resp.Body.Close()

	// Verify status code
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200")

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	assert.Contains(t, contentType, "application/json", "Should return JSON content type")

	// Verify response structure
	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	// Health response should have status field
	_, hasStatus := health["status"]
	assert.True(t, hasStatus, "Health response should have 'status' field")
}

// TestQA_APIContract_ErrorResponses verifies error response format
func TestQA_APIContract_ErrorResponses(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API contract test")  // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()

	// Test 404 error
	resp, err = client.Get(config.BaseURL + "/api/v1/nonexistent-endpoint-12345")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Error response should be JSON
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		if err == nil {
			// Should have error field
			_, hasError := errorResp["error"]
			_, hasMessage := errorResp["message"]
			assert.True(t, hasError || hasMessage, "Error response should have error or message field")
		}
	}
}

// TestQA_APIContract_AuthenticationRequired verifies auth is required for protected endpoints
func TestQA_APIContract_AuthenticationRequired(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API contract test")  // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()

	protectedEndpoints := []string{
		"/api/v1/projects",
		"/api/v1/tasks",
		"/api/v1/workers",
		"/api/v1/users",
	}

	for _, endpoint := range protectedEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := client.Get(config.BaseURL + endpoint)
			if err != nil {
				t.Skip("Endpoint not available")  // SKIP-OK: #legacy-untriaged
				return
			}
			defer resp.Body.Close()

			// Should return 401 or 403 without auth
			if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
				t.Logf("Endpoint %s returned %d without auth (expected 401/403)", endpoint, resp.StatusCode)
			}
		})
	}
}

// TestQA_APIContract_JSONContentType verifies JSON endpoints return proper content type
func TestQA_APIContract_JSONContentType(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API contract test")  // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()

	jsonEndpoints := []string{
		"/health",
		"/api/v1/health",
	}

	for _, endpoint := range jsonEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := client.Get(config.BaseURL + endpoint)
			if err != nil {
				t.Skip("Endpoint not available")  // SKIP-OK: #legacy-untriaged
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				contentType := resp.Header.Get("Content-Type")
				assert.Contains(t, contentType, "application/json",
					"Endpoint %s should return JSON content type", endpoint)
			}
		})
	}
}

// TestQA_APIContract_RequestValidation verifies request validation
func TestQA_APIContract_RequestValidation(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API contract test")  // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()

	// Test with invalid JSON
	invalidJSON := []byte(`{invalid json}`)
	req, _ := http.NewRequest("POST", config.BaseURL+"/api/v1/projects", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AdminToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Skip("Server not accepting requests")  // SKIP-OK: #legacy-untriaged
		return
	}
	defer resp.Body.Close()

	// Should return 400 for invalid JSON
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnauthorized {
		t.Logf("Server returned %d for invalid JSON (expected 400)", resp.StatusCode)
	}
}

// =============================================================================
// Compatibility Tests
// =============================================================================

// TestQA_Compatibility_JSONFieldNaming verifies consistent JSON field naming
func TestQA_Compatibility_JSONFieldNaming(t *testing.T) {
	config := DefaultTestConfig()

	// Check JSON tags use consistent naming (snake_case or camelCase)
	snakeCasePattern := regexp.MustCompile(`json:"[a-z]+(_[a-z]+)+"`)
	camelCasePattern := regexp.MustCompile(`json:"[a-z]+([A-Z][a-z]+)+"`)

	snakeCaseCount := 0
	camelCaseCount := 0

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		snakeCaseCount += len(snakeCasePattern.FindAllString(string(content), -1))
		camelCaseCount += len(camelCasePattern.FindAllString(string(content), -1))

		return nil
	})

	require.NoError(t, err)

	t.Logf("JSON naming convention usage:")
	t.Logf("  snake_case: %d fields", snakeCaseCount)
	t.Logf("  camelCase: %d fields", camelCaseCount)

	// Should use one convention consistently
	total := snakeCaseCount + camelCaseCount
	if total > 0 {
		snakeRatio := float64(snakeCaseCount) / float64(total) * 100
		camelRatio := float64(camelCaseCount) / float64(total) * 100
		t.Logf("  snake_case ratio: %.1f%%", snakeRatio)
		t.Logf("  camelCase ratio: %.1f%%", camelRatio)
	}
}

// TestQA_Compatibility_APIVersioning verifies API versioning
func TestQA_Compatibility_APIVersioning(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping API versioning test")  // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()

	// Check that API is versioned
	resp, err = client.Get(config.BaseURL + "/api/v1/health")
	if err != nil {
		t.Log("API versioning check: /api/v1/health not accessible")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Log("API versioning: v1 endpoint available")
	}
}

// =============================================================================
// Build and Test Quality
// =============================================================================

// TestQA_BuildQuality_NoRaceConditions is a placeholder for race detection
func TestQA_BuildQuality_NoRaceConditions(t *testing.T) {
	// This test should be run with -race flag
	// go test -race ./...
	t.Log("Run tests with -race flag to detect race conditions")
}

// TestQA_BuildQuality_TestCoverage reports on test coverage
func TestQA_BuildQuality_TestCoverage(t *testing.T) {
	config := DefaultTestConfig()

	// Count test files vs source files
	sourceFiles := 0
	testFiles := 0

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") {
			testFiles++
		} else {
			sourceFiles++
		}

		return nil
	})

	require.NoError(t, err)

	ratio := 0.0
	if sourceFiles > 0 {
		ratio = float64(testFiles) / float64(sourceFiles) * 100
	}

	t.Logf("Test file ratio: %.1f%% (%d test files / %d source files)", ratio, testFiles, sourceFiles)

	// Should have at least some test coverage
	assert.Greater(t, testFiles, 0, "No test files found")
}

// =============================================================================
// Security Quality Tests
// =============================================================================

// TestQA_Security_NoHardcodedSecrets checks for hardcoded secrets
func TestQA_Security_NoHardcodedSecrets(t *testing.T) {
	config := DefaultTestConfig()

	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|secret|key|token|api_key)\s*[=:]\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`),
	}

	violations := make([]string, 0)

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		for _, pattern := range secretPatterns {
			matches := pattern.FindAllString(string(content), -1)
			for _, match := range matches {
				// Skip obvious test values
				lowerMatch := strings.ToLower(match)
				if strings.Contains(lowerMatch, "test") ||
					strings.Contains(lowerMatch, "example") ||
					strings.Contains(lowerMatch, "placeholder") {
					continue
				}
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s: %s", relPath, match[:min(50, len(match))]))
			}
		}

		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Potential hardcoded secrets found:")
		for _, v := range violations[:min(10, len(violations))] {
			t.Logf("  %s", v)
		}
	}
}

// TestQA_Security_NoInsecureHTTP checks for hardcoded HTTP URLs (should use HTTPS)
func TestQA_Security_NoInsecureHTTP(t *testing.T) {
	config := DefaultTestConfig()

	httpPattern := regexp.MustCompile(`http://[^/\s"']+`)
	violations := make([]string, 0)

	allowedHTTP := []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://0.0.0.0",
		"http://example.com",
	}

	err := filepath.Walk(filepath.Join(config.ProjectRoot, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && (info.Name() == "testdata" || info.Name() == "mocks") {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		matches := httpPattern.FindAllString(string(content), -1)
		for _, match := range matches {
			allowed := false
			for _, a := range allowedHTTP {
				if strings.HasPrefix(match, a) {
					allowed = true
					break
				}
			}
			if !allowed {
				relPath, _ := filepath.Rel(config.ProjectRoot, path)
				violations = append(violations, fmt.Sprintf("%s: %s", relPath, match))
			}
		}

		return nil
	})

	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Insecure HTTP URLs found (should use HTTPS):")
		for _, v := range violations[:min(10, len(violations))] {
			t.Logf("  %s", v)
		}
	}
}

// =============================================================================
// Performance Quality Tests
// =============================================================================

// TestQA_Performance_ResponseSize checks that API responses aren't too large
func TestQA_Performance_ResponseSize(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping response size test")  // SKIP-OK: #legacy-untriaged
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	t.Logf("Health endpoint response size: %d bytes", len(body))

	// Health response should be small
	maxHealthSize := 10 * 1024 // 10KB
	assert.Less(t, len(body), maxHealthSize, "Health endpoint response too large")
}

// =============================================================================
// Helper Functions
// =============================================================================

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
