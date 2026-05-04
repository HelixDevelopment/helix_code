package permissions

import (
	"strings"

	"dev.helix.code/internal/tools/confirmation"
)

// readOnlyCommands lists Bash command prefixes that are auto-allowed under dontAsk.
// Conservative — anything not listed asks.
var readOnlyCommands = []string{
	"ls", "cat", "find", "grep", "rg", "head", "tail", "wc", "ps", "top", "df", "du",
	"env", "uname", "which", "whoami", "date", "lsb_release", "echo", "pwd", "stat",
	"git status", "git log", "git diff", "git branch", "git show", "git remote",
	"git config --get", "git rev-parse",
	"go version", "node --version", "npm --version", "python --version", "python3 --version",
	"rustc --version", "cargo --version", "java -version", "ruby --version",
	"docker --version", "kubectl version --client",
}

// writeCommands lists Bash command prefixes that are auto-allowed under acceptEdits
// (because acceptEdits is for edit/write operations including via shell).
var writeCommands = []string{
	"git add", "git commit", "git push", "git pull", "git merge", "git rebase",
	"git checkout", "git reset", "git stash", "git tag",
	"rm", "mv", "cp", "mkdir", "touch", "chmod", "chown", "ln", "rmdir",
	"tar", "zip", "unzip", "wget", "curl",
	"npm install", "npm run", "go get", "go mod", "go build", "go install",
	"cargo build", "cargo install", "make", "cmake", "docker", "kubectl", "helm",
}

// IsReadOnlyCommand reports whether cmd starts with one of the documented
// read-only prefixes (case-insensitive on the first token).
func IsReadOnlyCommand(cmd string) bool {
	return startsWithAny(strings.TrimSpace(cmd), readOnlyCommands)
}

// IsWriteCommand reports whether cmd starts with one of the documented
// write prefixes.
func IsWriteCommand(cmd string) bool {
	return startsWithAny(strings.TrimSpace(cmd), writeCommands)
}

func startsWithAny(cmd string, prefixes []string) bool {
	lower := strings.ToLower(cmd)
	for _, p := range prefixes {
		pl := strings.ToLower(p)
		if lower == pl || strings.HasPrefix(lower, pl+" ") || strings.HasPrefix(lower, pl+"\t") {
			return true
		}
	}
	return false
}

// PresetRules returns the built-in rule slice for a named preset.
// Returns nil for unknown names.
func PresetRules(mode string) []Rule {
	switch mode {
	case "default":
		return nil
	case "auto":
		return []Rule{
			{Pattern: "*(*)", Action: confirmation.ActionAllow, Priority: 0,
				Description: "auto-allow everything (auto preset)", Source: ScopePreset},
		}
	case "acceptEdits":
		rules := []Rule{
			{Pattern: "Edit(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow Edit (acceptEdits preset)", Source: ScopePreset},
			{Pattern: "Write(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow Write (acceptEdits preset)", Source: ScopePreset},
			{Pattern: "MultiEdit(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow MultiEdit (acceptEdits preset)", Source: ScopePreset},
		}
		for _, p := range writeCommands {
			rules = append(rules, Rule{
				Pattern:     "Bash(" + p + "*)",
				Action:      confirmation.ActionAllow,
				Priority:    50,
				Description: "auto-allow write Bash (" + p + "*) (acceptEdits preset)",
				Source:      ScopePreset,
			})
		}
		return rules
	case "dontAsk":
		rules := []Rule{
			{Pattern: "Read(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow Read (dontAsk preset)", Source: ScopePreset},
			{Pattern: "Glob(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow Glob (dontAsk preset)", Source: ScopePreset},
			{Pattern: "Grep(*)", Action: confirmation.ActionAllow, Priority: 100,
				Description: "auto-allow Grep (dontAsk preset)", Source: ScopePreset},
		}
		for _, p := range readOnlyCommands {
			rules = append(rules, Rule{
				Pattern:     "Bash(" + p + "*)",
				Action:      confirmation.ActionAllow,
				Priority:    50,
				Description: "auto-allow read-only Bash (" + p + "*) (dontAsk preset)",
				Source:      ScopePreset,
			})
		}
		return rules
	case "bypassPermissions":
		return []Rule{
			{Pattern: "*(*)", Action: confirmation.ActionAllow, Priority: 1_000_000,
				Description: "BYPASS — auto-allow everything (operator-only safety hatch)",
				Source:      ScopePreset},
		}
	}
	return nil
}
