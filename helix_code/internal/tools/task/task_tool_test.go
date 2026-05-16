package task

// Tests for TaskTool (P1-F15-T07).
//
// Strategy: TaskTool depends on the TaskExecutor seam (a subset of
// *subagent.SubagentManager). We exercise the tool against a fakeTaskExecutor
// implementing that seam — this is a hexagonal port, NOT a mock substituting
// for the real Dispatch / WaitAll behaviour of SubagentManager. The real
// SubagentManager is covered by manager_test.go with real spawners.
//
// Tests verify:
//   - Tool interface contract (Name / Description / Category / Schema /
//     Validate / Execute) — matches dev.helix.code/internal/tools.Tool.
//   - Args → SubagentTask mapping (description / prompt / isolation /
//     subagent_type / timeout_seconds / base_branch / merge_on_success).
//   - Validate rejects missing required args, bad types, and out-of-range
//     timeouts.
//   - Dispatch + WaitAll error propagation surfaces a clear human message.
//   - SubagentResult is propagated byte-for-byte from the executor.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/tools"
)

// ---------- fake task executor (hexagonal seam) ----------

// fakeTaskExecutor records Dispatch + WaitAll calls and returns
// caller-controlled (id, err) / (results, err). Safe for concurrent use.
type fakeTaskExecutor struct {
	mu sync.Mutex

	// last Dispatch invocation
	dispatchedTask subagent.SubagentTask
	dispatchCalls  int
	dispatchID     string
	dispatchErr    error

	// last WaitAll invocation
	waitAllIDs    []string
	waitAllCalls  int
	waitAllResult []subagent.SubagentResult
	waitAllErr    error
}

func (f *fakeTaskExecutor) Dispatch(ctx context.Context, task subagent.SubagentTask) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dispatchCalls++
	f.dispatchedTask = task
	return f.dispatchID, f.dispatchErr
}

func (f *fakeTaskExecutor) WaitAll(ctx context.Context, taskIDs []string) ([]subagent.SubagentResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waitAllCalls++
	cp := make([]string, len(taskIDs))
	copy(cp, taskIDs)
	f.waitAllIDs = cp
	return f.waitAllResult, f.waitAllErr
}

func (f *fakeTaskExecutor) snapshot() (subagent.SubagentTask, int, []string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idsCopy := make([]string, len(f.waitAllIDs))
	copy(idsCopy, f.waitAllIDs)
	return f.dispatchedTask, f.dispatchCalls, idsCopy, f.waitAllCalls
}

// newFakeOK returns a fake that successfully dispatches + returns one result.
func newFakeOK() *fakeTaskExecutor {
	return &fakeTaskExecutor{
		dispatchID: "task-id-123",
		waitAllResult: []subagent.SubagentResult{
			{
				TaskID:    "task-id-123",
				State:     subagent.StateSucceeded,
				Output:    "subagent-output",
				Duration:  100 * time.Millisecond,
				Isolation: subagent.IsolationNone,
			},
		},
	}
}

// validArgs returns a base map with all required fields present and valid.
func validArgs() map[string]interface{} {
	return map[string]interface{}{
		"description": "summarise repo",
		"prompt":      "Read README.md and summarise.",
	}
}

// ---------- shape: Name / Description / Category / Schema ----------

func TestTaskTool_Name(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	if got, want := tool.Name(), "task"; got != want {
		t.Errorf("Name(): got %q want %q", got, want)
	}
}

func TestTaskTool_DescriptionMentionsSubagent(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	desc := tool.Description()
	if strings.TrimSpace(desc) == "" {
		t.Fatalf("Description(): empty")
	}
	if !strings.Contains(strings.ToLower(desc), "subagent") {
		t.Errorf("Description(): expected to mention 'subagent', got %q", desc)
	}
}

func TestTaskTool_Category(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	if got, want := tool.Category(), tools.CategorySubagent; got != want {
		t.Errorf("Category(): got %q want %q", got, want)
	}
}

func TestTaskTool_Schema_HasRequiredFields(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	schema := tool.Schema()

	if schema.Type != "object" {
		t.Errorf("Schema().Type: got %q want %q", schema.Type, "object")
	}
	for _, key := range []string{"description", "prompt", "isolation"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Schema(): missing property %q", key)
		}
	}

	// description + prompt must be Required.
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}
	for _, want := range []string{"description", "prompt"} {
		if !requiredSet[want] {
			t.Errorf("Schema().Required: missing %q (got %v)", want, schema.Required)
		}
	}
}

// ---------- Validate ----------

func TestTaskTool_Validate_RequiresDescription(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	args := map[string]interface{}{
		"prompt": "do something",
	}
	if err := tool.Validate(args); err == nil {
		t.Errorf("Validate(): expected error for missing description, got nil")
	}
}

func TestTaskTool_Validate_RequiresPrompt(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	args := map[string]interface{}{
		"description": "summarise",
	}
	if err := tool.Validate(args); err == nil {
		t.Errorf("Validate(): expected error for missing prompt, got nil")
	}
}

