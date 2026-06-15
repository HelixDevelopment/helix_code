package config

// CONST-042 / §11.4.10 / §11.4.30 / §12.1 — No-Secret-Leak hardening.
//
// These tests guard the file-permission posture of EVERY write path in the
// config package that can persist a plaintext secret (Auth.JWTSecret,
// Redis.Password, provider APIKeys, Cognee APIKey / RemoteAPI.APIKey).
//
// §11.4.115 RED→GREEN polarity:
//   RED_MODE=1 (env HELIX_SECRET_MODE_RED=1) — REPRODUCE the defect: assert the
//     pre-fix world-readable (0644) / shared-/tmp posture is present. Run this
//     against the BROKEN artifact to prove the leak was genuinely there.
//   RED_MODE=0 (default) — the standing GREEN regression guard: assert every
//     write path produces mode 0600 and createBackup lands in a private 0700
//     directory (never shared world-traversable /tmp).
//
// SECURITY: synthetic secrets only; assertions are booleans on os.Stat — the
// secret VALUE is never printed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// redMode reports whether the RED (reproduce-the-defect) polarity is active.
func redMode() bool {
	return os.Getenv("HELIX_SECRET_MODE_RED") == "1"
}

const (
	synthJWTSecret   = "SYNTHETIC-jwt-signing-secret-do-not-use"
	synthRedisPass   = "SYNTHETIC-redis-password"
	synthAPIKey      = "SYNTHETIC-provider-api-key"
	synthRemoteKey   = "SYNTHETIC-cognee-remote-api-key"
)

// secretLadenConfig builds a Config carrying synthetic secrets in every
// secret-bearing field, so the on-disk artifact would leak if world-readable.
func secretLadenConfig() *Config {
	c := getDefaultConfig()
	c.Auth.JWTSecret = synthJWTSecret
	c.Redis.Password = synthRedisPass
	c.Providers.Mem0.APIKey = synthAPIKey
	c.Providers.Zep.APIKey = synthAPIKey
	c.Providers.Memonto.APIKey = synthAPIKey
	c.Providers.BaseAI.APIKey = synthAPIKey
	return c
}

// assertSecretFileMode asserts the on-disk permission posture of a written
// secret file according to the active polarity. Never prints file contents.
func assertSecretFileMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", filepath.Base(path), err)
	}
	perm := info.Mode().Perm()
	worldReadable := perm&0o044 != 0 // group- or other-readable
	if redMode() {
		// Reproduce the defect: pre-fix files are world-readable (0644).
		if !worldReadable {
			t.Fatalf("RED expected world-readable secret file, got perm %#o", perm)
		}
		return
	}
	// GREEN guard: secret files MUST be exactly 0600 (owner rw only).
	if perm != 0o600 {
		t.Fatalf("secret file leaked: perm %#o, want 0600 (CONST-042)", perm)
	}
	if worldReadable {
		t.Fatalf("secret file world-readable: perm %#o (CONST-042)", perm)
	}
}

func TestSecretMode_SaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := &ConfigManager{config: secretLadenConfig(), configPath: path}
	if err := m.saveConfig(); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	assertSecretFileMode(t, path)
}

func TestSecretMode_ExportConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exported.json")
	m := &ConfigManager{config: secretLadenConfig(), configPath: filepath.Join(dir, "config.json")}
	if err := m.ExportConfig(path); err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}
	assertSecretFileMode(t, path)
}

func TestSecretMode_BackupConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.json")
	m := &ConfigManager{config: secretLadenConfig(), configPath: filepath.Join(dir, "config.json")}
	if err := m.BackupConfig(path); err != nil {
		t.Fatalf("BackupConfig: %v", err)
	}
	assertSecretFileMode(t, path)
}

func TestSecretMode_SaveTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "template.json")
	tm := NewConfigurationTemplateManager(dir)
	tmpl := &ConfigurationTemplate{ID: "t1", Name: "t1", Config: secretLadenConfig()}
	if err := tm.SaveTemplate(tmpl, path); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	assertSecretFileMode(t, path)
}

func TestSecretMode_SaveCogneeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cognee.json")
	cfg := DefaultCogneeConfig()
	if cfg.RemoteAPI == nil {
		cfg.RemoteAPI = &CogneeRemoteAPIConfig{}
	}
	cfg.RemoteAPI.APIKey = synthRemoteKey
	if err := SaveCogneeConfig(cfg, path); err != nil {
		t.Fatalf("SaveCogneeConfig: %v", err)
	}
	assertSecretFileMode(t, path)
}

func TestSecretMode_CreateDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	if err := CreateDefaultConfig(path); err != nil {
		t.Fatalf("CreateDefaultConfig: %v", err)
	}
	assertSecretFileMode(t, path)
}

// TestSecretMode_CreateBackup is the D1 (CRITICAL) guard: createBackup must NOT
// land a plaintext secret file in a shared world-traversable /tmp at 0644 with
// a predictable name. Post-fix it must live in a private 0700 directory at 0600.
func TestSecretMode_CreateBackup(t *testing.T) {
	// Redirect the user config dir to an isolated tree so the test never
	// touches the real ~/.config and so we can inspect the produced backup.
	tmpHome := t.TempDir()
	t.Setenv("HELIX_CONFIG_PATH", filepath.Join(tmpHome, ".config", "helixcode", "config.json"))

	mig := &ConfigurationMigrator{}
	if err := mig.createBackup(secretLadenConfig(), "v1"); err != nil {
		t.Fatalf("createBackup: %v", err)
	}

	if redMode() {
		// Reproduce the defect: a predictable world-readable backup in shared /tmp.
		found := findBackupFiles(os.TempDir())
		if len(found) == 0 {
			t.Fatalf("RED expected a backup file in shared TempDir %q", os.TempDir())
		}
		var anyWorldReadable bool
		for _, f := range found {
			if info, err := os.Stat(f); err == nil && info.Mode().Perm()&0o044 != 0 {
				anyWorldReadable = true
			}
		}
		if !anyWorldReadable {
			t.Fatalf("RED expected a world-readable backup in shared TempDir")
		}
		// Clean up the leaked artifacts the RED run produced.
		for _, f := range found {
			_ = os.Remove(f)
		}
		return
	}

	// GREEN guards.
	// (1) No backup may be written into shared, world-traversable os.TempDir().
	if leaks := findBackupFiles(os.TempDir()); len(leaks) > 0 {
		for _, f := range leaks {
			_ = os.Remove(f)
		}
		t.Fatalf("createBackup leaked %d file(s) into shared TempDir (CONST-042)", len(leaks))
	}

	// (2) The backup must live under a PRIVATE dir mode 0700, file mode 0600.
	backupDir := configBackupDir()
	di, err := os.Stat(backupDir)
	if err != nil {
		t.Fatalf("backup dir %q missing: %v", backupDir, err)
	}
	if !di.IsDir() {
		t.Fatalf("backup path %q is not a directory", backupDir)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("backup dir perm %#o, want 0700 (CONST-042)", di.Mode().Perm())
	}
	backups := findBackupFiles(backupDir)
	if len(backups) == 0 {
		t.Fatalf("no backup produced in private dir %q", backupDir)
	}
	for _, f := range backups {
		assertSecretFileMode(t, f)
	}
}

// findBackupFiles returns helix config backup files directly inside dir.
func findBackupFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "helix_config_backup_") && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}
