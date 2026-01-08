// Package commands provides a slash command system for HelixCode, enabling
// users to invoke special actions through a command-line style interface.
//
// # Overview
//
// The commands package implements a complete slash command framework with
// command registration, parsing, execution, and autocompletion. Commands
// are prefixed with "/" and can accept arguments and flags.
//
// # Architecture
//
// The package is organized around several core components:
//
//   - Command interface: Contract for all command implementations
//   - Registry: Central storage for registered commands and aliases
//   - Parser: Parses user input into command name, arguments, and flags
//   - Executor: Orchestrates command parsing and execution
//
// # Command Interface
//
// All commands implement the Command interface:
//
//	type Command interface {
//	    Name() string                                           // Command name (without /)
//	    Aliases() []string                                      // Alternative names
//	    Description() string                                    // Short description
//	    Usage() string                                          // Usage information
//	    Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error)
//	}
//
// # Basic Usage
//
// Setting up the command system:
//
//	// Create registry and executor
//	registry := commands.NewRegistry()
//	executor := commands.NewExecutor(registry)
//
//	// Register commands
//	registry.Register(myCommand)
//
//	// Execute a command
//	result, err := executor.Execute(ctx, "/mycommand arg1 --flag=value", cmdCtx)
//
// # Creating Commands
//
// Implementing a custom command:
//
//	type MyCommand struct{}
//
//	func (c *MyCommand) Name() string        { return "mycommand" }
//	func (c *MyCommand) Aliases() []string   { return []string{"mc"} }
//	func (c *MyCommand) Description() string { return "Does something useful" }
//	func (c *MyCommand) Usage() string       { return "/mycommand <arg> [--flag=value]" }
//
//	func (c *MyCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
//	    // Access arguments and flags
//	    arg := cmdCtx.Args[0]
//	    flag := cmdCtx.Flags["flag"]
//
//	    return &CommandResult{
//	        Success: true,
//	        Message: "Command executed successfully",
//	    }, nil
//	}
//
// # Command Context
//
// CommandContext provides execution context:
//
//	type CommandContext struct {
//	    UserID      string                 // Current user ID
//	    SessionID   string                 // Current session ID
//	    ProjectID   string                 // Current project ID
//	    Args        []string               // Parsed arguments
//	    Flags       map[string]string      // Parsed flags
//	    RawInput    string                 // Original input
//	    ChatHistory []ChatMessage          // Conversation history
//	    WorkingDir  string                 // Current working directory
//	    Metadata    map[string]interface{} // Additional metadata
//	}
//
// # Command Result
//
// Commands return CommandResult:
//
//	type CommandResult struct {
//	    Success     bool                   // Execution success status
//	    Message     string                 // Result message
//	    Data        interface{}            // Optional result data
//	    Actions     []Action               // Follow-up actions
//	    ShouldReply bool                   // Whether to send a reply
//	    Metadata    map[string]interface{} // Additional metadata
//	}
//
// # Parsing Commands
//
// The parser handles command syntax:
//
//	// Simple command
//	/help
//
//	// Command with arguments
//	/newtask "Implement authentication"
//
//	// Command with flags
//	/build --target=linux --verbose
//
//	// Combined
//	/deploy production --force --dry-run
//
// Flag syntax supports:
//
//	--flag=value    // Explicit value
//	--flag value    // Space-separated value
//	--flag          // Boolean flag (value = "true")
//
// Quoted strings are preserved:
//
//	/task "Multi word description" --priority=high
//
// # Using the Parser Directly
//
// For advanced parsing needs:
//
//	parser := commands.NewParser()
//
//	name, args, flags, isCommand := parser.Parse("/deploy prod --force")
//	// name: "deploy"
//	// args: ["prod"]
//	// flags: {"force": "true"}
//	// isCommand: true
//
// # Registry Operations
//
// Managing registered commands:
//
//	// Register a command
//	err := registry.Register(cmd)
//
//	// Unregister a command
//	registry.Unregister("mycommand")
//
//	// Get a command by name or alias
//	cmd, exists := registry.Get("mycommand")
//	cmd, exists := registry.Get("mc") // alias
//
//	// List all commands
//	cmds := registry.List()
//
//	// Get command names
//	names := registry.ListNames()
//
//	// Get help text
//	help := registry.GetHelp("mycommand")
//	allHelp := registry.GetAllHelp()
//
// # Autocompletion
//
// The executor provides command autocompletion:
//
//	matches := executor.Autocomplete("/dep")
//	// Returns: ["/deploy", "/dependencies", ...]
//
// # Built-in Commands
//
// The builtin subpackage provides standard commands:
//
//   - /newtask: Create a new task
//   - /newrule: Create a new rule
//   - /deepplanning: Enable deep planning mode
//   - /condense: Condense conversation context
//   - /reportbug: Report a bug
//   - /workflows: Manage workflows
//
// Register built-in commands:
//
//	builtin.Register(registry)
//
// # Context Validation
//
// Validate required context fields:
//
//	err := executor.ValidateContext(cmdCtx, []string{"user_id", "project_id"})
//	// Returns error if required fields are missing
//
// # Actions
//
// Commands can return actions for follow-up processing:
//
//	result := &CommandResult{
//	    Success: true,
//	    Actions: []Action{
//	        {
//	            Type: "create_task",
//	            Data: map[string]interface{}{"name": "New Task"},
//	        },
//	        {
//	            Type: "switch_mode",
//	            Data: map[string]interface{}{"mode": "planning"},
//	        },
//	    },
//	}
//
// # Error Handling
//
// Command errors include context:
//
//	type CommandError struct {
//	    Command string // Command name
//	    Message string // Error message
//	    Err     error  // Underlying error
//	}
//
// # Thread Safety
//
// The Registry is safe for concurrent use. Commands should be registered
// before starting to process user input to avoid race conditions.
//
// # Subpackages
//
// The commands package contains:
//
//   - builtin: Built-in command implementations
package commands
