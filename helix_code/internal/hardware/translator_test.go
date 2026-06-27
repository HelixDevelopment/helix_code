// Unit tests for the internal/hardware package-level translator +
// tr() helper (CONST-046 round-158 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package hardware

import (
	"bytes"
	stdctx "context"
	"errors"
	"log"
	"strings"
	"testing"

	hardwarei18n "dev.helix.code/internal/hardware/i18n"
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

// captureLog redirects log output to a buffer so call-site tests can
// assert tr() actually ran on the path.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	old := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	log.SetOutput(buf)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(old)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
	return buf
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_hardware_detection_starting", nil)
	if got == "internal_hardware_detection_starting" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_hardware_detection_completed", nil)
	if got != "<TR:internal_hardware_detection_completed>" {
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

	got := tr(stdctx.Background(), "internal_hardware_cpu_detection_failed", nil)
	if got != "internal_hardware_cpu_detection_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_hardware_detection_starting", nil)
	if got == "internal_hardware_detection_starting" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(hardwarei18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_hardware_memory_fallback_to_estimate", nil)
	if got != "internal_hardware_memory_fallback_to_estimate" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestDetect_EmitsTranslatedLifecycleMessages covers the start/end
// lifecycle log lines. With a sentinel translator wired, both lines
// MUST surface the sentinel-wrapped message IDs — proving the
// literals were NOT hardcoded on the path.
func TestDetect_EmitsTranslatedLifecycleMessages(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	buf := captureLog(t)

	d := NewDetector()
	_, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect returned err = %v", err)
	}

	out := buf.String()
	wantStart := "<TR:internal_hardware_detection_starting>"
	wantEnd := "<TR:internal_hardware_detection_completed>"
	if !strings.Contains(out, wantStart) {
		t.Fatalf("log = %q, want contain %q — start lifecycle bypassed tr()", out, wantStart)
	}
	if !strings.Contains(out, wantEnd) {
		t.Fatalf("log = %q, want contain %q — end lifecycle bypassed tr()", out, wantEnd)
	}
}

// TestParseMemorySize_ParseFailureGoesThroughTranslator covers the
// memory-size-parse warning emitted by Detector.parseMemorySize when
// the regex returns no match. The sentinel translator wired, the
// warning MUST surface the sentinel-wrapped message ID.
func TestParseMemorySize_ParseFailureGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	buf := captureLog(t)

	d := &Detector{info: &HardwareInfo{}}
	got := d.parseMemorySize("absolutely-not-a-memory-size")
	if got != 0 {
		t.Fatalf("parseMemorySize on garbage = %d, want 0", got)
	}

	out := buf.String()
	want := "<TR:internal_hardware_parse_memory_size_failed>"
	if !strings.Contains(out, want) {
		t.Fatalf("log = %q, want contain %q — parse warning bypassed tr()", out, want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), Detect emits the bundle message IDs verbatim —
// confirming the migration didn't accidentally pass an empty string
// or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	buf := captureLog(t)

	d := NewDetector()
	_, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect returned err = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "internal_hardware_detection_starting") {
		t.Fatalf("log = %q, want contain raw start ID (Noop echo)", out)
	}
	if !strings.Contains(out, "internal_hardware_detection_completed") {
		t.Fatalf("log = %q, want contain raw end ID (Noop echo)", out)
	}
}
