package main

import (
	"os"
	"path/filepath"
	"testing"
)

// redMode mirrors the §11.4.115 polarity switch for the D-3 startup-wiring guard.
//
//	RED_MODE=1 (default): reproduce the DEAD-CODE defect — when the loader is
//	            NOT invoked at startup, a key supplied only via .env is NOT
//	            recognized (the pre-fix state: secrets.LoadAPIKeys never called).
//	RED_MODE=0: the GREEN guard — loadAPIKeysAtStartup() (now wired into main())
//	            recognizes the .env-only key.
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// TestLoadAPIKeys_WiredAtStartup proves D-3: a provider key present ONLY in a
// walked-up .env (no shell export) is recognized by the CLI startup path.
func TestLoadAPIKeys_WiredAtStartup(t *testing.T) {
	const key = "HELIX_SP1_DEEPSEEK_API_KEY"

	home := t.TempDir() // no api_keys.sh -> forces the .env path
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, ".env"), []byte(key+"=sk-deepseek-test\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	prevWD, _ := os.Getwd()
	if _, ok := os.LookupEnv(key); ok {
		t.Fatalf("precondition: %s must be unset before the test", key)
	}
	t.Setenv("HOME", home)
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
		os.Unsetenv(key)
	})

	if redMode() {
		// RED: loader NOT invoked -> the .env-only key is invisible to env.
		if _, ok := os.LookupEnv(key); ok {
			t.Fatalf("RED expected %s to be ABSENT (loader not wired), but it was present", key)
		}
		return
	}

	// GREEN: the wired startup helper recognizes the .env-only key.
	if !loadAPIKeysAtStartup() {
		t.Fatalf("loadAPIKeysAtStartup() returned false; expected the .env source to load")
	}
	if got := os.Getenv(key); got != "sk-deepseek-test" {
		t.Fatalf("%s=%q want sk-deepseek-test (key recognized from .env at startup)", key, got)
	}
}
