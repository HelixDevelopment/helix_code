// Unit tests for the internal/context package-level translator + tr()
// helper (CONST-046 round-151 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package context

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	contexti18n "dev.helix.code/internal/context/i18n"
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
	got := tr(stdctx.Background(), "internal_context_item_not_found", nil)
	if got == "internal_context_item_not_found" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_context_session_not_found", nil)
	if got != "<TR:internal_context_session_not_found>" {
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

	got := tr(stdctx.Background(), "internal_context_project_not_found", nil)
	if got != "internal_context_project_not_found" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_context_global_manager_not_initialized", nil)
	if got == "internal_context_global_manager_not_initialized" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(contexti18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_context_item_expired", nil)
	if got != "internal_context_item_expired" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestContextManager_MigratedErrors_GoThroughTranslator is the
// call-site paired-mutation: with a sentinel translator wired, every
// migrated fmt.Errorf path on ContextManager MUST surface the
// sentinel-wrapped message ID — proving the literal was NOT hardcoded
// anywhere on the path. If a future refactor inlines any string, the
// matching case fails.
func TestContextManager_MigratedErrors_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cm := NewContextManager(nil)
	ctx := stdctx.Background()

	t.Run("retrieve_missing_item", func(t *testing.T) {
		_, err := cm.Retrieve(ctx, "nonexistent-id-12345")
		if err == nil {
			t.Fatal("Retrieve(nonexistent) returned no error")
		}
		want := "<TR:internal_context_item_not_found>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Retrieve error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
		}
	})

	t.Run("delete_missing_item", func(t *testing.T) {
		err := cm.Delete(ctx, "nonexistent-id-67890")
		if err == nil {
			t.Fatal("Delete(nonexistent) returned no error")
		}
		want := "<TR:internal_context_item_not_found>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Delete error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
		}
	})

	t.Run("get_session_missing", func(t *testing.T) {
		_, err := cm.GetSessionContext("session-does-not-exist")
		if err == nil {
			t.Fatal("GetSessionContext(missing) returned no error")
		}
		want := "<TR:internal_context_session_not_found>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("GetSessionContext error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
		}
	})

	t.Run("get_project_missing", func(t *testing.T) {
		_, err := cm.GetProjectContext("project-does-not-exist")
		if err == nil {
			t.Fatal("GetProjectContext(missing) returned no error")
		}
		want := "<TR:internal_context_project_not_found>"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("GetProjectContext error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
		}
	})
}

// TestGlobalManager_NotInitialized_GoesThroughTranslator covers the
// three free-function pre-init guards (StoreGlobal / RetrieveGlobal /
// SearchGlobal). Each MUST surface the sentinel-wrapped message ID
// when no global manager has been wired.
func TestGlobalManager_NotInitialized_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	// Save & reset globalManager to simulate "not initialized" without
	// leaking state to sibling tests.
	saved := globalManager
	globalManager = nil
	defer func() { globalManager = saved }()

	ctx := stdctx.Background()
	want := "<TR:internal_context_global_manager_not_initialized>"

	t.Run("store_global", func(t *testing.T) {
		err := StoreGlobal(ctx, &ContextItem{ID: "x"})
		if err == nil {
			t.Fatal("StoreGlobal(uninit) returned no error")
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("StoreGlobal error = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("retrieve_global", func(t *testing.T) {
		_, err := RetrieveGlobal(ctx, "id")
		if err == nil {
			t.Fatal("RetrieveGlobal(uninit) returned no error")
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("RetrieveGlobal error = %q, want contain %q", err.Error(), want)
		}
	})

	t.Run("search_global", func(t *testing.T) {
		_, err := SearchGlobal(ctx, "*", ContextTypeGlobal)
		if err == nil {
			t.Fatal("SearchGlobal(uninit) returned no error")
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("SearchGlobal error = %q, want contain %q", err.Error(), want)
		}
	})
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), Retrieve emits the bundle message ID — confirming
// the migration didn't accidentally pass an empty string or different
// literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	cm := NewContextManager(nil)
	_, err := cm.Retrieve(stdctx.Background(), "missing")
	if err == nil {
		t.Fatal("Retrieve(missing) returned no error")
	}
	if !strings.Contains(err.Error(), "internal_context_item_not_found") {
		t.Fatalf("Retrieve error = %q, want raw message ID (Noop echo)", err.Error())
	}
}
