package i18n

import (
	"errors"
	"testing"

	"golang.org/x/text/language"
)

func newLoadedLocalizer(t *testing.T) *Localizer {
	t.Helper()
	b := NewBundle(language.English)
	if err := b.LoadMessageFile("testdata/active.en.yaml"); err != nil {
		t.Fatalf("LoadMessageFile failed: %v", err)
	}
	return NewLocalizer(b, "en")
}

func TestLocalizer_T_RealMessage(t *testing.T) {
	loc := newLoadedLocalizer(t)
	got, err := loc.T("greeting", map[string]any{"Name": "Milos"})
	if err != nil {
		t.Fatalf("T(greeting) error: %v", err)
	}
	want := "Hello, Milos!"
	if got != want {
		t.Fatalf("T(greeting,Name=Milos) = %q; want %q", got, want)
	}
}

func TestLocalizer_T_LiteralMessage(t *testing.T) {
	loc := newLoadedLocalizer(t)
	got, err := loc.T("farewell")
	if err != nil {
		t.Fatalf("T(farewell) error: %v", err)
	}
	want := "Goodbye."
	if got != want {
		t.Fatalf("T(farewell) = %q; want %q", got, want)
	}
}

func TestLocalizer_TPlural_RealMessage_One(t *testing.T) {
	loc := newLoadedLocalizer(t)
	got, err := loc.TPlural("unread_messages", 1)
	if err != nil {
		t.Fatalf("TPlural(unread_messages,1) error: %v", err)
	}
	want := "You have 1 unread message."
	if got != want {
		t.Fatalf("TPlural(unread_messages,1) = %q; want %q", got, want)
	}
}

func TestLocalizer_TPlural_RealMessage_Other(t *testing.T) {
	loc := newLoadedLocalizer(t)
	got, err := loc.TPlural("unread_messages", 5)
	if err != nil {
		t.Fatalf("TPlural(unread_messages,5) error: %v", err)
	}
	want := "You have 5 unread messages."
	if got != want {
		t.Fatalf("TPlural(unread_messages,5) = %q; want %q", got, want)
	}
}

func TestLocalizer_T_MissingMessageReturnsSentinel(t *testing.T) {
	loc := newLoadedLocalizer(t)
	_, err := loc.T("definitely_does_not_exist_xyz")
	if err == nil {
		t.Fatalf("T(unknown) returned nil error; want ErrMessageNotFound. " +
			"If go-i18n's default fallback changed to silently return the message " +
			"ID string, this test fired correctly — tighten the wrapper.")
	}
	if !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("T(unknown) error = %v; want errors.Is(err, ErrMessageNotFound) == true", err)
	}
}
