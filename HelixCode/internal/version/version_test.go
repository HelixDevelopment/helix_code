package version

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetVersion Tests
// =============================================================================

func TestGetVersion_DefaultDev(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	// Test with dev commit
	Version = "1.0.0"
	GitCommit = "dev"

	version := GetVersion()
	assert.Equal(t, "1.0.0", version)
}

func TestGetVersion_WithCommitHash(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "1.0.0"
	GitCommit = "abc123456789def"

	version := GetVersion()
	assert.Equal(t, "1.0.0-abc1234", version)
	assert.Contains(t, version, "abc1234")
}

func TestGetVersion_ShortCommitHash(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "2.0.0"
	GitCommit = "abc12" // Less than 7 characters

	version := GetVersion()
	assert.Equal(t, "2.0.0", version)
}

func TestGetVersion_SevenCharCommit(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "1.0.0"
	GitCommit = "abc1234" // Exactly 7 characters

	version := GetVersion()
	// 7 chars is not > 7, so should return just version
	assert.Equal(t, "1.0.0", version)
}

func TestGetVersion_EightCharCommit(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "1.0.0"
	GitCommit = "abc12345" // 8 characters

	version := GetVersion()
	assert.Equal(t, "1.0.0-abc1234", version)
}

// =============================================================================
// GetFullVersion Tests
// =============================================================================

func TestGetFullVersion_Format(t *testing.T) {
	full := GetFullVersion()

	assert.True(t, strings.HasPrefix(full, "HelixCode"))
	assert.Contains(t, full, "commit:")
	assert.Contains(t, full, "built:")
	assert.Contains(t, full, "go:")
}

func TestGetFullVersion_ContainsVersion(t *testing.T) {
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	Version = "3.5.0"
	full := GetFullVersion()

	assert.Contains(t, full, "3.5.0")
}

func TestGetFullVersion_ContainsCommit(t *testing.T) {
	originalCommit := GitCommit
	defer func() {
		GitCommit = originalCommit
	}()

	GitCommit = "testcommit123"
	full := GetFullVersion()

	assert.Contains(t, full, "testcommit123")
}

func TestGetFullVersion_ContainsBuildDate(t *testing.T) {
	originalBuildDate := BuildDate
	defer func() {
		BuildDate = originalBuildDate
	}()

	BuildDate = "2024-01-15T10:30:00Z"
	full := GetFullVersion()

	assert.Contains(t, full, "2024-01-15T10:30:00Z")
}

func TestGetFullVersion_ContainsGoVersion(t *testing.T) {
	full := GetFullVersion()

	assert.Contains(t, full, GoVersion)
}

// =============================================================================
// GetBuildInfo Tests
// =============================================================================

func TestGetBuildInfo_ReturnsMap(t *testing.T) {
	info := GetBuildInfo()

	require.NotNil(t, info)
	assert.IsType(t, map[string]string{}, info)
}

func TestGetBuildInfo_ContainsAllFields(t *testing.T) {
	info := GetBuildInfo()

	expectedFields := []string{
		"version",
		"commit",
		"build_date",
		"go_version",
		"os",
		"arch",
	}

	for _, field := range expectedFields {
		_, exists := info[field]
		assert.True(t, exists, "Expected field %s not found", field)
	}
}

func TestGetBuildInfo_Version(t *testing.T) {
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	Version = "4.0.0"
	info := GetBuildInfo()

	assert.Equal(t, "4.0.0", info["version"])
}

func TestGetBuildInfo_Commit(t *testing.T) {
	originalCommit := GitCommit
	defer func() {
		GitCommit = originalCommit
	}()

	GitCommit = "buildinfo123"
	info := GetBuildInfo()

	assert.Equal(t, "buildinfo123", info["commit"])
}

func TestGetBuildInfo_BuildDate(t *testing.T) {
	originalBuildDate := BuildDate
	defer func() {
		BuildDate = originalBuildDate
	}()

	BuildDate = "2024-06-20"
	info := GetBuildInfo()

	assert.Equal(t, "2024-06-20", info["build_date"])
}

func TestGetBuildInfo_GoVersion(t *testing.T) {
	info := GetBuildInfo()

	assert.Equal(t, runtime.Version(), info["go_version"])
}

func TestGetBuildInfo_OS(t *testing.T) {
	info := GetBuildInfo()

	assert.Equal(t, runtime.GOOS, info["os"])
}

func TestGetBuildInfo_Arch(t *testing.T) {
	info := GetBuildInfo()

	assert.Equal(t, runtime.GOARCH, info["arch"])
}

// =============================================================================
// Package Variable Tests
// =============================================================================

func TestDefaultVersion(t *testing.T) {
	// These tests verify the default values when not overridden
	assert.NotEmpty(t, Version)
}

func TestDefaultGoVersion(t *testing.T) {
	assert.Equal(t, runtime.Version(), GoVersion)
	assert.True(t, strings.HasPrefix(GoVersion, "go"))
}

func TestVersionVariablesExist(t *testing.T) {
	// Verify all version variables are defined
	assert.NotNil(t, Version)
	assert.NotNil(t, GitCommit)
	assert.NotNil(t, BuildDate)
	assert.NotNil(t, GoVersion)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestVersionConsistency(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "5.0.0"
	GitCommit = "integration123456"

	// Verify GetVersion uses the version from GetBuildInfo
	version := GetVersion()
	info := GetBuildInfo()

	assert.Contains(t, version, info["version"])
}

func TestFullVersionContainsAllInfo(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildDate := BuildDate
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
		BuildDate = originalBuildDate
	}()

	Version = "6.0.0"
	GitCommit = "fulltest123"
	BuildDate = "2024-12-25"

	full := GetFullVersion()
	info := GetBuildInfo()

	assert.Contains(t, full, info["version"])
	assert.Contains(t, full, info["commit"])
	assert.Contains(t, full, info["build_date"])
	assert.Contains(t, full, info["go_version"])
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestGetVersion_EmptyVersion(t *testing.T) {
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	Version = ""
	GitCommit = "dev"

	version := GetVersion()
	assert.Equal(t, "", version)
}

func TestGetVersion_EmptyCommit(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
	}()

	Version = "1.0.0"
	GitCommit = ""

	version := GetVersion()
	assert.Equal(t, "1.0.0", version)
}

func TestGetBuildInfo_UnknownBuildDate(t *testing.T) {
	originalBuildDate := BuildDate
	defer func() {
		BuildDate = originalBuildDate
	}()

	BuildDate = "unknown"
	info := GetBuildInfo()

	assert.Equal(t, "unknown", info["build_date"])
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkGetVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetVersion()
	}
}

func BenchmarkGetFullVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetFullVersion()
	}
}

func BenchmarkGetBuildInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetBuildInfo()
	}
}
