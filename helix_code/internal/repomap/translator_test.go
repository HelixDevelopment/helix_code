// Unit tests for the internal/repomap package-level translator +
// tr() helper (CONST-046 round-198 §11.4 anti-bluff sweep,
// 2026-05-19; recovery from round-174 stall).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at the seam. Mocks ALLOWED per CONST-050(A)
// (unit tests only).
package repomap

import (
	stdctx "context"
	"errors"
	"testing"

	repomapi18n "dev.helix.code/internal/repomap/i18n"
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
	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	want := "<TR:internal_repomap_tool_description>"
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

	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
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

	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
		t.Fatalf("tr on empty = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(repomapi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
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

	got := tr(stdctx.Background(), "internal_repomap_tool_description", nil)
	if got != "internal_repomap_tool_description" {
		t.Fatalf("tr after nil translator = %q, want raw ID echo (self-healed to Noop)", got)
	}
}

// TestRepoMapTool_DescriptionUsesTranslatorSeam is the real-call-site
// paired mutation: with the sentinel Translator wired, RepoMapTool's
// Description() MUST return the sentinel-decorated message ID rather
// than the raw English bundle value. Catches any regression that
// reverts the call site to a hardcoded literal.
func TestRepoMapTool_DescriptionUsesTranslatorSeam(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tool := &RepoMapTool{}
	got := tool.Description()
	want := "<TR:internal_repomap_tool_description>"
	if got != want {
		t.Fatalf("RepoMapTool.Description() = %q, want %q — call site bypassed Translator (CONST-046 regression)", got, want)
	}
}

// TestRepoMapTool_DescriptionDefaultEchoesID confirms that under the
// Noop default, Description() returns the raw message ID (which is
// the loud-echo failure mode by design). Paired with the above
// sentinel test, this proves the call site reads from tr() and not
// from a frozen literal.
func TestRepoMapTool_DescriptionDefaultEchoesID(t *testing.T) {
	resetTranslator(t)
	tool := &RepoMapTool{}
	got := tool.Description()
	if got != "internal_repomap_tool_description" {
		t.Fatalf("RepoMapTool.Description() default = %q, want raw ID echo", got)
	}
}
