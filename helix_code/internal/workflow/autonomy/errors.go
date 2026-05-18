package autonomy

import (
	"errors"
	"fmt"
)

var (
	// Mode errors
	ErrInvalidMode      = errors.New("invalid autonomy mode")
	ErrModeSwitchDenied = errors.New("mode switch not allowed")
	ErrModeNotPersisted = errors.New("failed to persist mode")

	// Permission errors
	ErrPermissionDenied   = errors.New("permission denied")
	ErrConfirmationFailed = errors.New("user confirmation failed")
	ErrGuardrailViolation = errors.New("guardrail violation")

	// Execution errors
	ErrActionFailed    = errors.New("action execution failed")
	ErrRetryExhausted  = errors.New("retry attempts exhausted")
	ErrUnsafeOperation = errors.New("operation deemed unsafe")

	// ErrActionHandlerNotRegistered is returned by ActionExecutor.executeAction when
	// no handler has been registered for the action's Type. Before round-31
	// (§11.4 CRITICAL anti-bluff sweep, 2026-05-18) executeAction fabricated
	// Success=true with a placeholder "Executed: <description>" output regardless
	// of action type, certifying every action as PASS independent of reality.
	// The autonomy workflow now requires callers to register a real handler via
	// ActionExecutor.RegisterHandler(actionType, handler) before executing
	// actions of that type; absent a handler this sentinel surfaces so the
	// caller knows the action genuinely did not run.
	ErrActionHandlerNotRegistered = errors.New("autonomy executor: no handler registered for action type — executeAction previously fabricated Success=true with a placeholder 'Executed: ...' string regardless of action type (§11.4 CRITICAL: autonomy workflow certified every action as PASS, masking real failures); register a handler via executor.RegisterHandler(actionType, handler) before executing actions of that type")

	// Escalation errors
	ErrEscalationDenied  = errors.New("escalation request denied")
	ErrEscalationExpired = errors.New("escalation has expired")
	ErrAlreadyEscalated  = errors.New("already at requested level")
)

// AutonomyError provides detailed error information
type AutonomyError struct {
	Op      string       // Operation that failed
	Mode    AutonomyMode // Current mode
	Action  *Action      // Related action
	Err     error        // Underlying error
	Reason  string       // Human-readable reason
	Fixable bool         // Whether error can be fixed
}

func (e *AutonomyError) Error() string {
	if e.Action != nil {
		return fmt.Sprintf("%s (mode: %s, action: %s): %v - %s",
			e.Op, e.Mode, e.Action.Type, e.Err, e.Reason)
	}
	return fmt.Sprintf("%s (mode: %s): %v - %s",
		e.Op, e.Mode, e.Err, e.Reason)
}

func (e *AutonomyError) Unwrap() error {
	return e.Err
}

// NewAutonomyError creates a new autonomy error
func NewAutonomyError(op string, mode AutonomyMode, err error, reason string) *AutonomyError {
	return &AutonomyError{
		Op:      op,
		Mode:    mode,
		Err:     err,
		Reason:  reason,
		Fixable: false,
	}
}

// WithAction adds action context to the error
func (e *AutonomyError) WithAction(action *Action) *AutonomyError {
	e.Action = action
	return e
}

// WithFixable marks the error as fixable
func (e *AutonomyError) WithFixable(fixable bool) *AutonomyError {
	e.Fixable = fixable
	return e
}
