package editor

import (
	"bufio"
	"io"
	"os"
)

// readLinesPreservingNewline reads a file into a slice of lines WITHOUT the
// 64KiB-per-line cap of bufio.Scanner's default token buffer, and reports
// whether the file ended with a trailing newline.
//
// Two defects this fixes vs. the previous bufio.Scanner(file)+Scan() pattern:
//
//	DEFECT-A: bufio.Scanner with its default token buffer fails with
//	"bufio.Scanner: token too long" on any line longer than 64KiB (minified
//	JS/CSS, large JSON, lock files, generated code). bufio.Reader.ReadString
//	has no per-line cap (it grows as needed), so arbitrarily long lines load.
//
//	DEFECT-B: bufio.Scanner discards trailing-newline state, so a writer that
//	appends "\n" after every line silently adds a trailing newline to files
//	that ended without one. We capture endsWithNewline here so the writer can
//	preserve the original state exactly.
//
// Each returned line has its line terminator stripped (matching the previous
// scanner.Text() semantics). A trailing empty final element is NOT produced for
// a newline-terminated file (again matching scanner semantics), so the line
// count is identical to the old scanner path for both newline-terminated and
// non-terminated inputs. For an empty file, lines is empty and endsWithNewline
// is false.
func readLinesPreservingNewline(filePath string) (lines []string, endsWithNewline bool, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		segment, readErr := reader.ReadString('\n')
		if len(segment) > 0 {
			if segment[len(segment)-1] == '\n' {
				endsWithNewline = true
				// Strip the trailing '\n' (and a preceding '\r' if present)
				// to match scanner.Text() which drops the line terminator.
				segment = segment[:len(segment)-1]
				if len(segment) > 0 && segment[len(segment)-1] == '\r' {
					segment = segment[:len(segment)-1]
				}
				lines = append(lines, segment)
			} else {
				// Final segment with no trailing newline.
				endsWithNewline = false
				lines = append(lines, segment)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, false, readErr
		}
	}

	return lines, endsWithNewline, nil
}

// writeLinesPreservingNewline writes lines to a file, joining them with "\n"
// and emitting a trailing newline only if endsWithNewline is true — preserving
// the original file's trailing-newline state (DEFECT-B) and never corrupting
// bytes the caller did not intend to change.
func writeLinesPreservingNewline(filePath string, lines []string, endsWithNewline bool) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, line := range lines {
		if _, err := writer.WriteString(line); err != nil {
			return err
		}
		// Emit a newline after every line except the last; for the last line
		// emit one only if the original file ended with a newline.
		if i < len(lines)-1 || endsWithNewline {
			if err := writer.WriteByte('\n'); err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}
