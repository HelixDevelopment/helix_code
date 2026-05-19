// Package secrets implements the HelixCode API-key loader (P1.5-WP4).
//
// LoadAPIKeys reads credentials from $HOME/api_keys.sh first, then falls back
// to a .env file walked up from the current working directory, and applies
// them to the process environment via os.Setenv.
//
// Constitutional anchors:
//   - CONST-042 (No-Secret-Leak): values are NEVER logged at any level.
//   - CONST-035 (Zero-Bluff): real file I/O, real os.Setenv, no simulation.
package secrets

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LoadAPIKeys loads API keys into the process env. Precedence:
//  1. $HOME/api_keys.sh (POSIX shell with `export VAR=value` lines).
//  2. .env file (walks up from cwd to find one).
//
// Returns nil if either source loaded; returns error if neither was found.
// Idempotent: callers may invoke at startup; subsequent calls re-apply
// (harmless because os.Setenv overwrites with the same value).
//
// Variable values are NOT logged at any level (CONST-042).
func LoadAPIKeys() error {
	home, _ := os.UserHomeDir()
	if home != "" {
		shPath := filepath.Join(home, "api_keys.sh")
		if _, err := os.Stat(shPath); err == nil {
			return loadFromShell(shPath)
		}
	}

	envPath, ok := findEnvFile()
	if !ok {
		// CONST-046: resolve through the translator seam so non-English
		// operators see this boot-time error in their active locale.
		// CONST-042 §12.1: no secret material is involved here — the
		// message describes only the ABSENCE of the secret files.
		return errors.New(tr(context.Background(), "internal_secrets_no_source_found", nil))
	}
	return loadFromEnv(envPath)
}

// loadFromShell parses lines like `export VAR=value` and sets them in
// os.Environ. Quotes (single + double) are stripped. Comments and blank
// lines are ignored. Lines without the `export ` prefix are skipped so the
// shell-format and env-format files cannot accidentally overlap.
func loadFromShell(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "export ") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := line[:eq]
		val := stripQuotes(line[eq+1:])
		os.Setenv(key, val)
	}
	return scanner.Err()
}

// loadFromEnv parses lines like `VAR=value` (no `export` prefix).
func loadFromEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := line[:eq]
		val := stripQuotes(line[eq+1:])
		os.Setenv(key, val)
	}
	return scanner.Err()
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func findEnvFile() (string, bool) {
	dir, _ := os.Getwd()
	for dir != "/" && dir != "" {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath, true
		}
		dir = filepath.Dir(dir)
	}
	return "", false
}
