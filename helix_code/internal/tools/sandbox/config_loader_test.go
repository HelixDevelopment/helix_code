package sandbox

// config_loader_test.go (P1-F14-T08): real-disk tests for the secret-safe
// sandbox config YAML loader and writer. These tests exercise mode 0600 +
// O_EXCL + atomic overwrite semantics against a real temp directory — no
// filesystem mocks. Mirrors the F12 wizard_writer test pattern.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestDefaultConfigPath_XDGSet — when XDG_CONFIG_HOME is exported, the
// canonical sandbox.yaml path is rooted under it.
func TestDefaultConfigPath_XDGSet(t *testing.T) {
	envLookup := func(k string) string {
		switch k {
		case "XDG_CONFIG_HOME":
			return "/foo"
		case "HOME":
			return "/home/u"
		}
		return ""
	}
	got := DefaultConfigPath(envLookup)
	want := filepath.Join("/foo", "helixcode", "sandbox.yaml")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

// TestDefaultConfigPath_XDGUnset_HOMESet — when XDG is unset, fall back to
// $HOME/.config/helixcode/sandbox.yaml per XDG-Base-Dir spec.
func TestDefaultConfigPath_XDGUnset_HOMESet(t *testing.T) {
	envLookup := func(k string) string {
		if k == "HOME" {
			return "/home/u"
		}
		return ""
	}
	got := DefaultConfigPath(envLookup)
	want := filepath.Join("/home/u", ".config", "helixcode", "sandbox.yaml")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

// TestDefaultConfigPath_NeitherSet — when both XDG_CONFIG_HOME and HOME are
// empty, fall back to a relative path under the working directory.
// Document: this is a pragmatic last-resort fallback; production should
// always have HOME set.
func TestDefaultConfigPath_NeitherSet(t *testing.T) {
	envLookup := func(k string) string { return "" }
	got := DefaultConfigPath(envLookup)
	want := filepath.Join(".helixcode", "sandbox.yaml")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q (relative fallback)", got, want)
	}
}

// TestLoadSandboxConfig_MissingFileReturnsDefault — sandbox without a config
// file is allowed; loader returns DefaultSandboxConfig() and no error so
// callers can run with safe defaults.
func TestLoadSandboxConfig_MissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("LoadSandboxConfig(missing) err = %v, want nil", err)
	}
	want := DefaultSandboxConfig()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadSandboxConfig(missing) = %+v, want %+v", got, want)
	}
}

// TestLoadSandboxConfig_RoundTripPolicy — write a custom SandboxConfig via
// WriteSandboxConfig, read back via LoadSandboxConfig, deep-equal.
func TestLoadSandboxConfig_RoundTripPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	in := SandboxConfig{
		DefaultPolicy: SandboxPolicy{
			NetworkAllowed: true,
			Timeout:        45 * time.Second,
			MemoryLimitMB:  512,
			CPULimitPct:    50,
			ReadOnlyRoot:   false,
			BindMounts: []BindMount{
				{Source: "/tmp/work", Target: "/work", ReadOnly: false},
			},
			ExtraDeny: []string{`^\s*curl\s+evil\.example\.com`},
		},
		UserDenyList: []string{`^rm -rf /`, `^mkfs\.`},
	}

	if err := WriteSandboxConfig(path, in); err != nil {
		t.Fatalf("WriteSandboxConfig() = %v, want nil", err)
	}

	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("LoadSandboxConfig() = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch:\n got: %+v\nwant: %+v", got, in)
	}
}

