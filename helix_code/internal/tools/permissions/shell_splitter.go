package permissions

import (
	"bytes"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// SplitCommands extracts every leaf call expression from a shell script,
// recursing into command substitutions ($(...), backticks). Returns one
// entry per leaf command.
//
// Quoted operators ("foo && bar") are NOT split — the parser treats them
// as literal arguments. A malformed input returns a non-nil error so that
// callers can fail-closed.
func SplitCommands(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	parser := syntax.NewParser()
	f, err := parser.Parse(strings.NewReader(input), "")
	if err != nil {
		return nil, err
	}
	var cmds []string
	syntax.Walk(f, func(n syntax.Node) bool {
		if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			cmds = append(cmds, callExprString(call))
		}
		return true
	})
	return cmds, nil
}

// callExprString renders a CallExpr to its source-equivalent string,
// stripping outer quotes/backticks from word parts so wildcard patterns
// match the underlying command text.
//
// Arguments that are purely a command substitution ($(...) or backtick) are
// omitted from the rendered string — they are already emitted as independent
// leaf calls by the Walk in SplitCommands. This ensures the outer call (e.g.
// "echo") is represented without the substitution noise, and the inner call
// (e.g. "rm -rf /tmp/x") appears as its own entry.
func callExprString(call *syntax.CallExpr) string {
	var sb strings.Builder
	written := 0
	for _, arg := range call.Args {
		if wordIsPureCmdSubst(arg) {
			// Skip: already captured as a separate leaf by the walker.
			continue
		}
		if written > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(wordString(arg))
		written++
	}
	return sb.String()
}

// wordIsPureCmdSubst returns true when a Word consists of exactly one part
// that is a CmdSubst — meaning the argument is purely $(…) or `…`.
func wordIsPureCmdSubst(w *syntax.Word) bool {
	if len(w.Parts) != 1 {
		return false
	}
	_, ok := w.Parts[0].(*syntax.CmdSubst)
	return ok
}

// wordString prints a single word; double-quoted parts have their quotes
// stripped (the contents are emitted as-is) so that "git push" inside
// quotes still produces "git push" for matching.
func wordString(w *syntax.Word) string {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				if lit, ok := inner.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		case *syntax.CmdSubst:
			// Substitutions render as their inner text; the leaf walk in
			// SplitCommands has already extracted them as separate calls.
			// Emit the substitution inline so the parent's wildcard match
			// still sees the surrounding tokens correctly.
			sb.WriteString("$(")
			for _, stmt := range p.Stmts {
				if call, ok := stmt.Cmd.(*syntax.CallExpr); ok {
					sb.WriteString(callExprString(call))
				}
			}
			sb.WriteString(")")
		default:
			// Use the syntax printer to render unknown word parts.
			var buf bytes.Buffer
			printer := syntax.NewPrinter()
			_ = printer.Print(&buf, part)
			sb.WriteString(strings.TrimSpace(buf.String()))
		}
	}
	return sb.String()
}
