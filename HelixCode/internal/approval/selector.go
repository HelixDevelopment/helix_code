// selector.go (P2-F21-T03): pure resolver that picks the active ApprovalMode
// from four ordered sources — CLI flag, env var, YAML config file, and the
// built-in default. The Selector intentionally does no env reads of its own
// (it accepts an EnvLookup) and no construction; it is exercised exhaustively
// from selector_test.go.
//
// The precedence chain is:
//
//	1. Flag       (CLI --approval=<mode>)
//	2. Env        (HELIXCODE_APPROVAL=<mode>)
//	3. Config     (XDG-style YAML, schema: { mode: <mode> })
//	4. Default    (ModeSuggest — safest)
//
// Garbage values do NOT abort the chain. A bad flag falls through to env,
// a bad env to config, and so on, with the offending source's parse error
// aggregated into the returned error so callers (CLI / log shim) can print a
// warning while the user still gets a sensible runtime mode. This matches the
// "anti-bluff" principle: the selector never silently swallows a bad value
// nor pretends a garbage value resolved successfully.
//
// References:
//   - Spec 7128289 §3.2 (Selector)
//   - Plan bbb61de T03
//   - F12 T07 internal/llm/provider_factory.go (sibling Selector pattern)

package approval

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EnvVarName is the canonical environment variable consulted by Select for the
// approval-mode override. Exported so /approval status and CLI help text can
// surface the name without hard-coding it at the call site.
const EnvVarName = "HELIXCODE_APPROVAL"

// SelectorInput captures all four sources for the selector. Each field is
// independently optional; empty/zero values disable that source. The field
// order matches the precedence order applied by Select.
type SelectorInput struct {
	// Flag is the CLI flag value (--approval=<mode>). Highest precedence.
	// Empty string means "user did not pass the flag".
	Flag string

	// Env is the value the caller already pulled from the env var. The
	// selector also consults EnvLookup(EnvVarName) — Env is the fallback
	// path for callers that prefer to inject the value rather than rely on
	// the lookup, and the two are unioned (first non-empty wins).
	Env string

	// ConfigPath is the absolute path to the YAML config file. Empty string
	// disables the config source entirely. A non-empty path that does not
	// exist is treated as "config absent" (graceful), not as an error.
	ConfigPath string

	// EnvLookup is the function used to read environment variables. Tests
	// inject a deterministic map-backed implementation. Production callers
	// should pass os.Getenv (or leave nil — Select falls back to os.Getenv).
	EnvLookup func(string) string
}

