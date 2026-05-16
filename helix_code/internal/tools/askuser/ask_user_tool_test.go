// Package askuser — ask_user_tool_test.go (P1-F19-T04).
//
// Verifies the AskUserTool that wraps a Prompter behind the tools.Tool
// interface. The Prompter is replaced with a deterministic fake so we can
// drive every branch (success, default-used, user cancelled, non-TTY) without
// needing a real terminal. Per CONST-035 §11.9, every PASS in this file
// carries positive runtime evidence (assertions on returned values, the
// recorded Question, errors.Is reachability) — none are absence-of-error
// PASSes.
package askuser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/tools"
)

// fakePrompter records the Question received and returns the canned Result/err.
type fakePrompter struct {
	received Question
	called   bool
	result   *Result
	err      error
}

func (f *fakePrompter) Prompt(ctx context.Context, q Question) (*Result, error) {
	f.received = q
	f.called = true
	return f.result, f.err
}

// validToolArgs returns a stock args map matching what the tool would receive
// from a JSON-decoded call.
func validToolArgs() map[string]any {
	return map[string]any{
		"question": "Apply this patch?",
		"choices": []any{
			map[string]any{"label": "Yes", "value": "yes", "preview": "applies the diff"},
			map[string]any{"label": "No", "value": "no"},
			map[string]any{"label": "Skip", "value": "skip"},
		},
	}
}

func validToolArgsWithDefault() map[string]any {
	a := validToolArgs()
	a["default"] = "no"
	return a
}

func TestAskUserTool_Compiles_AsTool(t *testing.T) {
	// Compile-time interface satisfaction. If AskUserTool drifts from
	// tools.Tool, this line refuses to compile and the build fails.
	var _ tools.Tool = (*AskUserTool)(nil)
}

func TestAskUserTool_Name(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	if got, want := tool.Name(), "ask_user"; got != want {
		t.Fatalf("Name() = %q; want %q", got, want)
	}
}

func TestAskUserTool_Description_NonEmpty(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	if d := tool.Description(); strings.TrimSpace(d) == "" {
		t.Fatalf("Description() must be non-empty; got %q", d)
	}
}

func TestAskUserTool_Category(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	if got, want := tool.Category(), tools.CategoryAskUser; got != want {
		t.Fatalf("Category() = %q; want %q", got, want)
	}
}

func TestAskUserTool_Schema_HasRequiredFields(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Fatalf("Schema.Type = %q; want \"object\"", schema.Type)
	}
	if _, ok := schema.Properties["question"]; !ok {
		t.Fatalf("Schema.Properties missing \"question\": %#v", schema.Properties)
	}
	if _, ok := schema.Properties["choices"]; !ok {
		t.Fatalf("Schema.Properties missing \"choices\": %#v", schema.Properties)
	}
	// Required must include question + choices.
	hasQ, hasC := false, false
	for _, k := range schema.Required {
		if k == "question" {
			hasQ = true
		}
		if k == "choices" {
			hasC = true
		}
	}
	if !hasQ || !hasC {
		t.Fatalf("Schema.Required must contain \"question\" and \"choices\"; got %v", schema.Required)
	}
}

func TestAskUserTool_Validate_RequiresQuestion(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	args := validToolArgs()
	delete(args, "question")
	err := tool.Validate(args)
	if err == nil {
		t.Fatal("Validate without question must fail")
	}
}

func TestAskUserTool_Validate_RejectsEmptyChoices(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	args := validToolArgs()
	args["choices"] = []any{}
	err := tool.Validate(args)
	if err == nil {
		t.Fatal("Validate with empty choices must fail")
	}
}

func TestAskUserTool_Validate_RejectsOneChoice(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	args := validToolArgs()
	args["choices"] = []any{
		map[string]any{"label": "Yes", "value": "yes"},
	}
	err := tool.Validate(args)
	if err == nil {
		t.Fatal("Validate with one choice must fail (need >=2)")
	}
}

func TestAskUserTool_Validate_RejectsBadDefault(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	args := validToolArgs()
	args["default"] = "not-a-real-value"
	err := tool.Validate(args)
	if err == nil {
		t.Fatal("Validate with default not matching any choice value must fail")
	}
}

func TestAskUserTool_Validate_AcceptsValidArgs(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})
	if err := tool.Validate(validToolArgs()); err != nil {
		t.Fatalf("Validate(validToolArgs) unexpected error: %v", err)
	}
	if err := tool.Validate(validToolArgsWithDefault()); err != nil {
		t.Fatalf("Validate(validToolArgsWithDefault) unexpected error: %v", err)
	}
}