func TestTaskTool_Validate_RejectsEmptyDescription(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	args := map[string]interface{}{
		"description": "",
		"prompt":      "do something",
	}
	if err := tool.Validate(args); err == nil {
		t.Errorf("Validate(): expected error for empty description, got nil")
	}
}

func TestTaskTool_Validate_RejectsBadIsolation(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	args := validArgs()
	args["isolation"] = "wormhole"
	if err := tool.Validate(args); err == nil {
		t.Errorf("Validate(): expected error for bad isolation, got nil")
	}
}

func TestTaskTool_Validate_AcceptsValidIsolations(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	for _, iso := range []string{"none", "worktree"} {
		args := validArgs()
		args["isolation"] = iso
		if err := tool.Validate(args); err != nil {
			t.Errorf("Validate(): isolation=%q unexpected error: %v", iso, err)
		}
	}
}

func TestTaskTool_Validate_RejectsTimeoutOutOfRange(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	for _, bad := range []int{0, -1, 1801, 9999} {
		args := validArgs()
		args["timeout_seconds"] = bad
		if err := tool.Validate(args); err == nil {
			t.Errorf("Validate(): expected error for timeout_seconds=%d, got nil", bad)
		}
	}
}

func TestTaskTool_Validate_RejectsBadTypes(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "description not string",
			args: map[string]interface{}{"description": 42, "prompt": "p"},
		},
		{
			name: "prompt not string",
			args: map[string]interface{}{"description": "d", "prompt": 42},
		},
		{
			name: "isolation not string",
			args: map[string]interface{}{"description": "d", "prompt": "p", "isolation": 1},
		},
		{
			name: "subagent_type not string",
			args: map[string]interface{}{"description": "d", "prompt": "p", "subagent_type": 1},
		},
		{
			name: "timeout_seconds not int",
			args: map[string]interface{}{"description": "d", "prompt": "p", "timeout_seconds": "abc"},
		},
		{
			name: "base_branch not string",
			args: map[string]interface{}{"description": "d", "prompt": "p", "base_branch": 1},
		},
		{
			name: "merge_on_success not bool",
			args: map[string]interface{}{"description": "d", "prompt": "p", "merge_on_success": "yes"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tool.Validate(tc.args); err == nil {
				t.Errorf("Validate(): expected error, got nil")
			}
		})
	}
}

func TestTaskTool_Validate_AcceptsAllValidFields(t *testing.T) {
	tool := NewTaskTool(newFakeOK())
	args := map[string]interface{}{
		"description":      "summarise repo",
		"prompt":           "Read README.md and summarise it",
		"isolation":        "worktree",
		"subagent_type":    "code-reviewer",
		"timeout_seconds":  120,
		"base_branch":      "main",
		"merge_on_success": true,
	}
	if err := tool.Validate(args); err != nil {
		t.Errorf("Validate(): unexpected error for full valid args: %v", err)
	}
}

func TestTaskTool_Validate_AcceptsTimeoutAsFloat(t *testing.T) {
	// JSON decoders typically deliver numbers as float64; the validator
	// must accept whole-number floats.
	tool := NewTaskTool(newFakeOK())
	args := validArgs()
	args["timeout_seconds"] = float64(120)
	if err := tool.Validate(args); err != nil {
		t.Errorf("Validate(): unexpected error for float64 timeout_seconds: %v", err)
	}
}

// ---------- Execute ----------

func TestTaskTool_Execute_DispatchesAndWaits(t *testing.T) {
	fake := newFakeOK()
	tool := NewTaskTool(fake)

	_, err := tool.Execute(context.Background(), validArgs())
	if err != nil {
		t.Fatalf("Execute(): unexpected error: %v", err)
	}

	_, dispatchCalls, waitIDs, waitCalls := fake.snapshot()
	if dispatchCalls != 1 {
		t.Errorf("Dispatch call count: got %d want 1", dispatchCalls)
	}
	if waitCalls != 1 {
		t.Errorf("WaitAll call count: got %d want 1", waitCalls)
	}
	if len(waitIDs) != 1 || waitIDs[0] != fake.dispatchID {
		t.Errorf("WaitAll IDs: got %v want [%q]", waitIDs, fake.dispatchID)
	}
}

func TestTaskTool_Execute_PropagatesDispatchError(t *testing.T) {
	fake := newFakeOK()
	fake.dispatchErr = subagent.ErrMaxConcurrency
	tool := NewTaskTool(fake)

	_, err := tool.Execute(context.Background(), validArgs())
	if err == nil {
		t.Fatalf("Execute(): expected error, got nil")
	}
	if !errors.Is(err, subagent.ErrMaxConcurrency) {
		t.Errorf("Execute(): expected wrapped ErrMaxConcurrency, got %v", err)
	}
}

