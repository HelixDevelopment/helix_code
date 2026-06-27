// Unit tests for the internal/projectmemory package-level translator
// + tr() helper (CONST-046 round-235 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at the seam. Mocks ALLOWED per CONST-050(A)
// (unit tests only).
//
// Round-235 status: NO-OP INFRA round. The audit gate reports zero
// CONST-046 hardcoded-content violations in internal/projectmemory/
// at HEAD, so there are no real call sites to exercise yet. These
// tests exercise the seam itself (Translator interface contract,
// NoopTranslator behaviour, SetTranslator nil-reset, tr() error
// degradation) so that when a FUTURE round wires a real call site,
// regressions in the seam surface immediately.
package projectmemory

import (
	stdctx "context"
	"errors"
	"testing"

	pmi18n "dev.helix.code/internal/projectmemory/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so seam tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

// errTranslator forces tr() onto the degradation path so we can prove
// it surfaces the raw message ID instead of an empty string (which
// would be a §11.4 PASS-bluff at the i18n layer — user sees blank
// output).
type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// emptyTranslator returns "" without error — exercises the secondary
// "out == \"\"" degradation path in tr().
type emptyTranslator struct{}

func (emptyTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (emptyTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", nil
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got == "internal_projectmemory_reserved_placeholder" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	want := "<TR:internal_projectmemory_reserved_placeholder>"
	if got != want {
		t.Fatalf("tr = %q, want %q — seam bypassed Translator", got, want)
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

	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got != "internal_projectmemory_reserved_placeholder" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestTr_TranslatorEmptyStringReturnsMessageID(t *testing.T) {
	// Anti-bluff sister: even on error-less empty-string return, tr()
	// MUST degrade to the message ID rather than silently emitting
	// blank output.
	resetTranslator(t)
	SetTranslator(emptyTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got != "internal_projectmemory_reserved_placeholder" {
		t.Fatalf("tr on empty = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got == "internal_projectmemory_reserved_placeholder" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(pmi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got != "internal_projectmemory_reserved_placeholder" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestNoopTranslator_TPluralEchoesID covers the plural contract on
// the default Noop — pluralisation MUST also fall back to raw ID
// echo, not silently swallow.
func TestNoopTranslator_TPluralEchoesID(t *testing.T) {
	got, err := pmi18n.NoopTranslator{}.TPlural(stdctx.Background(), "internal_projectmemory_reserved_placeholder", 7, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural err = %v, want nil", err)
	}
	if got != "internal_projectmemory_reserved_placeholder" {
		t.Fatalf("NoopTranslator.TPlural = %q, want raw ID echo", got)
	}
}

// TestTr_RecoversFromNilPackageTranslator simulates the (impossible
// in production but defensively guarded) scenario where the
// package-level translator is nil. tr() MUST self-heal back to the
// Noop default rather than panic.
func TestTr_RecoversFromNilPackageTranslator(t *testing.T) {
	resetTranslator(t)
	// Force package-level translator nil to exercise the defensive
	// branch in tr(). This is the only test that touches the
	// unexported package-level variable directly.
	translator = nil
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_projectmemory_reserved_placeholder", nil)
	if got != "internal_projectmemory_reserved_placeholder" {
		t.Fatalf("tr after nil translator = %q, want raw ID echo (self-healed to Noop)", got)
	}
}

// TestSentinelTranslator_TPluralRoundtrip ensures the sentinel itself
// honours its plural contract — otherwise sibling-package translator
// tests that reuse this pattern could mask regressions.
func TestSentinelTranslator_TPluralRoundtrip(t *testing.T) {
	got, err := sentinelTranslator{}.TPlural(stdctx.Background(), "x", 3, nil)
	if err != nil {
		t.Fatalf("sentinelTranslator.TPlural err = %v, want nil", err)
	}
	if got != "<TR:x>" {
		t.Fatalf("sentinelTranslator.TPlural = %q, want %q", got, "<TR:x>")
	}
}
