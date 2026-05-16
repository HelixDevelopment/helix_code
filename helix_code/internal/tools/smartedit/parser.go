package smartedit

import (
	"bufio"
	"fmt"
	"strings"
)

// Parse converts a SEARCH/REPLACE prompt into an EditPlan. It is the entry
// point for stage 1 of the smart-edit pipeline (T03).
//
// Format:
//
//	relative/path/to/file.go
//	<<<<<<< SEARCH
//	old content
//	=======
//	new content
//	>>>>>>> REPLACE
//
// Path-stickiness: a non-blank, non-marker line that appears IMMEDIATELY
// before a `<<<<<<< SEARCH` marker is the path for that block. The path
// then sticks for any subsequent block until a NEW path line appears. The
// first block in the prompt MUST have a preceding path line — there is no
// default path.
//
// Marker matching is strict-column-0: only a line whose first byte is `<`
// (or `=` / `>`) and whose `TrimRight(" \t")` form equals the literal
// marker constant from types.go is treated as a marker. Lines with any
// leading whitespace are CONTENT — this is what allows a SEARCH section
// to legally contain text that looks like a marker (the indented-marker
// edge case enforced by TestParse_IndentedMarkerNotTreatedAsMarker).
//
// Errors are sentinel-only (declared in types.go) and carry context via
// fmt.Errorf wrapping where useful. On any error Parse returns (nil, err);
// partial plans are NEVER returned. This is the whole-prompt atomicity
// gate: if any block in the prompt is malformed, the entire prompt is
// rejected and the applier never sees a half-parsed plan.
func Parse(prompt string) (*EditPlan, error) {
	if len(prompt) > MaxPromptBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrPromptTooLarge, len(prompt))
	}

	lines, err := splitLines(prompt)
	if err != nil {
		return nil, err
	}

	plan, err := parseLines(lines)
	if err != nil {
		return nil, err
	}
	plan.SourceBytes = len(prompt)
	return plan, nil
}

// splitLines uses bufio.Scanner with a buffer sized for MaxPromptBytes so
// the parser can handle prompts at the upper size limit without panicking.
// The scanner strips the trailing `\n` from each line; we re-attach it
// when reconstructing SEARCH / REPLACE bodies inside parseLines.
func splitLines(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	// MaxScanTokenSize default is 64 KiB; a single SEARCH/REPLACE section
	// is capped at MaxSearchBytes / MaxReplaceBytes (both 64 KiB), but a
	// path or content line is always smaller. Provide a generous buffer
	// to keep the scanner happy on long-but-legal inputs.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, MaxPromptBytes+1)
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan prompt: %w", err)
	}
	return out, nil
}

// isMarkerLine reports whether `line` is exactly `marker` after trimming
// trailing spaces and tabs. Leading whitespace disqualifies a line — a
// marker MUST start at column 0 to be honoured. This is the column-0
// rule from spec §11.5.
func isMarkerLine(line, marker string) bool {
	if len(line) == 0 {
		return false
	}
	// Reject any leading whitespace: the marker MUST be column-0 literal.
	if line[0] == ' ' || line[0] == '\t' {
		return false
	}
	return strings.TrimRight(line, " \t") == marker
}

// parseState enumerates the four positions the line-by-line state machine
// can occupy while consuming the prompt.
type parseState int

const (
	stateSeekPath     parseState = iota // before any block, or between blocks; expecting path or SEARCH
	stateInSearch                       // inside SEARCH section, before divider
	stateInReplace                      // inside REPLACE section, before REPLACE marker
	stateAfterReplace                   // after a REPLACE marker; expecting next path or SEARCH (sticky)
)

