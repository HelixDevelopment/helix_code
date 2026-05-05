package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// MarkdownCommand is a user-defined slash command parsed from a Markdown file
// with optional YAML frontmatter (title, description, variables) followed by
// a body that supports {{TOKEN}} substitution.
type MarkdownCommand struct {
	name        string
	title       string
	description string
	body        string
	variables   map[string]string
	sourcePath  string
}

// Name returns the command name (without /).
func (c *MarkdownCommand) Name() string { return c.name }

// Aliases returns no aliases for Markdown commands; names are taken from filename.
func (c *MarkdownCommand) Aliases() []string { return nil }

// Description returns the description parsed from frontmatter (or empty string).
func (c *MarkdownCommand) Description() string { return c.description }

// Usage returns a simple usage line.
func (c *MarkdownCommand) Usage() string { return "/" + c.name + " [args]" }

// SourcePath returns the .md file path this command was loaded from.
func (c *MarkdownCommand) SourcePath() string { return c.sourcePath }

// Body returns the raw (unrendered) body string of the command.
func (c *MarkdownCommand) Body() string { return c.body }

// Execute renders the command body with token substitution and returns it as Output.
func (c *MarkdownCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	if cc == nil {
		cc = &CommandContext{}
	}
	out, err := c.render(cc)
	if err != nil {
		return nil, err
	}
	return &CommandResult{Success: true, Output: out}, nil
}

// frontmatterMeta is the YAML shape of a Markdown command's frontmatter block.
type frontmatterMeta struct {
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
}

// parseMarkdownCommand parses raw .md content into a MarkdownCommand.
// Frontmatter is optional; when present it must be valid YAML between "---" lines
// at the start of the file.  Body text is trimmed of leading/trailing whitespace.
func parseMarkdownCommand(name, raw, sourcePath string) (*MarkdownCommand, error) {
	cmd := &MarkdownCommand{
		name:       name,
		sourcePath: sourcePath,
		variables:  make(map[string]string),
	}

	body := raw
	if strings.HasPrefix(raw, "---\n") {
		// Look for closing ---
		rest := raw[4:]
		end := strings.Index(rest, "\n---")
		if end == -1 {
			return nil, fmt.Errorf("markdown command %s: unterminated frontmatter", name)
		}
		fmBlock := rest[:end]
		// Skip past closing --- and optional trailing newline
		body = rest[end+4:]
		body = strings.TrimPrefix(body, "\n")

		var meta frontmatterMeta
		if err := yaml.Unmarshal([]byte(fmBlock), &meta); err != nil {
			return nil, fmt.Errorf("markdown command %s: parse frontmatter: %w", name, err)
		}
		cmd.title = meta.Title
		cmd.description = meta.Description
		if meta.Variables != nil {
			cmd.variables = meta.Variables
		}
	}

	cmd.body = strings.TrimSpace(body)
	return cmd, nil
}

