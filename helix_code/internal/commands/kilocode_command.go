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
	return "AST-aware refactoring: rename, impact analysis, multi-edit"
}
func (c *KilocodeCommand) Usage() string {
	return "/kilocode [rename|impact|edit]"
}

func (c *KilocodeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: "/kilocode rename <old> <new> | impact <symbol> | edit <extract|inline> <file>"}, nil
	}

	switch args[0] {
	case "rename":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: "usage: /kilocode rename <old_name> <new_name>"}, nil
		}
		result, err := c.engine.Rename(ctx, args[1], args[2])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("rename: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Renamed '%s' → '%s': %d files, %d occurrences",
			result.Symbol.Name, result.NewName, result.FilesModified, result.Occurrences)}, nil

	case "impact":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: "usage: /kilocode impact <symbol>"}, nil
		}
		result, err := c.analyzer.Analyze(args[1])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("impact: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Impact of '%s': %d callers, %d callees, %d affected files (blast radius: %d, risk: %.1f)",
			result.Symbol.Name, len(result.Callers), len(result.Callees), len(result.AffectedFiles), result.BlastRadius, result.RiskScore)}, nil

	case "edit":
		return &CommandResult{Success: true, Message: "Use the kilocode_extract or kilocode_inline tools for AST-aware refactoring."}, nil

	default:
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown: %s. Use rename, impact, or edit", args[0])}, nil
	}
}
