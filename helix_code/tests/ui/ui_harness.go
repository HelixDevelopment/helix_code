// Package ui is a HelixCode-LOCAL UI harness. The TUI surface (tview/tcell) is
// fully headless-testable via tcell.SimulationScreen: build the REAL tview
// components, render them to an in-memory cell grid, inject keys, and read back the
// EXACT rendered cells. This is genuine pixel-equivalent evidence — tcell actually
// composited the widgets — NOT a HelixQA shell delegation.
//
// HONEST SPLIT (§11.4.6 / §11.4.52): the TUI is fully automatable here. The Fyne
// desktop GUI's widget-tree is automatable via fyne.io/fyne/v2/test, but its
// pixel-/native-window layer is NOT headlessly assertable and is honestly tracked
// operator-attended. Non-introspectable canvas surfaces where neither the widget
// tree nor an OCR/CV oracle is feasible are SKIP-with-reason (§11.4.3) + an
// operator-attended migration item — never a fake PASS.
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RenderedCellsReport is the closed-set rendered_cells.json evidence shape.
type RenderedCellsReport struct {
	Name            string           `json:"name"`
	Width           int              `json:"width"`
	Height          int              `json:"height"`
	AssertedStrings []AssertedString `json:"asserted_strings"`
	Timestamp       string           `json:"timestamp"`
}

// AssertedString records whether an expected string was found in the rendered grid.
type AssertedString struct {
	Text  string `json:"text"`
	Found bool   `json:"found"`
}

var (
	runIDOnce sync.Once
	runIDVal  string
)

func runID() string {
	runIDOnce.Do(func() {
		if v := os.Getenv("UI_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		if v := os.Getenv("STRESSCHAOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		runIDVal = time.Now().UTC().Format("20060102T150405Z")
	})
	return runIDVal
}

// EvidenceRoot resolves qa-results/<run-id>/. Override with UI_EVIDENCE_ROOT.
func EvidenceRoot() string {
	if v := os.Getenv("UI_EVIDENCE_ROOT"); v != "" {
		return filepath.Join(v, runID())
	}
	return filepath.Join(moduleRoot(), "qa-results", runID())
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "qa-results-fallback"
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

func evidenceDir(t testing.TB, name string) string {
	t.Helper()
	dir := filepath.Join(EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("ui: cannot create evidence dir %s: %v", dir, err)
	}
	return dir
}

func writeBytes(t testing.TB, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("ui: write evidence %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ui: evidence artefact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("ui: evidence artefact empty (not evidence per §11.4.5) %s", path)
	}
}

func writeJSON(t testing.TB, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("ui: marshal evidence %s: %v", path, err)
	}
	writeBytes(t, path, b)
}

// Renderer wraps a tview.Application bound to a tcell.SimulationScreen so a test
// can render a real component headlessly, inject keys, and read back the cell grid.
//
// tview's Draw()/ForceDraw() block on the application's internal update queue which
// is ONLY drained by the running event loop (Run). So the Renderer runs the REAL
// event loop in a goroutine and synchronises on draws via SetAfterDrawFunc — this
// is the production rendering path (the same loop main.go's Run() drives), not a
// bypass. Stop() tears the loop down.
type Renderer struct {
	app    *tview.Application
	screen tcell.SimulationScreen
	width  int
	height int

	mu       sync.Mutex
	drawCh   chan struct{}
	started  bool
	runErrCh chan error
}

// NewRenderer builds a Renderer of the given cell size with the REAL tview
// Application driving a tcell.SimulationScreen, and starts its event loop.
func NewRenderer(t testing.TB, width, height int) *Renderer {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("ui: simulation screen init: %v", err)
	}
	sim.SetSize(width, height)
	app := tview.NewApplication().SetScreen(sim)
	r := &Renderer{
		app: app, screen: sim, width: width, height: height,
		drawCh: make(chan struct{}, 16), runErrCh: make(chan error, 1),
	}
	// Signal every completed draw so Render/InjectKey can wait for a fresh frame.
	app.SetAfterDrawFunc(func(screen tcell.Screen) {
		select {
		case r.drawCh <- struct{}{}:
		default:
		}
	})
	t.Cleanup(func() {
		app.Stop()
		// Drain the run error (Run returns nil on Stop).
		select {
		case <-r.runErrCh:
		case <-time.After(2 * time.Second):
		}
	})
	return r
}

// start launches the real event loop once (idempotent).
func (r *Renderer) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true
	go func() { r.runErrCh <- r.app.Run() }()
}

// waitDraw blocks until the next completed frame (bounded), so reads see a fully
// composited grid.
func (r *Renderer) waitDraw(t testing.TB) {
	t.Helper()
	select {
	case <-r.drawCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("ui: timed out waiting for a tview draw — render path may be wedged")
	}
}

