package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the editor package.
//
// Chaos classes exercised against the REAL editors (no fakes — real files in
// t.TempDir(), real *CodeEditor / *DiffEditor / *WholeEditor / *LineEditor /
// *SearchReplaceEditor):
//
//   - input-corruption: structurally hostile edit content — malformed unified
//     diffs, invalid regex search/replace, off-by-one and out-of-range line
//     edits, overlapping line edits, binary / invalid-UTF-8 / huge content,
//     wrong-typed Content. Apply/Validate MUST reject cleanly without crashing
//     — a panic on malformed input is a §11.4.85(B) Fatal.
//   - state-corruption under contention: a single *CodeEditor is concurrently
//     SetFormat / SetValidator / ApplyEdit / ValidateEdit'd from many
//     goroutines mid-flight. The RWMutex must serialise so the editor never
//     panics or races and stays usable afterwards.
//   - process-death: a long apply loop honours a cancellable context and must
//     unwind cleanly without leaking a goroutine.
//   - resource-pressure: applying edits proceeds under bounded memory pressure
//     without OOM-crash.

// TestEditor_Chaos_CorruptDiffContent feeds structurally hostile unified-diff
// content to the REAL DiffEditor via the CodeEditor. A malformed diff must be
// rejected with an error (Degraded) or normalised — never panic.
func TestEditor_Chaos_CorruptDiffContent(t *testing.T) {
	dir := t.TempDir()
	ce, err := NewCodeEditor(EditFormatDiff)
	if err != nil {
		t.Fatalf("new editor: %v", err)
	}

	corrupt := [][]byte{
		[]byte("@@ -notanumber,3 +1,4 @@\n+x"),                       // 0: non-numeric hunk range
		[]byte("@@ -1 +1 @@\n-line that does not match the file\n+y"), // 1: context/delete mismatch
		[]byte("@@ malformed header without proper ranges @@\n+z"),    // 2: bad header
		[]byte("\x00\x01\x02\xff\xfe not a diff at all \x00"),         // 3: binary garbage
		[]byte("@@ -1,2 +1,2 @@\n garbageonlyline-no-prefix"),         // 4: line w/o +/-/space prefix
		[]byte("@@ -1, +1, @@\n+w"),                                   // 5: empty count fields
		[]byte(strings.Repeat("@@ -1,1 +1,1 @@\n+spam\n", 5000)),      // 6: huge diff
		[]byte("@@ -1,1 +1,1 @@\n+\xc3\x28 invalid utf8"),             // 7: invalid UTF-8 in content
	}

	// Each diff applies against a freshly-seeded real file so the real read path runs.
	stresschaos.ChaosCorruptInputDuring(t, "editor_corrupt_diff_content", corrupt,
		func(input []byte) error {
			path := filepath.Join(dir, fmt.Sprintf("diff_%d.txt", len(input)%97))
			if err := os.WriteFile(path, []byte("line that does not match the file is absent\nbody\n"), 0o644); err != nil {
				return fmt.Errorf("seed: %w", err)
			}
			return ce.ApplyEdit(Edit{FilePath: path, Format: EditFormatDiff, Content: string(input)})
		})
}

