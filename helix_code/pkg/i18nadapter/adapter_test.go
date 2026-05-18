package i18nadapter

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/pkg/i18n"
	"golang.org/x/text/language"
)

const enBundleYAML = `greeting:
  other: "Hello, {{.Name}}!"
unread_messages:
  one: "You have {{.PluralCount}} unread message."
  other: "You have {{.PluralCount}} unread messages."
farewell:
  other: "Goodbye."
`

func newAdapter(t *testing.T) *Translator {
	t.Helper()
	b := i18n.NewBundle(language.English)
	b.MustParseMessageFileBytes([]byte(enBundleYAML), "active.en.yaml")
	loc := i18n.NewLocalizer(b, "en")
	return New(loc)
}

func TestNew_NilLocalizerPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("New(nil) did not panic; nil-localizer is a §11.4 PASS-bluff and MUST panic")
		}
	}()
	_ = New(nil)
}

func TestTranslator_T_WithTemplateData(t *testing.T) {
	tr := newAdapter(t)
	got, err := tr.T(context.Background(), "greeting", map[string]any{"Name": "Milos"})
	if err != nil {
		t.Fatalf("T(greeting, Name=Milos) error: %v", err)
	}
	want := "Hello, Milos!"
	if got != want {
		t.Fatalf("T(greeting, Name=Milos) = %q; want %q", got, want)
	}
}

func TestTranslator_T_NilTemplateData(t *testing.T) {
	tr := newAdapter(t)
	got, err := tr.T(context.Background(), "farewell", nil)
	if err != nil {
		t.Fatalf("T(farewell, nil) error: %v", err)
	}
	want := "Goodbye."
	if got != want {
		t.Fatalf("T(farewell, nil) = %q; want %q", got, want)
	}
}

func TestTranslator_TPlural_One(t *testing.T) {
	tr := newAdapter(t)
	got, err := tr.TPlural(context.Background(), "unread_messages", 1, nil)
	if err != nil {
		t.Fatalf("TPlural(unread_messages, 1) error: %v", err)
	}
	want := "You have 1 unread message."
	if got != want {
		t.Fatalf("TPlural(unread_messages, 1) = %q; want %q", got, want)
	}
}

func TestTranslator_TPlural_Other(t *testing.T) {
	tr := newAdapter(t)
	got, err := tr.TPlural(context.Background(), "unread_messages", 7, nil)
	if err != nil {
		t.Fatalf("TPlural(unread_messages, 7) error: %v", err)
	}
	want := "You have 7 unread messages."
	if got != want {
		t.Fatalf("TPlural(unread_messages, 7) = %q; want %q", got, want)
	}
}

func TestTranslator_T_MissingMessageSurfacesSentinel(t *testing.T) {
	tr := newAdapter(t)
	_, err := tr.T(context.Background(), "nonexistent_key_xyz", nil)
	if err == nil {
		t.Fatalf("T(unknown) returned nil error; want ErrMessageNotFound")
	}
	if !errors.Is(err, i18n.ErrMessageNotFound) {
		t.Fatalf("T(unknown) err = %v; want errors.Is(err, i18n.ErrMessageNotFound) == true", err)
	}
}
