package auth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAuthService_NilDB_NoPanic is the HXC-043 regression guard.
//
// Bug (root-caused as FACT): the HelixCode server boots with db=nil on its
// documented "continuing without database" path. The FIRST db-touching auth
// call — Login at auth.go:156 (s.db.GetUserByUsername) — then dereferenced a
// nil s.db and panicked (nil-pointer), which Gin Recovery turned into an
// empty HTTP 500. Reproduced live: see RED curl evidence captured in the
// HXC-043 fix commit (HTTP 500, Content-Length: 0, panic at auth.go:156
// reached via handlers.go:162).
//
// §11.4.115 RED-polarity switch:
//   - RED_MODE=1 documents the pre-fix defect: on the pre-fix artifact each
//     db-touching call below PANICS (the harness records that as the captured
//     defect). It is intended to be run against the BROKEN binary/source.
//   - RED_MODE=0 (default) is the standing GREEN regression guard: every
//     db-touching AuthService method called with a nil data store MUST return
//     a clean error (NOT panic). This is the permanent guard the suite keeps.
//
// The guard also covers the nil-*AuthService receiver case, which is the
// ACTUAL server boot path (server.New leaves authService nil when db==nil; a
// method call on a nil pointer receiver is legal in Go until the body
// dereferences a field — the same auth.go:156 line panics).
func TestAuthService_NilDB_NoPanic(t *testing.T) {
	ctx := context.Background()
	redMode := os.Getenv("RED_MODE") == "1"

	// Two equivalent broken topologies the server can present at boot:
	//   1. a non-nil *AuthService whose db field is nil
	//   2. a nil *AuthService pointer (the real server.New boot path)
	cases := map[string]*AuthService{
		"nil_db_field":    NewAuthService(DefaultConfig(), nil),
		"nil_authservice": (*AuthService)(nil),
	}

	for name, svc := range cases {
		svc := svc
		t.Run(name, func(t *testing.T) {
			// runGuarded runs fn, capturing panic-vs-error. `redMustPanic`
			// marks the calls whose pre-fix code path reaches s.db BEFORE any
			// other early return — those PANIC on the broken artifact and are
			// the §11.4.115 RED reproduction. (Login is the canonical
			// root-caused FACT: auth.go:156 s.db.GetUserByUsername.) Methods
			// that hit an earlier error path on the pre-fix code (e.g.
			// VerifyJWTWithDB parses the JWT first) do not panic pre-fix, so
			// RED_MODE only requires them to NOT silently succeed.
			runGuarded := func(label string, redMustPanic bool, fn func() error) {
				t.Helper()
				didPanic := false
				var gotErr error
				func() {
					defer func() {
						if r := recover(); r != nil {
							didPanic = true
						}
					}()
					gotErr = fn()
				}()

				if redMode {
					if redMustPanic {
						require.True(t, didPanic,
							"%s: RED_MODE expects the pre-fix nil-db defect to PANIC on the broken artifact", label)
					} else {
						require.True(t, didPanic || gotErr != nil,
							"%s: RED_MODE expects the pre-fix call to panic OR error, never silently succeed", label)
					}
					return
				}
				// GREEN: must not panic, must surface a clean error.
				require.False(t, didPanic,
					"%s: nil-db auth call must NOT panic (HXC-043)", label)
				require.Error(t, gotErr,
					"%s: nil-db auth call must return a clean error, not nil", label)
			}
			// assertCleanError is the common case: a db-touching call that
			// reaches s.db immediately on the pre-fix path (RED → panic).
			assertCleanError := func(label string, fn func() error) {
				runGuarded(label, true, fn)
			}

			// Login: returns ErrInvalidCredentials (handler maps to 401).
			assertCleanError("Login", func() error {
				_, _, err := svc.Login(ctx, "x", "y", "rest_api", "", "")
				if err == nil {
					return nil
				}
				// Pin the exact contract: nil-db Login is invalid-credentials.
				require.True(t, errors.Is(err, ErrInvalidCredentials),
					"Login on nil db must be ErrInvalidCredentials, got %v", err)
				return err
			})

			// Register: returns ErrAuthBackendUnavailable.
			assertCleanError("Register", func() error {
				_, err := svc.Register(ctx, "user", "user@example.com", "password123", "User")
				if err != nil {
					require.True(t, errors.Is(err, ErrAuthBackendUnavailable),
						"Register on nil db must be ErrAuthBackendUnavailable, got %v", err)
				}
				return err
			})

			// VerifySession: returns ErrTokenInvalid.
			assertCleanError("VerifySession", func() error {
				_, err := svc.VerifySession(ctx, "some-token")
				if err != nil {
					require.True(t, errors.Is(err, ErrTokenInvalid),
						"VerifySession on nil db must be ErrTokenInvalid, got %v", err)
				}
				return err
			})

			// Logout / LogoutAll / DeleteUser: ErrAuthBackendUnavailable.
			assertCleanError("Logout", func() error {
				err := svc.Logout(ctx, "some-token")
				if err != nil {
					require.True(t, errors.Is(err, ErrAuthBackendUnavailable),
						"Logout on nil db must be ErrAuthBackendUnavailable, got %v", err)
				}
				return err
			})
			assertCleanError("LogoutAll", func() error {
				return svc.LogoutAll(ctx, uuid.New())
			})
			assertCleanError("DeleteUser", func() error {
				return svc.DeleteUser(ctx, uuid.New())
			})

			// UpdateUser / VerifyJWTWithDB: ErrAuthBackendUnavailable.
			assertCleanError("UpdateUser", func() error {
				_, err := svc.UpdateUser(ctx, uuid.New(), "New Name", "new@example.com")
				return err
			})
			// VerifyJWTWithDB parses the JWT before touching s.db, so the
			// pre-fix path errors (parse failure) rather than panics — RED
			// only requires "not a silent success".
			runGuarded("VerifyJWTWithDB", false, func() error {
				_, err := svc.VerifyJWTWithDB(ctx, "any.jwt.token")
				return err
			})
		})
	}
}
