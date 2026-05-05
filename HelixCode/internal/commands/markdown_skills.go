package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
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

// SkillLoader scans project + user skill directories and registers each
// .md file as a Skill in the supplied SkillRegistry. Project files override
// user files of the same name on collision. Non-existent directories are
// silently skipped; per-file parse errors are logged at WARN and do not
// cause Load/Reload to fail.
type SkillLoader struct {
	registry   *SkillRegistry
	projectDir string
	userDir    string
	mu         sync.Mutex
	loaded     map[string]string // skill name → source path
	log        *zap.Logger
}

// NewSkillLoader constructs a loader. Either dir may be empty or
// nonexistent; the loader gracefully skips missing dirs.
func NewSkillLoader(reg *SkillRegistry, projectDir, userDir string) *SkillLoader {
	return &SkillLoader{
		registry:   reg,
		projectDir: projectDir,
		userDir:    userDir,
		loaded:     map[string]string{},
		log:        zap.NewNop(),
	}
}

// SetLogger installs a non-noop logger.
func (l *SkillLoader) SetLogger(log *zap.Logger) { l.log = log }

// Load is a synonym for Reload. Provided for clarity at startup.
func (l *SkillLoader) Load() error { return l.Reload() }

// Reload re-scans both directories and reconciles the registry. Added files
// are registered, removed files are unregistered, changed files are replaced.
// Per-file errors (parse, read) are logged at WARN and the file is skipped;
// they do NOT cause Reload to fail.
func (l *SkillLoader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	want := map[string]*Skill{}
	// Order: user first, then project (project overrides user on collision).
	for _, dir := range []string{l.userDir, l.projectDir} {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			l.log.Warn("skill loader: read dir", zap.String("dir", dir), zap.Error(err))
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".md")
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				l.log.Warn("skill loader: read file", zap.String("path", path), zap.Error(err))
				continue
			}
			s, err := parseSkillFile(name, string(data), path)
			if err != nil {
				l.log.Warn("skill loader: parse error", zap.String("path", path), zap.Error(err))
				continue
			}
			want[name] = s
		}
	}

	// Remove names that disappeared (only those previously loaded by us).
	for name := range l.loaded {
		if _, ok := want[name]; !ok {
			l.registry.Remove(name)
			delete(l.loaded, name)
		}
	}
	// Add or replace.
	for name, s := range want {
		l.registry.Add(s)
		l.loaded[name] = s.SourcePath()
	}
	return nil
}

// Loaded returns a snapshot of skill name → source path for all skills
// currently managed by this loader.
func (l *SkillLoader) Loaded() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]string, len(l.loaded))
	for k, v := range l.loaded {
		out[k] = v
	}
	return out
}
