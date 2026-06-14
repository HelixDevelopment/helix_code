package core

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMobileCore(t *testing.T) {
	core := NewMobileCore()
	assert.NotNil(t, core)
	assert.NotNil(t, core.themeManager)
}

func TestThemeManager_GetAvailableThemes(t *testing.T) {
	tm := NewThemeManager()
	themes := tm.GetAvailableThemes()
	assert.Contains(t, themes, "dark")
	assert.Contains(t, themes, "light")
	assert.Contains(t, themes, "helix")
}

func TestThemeManager_SetTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test valid theme
	assert.True(t, tm.SetTheme("light"))
	assert.Equal(t, "Light", tm.GetCurrentTheme().Name)

	// Test invalid theme
	assert.False(t, tm.SetTheme("invalid"))
}

func TestMobileCore_GetDashboardData(t *testing.T) {
	core := NewMobileCore()
	data := core.GetDashboardData()
	assert.NotEmpty(t, data)
	assert.Contains(t, data, "isConnected")
}

func TestMobileCore_GetTasks(t *testing.T) {
	core := NewMobileCore()
	tasks := core.GetTasks()
	assert.NotEmpty(t, tasks)
	// Should contain tasks array
	assert.Contains(t, tasks, `"tasks":`)
}

func TestMobileCore_GetWorkers(t *testing.T) {
	core := NewMobileCore()
	workers := core.GetWorkers()
	assert.NotEmpty(t, workers)
	// Should contain workers array
	assert.Contains(t, workers, `"workers":`)
}

func TestMobileCore_CreateTask(t *testing.T) {
	core := NewMobileCore()
	result := core.CreateTask("Test Task", "Test Description")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "success")
}

func TestMobileCore_GetTheme(t *testing.T) {
	core := NewMobileCore()
	theme := core.GetTheme()
	assert.NotEmpty(t, theme)
	assert.Contains(t, theme, "name")
}

func TestMobileCore_SetTheme(t *testing.T) {
	core := NewMobileCore()

	// Test valid theme
	assert.True(t, core.SetTheme("dark"))

	// Test invalid theme
	assert.False(t, core.SetTheme("invalid"))
}

func TestMobileCore_GetAvailableThemes(t *testing.T) {
	core := NewMobileCore()
	themes := core.GetAvailableThemes()
	assert.NotEmpty(t, themes)
	assert.Contains(t, themes, "dark")
	assert.Contains(t, themes, "light")
	assert.Contains(t, themes, "helix")
}

// TestMobileCore_Generate_EmptyPrompt exercises the real Generate method's
// input-validation error path. This is hermetic (no network): an empty/
// whitespace prompt MUST return a non-nil error and an empty string BEFORE any
// provider is contacted, mirroring how the server's generate endpoint rejects
// empty input. It also proves the method is real (not a stub returning a fixed
// success string regardless of input).
func TestMobileCore_Generate_EmptyPrompt(t *testing.T) {
	core := NewMobileCore()

	out, err := core.Generate("")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "prompt must not be empty")

	out, err = core.Generate("   \t\n  ")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "prompt must not be empty")
}

// TestMobileCore_Generate_NoProviderReachable proves the no-provider error
// path is honest (anti-bluff): with a deliberately bogus HELIX_LLM_PROVIDER and
// no reachable backend, a real prompt MUST NOT yield a fabricated success.
// Either provider construction yields no usable provider (error), or the real
// provider.Generate call fails against the unreachable local Ollama default.
// In BOTH cases the result is an error + empty output — never a fake response.
func TestMobileCore_Generate_NoProviderReachable(t *testing.T) {
	// Point the resolver at a non-cloud value so it falls through to the
	// local Ollama default.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	// Determinism guard: if a real Ollama is listening locally it would
	// legitimately answer, so this negative-path assertion does not apply.
	// Skip honestly in that case rather than assert a false expectation.
	if conn, derr := net.DialTimeout("tcp", "localhost:11434", 200*time.Millisecond); derr == nil {
		_ = conn.Close()
		t.Skip("SKIP-OK: local Ollama is reachable on :11434; negative no-provider path not applicable")
	}

	core := NewMobileCore()
	out, err := core.Generate("Say hello.")

	// Anti-bluff: a stub would return a canned non-empty string with nil
	// error. The real implementation, with no reachable backend, returns an
	// error and empty content.
	require.Error(t, err)
	assert.Empty(t, out)
	// The error must originate from the real generation path, not a stub.
	assert.Contains(t, err.Error(), "generate:")
}
