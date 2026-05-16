package permissions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"dev.helix.code/internal/tools/confirmation"
)

const expectedAPIVersion = "helixcode.permissions/v1"

// fileSchema is the on-disk YAML structure for a permissions file.
type fileSchema struct {
	APIVersion string       `yaml:"apiVersion"`
	Mode       string       `yaml:"mode"`
	Rules      []ruleSchema `yaml:"rules"`
}

type ruleSchema struct {
	Pattern     string `yaml:"pattern"`
	Action      string `yaml:"action"`
	Priority    int    `yaml:"priority"`
	Description string `yaml:"description"`
}

// FileLoader reads layered permission files.
type FileLoader struct {
	UserPath    string
	ProjectPath string
	Mode        string // overrides the file's mode: key when non-empty (e.g. CLI flag)
}

// Load merges user + project files with the selected mode preset.
// Returns an error for malformed YAML, unknown apiVersion, or unknown preset.
// A missing file is treated as empty (not an error).
func (l *FileLoader) Load(ctx context.Context) (*RuleSet, error) {
	userFile, err := readFileIfExists(l.UserPath)
	if err != nil {
		return nil, fmt.Errorf("reading user file %s: %w", l.UserPath, err)
	}
	projectFile, err := readFileIfExists(l.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("reading project file %s: %w", l.ProjectPath, err)
	}

	var sources []string
	if userFile != nil {
		if err := validateAPIVersion(userFile.APIVersion); err != nil {
			return nil, fmt.Errorf("%s: %w", l.UserPath, err)
		}
		sources = append(sources, l.UserPath)
	}
	if projectFile != nil {
		if err := validateAPIVersion(projectFile.APIVersion); err != nil {
			return nil, fmt.Errorf("%s: %w", l.ProjectPath, err)
		}
		sources = append(sources, l.ProjectPath)
	}

	mode := l.Mode
	if mode == "" && projectFile != nil && projectFile.Mode != "" {
		mode = projectFile.Mode
	}
	if mode == "" && userFile != nil && userFile.Mode != "" {
		mode = userFile.Mode
	}
	if mode == "" {
		mode = "default"
	}
	if !IsValidMode(mode) {
		return nil, fmt.Errorf("unknown permission mode %q (valid: %s)", mode, strings.Join(ValidModes, ", "))
	}

	merged, err := mergeRules(projectFile, userFile)
	if err != nil {
		return nil, err
	}
	merged = append(merged, PresetRules(mode)...)

	return &RuleSet{Mode: mode, Rules: merged, Sources: sources}, nil
}

// Save writes a single rule to the user or project file, creating it if
// missing. Directory mode 0700, file mode 0600.
func (l *FileLoader) Save(ctx context.Context, scope Scope, rule Rule) error {
	var path string
	switch scope {
	case ScopeUser:
		path = l.UserPath
	case ScopeProject:
		path = l.ProjectPath
	default:
		return fmt.Errorf("scope %s is not persistable (use user or project)", scope)
	}
	if path == "" {
		return fmt.Errorf("scope %s has no configured path", scope)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating dir for %s: %w", path, err)
	}
	existing, err := readFileIfExists(path)
	if err != nil {
		return err
	}
	if existing == nil {
		existing = &fileSchema{APIVersion: expectedAPIVersion}
	}
	replaced := false
	for i, r := range existing.Rules {
		if r.Pattern == rule.Pattern {
			existing.Rules[i] = ruleToSchema(rule)
			replaced = true
			break
		}
	}
	if !replaced {
		existing.Rules = append(existing.Rules, ruleToSchema(rule))
	}
	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshalling %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Remove deletes the rule with the given pattern from the chosen scope's file.
// No-op (no error) if the rule does not exist.
func (l *FileLoader) Remove(ctx context.Context, scope Scope, pattern string) error {
	var path string
	switch scope {
	case ScopeUser:
		path = l.UserPath
	case ScopeProject:
		path = l.ProjectPath
	default:
		return fmt.Errorf("scope %s is not persistable", scope)
	}
	existing, err := readFileIfExists(path)
	if err != nil || existing == nil {
		return err
	}
	out := existing.Rules[:0]
	for _, r := range existing.Rules {
		if r.Pattern != pattern {
			out = append(out, r)
		}
	}
	existing.Rules = out
	body, err := yaml.Marshal(existing)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func readFileIfExists(path string) (*fileSchema, error) {
	if path == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f fileSchema
	if err := yaml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return &f, nil
}

func validateAPIVersion(v string) error {
	if v == "" {
		return fmt.Errorf("missing apiVersion (expected %q)", expectedAPIVersion)
	}
	if v != expectedAPIVersion {
		return fmt.Errorf("unsupported apiVersion %q (expected %q)", v, expectedAPIVersion)
	}
	return nil
}

func mergeRules(project, user *fileSchema) ([]Rule, error) {
	var merged []Rule
	projectPatterns := map[string]bool{}
	if project != nil {
		for _, r := range project.Rules {
			converted, err := schemaToRule(r, ScopeProject)
			if err != nil {
				return nil, err
			}
			merged = append(merged, converted)
			projectPatterns[r.Pattern] = true
		}
	}
	if user != nil {
		for _, r := range user.Rules {
			if projectPatterns[r.Pattern] {
				continue
			}
			converted, err := schemaToRule(r, ScopeUser)
			if err != nil {
				return nil, err
			}
			merged = append(merged, converted)
		}
	}
	return merged, nil
}

func schemaToRule(s ruleSchema, source Scope) (Rule, error) {
	action, err := parseAction(s.Action)
	if err != nil {
		return Rule{}, fmt.Errorf("rule %q: %w", s.Pattern, err)
	}
	if _, err := ParsePattern(s.Pattern); err != nil {
		return Rule{}, err
	}
	return Rule{
		Pattern:     s.Pattern,
		Action:      action,
		Priority:    s.Priority,
		Description: s.Description,
		Source:      source,
	}, nil
}

func ruleToSchema(r Rule) ruleSchema {
	return ruleSchema{
		Pattern:     r.Pattern,
		Action:      actionString(r.Action),
		Priority:    r.Priority,
		Description: r.Description,
	}
}

func parseAction(s string) (confirmation.Action, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "allow":
		return confirmation.ActionAllow, nil
	case "ask":
		return confirmation.ActionAsk, nil
	case "deny":
		return confirmation.ActionDeny, nil
	}
	return 0, fmt.Errorf("invalid action %q (expected allow|ask|deny)", s)
}

func actionString(a confirmation.Action) string {
	switch a {
	case confirmation.ActionAllow:
		return "allow"
	case confirmation.ActionAsk:
		return "ask"
	case confirmation.ActionDeny:
		return "deny"
	}
	return "ask"
}
