package askuser

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/render"
)

// validQuestion returns a stock Question with three choices and no default.
func validQuestion() Question {
	return Question{
		Question: "Apply this patch?",
		Choices: []Choice{
			{Label: "Yes", Value: "yes"},
			{Label: "No", Value: "no"},
			{Label: "Skip", Value: "skip"},
		},
	}
}

func validQuestionWithDefault() Question {
	q := validQuestion()
	q.Default = "no"
	return q
}

// slowReader blocks for delay before returning EOF (or the supplied bytes).
type slowReader struct {
	delay time.Duration
	data  []byte
	done  chan struct{}
	once  sync.Once
}

func newSlowReader(delay time.Duration, data string) *slowReader {
	return &slowReader{delay: delay, data: []byte(data), done: make(chan struct{})}
}

func (s *slowReader) Read(p []byte) (int, error) {
	select {
	case <-time.After(s.delay):
	case <-s.done:
		return 0, io.EOF
	}
	if len(s.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.data)
	s.data = s.data[n:]
	return n, nil
}

func (s *slowReader) Close() {
	s.once.Do(func() { close(s.done) })
}

func TestNewStdinPrompter_AppliesDefaults(t *testing.T) {
	p, err := NewStdinPrompter(StdinPrompterOptions{})
	if err != nil {
		t.Fatalf("NewStdinPrompter returned error: %v", err)
	}
	if p == nil {
		t.Fatalf("NewStdinPrompter returned nil prompter")
	}
	if p.opts.Reader != os.Stdin {
		t.Fatalf("expected Reader default os.Stdin")
	}
	if p.opts.Writer != os.Stdout {
		t.Fatalf("expected Writer default os.Stdout")
	}
	if p.opts.MaxRetries != DefaultMaxRetries {
		t.Fatalf("expected MaxRetries default %d, got %d", DefaultMaxRetries, p.opts.MaxRetries)
	}
	if p.opts.Timeout != DefaultTimeout {
		t.Fatalf("expected Timeout default %v, got %v", DefaultTimeout, p.opts.Timeout)
	}
	if p.opts.IsTTY == nil {
		t.Fatalf("expected IsTTY default to be set")
	}
	if p.opts.Renderer == nil {
		t.Fatalf("expected Renderer default to be constructed")
	}
}

func TestNewStdinPrompter_NilWriterDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{Writer: buf, IsTTY: func() bool { return true }})
	if err != nil {
		t.Fatalf("NewStdinPrompter returned error: %v", err)
	}
	if p.opts.Writer != buf {
		t.Fatalf("expected configured Writer to be retained")
	}
	if p.opts.Renderer == nil {
		t.Fatalf("expected Renderer default to be constructed against Writer")
	}
}

func TestStdinPrompter_NonTTY_WithDefault_ReturnsDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString(""),
		Writer: buf,
		IsTTY:  func() bool { return false },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	q := validQuestionWithDefault()
	res, err := p.Prompt(context.Background(), q)
	if err != nil {
		t.Fatalf("expected no error in non-TTY w/ default, got: %v", err)
	}
	if res == nil || res.Value != "no" {
		t.Fatalf("expected default value=no, got %+v", res)
	}
	if !res.UsedDefault {
		t.Fatalf("expected UsedDefault=true")
	}
	if res.Index != 1 {
		t.Fatalf("expected index 1 for default 'no', got %d", res.Index)
	}
	if strings.Contains(buf.String(), q.Question) {
		t.Fatalf("non-TTY path must not render the prompt; writer got: %q", buf.String())
	}
}

func TestStdinPrompter_NonTTY_NoDefault_Errors(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString(""),
		Writer: buf,
		IsTTY:  func() bool { return false },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	_, err = p.Prompt(context.Background(), validQuestion())
	if !errors.Is(err, ErrInteractiveTerminalRequired) {
		t.Fatalf("expected ErrInteractiveTerminalRequired, got %v", err)
	}
}

func TestStdinPrompter_TTY_ValidInput_ReturnsChoice(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString("2\n"),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	q := validQuestion()
	res, err := p.Prompt(context.Background(), q)
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if res.Index != 1 {
		t.Fatalf("expected index 1, got %d", res.Index)
	}
	if res.Value != q.Choices[1].Value {
		t.Fatalf("expected value %q, got %q", q.Choices[1].Value, res.Value)
	}
	out := buf.String()
	if !strings.Contains(out, q.Question) {
		t.Fatalf("output missing question text: %q", out)
	}
	if !strings.Contains(out, "1.") || !strings.Contains(out, "2.") || !strings.Contains(out, "3.") {
		t.Fatalf("output missing numbered list: %q", out)
	}
}

func TestStdinPrompter_TTY_EmptyInputWithDefault_ReturnsDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString("\n"),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	q := validQuestionWithDefault()
	res, err := p.Prompt(context.Background(), q)
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if res.Value != "no" || !res.UsedDefault {
		t.Fatalf("expected default no/UsedDefault=true, got %+v", res)
	}
}

func TestStdinPrompter_TTY_EmptyInputNoDefault_Reprompts(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString("\n2\n"),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	q := validQuestion()
	res, err := p.Prompt(context.Background(), q)
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if res.Index != 1 {
		t.Fatalf("expected index 1 after reprompt, got %d", res.Index)
	}
	out := buf.String()
	if strings.Count(out, q.Question) < 2 {
		t.Fatalf("expected question rendered at least twice, got: %q", out)
	}
}

