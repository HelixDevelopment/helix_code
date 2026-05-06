// Package askuser — ask_user_tool.go (P1-F19-T04).
//
// AskUserTool is the agent-callable Tool that wraps a Prompter (typically the
// stdinPrompter from T03) behind the tools.Tool interface. It is registered
// with the tool registry under the stable name "ask_user" and category
// tools.CategoryAskUser.
//
// The tool depends on the Prompter seam so it can be exercised in unit tests
// with a fake without needing a real interactive terminal. The production
// path wires *stdinPrompter via NewStdinPrompter (see T03).
//
// Argument shape (JSON, map[string]any via Tool.Execute):
//
//	question  string                       required
//	choices   []map[string]any             required, >= 2 entries
//	    each entry: label  string  required
//	                value  string  required
//	                preview string optional
//	default   string                       optional; must match a choice value
//
// Anti-bluff contract (CONST-035 §11.9): this tool NEVER fabricates a Result
// and NEVER simulates user input. Every Execute call routes through the wired
// Prompter (or, in tests, a fake that records the dispatch). Prompter errors
// (ErrUserCancelled, ErrInteractiveTerminalRequired, ErrTooManyRetries,
// ErrPrompterTimeout, ctx.Err) surface verbatim with %w wrapping so callers
// can match with errors.Is.
//
// Placement note: this tool lives in `internal/tools/askuser/` (a subpackage)
// rather than directly in `internal/tools/` to keep the Prompter type
// contract co-located with its sole consumer. The pattern mirrors F14's
// `internal/tools/sandbox/sandboxed_shell_tool.go` and F15's
// `internal/tools/task/task_tool.go`.
package askuser

import (
	"context"
	"fmt"

	"dev.helix.code/internal/tools"
)

// AskUserTool is the Tool implementation registered as "ask_user". A nil
// prompter is rejected at Execute time with a clear error rather than at
// construction so the registry can wire the tool before the prompter is fully
// constructed (mirrors the LSP/sandbox/task tool patterns).
type AskUserTool struct {
	prompter Prompter
}

// NewAskUserTool wires the tool to a Prompter (typically *stdinPrompter from
// T03 — but any Prompter works, including a unit-test fake).
func NewAskUserTool(p Prompter) *AskUserTool {
	return &AskUserTool{prompter: p}
}

// Name returns "ask_user" — the stable, claude-code-compatible name. Keeping
// the name stable across CLI agents lets CLAUDE.md / AGENTS.md prompts that
// mention "use the ask_user tool to ask the operator a multiple-choice
// question" work without per-CLI rewrites.
func (t *AskUserTool) Name() string { return "ask_user" }

// Description is shown to the agent so it knows when to call this tool. It
// must mention that the tool blocks until the user responds — otherwise the
// agent may try to use it for non-interactive flows where it would deadlock
// (the non-TTY path returns ErrInteractiveTerminalRequired in that case).
func (t *AskUserTool) Description() string {
	return "Ask the human operator a multiple-choice question and block until they respond. Renders the question + numbered choices through the F18 renderer; reads a single line from stdin. When stdout is not a TTY, returns the default value (if set) or an error. Use this when you need explicit human approval before a destructive or ambiguous action."
}

// Category returns tools.CategoryAskUser so registry filtering by category
// surfaces ask-user-related tools together.
func (t *AskUserTool) Category() tools.ToolCategory { return tools.CategoryAskUser }

// Schema returns the JSON schema for the tool's args. The shape matches the
// argument contract in the package doc comment above.
func (t *AskUserTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The question to ask the user. Required. Non-empty.",
			},
			"choices": map[string]interface{}{
				"type":        "array",
				"description": "List of >= 2 choices. Each choice is an object with required string fields \"label\" + \"value\" and optional \"preview\".",
				"minItems":    2,
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"label":   map[string]interface{}{"type": "string"},
						"value":   map[string]interface{}{"type": "string"},
						"preview": map[string]interface{}{"type": "string"},
					},
					"required": []string{"label", "value"},
				},
			},
			"default": map[string]interface{}{
				"type":        "string",
				"description": "Optional. The choice value to use when stdout is not a TTY (the prompter cannot prompt) OR when the user hits Enter on an empty input. Must match one of the choices' values exactly.",
			},
		},
		Required:    []string{"question", "choices"},
		Description: "Ask the human operator a multiple-choice question and block for their response.",
	}
}

// Validate enforces the args contract before Execute. The registry calls this
// before dispatch (registry.Execute path); we also call it defensively from
// inside Execute so direct callers (bypassing the registry) get the same
// protection.
//
// The bulk of the work delegates to Question.Validate (see types.go) — we
// first parse args into a Question and then re-use the canonical validator so
// the tool and the underlying types share a single validation contract.
func (t *AskUserTool) Validate(params map[string]interface{}) error {
	q, err := parseQuestionFromArgs(params)
	if err != nil {
		return err
	}
	if err := q.Validate(); err != nil {
		return err
	}
	return nil
}