// substRegex matches {{TOKEN}} patterns.
// Token forms supported:
//   - ARG1, ARG2, … — positional arguments (1-based)
//   - ARG.name      — named variable from frontmatter
//   - SELECTION     — current text selection from CommandContext
//   - CURRENT_FILE  — current file path from CommandContext
//   - CWD           — current working directory
//   - ENV.NAME      — environment variable
//   - FILE:/path    — contents of a file
//
// Unknown tokens are left verbatim ({{TOKEN}}).
var substRegex = regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*(?:\.[A-Za-z0-9_]+)?(?::[^}]+)?)\}\}`)

// render performs {{TOKEN}} substitution on the command body using the supplied context.
func (c *MarkdownCommand) render(cc *CommandContext) (string, error) {
	resolver := c.buildResolver(cc)
	out := substRegex.ReplaceAllStringFunc(c.body, func(match string) string {
		token := match[2 : len(match)-2] // strip {{ and }}
		return resolver(token)
	})
	return out, nil
}

// buildResolver returns a closure that maps a token string to its substituted value.
func (c *MarkdownCommand) buildResolver(cc *CommandContext) func(string) string {
	cwd, _ := os.Getwd()

	return func(token string) string {
		switch {
		// Positional: ARG1, ARG2, …
		case strings.HasPrefix(token, "ARG") && len(token) > 3 && token[3] != '.':
			numStr := token[3:]
			n, err := strconv.Atoi(numStr)
			if err != nil || n < 1 {
				return "{{" + token + "}}" // unrecognised — leave verbatim
			}
			if n-1 < len(cc.Args) {
				return cc.Args[n-1]
			}
			return ""

		// Named variable: ARG.name
		case strings.HasPrefix(token, "ARG."):
			varName := strings.TrimPrefix(token, "ARG.")
			return c.variables[varName]

		case token == "SELECTION":
			return cc.Selection

		case token == "CURRENT_FILE":
			return cc.CurrentFile

		case token == "CWD":
			return cwd

		case strings.HasPrefix(token, "ENV."):
			return os.Getenv(strings.TrimPrefix(token, "ENV."))

		case strings.HasPrefix(token, "FILE:"):
			path := strings.TrimPrefix(token, "FILE:")
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Sprintf("[FILE NOT FOUND: %s]", path)
			}
			const maxSize = 1 << 20 // 1 MiB
			if info.Size() > maxSize {
				return fmt.Sprintf("[FILE TOO LARGE: %s]", path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Sprintf("[FILE READ ERROR: %s: %v]", path, err)
			}
			return string(data)

		default:
			// Unknown token — leave verbatim so the rendered output is inspectable.
			return "{{" + token + "}}"
		}
	}
}

// MarkdownLoader scans project + user command directories and registers each
// .md file as a MarkdownCommand in the supplied Registry. Project files
// override user files of the same name on collision.
type MarkdownLoader struct {
	registry   *Registry
	projectDir string
	userDir    string
	mu         sync.Mutex
	loaded     map[string]string // command name → source path
	log        *zap.Logger
}

// NewMarkdownLoader constructs a loader. Either dir may be empty or
// nonexistent; the loader gracefully skips missing dirs.
func NewMarkdownLoader(registry *Registry, projectDir, userDir string) *MarkdownLoader {
	return &MarkdownLoader{
		registry:   registry,
		projectDir: projectDir,
		userDir:    userDir,
		loaded:     map[string]string{},
		log:        zap.NewNop(),
	}
}

// SetLogger installs a non-noop logger.
func (l *MarkdownLoader) SetLogger(log *zap.Logger) { l.log = log }

// Load is a synonym for Reload. Provided for clarity at startup.
func (l *MarkdownLoader) Load() error { return l.Reload() }

// Reload re-scans both directories and reconciles the registry. Added files
// are registered, removed files are unregistered, changed files are replaced.
// Per-file errors (parse, read) are logged at WARN and the file is skipped;
// they do NOT cause Reload to fail.
func (l *MarkdownLoader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	want := map[string]*MarkdownCommand{}
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
			l.log.Warn("markdown loader: read dir", zap.String("dir", dir), zap.Error(err))
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
				l.log.Warn("markdown loader: read file", zap.String("path", path), zap.Error(err))
				continue
			}
			cmd, err := parseMarkdownCommand(name, string(data), path)
			if err != nil {
				l.log.Warn("markdown loader: parse error", zap.String("path", path), zap.Error(err))
				continue
			}
			want[name] = cmd
		}
	}

	// Remove names that disappeared (only those previously loaded by us).
	for name := range l.loaded {
		if _, ok := want[name]; !ok {
			l.registry.Unregister(name)
			delete(l.loaded, name)
		}
	}
	// Add or replace.
	for name, cmd := range want {
		if existing, ok := l.registry.Get(name); ok {
			if _, isMd := existing.(*MarkdownCommand); !isMd {
				l.log.Warn("markdown loader: name conflicts with built-in",
					zap.String("name", name), zap.String("source", cmd.sourcePath))
				continue
			}
			l.registry.Unregister(name)
		}
		if err := l.registry.Register(cmd); err != nil {
			l.log.Warn("markdown loader: register", zap.String("name", name), zap.Error(err))
			continue
		}
		l.loaded[name] = cmd.sourcePath
	}
	return nil
}

// Loaded returns a snapshot of name → source path.
func (l *MarkdownLoader) Loaded() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]string, len(l.loaded))
	for k, v := range l.loaded {
		out[k] = v
	}
	return out
}
