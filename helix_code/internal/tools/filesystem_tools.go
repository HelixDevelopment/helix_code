package tools

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/filesystem"
)

// FSReadTool implements file reading
type FSReadTool struct {
	registry *ToolRegistry
}

func (t *FSReadTool) Name() string { return "fs_read" }

// RequiresApproval — pure read of file contents (spec §3.6).
func (t *FSReadTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *FSReadTool) Description() string {
	return tr(context.Background(), "internal_tools_fs_read_description", nil)
}

func (t *FSReadTool) Category() ToolCategory {
	return CategoryFileSystem
}

func (t *FSReadTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
			"start_line": map[string]interface{}{
				"type":        "integer",
				"description": "Optional: Start line number for partial read",
			},
			"end_line": map[string]interface{}{
				"type":        "integer",
				"description": "Optional: End line number for partial read",
			},
		},
		Required:    []string{"path"},
		Description: "Read file contents from the filesystem",
	}
}

func (t *FSReadTool) Validate(params map[string]interface{}) error {
	if _, ok := params["path"]; !ok {
		return fmt.Errorf("path is required")
	}
	return nil
}

func (t *FSReadTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path := params["path"].(string)
	reader := t.registry.filesystem.Reader()

	var result *filesystem.FileContent
	var err error
	if startLine, ok := params["start_line"].(int); ok {
		endLine := params["end_line"].(int)
		result, err = reader.ReadLines(ctx, path, startLine, endLine)
	} else {
		result, err = reader.Read(ctx, path)
	}

	// Graceful directory handling: when fs_read is invoked on a directory the
	// reader returns an is_directory error. Instead of surfacing that NEGATIVE
	// error to the model, fall back to a readable, bounded directory listing
	// (a helpful "ls") so the model gets a positive, usable result.
	if err != nil && isDirectoryError(err) {
		return reader.ListDir(ctx, path)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// isDirectoryError reports whether err is the filesystem "is_directory" error.
func isDirectoryError(err error) bool {
	var fsErr *filesystem.FileSystemError
	return errors.As(err, &fsErr) && fsErr.Type == filesystem.ErrorIsDirectory
}

// FSWriteTool implements file writing
type FSWriteTool struct {
	approval.DefaultLevelEdit // §3.6 LevelEdit — mutates filesystem.
	registry                  *ToolRegistry
}

func (t *FSWriteTool) Name() string { return "fs_write" }

func (t *FSWriteTool) Description() string {
	return tr(context.Background(), "internal_tools_fs_write_description", nil)
}

func (t *FSWriteTool) Category() ToolCategory {
	return CategoryFileSystem
}

func (t *FSWriteTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
			"backup": map[string]interface{}{
				"type":        "boolean",
				"description": "Create backup before writing (default: false)",
			},
		},
		Required:    []string{"path", "content"},
		Description: "Write content to a file",
	}
}

func (t *FSWriteTool) Validate(params map[string]interface{}) error {
	if _, ok := params["path"]; !ok {
		return fmt.Errorf("path is required")
	}
	if _, ok := params["content"]; !ok {
		return fmt.Errorf("content is required")
	}
	return nil
}

func (t *FSWriteTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path := params["path"].(string)
	content := []byte(params["content"].(string))

	return nil, t.registry.filesystem.Writer().Write(ctx, path, content)
}

// FSEditTool implements file editing
type FSEditTool struct {
	approval.DefaultLevelEdit // §3.6 LevelEdit — mutates filesystem.
	registry                  *ToolRegistry
}

func (t *FSEditTool) Name() string { return "fs_edit" }

func (t *FSEditTool) Description() string {
	return tr(context.Background(), "internal_tools_fs_edit_description", nil)
}

func (t *FSEditTool) Category() ToolCategory {
	return CategoryFileSystem
}

func (t *FSEditTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "String to replace (must be unique in file)",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "Replacement string",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all occurrences (default: false)",
			},
		},
		Required:    []string{"path", "old_string", "new_string"},
		Description: "Edit file contents by replacing strings",
	}
}

func (t *FSEditTool) Validate(params map[string]interface{}) error {
	if _, ok := params["path"]; !ok {
		return fmt.Errorf("path is required")
	}
	if _, ok := params["old_string"]; !ok {
		return fmt.Errorf("old_string is required")
	}
	if _, ok := params["new_string"]; !ok {
		return fmt.Errorf("new_string is required")
	}
	return nil
}

func (t *FSEditTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path := params["path"].(string)
	oldStr := params["old_string"].(string)
	newStr := params["new_string"].(string)
	replaceAll := false

	if val, ok := params["replace_all"].(bool); ok {
		replaceAll = val
	}

	return t.registry.filesystem.Editor().Replace(ctx, path, oldStr, newStr, replaceAll)
}

// GlobTool implements glob pattern matching
type GlobTool struct {
	registry *ToolRegistry
}

func (t *GlobTool) Name() string { return "glob" }

// RequiresApproval — pure read of filesystem entries (spec §3.6).
func (t *GlobTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *GlobTool) Description() string {
	return tr(context.Background(), "internal_tools_glob_description", nil)
}

func (t *GlobTool) Category() ToolCategory {
	return CategoryFileSystem
}

func (t *GlobTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to match (e.g., '**/*.go')",
			},
			"root": map[string]interface{}{
				"type":        "string",
				"description": "Root directory to search from (default: workspace root)",
			},
		},
		Required:    []string{"pattern"},
		Description: "Find files matching a glob pattern",
	}
}

func (t *GlobTool) Validate(params map[string]interface{}) error {
	if _, ok := params["pattern"]; !ok {
		return fmt.Errorf("pattern is required")
	}
	return nil
}

func (t *GlobTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pattern := params["pattern"].(string)
	return t.registry.filesystem.Searcher().Glob(ctx, pattern)
}

// GrepTool implements content search
type GrepTool struct {
	registry *ToolRegistry
}

func (t *GrepTool) Name() string { return "grep" }

// RequiresApproval — pure read; matches text in files (spec §3.6).
func (t *GrepTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *GrepTool) Description() string {
	return tr(context.Background(), "internal_tools_grep_description", nil)
}

func (t *GrepTool) Category() ToolCategory {
	return CategoryFileSystem
}

func (t *GrepTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Pattern to search for",
			},
			"root": map[string]interface{}{
				"type":        "string",
				"description": "Root directory to search from (default: workspace root)",
			},
			"regex": map[string]interface{}{
				"type":        "boolean",
				"description": "Use regex pattern matching (default: false)",
			},
			"case_sensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "Case sensitive search (default: true)",
			},
			"max_matches": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of matches to return",
			},
		},
		Required:    []string{"pattern"},
		Description: "Search file contents for a pattern",
	}
}

func (t *GrepTool) Validate(params map[string]interface{}) error {
	if _, ok := params["pattern"]; !ok {
		return fmt.Errorf("pattern is required")
	}
	return nil
}

func (t *GrepTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	opts := filesystem.ContentSearchOptions{
		Pattern:       params["pattern"].(string),
		CaseSensitive: true,
	}

	if root, ok := params["root"].(string); ok {
		opts.Root = root
	}

	if regex, ok := params["regex"].(bool); ok {
		opts.IsRegex = regex
	}

	if caseSens, ok := params["case_sensitive"].(bool); ok {
		opts.CaseSensitive = caseSens
	}

	if maxMatches, ok := params["max_matches"].(int); ok {
		opts.MaxMatches = maxMatches
	}

	return t.registry.filesystem.Searcher().SearchContent(ctx, opts)
}
