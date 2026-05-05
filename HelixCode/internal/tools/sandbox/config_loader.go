package sandbox

// config_loader.go (P1-F14-T08): YAML loader + secret-safe writer for the
// on-disk sandbox.yaml configuration file. Two write paths mirror the F12
// wizard_writer pattern:
//
//   WriteSandboxConfig(path, cfg)
//     Uses O_WRONLY|O_CREATE|O_EXCL with mode 0600. FAILS if the file
//     already exists. Parent directories are created with mode 0700 so
//     the file (which carries user-deny patterns and bind-mount paths)
//     is not enumerable through a wider parent.
//
//   OverwriteSandboxConfig(path, cfg)
//     Atomic replace via temp-file + rename(2). Same secret-safe modes
//     (0700 dir, 0600 file). Used by /sandbox commands that explicitly
//     intend to update an existing config.
//
// Read path:
//
//   LoadSandboxConfig(path)
//     Returns DefaultSandboxConfig() with no error when path doesn't
//     exist (sandbox without a config is allowed). On parse error or
//     validation failure (negative memory/cpu) returns (zero, error).
//     A zero Timeout in the loaded YAML is normalised to the 30s default
//     so downstream backends never receive a "no timeout" policy.
//
// Anti-bluff anchor: this file ALWAYS writes to a real disk; tests use
// t.TempDir() and os.Stat to verify mode. No "in-memory writer" fallback
// or simulation path — file IO is the product, not a test-only seam.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrSandboxConfigExists is returned by WriteSandboxConfig when the path
// already exists. It wraps os.ErrExist so callers can use
// errors.Is(err, os.ErrExist).
var ErrSandboxConfigExists = fmt.Errorf("sandbox config already exists: %w", os.ErrExist)

// defaultSandboxTimeout mirrors DefaultSandboxPolicy().Timeout. We
// duplicate the constant here so the loader's "fill zero with default"
// behaviour stays decoupled from accidental future edits to the policy
// constructor.
const defaultSandboxTimeout = 30 * time.Second

// DefaultConfigPath returns the canonical sandbox.yaml path resolved from
// the supplied envLookup function. Resolution order (XDG-Base-Dir spec):
//
//  1. $XDG_CONFIG_HOME/helixcode/sandbox.yaml when XDG_CONFIG_HOME is set
//  2. $HOME/.config/helixcode/sandbox.yaml when HOME is set
//  3. .helixcode/sandbox.yaml relative-fallback when neither is set
//
// The function is pure — it accepts envLookup so tests can drive it
// without mutating the process environment.
func DefaultConfigPath(envLookup func(string) string) string {
	if envLookup != nil {
		if xdg := envLookup("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "helixcode", "sandbox.yaml")
		}
		if home := envLookup("HOME"); home != "" {
			return filepath.Join(home, ".config", "helixcode", "sandbox.yaml")
		}
	}
	return filepath.Join(".helixcode", "sandbox.yaml")
}

// LoadSandboxConfig reads sandbox.yaml from disk and returns a
// SandboxConfig.
//
// Behaviour:
//   - Missing file → (DefaultSandboxConfig(), nil). A sandbox without a
//     config is a supported configuration; the caller falls back to safe
//     defaults.
//   - Bad YAML → (zero, error wrapping the yaml package error).
//   - Negative memory_limit_mb or cpu_limit_pct → (zero, validation error).
//   - Zero Timeout → normalised to 30s (defaultSandboxTimeout) before
//     return so downstream backends never see "no timeout" policy.
func LoadSandboxConfig(path string) (SandboxConfig, error) {
	if path == "" {
		return SandboxConfig{}, errors.New("LoadSandboxConfig: empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultSandboxConfig(), nil
		}
		return SandboxConfig{}, fmt.Errorf("LoadSandboxConfig: read %s: %w", path, err)
	}

	var cfg SandboxConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return SandboxConfig{}, fmt.Errorf("LoadSandboxConfig: yaml unmarshal %s: %w", path, err)
	}

	if err := validateSandboxConfig(&cfg); err != nil {
		return SandboxConfig{}, fmt.Errorf("LoadSandboxConfig: validate %s: %w", path, err)
	}

	if cfg.DefaultPolicy.Timeout <= 0 {
		cfg.DefaultPolicy.Timeout = defaultSandboxTimeout
	}

	return cfg, nil
}

