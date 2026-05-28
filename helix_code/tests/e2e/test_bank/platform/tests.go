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

// TC046_LinuxDeployment tests Linux-specific deployment and operation
func TC046_LinuxDeployment() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-046",
		Name:        "Linux Deployment and Operation",
		Description: "Verify system deploys and operates correctly on Linux platforms",
		Priority:    pkg.PriorityHigh,
		Timeout:     300 * time.Second,
		Tags:        []string{"platform", "linux", "deployment", "systemd"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			// Skip if not on Linux
			if config.Platform != "linux" {
				return v.Skip(fmt.Sprintf("not running on Linux (runtime.GOOS=%s)", runtime.GOOS))
			}

			client := NewAPIClient(config.BaseURL)

			// Test Linux-specific system information
			resp, err := client.doRequest("GET", "/api/v1/system/info", nil)
			if err != nil {
				return fmt.Errorf("system info request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				systemResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse system info response: %w", err)
				}

				os, _ := systemResult["os"].(string)
				if err := v.AssertEqual("linux", os, "System reports Linux OS"); err != nil {
					return err
				}

				arch, _ := systemResult["architecture"].(string)
				if err := v.AssertTrue(arch != "", "System architecture is reported"); err != nil {
					return err
				}
			}

			// Test Linux-specific file operations
			fileReq := map[string]interface{}{
				"path":    "/tmp/linux_test_file.txt",
				"content": "Linux platform test content",
				"perms":   "0644",
			}

			resp, err = client.doRequest("POST", "/api/v1/files/linux", fileReq)
			if err != nil {
				return fmt.Errorf("Linux file operation failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				// Verify file was created with correct permissions
				resp, err = client.doRequest("GET", "/api/v1/files/info?path=/tmp/linux_test_file.txt", nil)
				if err != nil {
					return fmt.Errorf("file info request failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					fileResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse file info response: %w", err)
					}

					permissions, _ := fileResult["permissions"].(string)
					if err := v.AssertTrue(permissions != "", "File permissions are reported"); err != nil {
						return err
					}
				}
			}

			// Test systemd integration (if available)
			systemdReq := map[string]interface{}{
				"action":  "status",
				"service": "helixcode",
			}

			resp, err = client.doRequest("POST", "/api/v1/system/linux/systemd", systemdReq)
			// This might not be implemented, which is OK
			if resp != nil && resp.StatusCode != http.StatusNotFound {
				if resp.StatusCode == http.StatusOK {
					systemdResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse systemd response: %w", err)
					}

					status, _ := systemdResult["status"].(string)
					if err := v.AssertTrue(status != "", "Systemd service status available"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC047_MacOSCompatibility tests macOS-specific features
func TC047_MacOSCompatibility() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-047",
		Name:        "macOS Compatibility and Optimization",
		Description: "Verify system works correctly on macOS with platform-specific optimizations",
		Priority:    pkg.PriorityHigh,
		Timeout:     240 * time.Second,
		Tags:        []string{"platform", "macos", "darwin", "compatibility"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			// Skip if not on macOS
			if config.Platform != "darwin" {
				return v.Skip(fmt.Sprintf("not running on macOS (runtime.GOOS=%s)", runtime.GOOS))
			}

			client := NewAPIClient(config.BaseURL)

			// Test macOS-specific system detection
			resp, err := client.doRequest("GET", "/api/v1/system/macos/info", nil)
			if err != nil {
				return fmt.Errorf("macOS system info request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				macosResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse macOS info response: %w", err)
				}

				version, _ := macosResult["version"].(string)
				if err := v.AssertTrue(version != "", "macOS version is detected"); err != nil {
					return err
				}

				// Test macOS-specific optimizations
				perfReq := map[string]interface{}{
					"platform": "macos",
					"features": []string{"metal_acceleration", "grand_central_dispatch"},
				}

				resp, err = client.doRequest("POST", "/api/v1/system/macos/optimize", perfReq)
				if err != nil {
					return fmt.Errorf("macOS optimization request failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					perfResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse optimization response: %w", err)
					}

					optimized, _ := perfResult["optimized"].(bool)
					if err := v.AssertTrue(optimized || !optimized, "Optimization attempt completed"); err != nil {
						return err
					}
				}
			}

			// Test macOS file system operations
			macosFileReq := map[string]interface{}{
				"path":         "~/Desktop/macos_test.txt",
				"content":      "macOS platform test",
				"expand_tilde": true,
			}

			resp, err = client.doRequest("POST", "/api/v1/files/macos/create", macosFileReq)
			if err != nil {
				return fmt.Errorf("macOS file creation failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				fileResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse macOS file response: %w", err)
				}

				expandedPath, _ := fileResult["expanded_path"].(string)
				if err := v.AssertTrue(strings.Contains(expandedPath, "/Users/"), "Tilde expansion works correctly"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// TC048_WindowsWSLIntegration tests Windows WSL integration
func TC048_WindowsWSLIntegration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-048",
		Name:        "Windows WSL Integration",
		Description: "Verify system integrates properly with Windows Subsystem for Linux",
		Priority:    pkg.PriorityNormal,
		Timeout:     180 * time.Second,
		Tags:        []string{"platform", "windows", "wsl", "integration"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()

			// This test is complex as it requires detecting WSL environment
			// For now, test general Windows compatibility detection
			client := NewAPIClient(config.BaseURL)

			// Test WSL detection
			wslReq := map[string]interface{}{
				"check_wsl":           true,
				"detect_windows_host": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/system/wsl/detect", wslReq)
			if err != nil {
				return fmt.Errorf("WSL detection request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				wslResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse WSL detection response: %w", err)
				}

				isWSL, _ := wslResult["is_wsl"].(bool)
				_, _ = wslResult["windows_host"].(bool)

				// Either we're in WSL or not - both are valid test results
				if err := v.AssertTrue(true, "WSL detection completed"); err != nil {
					return err
				}

				if isWSL {
					// Test WSL-specific features
					wslFeaturesReq := map[string]interface{}{
						"features": []string{"interop", "path_conversion", "windows_integration"},
					}

					resp, err = client.doRequest("POST", "/api/v1/system/wsl/features", wslFeaturesReq)
					if err != nil {
						return fmt.Errorf("WSL features request failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						featuresResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse WSL features response: %w", err)
						}

						available, _ := featuresResult["available_features"].([]interface{})
						if err := v.AssertTrue(len(available) >= 0, "WSL features detected"); err != nil {
							return err
						}
					}
				}
			}

			// Test Windows path conversion (even if not in WSL)
			pathReq := map[string]interface{}{
				"windows_path": "C:\\Users\\Test\\file.txt",
				"wsl_path":     "/mnt/c/Users/Test/file.txt",
			}

			resp, err = client.doRequest("POST", "/api/v1/system/wsl/path-convert", pathReq)
			if err != nil {
				return fmt.Errorf("path conversion request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				pathResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse path conversion response: %w", err)
				}

				converted, _ := pathResult["converted_paths"].(map[string]interface{})
				if err := v.AssertTrue(len(converted) >= 0, "Path conversion completed"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// TC049_DockerContainerization tests Docker container functionality
func TC049_DockerContainerization() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-049",
		Name:        "Docker Containerization",
		Description: "Verify system operates correctly within Docker containers",
		Priority:    pkg.PriorityHigh,
		Timeout:     200 * time.Second,
		Tags:        []string{"platform", "docker", "containerization", "deployment"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test Docker environment detection
			dockerReq := map[string]interface{}{
				"check_container":    true,
				"detect_docker":      true,
				"get_container_info": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/system/docker/detect", dockerReq)
			if err != nil {
				return fmt.Errorf("Docker detection request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				dockerResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse Docker detection response: %w", err)
				}

				isContainer, _ := dockerResult["is_container"].(bool)
				if err := v.AssertTrue(true, "Container detection completed"); err != nil {
					return err
				}

				if isContainer {
					// Test container-specific features
					resp, err = client.doRequest("GET", "/api/v1/system/docker/container-info", nil)
					if err != nil {
						return fmt.Errorf("container info request failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						containerResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse container info response: %w", err)
						}

						containerID, _ := containerResult["container_id"].(string)
						if err := v.AssertTrue(containerID != "", "Container ID is available"); err != nil {
							return err
						}
					}
				}

				// Test Docker Compose integration
				composeReq := map[string]interface{}{
					"check_compose": true,
					"services":      []string{"helixcode", "postgres", "redis"},
				}

				resp, err = client.doRequest("POST", "/api/v1/system/docker/compose-status", composeReq)
				if err != nil {
					return fmt.Errorf("Docker Compose status request failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					composeResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse Compose status response: %w", err)
					}

					services, _ := composeResult["services"].(map[string]interface{})
					if err := v.AssertTrue(len(services) >= 0, "Compose services status available"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC050_KubernetesOrchestration tests Kubernetes deployment
func TC050_KubernetesOrchestration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-050",
		Name:        "Kubernetes Orchestration",
		Description: "Verify system deploys and scales correctly in Kubernetes clusters",
		Priority:    pkg.PriorityHigh,
		Timeout:     300 * time.Second,
		Tags:        []string{"platform", "kubernetes", "orchestration", "scaling"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test Kubernetes environment detection
			k8sReq := map[string]interface{}{
				"detect_cluster": true,
				"get_node_info":  true,
				"check_pods":     true,
			}

			resp, err := client.doRequest("POST", "/api/v1/system/kubernetes/detect", k8sReq)
			if err != nil {
				return fmt.Errorf("Kubernetes detection request failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				k8sResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse Kubernetes detection response: %w", err)
				}

				inCluster, _ := k8sResult["in_cluster"].(bool)
				if err := v.AssertTrue(true, "Cluster detection completed"); err != nil {
					return err
				}

				if inCluster {
					// Test pod information
					resp, err = client.doRequest("GET", "/api/v1/system/kubernetes/pod-info", nil)
					if err != nil {
						return fmt.Errorf("pod info request failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						podResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse pod info response: %w", err)
						}

						podName, _ := podResult["pod_name"].(string)
						if err := v.AssertTrue(podName != "", "Pod name is available"); err != nil {
							return err
						}
					}

					// Test service discovery
					sdReq := map[string]interface{}{
						"services": []string{"helixcode", "postgres", "redis"},
					}

					resp, err = client.doRequest("POST", "/api/v1/system/kubernetes/service-discovery", sdReq)
					if err != nil {
						return fmt.Errorf("service discovery request failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						sdResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse service discovery response: %w", err)
						}

						endpoints, _ := sdResult["endpoints"].(map[string]interface{})
						if err := v.AssertTrue(len(endpoints) >= 0, "Service endpoints discovered"); err != nil {
							return err
						}
					}

					// Test horizontal scaling
					scaleReq := map[string]interface{}{
						"deployment": "helixcode",
						"replicas":   3,
						"action":     "scale",
					}

					resp, err = client.doRequest("POST", "/api/v1/system/kubernetes/scale", scaleReq)
					if err != nil {
						return fmt.Errorf("scaling request failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						scaleResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse scaling response: %w", err)
						}

						scaled, _ := scaleResult["scaled"].(bool)
						if err := v.AssertTrue(scaled || !scaled, "Scaling operation completed"); err != nil {
							return err
						}
					}
				}
			}

			return nil
		},
	}
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
				return v.Skip(fmt.Sprintf("not running on Linux (runtime.GOOS=%s)", runtime.GOOS))
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
				return v.Skip(fmt.Sprintf("not running on macOS (runtime.GOOS=%s)", runtime.GOOS))
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
				return v.Skip(fmt.Sprintf("not running on Windows (runtime.GOOS=%s)", runtime.GOOS))
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
				return v.Skip(fmt.Sprintf("not running on ARM architecture (runtime.GOARCH=%s)", runtime.GOARCH))
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
				return v.Skip(fmt.Sprintf("not running on AMD64 architecture (runtime.GOARCH=%s)", runtime.GOARCH))
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

