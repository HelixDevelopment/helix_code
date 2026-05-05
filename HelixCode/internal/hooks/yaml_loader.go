package hooks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// expectedAPIVersion is the only YAML schema version this loader accepts.
const expectedAPIVersion = "helixcode.hooks/v1"

// fileSchema is the on-disk YAML structure for hooks files.
type fileSchema struct {
	APIVersion string       `yaml:"apiVersion"`
	Hooks      []hookSchema `yaml:"hooks"`
}

type hookSchema struct {
	ID          string `yaml:"id"`
	Event       string `yaml:"event"`
	Script      string `yaml:"script"`
	Priority    int    `yaml:"priority"`
	Async       bool   `yaml:"async"`
	Timeout     string `yaml:"timeout"`
	Enabled     *bool  `yaml:"enabled"` // pointer so absent ≠ false
	Description string `yaml:"description"`
}

// FileLoader reads hooks from layered YAML files.
//
// Project file entries override user file entries with the same id.
// Disabled hooks are filtered out before return.
type FileLoader struct {
	UserPath    string
	ProjectPath string
}

// Load reads both files and returns the aggregated, enabled hooks plus
// the source paths in load order. Missing files are not errors.
func (l *FileLoader) Load(ctx context.Context) ([]*Hook, []string, error) {
	user, err := readFileIfExists(l.UserPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading user file %s: %w", l.UserPath, err)
	}
	project, err := readFileIfExists(l.ProjectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading project file %s: %w", l.ProjectPath, err)
	}

	var sources []string
	if user != nil {
		if err := validateAPIVersion(user.APIVersion); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", l.UserPath, err)
		}
		sources = append(sources, l.UserPath)
	}
	if project != nil {
		if err := validateAPIVersion(project.APIVersion); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", l.ProjectPath, err)
		}
		sources = append(sources, l.ProjectPath)
	}

	merged := mergeHooks(project, user)
	return filterEnabledAndConvert(merged), sources, nil
}

func readFileIfExists(path string) (*fileSchema, error) {
	if path == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

// mergeHooks merges project + user files. Identical IDs: project wins.
func mergeHooks(project, user *fileSchema) []hookSchema {
	var merged []hookSchema
	projectIDs := map[string]bool{}
	if project != nil {
		for _, h := range project.Hooks {
			merged = append(merged, h)
			projectIDs[h.ID] = true
		}
	}
	if user != nil {
		for _, h := range user.Hooks {
			if projectIDs[h.ID] {
				continue
			}
			merged = append(merged, h)
		}
	}
	return merged
}

// filterEnabledAndConvert turns hookSchema into *Hook entries, dropping
// disabled hooks and entries whose event type is unknown (logged, skipped).
func filterEnabledAndConvert(schemas []hookSchema) []*Hook {
	out := make([]*Hook, 0, len(schemas))
	for _, s := range schemas {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if !enabled {
			continue
		}
		evt, ok := parseEventType(s.Event)
		if !ok {
			log.Printf("WARN hooks loader: unknown event type %q in hook %q — skipping", s.Event, s.ID)
			continue
		}
		var timeout time.Duration
		if strings.TrimSpace(s.Timeout) != "" {
			d, err := time.ParseDuration(s.Timeout)
			if err != nil {
				log.Printf("WARN hooks loader: invalid timeout %q in hook %q — using 0", s.Timeout, s.ID)
			} else {
				timeout = d
			}
		}
		priority := HookPriority(s.Priority)
		if priority == 0 {
			priority = PriorityNormal
		}
		hook := &Hook{
			ID:          s.ID,
			Name:        s.ID, // use id as display name; users can override later
			Type:        evt,
			Description: s.Description,
			Priority:    priority,
			Async:       s.Async,
			Timeout:     timeout,
			Enabled:     true,
			CreatedAt:   time.Now(),
			Metadata:    map[string]string{"script": s.Script},
		}
		out = append(out, hook)
	}
	return out
}

// parseEventType resolves a YAML event-type string to a HookType, returning
// false if unknown. Covers all 19 known constants (13 pre-existing + 6 F05).
func parseEventType(s string) (HookType, bool) {
	switch HookType(s) {
	case HookTypeBeforeTask, HookTypeAfterTask,
		HookTypeBeforeLLM, HookTypeAfterLLM,
		HookTypeBeforeEdit, HookTypeAfterEdit,
		HookTypeBeforeBuild, HookTypeAfterBuild,
		HookTypeBeforeTest, HookTypeAfterTest,
		HookTypeOnError, HookTypeOnSuccess,
		HookTypeCustom,
		HookTypeBeforeToolCall, HookTypeAfterToolCall,
		HookTypeBeforeBash, HookTypeAfterBash,
		HookTypeOnCompaction, HookTypeOnPlanApproval:
		return HookType(s), true
	}
	return "", false
}
