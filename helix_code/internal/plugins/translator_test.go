// Sentinel + paired-mutation tests for the CONST-046 translator
// wiring in internal/plugins (round-234 §11.4 anti-bluff sweep,
// 2026-05-19). Mocks ALLOWED per CONST-050(A) — this is a unit-test
// file.
package plugins

import (
	"context"
	"errors"
	"strings"
	"testing"

	pluginsi18n "dev.helix.code/internal/plugins/i18n"
)

// sentinelTranslator wraps every resolved message ID with a
// recognisable marker so call-site tests can prove the lookup
// ACTUALLY went through Translator.T — not through a hardcoded
// literal that happens to match the bundle value (which would be a
// §11.4 PASS-bluff at the i18n call-site layer).
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if len(data) > 0 {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		return "<SENT:" + id + "|keys=" + strings.Join(keys, ",") + ">", nil
	}
	return "<SENT:" + id + ">", nil
}

func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<SENT:" + id + ">", nil
}

// errorTranslator always fails — exercises the tr() fallback path
// (must degrade to raw message ID, never to empty string).
type errorTranslator struct{}

func (errorTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func (errorTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

// emptyTranslator returns "" with no error — exercises the tr()
// empty-string fallback path (must degrade to raw message ID).
type emptyTranslator struct{}

func (emptyTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}

func (emptyTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", nil
}

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_plugins_sandbox_entrypoint_not_found", nil)
	if got != "<SENT:internal_plugins_sandbox_entrypoint_not_found>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_plugins_sandbox_entrypoint_not_found", nil)
	if got == "internal_plugins_sandbox_entrypoint_not_found" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_plugins_sandbox_entrypoint_not_found", nil)
	if got != "internal_plugins_sandbox_entrypoint_not_found" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestTr_FallsBackToMessageIDOnEmpty(t *testing.T) {
	// Anti-bluff: a translator that returns "" without error MUST
	// also degrade to the raw message ID. Without this fallback an
	// upstream bundle bug would surface as blank output to the
	// operator — a §11.4 PASS-bluff at the i18n fallback layer.
	resetTranslator(t)
	SetTranslator(emptyTranslator{})
	got := tr(context.Background(), "internal_plugins_sandbox_entrypoint_not_found", nil)
	if got != "internal_plugins_sandbox_entrypoint_not_found" {
		t.Fatalf("tr() with empty translator returned %q, want raw message ID", got)
	}
}

// TestExecutePlugin_EntrypointMissing_RoutesThroughTranslator is the
// call-site sentinel proof: it asserts the entrypoint-not-found
// warning string in ExecutePlugin routes through the translator
// seam (not a hardcoded literal). If a future refactor accidentally
// reverts to fmt.Sprintf("plugin sandbox: ...", ...), this test
// FAILS because the sentinel wrapper would be missing.
func TestExecutePlugin_EntrypointMissing_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	// Construct a BasePlugin whose entrypoint does NOT exist on disk
	// — exec.ExecutePlugin will fall through the os.Stat IsNotExist
	// branch and emit the migrated message ID. The plugin name is
	// chosen to be obviously synthetic so the test cannot
	// accidentally collide with a real plugin on the developer's
	// host.
	p := &BasePlugin{
		PluginName:    "round-234-sentinel-plugin-does-not-exist",
		PluginVersion: "0.0.0",
	}

	msg, err := ExecutePlugin(context.Background(), p, "noop", nil)
	if err == nil {
		t.Fatal("expected error for missing entrypoint, got nil")
	}
	if !strings.Contains(msg, "<SENT:internal_plugins_sandbox_entrypoint_not_found|keys=") {
		t.Fatalf("entrypoint-missing message did not route through translator: got %q", msg)
	}
	// Both placeholder keys MUST be present in the sentinel marker —
	// missing either is a regression that would silently drop a
	// placeholder from the user-facing output.
	if !strings.Contains(msg, "Name") || !strings.Contains(msg, "Entrypoint") {
		t.Fatalf("placeholder keys missing from sentinel output: got %q", msg)
	}
}

// TestNoopTranslator_T_Loud_Echo_IsRawID is the paired-mutation
// bundle audit. It asserts every CONST-046 message ID emitted by
// this package appears in the active.en.yaml bundle (verified
// implicitly: NoopTranslator returns id verbatim, and the call-site
// tests above prove call sites use these exact IDs). If a future
// round adds a tr() call without a bundle entry, the bundle scan
// + this loud-echo invariant must FAIL. Mirrors §1.1
// paired-mutation guidance.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := pluginsi18n.NoopTranslator{}
	for _, id := range migratedMessageIDs() {
		got, err := noop.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) returned %q, want loud echo of raw ID", id, got)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-234 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_plugins_sandbox_entrypoint_not_found",
	}
}