// TestEditor_Chaos_HostileEditDefinitions feeds hostile, wrong-typed, and
// out-of-range edit definitions to the REAL validation + apply path. Each must
// be rejected cleanly (error, not panic). A crash on malformed input is Fatal.
func TestEditor_Chaos_HostileEditDefinitions(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "editor_hostile_edit_definitions", "input-corruption")
	dir := t.TempDir()
	ce, err := NewCodeEditor(EditFormatWhole)
	if err != nil {
		t.Fatalf("new editor: %v", err)
	}

	seed := func(name string) string {
		p := filepath.Join(dir, name)
		if werr := os.WriteFile(p, []byte("l1\nl2\nl3\n"), 0o644); werr != nil {
			t.Fatalf("seed %s: %v", name, werr)
		}
		return p
	}

	type hostile struct {
		desc string
		edit Edit
	}
	bad := []hostile{
		{"empty file path", Edit{FilePath: "", Format: EditFormatWhole, Content: "x"}},
		{"nil content", Edit{FilePath: seed("a.txt"), Format: EditFormatWhole, Content: nil}},
		{"bogus format", Edit{FilePath: seed("b.txt"), Format: EditFormat("bogus"), Content: "x"}},
		{"whole wants string got int", Edit{FilePath: seed("c.txt"), Format: EditFormatWhole, Content: 42}},
		{"diff wants string got slice", Edit{FilePath: seed("d.txt"), Format: EditFormatDiff, Content: []int{1}}},
		{"search/replace wrong type", Edit{FilePath: seed("e.txt"), Format: EditFormatSearchReplace, Content: "notaslice"}},
		{"lines wrong type", Edit{FilePath: seed("f.txt"), Format: EditFormatLines, Content: "notaslice"}},
		// Out-of-range / off-by-one line edits (valid type, invalid values).
		{"line start zero", Edit{FilePath: seed("g.txt"), Format: EditFormatLines, Content: []LineEdit{{StartLine: 0, EndLine: 0, NewContent: "x"}}}},
		{"line end before start", Edit{FilePath: seed("h.txt"), Format: EditFormatLines, Content: []LineEdit{{StartLine: 2, EndLine: 1, NewContent: "x"}}}},
		{"line start way past eof", Edit{FilePath: seed("i.txt"), Format: EditFormatLines, Content: []LineEdit{{StartLine: 9999, EndLine: 9999, NewContent: "x"}}}},
		{"overlapping line edits", Edit{FilePath: seed("j.txt"), Format: EditFormatLines, Content: []LineEdit{{StartLine: 1, EndLine: 2, NewContent: "a"}, {StartLine: 2, EndLine: 3, NewContent: "b"}}}},
		{"empty line-edit slice", Edit{FilePath: seed("k.txt"), Format: EditFormatLines, Content: []LineEdit{}}},
		{"empty search string", Edit{FilePath: seed("l.txt"), Format: EditFormatSearchReplace, Content: []SearchReplace{{Search: "", Replace: "x", Count: -1}}}},
		{"invalid regex search", Edit{FilePath: seed("m.txt"), Format: EditFormatSearchReplace, Content: []SearchReplace{{Search: "[unterminated(", Replace: "x", Count: -1, Regex: true}}}},
	}

	for i, h := range bad {
		func(idx int, hh hostile) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("bad[%d] %q ApplyEdit panicked: %v", idx, hh.desc, p))
				}
			}()
			if err := ce.ApplyEdit(hh.edit); err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("bad[%d] %q rejected cleanly: %v", idx, hh.desc, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("bad[%d] %q applied without crash", idx, hh.desc))
			}
		}(i, h)
	}

	rec.AssertNoFatal()
	t.Log("editor survived hostile edit-definition injection")
}

// TestEditor_Chaos_CorruptFileContent applies edits over files whose on-disk
// content is structurally hostile (binary, invalid UTF-8, huge, NUL bytes, no
// trailing newline). The real read->edit->write path must survive — never panic.
func TestEditor_Chaos_CorruptFileContent(t *testing.T) {
	dir := t.TempDir()
	rec := stresschaos.NewChaosRecorder(t, "editor_corrupt_file_content", "input-corruption")

	hostileFiles := [][]byte{
		{0x00, 0x01, 0x02, 0xff, 0xfe, 0x00},              // binary w/ NUL
		[]byte("\xc3\x28\xa0\xa1 invalid utf8 sequence"),  // invalid UTF-8
		[]byte("no trailing newline at all"),              // missing terminator
		[]byte(strings.Repeat("x", 1<<20)),                // 1 MiB single line (no newline)
		[]byte("mixed\r\nwindows\r\nline\nendings\n"),     // mixed CRLF/LF
		{},                                                // empty file
	}

	for i, fc := range hostileFiles {
		func(idx int, content []byte) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("file[%d] edit panicked: %v", idx, p))
				}
			}()
			path := filepath.Join(dir, fmt.Sprintf("hostile_%d.bin", idx))
			if err := os.WriteFile(path, content, 0o644); err != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("file[%d] seed failed: %v", idx, err))
				return
			}
			// Literal search/replace over the hostile content.
			sre := NewSearchReplaceEditor()
			err := sre.Apply(Edit{FilePath: path, Format: EditFormatSearchReplace,
				Content: []SearchReplace{{Search: "x", Replace: "y", Count: -1}}})
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("file[%d] search/replace declined: %v", idx, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("file[%d] search/replace survived hostile content", idx))
			}
			// Whole-replace over the hostile content (always valid).
			we := NewWholeEditor()
			if err := we.Apply(Edit{FilePath: path, Format: EditFormatWhole, Content: "clean\n"}); err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("file[%d] whole-replace declined: %v", idx, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("file[%d] whole-replace survived hostile content", idx))
			}
		}(i, fc)
	}

	rec.AssertNoFatal()
	t.Log("editor survived corrupt on-disk file content")
}

