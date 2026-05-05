package builtin

import (
	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/tools/worktree"
)

// RegisterBuiltinCommands registers all built-in commands with the registry
func RegisterBuiltinCommands(registry *commands.Registry) {
	// Task management
	registry.Register(NewNewTaskCommand())

	// Context management
	registry.Register(NewCondenseCommand())

	// Code quality and guidelines
	registry.Register(NewNewRuleCommand())

	// Issue tracking
	registry.Register(NewReportBugCommand())

	// Workflow management
	registry.Register(NewWorkflowsCommand())

	// Planning and architecture
	registry.Register(NewDeepPlanningCommand())

	// Permission management
	registry.Register(commands.NewPermissionsCommand())
}

// RegisterBuiltinCommandsWithWorktree extends RegisterBuiltinCommands with
// the /worktree command, which requires a worktree.Manager dependency.
// Callers that have a Manager (cmd/cli/main.go startup) use this; callers
// without one (legacy paths) use the original RegisterBuiltinCommands.
func RegisterBuiltinCommandsWithWorktree(registry *commands.Registry, m *worktree.Manager) error {
	RegisterBuiltinCommands(registry)
	return registry.Register(commands.NewWorktreeCommand(m))
}

// GetBuiltinCommandNames returns names of all built-in commands
func GetBuiltinCommandNames() []string {
	return []string{
		"newtask",
		"condense",
		"newrule",
		"reportbug",
		"workflows",
		"deepplanning",
		"permissions",
		"worktree",
	}
}

// GetBuiltinCommandAliases returns all aliases for built-in commands
func GetBuiltinCommandAliases() map[string]string {
	return map[string]string{
		// newtask aliases
		"nt":   "newtask",
		"task": "newtask",

		// condense aliases
		"smol":      "condense",
		"compact":   "condense",
		"summarize": "condense",

		// newrule aliases
		"rule":      "newrule",
		"guideline": "newrule",

		// reportbug aliases
		"bug":   "reportbug",
		"issue": "reportbug",

		// workflows aliases
		"wf":   "workflows",
		"flow": "workflows",

		// deepplanning aliases
		"deepplan":  "deepplanning",
		"dp":        "deepplanning",
		"architect": "deepplanning",

		// permissions aliases
		"perms": "permissions",

		// worktree aliases
		"wt": "worktree",
	}
}
