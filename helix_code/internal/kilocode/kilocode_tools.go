package kilocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type KiloRenameTool struct {
	engine *RenameEngine
	approval.DefaultLevelEdit
}

func NewKiloRenameTool(engine *RenameEngine) *KiloRenameTool {
	return &KiloRenameTool{engine: engine}
}

func (t *KiloRenameTool) Name() string { return "kilocode_rename" }
func (t *KiloRenameTool) Description() string {
	return tr(context.Background(), "internal_kilocode_rename_tool_description", nil)
}
func (t *KiloRenameTool) Category() tools.ToolCategory {
	return tools.ToolCategory("kilocode")
}

func (t *KiloRenameTool) Schema() tools.ToolSchema {
	ctx := context.Background()
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"old_name": map[string]interface{}{"type": "string", "description": tr(ctx, "internal_kilocode_rename_old_name_description", nil)},
			"new_name": map[string]interface{}{"type": "string", "description": "New symbol name"},
		},
		Required: []string{"old_name", "new_name"},
	}
}

func (t *KiloRenameTool) Validate(params map[string]interface{}) error {
	if _, ok := params["old_name"].(string); !ok {
		return errors.New("old_name is required")
	}
	if _, ok := params["new_name"].(string); !ok {
		return errors.New("new_name is required")
	}
	return nil
}

func (t *KiloRenameTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	oldName := params["old_name"].(string)
	newName := params["new_name"].(string)

	result, err := t.engine.Rename(ctx, oldName, newName)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(result)
	var out map[string]interface{}
	json.Unmarshal(data, &out)
	return out, nil
}

type KiloImpactTool struct {
	analyzer *ImpactAnalyzer
	approval.DefaultLevelEdit
}

func NewKiloImpactTool(analyzer *ImpactAnalyzer) *KiloImpactTool {
	return &KiloImpactTool{analyzer: analyzer}
}

func (t *KiloImpactTool) Name() string { return "kilocode_impact" }
func (t *KiloImpactTool) Description() string {
	return tr(context.Background(), "internal_kilocode_impact_tool_description", nil)
}
func (t *KiloImpactTool) Category() tools.ToolCategory {
	return tools.ToolCategory("kilocode")
}
func (t *KiloImpactTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *KiloImpactTool) Schema() tools.ToolSchema {
	ctx := context.Background()
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"symbol": map[string]interface{}{"type": "string", "description": tr(ctx, "internal_kilocode_impact_symbol_description", nil)},
		},
		Required: []string{"symbol"},
	}
}

func (t *KiloImpactTool) Validate(params map[string]interface{}) error {
	if _, ok := params["symbol"].(string); !ok {
		return errors.New("symbol is required")
	}
	return nil
}

func (t *KiloImpactTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol := params["symbol"].(string)

	result, err := t.analyzer.Analyze(symbol)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(result)
	var out map[string]interface{}
	json.Unmarshal(data, &out)
	return out, nil
}

type KiloMultiEditTool struct {
	refactorer *Refactorer
	approval.DefaultLevelEdit
}

func NewKiloMultiEditTool(refactorer *Refactorer) *KiloMultiEditTool {
	return &KiloMultiEditTool{refactorer: refactorer}
}

func (t *KiloMultiEditTool) Name() string { return "kilocode_multi_edit" }
func (t *KiloMultiEditTool) Description() string {
	return tr(context.Background(), "internal_kilocode_multi_edit_tool_description", nil)
}
func (t *KiloMultiEditTool) Category() tools.ToolCategory {
	return tools.ToolCategory("kilocode")
}

func (t *KiloMultiEditTool) Schema() tools.ToolSchema {
	ctx := context.Background()
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"action":     map[string]interface{}{"type": "string", "description": "extract or inline"},
			"file":       map[string]interface{}{"type": "string", "description": tr(ctx, "internal_kilocode_multi_edit_file_description", nil)},
			"func_name":  map[string]interface{}{"type": "string", "description": "Function name"},
			"start_line": map[string]interface{}{"type": "number", "description": tr(ctx, "internal_kilocode_multi_edit_start_line_description", nil)},
			"end_line":   map[string]interface{}{"type": "number", "description": tr(ctx, "internal_kilocode_multi_edit_end_line_description", nil)},
		},
		Required: []string{"action", "file"},
	}
}

func (t *KiloMultiEditTool) Validate(params map[string]interface{}) error {
	if _, ok := params["action"].(string); !ok {
		return errors.New("action is required")
	}
	return nil
}

func (t *KiloMultiEditTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action := params["action"].(string)
	file := params["file"].(string)

	switch action {
	case "extract":
		funcName, _ := params["func_name"].(string)
		startLine, _ := params["start_line"].(float64)
		endLine, _ := params["end_line"].(float64)
		if err := t.refactorer.ExtractMethod(file, funcName, int(startLine), int(endLine)); err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		return map[string]interface{}{"status": "extracted", "file": file, "new_func": funcName}, nil

	case "inline":
		funcName, _ := params["func_name"].(string)
		if err := t.refactorer.InlineCall(file, funcName); err != nil {
			return nil, fmt.Errorf("inline: %w", err)
		}
		return map[string]interface{}{"status": "inlined", "file": file, "func": funcName}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s (use extract or inline)", action)
	}
}
