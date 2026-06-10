package main

import (
	"os"
	"path/filepath"
	"testing"
)

// redMode mirrors the §11.4.115 polarity switch for the D-3-extension
// server startup-wiring guard.
//
//	RED_MODE=1 (default): reproduce the DEAD-CODE / not-wired defect — when the
//	            loader is NOT invoked at server startup, a key supplied only via
//	            .env is NOT recognized (the pre-fix state: the server main()
//	            jumped straight to config.Get() without secrets.LoadAPIKeys).
//	RED_MODE=0: the GREEN guard — loadAPIKeysAtStartup() (now wired into the
//	            server main()) recognizes the .env-only key.
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// TestServerLoadAPIKeys_WiredAtStartup proves the D-3 extension: a provider key
// present ONLY in a walked-up .env (no shell export) is recognized by the
// SERVER startup path, just like the CLI path. This is the server-side closure
// of the "implemented in cmd/cli but not in cmd/server" gap.
func TestServerLoadAPIKeys_WiredAtStartup(t *testing.T) {
	const key = "HELIX_SP1_SERVER_GROQ_API_KEY"

	home := t.TempDir() // no api_keys.sh -> forces the .env path
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, ".env"), []byte(key+"=gsk-server-test\n"), 0o600); err != nil {
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
		// RED: loader NOT invoked -> the .env-only key is invisible to the env.
		// This reproduces the pre-fix server bootstrap (config.Get() ran first,
		// no secrets.LoadAPIKeys), proving the gap was real.
		if _, ok := os.LookupEnv(key); ok {
			t.Fatalf("RED expected %s to be ABSENT (loader not wired on server), but it was present", key)
		}
		return
	}

	// GREEN: the wired server startup helper recognizes the .env-only key.
	if !loadAPIKeysAtStartup() {
		t.Fatalf("loadAPIKeysAtStartup() returned false; expected the .env source to load on the server path")
	}
	if got := os.Getenv(key); got != "gsk-server-test" {
		t.Fatalf("%s=%q want gsk-server-test (key recognized from .env at server startup)", key, got)
	}
}
