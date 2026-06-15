package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHXC098_LoadHelixConfig_NoVersion is the permanent regression guard
// (§11.4.135) for HXC-098 on the REAL client path: terminal_ui + helix_config
// load via LoadHelixConfig -> LoadConfig -> NewHelixConfigManager, which used a
// plain json.Unmarshal that merged NONE of the viper defaults Load() applies.
// An out-of-box / hand-written operator config.json that omits top-level
// `version` (and other defaulted fields like server.port) must still come back
// fully defaulted so validateConfig does not reject it on those grounds and the
// fresh user's status/system/version commands work.
//
// Pre-fix this FAILED ("version is required", then "server port must be between
// 1 and 65535"); the fix (loadConfigLocked decoding ON TOP of getDefaultConfig)
// merges every default. A missing JWT secret is a SEPARATE, legitimate security
// gate — NOT the HXC-098 defect — so this guard scopes its assertions to the
// fields HXC-098 was about.
func TestHXC098_LoadHelixConfig_NoVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(`{"application":{"name":"HelixCode"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HELIX_CONFIG_PATH", p)

	cfg, err := LoadHelixConfig()
	if err != nil {
		t.Fatalf("LoadHelixConfig failed: %v", err)
	}
	if cfg.Version == "" {
		t.Fatalf("HXC-098 REPRODUCED: LoadHelixConfig left Version empty for a version-less config.json")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		t.Fatalf("HXC-098 REPRODUCED: LoadHelixConfig left server.port=%d (defaults not merged)", cfg.Server.Port)
	}
	if vErr := validateConfig(cfg); vErr != nil {
		if strings.Contains(vErr.Error(), "version") || strings.Contains(vErr.Error(), "port") {
			t.Fatalf("HXC-098 REPRODUCED: validateConfig still rejects out-of-box config on a defaulted field: %v", vErr)
		}
		t.Logf("OK: HXC-098 fields defaulted (Version=%q, Port=%d); remaining gate is the unrelated security requirement: %v",
			cfg.Version, cfg.Server.Port, vErr)
		return
	}
	t.Logf("OK: LoadHelixConfig defaulted Version=%q, Port=%d and validateConfig passed", cfg.Version, cfg.Server.Port)
}
