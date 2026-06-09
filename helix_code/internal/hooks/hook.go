package hooks

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// HookType represents the type of hook event
type HookType string

const (
	HookTypeBeforeTask  HookType = "before_task"  // Before task execution
	HookTypeAfterTask   HookType = "after_task"   // After task execution
	HookTypeBeforeLLM   HookType = "before_llm"   // Before LLM call
	HookTypeAfterLLM    HookType = "after_llm"    // After LLM call
	HookTypeBeforeEdit  HookType = "before_edit"  // Before file edit
	HookTypeAfterEdit   HookType = "after_edit"   // After file edit
	HookTypeBeforeBuild HookType = "before_build" // Before build
	HookTypeAfterBuild  HookType = "after_build"  // After build
	HookTypeBeforeTest  HookType = "before_test"  // Before test run
	HookTypeAfterTest   HookType = "after_test"   // After test run
	HookTypeOnError     HookType = "on_error"     // On error occurrence
	HookTypeOnSuccess   HookType = "on_success"   // On success
	HookTypeCustom      HookType = "custom"       // Custom hook

	// P1-F05 additions: claude-code-style lifecycle events.
	HookTypeBeforeToolCall HookType = "before_tool_call" // Before tool call
	HookTypeAfterToolCall  HookType = "after_tool_call"  // After tool call
	HookTypeBeforeBash     HookType = "before_bash"      // Before bash execution
	HookTypeAfterBash      HookType = "after_bash"       // After bash execution
	HookTypeOnCompaction   HookType = "on_compaction"    // On message compaction
	HookTypeOnPlanApproval HookType = "on_plan_approval" // On plan approval
	HookTypeOnPlanReject   HookType = "on_plan_reject"   // On plan rejection (F08)
)

// HookPriority determines execution order (higher = earlier)
type HookPriority int

const (
	PriorityLowest  HookPriority = 1
	PriorityLow     HookPriority = 25
	PriorityNormal  HookPriority = 50
	PriorityHigh    HookPriority = 75
	PriorityHighest HookPriority = 100
)

// HookStatus represents the execution status of a hook
type HookStatus string

const (
	StatusPending   HookStatus = "pending"   // Not yet executed
	StatusRunning   HookStatus = "running"   // Currently executing
	StatusCompleted HookStatus = "completed" // Successfully completed
	StatusSucceeded HookStatus = "succeeded" // Successfully completed (alias for Completed)
	StatusFailed    HookStatus = "failed"    // Failed with error
	StatusCanceled  HookStatus = "canceled"  // Canceled before completion
	StatusSkipped   HookStatus = "skipped"   // Skipped due to conditions
)

// HookFunc is the function signature for hook handlers
type HookFunc func(ctx context.Context, event *Event) error

// Hook represents a hook that can be executed in response to events
type Hook struct {
	ID          string            // Unique identifier
	Name        string            // Human-readable name
	Type        HookType          // Type of hook event
	Description string            // Hook description
	Handler     HookFunc          // Handler function
	Priority    HookPriority      // Execution priority
	Async       bool              // Execute asynchronously
	Timeout     time.Duration     // Execution timeout (0 = no timeout)
	Condition   func(*Event) bool // Optional condition to check before execution
	Tags        []string          // Tags for categorization
	Metadata    map[string]string // Custom metadata
	Enabled     bool              // Whether hook is enabled
	CreatedAt   time.Time         // When hook was created
}

// NewHook creates a new hook
func NewHook(name string, hookType HookType, handler HookFunc) *Hook {
	return &Hook{
		ID:        generateHookID(name, hookType),
		Name:      name,
		Type:      hookType,
		Handler:   handler,
		Priority:  PriorityNormal,
		Async:     false,
		Timeout:   0,
		Tags:      make([]string, 0),
		Metadata:  make(map[string]string),
		Enabled:   true,
		CreatedAt: time.Now(),
	}
}

// NewAsyncHook creates a new asynchronous hook
func NewAsyncHook(name string, hookType HookType, handler HookFunc) *Hook {
	hook := NewHook(name, hookType, handler)
	hook.Async = true
	return hook
}

// NewHookWithPriority creates a new hook with specified priority
func NewHookWithPriority(name string, hookType HookType, handler HookFunc, priority HookPriority) *Hook {
	hook := NewHook(name, hookType, handler)
	hook.Priority = priority
	return hook
}

// AddTag adds a tag to the hook
func (h *Hook) AddTag(tag string) {
	for _, t := range h.Tags {
		if t == tag {
			return // Already exists
		}
	}
	h.Tags = append(h.Tags, tag)
}

