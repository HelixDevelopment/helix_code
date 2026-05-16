package plantree

import (
	"fmt"

	"dev.helix.code/internal/tools"
)

func RegisterPlanTools(reg *tools.ToolRegistry, store Store) error {
	if reg == nil {
		return fmt.Errorf("RegisterPlanTools: nil registry")
	}
	if store == nil {
		return fmt.Errorf("RegisterPlanTools: nil store")
	}

	items := []tools.Tool{
		NewPlanCreateTool(store),
		NewPlanBranchTool(store),
		NewPlanMergeTool(store),
		NewPlanListTool(store),
		NewPlanShowTool(store),
		NewPlanDeleteTool(store),
	}
	for _, it := range items {
		reg.Register(it)
	}
	return nil
}