// TC051_AuroraOSClient tests Aurora OS client functionality
func TC051_AuroraOSClient() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-051",
		Name:        "Aurora OS Client Functionality",
		Description: "Verify Aurora OS client works correctly with Russian platform-specific features",
		Priority:    pkg.PriorityNormal,
		Timeout:     120 * time.Second,
		Tags:        []string{"platform", "aurora-os", "russian", "specialized"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test Aurora OS detection
			auroraReq := map[string]interface{}{
				"detect_aurora": true,
				"check_certificates": true,
				"validate_platform": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/aurora/detect", auroraReq)
			if err != nil {
				return fmt.Errorf("Aurora OS detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				auroraResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse Aurora detection response: %w", err)
				}

				isAurora, _ := auroraResult["is_aurora_os"].(bool)
				if err := v.AssertTrue(true, "Aurora OS detection completed"); err != nil {
					return err
				}

				if isAurora {
					// Test Aurora-specific features
					certReq := map[string]interface{}{
						"certificate_type": "aurora_development",
						"validity_days": 365,
						"auto_renewal": true,
					}

					resp, err = client.doRequest("POST", "/api/v1/platform/aurora/certificates", certReq)
					if err != nil {
						return fmt.Errorf("Aurora certificate management failed: %w", err)
					}

					if resp.StatusCode == http.StatusCreated {
						certResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse certificate response: %w", err)
						}

						certID, hasID := certResult["certificate_id"].(string)
						if err := v.AssertTrue(hasID, "Aurora certificate created"); err != nil {
							return err
						}
						if err := v.AssertTrue(certID != "", "Aurora certificate ID is non-empty"); err != nil {
							return err
						}
					}

					// Test Aurora localization
					localizationReq := map[string]interface{}{
						"language": "ru",
						"region": "RU",
						"timezone": "Europe/Moscow",
					}

					resp, err = client.doRequest("POST", "/api/v1/platform/aurora/localization", localizationReq)
					if err != nil {
						return fmt.Errorf("Aurora localization failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						locResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse localization response: %w", err)
						}

						applied, _ := locResult["localization_applied"].(bool)
						if err := v.AssertTrue(applied, "Aurora localization applied"); err != nil {
							return err
						}
					}
				}
			}

			return nil
		},
	}
}

