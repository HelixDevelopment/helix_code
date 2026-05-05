//go:build integration

package integration

// lsp_test.go (P1-F13-T10): integration tests covering the LSP pipeline wired
// into HelixCode CLI. These tests use the in-tree fake LSP server (built at
// TestMain time as a real subprocess) so the manager's whole job — driving a
// real subprocess over real LSP-framed JSON-RPC — is exercised end-to-end.
//
// Anti-bluff anchor: NO mocks. The fake server is a real binary produced by
// `go build`; the manager spawns it via os/exec and drives it over its real
// stdin/stdout. The gopls test gates on a real exec.LookPath check.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/tools"
)

// fakeServerBin is the absolute path to the freshly-built lsp_fakeserver
// binary. Populated by buildFakeLSPServerForIntegration (called from the
// shared TestMain in provider_integration_test.go) so every LSP integration
// test in this file can spawn it as a real subprocess.
var fakeServerBin string

// buildFakeLSPServerForIntegration compiles the in-tree fake LSP server into
// a fresh tempdir and stores the resulting binary path in fakeServerBin.
// Returns (tmpDir, cleanupFn, err). The caller (TestMain in
// provider_integration_test.go) is responsible for invoking cleanupFn after
// m.Run() returns. We deliberately use a real `go build` here — the
// LSPManager's whole job is to drive a real subprocess over real LSP frames,
// so the test scaffold must too.
func buildFakeLSPServerForIntegration() (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "helix-lsp-integration-")
	if err != nil {
		return "", func() {}, fmt.Errorf("mkdtemp: %w", err)
	}
	binName := "helix-lsp-fakeserver"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, "dev.helix.code/internal/tools/lsp_fakeserver")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", func() {}, fmt.Errorf("failed to build fake LSP server: %w", err)
	}
	fakeServerBin = binPath
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	return tmpDir, cleanup, nil
}

// fakeIntegrationSpec returns a curated-allowlist entry for the in-tree fake
// LSP server. We use the .fake extension so we never collide with real-language
// pickers in other tests.
func fakeIntegrationSpec() tools.LSPServerSpec {
	return tools.LSPServerSpec{
		Name:           "fake",
		Binary:         fakeServerBin,
		Args:           nil,
		FileExtensions: []string{".fake"},
		LanguageID:     "fake",
	}
}

// waitForLSPDiagnostics polls m.GetDiagnostics(filePath) for up to `timeout`,
// returning when the diagnostic count satisfies pred or timing out.
func waitForLSPDiagnostics(m *tools.LSPManager, filePath string, pred func(int) bool, timeout time.Duration) []tools.Diagnostic {
	deadline := time.Now().Add(timeout)
	var last []tools.Diagnostic
	for time.Now().Before(deadline) {
		last = m.GetDiagnostics(filePath)
		if pred(len(last)) {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	return last
}

// TestLSP_FakeServerEndToEnd verifies the LSPManager + fake LSP subprocess
// pipeline end-to-end: build the binary in TestMain, register a spec for the
// .fake extension, open a document with a fake-error pragma, wait for the
// publishDiagnostics notification, assert one error.
func TestLSP_FakeServerEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	workspace := t.TempDir()
	m := tools.NewLSPManager(workspace, []tools.LSPServerSpec{fakeIntegrationSpec()}, zap.NewNop())
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	})

	filePath := filepath.Join(workspace, "doc.fake")
	require.NoError(t, os.WriteFile(filePath, []byte("// @fake-error: bad code\nbody\n"), 0o600))

	require.NoError(t, m.NotifyOpen(ctx, filePath))

	diags := waitForLSPDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 5*time.Second)
	require.Len(t, diags, 1, "expected exactly 1 diagnostic, got %+v", diags)
	require.Equal(t, tools.SeverityError, diags[0].Severity)
	require.Contains(t, diags[0].Message, "bad code")

	// Confirm a server actually spawned (PID > 0).
	infos := m.Servers()
	require.Len(t, infos, 1)
	require.Equal(t, "fake", infos[0].Name)
	require.Greater(t, infos[0].PID, 0)
}