// HasTag checks if hook has a specific tag
func (h *Hook) HasTag(tag string) bool {
	for _, t := range h.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SetMetadata sets a metadata value
func (h *Hook) SetMetadata(key, value string) {
	h.Metadata[key] = value
}

// GetMetadata gets a metadata value
func (h *Hook) GetMetadata(key string) (string, bool) {
	value, ok := h.Metadata[key]
	return value, ok
}

// ShouldExecute checks if hook should execute for the given event
func (h *Hook) ShouldExecute(event *Event) bool {
	// Check if enabled
	if !h.Enabled {
		return false
	}

	// Check if types match
	if h.Type != event.Type {
		return false
	}

	// Check condition if present
	if h.Condition != nil {
		return h.Condition(event)
	}

	return true
}

// Execute executes the hook with the given event
func (h *Hook) Execute(ctx context.Context, event *Event) error {
	// Check if should execute
	if !h.ShouldExecute(event) {
		return nil
	}

	// Apply timeout if specified
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	// Execute handler
	return h.Handler(ctx, event)
}

// Clone creates a copy of the hook
func (h *Hook) Clone() *Hook {
	clone := &Hook{
		ID:          h.ID,
		Name:        h.Name,
		Type:        h.Type,
		Description: h.Description,
		Handler:     h.Handler,
		Priority:    h.Priority,
		Async:       h.Async,
		Timeout:     h.Timeout,
		Condition:   h.Condition,
		Tags:        make([]string, len(h.Tags)),
		Metadata:    make(map[string]string),
		Enabled:     h.Enabled,
		CreatedAt:   h.CreatedAt,
	}

	copy(clone.Tags, h.Tags)
	for k, v := range h.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}

// String returns a string representation of the hook
func (h *Hook) String() string {
	return fmt.Sprintf("%s (%s) - priority %d", h.Name, h.Type, h.Priority)
}

// Validate validates the hook.
//
// CONST-046 (round-160): all user-facing error literals resolved via
// tr(). Validate has no caller-supplied context — Background is the
// canonical fallback per rounds 146..159.
func (h *Hook) Validate() error {
	if h.ID == "" {
		return errors.New(tr(context.Background(), "internal_hooks_id_empty", nil))
	}

	if h.Name == "" {
		return errors.New(tr(context.Background(), "internal_hooks_name_empty", nil))
	}

	if h.Type == "" {
		return errors.New(tr(context.Background(), "internal_hooks_type_empty", nil))
	}

	if h.Handler == nil {
		return errors.New(tr(context.Background(), "internal_hooks_handler_nil", nil))
	}

	if h.Priority < PriorityLowest || h.Priority > PriorityHighest {
		return errors.New(tr(context.Background(), "internal_hooks_priority_out_of_range",
			map[string]any{
				"Priority": h.Priority,
				"Min":      PriorityLowest,
				"Max":      PriorityHighest,
			}))
	}

	return nil
}

// Event represents an event that triggers hooks
type Event struct {
	Type      HookType               // Event type
	Data      map[string]interface{} // Event data
	Context   context.Context        // Context for execution
	Timestamp time.Time              // When event occurred
	Source    string                 // Source of the event
	Metadata  map[string]string      // Additional metadata
}

// NewEvent creates a new event
func NewEvent(eventType HookType) *Event {
	return &Event{
		Type:      eventType,
		Data:      make(map[string]interface{}),
		Context:   context.Background(),
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// NewEventWithContext creates a new event with context
func NewEventWithContext(ctx context.Context, eventType HookType) *Event {
	event := NewEvent(eventType)
	event.Context = ctx
	return event
}

// SetData sets event data
func (e *Event) SetData(key string, value interface{}) {
	e.Data[key] = value
}

// GetData gets event data
func (e *Event) GetData(key string) (interface{}, bool) {
	value, ok := e.Data[key]
	return value, ok
}

// SetMetadata sets event metadata
func (e *Event) SetMetadata(key, value string) {
	e.Metadata[key] = value
}

// GetMetadata gets event metadata
func (e *Event) GetMetadata(key string) (string, bool) {
	value, ok := e.Metadata[key]
	return value, ok
}

// String returns a string representation of the event
func (e *Event) String() string {
	return fmt.Sprintf("Event %s at %s from %s", e.Type, e.Timestamp.Format(time.RFC3339), e.Source)
}

// ExecutionResult represents the result of hook execution
type ExecutionResult struct {
	HookID      string        // ID of executed hook
	HookName    string        // Name of executed hook
	Status      HookStatus    // Execution status
	Error       error         // Error if failed
	Duration    time.Duration // Execution duration
	StartedAt   time.Time     // When execution started
	CompletedAt time.Time     // When execution completed
}

// NewExecutionResult creates a new execution result
func NewExecutionResult(hook *Hook) *ExecutionResult {
	return &ExecutionResult{
		HookID:    hook.ID,
		HookName:  hook.Name,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}
}

// Complete marks the result as completed
func (r *ExecutionResult) Complete(err error) {
	r.CompletedAt = time.Now()
	r.Duration = r.CompletedAt.Sub(r.StartedAt)
	if r.Duration <= 0 {
		// HXC-045: a completed execution genuinely took a positive time; on fast
		// hardware the monotonic clock delta can round to 0. Floor to the smallest
		// representable unit so Duration is always a meaningful positive value.
		r.Duration = time.Nanosecond
	}

	if err != nil {
		r.Status = StatusFailed
		r.Error = err
	} else {
		r.Status = StatusCompleted
	}
}

// Cancel marks the result as canceled
func (r *ExecutionResult) Cancel() {
	r.CompletedAt = time.Now()
	r.Duration = r.CompletedAt.Sub(r.StartedAt)
	if r.Duration <= 0 {
		// HXC-045: see Complete — floor an instant cancel to a measurable positive duration.
		r.Duration = time.Nanosecond
	}
	r.Status = StatusCanceled
}

// Skip marks the result as skipped
func (r *ExecutionResult) Skip() {
	r.CompletedAt = time.Now()
	r.Duration = 0
	r.Status = StatusSkipped
}

// String returns a string representation of the result
func (r *ExecutionResult) String() string {
	return fmt.Sprintf("%s: %s (%.2fms)", r.HookName, r.Status, float64(r.Duration.Microseconds())/1000)
}

// generateHookID generates a unique ID for a hook
func generateHookID(name string, hookType HookType) string {
	return fmt.Sprintf("%s-%s-%d", string(hookType), sanitizeForID(name), time.Now().UnixNano())
}

// sanitizeForID sanitizes a string for use in an ID
func sanitizeForID(s string) string {
	result := ""
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			result += string(ch)
		} else {
			result += "-"
		}
	}
	return result
}