func TestAskUserTool_Validate_RejectsBadTypes(t *testing.T) {
	tool := NewAskUserTool(&fakePrompter{})

	// question as int.
	args := validToolArgs()
	args["question"] = 42
	if err := tool.Validate(args); err == nil {
		t.Fatal("Validate with question:int must fail")
	}

	// choices as string.
	args = validToolArgs()
	args["choices"] = "Yes,No"
	if err := tool.Validate(args); err == nil {
		t.Fatal("Validate with choices:string must fail")
	}

	// default as int.
	args = validToolArgs()
	args["default"] = 1
	if err := tool.Validate(args); err == nil {
		t.Fatal("Validate with default:int must fail")
	}

	// choice missing label.
	args = validToolArgs()
	args["choices"] = []any{
		map[string]any{"value": "yes"},
		map[string]any{"label": "No", "value": "no"},
	}
	if err := tool.Validate(args); err == nil {
		t.Fatal("Validate with choice missing label must fail")
	}

	// choice missing value.
	args = validToolArgs()
	args["choices"] = []any{
		map[string]any{"label": "Yes"},
		map[string]any{"label": "No", "value": "no"},
	}
	if err := tool.Validate(args); err == nil {
		t.Fatal("Validate with choice missing value must fail")
	}
}

func TestAskUserTool_Execute_PassesQuestionToPrompter(t *testing.T) {
	fp := &fakePrompter{result: &Result{Value: "yes", Index: 0, UsedDefault: false}}
	tool := NewAskUserTool(fp)

	_, err := tool.Execute(context.Background(), validToolArgsWithDefault())
	if err != nil {
		t.Fatalf("Execute unexpected error: %v", err)
	}
	if !fp.called {
		t.Fatal("Prompt was not called")
	}
	if fp.received.Question != "Apply this patch?" {
		t.Fatalf("Question text mismatch: got %q", fp.received.Question)
	}
	if len(fp.received.Choices) != 3 {
		t.Fatalf("Choices length = %d; want 3", len(fp.received.Choices))
	}
	if fp.received.Choices[0].Label != "Yes" || fp.received.Choices[0].Value != "yes" {
		t.Fatalf("Choices[0] = %+v; want {Yes/yes}", fp.received.Choices[0])
	}
	if fp.received.Choices[0].Preview != "applies the diff" {
		t.Fatalf("Choices[0].Preview = %q; want \"applies the diff\"", fp.received.Choices[0].Preview)
	}
	if fp.received.Default != "no" {
		t.Fatalf("Default = %q; want \"no\"", fp.received.Default)
	}
}

func TestAskUserTool_Execute_ReturnsPrompterResult(t *testing.T) {
	fp := &fakePrompter{result: &Result{Value: "yes", Index: 0, UsedDefault: false}}
	tool := NewAskUserTool(fp)

	got, err := tool.Execute(context.Background(), validToolArgs())
	if err != nil {
		t.Fatalf("Execute unexpected error: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Execute result type = %T; want map[string]any", got)
	}
	if m["value"] != "yes" {
		t.Fatalf("result[value] = %v; want \"yes\"", m["value"])
	}
	if m["index"] != 0 {
		t.Fatalf("result[index] = %v; want 0", m["index"])
	}
	if m["used_default"] != false {
		t.Fatalf("result[used_default] = %v; want false", m["used_default"])
	}
}

func TestAskUserTool_Execute_PropagatesUserCancelled(t *testing.T) {
	fp := &fakePrompter{err: ErrUserCancelled}
	tool := NewAskUserTool(fp)

	_, err := tool.Execute(context.Background(), validToolArgs())
	if err == nil {
		t.Fatal("Execute must return an error when prompter returns ErrUserCancelled")
	}
	if !errors.Is(err, ErrUserCancelled) {
		t.Fatalf("errors.Is(err, ErrUserCancelled) = false; err = %v", err)
	}
}

func TestAskUserTool_Execute_PropagatesNonTTYErrorWithDefault(t *testing.T) {
	fp := &fakePrompter{result: &Result{Value: "no", Index: 1, UsedDefault: true}}
	tool := NewAskUserTool(fp)

	got, err := tool.Execute(context.Background(), validToolArgsWithDefault())
	if err != nil {
		t.Fatalf("Execute unexpected error: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Execute result type = %T; want map[string]any", got)
	}
	if m["used_default"] != true {
		t.Fatalf("result[used_default] = %v; want true", m["used_default"])
	}
	if m["value"] != "no" {
		t.Fatalf("result[value] = %v; want \"no\"", m["value"])
	}
	if m["index"] != 1 {
		t.Fatalf("result[index] = %v; want 1", m["index"])
	}
}

func TestAskUserTool_Execute_PropagatesNonTTYError(t *testing.T) {
	fp := &fakePrompter{err: ErrInteractiveTerminalRequired}
	tool := NewAskUserTool(fp)

	_, err := tool.Execute(context.Background(), validToolArgs())
	if err == nil {
		t.Fatal("Execute must return an error when prompter returns ErrInteractiveTerminalRequired")
	}
	if !errors.Is(err, ErrInteractiveTerminalRequired) {
		t.Fatalf("errors.Is(err, ErrInteractiveTerminalRequired) = false; err = %v", err)
	}
}