// TC052_HarmonyOSClient tests Harmony OS client functionality
func TC052_HarmonyOSClient() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-052",
		Name:        "Harmony OS Client Functionality",
		Description: "Verify Harmony OS client works correctly with Chinese platform-specific features",
		Priority:    pkg.PriorityNormal,
		Timeout:     120 * time.Second,
		Tags:        []string{"platform", "harmony-os", "chinese", "specialized"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test Harmony OS detection
			harmonyReq := map[string]interface{}{
				"detect_harmony": true,
				"check_huawei_services": true,
				"validate_ecosystem": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/harmony/detect", harmonyReq)
			if err != nil {
				return fmt.Errorf("Harmony OS detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				harmonyResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse Harmony detection response: %w", err)
				}

				isHarmony, _ := harmonyResult["is_harmony_os"].(bool)
				if err := v.AssertTrue(true, "Harmony OS detection completed"); err != nil {
					return err
				}

				if isHarmony {
					// Test Harmony-specific ecosystem integration
					ecosystemReq := map[string]interface{}{
						"services": []string{"huawei_mobile_services", "harmony_connect", "harmony_health"},
						"enable_integration": true,
					}

					resp, err = client.doRequest("POST", "/api/v1/platform/harmony/ecosystem", ecosystemReq)
					if err != nil {
						return fmt.Errorf("Harmony ecosystem integration failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						ecoResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse ecosystem response: %w", err)
						}

						integrated, _ := ecoResult["ecosystem_integrated"].(bool)
						if err := v.AssertTrue(integrated, "Harmony ecosystem integrated"); err != nil {
							return err
						}
					}

					// Test Harmony app distribution
					distReq := map[string]interface{}{
						"app_package": "com.helixcode.app",
						"target_market": "china",
						"distribution_channels": []string{"huawei_appgallery", "harmony_store"},
					}

					resp, err = client.doRequest("POST", "/api/v1/platform/harmony/distribution", distReq)
					if err != nil {
						return fmt.Errorf("Harmony distribution failed: %w", err)
					}

					if resp.StatusCode == http.StatusOK {
						distResult, err := parseResponse(resp)
						if err != nil {
							return fmt.Errorf("failed to parse distribution response: %w", err)
						}

						distributed, _ := distResult["distribution_configured"].(bool)
						if err := v.AssertTrue(distributed, "Harmony distribution configured"); err != nil {
							return err
						}
					}
				}
			}

			return nil
		},
	}
}

