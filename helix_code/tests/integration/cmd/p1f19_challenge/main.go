// p1f19_challenge runs the F19 ask_user harness end-to-end against real
// *bytes.Buffer reader/writer pairs. Every always-runs phase asserts byte- and
// position-level invariants on the captured output and on the reader state.
// Article XI 11.9 anti-bluff anchor: a regression that "succeeds" by faking a
// Result without consuming the reader or rendering to the writer will fail
// either the reader-consumption / reader-untouched invariant, the writer-empty
// invariant, or the byte-offset positive-evidence assertion.
//
// Phases (all five always run; no SKIPs):
//
//	A. TTY-WITH-INPUT-RETURNS-CHOICE   - stdinPrompter wrapped in AskUserTool
//	                                     reads "2\n" from the bytes.Buffer
//	                                     reader. Asserts Value=="b" Index==1,
//	                                     writer contains the question text +
//	                                     numbered list, and reader is fully
//	                                     consumed (Len()==0 post-read).
//	B. NON-TTY-WITH-DEFAULT-RETURNS-DEFAULT - non-TTY short-circuit. Reader
//	                                     pre-loaded with "2\n" but MUST NOT
//	                                     be read - reader.Len() is invariant
//	                                     across the call (load-bearing proof
//	                                     that non-TTY truly short-circuits).
//	                                     Writer must be empty (no rendering
//	                                     to non-TTY).
//	C. NON-TTY-NO-DEFAULT-ERRORS       - non-TTY without Default returns
//	                                     ErrInteractiveTerminalRequired via
//	                                     errors.Is. Writer empty.
//	D. PREVIEW-VISIBLE-IN-OUTPUT       - TTY; choices carry Preview text.
//	                                     Captures the byte offset of the
//	                                     preview substring within the writer
//	                                     bytes - positive evidence that
//	                                     preview is rendered (not just
//	                                     metadata).
//	E. INVALID-INPUT-RETRY             - TTY; reader has "9\n2\n" (out-of-
//	                                     range then valid). Asserts the
//	                                     prompter retried (writer contains
//	                                     the question rendered twice and a
//	                                     1-3 hint) and ultimately returned
//	                                     Value=="b" Index==1.
//
// Exit code 0 on PASS; exit 1 with a diagnostic on any check failure.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"dev.helix.code/internal/render"
	"dev.helix.code/internal/tools/askuser"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F19 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F19 challenge harness PASS")
	return nil
}

// canonicalChoices is the 3-choice fixture used across phases A, B, C, E.
// Choice values "a"/"b"/"c" map to indices 0/1/2 so input "2\n" must yield
// Value=="b" Index==1.
func canonicalChoices() []askuser.Choice {
	return []askuser.Choice{
		{Label: "Apply", Value: "a"},
		{Label: "Backout", Value: "b"},
		{Label: "Cancel", Value: "c"},
	}
}

// buildTool wires a stdinPrompter against the supplied reader/writer/isTTY
// closure and returns the AskUserTool that wraps it. The renderer is forced
// to plain mode so writer captures contain no ANSI escapes that would
// confound substring assertions.
func buildTool(r io.Reader, w io.Writer, isTTY bool) (*askuser.AskUserTool, error) {
	rend, err := render.NewRenderer(render.FactoryOptions{
		Writer: w,
		Mode:   render.ModePlain,
	})
	if err != nil {
		return nil, fmt.Errorf("renderer: %w", err)
	}
	pr, err := askuser.NewStdinPrompter(askuser.StdinPrompterOptions{
		Reader:   r,
		Writer:   w,
		IsTTY:    func() bool { return isTTY },
		Renderer: rend,
	})
	if err != nil {
		return nil, fmt.Errorf("prompter: %w", err)
	}
	return askuser.NewAskUserTool(pr), nil
}

// asMap normalises Tool.Execute's interface{} return into the map shape the
// AskUserTool documents (value/index/used_default).
func asMap(v interface{}) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tool result not a map[string]any; got %T", v)
	}
	return m, nil
}

// argsFor builds the JSON-style args map for AskUserTool.Execute. Choices are
// passed as []map[string]any (a shape coerceChoices accepts) so we exercise
// the same parser path the registry would feed.
func argsFor(question string, choices []askuser.Choice, def string) map[string]any {
	cs := make([]map[string]any, 0, len(choices))
	for _, c := range choices {
		entry := map[string]any{"label": c.Label, "value": c.Value}
		if c.Preview != "" {
			entry["preview"] = c.Preview
		}
		cs = append(cs, entry)
	}
	args := map[string]any{
		"question": question,
		"choices":  cs,
	}
	if def != "" {
		args["default"] = def
	}
	return args
}

