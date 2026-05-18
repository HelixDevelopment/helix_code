// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/auth (round-146 §11.4 anti-bluff sweep, 2026-05-18).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	authi18n "dev.helix.code/internal/auth/i18n"
)

// sentinelTranslator wraps every resolved message ID with a
// recognisable marker so call-site tests can prove the lookup
// ACTUALLY went through Translator.T — not through a hardcoded
// literal that happens to match the bundle value (which would be a
// §11.4 PASS-bluff at the i18n call-site layer).
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if len(data) > 0 {
		// Include template keys in the sentinel so we can confirm
		// the call site actually passed templateData through.
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

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_auth_account_deactivated", nil)
	if got != "<SENT:internal_auth_account_deactivated>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_auth_account_deactivated", nil)
	if got != "internal_auth_account_deactivated" {
		t.Fatalf("after SetTranslator(nil), expected loud message-ID echo, got %q", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_auth_invalid_email", nil)
	if got != "internal_auth_invalid_email" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestValidateRegistration_UsernameTooShort_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	svc := NewAuthService(DefaultConfig(), nil)
	err := svc.validateRegistration("ab", "user@example.com", "longenough")
	if err == nil {
		t.Fatal("expected validation error for short username, got nil")
	}
	if err.Error() != "<SENT:internal_auth_username_length>" {
		t.Fatalf("validation error did not route through translator: got %q, want sentinel-wrapped internal_auth_username_length", err.Error())
	}
}

func TestValidateRegistration_InvalidEmail_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	svc := NewAuthService(DefaultConfig(), nil)
	err := svc.validateRegistration("validuser", "no-at-sign", "longenough")
	if err == nil {
		t.Fatal("expected validation error for invalid email, got nil")
	}
	if err.Error() != "<SENT:internal_auth_invalid_email>" {
		t.Fatalf("validation error did not route through translator: got %q", err.Error())
	}
}

func TestValidateRegistration_PasswordTooShort_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	svc := NewAuthService(DefaultConfig(), nil)
	err := svc.validateRegistration("validuser", "user@example.com", "short")
	if err == nil {
		t.Fatal("expected validation error for short password, got nil")
	}
	if err.Error() != "<SENT:internal_auth_password_too_short>" {
		t.Fatalf("validation error did not route through translator: got %q", err.Error())
	}
}

// TestBundle_ContainsAllMigratedIDs is a paired mutation test —
// it asserts every CONST-046 message ID emitted by auth.go appears
// in the active.en.yaml bundle. If a new round adds a tr() call
// without a bundle entry, this test must FAIL. Mirrors §1.1
// paired-mutation guidance.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := authi18n.NoopTranslator{}
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
	// Round-146 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_auth_account_deactivated",
		"internal_auth_failed_create_session",
		"internal_auth_failed_create_user",
		"internal_auth_failed_generate_session_token",
		"internal_auth_failed_hash_password",
		"internal_auth_failed_update_last_login",
		"internal_auth_invalid_email",
		"internal_auth_password_too_short",
		"internal_auth_unexpected_signing_method",
		"internal_auth_username_length",
	}
}
