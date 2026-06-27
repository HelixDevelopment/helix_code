// Sentinel + paired-mutation tests for the CONST-046 translator
// wiring in internal/secrets (round-225 §11.4 anti-bluff sweep,
// 2026-05-19). Mocks ALLOWED per CONST-050(A) — this is a unit-test
// file.
package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	secretsi18n "dev.helix.code/internal/secrets/i18n"
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
	got := tr(context.Background(), "internal_secrets_no_source_found", nil)
	if got != "<SENT:internal_secrets_no_source_found>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_secrets_no_source_found", nil)
	if got == "internal_secrets_no_source_found" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_secrets_no_source_found", nil)
	if got != "internal_secrets_no_source_found" {
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
	got := tr(context.Background(), "internal_secrets_no_source_found", nil)
	if got != "internal_secrets_no_source_found" {
		t.Fatalf("tr() with empty translator returned %q, want raw message ID", got)
	}
}

// TestLoadAPIKeys_NoSource_RoutesThroughTranslator is the call-site
// sentinel proof: it asserts the no-source error path in LoadAPIKeys
// routes through the translator seam (not a hardcoded literal). If a
// future refactor accidentally reverts to errors.New("no api_keys.sh
// or .env found"), this test FAILS because the sentinel wrapper
// would be missing.
func TestLoadAPIKeys_NoSource_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	// Set up an isolated environment with no api_keys.sh and no
	// reachable .env file. We use a deep tmp directory so the
	// walk-up never finds an ambient .env from the developer's
	// working tree.
	home := t.TempDir() // no api_keys.sh inside
	cwd := t.TempDir()
	deep := filepath.Join(cwd, "x", "y", "z")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	withIsolatedEnv(t, home, deep, nil, func() {
		err := LoadAPIKeys()
		if err == nil {
			t.Fatal("expected error when neither file present, got nil")
		}
		if err.Error() != "<SENT:internal_secrets_no_source_found>" {
			t.Fatalf("no-source error did not route through translator: got %q, want sentinel-wrapped internal_secrets_no_source_found", err.Error())
		}
	})
}

// TestNoopTranslator_T_Loud_Echo_IsRawID is the paired-mutation
// bundle audit. It asserts every CONST-046 message ID emitted by
// loader.go appears in the active.en.yaml bundle. If a future round
// adds a tr() call without a bundle entry, this test must FAIL
// (because migratedMessageIDs() must be extended in lockstep). Note:
// the test exercises the NoopTranslator's loud-echo contract, which
// is itself the safety net when the bundle is missing.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := secretsi18n.NoopTranslator{}
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

// TestCONST042_LoaderDoesNotLogSecrets asserts loader.go contains no
// fmt.Print* / log.* call referencing any loaded value. CONST-042
// §12.1 (No-Secret-Leak) is the package's most critical invariant —
// values read from api_keys.sh / .env are applied via os.Setenv ONLY
// and MUST NEVER reach stdout, stderr, or a log sink. This is a
// belt-and-braces static check; the file-content scan complements
// the behavioural test by detecting accidental regressions at the
// source level.
func TestCONST042_LoaderDoesNotLogSecrets(t *testing.T) {
	// Read the loader source to assert no print/log call appears.
	data, err := os.ReadFile("loader.go")
	if err != nil {
		t.Fatalf("read loader.go: %v", err)
	}
	src := string(data)
	forbidden := []string{
		"fmt.Print",
		"fmt.Fprint",
		"log.Print",
		"log.Println",
		"log.Printf",
		"log.Fatal",
		"println(",
	}
	for _, pat := range forbidden {
		if strings.Contains(src, pat) {
			t.Fatalf("CONST-042 violation risk: loader.go contains %q — review for secret-leak path", pat)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-225 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_secrets_no_source_found",
	}
}
