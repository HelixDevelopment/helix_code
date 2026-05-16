package approval

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// staticEnv returns an envLookup function that reads from the supplied map and
// returns "" for any unrecognised key. Centralising it here keeps test cases
// terse and deterministic — no real os.Getenv calls leak in.
func staticEnv(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

// writeTempConfig writes content to a fresh tempdir and returns the absolute
// file path. t.TempDir handles cleanup. Used to build LoadConfigFile / Select
// fixtures without polluting the working tree.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "approval.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTempConfig: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Select — precedence
// ---------------------------------------------------------------------------

func TestSelect_FlagWinsAll(t *testing.T) {
	cfgPath := writeTempConfig(t, "mode: suggest\n")
	mode, src, err := Select(SelectorInput{
		Flag:       "auto-edit",
		Env:        "full-auto",
		ConfigPath: cfgPath,
		EnvLookup:  staticEnv(map[string]string{EnvVarName: "full-auto"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeAutoEdit {
		t.Errorf("mode: got %q, want %q", mode, ModeAutoEdit)
	}
	if src != SourceFlag {
		t.Errorf("source: got %v, want %v", src, SourceFlag)
	}
}

func TestSelect_EnvWhenNoFlag(t *testing.T) {
	mode, src, err := Select(SelectorInput{
		Env:       "full-auto",
		EnvLookup: staticEnv(map[string]string{EnvVarName: "full-auto"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeFullAuto {
		t.Errorf("mode: got %q, want %q", mode, ModeFullAuto)
	}
	if src != SourceEnv {
		t.Errorf("source: got %v, want %v", src, SourceEnv)
	}
}

func TestSelect_ConfigWhenNoFlagOrEnv(t *testing.T) {
	cfgPath := writeTempConfig(t, "mode: dangerously-bypass\n")
	mode, src, err := Select(SelectorInput{
		ConfigPath: cfgPath,
		EnvLookup:  staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeDangerous {
		t.Errorf("mode: got %q, want %q", mode, ModeDangerous)
	}
	if src != SourceConfig {
		t.Errorf("source: got %v, want %v", src, SourceConfig)
	}
}

func TestSelect_DefaultWhenAllUnset(t *testing.T) {
	mode, src, err := Select(SelectorInput{
		EnvLookup: staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("mode: got %q, want %q", mode, ModeSuggest)
	}
	if src != SourceDefault {
		t.Errorf("source: got %v, want %v", src, SourceDefault)
	}
}

// ---------------------------------------------------------------------------
// Select — garbage-fall-through
// ---------------------------------------------------------------------------

func TestSelect_GarbageFlagFallsThrough(t *testing.T) {
	mode, src, err := Select(SelectorInput{
		Flag:      "banana",
		Env:       "auto-edit",
		EnvLookup: staticEnv(map[string]string{EnvVarName: "auto-edit"}),
	})
	if err == nil {
		t.Fatalf("expected error reporting bad flag value")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode in chain, got %v", err)
	}
	if mode != ModeAutoEdit {
		t.Errorf("mode: got %q, want %q", mode, ModeAutoEdit)
	}
	if src != SourceEnv {
		t.Errorf("source: got %v, want %v", src, SourceEnv)
	}
}

func TestSelect_GarbageEnvFallsThrough(t *testing.T) {
	cfgPath := writeTempConfig(t, "mode: full-auto\n")
	mode, src, err := Select(SelectorInput{
		Env:        "wat",
		ConfigPath: cfgPath,
		EnvLookup:  staticEnv(map[string]string{EnvVarName: "wat"}),
	})
	if err == nil {
		t.Fatalf("expected error reporting bad env value")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode in chain, got %v", err)
	}
	if mode != ModeFullAuto {
		t.Errorf("mode: got %q, want %q", mode, ModeFullAuto)
	}
	if src != SourceConfig {
		t.Errorf("source: got %v, want %v", src, SourceConfig)
	}
}

func TestSelect_GarbageConfigFallsThrough(t *testing.T) {
	cfgPath := writeTempConfig(t, "mode: not-a-mode\n")
	mode, src, err := Select(SelectorInput{
		ConfigPath: cfgPath,
		EnvLookup:  staticEnv(nil),
	})
	if err == nil {
		t.Fatalf("expected error reporting bad config value")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode in chain, got %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("mode: got %q, want %q", mode, ModeSuggest)
	}
	if src != SourceDefault {
		t.Errorf("source: got %v, want %v", src, SourceDefault)
	}
}

func TestSelect_AllGarbage_DefaultsToSuggest(t *testing.T) {
	cfgPath := writeTempConfig(t, "mode: pineapple\n")
	mode, src, err := Select(SelectorInput{
		Flag:       "banana",
		Env:        "wat",
		ConfigPath: cfgPath,
		EnvLookup:  staticEnv(map[string]string{EnvVarName: "wat"}),
	})
	if err == nil {
		t.Fatalf("expected aggregate error reporting at least one bad source")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode in chain, got %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("mode: got %q, want %q", mode, ModeSuggest)
	}
	if src != SourceDefault {
		t.Errorf("source: got %v, want %v", src, SourceDefault)
	}
}

// ---------------------------------------------------------------------------
// DefaultConfigPath
// ---------------------------------------------------------------------------

func TestDefaultConfigPath_XDGSet(t *testing.T) {
	got := DefaultConfigPath(staticEnv(map[string]string{
		"XDG_CONFIG_HOME": "/foo",
		"HOME":            "/home/ignored",
	}))
	want := "/foo/helixcode/approval.yaml"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_HOMEFallback(t *testing.T) {
	got := DefaultConfigPath(staticEnv(map[string]string{
		"HOME": "/home/u",
	}))
	want := "/home/u/.config/helixcode/approval.yaml"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_NeitherSet(t *testing.T) {
	// Documented: when neither XDG_CONFIG_HOME nor HOME is set we cannot form
	// an absolute path; the function returns "" so callers can fall straight
	// through to the built-in default mode without attempting a stat() on
	// nonsense input.
	got := DefaultConfigPath(staticEnv(nil))
	if got != "" {
		t.Errorf("expected empty string when neither env var set, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// LoadConfigFile
// ---------------------------------------------------------------------------

func TestLoadConfigFile_HappyPath(t *testing.T) {
	path := writeTempConfig(t, "mode: full-auto\n")
	mode, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeFullAuto {
		t.Errorf("got %q, want %q", mode, ModeFullAuto)
	}
}

func TestLoadConfigFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")
	mode, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("got %q, want %q (graceful default)", mode, ModeSuggest)
	}
}

func TestLoadConfigFile_BadYAML(t *testing.T) {
	path := writeTempConfig(t, "mode: [this is: not valid\n")
	mode, err := LoadConfigFile(path)
	if err == nil {
		t.Fatalf("expected error for malformed YAML")
	}
	if mode != ModeSuggest {
		t.Errorf("got %q, want %q on parse error", mode, ModeSuggest)
	}
}

func TestLoadConfigFile_InvalidMode(t *testing.T) {
	path := writeTempConfig(t, "mode: banana\n")
	mode, err := LoadConfigFile(path)
	if err == nil {
		t.Fatalf("expected error for invalid mode value")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode in chain, got %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("got %q, want %q on invalid mode", mode, ModeSuggest)
	}
}

func TestLoadConfigFile_EmptyFile(t *testing.T) {
	path := writeTempConfig(t, "")
	mode, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ModeSuggest {
		t.Errorf("got %q, want %q (empty file → default)", mode, ModeSuggest)
	}
}
