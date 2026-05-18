// Unit tests for the internal/editor package-level translator +
// tr() helper (CONST-046 round-155 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package editor

import (
	"context"
	"errors"
	"strings"
	"testing"

	editori18n "dev.helix.code/internal/editor/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
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
	got := tr(context.Background(), "internal_editor_file_path_required", nil)
	if got != "internal_editor_file_path_required" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_editor_content_required", nil)
	if got != "<TR:internal_editor_content_required>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade
	// to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_editor_apply_failed", nil)
	if got != "internal_editor_apply_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_editor_validation_failed", nil)
	if got != "internal_editor_validation_failed" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(editori18n.NoopTranslator{})
	got := tr(context.Background(), "internal_editor_backup_failed", nil)
	if got != "internal_editor_backup_failed" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestEditor_MigratedErrors_GoThroughTranslator is the call-site
// paired-mutation: with a sentinel translator wired, every migrated
// fmt.Errorf path in editor.go MUST surface the sentinel-wrapped
// message ID — proving the literal was NOT hardcoded anywhere on
// the path. If a future refactor inlines any string, the matching
// case fails.
func TestEditor_MigratedErrors_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	t.Run("new_code_editor_unsupported_format", func(t *testing.T) {
		_, err := NewCodeEditor(EditFormat("bogus-format"))
		if err == nil {
			t.Fatal("NewCodeEditor(bogus) returned no error")
		}
		want := "<TR:internal_editor_unsupported_format>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("NewCodeEditor err = %q, want contain %q — bypass", err.Error(), want)
		}
	})

	t.Run("set_format_unsupported", func(t *testing.T) {
		ce, err := NewCodeEditor(EditFormatWhole)
		if err != nil {
			t.Fatalf("NewCodeEditor: %v", err)
		}
		err = ce.SetFormat(EditFormat("nope"))
		if err == nil {
			t.Fatal("SetFormat(bogus) returned no error")
		}
		want := "<TR:internal_editor_unsupported_format>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("SetFormat err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("apply_edit_validation_failed", func(t *testing.T) {
		ce, err := NewCodeEditor(EditFormatWhole)
		if err != nil {
			t.Fatalf("NewCodeEditor: %v", err)
		}
		// Empty FilePath triggers validator failure.
		err = ce.ApplyEdit(Edit{Format: EditFormatWhole, Content: "x"})
		if err == nil {
			t.Fatal("ApplyEdit(empty path) returned no error")
		}
		want := "<TR:internal_editor_validation_failed>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ApplyEdit err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_file_path_required", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{Format: EditFormatWhole, Content: "x"})
		if err == nil {
			t.Fatal("Validate(empty path) returned no error")
		}
		want := "<TR:internal_editor_file_path_required>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_invalid_format", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormat("bogus"), Content: "x"})
		if err == nil {
			t.Fatal("Validate(bogus format) returned no error")
		}
		want := "<TR:internal_editor_invalid_format>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_content_required", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormatWhole, Content: nil})
		if err == nil {
			t.Fatal("Validate(nil content) returned no error")
		}
		want := "<TR:internal_editor_content_required>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_diff_requires_string", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormatDiff, Content: 42})
		if err == nil {
			t.Fatal("Validate(diff with int) returned no error")
		}
		want := "<TR:internal_editor_diff_requires_string>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_whole_requires_string", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormatWhole, Content: 42})
		if err == nil {
			t.Fatal("Validate(whole with int) returned no error")
		}
		want := "<TR:internal_editor_whole_requires_string>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_search_replace_requires_slice", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormatSearchReplace, Content: "not-a-slice"})
		if err == nil {
			t.Fatal("Validate(SR with string) returned no error")
		}
		want := "<TR:internal_editor_search_replace_requires_slice>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("validate_lines_requires_slice", func(t *testing.T) {
		v := NewDefaultValidator()
		err := v.Validate(Edit{FilePath: "/tmp/x", Format: EditFormatLines, Content: "not-a-slice"})
		if err == nil {
			t.Fatal("Validate(lines with string) returned no error")
		}
		want := "<TR:internal_editor_lines_requires_slice>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate err = %q, want contain %q", err.Error(), want)
		}
	})
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), Validate surfaces the bundle message ID —
// confirming the migration didn't accidentally pass an empty string
// or different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)
	v := NewDefaultValidator()
	err := v.Validate(Edit{Format: EditFormatWhole, Content: "x"})
	if err == nil {
		t.Fatal("Validate returned no error")
	}
	if !strings.Contains(err.Error(), "internal_editor_file_path_required") {
		t.Fatalf("Validate err = %q, want raw message ID (Noop echo)", err.Error())
	}
}