// phaseA: TTY + reader carrying "2\n" must yield Value=="b" Index==1 with the
// reader fully consumed and the writer carrying the rendered prompt body.
func phaseA() error {
	fmt.Println("==> phase A: TTY-WITH-INPUT-RETURNS-CHOICE (always runs)")

	const input = "2\n"
	reader := bytes.NewBufferString(input)
	initialLen := reader.Len()
	var writer bytes.Buffer

	tool, err := buildTool(reader, &writer, true)
	if err != nil {
		return err
	}

	res, err := tool.Execute(context.Background(), argsFor("Pick:", canonicalChoices(), ""))
	if err != nil {
		return fmt.Errorf("Execute: %w", err)
	}
	m, err := asMap(res)
	if err != nil {
		return err
	}
	if m["value"] != "b" {
		return fmt.Errorf("value mismatch: got %v want \"b\"", m["value"])
	}
	if m["index"] != 1 {
		return fmt.Errorf("index mismatch: got %v want 1", m["index"])
	}
	if m["used_default"] != false {
		return fmt.Errorf("used_default mismatch: got %v want false", m["used_default"])
	}

	out := writer.String()
	if !strings.Contains(out, "Pick:") {
		return fmt.Errorf("writer missing question text \"Pick:\"; got %q", out)
	}
	for i, c := range canonicalChoices() {
		want := fmt.Sprintf("%d. %s", i+1, c.Label)
		if !strings.Contains(out, want) {
			return fmt.Errorf("writer missing numbered choice %q; got %q", want, out)
		}
	}

	// Reader-consumption invariant: bufio.NewReader inside stdinPrompter wraps
	// the supplied reader and reads up to and through "\n". The single-line
	// input is fully consumed; the underlying bytes.Buffer must report Len==0.
	if reader.Len() != 0 {
		return fmt.Errorf("reader not fully consumed: %d bytes remain (initial %d)", reader.Len(), initialLen)
	}

	fmt.Printf("    phaseA: input=%q -> value=%q index=%v; writer-bytes=%d; reader-remaining=%d\n",
		input, m["value"], m["index"], len(out), reader.Len())
	fmt.Printf("    verdict: ask_user consumed input, returned correct choice, rendered prompt\n")
	return nil
}

// phaseB: non-TTY + Default. Reader pre-loaded but MUST NOT be touched.
// Writer MUST stay empty - no rendering to a non-TTY destination.
func phaseB() error {
	fmt.Println("==> phase B: NON-TTY-WITH-DEFAULT-RETURNS-DEFAULT (always runs)")

	reader := bytes.NewBufferString("2\n")
	initialLen := reader.Len()
	var writer bytes.Buffer

	tool, err := buildTool(reader, &writer, false)
	if err != nil {
		return err
	}

	res, err := tool.Execute(context.Background(), argsFor("Pick:", canonicalChoices(), "b"))
	if err != nil {
		return fmt.Errorf("Execute: %w", err)
	}
	m, err := asMap(res)
	if err != nil {
		return err
	}
	if m["value"] != "b" {
		return fmt.Errorf("value mismatch: got %v want \"b\"", m["value"])
	}
	if m["index"] != 1 {
		return fmt.Errorf("index mismatch: got %v want 1", m["index"])
	}
	if m["used_default"] != true {
		return fmt.Errorf("used_default mismatch: got %v want true", m["used_default"])
	}

	// Load-bearing reader-untouched invariant: a non-TTY short-circuit must
	// not have read a single byte from the buffer. If the prompter wrapped
	// the reader and called ReadString first, Len would have dropped.
	if reader.Len() != initialLen {
		return fmt.Errorf("reader was touched: initial=%d after=%d (non-TTY path must NOT read)",
			initialLen, reader.Len())
	}

	if writer.Len() != 0 {
		return fmt.Errorf("writer not empty after non-TTY call: %d bytes %q",
			writer.Len(), writer.String())
	}

	fmt.Printf("    phaseB: non-TTY+default=\"b\" -> value=%q used_default=%v; reader-remaining=%d (untouched=%t); writer-bytes=%d (empty=%t)\n",
		m["value"], m["used_default"], reader.Len(), reader.Len() == initialLen,
		writer.Len(), writer.Len() == 0)
	fmt.Printf("    verdict: non-TTY short-circuit honoured Default without reading reader or writing to writer\n")
	return nil
}

