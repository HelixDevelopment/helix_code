package plugins

// HXC-SEC: path-traversal → arbitrary out-of-tree binary execution + sandbox
// escape regression guards (DEFECT-1, SECURITY HIGH).
//
// §11.4.115 RED-on-broken-artifact polarity: with RED_MODE=1 the security RED
// (TestSecurity_PathTraversal_RED_ReproducesArbitraryExec) reproduces the
// defect on the PRE-FIX code path by demonstrating that an unsanitized plugin
// name escapes the intended plugins/sandbox tree. It runs a HARMLESS marker
// binary in a TEMP dir and cleans it up — never a damaging binary. With
// RED_MODE=0 (default) the same source asserts the defect is ABSENT (Validate
// rejects unsafe names AND ExecutePlugin refuses out-of-tree exec/mkdir).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// redMode reports whether the RED reproduction polarity is active. Default 0 =
// GREEN regression-guard; RED_MODE=1 = reproduce-the-defect-on-broken-artifact.
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}

// unsafeNames is the closed set of path-traversal / escape vectors a plugin
// name must NEVER be permitted to contain. Each must be rejected by both
// Validate() (defense layer 1) and the ExecutePlugin within-base check
// (defense layer 2).
var unsafeNames = []string{
	"../evil",
	"../../etc",
	"..",
	"./hidden",
	".hidden",
	"a/b",
	"a\\b",
	"with/slash",
	"x\x00y",
	"/abs",
}

// TestSecurity_Validate_RejectsUnsafeNames is the GREEN regression guard for
// defense layer 1: Manifest.Validate() MUST reject any name that is not a
// single safe path segment matching ^[A-Za-z0-9_-]+$.
func TestSecurity_Validate_RejectsUnsafeNames(t *testing.T) {
	for _, name := range unsafeNames {
		m := &Manifest{Name: name, Version: "1.0.0", Entrypoint: "main"}
		err := m.Validate()
		if err == nil {
			t.Errorf("Validate() accepted unsafe plugin name %q — path-traversal vector NOT rejected", name)
		}
	}
}

// TestSecurity_Validate_AcceptsSafeNames guards that legitimate plugin names
// keep working (no over-rejection regression).
func TestSecurity_Validate_AcceptsSafeNames(t *testing.T) {
	safe := []string{"test-plugin", "my_plugin", "Plugin1", "a", "ABC-123_x"}
	for _, name := range safe {
		m := &Manifest{Name: name, Version: "1.0.0", Entrypoint: "main"}
		if err := m.Validate(); err != nil {
			t.Errorf("Validate() rejected legitimate plugin name %q: %v", name, err)
		}
	}
}

