package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/kilocode"
)

type KilocodeCommand struct {
	engine     *kilocode.RenameEngine
	analyzer   *kilocode.ImpactAnalyzer
	refactorer *kilocode.Refactorer
}

func NewKilocodeCommand(engine *kilocode.RenameEngine, analyzer *kilocode.ImpactAnalyzer, refactorer *kilocode.Refactorer) *KilocodeCommand {
	return &KilocodeCommand{engine: engine, analyzer: analyzer, refactorer: refactorer}
}

func (c *KilocodeCommand) Name() string      { return "kilocode" }
func (c *KilocodeCommand) Aliases() []string { return []string{"kc"} }
func (c *KilocodeCommand) Description() string {
	return tr(context.Background(), "internal_commands_kilocode_description", nil)
}
func (c *KilocodeCommand) Usage() string {
	return tr(context.Background(), "internal_commands_kilocode_usage", nil)
}

func (c *KilocodeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_kilocode_usage_full", nil)}, nil
	}

	switch args[0] {
	case "rename":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_kilocode_rename_usage", nil)}, nil
		}
		result, err := c.engine.Rename(ctx, args[1], args[2])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("rename: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_kilocode_renamed", map[string]any{
			"Old": result.Symbol.Name, "New": result.NewName, "Files": result.FilesModified, "Occurrences": result.Occurrences})}, nil

	case "impact":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_kilocode_impact_usage", nil)}, nil
		}
		result, err := c.analyzer.Analyze(args[1])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("impact: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_kilocode_impact_result", map[string]any{
			"Symbol": result.Symbol.Name, "Callers": len(result.Callers), "Callees": len(result.Callees),
			"AffectedFiles": len(result.AffectedFiles), "BlastRadius": result.BlastRadius, "Risk": fmt.Sprintf("%.1f", result.RiskScore)})}, nil

	case "edit":
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_kilocode_edit_hint", nil)}, nil

	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_kilocode_unknown_subcommand", map[string]any{"Subcommand": args[0]})}, nil
	}
}