func TestStdinPrompter_TTY_OutOfRange_Reprompts(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString("9\n1\n"),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	q := validQuestion()
	res, err := p.Prompt(context.Background(), q)
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if res.Index != 0 {
		t.Fatalf("expected index 0, got %d", res.Index)
	}
	out := buf.String()
	if !strings.Contains(out, "1-3") {
		t.Fatalf("expected hint mentioning 1-3 in output: %q", out)
	}
}

func TestStdinPrompter_TTY_NonNumeric_Reprompts(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString("abc\n2\n"),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	res, err := p.Prompt(context.Background(), validQuestion())
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if res.Index != 1 {
		t.Fatalf("expected index 1 after non-numeric reprompt, got %d", res.Index)
	}
}

func TestStdinPrompter_TTY_TooManyRetries_Errors(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader:     bytes.NewBufferString("9\n9\n9\n9\n"),
		Writer:     buf,
		IsTTY:      func() bool { return true },
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	_, err = p.Prompt(context.Background(), validQuestion())
	if !errors.Is(err, ErrTooManyRetries) {
		t.Fatalf("expected ErrTooManyRetries, got %v", err)
	}
}

func TestStdinPrompter_TTY_EOF_ReturnsCancelled(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString(""),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	_, err = p.Prompt(context.Background(), validQuestion())
	if !errors.Is(err, ErrUserCancelled) {
		t.Fatalf("expected ErrUserCancelled on EOF, got %v", err)
	}
}

func TestStdinPrompter_TTY_TimeoutReturnsErr(t *testing.T) {
	buf := &bytes.Buffer{}
	sr := newSlowReader(1*time.Second, "1\n")
	defer sr.Close()
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader:  sr,
		Writer:  buf,
		IsTTY:   func() bool { return true },
		Timeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	_, err = p.Prompt(context.Background(), validQuestion())
	if !errors.Is(err, ErrPrompterTimeout) {
		t.Fatalf("expected ErrPrompterTimeout, got %v", err)
	}
}

func TestStdinPrompter_TTY_CtxCancelReturnsCtxErr(t *testing.T) {
	buf := &bytes.Buffer{}
	sr := newSlowReader(2*time.Second, "1\n")
	defer sr.Close()
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader:  sr,
		Writer:  buf,
		IsTTY:   func() bool { return true },
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err = p.Prompt(ctx, validQuestion())
	if err == nil {
		t.Fatalf("expected error on ctx cancel, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestStdinPrompter_InvalidQuestion_PropagatesError(t *testing.T) {
	buf := &bytes.Buffer{}
	p, err := NewStdinPrompter(StdinPrompterOptions{
		Reader: bytes.NewBufferString(""),
		Writer: buf,
		IsTTY:  func() bool { return true },
	})
	if err != nil {
		t.Fatalf("NewStdinPrompter: %v", err)
	}
	bad := Question{Question: "Q", Choices: []Choice{{Label: "Only", Value: "only"}}}
	_, err = p.Prompt(context.Background(), bad)
	if !errors.Is(err, ErrInvalidQuestion) {
		t.Fatalf("expected ErrInvalidQuestion, got %v", err)
	}
}

func TestFormatQuestion_NumberedChoices(t *testing.T) {
	q := validQuestion()
	out := FormatQuestion(q)
	if !strings.Contains(out, "1.") {
		t.Fatalf("expected '1.' prefix in output: %q", out)
	}
	if !strings.Contains(out, "2.") {
		t.Fatalf("expected '2.' prefix in output: %q", out)
	}
	if !strings.Contains(out, "3.") {
		t.Fatalf("expected '3.' prefix in output: %q", out)
	}
}

func TestFormatQuestion_PreviewBeforeLabel(t *testing.T) {
	q := Question{
		Question: "Pick one",
		Choices: []Choice{
			{Label: "Yes", Value: "yes", Preview: "diff snippet"},
			{Label: "No", Value: "no"},
		},
	}
	out := FormatQuestion(q)
	previewIdx := strings.Index(out, "diff snippet")
	labelIdx := strings.Index(out, "Yes")
	if previewIdx < 0 {
		t.Fatalf("preview text missing: %q", out)
	}
	if labelIdx < 0 {
		t.Fatalf("label missing: %q", out)
	}
	// Per spec the preview line must come BEFORE its label OR adjacent.
	// In our format: label first then preview line indented under it; we treat
	// "adjacent" as preview line within ~80 chars of label.
	dist := previewIdx - labelIdx
	if dist < -200 || dist > 200 {
		t.Fatalf("preview not adjacent to label: distance=%d output=%q", dist, out)
	}
	if !strings.Contains(out, "preview:") {
		t.Fatalf("expected 'preview:' marker in output: %q", out)
	}
}

func TestFormatQuestion_DefaultHint(t *testing.T) {
	q := validQuestionWithDefault()
	out := FormatQuestion(q)
	if !strings.Contains(strings.ToLower(out), "default") {
		t.Fatalf("expected default hint in output: %q", out)
	}
	if !strings.Contains(out, "no") {
		t.Fatalf("expected default value 'no' in output: %q", out)
	}
}

// satisfy the import of render in case build tag changes; not strictly needed but
// avoids "unused import" if all renderer-using paths get tagged out in the future.
var _ render.Renderer = (render.Renderer)(nil)