// TestSecurity_PathTraversal_RED_ReproducesArbitraryExec is the polarity-switch
// test. RED_MODE=1: prove the defect is present on the broken artifact by
// showing an unsanitized name resolves an entrypoint OUTSIDE the intended
// plugins tree (and would exec it). RED_MODE=0: prove the defect is absent —
// ExecutePlugin refuses to exec/mkdir anything that escapes the plugins root.
//
// The marker is HARMLESS (a shell script that writes a sentinel file) and is
// placed + removed inside a temp dir; no damaging binary is ever run.
func TestSecurity_PathTraversal_RED_ReproducesArbitraryExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: shell-script marker assumes a POSIX shell; security guard is OS-independent and covered by Validate + within-base checks")
	}

	// Build an isolated process CWD so the relative filepath.Join("plugins",
	// name, "main") base resolves under our temp tree, never the real repo.
	root := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// pluginsBase = <root>/plugins (the intended, safe tree).
	pluginsBase := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsBase, 0o755); err != nil {
		t.Fatal(err)
	}

	// Plant a HARMLESS marker binary OUTSIDE the plugins tree, at the location
	// the "../evil" traversal would resolve to: <root>/evil/main. If exec
	// happens, it writes a sentinel; we detect that to prove the escape.
	sentinel := filepath.Join(root, "ESCAPED_SENTINEL")
	evilDir := filepath.Join(root, "evil")
	if err := os.MkdirAll(evilDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(evilDir, "main")
	script := "#!/bin/sh\nprintf escaped > " + sentinel + "\n"
	if err := os.WriteFile(marker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(sentinel)
		_ = os.RemoveAll(evilDir)
	})

	// The attacker-controlled plugin name. The pre-fix ExecutePlugin resolved
	// filepath.Join("plugins", name, "main") → "plugins/../evil/main" →
	// "evil/main" (outside plugins/) and exec'd it with no guard.
	const evilName = "../evil"

	if redMode() {
		// RED: faithfully reproduce the PRE-FIX UNGUARDED algorithm — no name
		// validation, no within-base check — exactly what ExecutePlugin did
		// before the fix: resolve the entrypoint from the raw name and exec it.
		// This proves the escape vector is real (the marker, out-of-tree, runs)
		// and is what the fix's guards now block. It must reproduce regardless
		// of the (fixed) production guard, so it is a standing meta-check that
		// the defect was genuine.
		entrypoint := filepath.Join("plugins", evilName, "main") // → "evil/main"
		absEntrypoint, absErr := filepath.Abs(entrypoint)
		if absErr != nil {
			t.Fatal(absErr)
		}
		cmd := exec.CommandContext(context.Background(), absEntrypoint, "go")
		_ = cmd.Run()
		if _, statErr := os.Stat(sentinel); statErr != nil {
			t.Fatalf("RED_MODE=1: unguarded pre-fix resolution did NOT exec out-of-tree marker at %s — defect model is blind", marker)
		}
		t.Logf("RED reproduced: pre-fix unguarded resolution executed out-of-tree binary %q at %s", evilName, marker)
		return
	}

	// GREEN: drive the REAL fixed ExecutePlugin — it MUST refuse the escape
	// (no out-of-tree exec, sentinel never written).
	evilPlugin := &BasePlugin{PluginName: evilName, PluginVersion: "1.0.0"}
	_, execErr := ExecutePlugin(context.Background(), evilPlugin, "go", nil)

	if _, sentErr := os.Stat(sentinel); sentErr == nil {
		t.Fatalf("SECURITY: unsanitized plugin name %q escaped the plugins tree and executed %s — path-traversal NOT blocked", evilName, marker)
	}
	if execErr == nil {
		t.Fatalf("SECURITY: ExecutePlugin returned no error for traversal name %q — expected refusal", evilName)
	}
	if !strings.Contains(execErr.Error(), "unsafe") && !strings.Contains(execErr.Error(), "escape") && !strings.Contains(execErr.Error(), "invalid") {
		t.Logf("ExecutePlugin refused traversal name (err=%v)", execErr)
	}
}

// TestSecurity_ExecutePlugin_NoOutOfTreeMkdir guards that the sandbox MkdirAll
// target cannot escape the sandbox root via an unsanitized name (the second
// escape surface noted in the defect: filepath.Join(sandboxDir, name)).
func TestSecurity_ExecutePlugin_NoOutOfTreeMkdir(t *testing.T) {
	root := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Redirect the sandbox root into the temp tree so that even if the
	// within-base guard were ever reverted, this test could never create
	// directories on the real /tmp filesystem.
	origSandbox := sandboxDir
	sandboxDir = filepath.Join(root, "sandbox-root")
	t.Cleanup(func() { sandboxDir = origSandbox })

	// A name that would push the sandbox dir outside sandboxDir.
	evilPlugin := &BasePlugin{PluginName: "../../escape-sandbox", PluginVersion: "1.0.0"}
	_, execErr := ExecutePlugin(context.Background(), evilPlugin, "go", nil)
	if execErr == nil {
		t.Fatalf("SECURITY: ExecutePlugin accepted sandbox-escaping name — expected refusal")
	}
	// The out-of-sandbox dir must NOT have been created.
	escapedDir := filepath.Join(filepath.Dir(sandboxDir), "escape-sandbox")
	if _, statErr := os.Stat(escapedDir); statErr == nil {
		_ = os.RemoveAll(escapedDir)
		t.Fatalf("SECURITY: sandbox MkdirAll created out-of-sandbox dir %s", escapedDir)
	}
}

// TestSecurity_ExecutePlugin_ValidNameStillResolves guards that a legitimate
// plugin name is NOT broken by the within-base check: when the entrypoint is
// absent the error is the normal "not found", not a spurious "unsafe"/"escape"
// refusal (proving valid names pass the security gate).
func TestSecurity_ExecutePlugin_ValidNameStillResolves(t *testing.T) {
	root := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if err := os.MkdirAll(filepath.Join(root, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	valid := &BasePlugin{PluginName: "good-plugin", PluginVersion: "1.0.0"}
	_, execErr := ExecutePlugin(context.Background(), valid, "go", nil)
	if execErr == nil {
		t.Fatalf("expected entrypoint-not-found error (no binary planted), got nil")
	}
	if strings.Contains(execErr.Error(), "unsafe") || strings.Contains(execErr.Error(), "escape") {
		t.Fatalf("valid plugin name %q was wrongly refused by the security gate: %v", "good-plugin", execErr)
	}
}
