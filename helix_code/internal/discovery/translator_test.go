// Unit tests for the internal/discovery package-level translator +
// tr() helper (CONST-046 round-154 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package discovery

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	discoveryi18n "dev.helix.code/internal/discovery/i18n"
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
	got := tr(stdctx.Background(), "internal_discovery_no_default_port_configured", nil)
	if got != "internal_discovery_no_default_port_configured" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_discovery_registry_not_enabled", nil)
	if got != "<TR:internal_discovery_registry_not_enabled>" {
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

	got := tr(stdctx.Background(), "internal_discovery_service_unhealthy_or_expired", nil)
	if got != "internal_discovery_service_unhealthy_or_expired" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_discovery_no_default_port_configured", nil)
	if got != "internal_discovery_no_default_port_configured" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(discoveryi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_discovery_dns_discovery_not_enabled", nil)
	if got != "internal_discovery_dns_discovery_not_enabled" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestRegister_RegistryNotEnabled_GoesThroughTranslator covers the
// guard on Register. With a sentinel translator wired, the error MUST
// surface the sentinel-wrapped message ID — proving the literal was
// NOT hardcoded on the path.
func TestRegister_RegistryNotEnabled_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := &DiscoveryClient{config: DiscoveryClientConfig{
		EnableRegistry: false,
	}}
	err := c.Register(ServiceInfo{Name: "svc"})
	if err == nil {
		t.Fatal("Register on disabled registry returned no error")
	}
	want := "<TR:internal_discovery_registry_not_enabled_or_not_configured>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Register error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestDeregister_RegistryNotEnabled_GoesThroughTranslator covers the
// matching guard on Deregister.
func TestDeregister_RegistryNotEnabled_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := &DiscoveryClient{config: DiscoveryClientConfig{
		EnableRegistry: false,
	}}
	err := c.Deregister("svc")
	if err == nil {
		t.Fatal("Deregister on disabled registry returned no error")
	}
	want := "<TR:internal_discovery_registry_not_enabled_or_not_configured>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Deregister error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHeartbeat_RegistryNotEnabled_GoesThroughTranslator covers the
// matching guard on Heartbeat.
func TestHeartbeat_RegistryNotEnabled_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := &DiscoveryClient{config: DiscoveryClientConfig{
		EnableRegistry: false,
	}}
	err := c.Heartbeat("svc")
	if err == nil {
		t.Fatal("Heartbeat on disabled registry returned no error")
	}
	want := "<TR:internal_discovery_registry_not_enabled_or_not_configured>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Heartbeat error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), the guard emits the bundle message ID —
// confirming the migration didn't accidentally pass an empty string
// or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	c := &DiscoveryClient{config: DiscoveryClientConfig{
		EnableRegistry: false,
	}}
	err := c.Register(ServiceInfo{Name: "svc"})
	if err == nil {
		t.Fatal("Register on disabled registry returned no error")
	}
	if !strings.Contains(err.Error(), "internal_discovery_registry_not_enabled_or_not_configured") {
		t.Fatalf("Register error = %q, want raw message ID (Noop echo)", err.Error())
	}
}
