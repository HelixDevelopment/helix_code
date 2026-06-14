package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirToModuleRoot changes CWD to the inner Go module root (two levels up from
// internal/plugins), where the real `plugins/` directory lives. ExecutePlugin
// resolves the entrypoint as `plugins/<name>/main` relative to CWD, so the test
// must run from the module root to exercise the REAL on-disk demo plugin.
func chdirToModuleRoot(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(root, "plugins", "sysinfo", "main")); err != nil {
		t.Fatalf("demo plugin entrypoint not found under %s: %v", root, err)
	}
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestMaybeRunPlugin_RealSysinfoPluginRunsAndReturnsRealOutput(t *testing.T) {
	chdirToModuleRoot(t)
	ctx := context.Background()

	l, err := LoadPlugins(ctx, "plugins")
	require.NoError(t, err)
	_, ok := l.Get("sysinfo")
	require.True(t, ok, "sysinfo plugin loaded from plugins/sysinfo/manifest.yaml")

	out, ran, err := MaybeRunPlugin(ctx, l, "@plugin:sysinfo info")
	require.NoError(t, err)
	require.True(t, ran, "the @plugin:sysinfo syntax was recognised and executed")

	// Anti-bluff: a stub could not produce real `uname -a` output. Assert the
	// real kernel banner is present (Darwin/Linux) — proof of os/exec execution.
	assert.Contains(t, out, "sysinfo plugin v1.0.0")
	assert.True(t,
		strings.Contains(out, "Darwin") || strings.Contains(out, "Linux"),
		"real uname output present (got: %q)", out)
	assert.Contains(t, out, "uname:")
	assert.Contains(t, out, "git_head:")
	t.Logf("REAL plugin output:\n%s", out)
}

func TestMaybeRunPlugin_NonPluginPromptFallsThrough(t *testing.T) {
	out, ran, err := MaybeRunPlugin(context.Background(), nil, "explain the architecture")
	require.NoError(t, err)
	assert.False(t, ran, "ordinary prompt is not a plugin invocation")
	assert.Empty(t, out)
}

func TestMaybeRunPlugin_UnknownPluginIsAnError(t *testing.T) {
	chdirToModuleRoot(t)
	ctx := context.Background()
	l, err := LoadPlugins(ctx, "plugins")
	require.NoError(t, err)

	out, ran, err := MaybeRunPlugin(ctx, l, "@plugin:nope info")
	assert.True(t, ran, "ran=true marks it as a (failed) plugin invocation, not a chat prompt")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestMaybeRunPlugin_BadActionSurfacesRealExitCode(t *testing.T) {
	chdirToModuleRoot(t)
	ctx := context.Background()
	l, err := LoadPlugins(ctx, "plugins")
	require.NoError(t, err)

	// The real script exits 2 on an unknown action — proves real process exit
	// codes propagate through ExecutePlugin (not a print-and-sleep stub).
	_, ran, err := MaybeRunPlugin(ctx, l, "@plugin:sysinfo destroy-everything")
	assert.True(t, ran)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}