// Select resolves the active ApprovalMode using flag > env > config > default
// precedence. Garbage values fall through to the next source AND aggregate
// into the returned error so the caller can warn the user without losing the
// runtime mode.
//
// Returns:
//   - ApprovalMode    : the resolved mode (always one of the four canonical
//     modes; ModeSuggest if every source was empty/garbage).
//   - ResolvedSource  : which source ultimately won.
//   - error           : nil if the winning source parsed cleanly; otherwise
//     a wrapped error chain containing every parse failure
//     (errors.Is(err, ErrInvalidMode) is true for any).
func Select(input SelectorInput) (ApprovalMode, ResolvedSource, error) {
	lookup := input.EnvLookup
	if lookup == nil {
		lookup = os.Getenv
	}

	var aggErrs []error

	// 1. Flag.
	if input.Flag != "" {
		m, err := ParseMode(input.Flag)
		if err == nil {
			// Keep going only to check if there is anything else to report —
			// but we already have a winner, so return immediately. Any prior
			// errors above this branch don't exist (flag is the first
			// source), so a clean nil return is correct.
			return m, SourceFlag, nil
		}
		aggErrs = append(aggErrs, fmt.Errorf("flag %q: %w", input.Flag, err))
	}

	// 2. Env. Either the explicit Env field on the struct or the live lookup
	// is consulted; the explicit field wins to give callers a deterministic
	// override path under test, with EnvLookup as the production fallback.
	envVal := input.Env
	if envVal == "" {
		envVal = lookup(EnvVarName)
	}
	if envVal != "" {
		m, err := ParseMode(envVal)
		if err == nil {
			// Env wins. Surface any upstream parse error so the caller
			// can warn the user that --approval was garbage even though
			// the env var saved them.
			return m, SourceEnv, errors.Join(aggErrs...)
		}
		aggErrs = append(aggErrs, fmt.Errorf("env %s=%q: %w", EnvVarName, envVal, err))
	}

	// 3. Config file.
	if input.ConfigPath != "" {
		// Probe existence first — a missing config is "source absent",
		// not "source provided value ModeSuggest". This matters because
		// LoadConfigFile returns (ModeSuggest, nil) for missing files
		// (graceful default) but in Select that should fall through to
		// SourceDefault, not get reported as SourceConfig.
		if _, statErr := os.Stat(input.ConfigPath); statErr == nil {
			m, err := LoadConfigFile(input.ConfigPath)
			if err == nil {
				return m, SourceConfig, errors.Join(aggErrs...)
			}
			aggErrs = append(aggErrs, fmt.Errorf("config %s: %w", input.ConfigPath, err))
		}
	}

	// 4. Default.
	if len(aggErrs) > 0 {
		return ModeSuggest, SourceDefault, errors.Join(aggErrs...)
	}
	return ModeSuggest, SourceDefault, nil
}

// DefaultConfigPath returns the canonical YAML location:
//
//	$XDG_CONFIG_HOME/helixcode/approval.yaml   (preferred)
//	$HOME/.config/helixcode/approval.yaml      (fallback)
//	""                                         (neither set — caller must
//	                                            treat as "no config path")
//
// The function is pure: it consults envLookup only, never the real
// environment, so tests can drive it deterministically. Callers in production
// pass os.Getenv.
func DefaultConfigPath(envLookup func(string) string) string {
	if envLookup == nil {
		envLookup = os.Getenv
	}
	if xdg := envLookup("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "helixcode", "approval.yaml")
	}
	if home := envLookup("HOME"); home != "" {
		return filepath.Join(home, ".config", "helixcode", "approval.yaml")
	}
	// Documented sentinel: empty string signals "no canonical config path
	// available". Select treats this as "config source disabled" and falls
	// straight through to the built-in default.
	return ""
}

// LoadConfigFile parses the YAML at path. Schema:
//
//	mode: <suggest|auto-edit|full-auto|dangerously-bypass>
//
// Behaviour:
//   - missing file              → (ModeSuggest, nil)         graceful default
//   - present but empty         → (ModeSuggest, nil)         graceful default
//   - malformed YAML            → (ModeSuggest, error)
//   - mode key absent           → (ModeSuggest, nil)         graceful default
//   - mode key with bad value   → (ModeSuggest, ErrInvalidMode-wrapped error)
//   - mode key with good value  → (parsed mode, nil)
func LoadConfigFile(path string) (ApprovalMode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ModeSuggest, nil
		}
		return ModeSuggest, fmt.Errorf("read approval config %s: %w", path, err)
	}
	if len(data) == 0 {
		return ModeSuggest, nil
	}

	var cfg approvalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ModeSuggest, fmt.Errorf("parse approval config %s: %w", path, err)
	}
	if cfg.Mode == "" {
		// Empty mode key is the same as "config doesn't speak to mode" —
		// graceful default, not an error.
		return ModeSuggest, nil
	}
	mode, err := ParseMode(cfg.Mode)
	if err != nil {
		return ModeSuggest, fmt.Errorf("approval config %s: %w", path, err)
	}
	return mode, nil
}

// approvalConfig is the on-disk YAML schema. Kept private to the package; the
// public surface is (mode, error) via LoadConfigFile.
type approvalConfig struct {
	Mode string `yaml:"mode"`
}
