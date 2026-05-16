// p1f18_challenge runs the F18 no-flicker rendering harness end-to-end against
// real *bytes.Buffer sinks (and the live os.Stdout when it happens to be a
// TTY). Every always-runs phase asserts byte-level invariants on the rendered
// output - Article XI 11.9 anti-bluff anchor: a regression that "succeeds"
// without emitting the documented control sequences will fail the byte-count
// assertions; a regression that loses dirty-region diffing will fail Phase C's
// strict-smaller invariant.
//
// Phases:
//
//	A. STREAMING-FANCY   - ansiRenderer over bytes.Buffer; Begin + 10 tokens +
//	                       Commit + Close. Asserts hide-cursor, CR+clear-line
//	                       per token, show-cursor, terminal newline.
//	B. STREAMING-PLAIN   - plainRenderer over bytes.Buffer; same 10 tokens.
//	                       Asserts ZERO ANSI + ZERO CR bytes; all 10 words
//	                       present in output.
//	C. DIRTY-REGION-DIFF - ansiRenderer; render a 3-line block then re-render
//	                       with one line changed. Asserts the second render's
//	                       byte delta is strictly smaller than the first
//	                       render's bytes (proving partial update) and contains
//	                       exactly one cursor-up sequence (proving in-place
//	                       single-line rewrite).
//	D. TTY-FALLBACK      - factory over bytes.Buffer (non-TTY by definition)
//	                       with env unset; asserts auto-detect picks ModePlain
//	                       and zero ANSI/CR is emitted.
//	E. REAL-TTY          - gated on term.IsTerminal(stdout). When stdout is a
//	                       real terminal, construct factory with env unset and
//	                       assert ModeFancy. Otherwise SKIP-OK with reason.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"

	"dev.helix.code/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F18 challenge harness pid:", os.Getpid())

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
	fmt.Println("==> P1-F18 challenge harness PASS")
	return nil
}

// streamWords is the canonical 10-word stream used by Phase A and Phase B.
// Each entry is "<word> " (trailing space, no newline) so the renderer
// receives no \n until the final Commit closes the block. This matches the
// shape of an LLM token stream where the model emits whitespace-bounded
// tokens until the response naturally terminates.
var streamWords = []string{
	"alpha ", "beta ", "gamma ", "delta ", "epsilon ",
	"zeta ", "eta ", "theta ", "iota ", "kappa ",
}

// phaseA exercises ansiRenderer over a bytes.Buffer. The captured bytes MUST
// contain the documented ANSI control sequences in the documented quantities.
// A regression that lost the hide-cursor first-Begin emit, that lost the
// CR+clear-line per-token emit, or that lost the terminal newline at Commit
// will fail one of the four assertions below.
func phaseA() error {
	fmt.Println("==> phase A: STREAMING-FANCY (always runs)")

	var buf bytes.Buffer
	r := render.NewANSIRenderer(&buf)

	if err := r.Begin("a"); err != nil {
		return fmt.Errorf("Begin: %w", err)
	}
	for _, w := range streamWords {
		if err := r.WriteToken(w); err != nil {
			return fmt.Errorf("WriteToken(%q): %w", w, err)
		}
	}
	if err := r.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if err := r.Close(); err != nil {
		return fmt.Errorf("Close: %w", err)
	}

	out := buf.Bytes()
	hideCount := strings.Count(string(out), "\x1b[?25l")
	showCount := strings.Count(string(out), "\x1b[?25h")
	crClearCount := strings.Count(string(out), "\r\x1b[K")

	if hideCount < 1 {
		return fmt.Errorf("hide-cursor sequence missing; want >=1, got %d", hideCount)
	}
	if showCount < 1 {
		return fmt.Errorf("show-cursor sequence missing; want >=1, got %d", showCount)
	}
	if crClearCount < len(streamWords) {
		return fmt.Errorf("CR+clear-line count %d < tokens %d", crClearCount, len(streamWords))
	}
	if !bytes.HasSuffix(out, []byte("\n\x1b[?25h")) && !bytes.Contains(out, []byte("\n")) {
		return fmt.Errorf("terminal newline missing from output")
	}
	// Stronger: there MUST be at least one \n somewhere in the captured
	// bytes (Commit emits one when the line is non-empty).
	if !bytes.Contains(out, []byte("\n")) {
		return fmt.Errorf("output contains no newline; Commit did not emit terminator")
	}

	fmt.Printf("    phaseA: bytes=%d; hide-cursor=%d; CR-clear=%d; show-cursor=%d\n",
		len(out), hideCount, crClearCount, showCount)
	fmt.Printf("    verdict: ANSI control sequences emitted in expected quantities\n")
	return nil
}

