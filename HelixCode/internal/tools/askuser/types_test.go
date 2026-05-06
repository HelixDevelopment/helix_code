package askuser

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChoice_ZeroValueIsZero(t *testing.T) {
	var c Choice
	if !c.IsZero() {
		t.Fatalf("expected zero Choice to report IsZero true")
	}
}

func TestChoice_NonZero(t *testing.T) {
	cases := []Choice{
		{Label: "Yes"},
		{Value: "yes"},
		{Preview: "snippet"},
		{Label: "Yes", Value: "yes", Preview: "p"},
	}
	for i, c := range cases {
		if c.IsZero() {
			t.Fatalf("case %d: expected non-zero Choice, got IsZero=true (%+v)", i, c)
		}
	}
}

func TestQuestion_HasDefault_Empty(t *testing.T) {
	q := Question{Question: "Q", Choices: []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}}
	if q.HasDefault() {
		t.Fatalf("expected HasDefault=false for empty Default")
	}
}

func TestQuestion_HasDefault_Set(t *testing.T) {
	q := Question{Question: "Q", Choices: []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}, Default: "a"}
	if !q.HasDefault() {
		t.Fatalf("expected HasDefault=true when Default set")
	}
}

func TestQuestion_Validate_AcceptsValid(t *testing.T) {
	cases := []Question{
		{
			Question: "Pick one",
			Choices:  []Choice{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}},
		},
		{
			Question: "Three options",
			Choices: []Choice{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
				{Label: "C", Value: "c", Preview: "preview C"},
			},
			Default: "b",
		},
	}
	for i, q := range cases {
		if err := q.Validate(); err != nil {
			t.Fatalf("case %d: expected valid Question, got error %v", i, err)
		}
	}
}

func TestQuestion_Validate_RejectsEmptyQuestion(t *testing.T) {
	q := Question{
		Question: "",
		Choices:  []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}
	err := q.Validate()
	if !errors.Is(err, ErrEmptyQuestionText) {
		t.Fatalf("expected ErrEmptyQuestionText, got %v", err)
	}
}

func TestQuestion_Validate_RejectsZeroChoices(t *testing.T) {
	q := Question{Question: "Q", Choices: nil}
	err := q.Validate()
	if !errors.Is(err, ErrTooFewChoices) {
		t.Fatalf("expected ErrTooFewChoices, got %v", err)
	}
}

func TestQuestion_Validate_RejectsOneChoice(t *testing.T) {
	q := Question{Question: "Q", Choices: []Choice{{Label: "A", Value: "a"}}}
	err := q.Validate()
	if !errors.Is(err, ErrTooFewChoices) {
		t.Fatalf("expected ErrTooFewChoices, got %v", err)
	}
}

func TestQuestion_Validate_RejectsEmptyLabel(t *testing.T) {
	q := Question{
		Question: "Q",
		Choices:  []Choice{{Label: "", Value: "a"}, {Label: "B", Value: "b"}},
	}
	err := q.Validate()
	if !errors.Is(err, ErrEmptyChoiceLabel) {
		t.Fatalf("expected ErrEmptyChoiceLabel, got %v", err)
	}
}

func TestQuestion_Validate_RejectsEmptyValue(t *testing.T) {
	q := Question{
		Question: "Q",
		Choices:  []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: ""}},
	}
	err := q.Validate()
	if !errors.Is(err, ErrEmptyChoiceValue) {
		t.Fatalf("expected ErrEmptyChoiceValue, got %v", err)
	}
}

func TestQuestion_Validate_RejectsDuplicateValues(t *testing.T) {
	q := Question{
		Question: "Q",
		Choices: []Choice{
			{Label: "A", Value: "same"},
			{Label: "B", Value: "same"},
		},
	}
	err := q.Validate()
	if !errors.Is(err, ErrDuplicateChoiceValue) {
		t.Fatalf("expected ErrDuplicateChoiceValue, got %v", err)
	}
}

func TestQuestion_Validate_RejectsDefaultNotMatching(t *testing.T) {
	q := Question{
		Question: "Q",
		Choices:  []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
		Default:  "other",
	}
	err := q.Validate()
	if !errors.Is(err, ErrDefaultNotFound) {
		t.Fatalf("expected ErrDefaultNotFound, got %v", err)
	}
}

func TestQuestion_Validate_AcceptsDefaultMatching(t *testing.T) {
	q := Question{
		Question: "Q",
		Choices:  []Choice{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
		Default:  "b",
	}
	if err := q.Validate(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// stubPrompter exists purely as a compile-time check that Prompter is a usable
// interface.
type stubPrompter struct {
	resp *Result
	err  error
}

func (s *stubPrompter) Prompt(ctx context.Context, q Question) (*Result, error) {
	return s.resp, s.err
}

func TestPrompterInterface_Compiles(t *testing.T) {
	var p Prompter = &stubPrompter{
		resp: &Result{Value: "yes", Index: 0, UsedDefault: false},
	}
	res, err := p.Prompt(context.Background(), Question{
		Question: "Q",
		Choices:  []Choice{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Value != "yes" || res.Index != 0 || res.UsedDefault {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrInvalidQuestion,
		ErrEmptyQuestionText,
		ErrTooFewChoices,
		ErrEmptyChoiceLabel,
		ErrEmptyChoiceValue,
		ErrDuplicateChoiceValue,
		ErrDefaultNotFound,
		ErrUserCancelled,
		ErrInteractiveTerminalRequired,
		ErrTooManyRetries,
		ErrPrompterTimeout,
	}
	seen := make(map[string]bool, len(all))
	for i, e := range all {
		if e == nil {
			t.Fatalf("sentinel index %d is nil", i)
		}
		msg := e.Error()
		if msg == "" {
			t.Fatalf("sentinel index %d has empty message", i)
		}
		if seen[msg] {
			t.Fatalf("duplicate sentinel message %q at index %d", msg, i)
		}
		seen[msg] = true
	}
}

func TestDefaults_Documented(t *testing.T) {
	if DefaultMaxRetries != 3 {
		t.Fatalf("expected DefaultMaxRetries==3, got %d", DefaultMaxRetries)
	}
	if DefaultTimeout != 5*time.Minute {
		t.Fatalf("expected DefaultTimeout==5m, got %v", DefaultTimeout)
	}
}