// Render sets root, starts the loop on first use, waits for a real composited frame,
// and returns the rendered grid as one string per row.
func (r *Renderer) Render(t testing.TB, root tview.Primitive) []string {
	t.Helper()
	// drain any stale draw signals before this render
	for {
		select {
		case <-r.drawCh:
			continue
		default:
		}
		break
	}
	r.app.SetRoot(root, true)
	r.start()
	// SetRoot on an already-running app triggers a draw; if the loop was just
	// started Run performs the initial draw. Either way wait for the frame.
	r.app.Draw()
	r.waitDraw(t)
	return r.grid()
}

// InjectKey injects a key event into the running screen, waits for the resulting
// re-composited frame, and returns the new grid — so a test can prove an
// interaction changed the render.
func (r *Renderer) InjectKey(t testing.TB, key tcell.Key, ch rune, mod tcell.ModMask) []string {
	t.Helper()
	// drain stale signals so we wait for the POST-key frame
	for {
		select {
		case <-r.drawCh:
			continue
		default:
		}
		break
	}
	r.screen.InjectKey(key, ch, mod)
	r.app.Draw()
	r.waitDraw(t)
	return r.grid()
}

// grid reads the simulation screen contents back into one string per row.
func (r *Renderer) grid() []string {
	cells, w, h := r.screen.GetContents()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Bytes) == 0 {
				b.WriteByte(' ')
				continue
			}
			b.Write(c.Bytes)
		}
		rows[y] = b.String()
	}
	return rows
}

// gridContains reports whether any rendered row contains s.
func gridContains(grid []string, s string) bool {
	for _, row := range grid {
		if strings.Contains(row, s) {
			return true
		}
	}
	return false
}

// AssertRenderedContent FAILS if any expected string is absent from the rendered
// cell grid (a TUI that renders nothing -> empty grid -> FAIL). Writes
// rendered_cells.json. This is genuine pixel-equivalent evidence: tcell composited
// the real widgets.
func AssertRenderedContent(t testing.TB, name string, grid []string, expect []string) RenderedCellsReport {
	t.Helper()
	if len(expect) == 0 {
		t.Fatalf("ui: AssertRenderedContent %q has zero expected strings", name)
	}
	rows := make([]AssertedString, 0, len(expect))
	missing := make([]string, 0)
	for _, e := range expect {
		found := gridContains(grid, e)
		rows = append(rows, AssertedString{Text: e, Found: found})
		if !found {
			missing = append(missing, e)
		}
	}

	width := 0
	if len(grid) > 0 {
		width = len([]rune(grid[0]))
	}
	rep := RenderedCellsReport{
		Name:            name,
		Width:           width,
		Height:          len(grid),
		AssertedStrings: rows,
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "rendered_cells.json")
	writeJSON(t, path, rep)

	if len(missing) > 0 {
		t.Fatalf("ui: %q expected strings NOT in rendered cells: %v — the real TUI did not composite them (evidence: %s)",
			name, missing, path)
	}
	t.Logf("ui: %q rendered %dx%d, all %d expected strings present -> %s", name, width, rep.Height, len(rows), path)
	return rep
}

// AssertNoLeakedMessageID FAILS if any raw message ID appears in the rendered grid
// (a leaked ID in a TUI cell is a CONST-046 UX defect).
func AssertNoLeakedMessageID(t testing.TB, name string, grid []string, messageIDs []string) {
	t.Helper()
	for _, id := range messageIDs {
		if gridContains(grid, id) {
			t.Fatalf("ui: %q LEAKED raw message ID %q in a rendered cell — CONST-046 UX defect", name, id)
		}
	}
}

// AssertInteractionChangedRender FAILS if before and after grids are identical — an
// injected interaction that does not change the render means the input path is dead
// / stubbed.
func AssertInteractionChangedRender(t testing.TB, name string, before, after []string) {
	t.Helper()
	if strings.Join(before, "\n") == strings.Join(after, "\n") {
		t.Fatalf("ui: %q interaction did NOT change the rendered state — input path is dead/stubbed", name)
	}
}

// --- Real tview component builders (mirror applications/terminal_ui patterns) ---
// The production terminal_ui components live in `package main` (not importable), so
// these build the SAME tview widgets via the SAME tview API the production
// CreateList/CreateTable use (components.go: tview.NewList/.NewTable/.SetCell). The
// unit under test is tview's real rendering of these real widgets.

// BuildList builds a real tview.List with the given items + title (mirrors
// terminal_ui CreateList).
func BuildList(title string, items []string) *tview.List {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(title)
	for i, it := range items {
		shortcut := rune('a' + i)
		list.AddItem(it, "", shortcut, nil)
	}
	return list
}

// BuildTable builds a real tview.Table with headers + rows (mirrors terminal_ui
// CreateTable).
func BuildTable(headers []string, data [][]string) *tview.Table {
	table := tview.NewTable()
	table.SetBorder(true)
	for col, h := range headers {
		table.SetCell(0, col, tview.NewTableCell(h).SetSelectable(false))
	}
	for row, rd := range data {
		for col, cell := range rd {
			table.SetCell(row+1, col, tview.NewTableCell(cell))
		}
	}
	table.SetSelectable(true, false)
	return table
}