// phaseB exercises plainRenderer over a bytes.Buffer. The captured bytes MUST
// contain ZERO ANSI escapes (0x1b) and ZERO CR (0x0d) - this is the
// load-bearing zero-ANSI / zero-CR invariant for non-TTY destinations. The
// captured bytes MUST also contain every word from streamWords (visible
// content reaches the writer; the renderer is not eating tokens).
func phaseB() error {
	fmt.Println("==> phase B: STREAMING-PLAIN (always runs)")

	var buf bytes.Buffer
	r := render.NewPlainRenderer(&buf)

	if err := r.Begin("b"); err != nil {
		return fmt.Errorf("Begin: %w", err)
	}
	for _, w := range streamWords {
		if err := r.WriteToken(w); err != nil {
			return fmt.Errorf("WriteToken(%q): %w", w, err)
		}
	}
	if err := r.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if err := r.Close(); err != nil {
		return fmt.Errorf("Close: %w", err)
	}

	out := buf.Bytes()
	ansiCount := bytes.Count(out, []byte{0x1b})
	crCount := bytes.Count(out, []byte{0x0d})

	if ansiCount != 0 {
		return fmt.Errorf("plain mode emitted ANSI byte; want 0, got %d", ansiCount)
	}
	if crCount != 0 {
		return fmt.Errorf("plain mode emitted CR byte; want 0, got %d", crCount)
	}
	for _, w := range streamWords {
		// w has trailing space; the bare word itself must appear.
		bare := strings.TrimSpace(w)
		if !strings.Contains(string(out), bare) {
			return fmt.Errorf("plain output missing word %q", bare)
		}
	}

	fmt.Printf("    phaseB: bytes=%d; ANSI-count=%d; CR-count=%d\n",
		len(out), ansiCount, crCount)
	fmt.Printf("    verdict: zero ANSI / zero CR; all 10 words present in transcript\n")
	return nil
}

// phaseC is the load-bearing dirty-region diff proof. After rendering the
// initial 3-line frame, the second render with exactly one line changed MUST
// produce strictly fewer bytes than the first (delta < firstLen) and MUST
// contain exactly one cursor-up sequence \x1b[<n>A. A regression that
// re-rendered the whole frame would emit roughly the same bytes again
// (delta ~ firstLen) and would contain three cursor-up sequences (one per
// line) or none (full re-render). Either failure mode trips this phase.
func phaseC() error {
	fmt.Println("==> phase C: DIRTY-REGION-DIFF (always runs)")

	var buf bytes.Buffer
	r := render.NewANSIRenderer(&buf)

	const block1 = "alpha-line-22-chars-here\nbeta-line-22-chars-here\ngamma-line-22-chars-here"
	const block2 = "alpha-line-22-chars-here\nXXX-line-22-chars-here\ngamma-line-22-chars-here"

	if err := render.RenderTextBlock(r, "blk", block1); err != nil {
		return fmt.Errorf("RenderTextBlock first: %w", err)
	}
	firstLen := buf.Len()

	if err := render.RenderTextBlock(r, "blk", block2); err != nil {
		return fmt.Errorf("RenderTextBlock second: %w", err)
	}
	totalLen := buf.Len()
	delta := totalLen - firstLen

	if delta <= 0 {
		return fmt.Errorf("second render produced no bytes; delta=%d", delta)
	}
	if delta >= firstLen {
		return fmt.Errorf("dirty-region diff invariant broken: delta=%d >= firstLen=%d "+
			"(second render should have emitted only the changed line)", delta, firstLen)
	}

	// Exactly one cursor-up sequence in the delta. We use a tight regex
	// that matches \x1b[<digits>A.
	deltaBytes := buf.Bytes()[firstLen:]
	cursorUpRe := regexp.MustCompile(`\x1b\[\d+A`)
	cursorUpMatches := cursorUpRe.FindAll(deltaBytes, -1)
	if len(cursorUpMatches) != 1 {
		return fmt.Errorf("expected exactly 1 cursor-up sequence in delta; got %d (delta=%q)",
			len(cursorUpMatches), string(deltaBytes))
	}

	fmt.Printf("    phaseC: firstLen=%d delta=%d (delta<firstLen=%t); cursor-up-count=%d\n",
		firstLen, delta, delta < firstLen, len(cursorUpMatches))
	fmt.Printf("    verdict: only the changed line was emitted; in-place rewrite confirmed\n")
	return nil
}

