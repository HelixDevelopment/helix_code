package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

// redMode mirrors the §11.4.115 polarity switch for the secrets package.
//
//	RED_MODE=1 (default): assert the DEFECT — the loader OVERRIDES an
//	            already-exported shell var (current os.Setenv-unconditional).
//	RED_MODE=0: the GREEN guard — DECISION-1 gap-fill: an already-exported
//	            shell var WINS; the file value only fills gaps.
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// TestLoadAPIKeys_GapFillPrecedence (DECISION-1): a value already present in
// the process env (e.g. shell-exported) MUST NOT be overwritten by the
// .env / api_keys.sh file value. The file only fills gaps.
func TestLoadAPIKeys_GapFillPrecedence(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	// File would set FOO=from_file; but FOO is already exported as from_shell.
	writeFile(t, filepath.Join(home, "api_keys.sh"), "export FOO=from_file\nexport BAR=from_file\n")

	withIsolatedEnv(t, home, cwd, []string{"FOO", "BAR"}, func() {
		// Simulate an already-exported shell var (gap-fill must preserve it).
		os.Setenv("FOO", "from_shell")

		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}

		// BAR was unset → file fills the gap, both polarities.
		if got := os.Getenv("BAR"); got != "from_file" {
			t.Fatalf("BAR=%q want from_file (gap fill)", got)
		}

		got := os.Getenv("FOO")
		if redMode() {
			// RED: defect present — loader overrode the shell var.
			if got != "from_file" {
				t.Fatalf("RED expected defect (FOO overridden to from_file), got FOO=%q", got)
			}
			return
		}
		// GREEN: gap-fill — shell-exported value preserved.
		if got != "from_shell" {
			t.Fatalf("GREEN: FOO=%q want from_shell (already-exported var must win)", got)
		}
	})
}