func TestTaskTool_Execute_PropagatesWaitAllError(t *testing.T) {
	fake := newFakeOK()
	fake.waitAllErr = errors.New("boom-wait-all")
	fake.waitAllResult = nil
	tool := NewTaskTool(fake)

	_, err := tool.Execute(context.Background(), validArgs())
	if err == nil {
		t.Fatalf("Execute(): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom-wait-all") {
		t.Errorf("Execute(): expected wrapped WaitAll error, got %v", err)
	}
}

func TestTaskTool_Execute_BuildsTaskCorrectly(t *testing.T) {
	fake := newFakeOK()
	tool := NewTaskTool(fake)

	args := map[string]interface{}{
		"description":      "summarise repo",
		"prompt":           "Read README.md and summarise it",
		"isolation":        "worktree",
		"subagent_type":    "code-reviewer",
		"timeout_seconds":  120,
		"base_branch":      "main",
		"merge_on_success": true,
	}
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute(): unexpected error: %v", err)
	}

	got, _, _, _ := fake.snapshot()
	if got.Description != "summarise repo" {
		t.Errorf("Description: got %q want %q", got.Description, "summarise repo")
	}
	if got.Prompt != "Read README.md and summarise it" {
		t.Errorf("Prompt: got %q", got.Prompt)
	}
	if got.Isolation != subagent.IsolationWorktree {
		t.Errorf("Isolation: got %q want %q", got.Isolation, subagent.IsolationWorktree)
	}
	if got.SubagentType != "code-reviewer" {
		t.Errorf("SubagentType: got %q", got.SubagentType)
	}
	if got.Timeout != 120*time.Second {
		t.Errorf("Timeout: got %v want %v", got.Timeout, 120*time.Second)
	}
	if got.BaseBranch != "main" {
		t.Errorf("BaseBranch: got %q", got.BaseBranch)
	}
	if !got.MergeOnSuccess {
		t.Errorf("MergeOnSuccess: got false want true")
	}
}

func TestTaskTool_Execute_DefaultIsolationNone(t *testing.T) {
	fake := newFakeOK()
	tool := NewTaskTool(fake)

	if _, err := tool.Execute(context.Background(), validArgs()); err != nil {
		t.Fatalf("Execute(): unexpected error: %v", err)
	}
	got, _, _, _ := fake.snapshot()
	if got.Isolation != subagent.IsolationNone {
		t.Errorf("Isolation default: got %q want %q", got.Isolation, subagent.IsolationNone)
	}
}

func TestTaskTool_Execute_TimeoutSecondsConverts(t *testing.T) {
	fake := newFakeOK()
	tool := NewTaskTool(fake)

	args := validArgs()
	args["timeout_seconds"] = 120

	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute(): unexpected error: %v", err)
	}
	got, _, _, _ := fake.snapshot()
	if got.Timeout != 120*time.Second {
		t.Errorf("Timeout: got %v want %v", got.Timeout, 120*time.Second)
	}
}

func TestTaskTool_Execute_ResultPropagated(t *testing.T) {
	fake := newFakeOK()
	canned := subagent.SubagentResult{
		TaskID:    "task-id-123",
		State:     subagent.StateSucceeded,
		Output:    "subagent-output",
		Duration:  500 * time.Millisecond,
		Isolation: subagent.IsolationNone,
	}
	fake.waitAllResult = []subagent.SubagentResult{canned}

	tool := NewTaskTool(fake)
	out, err := tool.Execute(context.Background(), validArgs())
	if err != nil {
		t.Fatalf("Execute(): unexpected error: %v", err)
	}
	gotResult, ok := out.(subagent.SubagentResult)
	if !ok {
		t.Fatalf("Execute(): expected SubagentResult, got %T", out)
	}
	if gotResult != canned {
		t.Errorf("Execute(): result not propagated byte-for-byte\n  got:  %+v\n  want: %+v", gotResult, canned)
	}
}

func TestTaskTool_Execute_EmptyResultsReturnsError(t *testing.T) {
	fake := newFakeOK()
	fake.waitAllResult = []subagent.SubagentResult{}
	tool := NewTaskTool(fake)

	_, err := tool.Execute(context.Background(), validArgs())
	if err == nil {
		t.Fatalf("Execute(): expected error for empty results, got nil")
	}
}

func TestTaskTool_Execute_ValidatesBeforeExecutor(t *testing.T) {
	fake := newFakeOK()
	tool := NewTaskTool(fake)

	// missing prompt — should not invoke executor
	args := map[string]interface{}{"description": "d"}
	if _, err := tool.Execute(context.Background(), args); err == nil {
		t.Fatalf("Execute(): expected validation error, got nil")
	}
	_, dispatchCalls, _, waitCalls := fake.snapshot()
	if dispatchCalls != 0 || waitCalls != 0 {
		t.Errorf("Execute(): executor invoked despite validation failure (dispatch=%d wait=%d)", dispatchCalls, waitCalls)
	}
}

// ---------- Tool interface compile-time assertion ----------

// _ is a compile-time assertion that *TaskTool satisfies tools.Tool.
// If the interface drifts, the build breaks here rather than silently at
// registry.Register time.
var _ tools.Tool = (*TaskTool)(nil)