// TestEditor_Chaos_ConcurrentChurnSharedEditor hammers the SAME *CodeEditor with
// concurrent SetFormat / SetValidator / GetFormat / ValidateEdit / ApplyEdit
// from many goroutines. The CodeEditor.mu must serialise field mutations so the
// editor never panics or races and stays usable. The harshest state-corruption
// surface: a goroutine swaps the active format/applier while another is mid-apply.
// Run under -race.
func TestEditor_Chaos_ConcurrentChurnSharedEditor(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "editor_concurrent_churn_shared_editor", "state-corruption")
	dir := t.TempDir()
	ce, err := NewCodeEditor(EditFormatWhole)
	if err != nil {
		t.Fatalf("new editor: %v", err)
	}

	formats := []EditFormat{EditFormatWhole, EditFormatDiff, EditFormatSearchReplace, EditFormatLines}
	const goroutines = 12
	const iters = 250
	var wg sync.WaitGroup
	var sets, applies, reads int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				switch (id + it) % 5 {
				case 0:
					_ = ce.SetFormat(formats[(id+it)%len(formats)])
					atomic.AddInt64(&sets, 1)
				case 1:
					ce.SetValidator(NewDefaultValidator())
					atomic.AddInt64(&sets, 1)
				case 2:
					_ = ce.GetFormat()
					_ = ce.ValidateEdit(Edit{FilePath: "z.txt", Format: EditFormatWhole, Content: "v"})
					atomic.AddInt64(&reads, 1)
				default:
					// ApplyEdit with an EXPLICIT format (independent of churned
					// default) editing a disjoint file — proves the editors map
					// dispatch survives concurrent format swaps.
					path := filepath.Join(dir, fmt.Sprintf("churn_%d_%d.txt", id, it))
					_ = ce.ApplyEdit(Edit{FilePath: path, Format: EditFormatWhole, Content: "data\n"})
					atomic.AddInt64(&applies, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn: %d sets, %d applies, %d reads, no panic/race",
		atomic.LoadInt64(&sets), atomic.LoadInt64(&applies), atomic.LoadInt64(&reads)))

	// The editor must still be usable after the churn — a fresh apply must land.
	final := filepath.Join(dir, "final.txt")
	if err := ce.SetFormat(EditFormatWhole); err != nil {
		rec.Record(stresschaos.Degraded, "final SetFormat errored: "+err.Error())
	}
	if err := ce.ApplyEdit(Edit{FilePath: final, Format: EditFormatWhole, Content: "ok\n"}); err != nil {
		rec.Record(stresschaos.Fatal, "editor unusable after churn: "+err.Error())
	} else if got := readBack(t, final); got != "ok\n" {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("post-churn apply corrupted result: %q", got))
	} else {
		rec.Record(stresschaos.Recovered, "editor fully usable after churn — self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("editor churn: sets=%d applies=%d reads=%d", atomic.LoadInt64(&sets), atomic.LoadInt64(&applies), atomic.LoadInt64(&reads))
}

// TestEditor_Chaos_CancelDuringApplyLoop injects a process-death fault: a long
// apply+read loop honours a cancellable context and must unwind cleanly when the
// context is cancelled mid-flight, without leaking the worker goroutine.
func TestEditor_Chaos_CancelDuringApplyLoop(t *testing.T) {
	dir := t.TempDir()

	stresschaos.ChaosKillDuring(t, "editor_cancel_during_apply_loop", 40*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			ce, err := NewCodeEditor(EditFormatWhole)
			if err != nil {
				rec.Record(stresschaos.Fatal, "new editor: "+err.Error())
				return
			}
			iterations := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered, fmt.Sprintf("apply loop observed cancellation after %d iterations", iterations))
					return
				default:
				}
				path := filepath.Join(dir, fmt.Sprintf("loop_%d.txt", iterations%64))
				content := fmt.Sprintf("iter %d\n", iterations)
				if err := ce.ApplyEdit(Edit{FilePath: path, Format: EditFormatWhole, Content: content}); err != nil {
					rec.Record(stresschaos.Degraded, "apply errored mid-loop: "+err.Error())
					return
				}
				// Read back — real work.
				_, _ = os.ReadFile(path)
				iterations++
			}
		})
}

// TestEditor_Chaos_ApplyUnderMemoryPressure asserts applying edits proceeds
// under bounded memory pressure without OOM-crash (§11.4.85(B)(4)).
func TestEditor_Chaos_ApplyUnderMemoryPressure(t *testing.T) {
	dir := t.TempDir()
	ce, err := NewCodeEditor(EditFormatWhole)
	if err != nil {
		t.Fatalf("new editor: %v", err)
	}

	stresschaos.ChaosResourcePressureDuring(t, "editor_apply_under_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 200; i++ {
				path := filepath.Join(dir, fmt.Sprintf("mem_%d.txt", i))
				content := strings.Repeat(fmt.Sprintf("line-%d\n", i), 64)
				if err := ce.ApplyEdit(Edit{FilePath: path, Format: EditFormatWhole, Content: content}); err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("apply %d failed under pressure: %v", i, err))
					return
				}
				if got := readBack(t, path); got != content {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("apply %d produced wrong content under pressure", i))
					return
				}
			}
			rec.Record(stresschaos.Recovered, "applied 200 edits under memory pressure without OOM-crash")
		})
}
