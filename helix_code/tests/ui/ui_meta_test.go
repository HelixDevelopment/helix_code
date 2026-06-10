package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// §1.1 paired-mutation meta-tests prove the ui harness cannot bluff: each plants a
// rendering/interaction defect and asserts the harness DETECTS it (detection path
// is t.Fatalf, captured via failTB).

type failTB struct {
	testing.TB
	mu     sync.Mutex
	failed bool
	msg    string
}

func (f *failTB) Helper() {}
func (f *failTB) Fatalf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
	panic(sentinelFatal{})
}
func (f *failTB) Errorf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
}
func (f *failTB) Logf(format string, args ...interface{}) {}

type sentinelFatal struct{}

func runWithFailTB(body func(tb testing.TB)) (failed bool, msg string) {
	f := &failTB{TB: &testing.T{}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(sentinelFatal); !ok {
					panic(r)
				}
			}
		}()
		body(f)
	}()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failed, f.msg
}

func isolatedEvidence(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	old := os.Getenv("UI_EVIDENCE_ROOT")
	os.Setenv("UI_EVIDENCE_ROOT", tmp)
	t.Cleanup(func() { os.Setenv("UI_EVIDENCE_ROOT", old) })
}

// TestMeta_AssertRenderedContent_DetectsBlankRender plants a blank component (an
// empty tview.Box renders nothing but whitespace) and asserts the rendered-content
// assertion FAILS when the expected text is absent — an empty cell grid is not a
// PASS.
func TestMeta_AssertRenderedContent_DetectsBlankRender(t *testing.T) {
	isolatedEvidence(t)
	r := NewRenderer(t, 40, 10)
	blank := tview.NewBox() // no border, no title, no text -> renders blank
	grid := r.Render(t, blank)

	failed, _ := runWithFailTB(func(tb testing.TB) {
		AssertRenderedContent(tb, "meta-blank", grid, []string{"Expected Menu Item"})
	})
	if !failed {
		t.Fatal("meta: AssertRenderedContent did NOT detect a blank render (expected text absent) — harness is a bluff")
	}
}

// TestMeta_AssertInteractionChangedRender_DetectsDeadKeyHandler plants a component
// that ignores keys (a static tview.TextView re-renders identically) and asserts
// the interaction assertion detects the UNCHANGED screen-state and FAILS.
func TestMeta_AssertInteractionChangedRender_DetectsDeadKeyHandler(t *testing.T) {
	isolatedEvidence(t)
	r := NewRenderer(t, 40, 10)
	// A static TextView does not respond to navigation keys: before == after.
	static := tview.NewTextView().SetText("static content")
	before := r.Render(t, static)
	after := r.InjectKey(t, tcell.KeyDown, 0, tcell.ModNone) // ignored by a static view

	failed, _ := runWithFailTB(func(tb testing.TB) {
		AssertInteractionChangedRender(tb, "meta-dead-key", before, after)
	})
	if !failed {
		t.Fatal("meta: AssertInteractionChangedRender did NOT detect the dead key-handler (unchanged render) — harness is a bluff")
	}
}

// TestMeta_AssertNoLeakedMessageID_DetectsLeakedID renders a list whose label IS a
// raw message ID (the planted leak) and asserts the no-leak assertion catches it.
func TestMeta_AssertNoLeakedMessageID_DetectsLeakedID(t *testing.T) {
	isolatedEvidence(t)
	r := NewRenderer(t, 60, 12)
	// Planted defect: a raw message ID rendered straight into a cell.
	leaky := BuildList("Menu", []string{"internal_server_qa_engine_disabled"})
	grid := r.Render(t, leaky)

	failed, _ := runWithFailTB(func(tb testing.TB) {
		AssertNoLeakedMessageID(tb, "meta-leak", grid, []string{"internal_server_qa_engine_disabled"})
	})
	if !failed {
		t.Fatal("meta: AssertNoLeakedMessageID did NOT detect the raw message ID in a cell — harness is a bluff")
	}
}

// TestMeta_PositivePathWritesEvidence proves a real render writes a non-empty
// rendered_cells.json with found-string verdicts.
func TestMeta_PositivePathWritesEvidence(t *testing.T) {
	isolatedEvidence(t)
	r := NewRenderer(t, 60, 12)
	list := BuildList("Menu", []string{"Real Item One", "Real Item Two"})
	grid := r.Render(t, list)
	AssertRenderedContent(t, "meta-positive", grid, []string{"Real Item One", "Real Item Two"})

	path := filepath.Join(EvidenceRoot(), "meta-positive", "rendered_cells.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("meta: rendered_cells.json not written: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("meta: rendered_cells.json is empty — would be a hollow PASS")
	}
}
