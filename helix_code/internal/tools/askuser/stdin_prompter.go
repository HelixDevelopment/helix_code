// Package askuser — stdinPrompter (P1-F19-T03).
//
// stdinPrompter is the production Prompter implementation. It reads the user's
// choice from a buffered stdin reader and renders the question + numbered
// choice menu through the F18 render package so output respects the
// fancy/plain mode of the active terminal.
//
// Behaviour summary (full contract documented on Prompt):
//   - Validates the Question via Question.Validate before any rendering.
//   - When the destination is NOT a TTY: the prompter never renders and never
//     reads. If the question carries a Default the default value is returned
//     with UsedDefault=true; otherwise ErrInteractiveTerminalRequired is
//     returned. This matches CONST-035 §11.9 — non-interactive callers must
//     not be silently blocked on a question they cannot answer.
//   - When the destination IS a TTY: the question is rendered through
//     render.RenderTextBlock and a single line is read from the input. Empty
//     input falls back to Default when present; otherwise it counts as a
//     retry and the prompt is redrawn with a hint. After MaxRetries invalid
//     attempts ErrTooManyRetries is returned.
//   - Timeout and ctx cancellation: the read happens on a background
//     goroutine; the main goroutine selects across the result channel,
//     time.After(Timeout), and ctx.Done(). The reader goroutine may leak past
//     timeout/cancel — spec §11.10 documents this is acceptable for v1
//     because the underlying io.Reader is typically os.Stdin which lives for
//     the life of the process.
package askuser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"dev.helix.code/internal/render"
)

// StdinPrompterOptions configures NewStdinPrompter. Every field has a sane
// default so the zero value is a usable construction call.
type StdinPrompterOptions struct {
	// Reader is the input source. Defaults to os.Stdin when nil.
	Reader io.Reader
	// Writer is the destination for rendered prompts. Defaults to os.Stdout
	// when nil.
	Writer io.Writer
	// IsTTY reports whether the destination is an interactive terminal.
	// Defaults to a probe that returns true iff Writer is *os.File and the
	// underlying fd is a TTY. Tests inject a deterministic closure.
	IsTTY func() bool
	// Renderer is the F18 renderer used to draw the question. Defaults to a
	// renderer constructed against Writer via render.NewRenderer.
	Renderer render.Renderer
	// MaxRetries is the cap on invalid attempts before ErrTooManyRetries.
	// Defaults to DefaultMaxRetries (3).
	MaxRetries int
	// Timeout is the deadline for the user's response. Defaults to
	// DefaultTimeout (5 minutes). The deadline applies to each line read,
	// so a re-prompt resets the clock.
	Timeout time.Duration
}

// stdinPrompter is the concrete Prompter implementation backed by stdin.
type stdinPrompter struct {
	opts StdinPrompterOptions
}

// blockIDCounter is used to mint unique BlockIDs for ask-user prompts so
// successive Prompt calls in the same process do not alias against each other
// in the renderer's frame cache. Atomic so concurrent Prompt calls (rare but
// possible) can't collide.
var blockIDCounter uint64