// Execute parses args into a Question, dispatches to the prompter, and
// returns a map[string]any describing the Result. Prompter errors are wrapped
// with %w so errors.Is reaches the underlying sentinel (ErrUserCancelled,
// ErrInteractiveTerminalRequired, ErrTooManyRetries, ErrPrompterTimeout,
// ctx.Err).
//
// The result shape matches the Result struct's JSON tags:
//
//	{"value": <string>, "index": <int>, "used_default": <bool>}
//
// Returning a map (rather than the *Result struct) keeps the tool result
// JSON-encodable by callers that don't import this package.
func (t *AskUserTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	if t.prompter == nil {
		return nil, fmt.Errorf("ask_user tool: no prompter wired")
	}

	q, err := parseQuestionFromArgs(params)
	if err != nil {
		// Defensive: Validate already covered this, but guard against drift.
		return nil, err
	}

	res, err := t.prompter.Prompt(ctx, q)
	if err != nil {
		// Wrap with %w so callers can match against package sentinels via
		// errors.Is. The outer message gives a friendly summary; the wrapped
		// error preserves the typed sentinel.
		return nil, fmt.Errorf("ask_user: %w", err)
	}
	if res == nil {
		// Defensive: a prompter must return either a Result or an error. If
		// neither, surface a clear bug rather than panicking on nil deref.
		return nil, fmt.Errorf("ask_user: prompter returned nil result with nil error")
	}

	return map[string]any{
		"value":        res.Value,
		"index":        res.Index,
		"used_default": res.UsedDefault,
	}, nil
}

// parseQuestionFromArgs coerces the JSON-decoded args map into a Question.
// JSON arrays decode to []any whose elements are map[string]any (encoding/json
// default), but in-process callers may pass []map[string]any directly — both
// shapes are accepted.
//
// Returns a typed error on the first malformed field. Does NOT call
// Question.Validate — that step is the caller's responsibility (Validate or
// Execute) so this helper stays a pure parser.
func parseQuestionFromArgs(params map[string]interface{}) (Question, error) {
	var q Question

	rawQ, ok := params["question"]
	if !ok {
		return q, fmt.Errorf("question is required")
	}
	qStr, isString := rawQ.(string)
	if !isString {
		return q, fmt.Errorf("question must be a string, got %T", rawQ)
	}
	q.Question = qStr

	rawChoices, ok := params["choices"]
	if !ok {
		return q, fmt.Errorf("choices is required")
	}
	choices, err := coerceChoices(rawChoices)
	if err != nil {
		return q, err
	}
	q.Choices = choices

	if v, present := params["default"]; present {
		s, isString := v.(string)
		if !isString {
			return q, fmt.Errorf("default must be a string, got %T", v)
		}
		q.Default = s
	}

	return q, nil
}

// coerceChoices accepts either []any or []map[string]any (or, for kindness to
// in-process callers, []Choice) and returns a []Choice. Each entry must be a
// map with string label + string value (preview is optional).
func coerceChoices(raw any) ([]Choice, error) {
	switch v := raw.(type) {
	case []Choice:
		return v, nil
	case []map[string]any:
		out := make([]Choice, 0, len(v))
		for i, m := range v {
			c, err := coerceChoice(m, i)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case []any:
		out := make([]Choice, 0, len(v))
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("choices[%d] must be an object, got %T", i, item)
			}
			c, err := coerceChoice(m, i)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("choices must be an array, got %T", raw)
	}
}

// coerceChoice extracts label/value/preview from a single choice map.
func coerceChoice(m map[string]any, idx int) (Choice, error) {
	var c Choice

	rawLabel, ok := m["label"]
	if !ok {
		return c, fmt.Errorf("choices[%d] missing required field \"label\"", idx)
	}
	label, isString := rawLabel.(string)
	if !isString {
		return c, fmt.Errorf("choices[%d].label must be a string, got %T", idx, rawLabel)
	}
	c.Label = label

	rawValue, ok := m["value"]
	if !ok {
		return c, fmt.Errorf("choices[%d] missing required field \"value\"", idx)
	}
	value, isString := rawValue.(string)
	if !isString {
		return c, fmt.Errorf("choices[%d].value must be a string, got %T", idx, rawValue)
	}
	c.Value = value

	if rawPreview, present := m["preview"]; present {
		preview, isString := rawPreview.(string)
		if !isString {
			return c, fmt.Errorf("choices[%d].preview must be a string, got %T", idx, rawPreview)
		}
		c.Preview = preview
	}

	return c, nil
}
