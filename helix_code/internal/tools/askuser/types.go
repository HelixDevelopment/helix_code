// Package askuser defines the contract for an interactive ask_user tool that
// pauses execution to ask the human operator a multiple-choice question and
// returns their selection. The package contains only types, the Prompter
// interface, and sentinel errors; concrete prompters live in sibling files
// (e.g. stdin_prompter.go in T03).
package askuser

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Choice is one option in a Question.
type Choice struct {
	Label   string `json:"label"`             // short label (e.g., "Yes")
	Value   string `json:"value"`             // value returned to caller (e.g., "yes")
	Preview string `json:"preview,omitempty"` // optional inline preview text shown above the label
}

// IsZero reports whether c equals its zero value (all fields empty).
func (c Choice) IsZero() bool {
	return c.Label == "" && c.Value == "" && c.Preview == ""
}

// Question is the input to a Prompter.
type Question struct {
	Question string   `json:"question"`          // the question text
	Choices  []Choice `json:"choices"`           // >= 2 entries
	Default  string   `json:"default,omitempty"` // optional; must match a choice value if set
}

// HasDefault reports whether Default is non-empty.
func (q Question) HasDefault() bool {
	return q.Default != ""
}

// Validate enforces the structural invariants of a Question:
//   - Question text is non-empty.
//   - Choices has at least 2 entries.
//   - Each Choice.Label and Choice.Value is non-empty.
//   - Choice values are unique across the slice.
//   - Default, when set, matches one Choice.Value exactly.
//
// On failure, Validate returns a sentinel error from this package wrapped with
// contextual detail; callers may use errors.Is to test against the sentinel.
func (q Question) Validate() error {
	if q.Question == "" {
		return fmt.Errorf("%w: %w", ErrInvalidQuestion, ErrEmptyQuestionText)
	}
	if len(q.Choices) < 2 {
		return fmt.Errorf("%w: %w (got %d)", ErrInvalidQuestion, ErrTooFewChoices, len(q.Choices))
	}
	seen := make(map[string]struct{}, len(q.Choices))
	for i, c := range q.Choices {
		if c.Label == "" {
			return fmt.Errorf("%w: %w (choice index %d)", ErrInvalidQuestion, ErrEmptyChoiceLabel, i)
		}
		if c.Value == "" {
			return fmt.Errorf("%w: %w (choice index %d)", ErrInvalidQuestion, ErrEmptyChoiceValue, i)
		}
		if _, dup := seen[c.Value]; dup {
			return fmt.Errorf("%w: %w (value %q at index %d)", ErrInvalidQuestion, ErrDuplicateChoiceValue, c.Value, i)
		}
		seen[c.Value] = struct{}{}
	}
	if q.Default != "" {
		if _, ok := seen[q.Default]; !ok {
			return fmt.Errorf("%w: %w (default %q)", ErrInvalidQuestion, ErrDefaultNotFound, q.Default)
		}
	}
	return nil
}

// Result is the prompter's output.
type Result struct {
	Value       string `json:"value"`        // the chosen Choice.Value
	Index       int    `json:"index"`        // 0-based index into Question.Choices
	UsedDefault bool   `json:"used_default"` // true when a non-TTY path auto-picked Default
}

// Prompter is the contract: render the question and read the user's choice.
// Implementations must respect ctx cancellation and surface package sentinel
// errors so callers can react with errors.Is.
type Prompter interface {
	Prompt(ctx context.Context, q Question) (*Result, error)
}

// Sentinel errors. Concrete prompters wrap these with contextual detail; tests
// and call sites compare with errors.Is.
var (
	ErrInvalidQuestion             = errors.New("invalid question")
	ErrEmptyQuestionText           = errors.New("question text required")
	ErrTooFewChoices               = errors.New("at least 2 choices required")
	ErrEmptyChoiceLabel            = errors.New("choice label required")
	ErrEmptyChoiceValue            = errors.New("choice value required")
	ErrDuplicateChoiceValue        = errors.New("duplicate choice value")
	ErrDefaultNotFound             = errors.New("default does not match any choice value")
	ErrUserCancelled               = errors.New("user cancelled (EOF)")
	ErrInteractiveTerminalRequired = errors.New("ask_user requires interactive terminal")
	ErrTooManyRetries              = errors.New("too many invalid responses")
	ErrPrompterTimeout             = errors.New("user did not respond within timeout")
)

// Default values exposed for callers; T03 stdinPrompter applies these when no
// override is supplied.
const (
	DefaultMaxRetries = 3
	DefaultTimeout    = 5 * time.Minute
)
