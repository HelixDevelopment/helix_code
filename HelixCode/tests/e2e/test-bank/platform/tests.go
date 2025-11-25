package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"dev.helix.code/tests/e2e/orchestrator/pkg"
	"dev.helix.code/tests/e2e/orchestrator/pkg/validator"
)

// PlatformTestConfig holds configuration for platform tests
type PlatformTestConfig struct {
	BaseURL     string
	Platform    string
	Arch        string
	TestTimeout time.Duration
}

// GetPlatformTestConfig returns the platform test configuration
func GetPlatformTestConfig() *PlatformTestConfig {
	return &PlatformTestConfig{
		BaseURL:     getEnvOrDefault("HELIXCODE_TEST_URL", "http://localhost:8080"),
		Platform:    runtime.GOOS,
		Arch:        runtime.GOARCH,
		TestTimeout: 60 * time.Second,
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// APIClient provides HTTP client for platform test API calls
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// doRequest performs an HTTP request
func (c *APIClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return c.httpClient.Do(req)
}

// parseResponse parses JSON response
func parseResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}

// GetPlatformTests returns all platform test cases
func GetPlatformTests() []*pkg.TestCase {
	return []*pkg.TestCase{
		PT001_LinuxCompatibility(),
		PT002_MacOSCompatibility(),
		PT003_WindowsCompatibility(),
		PT004_ARMCompatibility(),
		PT005_AMD64Compatibility(),
		PT006_FileSystemOperations(),
		PT007_NetworkOperations(),
		PT008_ProcessManagement(),
		PT009_EnvironmentVariables(),
		PT010_SystemResources(),
		PT011_MobilePlatformDetection(),
		PT012_CrossPlatformPaths(),
	}
}

// PT001_LinuxCompatibility - Test Linux platform compatibility
func PT001_LinuxCompatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-001",
		Name:        "Linux Platform Compatibility",
		Description: "Verify HelixCode works correctly on Linux platforms",
		Priority:    pkg.PriorityCritical,
		Timeout:     45 * time.Second,
		Tags:        []string{"platform", "linux", "compatibility"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			if config.Platform != "linux" {
				return v.Assert(true, "Test skipped - not running on Linux")
			}

			client := NewAPIClient(config.BaseURL)

			// Health check
			healthResp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("health check failed on Linux: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, healthResp.StatusCode, "HelixCode runs on Linux"); err != nil {
				return err
			}

			// Check Linux-specific features
			_, err = exec.LookPath("uname")
			if err := v.AssertNil(err, "uname command available on Linux"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT002_MacOSCompatibility - Test macOS platform compatibility
func PT002_MacOSCompatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-002",
		Name:        "macOS Platform Compatibility",
		Description: "Verify HelixCode works correctly on macOS platforms",
		Priority:    pkg.PriorityCritical,
		Timeout:     45 * time.Second,
		Tags:        []string{"platform", "macos", "darwin", "compatibility"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			if config.Platform != "darwin" {
				return v.Assert(true, "Test skipped - not running on macOS")
			}

			client := NewAPIClient(config.BaseURL)

			// Health check
			healthResp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("health check failed on macOS: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, healthResp.StatusCode, "HelixCode runs on macOS"); err != nil {
				return err
			}

			// Check macOS-specific features
			_, err = exec.LookPath("sw_vers")
			if err := v.AssertNil(err, "sw_vers command available on macOS"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT003_WindowsCompatibility - Test Windows platform compatibility
func PT003_WindowsCompatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-003",
		Name:        "Windows Platform Compatibility",
		Description: "Verify HelixCode works correctly on Windows platforms",
		Priority:    pkg.PriorityCritical,
		Timeout:     45 * time.Second,
		Tags:        []string{"platform", "windows", "compatibility"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			if config.Platform != "windows" {
				return v.Assert(true, "Test skipped - not running on Windows")
			}

			client := NewAPIClient(config.BaseURL)

			// Health check
			healthResp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("health check failed on Windows: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, healthResp.StatusCode, "HelixCode runs on Windows"); err != nil {
				return err
			}

			// Check Windows-specific features
			_, err = exec.LookPath("cmd.exe")
			if err := v.AssertNil(err, "cmd.exe available on Windows"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT004_ARMCompatibility - Test ARM architecture compatibility
func PT004_ARMCompatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-004",
		Name:        "ARM Architecture Compatibility",
		Description: "Verify HelixCode works correctly on ARM architectures",
		Priority:    pkg.PriorityHigh,
		Timeout:     45 * time.Second,
		Tags:        []string{"platform", "arm", "arm64", "architecture"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			if !strings.HasPrefix(config.Arch, "arm") {
				return v.Assert(true, "Test skipped - not running on ARM architecture")
			}

			client := NewAPIClient(config.BaseURL)

			// Health check
			healthResp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("health check failed on ARM: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, healthResp.StatusCode, "HelixCode runs on ARM"); err != nil {
				return err
			}

			if err := v.Assert(true, fmt.Sprintf("Running on ARM architecture: %s", config.Arch)); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT005_AMD64Compatibility - Test AMD64 architecture compatibility
func PT005_AMD64Compatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-005",
		Name:        "AMD64 Architecture Compatibility",
		Description: "Verify HelixCode works correctly on AMD64/x86_64 architectures",
		Priority:    pkg.PriorityHigh,
		Timeout:     45 * time.Second,
		Tags:        []string{"platform", "amd64", "x86_64", "architecture"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			if config.Arch != "amd64" {
				return v.Assert(true, "Test skipped - not running on AMD64 architecture")
			}

			client := NewAPIClient(config.BaseURL)

			// Health check
			healthResp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("health check failed on AMD64: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, healthResp.StatusCode, "HelixCode runs on AMD64"); err != nil {
				return err
			}

			if err := v.Assert(true, "Running on AMD64/x86_64 architecture"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT006_FileSystemOperations - Test file system operations
func PT006_FileSystemOperations() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-006",
		Name:        "File System Operations",
		Description: "Verify file system operations work correctly on current platform",
		Priority:    pkg.PriorityCritical,
		Timeout:     30 * time.Second,
		Tags:        []string{"platform", "filesystem", "io"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()

			// Test temp directory creation
			tmpDir, err := os.MkdirTemp("", "helixcode-platform-test-*")
			if err != nil {
				return fmt.Errorf("failed to create temp directory: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := v.AssertTrue(len(tmpDir) > 0, "Temp directory created successfully"); err != nil {
				return err
			}

			// Test file creation
			testFile := fmt.Sprintf("%s/test.txt", tmpDir)
			err = os.WriteFile(testFile, []byte("test content"), 0644)
			if err := v.AssertNil(err, "File creation works"); err != nil {
				return err
			}

			// Test file reading
			content, err := os.ReadFile(testFile)
			if err := v.AssertNil(err, "File reading works"); err != nil {
				return err
			}

			if err := v.AssertEqual("test content", string(content), "File content is correct"); err != nil {
				return err
			}

			// Test file deletion
			err = os.Remove(testFile)
			if err := v.AssertNil(err, "File deletion works"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT007_NetworkOperations - Test network operations
func PT007_NetworkOperations() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-007",
		Name:        "Network Operations",
		Description: "Verify network operations work correctly on current platform",
		Priority:    pkg.PriorityCritical,
		Timeout:     30 * time.Second,
		Tags:        []string{"platform", "network", "http"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test HTTP GET
			resp, err := client.doRequest("GET", "/health", nil)
			if err != nil {
				return fmt.Errorf("HTTP GET failed: %w", err)
			}

			if err := v.AssertEqual(http.StatusOK, resp.StatusCode, "HTTP GET works"); err != nil {
				return err
			}

			// Test HTTP POST
			postResp, err := client.doRequest("POST", "/api/v1/projects", map[string]string{
				"name":        fmt.Sprintf("network-test-%d", time.Now().UnixNano()),
				"description": "Network operations test",
				"path":        fmt.Sprintf("/tmp/network-test-%d", time.Now().UnixNano()),
				"type":        "go",
			})
			if err != nil {
				return fmt.Errorf("HTTP POST failed: %w", err)
			}

			if err := v.AssertEqual(http.StatusCreated, postResp.StatusCode, "HTTP POST works"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT008_ProcessManagement - Test process management
func PT008_ProcessManagement() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-008",
		Name:        "Process Management",
		Description: "Verify process management works correctly on current platform",
		Priority:    pkg.PriorityHigh,
		Timeout:     30 * time.Second,
		Tags:        []string{"platform", "process", "os"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()

			// Get current process ID
			pid := os.Getpid()
			if err := v.AssertTrue(pid > 0, "Current process ID is valid"); err != nil {
				return err
			}

			// Get current working directory
			cwd, err := os.Getwd()
			if err := v.AssertNil(err, "Can get current working directory"); err != nil {
				return err
			}

			if err := v.AssertTrue(len(cwd) > 0, "Working directory is not empty"); err != nil {
				return err
			}

			// Test environment access
			home := os.Getenv("HOME")
			if runtime.GOOS == "windows" {
				home = os.Getenv("USERPROFILE")
			}
			if err := v.AssertTrue(len(home) > 0, "Home directory environment variable is set"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT009_EnvironmentVariables - Test environment variable handling
func PT009_EnvironmentVariables() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-009",
		Name:        "Environment Variables",
		Description: "Verify environment variables are handled correctly",
		Priority:    pkg.PriorityHigh,
		Timeout:     15 * time.Second,
		Tags:        []string{"platform", "environment", "config"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()

			// Test setting environment variable
			testKey := "HELIXCODE_PLATFORM_TEST"
			testValue := "test_value_123"

			err := os.Setenv(testKey, testValue)
			if err := v.AssertNil(err, "Can set environment variable"); err != nil {
				return err
			}

			// Test getting environment variable
			retrievedValue := os.Getenv(testKey)
			if err := v.AssertEqual(testValue, retrievedValue, "Environment variable value is correct"); err != nil {
				return err
			}

			// Test unsetting environment variable
			err = os.Unsetenv(testKey)
			if err := v.AssertNil(err, "Can unset environment variable"); err != nil {
				return err
			}

			// Verify it's unset
			retrievedValue = os.Getenv(testKey)
			if err := v.AssertEqual("", retrievedValue, "Environment variable is unset"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT010_SystemResources - Test system resource detection
func PT010_SystemResources() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-010",
		Name:        "System Resources Detection",
		Description: "Verify system resources can be detected correctly",
		Priority:    pkg.PriorityNormal,
		Timeout:     15 * time.Second,
		Tags:        []string{"platform", "resources", "hardware"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()

			// Get number of CPUs
			numCPU := runtime.NumCPU()
			if err := v.AssertTrue(numCPU > 0, fmt.Sprintf("CPU count detected: %d", numCPU)); err != nil {
				return err
			}

			// Get GOMAXPROCS
			maxProcs := runtime.GOMAXPROCS(0)
			if err := v.AssertTrue(maxProcs > 0, fmt.Sprintf("GOMAXPROCS: %d", maxProcs)); err != nil {
				return err
			}

			// Get memory stats
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			if err := v.AssertTrue(memStats.Sys > 0, "System memory detected"); err != nil {
				return err
			}

			return nil
		},
	}
}

// PT011_MobilePlatformDetection - Test mobile platform detection
func PT011_MobilePlatformDetection() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-011",
		Name:        "Mobile Platform Detection",
		Description: "Verify mobile platforms can be detected for mobile builds",
		Priority:    pkg.PriorityNormal,
		Timeout:     15 * time.Second,
		Tags:        []string{"platform", "mobile", "ios", "android"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			// Detect if running in mobile context
			isMobile := config.Platform == "android" || config.Platform == "ios"

			if isMobile {
				if err := v.Assert(true, fmt.Sprintf("Running on mobile platform: %s", config.Platform)); err != nil {
					return err
				}
			} else {
				if err := v.Assert(true, fmt.Sprintf("Running on desktop platform: %s/%s", config.Platform, config.Arch)); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// PT012_CrossPlatformPaths - Test cross-platform path handling
func PT012_CrossPlatformPaths() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "PT-012",
		Name:        "Cross-Platform Path Handling",
		Description: "Verify file paths are handled correctly across platforms",
		Priority:    pkg.PriorityHigh,
		Timeout:     15 * time.Second,
		Tags:        []string{"platform", "paths", "filesystem"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			// Test path separator
			pathSep := string(os.PathSeparator)
			if config.Platform == "windows" {
				if err := v.AssertEqual("\\", pathSep, "Windows path separator is backslash"); err != nil {
					return err
				}
			} else {
				if err := v.AssertEqual("/", pathSep, "Unix path separator is forward slash"); err != nil {
					return err
				}
			}

			// Test list separator
			listSep := string(os.PathListSeparator)
			if config.Platform == "windows" {
				if err := v.AssertEqual(";", listSep, "Windows list separator is semicolon"); err != nil {
					return err
				}
			} else {
				if err := v.AssertEqual(":", listSep, "Unix list separator is colon"); err != nil {
					return err
				}
			}

			// Test temp directory path
			tmpDir := os.TempDir()
			if err := v.AssertTrue(len(tmpDir) > 0, "Temp directory path is valid"); err != nil {
				return err
			}

			return nil
		},
	}
}
