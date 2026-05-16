// p1f13_challenge runs the F13 LSP Integration pipeline end-to-end against a
// real subprocess (in-tree fake LSP server) and the real ToolRegistry. Runtime
// evidence harness for the F13 Challenge.
//
// Phases:
//
//	0. Setup        — go-build the in-tree fake LSP server, capture path+size.
//	A. Lazy spawn   — NotifyOpen on a .fake file, wait for diagnostics,
//	                  assert one error containing "phase-A-bad", PID > 0.
//	B. DidChange    — overwrite content, NotifyChange, assert new diagnostic
//	                  containing "phase-B-different".
//	C. Restart      — Restart("fake"), reopen, assert new PID != old PID.
//	D. Stop         — Stop("fake"), assert no Servers remain (or status=stopped).
//	E. Auto-trigger — fresh ToolRegistry + fresh manager wired via SetLSPManager,
//	                  drive fs_write through registry.Execute, assert the
//	                  post-Execute LSP auto-trigger published a diagnostic
//	                  containing "phase-E-via-registry".
//	F. (gated) gopls — only if exec.LookPath("gopls") succeeds. Otherwise
//	                  prints "[skipped: gopls not on PATH]" and returns nil.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/tools"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F13 challenge harness pid:", os.Getpid())

	fakeBin, cleanup, err := phase0BuildFakeServer()
	if err != nil {
		return fmt.Errorf("phase 0: %w", err)
	}
	defer cleanup()

	workspace, err := os.MkdirTemp("", "p1f13-ws-")
	if err != nil {
		return fmt.Errorf("workspace tempdir: %w", err)
	}
	defer os.RemoveAll(workspace)
	fmt.Printf("    workspace      : %s\n", workspace)

	manager := newManager(workspace, fakeBin)
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = manager.Shutdown(shutCtx)
	}()

	filePath := filepath.Join(workspace, "doc.fake")

	oldPID, err := phaseA(manager, filePath)
	if err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(manager, filePath); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(manager, filePath, oldPID); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(manager); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(workspace, fakeBin); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}
	if err := phaseF(); err != nil {
		return fmt.Errorf("phase F: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F13 challenge harness PASS")
	return nil
}

// phase0BuildFakeServer compiles the in-tree fake LSP server into a tempdir,
// returning the absolute path to the binary plus a cleanup function for the
// tempdir. The build is a real `go build` exec — the LSPManager's whole job
// is driving a real subprocess, so the harness must too.
func phase0BuildFakeServer() (string, func(), error) {
	fmt.Println("==> phase 0: build in-tree fake LSP server")

	tmpDir, err := os.MkdirTemp("", "p1f13-fakebin-")
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
		return "", func() {}, fmt.Errorf("go build fakeserver: %w", err)
	}

	st, err := os.Stat(binPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", func() {}, fmt.Errorf("stat fakeserver: %w", err)
	}
	fmt.Printf("    fake LSP binary: %s\n", binPath)
	fmt.Printf("    binary size    : %d bytes\n", st.Size())

	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	return binPath, cleanup, nil
}

// newManager constructs an LSPManager bound to the in-tree fake server with
// the .fake extension routed to it. Uses zap.NewNop so the harness output is
// not polluted by manager-internal log lines.
func newManager(workspace, fakeBin string) *tools.LSPManager {
	spec := tools.LSPServerSpec{
		Name:           "fake",
		Binary:         fakeBin,
		Args:           nil,
		FileExtensions: []string{".fake"},
		LanguageID:     "fake",
	}
	return tools.NewLSPManager(workspace, []tools.LSPServerSpec{spec}, zap.NewNop())
}

