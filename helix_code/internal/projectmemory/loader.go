// Package projectmemory — loader.go (P2-F24-T03).
//
// MemoryLoader.Discover walks parent directories from the given cwd up to a
// .git directory marker (or filesystem root) looking for the first matching
// DiscoveryFilenames entry (case-insensitive). It then reads the user
// overlay at $XDG_CONFIG_HOME/helixcode/memory.md (or
// $HOME/.config/helixcode/memory.md fallback).
//
// Files larger than MaxMemoryBytes are read-truncated and the corresponding
// Truncated* flag is set — silent truncation is forbidden by the anti-bluff
// mandate (spec §5.2 Bluff #4).
//
// Missing files are NOT errors. An empty Memory{ProjectPath: ""} is the
// canonical "no memory" result; downstream code (BaseAgent.getSystemPrompt)
// detects it via Memory.Render() == "".
package projectmemory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MemoryLoader encapsulates project + user memory discovery and reading.
// Stateless across calls — Discover may be invoked concurrently from
// different cwds; xdgConfigHome is captured once at construction so per-call
// env reads stay deterministic for tests.
//
// Per CONST-042: the logger receives only paths and byte counts at INFO,
// never the file body.
type MemoryLoader struct {
	xdgConfigHome string
	log           *zap.Logger
}

// NewMemoryLoader resolves $XDG_CONFIG_HOME (falling back to
// $HOME/.config) and constructs a MemoryLoader. Passing nil log is safe;
// it falls back to zap.NewNop() so the loader is always log-safe.
//
// Note: xdgConfigHome is captured at construction. Callers that want to
// observe a runtime $XDG_CONFIG_HOME change must construct a new loader.
// Tests use t.Setenv + a fresh NewMemoryLoader to exercise this.
func NewMemoryLoader(log *zap.Logger) *MemoryLoader {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdg = filepath.Join(home, ".config")
		}
	}
	if xdg != "" && !filepath.IsAbs(xdg) {
		if abs, err := filepath.Abs(xdg); err == nil {
			xdg = abs
		}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &MemoryLoader{xdgConfigHome: xdg, log: log}
}

// Discover walks parent dirs from cwd to find a project memory file, reads
// it (capped at MaxMemoryBytes), then reads the user overlay (if present).
//
// Errors propagate ONLY when an existing file cannot be read (permission
// denied, I/O error). Missing files yield empty fields + nil error.
//
// On read errors the partially-loaded Memory is NOT returned — the registry
// keeps its previous value.
func (l *MemoryLoader) Discover(cwd string) (Memory, error) {
	projPath, err := l.findProjectMemory(cwd)
	if err != nil {
		return Memory{}, err
	}

	var (
		project   string
		truncProj bool
	)
	if projPath != "" {
		b, rerr := os.ReadFile(projPath)
		if rerr != nil {
			return Memory{}, fmt.Errorf("projectmemory: read %s: %w", projPath, rerr)
		}
		if len(b) > MaxMemoryBytes {
			b = b[:MaxMemoryBytes]
			truncProj = true
			l.log.Warn("project memory file truncated",
				zap.String("path", projPath),
				zap.Int("max_bytes", MaxMemoryBytes))
		}
		project = string(b)
	}

	var (
		userPath  string
		user      string
		truncUser bool
	)
	if l.xdgConfigHome != "" {
		userPath = filepath.Join(l.xdgConfigHome, "helixcode", "memory.md")
		b, rerr := os.ReadFile(userPath)
		switch {
		case rerr == nil:
			if len(b) > MaxMemoryBytes {
				b = b[:MaxMemoryBytes]
				truncUser = true
				l.log.Warn("user memory file truncated",
					zap.String("path", userPath),
					zap.Int("max_bytes", MaxMemoryBytes))
			}
			user = string(b)
		case os.IsNotExist(rerr):
			// Missing user overlay is normal — clear path so /memory status reports it correctly.
			userPath = ""
		default:
			return Memory{}, fmt.Errorf("projectmemory: read %s: %w", userPath, rerr)
		}
	}

	return Memory{
		Project:          project,
		User:             user,
		ProjectPath:      projPath,
		UserPath:         userPath,
		LoadedAt:         time.Now(),
		TruncatedProject: truncProj,
		TruncatedUser:    truncUser,
	}, nil
}

// findProjectMemory walks from start up to a .git-directory marker (or the
// filesystem root) looking for the first matching DiscoveryFilenames entry.
//
// Match is case-INSENSITIVE: a two-pass scan tries each canonical name
// against os.Stat (fast path on case-insensitive filesystems like APFS),
// then falls back to listing the directory and case-insensitively comparing
// entries (correct path on case-sensitive filesystems like ext4).
//
// Returns ("", nil) when no match found before the bound. Returns ("", err)
// only on I/O errors (permission denied on intermediate dirs).
func (l *MemoryLoader) findProjectMemory(start string) (string, error) {
	if start == "" {
		return "", nil
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("projectmemory: resolve %s: %w", start, err)
	}

	for {
		// Phase 1: try canonical names directly.
		for _, name := range DiscoveryFilenames {
			candidate := filepath.Join(dir, name)
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}

		// Phase 2: case-insensitive directory scan (handles ext4 with
		// agents.md vs AGENTS.md differences).
		if entries, readErr := os.ReadDir(dir); readErr == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				lower := strings.ToLower(entry.Name())
				for _, want := range DiscoveryFilenames {
					if lower == strings.ToLower(want) {
						return filepath.Join(dir, entry.Name()), nil
					}
				}
			}
		}

		// Stop at a git root marker.
		if info, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil && info.IsDir() {
			return "", nil
		}

		// Walk up. Stop at filesystem root.
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