// parseLines is the heart of Parse. It walks the line slice with a small
// state machine, accumulating SEARCH and REPLACE bodies and emitting an
// EditBlock each time it observes a complete `<<<<<<< SEARCH … =======
// … >>>>>>> REPLACE` triplet.
//
// Path-stickiness is implemented by retaining the most recently observed
// path line in `currentPath`. A line is recorded as a path candidate only
// when it is non-blank and not a marker AND the parser is in
// stateSeekPath / stateAfterReplace (i.e. between blocks). Inside SEARCH
// or REPLACE, every non-marker line is content.
func parseLines(lines []string) (*EditPlan, error) {
	plan := &EditPlan{
		Blocks:  nil,
		PerFile: make(map[string][]EditBlock),
	}

	var (
		state         = stateSeekPath
		currentPath   string
		pendingPath   string // most recent non-blank candidate path line (cleared on each block start)
		searchBuf     strings.Builder
		replaceBuf    strings.Builder
		blockLineStrt int
	)

	for i, line := range lines {
		lineNum := i + 1 // 1-indexed for LineStart / LineEnd

		switch state {
		case stateSeekPath, stateAfterReplace:
			switch {
			case isMarkerLine(line, MarkerSearch):
				// Resolve the path for the upcoming block.
				path := pendingPath
				if path == "" {
					path = currentPath
				}
				if path == "" {
					return nil, fmt.Errorf("%w: line %d", ErrPathRequired, lineNum)
				}
				currentPath = path
				pendingPath = ""
				blockLineStrt = lineNum
				searchBuf.Reset()
				replaceBuf.Reset()
				state = stateInSearch
			case isMarkerLine(line, MarkerDivider):
				return nil, fmt.Errorf("%w: stray divider at line %d", ErrInvalidBlockStructure, lineNum)
			case isMarkerLine(line, MarkerReplace):
				return nil, fmt.Errorf("%w: stray REPLACE marker at line %d", ErrInvalidBlockStructure, lineNum)
			case strings.TrimSpace(line) == "":
				// Blank line between blocks — ignore. Any pendingPath
				// observed before this blank line is preserved (path
				// lines may be separated from the SEARCH marker only
				// by the optional path line itself; per spec the path
				// must be the line IMMEDIATELY preceding SEARCH, but
				// we are lenient and allow blanks between them).
				// To stay strictly compliant with "immediately
				// preceding", clear pendingPath here.
				pendingPath = ""
			default:
				// Non-blank, non-marker line between blocks: candidate path.
				pendingPath = line
			}

		case stateInSearch:
			switch {
			case isMarkerLine(line, MarkerSearch):
				return nil, fmt.Errorf("%w: nested SEARCH marker at line %d", ErrInvalidBlockStructure, lineNum)
			case isMarkerLine(line, MarkerReplace):
				return nil, fmt.Errorf("%w: REPLACE before divider at line %d", ErrInvalidBlockStructure, lineNum)
			case isMarkerLine(line, MarkerDivider):
				if searchBuf.Len() == 0 {
					return nil, fmt.Errorf("%w: line %d", ErrSearchEmpty, lineNum)
				}
				if searchBuf.Len() > MaxSearchBytes {
					return nil, fmt.Errorf("%w: %d bytes at line %d", ErrSearchTooLarge, searchBuf.Len(), lineNum)
				}
				state = stateInReplace
			default:
				searchBuf.WriteString(line)
				searchBuf.WriteByte('\n')
				if searchBuf.Len() > MaxSearchBytes {
					return nil, fmt.Errorf("%w: %d bytes at line %d", ErrSearchTooLarge, searchBuf.Len(), lineNum)
				}
			}

		case stateInReplace:
			switch {
			case isMarkerLine(line, MarkerSearch):
				return nil, fmt.Errorf("%w: nested SEARCH marker in REPLACE at line %d", ErrInvalidBlockStructure, lineNum)
			case isMarkerLine(line, MarkerDivider):
				return nil, fmt.Errorf("%w: duplicate divider at line %d", ErrInvalidBlockStructure, lineNum)
			case isMarkerLine(line, MarkerReplace):
				if replaceBuf.Len() > MaxReplaceBytes {
					return nil, fmt.Errorf("%w: %d bytes at line %d", ErrReplaceTooLarge, replaceBuf.Len(), lineNum)
				}
				blk := EditBlock{
					Path:      currentPath,
					Search:    searchBuf.String(),
					Replace:   replaceBuf.String(),
					LineStart: blockLineStrt,
					LineEnd:   lineNum,
				}
				plan.Blocks = append(plan.Blocks, blk)
				plan.PerFile[currentPath] = append(plan.PerFile[currentPath], blk)
				if len(plan.Blocks) > MaxBlocksPerPrompt {
					return nil, fmt.Errorf("%w: %d blocks", ErrTooManyBlocks, len(plan.Blocks))
				}
				searchBuf.Reset()
				replaceBuf.Reset()
				state = stateAfterReplace
			default:
				replaceBuf.WriteString(line)
				replaceBuf.WriteByte('\n')
				if replaceBuf.Len() > MaxReplaceBytes {
					return nil, fmt.Errorf("%w: %d bytes at line %d", ErrReplaceTooLarge, replaceBuf.Len(), lineNum)
				}
			}
		}
	}

	// EOF reached. The only legal terminal states are stateSeekPath
	// (no blocks yet) or stateAfterReplace (last block closed cleanly).
	switch state {
	case stateInSearch, stateInReplace:
		return nil, fmt.Errorf("%w: prompt ended mid-block", ErrInvalidBlockStructure)
	}

	return plan, nil
}