// waitForDiagnostics polls manager.GetDiagnostics(filePath) every 50ms up to
// the timeout, returning when pred(len(diags)) is true. Returns whatever was
// last seen if the deadline expires first.
func waitForDiagnostics(m *tools.LSPManager, filePath string, pred func(int) bool, timeout time.Duration) []tools.Diagnostic {
	deadline := time.Now().Add(timeout)
	var last []tools.Diagnostic
	for time.Now().Before(deadline) {
		last = m.GetDiagnostics(filePath)
		if pred(len(last)) {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	return last
}

// waitForDiagnosticMessage polls manager.GetDiagnostics(filePath) every 50ms
// up to the timeout, returning when at least one diagnostic's Message contains
// the want substring. Useful after a NotifyChange where stale diagnostics may
// briefly persist before the server re-publishes against the new content.
func waitForDiagnosticMessage(m *tools.LSPManager, filePath, want string, timeout time.Duration) []tools.Diagnostic {
	deadline := time.Now().Add(timeout)
	var last []tools.Diagnostic
	for time.Now().Before(deadline) {
		last = m.GetDiagnostics(filePath)
		for _, d := range last {
			if containsString(d.Message, want) {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return last
}

// containsString is a tiny helper to avoid importing "strings" only for one
// substring check. Equivalent to strings.Contains.
func containsString(s, sub string) bool {
	if sub == "" {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// phaseA writes a .fake file, calls NotifyOpen, and waits for the fake server
// to publish exactly one error diagnostic containing "phase-A-bad". Returns
// the PID of the spawned server so phaseC can verify it changes after Restart.
func phaseA(m *tools.LSPManager, filePath string) (int, error) {
	fmt.Println("==> phase A: lazy spawn + diagnostics round-trip")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := "// @fake-error: phase-A-bad\nbody-A\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", filePath, err)
	}
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		return 0, fmt.Errorf("NotifyOpen: %w", err)
	}

	diags := waitForDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 2*time.Second)
	if len(diags) != 1 {
		return 0, fmt.Errorf("expected exactly 1 diagnostic; got %d (%+v)", len(diags), diags)
	}
	if diags[0].Severity != tools.SeverityError {
		return 0, fmt.Errorf("diagnostic severity = %v; want SeverityError", diags[0].Severity)
	}
	if !containsString(diags[0].Message, "phase-A-bad") {
		return 0, fmt.Errorf("diagnostic message = %q; want to contain %q", diags[0].Message, "phase-A-bad")
	}

	infos := m.Servers()
	if len(infos) != 1 {
		return 0, fmt.Errorf("expected exactly 1 managed server; got %d", len(infos))
	}
	if infos[0].Name != "fake" {
		return 0, fmt.Errorf("server name = %q; want %q", infos[0].Name, "fake")
	}
	if infos[0].PID <= 0 {
		return 0, fmt.Errorf("server PID = %d; want > 0", infos[0].PID)
	}

	fmt.Printf("    spawned server : name=%q pid=%d status=%q\n",
		infos[0].Name, infos[0].PID, infos[0].Status.String())
	fmt.Printf("    diagnostic     : severity=%s message=%q\n",
		diags[0].Severity.String(), diags[0].Message)
	return infos[0].PID, nil
}

// phaseB rewrites the file content, calls NotifyChange with the new bytes,
// and waits for the fake server to re-publish a diagnostic that mentions the
// new pragma marker.
func phaseB(m *tools.LSPManager, filePath string) error {
	fmt.Println("==> phase B: didChange round-trip")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newContent := "// @fake-error: phase-B-different\nbody-B\n"
	if err := os.WriteFile(filePath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("rewrite %s: %w", filePath, err)
	}
	if err := m.NotifyChange(ctx, filePath, newContent); err != nil {
		return fmt.Errorf("NotifyChange: %w", err)
	}

	diags := waitForDiagnosticMessage(m, filePath, "phase-B-different", 2*time.Second)
	found := false
	for _, d := range diags {
		if containsString(d.Message, "phase-B-different") {
			found = true
			fmt.Printf("    didChange diag : severity=%s message=%q\n",
				d.Severity.String(), d.Message)
			break
		}
	}
	if !found {
		return fmt.Errorf("no diagnostic mentioning %q after NotifyChange (last seen: %+v)",
			"phase-B-different", diags)
	}
	return nil
}

// phaseC restarts the named server and re-opens the document, asserting that
// the new server PID is different from the one captured in phaseA — proof
// that Restart actually cycles the OS process rather than just a logical
// in-memory marker.
func phaseC(m *tools.LSPManager, filePath string, oldPID int) error {
	fmt.Println("==> phase C: Restart cycles the OS process")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.Restart(ctx, "fake"); err != nil {
		return fmt.Errorf("Restart: %w", err)
	}

	// After Restart the entry is dropped; re-open to lazy-respawn.
	content := "// @fake-error: phase-C-after-restart\nbody-C\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("rewrite %s: %w", filePath, err)
	}
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		return fmt.Errorf("NotifyOpen post-restart: %w", err)
	}

	diags := waitForDiagnosticMessage(m, filePath, "phase-C-after-restart", 5*time.Second)
	if len(diags) == 0 {
		return fmt.Errorf("no diagnostics after restart+reopen")
	}

	infos := m.Servers()
	if len(infos) != 1 {
		return fmt.Errorf("expected exactly 1 managed server post-restart; got %d", len(infos))
	}
	newPID := infos[0].PID
	if newPID <= 0 {
		return fmt.Errorf("post-restart PID = %d; want > 0", newPID)
	}
	if newPID == oldPID {
		return fmt.Errorf("post-restart PID = %d; same as pre-restart PID — Restart did not cycle the process", newPID)
	}

	fmt.Printf("    pre-restart pid : %d\n", oldPID)
	fmt.Printf("    post-restart pid: %d (different — process cycled)\n", newPID)
	return nil
}

// phaseD stops the named server and asserts that the manager either drops it
// from the Servers slice entirely or reports it with Stopped status — both
// outcomes prove the process is no longer ready to receive notifications.
func phaseD(m *tools.LSPManager) error {
	fmt.Println("==> phase D: Stop tears the server down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.Stop(ctx, "fake"); err != nil {
		return fmt.Errorf("Stop: %w", err)
	}

	infos := m.Servers()
	if len(infos) == 0 {
		fmt.Println("    Servers() empty after Stop — server fully torn down")
		return nil
	}
	if len(infos) != 1 {
		return fmt.Errorf("expected 0 or 1 server entries after Stop; got %d", len(infos))
	}
	st := infos[0].Status
	if st != tools.ServerStatusStopped {
		return fmt.Errorf("post-Stop status = %q; want %q (or empty Servers slice)",
			st.String(), tools.ServerStatusStopped.String())
	}
	fmt.Printf("    Servers()[0]   : name=%q status=%q (stopped)\n",
		infos[0].Name, infos[0].Status.String())
	return nil
}

// phaseE drives the registry auto-trigger end-to-end. Construct a FRESH
// ToolRegistry (the prior manager was Stopped in phase D) wired to a FRESH
// LSPManager; call registry.Execute("fs_write", …) and assert the post-Execute
// auto-trigger published a diagnostic containing "phase-E-via-registry".
func phaseE(workspace, fakeBin string) error {
	fmt.Println("==> phase E: auto-trigger after registry.Execute(fs_write)")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := tools.DefaultRegistryConfig()
	cfg.FileSystemConfig.WorkspaceRoot = workspace
	cfg.MappingWorkspace = workspace
	r, err := tools.NewToolRegistry(cfg)
	if err != nil {
		return fmt.Errorf("NewToolRegistry: %w", err)
	}
	defer func() { _ = r.Close() }()

	m := newManager(workspace, fakeBin)
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	}()
	r.SetLSPManager(m)

	path := filepath.Join(workspace, "phaseE.fake")
	content := "// @fake-error: phase-E-via-registry\nbody-E\n"

	// fs_write returns (nil, nil) on success — the registry's auto-trigger
	// fires regardless of result shape, so we only assert no error here.
	if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
		"path":    path,
		"content": content,
	}); err != nil {
		return fmt.Errorf("registry.Execute(fs_write): %w", err)
	}
	// Verify fs_write actually wrote the file to disk before checking
	// auto-trigger output — defends against the auto-trigger silently
	// swallowing a missing-file from a future fs_write regression.
	if st, err := os.Stat(path); err != nil {
		return fmt.Errorf("fs_write did not produce file at %s: %w", path, err)
	} else if st.Size() == 0 {
		return fmt.Errorf("fs_write produced empty file at %s", path)
	}

	diags := waitForDiagnosticMessage(m, path, "phase-E-via-registry", 5*time.Second)
	found := false
	for _, d := range diags {
		if containsString(d.Message, "phase-E-via-registry") {
			found = true
			fmt.Printf("    auto-trigger   : severity=%s message=%q file=%s\n",
				d.Severity.String(), d.Message, filepath.Base(d.FilePath))
			break
		}
	}
	if !found {
		return fmt.Errorf("auto-trigger did not publish %q after fs_write (last seen: %+v)",
			"phase-E-via-registry", diags)
	}

	infos := m.Servers()
	if len(infos) != 1 {
		return fmt.Errorf("expected 1 managed server post-auto-trigger; got %d", len(infos))
	}
	if infos[0].PID <= 0 {
		return fmt.Errorf("auto-trigger spawned server with PID=%d; want > 0", infos[0].PID)
	}
	fmt.Printf("    auto-trigger pid: %d\n", infos[0].PID)
	return nil
}

