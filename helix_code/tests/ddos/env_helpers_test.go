// HXC-150: unit tests proving envOrHelix / envOrIntHelix read HELIX_ vars
// first (.env.full-test convention), then fall back to legacy TEST_* vars,
// then to the hardcoded default. No infra needed — pure env-var tests.
package ddos

import (
	"os"
	"testing"
)

func TestEnvOrHelix_ReadsHelixFirst(t *testing.T) {
	t.Setenv("HELIX_DATABASE_USER", "helixcode")
	t.Setenv("TEST_PG_USER", "legacy_helix")
	if got := envOrHelix("HELIX_DATABASE_USER", "TEST_PG_USER", "fallback"); got != "helixcode" {
		t.Errorf("expected helixcode from HELIX_, got %q", got)
	}
}

func TestEnvOrHelix_FallsBackToLegacy(t *testing.T) {
	os.Unsetenv("HELIX_DATABASE_USER")
	t.Setenv("TEST_PG_USER", "legacy_helix")
	if got := envOrHelix("HELIX_DATABASE_USER", "TEST_PG_USER", "fallback"); got != "legacy_helix" {
		t.Errorf("expected legacy_helix from TEST_, got %q", got)
	}
}

func TestEnvOrHelix_FallsBackToDefault(t *testing.T) {
	os.Unsetenv("HELIX_DATABASE_USER")
	os.Unsetenv("TEST_PG_USER")
	if got := envOrHelix("HELIX_DATABASE_USER", "TEST_PG_USER", "fallback"); got != "fallback" {
		t.Errorf("expected fallback default, got %q", got)
	}
}

func TestEnvOrIntHelix_ReadsHelixFirst(t *testing.T) {
	t.Setenv("HELIX_DATABASE_PORT", "5433")
	t.Setenv("TEST_PG_PORT", "9999")
	if got := envOrIntHelix("HELIX_DATABASE_PORT", "TEST_PG_PORT", 5432); got != 5433 {
		t.Errorf("expected 5433 from HELIX_, got %d", got)
	}
}

func TestEnvOrIntHelix_FallsBackToLegacy(t *testing.T) {
	os.Unsetenv("HELIX_DATABASE_PORT")
	t.Setenv("TEST_PG_PORT", "9999")
	if got := envOrIntHelix("HELIX_DATABASE_PORT", "TEST_PG_PORT", 5432); got != 9999 {
		t.Errorf("expected 9999 from TEST_, got %d", got)
	}
}

func TestEnvOrIntHelix_FallsBackToDefault(t *testing.T) {
	os.Unsetenv("HELIX_DATABASE_PORT")
	os.Unsetenv("TEST_PG_PORT")
	if got := envOrIntHelix("HELIX_DATABASE_PORT", "TEST_PG_PORT", 5432); got != 5432 {
		t.Errorf("expected 5432 default, got %d", got)
	}
}