// TestLSP_AutoTriggerAfterFSEdit verifies the T08 wiring: after a successful
// Edit-class tool execution, the registry's post-Execute hook calls the
// LSPManager (NotifyChange / soft didOpen) so subsequent GetDiagnostics
// returns fresh diagnostics. Uses the real fs_write tool (not a stub) so the
// integration covers the production code path.
func TestLSP_AutoTriggerAfterFSEdit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	workspace := t.TempDir()
	cfg := tools.DefaultRegistryConfig()
	cfg.FileSystemConfig.WorkspaceRoot = workspace
	cfg.MappingWorkspace = workspace
	r, err := tools.NewToolRegistry(cfg)
	require.NoError(t, err)

	m := tools.NewLSPManager(workspace, []tools.LSPServerSpec{fakeIntegrationSpec()}, zap.NewNop())
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	})
	r.SetLSPManager(m)

	// Use the real fs_write tool already registered by NewToolRegistry.
	path := filepath.Join(workspace, "doc.fake")
	_, err = r.Execute(ctx, "fs_write", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: auto-trigger-from-fs-write\n",
	})
	require.NoError(t, err)

	diags := waitForLSPDiagnostics(m, path, func(n int) bool { return n >= 1 }, 5*time.Second)
	require.Len(t, diags, 1, "expected auto-trigger to publish 1 diagnostic, got %+v", diags)
	require.Contains(t, diags[0].Message, "auto-trigger-from-fs-write")
}

// TestLSP_GoplsRoundTrip exercises the manager against a real gopls server
// when one is available on PATH. Skipped (with SKIP-OK marker) on hosts that
// don't have gopls installed.
func TestLSP_GoplsRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("SKIP-OK: P1-F13 gopls not on PATH (apt install golang-go OR brew install gopls)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspace := t.TempDir()
	// Minimal go.mod so gopls treats workspace as a module.
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "go.mod"),
		[]byte("module helix.test/lspround\n\ngo 1.21\n"), 0o600))

	goplsSpec := tools.LSPServerSpec{
		Name:           "gopls",
		Binary:         "gopls",
		Args:           []string{"serve"},
		FileExtensions: []string{".go"},
		LanguageID:     "go",
	}
	m := tools.NewLSPManager(workspace, []tools.LSPServerSpec{goplsSpec}, zap.NewNop())
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	})

	// Syntactically broken Go file: gopls publishes a parse error.
	src := "package x\n\nfunc f() { return invalid }\n"
	srcPath := filepath.Join(workspace, "broken.go")
	require.NoError(t, os.WriteFile(srcPath, []byte(src), 0o600))

	require.NoError(t, m.NotifyOpen(ctx, srcPath))

	diags := waitForLSPDiagnostics(m, srcPath, func(n int) bool { return n >= 1 }, 20*time.Second)
	require.GreaterOrEqual(t, len(diags), 1, "gopls did not publish any diagnostics for broken file (got %+v)", diags)
}

// TestLSP_ListServersShowsAllCurated drives the /lsp slash command's
// list-servers subcommand directly and asserts every curated server name
// appears in the rendered output. This validates that the slash command (and
// therefore the cobra command which delegates to the same renderer) sees the
// full curated allowlist regardless of which servers are running.
func TestLSP_ListServersShowsAllCurated(t *testing.T) {
	ctx := context.Background()
	curated := tools.CuratedServerSpecs()
	require.Len(t, curated, 5, "curated allowlist must contain exactly 5 servers")

	// Build a manager with no specs so no servers can possibly be running.
	// list-servers must still render every curated entry with RUNNING=no.
	m := tools.NewLSPManager(t.TempDir(), nil, zap.NewNop())
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	})

	cmd := commands.NewLSPCommand(m, curated)
	res, err := cmd.Execute(ctx, &commands.CommandContext{Args: []string{"list-servers"}})
	require.NoError(t, err)
	require.True(t, res.Success)

	for _, spec := range curated {
		require.Contains(t, res.Output, spec.Name,
			"list-servers output must mention curated server %q (full output:\n%s)", spec.Name, res.Output)
	}
}