// phaseC: non-TTY + no Default => ErrInteractiveTerminalRequired (errors.Is).
func phaseC() error {
	fmt.Println("==> phase C: NON-TTY-NO-DEFAULT-ERRORS (always runs)")

	reader := bytes.NewBufferString("")
	var writer bytes.Buffer

	tool, err := buildTool(reader, &writer, false)
	if err != nil {
		return err
	}

	res, execErr := tool.Execute(context.Background(), argsFor("Pick:", canonicalChoices(), ""))
	if execErr == nil {
		return fmt.Errorf("expected error, got nil with result %v", res)
	}
	if !errors.Is(execErr, askuser.ErrInteractiveTerminalRequired) {
		return fmt.Errorf("error chain missing ErrInteractiveTerminalRequired: %v", execErr)
	}

	if writer.Len() != 0 {
		return fmt.Errorf("writer not empty on error path: %d bytes %q",
			writer.Len(), writer.String())
	}

	fmt.Printf("    phaseC: non-TTY+no-default -> %v; errors.Is(ErrInteractiveTerminalRequired)=true; writer-bytes=%d\n",
		execErr, writer.Len())
	fmt.Printf("    verdict: error sentinel propagated through tool wrapper; writer untouched\n")
	return nil
}

// phaseD: TTY; choices carry Preview text. The writer MUST contain the
// preview text of the first choice; we record the byte offset where the
// preview substring starts as positive runtime evidence.
func phaseD() error {
	fmt.Println("==> phase D: PREVIEW-VISIBLE-IN-OUTPUT (always runs)")

	const previewA = "applies the change to disk"
	const previewB = "discards the staged change"
	choices := []askuser.Choice{
		{Label: "Apply", Value: "a", Preview: previewA},
		{Label: "Backout", Value: "b", Preview: previewB},
		{Label: "Cancel", Value: "c"},
	}

	reader := bytes.NewBufferString("1\n")
	var writer bytes.Buffer

	tool, err := buildTool(reader, &writer, true)
	if err != nil {
		return err
	}

	res, err := tool.Execute(context.Background(), argsFor("Choose action:", choices, ""))
	if err != nil {
		return fmt.Errorf("Execute: %w", err)
	}
	m, err := asMap(res)
	if err != nil {
		return err
	}
	if m["value"] != "a" {
		return fmt.Errorf("value mismatch: got %v want \"a\"", m["value"])
	}

	out := writer.String()
	offsetA := strings.Index(out, previewA)
	if offsetA < 0 {
		return fmt.Errorf("preview text %q absent from writer output: %q", previewA, out)
	}
	offsetB := strings.Index(out, previewB)
	if offsetB < 0 {
		return fmt.Errorf("preview text %q absent from writer output: %q", previewB, out)
	}

	fmt.Printf("    phaseD: preview %q appears at byte offset %d; preview %q appears at byte offset %d; writer-bytes=%d\n",
		previewA, offsetA, previewB, offsetB, len(out))
	fmt.Printf("    verdict: choice previews rendered to writer (positive byte-offset evidence, not metadata)\n")
	return nil
}

// phaseE: TTY; reader has "9\n2\n" (out-of-range then valid). The prompter
// must redraw the prompt after the rejected input and ultimately return the
// valid choice. Writer must contain the question text twice (one per attempt)
// and the "1-3" hint.
func phaseE() error {
	fmt.Println("==> phase E: INVALID-INPUT-RETRY (always runs)")

	const input = "9\n2\n"
	const question = "Pick:"
	reader := bytes.NewBufferString(input)
	var writer bytes.Buffer

	tool, err := buildTool(reader, &writer, true)
	if err != nil {
		return err
	}

	res, err := tool.Execute(context.Background(), argsFor(question, canonicalChoices(), ""))
	if err != nil {
		return fmt.Errorf("Execute: %w", err)
	}
	m, err := asMap(res)
	if err != nil {
		return err
	}
	if m["value"] != "b" {
		return fmt.Errorf("value mismatch: got %v want \"b\"", m["value"])
	}
	if m["index"] != 1 {
		return fmt.Errorf("index mismatch: got %v want 1", m["index"])
	}

	out := writer.String()
	promptCount := strings.Count(out, question)
	if promptCount < 2 {
		return fmt.Errorf("expected >=2 question renderings in writer (one per attempt); got %d in %q",
			promptCount, out)
	}
	if !strings.Contains(out, "1-3") {
		return fmt.Errorf("writer missing 1-3 range hint after invalid input; got %q", out)
	}

	if reader.Len() != 0 {
		return fmt.Errorf("reader not fully consumed: %d bytes remain", reader.Len())
	}

	fmt.Printf("    phaseE: invalid-then-valid retry succeeded; question rendered %d time(s); 1-3 hint present; writer-bytes=%d; reader-remaining=%d\n",
		promptCount, len(out), reader.Len())
	fmt.Printf("    verdict: prompter rejected out-of-range input, redrew prompt, accepted valid follow-up\n")
	return nil
}
