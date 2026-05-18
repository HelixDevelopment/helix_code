package i18n

import (
	"embed"
	"errors"
	"testing"

	"golang.org/x/text/language"
)

//go:embed testdata/active.en.yaml
var embeddedTestdata embed.FS

func TestBundle_LoadMessageFile_RealYAML(t *testing.T) {
	b := NewBundle(language.English)
	if err := b.LoadMessageFile("testdata/active.en.yaml"); err != nil {
		t.Fatalf("LoadMessageFile failed: %v", err)
	}
	if !b.hasLoadedFiles() {
		t.Fatalf("hasLoadedFiles() = false after successful load; want true")
	}
	// Verify each of the three seed IDs is resolvable end-to-end.
	loc := NewLocalizer(b, "en")
	for _, id := range []string{"greeting", "farewell"} {
		data := map[string]any{}
		if id == "greeting" {
			data["Name"] = "World"
		}
		got, err := loc.T(id, data)
		if err != nil {
			t.Fatalf("T(%q) returned error: %v", id, err)
		}
		if got == "" {
			t.Fatalf("T(%q) returned empty string; want non-empty", id)
		}
	}
	// Plural ID validated separately below.
	if _, err := loc.TPlural("unread_messages", 1); err != nil {
		t.Fatalf("TPlural(unread_messages,1) failed: %v", err)
	}
}

func TestBundle_LoadMessageFileFS_RealEmbed(t *testing.T) {
	b := NewBundle(language.English)
	if err := b.LoadMessageFileFS(embeddedTestdata, "testdata/active.en.yaml"); err != nil {
		t.Fatalf("LoadMessageFileFS failed: %v", err)
	}
	loc := NewLocalizer(b, "en")
	got, err := loc.T("greeting", map[string]any{"Name": "Embed"})
	if err != nil {
		t.Fatalf("T(greeting) via embed.FS failed: %v", err)
	}
	want := "Hello, Embed!"
	if got != want {
		t.Fatalf("embed.FS T(greeting) = %q; want %q", got, want)
	}
}

func TestBundle_MustParseMessageFileBytes(t *testing.T) {
	b := NewBundle(language.English)
	buf := []byte("ping:\n  other: \"pong\"\n")
	b.MustParseMessageFileBytes(buf, "active.en.yaml")
	loc := NewLocalizer(b, "en")
	got, err := loc.T("ping")
	if err != nil {
		t.Fatalf("T(ping) after in-memory parse failed: %v", err)
	}
	if got != "pong" {
		t.Fatalf("T(ping) = %q; want %q", got, "pong")
	}
}

func TestBundle_NoFilesLoaded_LookupReturnsSentinel(t *testing.T) {
	b := NewBundle(language.English)
	loc := NewLocalizer(b, "en")
	_, err := loc.T("greeting")
	if err == nil {
		t.Fatalf("T() on empty bundle returned nil error; want ErrBundleNotConfigured")
	}
	if !errors.Is(err, ErrBundleNotConfigured) {
		t.Fatalf("T() error = %v; want errors.Is(err, ErrBundleNotConfigured) == true", err)
	}
	// TPlural must enforce the same guard.
	_, err = loc.TPlural("unread_messages", 1)
	if err == nil {
		t.Fatalf("TPlural() on empty bundle returned nil error; want ErrBundleNotConfigured")
	}
	if !errors.Is(err, ErrBundleNotConfigured) {
		t.Fatalf("TPlural() error = %v; want errors.Is(err, ErrBundleNotConfigured) == true", err)
	}
}
