package version

import (
	"context"
	"fmt"
	"runtime"
)

// These variables are set at build time using -ldflags
var (
	// Version is the semantic version
	Version = "0.1.0"

	// GitCommit is the git commit hash
	GitCommit = "dev"

	// BuildDate is the build timestamp
	BuildDate = "unknown"

	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
)

// CONST-046 message IDs migrated in round-183 (2026-05-19, Phase 4
// round 76). Constants — not literals — so a future audit/grep can
// trace every emission site to a bundle entry.
const (
	msgIDShortWithCommit   = "internal_version_short_with_commit"
	msgIDFullVersionBanner = "internal_version_full_version_banner"
)

// GetVersion returns the full version string. CONST-046-migrated:
// resolves through the wired Translator when set; falls back to the
// canonical Sprintf format when no translator is wired (NoopTranslator
// returns the message ID, which equals the const sentinel and triggers
// the local fallback).
func GetVersion() string {
	if GitCommit != "dev" && len(GitCommit) > 7 {
		shortCommit := GitCommit[:7]
		translated := tr(context.Background(), msgIDShortWithCommit, map[string]any{
			"Version":     Version,
			"ShortCommit": shortCommit,
		})
		if translated != msgIDShortWithCommit {
			return translated
		}
		// Translator not wired (NoopTranslator loud-echo) — render the
		// canonical en bundle format locally to preserve backward-
		// compatible output. This branch is the §11.4 anti-bluff
		// guard: silent translator failure MUST NOT produce an empty
		// or message-ID-shaped version string.
		return fmt.Sprintf("%s-%s", Version, shortCommit)
	}
	return Version
}

// GetFullVersion returns detailed version information. CONST-046-
// migrated: resolves through the wired Translator when set; falls back
// to the canonical Sprintf banner when no translator is wired.
func GetFullVersion() string {
	translated := tr(context.Background(), msgIDFullVersionBanner, map[string]any{
		"Version":   Version,
		"Commit":    GitCommit,
		"BuildDate": BuildDate,
		"GoVersion": GoVersion,
	})
	if translated != msgIDFullVersionBanner {
		return translated
	}
	// Translator not wired (NoopTranslator loud-echo) — render the
	// canonical en bundle format locally so existing callers
	// (CLI --version, /health endpoint) continue to receive the
	// historical "HelixCode <v> (commit: ..., built: ..., go: ...)"
	// shape rather than a raw message ID.
	return fmt.Sprintf("HelixCode %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, GoVersion)
}

// GetBuildInfo returns structured build information
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"commit":     GitCommit,
		"build_date": BuildDate,
		"go_version": GoVersion,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
}
