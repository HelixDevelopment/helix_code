package continua

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type ContinueEditTool struct {
	editor *WorkspaceEditor
	approval.DefaultLevelEdit
}

func NewContinueEditTool(e *WorkspaceEditor) *ContinueEditTool { return &ContinueEditTool{editor: e} }
func (t *ContinueEditTool) Name() string             { return "continue_edit" }
func (t *ContinueEditTool) Description() string      { return "Open, edit, or save a file in the workspace" }
func (t *ContinueEditTool) Category() tools.ToolCategory { return tools.ToolCategory("continue") }

func (t *ContinueEditTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"action": map[string]interface{}{"type": "string"},
			"file":   map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
		},
		Required: []string{"action", "file"},
	}
}

func (t *ContinueEditTool) Validate(p map[string]interface{}) error {
	if _, ok := p["action"].(string); !ok { return errors.New("action required") }
	return nil
}

func (t *ContinueEditTool) Execute(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	action := p["action"].(string)
	file := p["file"].(string)

	switch action {
	case "open":
		result, err := t.editor.Open(ctx, file)
		if err != nil { return nil, err }
		data, _ := json.Marshal(result)
		var out map[string]interface{}
		json.Unmarshal(data, &out)
		return out, nil
	case "edit":
		content, _ := p["content"].(string)
		result, err := t.editor.Edit(ctx, file, content)
		if err != nil { return nil, err }
		data, _ := json.Marshal(result)
		var out map[string]interface{}
		json.Unmarshal(data, &out)
		return out, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

type ContinueCompleteTool struct {
	completion *CompletionEngine
	approval.DefaultLevelEdit
}

func NewContinueCompleteTool(c *CompletionEngine) *ContinueCompleteTool {
	return &ContinueCompleteTool{completion: c}
}
func (t *ContinueCompleteTool) Name() string             { return "continue_complete" }
func (t *ContinueCompleteTool) Description() string      { return "Get inline code completion at cursor" }
func (t *ContinueCompleteTool) Category() tools.ToolCategory { return tools.ToolCategory("continue") }

func (t *ContinueCompleteTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"file":   map[string]interface{}{"type": "string"},
			"line":   map[string]interface{}{"type": "number"},
			"column": map[string]interface{}{"type": "number"},
		},
		Required: []string{"file", "line", "column"},
	}
}

func (t *ContinueCompleteTool) Validate(p map[string]interface{}) error {
	if _, ok := p["file"].(string); !ok { return errors.New("file required") }
	return nil
}

func (t *ContinueCompleteTool) Execute(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	file := p["file"].(string)
	line := int(p["line"].(float64))
	col := int(p["column"].(float64))

	result, err := t.completion.Complete(ctx, file, line, col)
	if err != nil { return nil, err }

	data, _ := json.Marshal(result)
	var out map[string]interface{}
	json.Unmarshal(data, &out)
	return out, nil
}
