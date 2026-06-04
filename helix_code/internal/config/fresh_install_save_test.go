package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveHelixConfig_FreshInstall_CreatesMissingDir is the W4C regression guard
// (§11.4.115 RED→GREEN). On a fresh machine ~/.config/helixcode/ does not exist;
// SaveHelixConfig / LoadHelixConfig must create the parent directory tree before
// writing, otherwise `helix-config reset --force` fails with
// "no such file or directory" on a clean install.
//
// RED (pre-fix, saveConfigLocked lacks os.MkdirAll): SaveHelixConfig returns the
// missing-dir error and no file is created.
// GREEN (post-fix): the nested dir tree is created and the config file is written.
func TestSaveHelixConfig_FreshInstall_CreatesMissingDir(t *testing.T) {
	// Point HELIX_CONFIG_PATH at a nested dir that does NOT exist yet — the
	// fresh-install scenario. t.Setenv restores the prior value automatically.
	base := t.TempDir()
	nested := filepath.Join(base, "does", "not", "exist", "yet", "config.json")
	t.Setenv("HELIX_CONFIG_PATH", nested)

	if _, err := os.Stat(filepath.Dir(nested)); !os.IsNotExist(err) {
		t.Fatalf("precondition: parent dir must not exist, stat err=%v", err)
	}

	cfg := getDefaultConfig()
	if err := SaveHelixConfig(cfg); err != nil {
		t.Fatalf("SaveHelixConfig on fresh install must create the missing dir and succeed, got: %v", err)
	}

	// The file must really exist on disk (anti-bluff: assert the artefact).
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("config file must exist after save, stat err=%v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("config file must be non-empty after save, size=0")
	}

	// Round-trip: LoadHelixConfig must read it back from the freshly-created tree.
	if _, err := LoadHelixConfig(); err != nil {
		t.Fatalf("LoadHelixConfig must succeed after fresh-install save, got: %v", err)
	}
}

// TestExportConfig_CreatesMissingDir guards the sibling writer ExportConfig,
// which has the same missing-MkdirAll gap when handed a path in a non-existent
// directory tree.
func TestExportConfig_CreatesMissingDir(t *testing.T) {
	base := t.TempDir()
	mgr := &ConfigManager{configPath: filepath.Join(base, "config.json")}
	mgr.config = getDefaultConfig()

	exportPath := filepath.Join(base, "export", "nested", "exported.json")
	if err := mgr.ExportConfig(exportPath); err != nil {
		t.Fatalf("ExportConfig must create the missing dir and succeed, got: %v", err)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("exported file must exist, stat err=%v", err)
	}
}