// phaseF runs against a real gopls server when one is on PATH; otherwise it
// prints the gated-skip line and returns nil. Per the F11/F12 precedent the
// skip is honest and counted as success.
func phaseF() error {
	fmt.Println("==> phase F: real gopls round-trip (gated on PATH)")

	if _, err := exec.LookPath("gopls"); err != nil {
		fmt.Println("    [skipped: gopls not on PATH]")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspace, err := os.MkdirTemp("", "p1f13-gopls-")
	if err != nil {
		return fmt.Errorf("gopls workspace tempdir: %w", err)
	}
	defer os.RemoveAll(workspace)

	if err := os.WriteFile(filepath.Join(workspace, "go.mod"),
		[]byte("module helix.test/p1f13gopls\n\ngo 1.21\n"), 0o600); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	srcPath := filepath.Join(workspace, "broken.go")
	src := "package x\n\nfunc f() { return invalid }\n"
	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		return fmt.Errorf("write broken.go: %w", err)
	}

	goplsSpec := tools.LSPServerSpec{
		Name:           "gopls",
		Binary:         "gopls",
		Args:           []string{"serve"},
		FileExtensions: []string{".go"},
		LanguageID:     "go",
	}
	m := tools.NewLSPManager(workspace, []tools.LSPServerSpec{goplsSpec}, zap.NewNop())
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = m.Shutdown(shutCtx)
	}()

	if err := m.NotifyOpen(ctx, srcPath); err != nil {
		return fmt.Errorf("gopls NotifyOpen: %w", err)
	}

	diags := waitForDiagnostics(m, srcPath, func(n int) bool { return n >= 1 }, 20*time.Second)
	if len(diags) < 1 {
		return fmt.Errorf("gopls published no diagnostics for syntactically broken file")
	}
	infos := m.Servers()
	if len(infos) != 1 || infos[0].PID <= 0 {
		return fmt.Errorf("gopls did not produce a managed server with valid PID; got %+v", infos)
	}
	fmt.Printf("    gopls pid       : %d diagnostics=%d first=%q\n",
		infos[0].PID, len(diags), diags[0].Message)
	return nil
}