// NewStdinPrompter constructs a stdinPrompter, applying defaults for any
// unset field. Returns an error only if the renderer default cannot be
// constructed (which in turn surfaces an invalid HELIXCODE_RENDER value).
func NewStdinPrompter(opts StdinPrompterOptions) (*stdinPrompter, error) {
	if opts.Reader == nil {
		opts.Reader = os.Stdin
	}
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.IsTTY == nil {
		w := opts.Writer
		opts.IsTTY = func() bool {
			f, ok := w.(*os.File)
			if !ok {
				return false
			}
			// Use the same probe gateway as the render factory: defer to
			// the OS via a stat fallback when not on a TTY.
			fi, err := f.Stat()
			if err != nil {
				return false
			}
			return (fi.Mode() & os.ModeCharDevice) != 0
		}
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = DefaultMaxRetries
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Renderer == nil {
		r, err := render.NewRenderer(render.FactoryOptions{Writer: opts.Writer})
		if err != nil {
			return nil, fmt.Errorf("askuser: construct default renderer: %w", err)
		}
		opts.Renderer = r
	}
	return &stdinPrompter{opts: opts}, nil
}

// Prompt implements Prompter — see package doc for the full contract.
func (p *stdinPrompter) Prompt(ctx context.Context, q Question) (*Result, error) {
	if err := q.Validate(); err != nil {
		return nil, fmt.Errorf("askuser: validate question: %w", err)
	}

	if !p.opts.IsTTY() {
		if q.HasDefault() {
			idx, ok := defaultIndex(q)
			if !ok {
				// Validate would have caught this; defensive only.
				return nil, fmt.Errorf("askuser: %w", ErrDefaultNotFound)
			}
			return &Result{
				Value:       q.Default,
				Index:       idx,
				UsedDefault: true,
			}, nil
		}
		return nil, fmt.Errorf("askuser: %w", ErrInteractiveTerminalRequired)
	}

	blockID := nextAskBlockID()
	bufReader := bufio.NewReader(p.opts.Reader)

	hint := ""
	for attempt := 0; attempt <= p.opts.MaxRetries; attempt++ {
		// Render question + (optional) hint. We render through the F18
		// renderer so plain mode strips ANSI and fancy mode can later
		// diff successive prompts against the same blockID.
		body := FormatQuestion(q)
		if hint != "" {
			body = hint + "\n" + body
		}
		if err := render.RenderTextBlock(p.opts.Renderer, blockID, body); err != nil {
			return nil, fmt.Errorf("askuser: render prompt: %w", err)
		}

		line, err := readLineWithTimeout(ctx, bufReader, p.opts.Timeout)
		if err != nil {
			return nil, err
		}

		raw := strings.TrimRight(line, "\r\n")
		raw = strings.TrimSpace(raw)

		// Empty input + default => accept default.
		if raw == "" {
			if q.HasDefault() {
				idx, ok := defaultIndex(q)
				if !ok {
					return nil, fmt.Errorf("askuser: %w", ErrDefaultNotFound)
				}
				return &Result{
					Value:       q.Default,
					Index:       idx,
					UsedDefault: true,
				}, nil
			}
			hint = invalidChoiceHint(ctx, len(q.Choices))
			continue
		}

		// Numeric input.
		n, perr := strconv.Atoi(raw)
		if perr != nil {
			hint = invalidChoiceHint(ctx, len(q.Choices))
			continue
		}
		if n < 1 || n > len(q.Choices) {
			hint = invalidChoiceHint(ctx, len(q.Choices))
			continue
		}

		choice := q.Choices[n-1]
		return &Result{
			Value:       choice.Value,
			Index:       n - 1,
			UsedDefault: false,
		}, nil
	}

	return nil, fmt.Errorf("askuser: %w (max=%d)", ErrTooManyRetries, p.opts.MaxRetries)
}

// readLineWithTimeout reads a single line from r, honouring both the supplied
// timeout and ctx cancellation. The reader goroutine may leak past
// timeout/cancel — see package doc for the rationale.
func readLineWithTimeout(ctx context.Context, r *bufio.Reader, timeout time.Duration) (string, error) {
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		s, err := r.ReadString('\n')
		ch <- readResult{line: s, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case rr := <-ch:
		if rr.err != nil {
			// EOF without any data => user cancelled. EOF after a partial
			// line still means the input stream is gone; treat as
			// cancelled too (the partial line cannot be a valid choice
			// because it lacks a terminator and we drop the line).
			if errors.Is(rr.err, io.EOF) {
				return "", fmt.Errorf("askuser: %w", ErrUserCancelled)
			}
			return "", fmt.Errorf("askuser: read input: %w", rr.err)
		}
		return rr.line, nil
	case <-timer.C:
		return "", fmt.Errorf("askuser: %w (after %s)", ErrPrompterTimeout, timeout)
	case <-ctx.Done():
		return "", fmt.Errorf("askuser: ctx cancelled: %w", ctx.Err())
	}
}

// defaultIndex returns the index of the choice whose Value matches q.Default.
// The second return value reports whether such a choice was found; Validate
// guarantees ok==true when q.HasDefault, so callers may treat false as a bug.
func defaultIndex(q Question) (int, bool) {
	for i, c := range q.Choices {
		if c.Value == q.Default {
			return i, true
		}
	}
	return 0, false
}

// nextAskBlockID mints a unique BlockID per Prompt call. Successive frames
// within a single Prompt (re-prompt on invalid input) share the same ID so
// the renderer's diff path can reuse the previous frame's geometry.
func nextAskBlockID() string {
	n := atomic.AddUint64(&blockIDCounter, 1)
	return fmt.Sprintf("ask-user-%d", n)
}

// invalidChoiceHint resolves the CONST-046 retry hint shown after empty,
// non-numeric, or out-of-range input. max is the highest valid choice number.
// Routed through the package translator seam (tr) so non-English operators see
// the prompt in their locale — see translator.go / i18n/translator.go.
func invalidChoiceHint(ctx context.Context, max int) string {
	return tr(ctx, "askuser_prompt_invalid_choice_hint", map[string]any{
		"Max": max,
	})
}

// FormatQuestion renders a Question into the human-readable string that the
// prompter writes to the output. Exported so tests can assert on the format
// directly and so the future /ask slash command can reuse it without
// constructing a prompter. Pure relative to its inputs: no I/O, no time —
// user-facing literals resolve through the package translator seam (CONST-046)
// against context.Background(), so a wired *i18nadapter.Translator localises
// the menu footer + preview label without changing the call signature.
//
// Format:
//
//	<question text>
//
//	1. <Choice[0].Label>
//	   preview: <Choice[0].Preview>      (only when preview != "")
//	2. <Choice[1].Label>
//	...
//	Enter choice [1-N][, default <Default>]:
func FormatQuestion(q Question) string {
	ctx := context.Background()
	var b strings.Builder
	b.WriteString(q.Question)
	b.WriteString("\n\n")
	for i, c := range q.Choices {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c.Label)
		if c.Preview != "" {
			b.WriteString(tr(ctx, "askuser_prompt_choice_preview_label", map[string]any{
				"Preview": c.Preview,
			}))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	if q.HasDefault() {
		b.WriteString(tr(ctx, "askuser_prompt_enter_choice_with_default", map[string]any{
			"Max":     len(q.Choices),
			"Default": q.Default,
		}))
	} else {
		b.WriteString(tr(ctx, "askuser_prompt_enter_choice_no_default", map[string]any{
			"Max": len(q.Choices),
		}))
	}
	return b.String()
}