// phaseD exercises the factory's auto-detect ladder. A bytes.Buffer is by
// definition not a TTY, so with HELIXCODE_RENDER unset the factory MUST
// resolve to ModePlain. The renderer constructed against the buffer MUST
// then emit zero ANSI and zero CR for a normal token stream.
func phaseD() error {
	fmt.Println("==> phase D: TTY-FALLBACK (always runs)")

	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{
		Writer:    &buf,
		EnvLookup: func(string) string { return "" },
	})
	if err != nil {
		return fmt.Errorf("NewRenderer: %w", err)
	}
	if r.Mode() != render.ModePlain {
		return fmt.Errorf("auto-detect mode mismatch: got %q want %q",
			r.Mode(), render.ModePlain)
	}

	if err := r.Begin("d"); err != nil {
		return fmt.Errorf("Begin: %w", err)
	}
	if err := r.WriteToken("hello world\n"); err != nil {
		return fmt.Errorf("WriteToken: %w", err)
	}
	if err := r.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if err := r.Close(); err != nil {
		return fmt.Errorf("Close: %w", err)
	}

	out := buf.Bytes()
	ansiCount := bytes.Count(out, []byte{0x1b})
	crCount := bytes.Count(out, []byte{0x0d})
	if ansiCount != 0 || crCount != 0 {
		return fmt.Errorf("plain mode invariants violated: ANSI=%d CR=%d", ansiCount, crCount)
	}

	fmt.Printf("    phaseD: mode=%s; auto-detect-on-buffer-correctly-picked-plain\n", r.Mode())
	fmt.Printf("    verdict: factory ladder resolved bytes.Buffer to ModePlain; no escapes leaked\n")
	return nil
}

// phaseE is the gated REAL-TTY phase. When stdout is a real terminal,
// construct a factory pointed at os.Stdout with the env unset and assert
// ModeFancy. Otherwise SKIP-OK with explicit reason - non-TTY environments
// (CI consoles, logged runs, redirected output) cannot exercise this path
// and a forced-pass would be a bluff.
func phaseE() error {
	fmt.Println("==> phase E: REAL-TTY (gated)")

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println("    [skipped: stdout is not a TTY]")
		fmt.Println("    SKIP-OK: real-TTY assertions only meaningful when run under an interactive terminal")
		return nil
	}

	r, err := render.NewRenderer(render.FactoryOptions{
		Writer:    os.Stdout,
		EnvLookup: func(string) string { return "" },
	})
	if err != nil {
		return fmt.Errorf("NewRenderer: %w", err)
	}
	if r.Mode() != render.ModeFancy {
		return fmt.Errorf("real-TTY mode mismatch: got %q want %q", r.Mode(), render.ModeFancy)
	}

	if err := render.RenderTextBlock(r, "phaseE-blk", "real-tty-line-1\nreal-tty-line-2"); err != nil {
		return fmt.Errorf("RenderTextBlock: %w", err)
	}
	if err := r.Close(); err != nil {
		return fmt.Errorf("Close: %w", err)
	}

	fmt.Println("    phaseE: real-TTY rendered")
	return nil
}
