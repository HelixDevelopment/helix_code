package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

// withIsolatedEnv runs fn with HOME pointing at homeDir and the working
// directory set to cwd. It saves and restores both, plus the named env vars
// so tests do not bleed into each other.
func withIsolatedEnv(t *testing.T, homeDir, cwd string, vars []string, fn func()) {
	t.Helper()

	prevHome := os.Getenv("HOME")
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	saved := make(map[string]string, len(vars))
	hadVar := make(map[string]bool, len(vars))
	for _, v := range vars {
		val, ok := os.LookupEnv(v)
		hadVar[v] = ok
		saved[v] = val
		os.Unsetenv(v)
	}

	t.Setenv("HOME", homeDir)
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	defer func() {
		_ = os.Chdir(prevWD)
		t.Setenv("HOME", prevHome)
		for _, v := range vars {
			if hadVar[v] {
				os.Setenv(v, saved[v])
			} else {
				os.Unsetenv(v)
			}
		}
	}()

	fn()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadAPIKeys_FromShellFormat(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"), "export FOO=bar\n")

	withIsolatedEnv(t, home, cwd, []string{"FOO"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("FOO"); got != "bar" {
			t.Fatalf("FOO=%q want bar", got)
		}
	})
}

func TestLoadAPIKeys_FromEnvFile(t *testing.T) {
	home := t.TempDir() // no api_keys.sh inside
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".env"), "FOO=baz\n")

	withIsolatedEnv(t, home, cwd, []string{"FOO"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("FOO"); got != "baz" {
			t.Fatalf("FOO=%q want baz", got)
		}
	})
}

func TestLoadAPIKeys_PrefersShellOverEnv(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"), "export FOO=from_sh\n")
	writeFile(t, filepath.Join(cwd, ".env"), "FOO=from_env\n")

	withIsolatedEnv(t, home, cwd, []string{"FOO"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("FOO"); got != "from_sh" {
			t.Fatalf("FOO=%q want from_sh (shell must win)", got)
		}
	})
}

func TestLoadAPIKeys_StripsQuotes(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"),
		"export DQ=\"double quoted\"\nexport SQ='single quoted'\n")

	withIsolatedEnv(t, home, cwd, []string{"DQ", "SQ"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("DQ"); got != "double quoted" {
			t.Fatalf("DQ=%q want %q", got, "double quoted")
		}
		if got := os.Getenv("SQ"); got != "single quoted" {
			t.Fatalf("SQ=%q want %q", got, "single quoted")
		}
	})
}

func TestLoadAPIKeys_IgnoresComments(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"),
		"# this is a comment\nexport REAL=value\n# trailing comment\n")

	withIsolatedEnv(t, home, cwd, []string{"REAL"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("REAL"); got != "value" {
			t.Fatalf("REAL=%q want value", got)
		}
	})
}

func TestLoadAPIKeys_IgnoresBlank(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"),
		"\n\n   \nexport AFTER_BLANKS=ok\n\n")

	withIsolatedEnv(t, home, cwd, []string{"AFTER_BLANKS"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if got := os.Getenv("AFTER_BLANKS"); got != "ok" {
			t.Fatalf("AFTER_BLANKS=%q want ok", got)
		}
	})
}

func TestLoadAPIKeys_HandlesMissingExport(t *testing.T) {
	// In an api_keys.sh file, lines without `export ` MUST be skipped so the
	// two formats stay distinct. (NOPE_= would otherwise pollute the env.)
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, "api_keys.sh"),
		"NOPE=should_not_load\nexport YEP=loaded\n")

	withIsolatedEnv(t, home, cwd, []string{"NOPE", "YEP"}, func() {
		if err := LoadAPIKeys(); err != nil {
			t.Fatalf("LoadAPIKeys: %v", err)
		}
		if _, ok := os.LookupEnv("NOPE"); ok {
			t.Fatalf("NOPE was set; lines without `export ` must be skipped in shell file")
		}
		if got := os.Getenv("YEP"); got != "loaded" {
			t.Fatalf("YEP=%q want loaded", got)
		}
	})
}

func TestLoadAPIKeys_NeitherFound_ReturnsError(t *testing.T) {
	home := t.TempDir()
	// Use a leaf cwd with no .env anywhere along its parent chain by
	// creating a deeply nested dir under TempDir; t.TempDir parents
	// (under /tmp) generally do not contain a .env, so this is reliable.
	cwd := t.TempDir()
	deep := filepath.Join(cwd, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	withIsolatedEnv(t, home, deep, nil, func() {
		err := LoadAPIKeys()
		if err == nil {
			t.Fatalf("LoadAPIKeys: expected error when neither file present, got nil")
		}
	})
}
