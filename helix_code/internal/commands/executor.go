package commands

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Executor executes commands
type Executor struct {
	registry *Registry
	parser   *Parser
}

// NewExecutor creates a new command executor
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry: registry,
		parser:   NewParser(),
	}
}

// Execute parses and executes a command
func (e *Executor) Execute(ctx context.Context, input string, cmdCtx *CommandContext) (*CommandResult, error) {
	// Parse the command
	commandName, args, flags, isCommand := e.parser.Parse(input)
	if !isCommand {
		return nil, &CommandError{
			Command: input,
			Message: "not a valid command",
		}
	}

	// Get the command
	cmd, exists := e.registry.Get(commandName)
	if !exists {
		return nil, &CommandError{
			Command: commandName,
			Message: "command not found",
		}
	}

	// Prepare command context
	if cmdCtx == nil {
		cmdCtx = &CommandContext{}
	}
	cmdCtx.Args = args
	cmdCtx.Flags = flags
	cmdCtx.RawInput = input

	// Execute the command
	start := time.Now()
	log.Printf("Executing command: /%s with args: %v", commandName, args)

	// Isolate the command: a slash command may be supplied by user/markdown
	// definitions or third-party registrations, so a panic inside its Execute
	// MUST NOT propagate up and crash the host process (CLI / server) along with
	// every unrelated goroutine. Recover it here and surface it as a controlled
	// CommandError (graceful degradation), keeping the dispatcher usable for the
	// next command. CONST-046: the message is resolved through the package
	// translator, never a hardcoded literal.
	result, err := e.executeRecovered(ctx, cmd, commandName, cmdCtx)

	duration := time.Since(start)
	log.Printf("Command /%s completed in %v (success: %t)", commandName, duration, err == nil && result != nil && result.Success)

	if err != nil {
		return nil, &CommandError{
			Command: commandName,
			Message: "execution failed",
			Err:     err,
		}
	}

	return result, nil
}

// executeRecovered invokes cmd.Execute with a panic guard. If the command
// panics, the panic is recovered and converted into a non-nil error so the
// dispatcher never crashes the host process. A recovered panic is logged with
// its value (operator visibility) and returned as a CONST-046-resolved error.
func (e *Executor) executeRecovered(ctx context.Context, cmd Command, commandName string, cmdCtx *CommandContext) (result *CommandResult, err error) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Command /%s panicked and was recovered: %v", commandName, p)
			result = nil
			err = fmt.Errorf("%s: %v", tr(ctx, "internal_commands_command_panicked", nil), p)
		}
	}()
	return cmd.Execute(ctx, cmdCtx)
}

// ExecuteWithDefault executes a command with default context
func (e *Executor) ExecuteWithDefault(ctx context.Context, input string) (*CommandResult, error) {
	return e.Execute(ctx, input, &CommandContext{})
}

// CanExecute checks if input is a valid command
func (e *Executor) CanExecute(input string) bool {
	commandName, _, _, isCommand := e.parser.Parse(input)
	if !isCommand {
		return false
	}

	_, exists := e.registry.Get(commandName)
	return exists
}

// GetHelp returns help text for a command
func (e *Executor) GetHelp(commandName string) string {
	if commandName == "" {
		return e.registry.GetAllHelp()
	}
	return e.registry.GetHelp(commandName)
}

// ListCommands returns all available commands
func (e *Executor) ListCommands() []Command {
	return e.registry.List()
}

// Autocomplete provides command name autocompletion
func (e *Executor) Autocomplete(partial string) []string {
	if !e.parser.IsCommand(partial) {
		return nil
	}

	partial = partial[1:]  // Remove leading /
	partial = partial + "" // ensure string

	matches := make([]string, 0)
	names := e.registry.ListNames()

	for _, name := range names {
		if partial == "" || len(partial) < len(name) && name[:len(partial)] == partial {
			matches = append(matches, "/"+name)
		}
	}

	return matches
}

// ValidateContext validates that the command context has required fields.
//
// CONST-046 (round-149): the four "<field> is required" error strings
// are resolved through the package-level translator. We thread
// context.Background() because ValidateContext does not currently
// accept a context — future signature change should pass the caller's
// context for locale-aware resolution.
func (e *Executor) ValidateContext(cmdCtx *CommandContext, required []string) error {
	ctx := context.Background()
	for _, field := range required {
		switch field {
		case "user_id":
			if cmdCtx.UserID == "" {
				return fmt.Errorf("%s", tr(ctx, "internal_commands_user_id_is_required", nil))
			}
		case "session_id":
			if cmdCtx.SessionID == "" {
				return fmt.Errorf("%s", tr(ctx, "internal_commands_session_id_is_required", nil))
			}
		case "project_id":
			if cmdCtx.ProjectID == "" {
				return fmt.Errorf("%s", tr(ctx, "internal_commands_project_id_is_required", nil))
			}
		case "working_dir":
			if cmdCtx.WorkingDir == "" {
				return fmt.Errorf("%s", tr(ctx, "internal_commands_working_dir_is_required", nil))
			}
		}
	}
	return nil
}
