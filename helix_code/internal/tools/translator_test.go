// Unit tests for the internal/tools package-level translator + tr()
// helper (CONST-046 round-181 §11.4 anti-bluff sweep, 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package tools

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	toolsi18n "dev.helix.code/internal/tools/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_tools_fs_read_description", nil)
	if got == "internal_tools_fs_read_description" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_fs_write_description", nil)
	if got != "<TR:internal_tools_fs_write_description>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade to
	// the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_glob_description", nil)
	if got != "internal_tools_glob_description" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_grep_description", nil)
	if got == "internal_tools_grep_description" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(toolsi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_tools_browser_click_description", nil)
	if got != "internal_tools_browser_click_description" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestFSReadTool_Description_GoesThroughTranslator covers the
// FSReadTool.Description() call site. With a sentinel translator wired,
// the description MUST surface the sentinel-wrapped message ID —
// proving the literal was NOT hardcoded on the path.
func TestFSReadTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &FSReadTool{}
	got := tool.Description()
	want := "<TR:internal_tools_fs_read_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("FSReadTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestFSWriteTool_Description_GoesThroughTranslator covers the
// FSWriteTool.Description() call site.
func TestFSWriteTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &FSWriteTool{}
	got := tool.Description()
	want := "<TR:internal_tools_fs_write_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("FSWriteTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestFSEditTool_Description_GoesThroughTranslator covers the
// FSEditTool.Description() call site.
func TestFSEditTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &FSEditTool{}
	got := tool.Description()
	want := "<TR:internal_tools_fs_edit_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("FSEditTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestGlobTool_Description_GoesThroughTranslator covers the
// GlobTool.Description() call site.
func TestGlobTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &GlobTool{}
	got := tool.Description()
	want := "<TR:internal_tools_glob_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("GlobTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestGrepTool_Description_GoesThroughTranslator covers the
// GrepTool.Description() call site.
func TestGrepTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &GrepTool{}
	got := tool.Description()
	want := "<TR:internal_tools_grep_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("GrepTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestBrowserClickTool_Description_GoesThroughTranslator covers the
// BrowserClickTool.Description() call site.
func TestBrowserClickTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &BrowserClickTool{}
	got := tool.Description()
	want := "<TR:internal_tools_browser_click_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("BrowserClickTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestBrowserTypeTool_Description_GoesThroughTranslator covers the
// BrowserTypeTool.Description() call site.
func TestBrowserTypeTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &BrowserTypeTool{}
	got := tool.Description()
	want := "<TR:internal_tools_browser_type_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("BrowserTypeTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestBrowserCloseToolV2_Description_GoesThroughTranslator covers
// the BrowserCloseToolV2.Description() call site.
func TestBrowserCloseToolV2_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &BrowserCloseToolV2{}
	got := tool.Description()
	want := "<TR:internal_tools_browser_close_v2_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("BrowserCloseToolV2.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestBrowserScreenshotToolV2_Description_GoesThroughTranslator
// covers the BrowserScreenshotToolV2.Description() call site.
func TestBrowserScreenshotToolV2_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &BrowserScreenshotToolV2{}
	got := tool.Description()
	want := "<TR:internal_tools_browser_screenshot_v2_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("BrowserScreenshotToolV2.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestTaskStopTool_Description_GoesThroughTranslator covers the
// TaskStopTool.Description() call site.
func TestTaskStopTool_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &TaskStopTool{}
	got := tool.Description()
	want := "<TR:internal_tools_task_stop_description>"
	if !strings.Contains(got, want) {
		t.Fatalf("TaskStopTool.Description = %q, want contain %q — call site bypassed tr()", got, want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), every migrated call site emits the bundle message
// ID — confirming the migration didn't accidentally pass an empty
// string (which would render to nothing in production).
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	ids := []struct {
		name string
		got  string
		want string
	}{
		{"FSReadTool", (&FSReadTool{}).Description(), "Read file contents from the filesystem"},
		{"FSWriteTool", (&FSWriteTool{}).Description(), "Write content to a file"},
		{"FSEditTool", (&FSEditTool{}).Description(), "Edit file contents with structured operations"},
		{"GlobTool", (&GlobTool{}).Description(), "Find files matching a glob pattern"},
		{"GrepTool", (&GrepTool{}).Description(), "Search file contents for a pattern"},
		{"BrowserClickTool", (&BrowserClickTool{}).Description(), "Click an element by CSS selector."},
		{"BrowserTypeTool", (&BrowserTypeTool{}).Description(), "Type text into an input/textarea/contenteditable element by selector."},
		{"BrowserCloseToolV2", (&BrowserCloseToolV2{}).Description(), "Close the active browser session and terminate the chromium subprocess."},
		{"BrowserScreenshotToolV2", (&BrowserScreenshotToolV2{}).Description(), "Capture a screenshot of the current page as a PNG file."},
		{"TaskStopTool", (&TaskStopTool{}).Description(), "Cancel a running background task by ID."},
	}
	for _, c := range ids {
		if c.got != c.want {
			t.Errorf("%s default Description = %q, want %q (should resolve to bundle prose)", c.name, c.got, c.want)
		}
	}
}