// TestLoadSandboxConfig_BadYAMLErrors — non-YAML garbage returns a parse
// error from the underlying yaml package.
func TestLoadSandboxConfig_BadYAMLErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	if err := os.WriteFile(path, []byte("this is: not: valid: : yaml\n\t-bad indent"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := LoadSandboxConfig(path)
	if err == nil {
		t.Fatalf("LoadSandboxConfig(bad yaml) err = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "yaml") {
		t.Fatalf("LoadSandboxConfig(bad yaml) err = %v, want substring 'yaml'", err)
	}
}

// TestLoadSandboxConfig_RejectsNegativeMemory — negative memory_limit_mb is
// nonsensical; loader rejects it.
func TestLoadSandboxConfig_RejectsNegativeMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	yaml := "default_policy:\n  memory_limit_mb: -1\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := LoadSandboxConfig(path)
	if err == nil {
		t.Fatalf("LoadSandboxConfig(negative memory) err = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "memory") {
		t.Fatalf("LoadSandboxConfig(negative memory) err = %v, want substring 'memory'", err)
	}
}

// TestLoadSandboxConfig_RejectsNegativeCPU — same for negative cpu_limit_pct.
func TestLoadSandboxConfig_RejectsNegativeCPU(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	yaml := "default_policy:\n  cpu_limit_pct: -5\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := LoadSandboxConfig(path)
	if err == nil {
		t.Fatalf("LoadSandboxConfig(negative cpu) err = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cpu") {
		t.Fatalf("LoadSandboxConfig(negative cpu) err = %v, want substring 'cpu'", err)
	}
}

// TestLoadSandboxConfig_TimeoutDefaultedWhenZero — a yaml with timeout: 0
// (or omitted) is normalised to the 30s default per the field contract.
func TestLoadSandboxConfig_TimeoutDefaultedWhenZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	// Timeout entirely omitted → zero value → normalised to 30s.
	yaml := "default_policy:\n  network_allowed: false\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("LoadSandboxConfig() = %v, want nil", err)
	}
	if got.DefaultPolicy.Timeout != 30*time.Second {
		t.Fatalf("Timeout default-fill: got %v, want 30s", got.DefaultPolicy.Timeout)
	}
}

// TestWriteSandboxConfig_O_EXCL_FailsIfExists — refuse to clobber a
// pre-existing file. Use OverwriteSandboxConfig if you mean it.
func TestWriteSandboxConfig_O_EXCL_FailsIfExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	if err := os.WriteFile(path, []byte("preexisting"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := WriteSandboxConfig(path, DefaultSandboxConfig())
	if err == nil {
		t.Fatalf("WriteSandboxConfig() to existing path: expected error, got nil")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("WriteSandboxConfig() err = %v, want os.ErrExist (or wrapped)", err)
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("re-read preexisting: %v", readErr)
	}
	if string(got) != "preexisting" {
		t.Fatalf("WriteSandboxConfig clobbered existing file: got %q", string(got))
	}
}

// TestWriteSandboxConfig_CreatesWithMode0600 — fresh file lands at 0600.
func TestWriteSandboxConfig_CreatesWithMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F14-T08 — POSIX file mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	if err := WriteSandboxConfig(path, DefaultSandboxConfig()); err != nil {
		t.Fatalf("WriteSandboxConfig() = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("file mode = %v, want %v", got, want)
	}
}

// TestWriteSandboxConfig_ParentDirCreatedWith0700 — missing parent dirs are
// created at 0700.
func TestWriteSandboxConfig_ParentDirCreatedWith0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F14-T08 — POSIX dir mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	parent := filepath.Join(dir, "helixcode_x", "subconfig")
	path := filepath.Join(parent, "sandbox.yaml")

	if err := WriteSandboxConfig(path, DefaultSandboxConfig()); err != nil {
		t.Fatalf("WriteSandboxConfig() = %v, want nil", err)
	}

	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("Stat parent: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Fatalf("parent dir mode = %v, want %v", got, want)
	}
}

// TestOverwriteSandboxConfig_ReplacesExistingAtomically — overwrite
// succeeds, mode stays 0600, contents reflect new config, no temp files
// leaked beside the destination.
func TestOverwriteSandboxConfig_ReplacesExistingAtomically(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F14-T08 — POSIX file mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	if err := os.WriteFile(path, []byte("oldcontent"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Use explicit empty slices for BindMounts/ExtraDeny — yaml.v3
	// round-trips a nil slice as `[]` which unmarshals back to an empty
	// (non-nil) slice; we want DeepEqual to match the post-round-trip
	// shape, not the pre-marshal one.
	cfg := SandboxConfig{
		DefaultPolicy: SandboxPolicy{
			NetworkAllowed: true,
			Timeout:        90 * time.Second,
			ReadOnlyRoot:   true,
			BindMounts:     []BindMount{},
			ExtraDeny:      []string{},
		},
		UserDenyList: []string{`^rm -rf /`},
	}

	if err := OverwriteSandboxConfig(path, cfg); err != nil {
		t.Fatalf("OverwriteSandboxConfig() = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("post-overwrite mode = %v, want %v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "oldcontent") {
		t.Fatalf("OverwriteSandboxConfig left old content: %q", string(raw))
	}

	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("LoadSandboxConfig: %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Fatalf("contents mismatch: got %+v, want %+v", got, cfg)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == filepath.Base(path) {
			continue
		}
		t.Errorf("unexpected stray file in dir after atomic rename: %q", e.Name())
	}
}

// TestOverwriteSandboxConfig_FinalModeIs0600 — even on a host with a
// permissive umask, the final file is 0600.
func TestOverwriteSandboxConfig_FinalModeIs0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F14-T08 — POSIX file mode semantics not applicable on Windows")
	}
	old := syscallUmask(0o022)
	defer syscallUmask(old)

	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	if err := OverwriteSandboxConfig(path, DefaultSandboxConfig()); err != nil {
		t.Fatalf("OverwriteSandboxConfig() = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("file mode = %v, want %v", got, want)
	}
}

// TestLoadSandboxConfig_PreservesUserDenyList — UserDenyList survives the
// round-trip without truncation/reordering.
func TestLoadSandboxConfig_PreservesUserDenyList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	cfg := DefaultSandboxConfig()
	cfg.UserDenyList = []string{`^rm -rf /`, `^mkfs\.`, `^dd\s+if=`}

	if err := WriteSandboxConfig(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got.UserDenyList, cfg.UserDenyList) {
		t.Fatalf("UserDenyList: got %v, want %v", got.UserDenyList, cfg.UserDenyList)
	}
}

// TestLoadSandboxConfig_PreservesBindMounts — BindMounts (Source/Target/ReadOnly)
// survive the round-trip.
func TestLoadSandboxConfig_PreservesBindMounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.yaml")

	cfg := DefaultSandboxConfig()
	cfg.DefaultPolicy.BindMounts = []BindMount{
		{Source: "/host/work", Target: "/sandbox/work", ReadOnly: false},
		{Source: "/host/data", Target: "/sandbox/data", ReadOnly: true},
	}

	if err := WriteSandboxConfig(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := LoadSandboxConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got.DefaultPolicy.BindMounts, cfg.DefaultPolicy.BindMounts) {
		t.Fatalf("BindMounts: got %+v, want %+v",
			got.DefaultPolicy.BindMounts, cfg.DefaultPolicy.BindMounts)
	}
}
