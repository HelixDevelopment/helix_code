# HXC-043 — auth Login nil-DB panic → HTTP 500 (real product bug, found by HXC-041 live run)
Case HXC-AUTH-003 (login, expect 401) got HTTP 500 empty body. ROOT CAUSE (FACT from server stacktrace):
with db=nil (server's graceful "continuing without database" path), helix_code/internal/auth/auth.go:156
(*AuthService).Login calls s.db.GetUserByUsername on nil s.db → nil-pointer panic → Gin Recovery → HTTP 500.
Reached via internal/server/handlers.go:162. Raw curl confirmed HTTP/1.1 500. FIX: guard nil s.db in Login +
sibling db-touching auth paths → clean 401/503. RED test = helixcode-auth.yaml HXC-AUTH-003 via `helixqa http`.
[Fix in progress under subagent — RED→GREEN evidence to be appended.]

## FIXED (S1, commit fe764e96) — RED→GREEN, real captured evidence
Guarded all 8 db-touching auth methods via new nil-receiver-safe (*AuthService).dbAvailable()
(handles nil db-field AND nil *AuthService receiver = the real boot path). Login→ErrInvalidCredentials(401),
Register/Logout/LogoutAll/UpdateUser/DeleteUser/VerifyJWTWithDB→ErrAuthBackendUnavailable(503),
VerifySession→ErrTokenInvalid. handlers.go login maps ErrAuthBackendUnavailable→503, keeps 401 default.
- RED (pre-fix binary, db=nil): POST /api/v1/auth/login → HTTP 500 + panic at auth.go:156.
- verify-compile: exit 0.
- GREEN (fixed binary, db=nil): same curl → HTTP 401 {"error":"invalid credentials","message":"Login failed"}, panic_count=0.
- helixqa bank vs live server: 16 cases — 16 PASS, 0 FAIL, 0 SKIP (was 15/16). HXC-AUTH-003 now PASS.
- New unit test auth_nildb_test.go TestAuthService_NilDB_NoPanic GREEN; §11.4.115 RED_MODE=1 on guard-stripped copy PASSES (defect genuinely reproduces pre-fix).
Commit fe764e96 (auth.go, auth_nildb_test.go, handlers.go) — already in meta history + pushed (ancestor of 81830bba).
