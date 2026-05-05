package commands

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Skill is an agent-invoked Markdown skill loaded from .helix/skills/*.md.
type Skill struct {
	name              string
	description       string
	body              string
	variables         map[string]string
	triggerPatterns   []string         // raw strings (preserved for /skills show)
	triggers          []*regexp.Regexp // compiled (bad regex skipped at parse time)
	requiresIsolation bool
	sourcePath        string
}

// skillFrontmatter is the YAML shape of a skill's frontmatter block.
type skillFrontmatter struct {
	Description       string            `yaml:"description"`
	Triggers          []string          `yaml:"triggers"`
	Variables         map[string]string `yaml:"variables"`
	RequiresIsolation bool              `yaml:"requires_isolation"`
}

// Name returns the skill name (derived from filename, without extension).
func (s *Skill) Name() string { return s.name }

// Description returns the description parsed from frontmatter.
func (s *Skill) Description() string { return s.description }

// SourcePath returns the .md file path this skill was loaded from.
func (s *Skill) SourcePath() string { return s.sourcePath }

// RequiresIsolation reports whether the skill requests sandbox isolation.
func (s *Skill) RequiresIsolation() bool { return s.requiresIsolation }

// Body returns the raw (unrendered) body string of the skill.
func (s *Skill) Body() string { return s.body }

// TriggerPatterns returns a copy of the raw trigger regex strings from frontmatter.
func (s *Skill) TriggerPatterns() []string {
	out := make([]string, len(s.triggerPatterns))
	copy(out, s.triggerPatterns)
	return out
}

// parseSkillFile parses raw .md content into a Skill. Frontmatter is required
// for skills (it carries the trigger patterns). Bad regex patterns are
// silently dropped from the compiled triggers list; the raw patterns are
// preserved in triggerPatterns for diagnostics.
func parseSkillFile(name, raw, sourcePath string) (*Skill, error) {
	s := &Skill{
		name:       name,
		sourcePath: sourcePath,
		variables:  map[string]string{},
	}

	body := raw
	if strings.HasPrefix(raw, "---\n") {
		// Locate closing ---
		rest := raw[4:]
		end := strings.Index(rest, "\n---")
		if end == -1 {
			return nil, fmt.Errorf("skill %s: unterminated frontmatter", name)
		}
		fmBlock := rest[:end]
		body = strings.TrimPrefix(rest[end+4:], "\n")

		var meta skillFrontmatter
		if err := yaml.Unmarshal([]byte(fmBlock), &meta); err != nil {
			return nil, fmt.Errorf("skill %s: parse frontmatter: %w", name, err)
		}
		s.description = meta.Description
		s.triggerPatterns = append([]string(nil), meta.Triggers...)
		s.requiresIsolation = meta.RequiresIsolation
		if meta.Variables != nil {
			s.variables = meta.Variables
		}
		// Compile trigger patterns; silently skip any that are invalid regex.
		for _, p := range meta.Triggers {
			re, err := regexp.Compile(p)
			if err != nil {
				continue
			}
			s.triggers = append(s.triggers, re)
		}
	}

	s.body = strings.TrimSpace(body)
	return s, nil
}

// Render fills the body with positional args and the skill's default variables.
// args maps to {{ARG1}}, {{ARG2}}, … (1-based). selection and currentFile map
// to {{SELECTION}} and {{CURRENT_FILE}}.
func (s *Skill) Render(args []string, selection, currentFile string) (string, error) {
	cc := &CommandContext{Args: args, Selection: selection, CurrentFile: currentFile}
	return (&MarkdownCommand{name: s.name, body: s.body, variables: s.variables}).render(cc)
}

// RenderWithCaptures merges the captures map into the skill's default variables
// and renders the body. Captures take precedence over skill defaults on
// key collision. args, selection, and currentFile are forwarded to the
// substitution engine as usual.
func (s *Skill) RenderWithCaptures(args []string, captures map[string]string, selection, currentFile string) (string, error) {
	merged := make(map[string]string, len(s.variables)+len(captures))
	for k, v := range s.variables {
		merged[k] = v
	}
	for k, v := range captures {
		merged[k] = v
	}
	cc := &CommandContext{Args: args, Selection: selection, CurrentFile: currentFile}
	return (&MarkdownCommand{name: s.name, body: s.body, variables: merged}).render(cc)
}

// SkillRegistry stores skills by name and matches user input against trigger
// patterns. All operations are safe for concurrent use.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewSkillRegistry constructs an empty SkillRegistry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{skills: map[string]*Skill{}}
}

// Add registers a skill, replacing any existing skill with the same name.
func (r *SkillRegistry) Add(s *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name()] = s
}

// Remove unregisters the skill with the given name (no-op if absent).
func (r *SkillRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// Get returns the skill registered under name, or (nil, false) if absent.
func (r *SkillRegistry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns all registered skills in lexicographic name order.
func (r *SkillRegistry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*Skill, 0, len(names))
	for _, n := range names {
		out = append(out, r.skills[n])
	}
	return out
}

// FindMatching iterates skills in lexicographic name order and returns the
// first skill whose compiled trigger patterns match input. The second return
// value is a map of named capture groups from the matching regex. Returns
// (nil, nil, false) when no skill matches.
func (r *SkillRegistry) FindMatching(input string) (*Skill, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		s := r.skills[n]
		for _, re := range s.triggers {
			m := re.FindStringSubmatch(input)
			if m == nil {
				continue
			}
			caps := map[string]string{}
			for i, capName := range re.SubexpNames() {
				if capName == "" || i >= len(m) {
					continue
				}
				caps[capName] = m[i]
			}
			return s, caps, true
		}
	}
	return nil, nil, false
}
