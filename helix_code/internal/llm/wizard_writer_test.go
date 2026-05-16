package llm

// wizard_writer_test.go (P1-F12-T08): real-disk tests for the secret-safe
// wizard config writer. These tests exercise mode 0600 + O_EXCL + atomic
// overwrite semantics against a real temp directory — no filesystem mocks.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWriteWizardConfig_O_EXCL_FailsIfExists verifies that the create-only
// path refuses to clobber a pre-existing file. This is the core
// secret-safety invariant: the wizard MUST NOT silently overwrite a
// previously-saved config (which may belong to another user / another
// account / another deployment).
func TestWriteWizardConfig_O_EXCL_FailsIfExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm.yaml")

	// Pre-create the file with arbitrary content.
	if err := os.WriteFile(path, []byte("preexisting"), 0o600); err != nil {
		t.Fatalf("seeding existing file: %v", err)
	}

	res := newWizardResultFixture()
	err := WriteWizardConfig(path, res)
	if err == nil {
		t.Fatalf("WriteWizardConfig() to existing path: expected error, got nil")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("WriteWizardConfig() error = %v, want os.ErrExist (or wrapped)", err)
	}

	// Original contents must be untouched.
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("re-reading preexisting file: %v", readErr)
	}
	if string(got) != "preexisting" {
		t.Fatalf("WriteWizardConfig() clobbered existing file: got %q, want %q",
			string(got), "preexisting")
	}
}

// TestWriteWizardConfig_CreatesWithMode0600 verifies that a freshly
// written config file lands on disk with mode 0600 — owner-only
// read/write — so secrets in Parameters are not world- or group-readable.
func TestWriteWizardConfig_CreatesWithMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F12-T08 — POSIX file mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "llm.yaml")

	res := newWizardResultFixture()
	if err := WriteWizardConfig(path, res); err != nil {
		t.Fatalf("WriteWizardConfig() = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s): %v", path, err)
	}
	gotMode := info.Mode().Perm()
	wantMode := fs.FileMode(0o600)
	if gotMode != wantMode {
		t.Fatalf("file mode = %v, want %v", gotMode, wantMode)
	}
}

// TestWriteWizardConfig_ParentDirCreatedWith0700 verifies that any
// missing intermediate directories the writer creates land at mode 0700
// so the file can't be enumerated through a wider parent.
func TestWriteWizardConfig_ParentDirCreatedWith0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F12-T08 — POSIX dir mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	// Two levels of new dirs the writer must create.
	parent := filepath.Join(dir, "helixcode_x", "subconfig")
	path := filepath.Join(parent, "llm.yaml")

	res := newWizardResultFixture()
	if err := WriteWizardConfig(path, res); err != nil {
		t.Fatalf("WriteWizardConfig() = %v, want nil", err)
	}

	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("Stat(parent): %v", err)
	}
	gotMode := info.Mode().Perm()
	wantMode := fs.FileMode(0o700)
	if gotMode != wantMode {
		t.Fatalf("parent dir mode = %v, want %v", gotMode, wantMode)
	}
}

// TestWriteWizardConfig_RoundTripYAML verifies the on-disk YAML can be
// read back into an equivalent WizardResult — provider type, parameters,
// and api key all survive the round-trip intact.
func TestWriteWizardConfig_RoundTripYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm.yaml")

	in := &WizardResult{
		ProviderType: ProviderTypeBedrock,
		ConfigEntry: ProviderConfigEntry{
			Type:    ProviderTypeBedrock,
			APIKey:  "AKIAEXAMPLE",
			Enabled: true,
			Parameters: map[string]interface{}{
				"region":                "us-west-2",
				"aws_access_key_id":     "AKIAEXAMPLE",
				"aws_secret_access_key": "secret-shh",
			},
		},
		ConfigPath: path,
	}
	if err := WriteWizardConfig(path, in); err != nil {
		t.Fatalf("WriteWizardConfig() = %v, want nil", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var out WizardResult
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("yaml.Unmarshal: %v\nraw:\n%s", err, string(raw))
	}

	if out.ProviderType != in.ProviderType {
		t.Errorf("ProviderType: got %q, want %q", out.ProviderType, in.ProviderType)
	}
	if out.ConfigEntry.Type != in.ConfigEntry.Type {
		t.Errorf("ConfigEntry.Type: got %q, want %q", out.ConfigEntry.Type, in.ConfigEntry.Type)
	}
	if out.ConfigEntry.APIKey != in.ConfigEntry.APIKey {
		t.Errorf("APIKey: got %q, want %q", out.ConfigEntry.APIKey, in.ConfigEntry.APIKey)
	}
	if got := out.ConfigEntry.Parameters["region"]; got != "us-west-2" {
		t.Errorf("Parameters[region]: got %v, want us-west-2", got)
	}
}

// TestOverwriteWizardConfig_ReplacesExistingAtomically verifies the
// overwrite path: pre-existing file is replaced, ends at mode 0600,
// contents match the new result, and no temp-file leaks remain in the
// directory.
func TestOverwriteWizardConfig_ReplacesExistingAtomically(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P1-F12-T08 — POSIX file mode semantics not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "llm.yaml")

	if err := os.WriteFile(path, []byte("oldcontent"), 0o600); err != nil {
		t.Fatalf("seeding old file: %v", err)
	}

	res := &WizardResult{
		ProviderType: ProviderTypeAnthropic,
		ConfigEntry: ProviderConfigEntry{
			Type:    ProviderTypeAnthropic,
			APIKey:  "sk-ant-newvalue",
			Enabled: true,
			Parameters: map[string]interface{}{
				"api_key": "sk-ant-newvalue",
			},
		},
		ConfigPath: path,
	}

	if err := OverwriteWizardConfig(path, res); err != nil {
		t.Fatalf("OverwriteWizardConfig() = %v, want nil", err)
	}

	// File mode is still 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s): %v", path, err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("post-overwrite mode = %v, want %v", got, want)
	}

	// Contents reflect the new wizard result, not the old.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "oldcontent") {
		t.Fatalf("OverwriteWizardConfig() left old content in place: %q", string(raw))
	}
	var out WizardResult
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("yaml.Unmarshal: %v\nraw:\n%s", err, string(raw))
	}
	if out.ConfigEntry.APIKey != "sk-ant-newvalue" {
		t.Errorf("APIKey: got %q, want sk-ant-newvalue", out.ConfigEntry.APIKey)
	}

	// No leftover temp files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == filepath.Base(path) {
			continue
		}
		t.Errorf("unexpected stray file in dir after atomic rename: %q", name)
	}
}

// newWizardResultFixture returns a minimal valid WizardResult for tests
// that only need to exercise the writer's I/O contract.
func newWizardResultFixture() *WizardResult {
	return &WizardResult{
		ProviderType: ProviderTypeAnthropic,
		ConfigEntry: ProviderConfigEntry{
			Type:    ProviderTypeAnthropic,
			APIKey:  "sk-ant-test",
			Enabled: true,
			Parameters: map[string]interface{}{
				"api_key": "sk-ant-test",
			},
		},
	}
}
