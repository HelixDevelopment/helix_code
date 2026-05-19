// Tests for the CONST-046 i18n seam wired into internal/version
// (round-183 §11.4 anti-bluff sweep, 2026-05-19, CONST-046 Phase 4
// round 76). Mocks ALLOWED here per CONST-050(A) — this is a unit
// test invoked without the integration build tag.
package version

import (
	"context"
	"strings"
	"sync"
	"testing"

	versioni18n "dev.helix.code/internal/version/i18n"
)

// fakeTranslator returns sentinel-wrapped message IDs so call-site
// tests can assert lookup actually went through Translator.T rather
// than a hardcoded literal that happens to match the bundle value.
type fakeTranslator struct {
	mu     sync.Mutex
	seenT  []string
	render func(id string, data map[string]any) string
}

func (f *fakeTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	f.mu.Lock()
	f.seenT = append(f.seenT, id)
	f.mu.Unlock()
	if f.render != nil {
		return f.render(id, data), nil
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f *fakeTranslator) TPlural(ctx context.Context, id string, _ int, data map[string]any) (string, error) {
	return f.T(ctx, id, data)
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Save + restore translator across the test so we don't leak
	// state into sibling tests in this package.
	defer SetTranslator(nil)

	SetTranslator(&fakeTranslator{})
	SetTranslator(nil)

	got := tr(context.Background(), "internal_version_full_version_banner", nil)
	if got != "internal_version_full_version_banner" {
		t.Fatalf("after SetTranslator(nil), tr() returned %q, want loud echo of message ID", got)
	}
}

func TestTr_UsesWiredTranslator(t *testing.T) {
	defer SetTranslator(nil)

	fake := &fakeTranslator{}
	SetTranslator(fake)

	got := tr(context.Background(), "internal_version_full_version_banner", nil)
	want := "<TRANSLATED:internal_version_full_version_banner>"
	if got != want {
		t.Fatalf("tr() returned %q, want %q", got, want)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.seenT) != 1 || fake.seenT[0] != "internal_version_full_version_banner" {
		t.Fatalf("fake.seenT = %v, want exactly [internal_version_full_version_banner]", fake.seenT)
	}
}

func TestGetFullVersion_RoutesThroughTranslatorWhenWired(t *testing.T) {
	defer SetTranslator(nil)

	// Save + restore the package-level build vars so the assertion
	// below is deterministic regardless of how the binary was built.
	origVersion, origCommit, origBuildDate, origGoVersion := Version, GitCommit, BuildDate, GoVersion
	defer func() {
		Version, GitCommit, BuildDate, GoVersion = origVersion, origCommit, origBuildDate, origGoVersion
	}()
	Version = "9.9.9"
	GitCommit = "deadbeef00112233"
	BuildDate = "2026-05-19T00:00:00Z"
	GoVersion = "go1.26-test"

	SetTranslator(&fakeTranslator{
		render: func(id string, data map[string]any) string {
			// Anti-bluff: prove the placeholders actually arrived.
			// If GetFullVersion is migrated incorrectly (no template
			// data passed) this assertion would fail loudly.
			if data["Version"] != "9.9.9" {
				t.Errorf("Translator.T(full banner) missing Version placeholder; data=%v", data)
			}
			if data["Commit"] != "deadbeef00112233" {
				t.Errorf("Translator.T(full banner) missing Commit placeholder; data=%v", data)
			}
			if data["BuildDate"] != "2026-05-19T00:00:00Z" {
				t.Errorf("Translator.T(full banner) missing BuildDate placeholder; data=%v", data)
			}
			if data["GoVersion"] != "go1.26-test" {
				t.Errorf("Translator.T(full banner) missing GoVersion placeholder; data=%v", data)
			}
			return "WIRED-BANNER:" + id
		},
	})

	got := GetFullVersion()
	if !strings.HasPrefix(got, "WIRED-BANNER:") {
		t.Fatalf("GetFullVersion() returned %q, want prefix \"WIRED-BANNER:\" (translator-routed)", got)
	}
}

func TestGetVersion_RoutesThroughTranslatorWhenWired(t *testing.T) {
	defer SetTranslator(nil)

	origVersion, origCommit := Version, GitCommit
	defer func() {
		Version, GitCommit = origVersion, origCommit
	}()
	Version = "9.9.9"
	GitCommit = "deadbeef00112233"

	SetTranslator(&fakeTranslator{
		render: func(id string, data map[string]any) string {
			if data["Version"] != "9.9.9" {
				t.Errorf("Translator.T(short) missing Version placeholder; data=%v", data)
			}
			if data["ShortCommit"] != "deadbee" {
				t.Errorf("Translator.T(short) missing ShortCommit=deadbee; data=%v", data)
			}
			return "WIRED-SHORT:" + id
		},
	})

	got := GetVersion()
	if !strings.HasPrefix(got, "WIRED-SHORT:") {
		t.Fatalf("GetVersion() returned %q, want prefix \"WIRED-SHORT:\" (translator-routed)", got)
	}
}

func TestGetFullVersion_FallbackPreservesHistoricalShape(t *testing.T) {
	// With no wired translator, GetFullVersion MUST still return the
	// canonical "HelixCode <v> (commit: ..., built: ..., go: ...)"
	// shape — the §11.4 anti-bluff guard against silent translator
	// failure producing a raw message ID in user-facing output.
	defer SetTranslator(nil)
	SetTranslator(nil)

	got := GetFullVersion()
	if !strings.HasPrefix(got, "HelixCode ") {
		t.Fatalf("fallback GetFullVersion() = %q, want prefix \"HelixCode \"", got)
	}
	if !strings.Contains(got, "commit:") || !strings.Contains(got, "built:") || !strings.Contains(got, "go:") {
		t.Fatalf("fallback GetFullVersion() = %q, missing one of commit:/built:/go:", got)
	}
}

func TestSetTranslator_AssertsTypeContract(t *testing.T) {
	// Compile-time guarantee that *fakeTranslator satisfies the
	// versioni18n.Translator contract used by SetTranslator. If the
	// interface ever drifts, this test fails at build, not silently
	// at runtime.
	var _ versioni18n.Translator = (*fakeTranslator)(nil)
}
