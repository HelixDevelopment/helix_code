package permissions

import (
	"dev.helix.code/internal/tools/confirmation"
)

// Scope identifies where a rule lives.
type Scope int

const (
	ScopeCLI Scope = iota
	ScopeProject
	ScopeUser
	ScopePreset
)

// String returns the lowercase scope name.
func (s Scope) String() string {
	switch s {
	case ScopeCLI:
		return "cli"
	case ScopeProject:
		return "project"
	case ScopeUser:
		return "user"
	case ScopePreset:
		return "preset"
	default:
		return "unknown"
	}
}

// Rule is a single permission rule.
type Rule struct {
	Pattern     string              // e.g. "Bash(git status:*)"
	Action      confirmation.Action // ActionAllow / ActionAsk / ActionDeny
	Priority    int                 // higher wins; default 0
	Description string
	Source      Scope               // where this rule was loaded from
}

// RuleSet is the resolved set of rules for a session.
type RuleSet struct {
	Mode    string   // "default" | "auto" | "acceptEdits" | "dontAsk" | "bypassPermissions"
	Rules   []Rule
	Sources []string // file paths in load order
}

// ValidModes lists the five preset names accepted by --permission-mode and the YAML mode: key.
var ValidModes = []string{"default", "auto", "acceptEdits", "dontAsk", "bypassPermissions"}

// IsValidMode reports whether name is a recognised preset.
func IsValidMode(name string) bool {
	for _, m := range ValidModes {
		if m == name {
			return true
		}
	}
	return false
}