// WriteSandboxConfig serialises cfg to YAML and writes it atomically to
// path with mode 0600. Parent directories are created with mode 0700 if
// missing. Uses O_EXCL — refuses to overwrite an existing file. Use
// OverwriteSandboxConfig for explicit replace semantics.
func WriteSandboxConfig(path string, cfg SandboxConfig) error {
	if path == "" {
		return errors.New("WriteSandboxConfig: empty path")
	}
	if err := ensureSandboxParentDir(path); err != nil {
		return err
	}

	data, err := marshalSandboxConfig(cfg)
	if err != nil {
		return fmt.Errorf("WriteSandboxConfig: marshal: %w", err)
	}

	// O_EXCL is the load-bearing flag. mode 0600 is the load-bearing
	// permission. Anything weaker leaks user-deny patterns / bind paths.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrSandboxConfigExists
		}
		return fmt.Errorf("WriteSandboxConfig: open: %w", err)
	}

	cleanupOnError := true
	defer func() {
		_ = f.Close()
		if cleanupOnError {
			_ = os.Remove(path)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("WriteSandboxConfig: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("WriteSandboxConfig: fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("WriteSandboxConfig: close: %w", err)
	}
	cleanupOnError = false
	return nil
}

// OverwriteSandboxConfig atomically replaces the file at path with the
// marshalled cfg. Uses a temp file in the same directory plus rename(2)
// so readers never observe a partially-written file. Final file mode is
// 0600; final parent dir mode is 0700.
func OverwriteSandboxConfig(path string, cfg SandboxConfig) error {
	if path == "" {
		return errors.New("OverwriteSandboxConfig: empty path")
	}
	if err := ensureSandboxParentDir(path); err != nil {
		return err
	}

	data, err := marshalSandboxConfig(cfg)
	if err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: marshal: %w", err)
	}

	dir := filepath.Dir(path)

	// Create temp file in the same directory so rename(2) is atomic
	// (cross-fs renames are NOT atomic on POSIX). os.CreateTemp gives a
	// unique name and mode 0600 by default.
	tmp, err := os.CreateTemp(dir, ".sandbox-*.tmp")
	if err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	cleanupTmp := true
	defer func() {
		_ = tmp.Close()
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	// CreateTemp on POSIX gives 0600 already; chmod is defensive for
	// platforms (or umasks) that bend the bits.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: chmod tmp: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: close tmp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("OverwriteSandboxConfig: rename: %w", err)
	}
	cleanupTmp = false
	return nil
}

// validateSandboxConfig rejects nonsensical input (negative limits) and
// is the central point where we draw a line between "file parsed" and
// "config is usable". Best-effort BindMount source-existence checks are
// intentionally NOT performed here — a missing source path may be
// intentional (the user mounts a directory they will create later).
func validateSandboxConfig(cfg *SandboxConfig) error {
	if cfg.DefaultPolicy.MemoryLimitMB < 0 {
		return fmt.Errorf("default_policy.memory_limit_mb: %d is negative", cfg.DefaultPolicy.MemoryLimitMB)
	}
	if cfg.DefaultPolicy.CPULimitPct < 0 {
		return fmt.Errorf("default_policy.cpu_limit_pct: %d is negative", cfg.DefaultPolicy.CPULimitPct)
	}
	return nil
}

// marshalSandboxConfig wraps yaml.Marshal so the serialisation strategy
// (and any future custom encoder) lives in one place. We currently rely
// on the default encoder which serialises time.Duration as the underlying
// int64 nanosecond count — not the prettiest, but perfectly
// round-trippable. A future enhancement can introduce a custom Duration
// type with `30s`-style marshaling without touching call sites.
func marshalSandboxConfig(cfg SandboxConfig) ([]byte, error) {
	return yaml.Marshal(cfg)
}

// ensureSandboxParentDir guarantees that the parent directory of path
// exists with mode 0700. If the directory already exists we leave its
// permissions alone — overriding user-set perms on an existing dir would
// be presumptuous (and potentially worse than the current state).
func ensureSandboxParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat parent dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir parent %s: %w", dir, err)
	}
	// MkdirAll honours umask — re-chmod the deepest dir we just made so
	// we get a deterministic 0700 even when umask is 022.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod parent %s: %w", dir, err)
	}
	return nil
}
