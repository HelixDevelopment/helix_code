// Unit tests for the internal/persistence package-level translator +
// tr() helper (CONST-046 round-169 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package persistence

import (
	stdctx "context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	persistencei18n "dev.helix.code/internal/persistence/i18n"
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
	got := tr(stdctx.Background(), "internal_persistence_base_path_create_failed", nil)
	if got != "internal_persistence_base_path_create_failed" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_persistence_save_sessions_failed", nil)
	if got != "<TR:internal_persistence_save_sessions_failed>" {
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

	got := tr(stdctx.Background(), "internal_persistence_load_sessions_failed", nil)
	if got != "internal_persistence_load_sessions_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_persistence_serializer_unknown_format", nil)
	if got != "internal_persistence_serializer_unknown_format" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(persistencei18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_persistence_detect_empty_data", nil)
	if got != "internal_persistence_detect_empty_data" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestValidate_UnknownFormatGoesThroughTranslator covers the unknown
// format path in Validate. With sentinel wired, the error string MUST
// surface the sentinel-wrapped message ID — proving the literal was
// NOT hardcoded on the path.
func TestValidate_UnknownFormatGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	err := Validate([]byte("anything"), Format("nope-not-a-format"))
	if err == nil {
		t.Fatal("Validate(unknown format) returned nil — want error")
	}
	want := "<TR:internal_persistence_serializer_unknown_format>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want contain %q — Validate bypassed tr()", err.Error(), want)
	}
}

// TestValidateBinary_TooShortGoesThroughTranslator covers the
// binary-data-too-short path. With sentinel wired, the error string
// MUST surface the sentinel-wrapped message ID.
func TestValidateBinary_TooShortGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	err := Validate([]byte{0x01}, FormatBinary)
	if err == nil {
		t.Fatal("Validate(short binary) returned nil — want error")
	}
	want := "<TR:internal_persistence_binary_data_too_short>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want contain %q — Validate bypassed tr()", err.Error(), want)
	}
}

// TestDetectFormat_EmptyDataGoesThroughTranslator covers the
// empty-data path in DetectFormat.
func TestDetectFormat_EmptyDataGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	_, err := DetectFormat([]byte{})
	if err == nil {
		t.Fatal("DetectFormat(empty) returned nil — want error")
	}
	want := "<TR:internal_persistence_detect_empty_data>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want contain %q — DetectFormat bypassed tr()", err.Error(), want)
	}
}

// TestNewStore_BadBasePathGoesThroughTranslator exercises the
// base-path-create-failed error wrap by passing a path that cannot
// be created (a regular file's child).
func TestNewStore_BadBasePathGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tmp := t.TempDir()
	// Make a regular file then try to create a subdir under it.
	f := filepath.Join(tmp, "regular.file")
	if err := writeAtomic(f, []byte("x")); err != nil {
		t.Fatalf("writeAtomic prep failed: %v", err)
	}
	bad := filepath.Join(f, "subdir") // can't MkdirAll under a file

	_, err := NewStore(bad)
	if err == nil {
		t.Fatal("NewStore(bad path) returned nil — want error")
	}
	want := "<TR:internal_persistence_base_path_create_failed>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want contain %q — NewStore bypassed tr()", err.Error(), want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), Validate emits the bundle message IDs verbatim
// — confirming the migration didn't accidentally pass an empty string
// or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	err := Validate([]byte("anything"), Format("nope-not-a-format"))
	if err == nil {
		t.Fatal("Validate(unknown format) returned nil — want error")
	}
	if !strings.Contains(err.Error(), "internal_persistence_serializer_unknown_format") {
		t.Fatalf("err = %q, want contain raw ID (Noop echo)", err.Error())
	}
}
