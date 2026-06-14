package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.code/internal/approval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireLSP_RegistersReadOnlyDiagnosticsTools verifies WireLSP wires the
// manager and registers both diagnostics tools at LevelReadOnly so the
// ReadOnlyOnly agent loop accepts them.
func TestWireLSP_RegistersReadOnlyDiagnosticsTools(t *testing.T) {
	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)

	mgr := WireLSP(reg, t.TempDir(), nil)
	require.NotNil(t, mgr, "WireLSP must always return a non-nil manager")

	for _, name := range []string{"lsp_get_diagnostics", "lsp_analyze_diagnostic"} {
		tool, err := reg.Get(name)
		require.NoErrorf(t, err, "WireLSP must register %q", name)
		assert.Equalf(t, approval.LevelReadOnly, tool.RequiresApproval(),
			"%q must be LevelReadOnly", name)
	}
}

// TestWireLSP_DetectsGopls confirms WireLSP's manager picked up the curated
// gopls spec when gopls is installed (it is, per task step 1). Skips if
// gopls is not on PATH.
func TestWireLSP_DetectsGopls(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("SKIP-OK: gopls not on PATH")
	}
	specs := DetectAvailableServers(CuratedServerSpecs())
	found := false
	for _, s := range specs {
		if s.Name == "gopls" {
			found = true
		}
	}
	assert.True(t, found, "gopls is installed → DetectAvailableServers must include it")
}

// TestWireLSPAndOpen_RealGoplsDiagnostics is the end-to-end anti-bluff
// proof: WireLSP a registry, open a Go file containing a deliberate
// compile error via the real gopls server, and assert lsp_get_diagnostics
// reports a real error. Skips (never fakes) if gopls is unavailable.
func TestWireLSPAndOpen_RealGoplsDiagnostics(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("SKIP-OK: gopls not on PATH; cannot run real LSP diagnostics")
	}

	// A self-contained Go module with one deliberate error (undefined symbol).
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module lsptest\n\ngo 1.21\n"), 0o644))
	src := "package main\n\nfunc main() {\n\tx := DoesNotExist()\n\t_ = x\n}\n"
	goFile := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(src), 0o644))

	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	mgr, err := WireLSPAndOpen(ctx, reg, dir, nil, goFile)
	require.NoError(t, err, "WireLSPAndOpen on a readable Go file should not error")
	defer func() {
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = mgr.Shutdown(sctx)
	}()

	tool, err := reg.Get("lsp_get_diagnostics")
	require.NoError(t, err)

	// gopls analyses asynchronously; poll the read-only diagnostics tool
	// until it reports the real error (or the deadline trips).
	var gotErr int
	require.Eventually(t, func() bool {
		res, execErr := tool.Execute(ctx, map[string]interface{}{"file_path": goFile})
		if execErr != nil {
			return false
		}
		sum, ok := res.(DiagnosticSummary)
		if !ok {
			return false
		}
		gotErr = sum.TotalErrors
		return sum.TotalErrors > 0
	}, 50*time.Second, 500*time.Millisecond,
		"real gopls must publish at least one error diagnostic for the undefined symbol")

	assert.Greater(t, gotErr, 0, "expected ≥1 real gopls error diagnostic")
}