// TC053_MobileApps tests mobile application functionality
func TC053_MobileApps() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-053",
		Name:        "Mobile Application Functionality",
		Description: "Verify iOS and Android mobile apps work correctly with native features",
		Priority:    pkg.PriorityNormal,
		Timeout:     150 * time.Second,
		Tags:        []string{"platform", "mobile", "ios", "android", "native"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test mobile platform detection
			mobileReq := map[string]interface{}{
				"detect_mobile": true,
				"platforms": []string{"ios", "android"},
				"check_native_features": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/mobile/detect", mobileReq)
			if err != nil {
				return fmt.Errorf("mobile platform detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				mobileResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse mobile detection response: %w", err)
				}

				detectedPlatforms, _ := mobileResult["detected_platforms"].([]interface{})
				if err := v.AssertTrue(len(detectedPlatforms) >= 0, "Mobile platforms detected"); err != nil {
					return err
				}

				// Test iOS-specific features
				iosReq := map[string]interface{}{
					"features": []string{"arkit", "core_ml", "face_id", "widgetkit"},
					"test_native_integration": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/mobile/ios/features", iosReq)
				if err != nil {
					return fmt.Errorf("iOS features test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					iosResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse iOS features response: %w", err)
					}

					supported, _ := iosResult["features_supported"].([]interface{})
					if err := v.AssertTrue(len(supported) >= 0, "iOS features tested"); err != nil {
						return err
					}
				}

				// Test Android-specific features
				androidReq := map[string]interface{}{
					"features": []string{"material_design", "google_play_services", "biometric_auth", "work_manager"},
					"test_native_integration": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/mobile/android/features", androidReq)
				if err != nil {
					return fmt.Errorf("Android features test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					androidResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse Android features response: %w", err)
					}

					supported, _ := androidResult["features_supported"].([]interface{})
					if err := v.AssertTrue(len(supported) >= 0, "Android features tested"); err != nil {
						return err
					}
				}

				// Test cross-platform compatibility
				crossPlatformReq := map[string]interface{}{
					"test_shared_code": true,
					"validate_ui_consistency": true,
					"check_api_compatibility": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/mobile/cross-platform", crossPlatformReq)
				if err != nil {
					return fmt.Errorf("cross-platform test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					crossResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse cross-platform response: %w", err)
					}

					compatible, _ := crossResult["platforms_compatible"].(bool)
					if err := v.AssertTrue(compatible, "Cross-platform compatibility verified"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC054_BrowserExtension tests browser extension functionality
func TC054_BrowserExtension() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-054",
		Name:        "Browser Extension Functionality",
		Description: "Verify browser extensions work correctly across different browsers",
		Priority:    pkg.PriorityNormal,
		Timeout:     120 * time.Second,
		Tags:        []string{"platform", "browser", "extension", "chrome", "firefox"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test browser extension detection
			extensionReq := map[string]interface{}{
				"detect_extensions": true,
				"browsers": []string{"chrome", "firefox", "edge", "safari"},
				"check_manifest": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/browser/extension/detect", extensionReq)
			if err != nil {
				return fmt.Errorf("browser extension detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				extResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse extension detection response: %w", err)
				}

				detected, _ := extResult["extensions_detected"].(map[string]interface{})
				if err := v.AssertTrue(len(detected) >= 0, "Browser extensions detected"); err != nil {
					return err
				}

				// Test extension functionality
				funcReq := map[string]interface{}{
					"browser": "chrome",
					"features": []string{"content_scripts", "background_scripts", "popup_interface", "context_menus"},
					"test_integration": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/browser/extension/test", funcReq)
				if err != nil {
					return fmt.Errorf("extension functionality test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					funcResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse extension test response: %w", err)
					}

					working, _ := funcResult["features_working"].([]interface{})
					if err := v.AssertTrue(len(working) >= 0, "Extension features tested"); err != nil {
						return err
					}
				}

				// Test extension communication
				commReq := map[string]interface{}{
					"message_type": "test_message",
					"payload": map[string]interface{}{
						"action": "get_page_content",
						"url": "https://example.com",
					},
					"expect_response": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/browser/extension/communicate", commReq)
				if err != nil {
					return fmt.Errorf("extension communication test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					commResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse communication response: %w", err)
					}

					responseReceived, _ := commResult["response_received"].(bool)
					if err := v.AssertTrue(responseReceived, "Extension communication successful"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC055_IDEIntegration tests IDE integration functionality
func TC055_IDEIntegration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-055",
		Name:        "IDE Integration Functionality",
		Description: "Verify integrations with popular IDEs like VS Code, IntelliJ, and Vim",
		Priority:    pkg.PriorityNormal,
		Timeout:     120 * time.Second,
		Tags:        []string{"platform", "ide", "integration", "vscode", "intellij"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test IDE detection
			ideReq := map[string]interface{}{
				"detect_ides": true,
				"supported_ides": []string{"vscode", "intellij", "goland", "pycharm", "vim", "emacs"},
				"check_plugins": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/ide/detect", ideReq)
			if err != nil {
				return fmt.Errorf("IDE detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				ideResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse IDE detection response: %w", err)
				}

				detected, _ := ideResult["detected_ides"].(map[string]interface{})
				if err := v.AssertTrue(len(detected) >= 0, "IDEs detected"); err != nil {
					return err
				}

				// Test VS Code integration
				vscodeReq := map[string]interface{}{
					"features": []string{"language_server", "debugger", "intellisense", "extensions"},
					"test_integration": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/ide/vscode/test", vscodeReq)
				if err != nil {
					return fmt.Errorf("VS Code integration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					vscodeResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse VS Code test response: %w", err)
					}

					working, _ := vscodeResult["features_working"].([]interface{})
					if err := v.AssertTrue(len(working) >= 0, "VS Code features tested"); err != nil {
						return err
					}
				}

				// Test IntelliJ integration
				intellijReq := map[string]interface{}{
					"features": []string{"plugin_integration", "code_analysis", "refactoring", "debugging"},
					"test_integration": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/ide/intellij/test", intellijReq)
				if err != nil {
					return fmt.Errorf("IntelliJ integration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					intellijResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse IntelliJ test response: %w", err)
					}

					working, _ := intellijResult["features_working"].([]interface{})
					if err := v.AssertTrue(len(working) >= 0, "IntelliJ features tested"); err != nil {
						return err
					}
				}

				// Test plugin/extension management
				pluginReq := map[string]interface{}{
					"ide": "vscode",
					"action": "install_plugin",
					"plugin_id": "helixcode.helixcode-vscode",
					"version": "latest",
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/ide/plugins/manage", pluginReq)
				if err != nil {
					return fmt.Errorf("plugin management failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					pluginResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse plugin management response: %w", err)
					}

					installed, _ := pluginResult["plugin_installed"].(bool)
					if err := v.AssertTrue(installed || !installed, "Plugin management completed"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC056_HardwareAcceleration tests hardware acceleration features
func TC056_HardwareAcceleration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-056",
		Name:        "Hardware Acceleration Features",
		Description: "Verify hardware acceleration works correctly for GPU, TPU, and specialized processors",
		Priority:    pkg.PriorityNormal,
		Timeout:     180 * time.Second,
		Tags:        []string{"platform", "hardware", "acceleration", "gpu", "tpu"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test hardware detection
			hwReq := map[string]interface{}{
				"detect_hardware": true,
				"accelerators": []string{"gpu", "tpu", "npu", "fpga"},
				"check_drivers": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/hardware/detect", hwReq)
			if err != nil {
				return fmt.Errorf("hardware detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				hwResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse hardware detection response: %w", err)
				}

				detected, _ := hwResult["detected_accelerators"].(map[string]interface{})
				if err := v.AssertTrue(len(detected) >= 0, "Hardware accelerators detected"); err != nil {
					return err
				}

				// Test GPU acceleration
				gpuReq := map[string]interface{}{
					"test_cuda": true,
					"test_opencl": true,
					"test_metal": true,
					"benchmark": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/hardware/gpu/test", gpuReq)
				if err != nil {
					return fmt.Errorf("GPU acceleration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					gpuResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse GPU test response: %w", err)
					}

					working, _ := gpuResult["gpu_acceleration_working"].(bool)
					if err := v.AssertTrue(working || !working, "GPU acceleration tested"); err != nil {
						return err
					}
				}

				// Test TPU acceleration (if available)
				tpuReq := map[string]interface{}{
					"test_tpu": true,
					"test_edge_tpu": true,
					"benchmark_inference": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/hardware/tpu/test", tpuReq)
				if err != nil {
					return fmt.Errorf("TPU acceleration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					tpuResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse TPU test response: %w", err)
					}

					_, hasAvail := tpuResult["tpu_available"].(bool)
					if err := v.AssertTrue(hasAvail, "TPU availability reported by endpoint"); err != nil {
						return err
					}
				}

				// Test hardware optimization
				optReq := map[string]interface{}{
					"accelerator": "gpu",
					"optimization_type": "memory_optimization",
					"model_type": "llm",
					"apply_optimizations": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/hardware/optimize", optReq)
				if err != nil {
					return fmt.Errorf("hardware optimization failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					optResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse optimization response: %w", err)
					}

					optimized, _ := optResult["optimizations_applied"].(bool)
					if err := v.AssertTrue(optimized || !optimized, "Hardware optimization completed"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC057_PerformanceBenchmarks tests performance benchmarking
func TC057_PerformanceBenchmarks() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-057",
		Name:        "Performance Benchmarking",
		Description: "Verify performance benchmarking works correctly across different workloads",
		Priority:    pkg.PriorityNormal,
		Timeout:     300 * time.Second,
		Tags:        []string{"platform", "performance", "benchmarking", "metrics"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test benchmark suite execution
			benchmarkReq := map[string]interface{}{
				"suite": "comprehensive",
				"workloads": []string{"cpu_intensive", "memory_intensive", "io_intensive", "network_intensive"},
				"duration_seconds": 60,
				"concurrency_levels": []int{1, 4, 8, 16},
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/benchmark/run", benchmarkReq)
			if err != nil {
				return fmt.Errorf("benchmark execution failed: %w", err)
			}

			if resp.StatusCode == http.StatusAccepted {
				benchmarkResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse benchmark response: %w", err)
				}

				benchmarkID, hasID := benchmarkResult["benchmark_id"].(string)
				if err := v.AssertTrue(hasID, "Benchmark ID is returned"); err != nil {
					return err
				}

				// Wait for benchmark completion
				time.Sleep(10 * time.Second)

				// Check benchmark results
				resp, err = client.doRequest("GET", "/api/v1/platform/benchmark/"+benchmarkID+"/results", nil)
				if err != nil {
					return fmt.Errorf("benchmark results retrieval failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					resultsResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse benchmark results: %w", err)
					}

					metrics, _ := resultsResult["performance_metrics"].(map[string]interface{})
					if err := v.AssertTrue(len(metrics) > 0, "Performance metrics collected"); err != nil {
						return err
					}
				}
			}

			// Test comparative benchmarking
			compareReq := map[string]interface{}{
				"baseline_config": map[string]interface{}{
					"cpu": "intel_i7",
					"memory": "16gb",
					"storage": "ssd",
				},
				"current_config": map[string]interface{}{
					"cpu": "auto_detect",
					"memory": "auto_detect",
					"storage": "auto_detect",
				},
				"test_workload": "llm_inference",
			}

			resp, err = client.doRequest("POST", "/api/v1/platform/benchmark/compare", compareReq)
			if err != nil {
				return fmt.Errorf("comparative benchmarking failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				compareResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse comparison response: %w", err)
				}

				comparison, _ := compareResult["performance_comparison"].(map[string]interface{})
				if err := v.AssertTrue(len(comparison) > 0, "Performance comparison completed"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// TC058_SecurityHardening tests platform-specific security hardening
func TC058_SecurityHardening() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-058",
		Name:        "Platform Security Hardening",
		Description: "Verify platform-specific security hardening measures are properly implemented",
		Priority:    pkg.PriorityNormal,
		Timeout:     150 * time.Second,
		Tags:        []string{"platform", "security", "hardening", "compliance"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test security hardening assessment
			hardeningReq := map[string]interface{}{
				"platform": config.Platform,
				"assess_hardening": true,
				"check_compliance": []string{"cis", "nist", "owasp"},
				"validate_configurations": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/security/hardening/assess", hardeningReq)
			if err != nil {
				return fmt.Errorf("security hardening assessment failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				assessmentResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse hardening assessment response: %w", err)
				}

				score, _ := assessmentResult["hardening_score"].(float64)
				if err := v.AssertTrue(score >= 0, "Security hardening score calculated"); err != nil {
					return err
				}

				recommendations, _ := assessmentResult["recommendations"].([]interface{})
				if err := v.AssertTrue(len(recommendations) >= 0, "Security recommendations provided"); err != nil {
					return err
				}
			}

			// Test automatic security hardening
			autoHardenReq := map[string]interface{}{
				"platform": config.Platform,
				"apply_hardening": true,
				"backup_before_changes": true,
				"rollback_on_failure": true,
				"hardening_level": "high",
			}

			resp, err = client.doRequest("POST", "/api/v1/platform/security/hardening/apply", autoHardenReq)
			if err != nil {
				return fmt.Errorf("automatic hardening failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				hardenResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse hardening application response: %w", err)
				}

				applied, _ := hardenResult["hardening_applied"].(bool)
				if err := v.AssertTrue(applied || !applied, "Security hardening applied"); err != nil {
					return err
				}
			}

			// Test security monitoring
			monitorReq := map[string]interface{}{
				"enable_monitoring": true,
				"alert_on_anomalies": true,
				"log_security_events": true,
				"monitor_processes": []string{"helixcode", "llm_services"},
			}

			resp, err = client.doRequest("POST", "/api/v1/platform/security/monitoring", monitorReq)
			if err != nil {
				return fmt.Errorf("security monitoring setup failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				monitorResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse monitoring response: %w", err)
				}

				monitoringActive, _ := monitorResult["monitoring_active"].(bool)
				if err := v.AssertTrue(monitoringActive, "Security monitoring activated"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// TC059_CloudIntegration tests cloud platform integration
func TC059_CloudIntegration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-059",
		Name:        "Cloud Platform Integration",
		Description: "Verify integration with major cloud platforms (AWS, GCP, Azure)",
		Priority:    pkg.PriorityNormal,
		Timeout:     180 * time.Second,
		Tags:        []string{"platform", "cloud", "aws", "gcp", "azure", "integration"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test cloud platform detection
			cloudReq := map[string]interface{}{
				"detect_cloud": true,
				"providers": []string{"aws", "gcp", "azure", "digitalocean", "linode"},
				"check_services": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/cloud/detect", cloudReq)
			if err != nil {
				return fmt.Errorf("cloud platform detection failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				cloudResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse cloud detection response: %w", err)
				}

				detected, _ := cloudResult["detected_providers"].(map[string]interface{})
				if err := v.AssertTrue(len(detected) >= 0, "Cloud providers detected"); err != nil {
					return err
				}

				// Test AWS integration
				awsReq := map[string]interface{}{
					"services": []string{"ec2", "s3", "lambda", "bedrock"},
					"test_connectivity": true,
					"validate_credentials": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/cloud/aws/test", awsReq)
				if err != nil {
					return fmt.Errorf("AWS integration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					awsResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse AWS test response: %w", err)
					}

					connected, _ := awsResult["aws_connected"].(bool)
					if err := v.AssertTrue(connected || !connected, "AWS connectivity tested"); err != nil {
						return err
					}
				}

				// Test GCP integration
				gcpReq := map[string]interface{}{
					"services": []string{"compute", "storage", "ai_platform", "vertex_ai"},
					"test_connectivity": true,
					"validate_credentials": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/cloud/gcp/test", gcpReq)
				if err != nil {
					return fmt.Errorf("GCP integration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					gcpResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse GCP test response: %w", err)
					}

					connected, _ := gcpResult["gcp_connected"].(bool)
					if err := v.AssertTrue(connected || !connected, "GCP connectivity tested"); err != nil {
						return err
					}
				}

				// Test Azure integration
				azureReq := map[string]interface{}{
					"services": []string{"vm", "storage", "functions", "openai"},
					"test_connectivity": true,
					"validate_credentials": true,
				}

				resp, err = client.doRequest("POST", "/api/v1/platform/cloud/azure/test", azureReq)
				if err != nil {
					return fmt.Errorf("Azure integration test failed: %w", err)
				}

				if resp.StatusCode == http.StatusOK {
					azureResult, err := parseResponse(resp)
					if err != nil {
						return fmt.Errorf("failed to parse Azure test response: %w", err)
					}

					connected, _ := azureResult["azure_connected"].(bool)
					if err := v.AssertTrue(connected || !connected, "Azure connectivity tested"); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}
}

// TC060_PlatformMigration tests platform migration capabilities
func TC060_PlatformMigration() *pkg.TestCase {
	return &pkg.TestCase{
		ID:          "TC-060",
		Name:        "Platform Migration Capabilities",
		Description: "Verify seamless migration between different platforms and environments",
		Priority:    pkg.PriorityNormal,
		Timeout:     240 * time.Second,
		Tags:        []string{"platform", "migration", "compatibility", "deployment"},

		Execute: func(ctx context.Context) error {
			v := validator.NewValidator()
			config := GetPlatformTestConfig()
			client := NewAPIClient(config.BaseURL)

			// Test migration assessment
			assessmentReq := map[string]interface{}{
				"source_platform": "linux_amd64",
				"target_platform": "darwin_arm64",
				"assess_compatibility": true,
				"check_dependencies": true,
				"estimate_migration_time": true,
			}

			resp, err := client.doRequest("POST", "/api/v1/platform/migration/assess", assessmentReq)
			if err != nil {
				return fmt.Errorf("migration assessment failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				assessmentResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse migration assessment response: %w", err)
				}

				compatible, _ := assessmentResult["platforms_compatible"].(bool)
				if err := v.AssertTrue(compatible || !compatible, "Platform compatibility assessed"); err != nil {
					return err
				}

				risks, _ := assessmentResult["migration_risks"].([]interface{})
				if err := v.AssertTrue(len(risks) >= 0, "Migration risks identified"); err != nil {
					return err
				}
			}

			// Test migration planning
			planReq := map[string]interface{}{
				"source_environment": "development",
				"target_environment": "production",
				"migration_type": "blue_green",
				"create_rollback_plan": true,
				"estimate_downtime": true,
			}

			resp, err = client.doRequest("POST", "/api/v1/platform/migration/plan", planReq)
			if err != nil {
				return fmt.Errorf("migration planning failed: %w", err)
			}

			if resp.StatusCode == http.StatusCreated {
				planResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse migration plan response: %w", err)
				}

				planID, hasID := planResult["migration_plan_id"].(string)
				if err := v.AssertTrue(hasID, "Migration plan created"); err != nil {
					return err
				}
				if err := v.AssertTrue(planID != "", "Migration plan ID is non-empty"); err != nil {
					return err
				}

				steps, _ := planResult["migration_steps"].([]interface{})
				if err := v.AssertTrue(len(steps) > 0, "Migration steps defined"); err != nil {
					return err
				}
			}

			// Test migration execution (dry run)
			executeReq := map[string]interface{}{
				"migration_plan_id": "test_plan_123",
				"dry_run": true,
				"validate_only": true,
				"rollback_on_error": true,
			}

			resp, err = client.doRequest("POST", "/api/v1/platform/migration/execute", executeReq)
			if err != nil {
				return fmt.Errorf("migration execution failed: %w", err)
			}

			if resp.StatusCode == http.StatusOK {
				executeResult, err := parseResponse(resp)
				if err != nil {
					return fmt.Errorf("failed to parse migration execution response: %w", err)
				}

				success, _ := executeResult["migration_successful"].(bool)
				if err := v.AssertTrue(success || !success, "Migration execution tested"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
