// Package builtin provides the standard set of built-in slash commands.
//
// Slash commands provide quick access to common operations during interactive
// sessions. Users invoke commands with a leading slash (e.g., /newtask) and
// can use short aliases for convenience.
//
// # Built-in Commands
//
// The package provides the following commands:
//
// /newtask (aliases: /nt, /task)
// Creates a new task in the task management system.
//
// /condense (aliases: /smol, /compact, /summarize)
// Compresses the current conversation context to reduce token usage while
// preserving important information.
//
// /newrule (aliases: /rule, /guideline)
// Creates a new coding rule or guideline that applies to the current project.
//
// /reportbug (aliases: /bug, /issue)
// Reports a bug or issue for tracking.
//
// /workflows (aliases: /wf, /flow)
// Lists and manages available workflows.
//
// /deepplanning (aliases: /deepplan, /dp, /architect)
// Initiates deep planning mode for complex architectural decisions.
//
// # Registration
//
// All built-in commands are registered with the command registry at startup:
//
//	registry := commands.NewRegistry()
//	builtin.RegisterBuiltinCommands(registry)
//
// # Creating Custom Commands
//
// Commands implement the commands.Command interface from the parent package.
// See individual command files for implementation examples.
package builtin
