# S1 — CORSHeaders stale-test reconciliation (§11.4.120)

## Scope
- Edited: `helix_code/tests/regression/critical_paths_test.go`
  (`TestCriticalPath_APIEndpointAvailability` router setup + the
  `CORSHeaders` subtest only).
- Read-only (confirmed, not modified): `helix_code/internal/server/server.go`
  (`CORSMiddleware`, lines ~749-776) — used to learn the actual secure
  behavior; `helix_code/internal/server/cors_security_test.go` — used as the
  canonical reference pattern for asserting the hardened allowlist behavior.

## Root cause (why the test was stale, not the code)
`CORSMiddleware(allowedOrigins []string)` was hardened to a strict
allowlist: an allowlisted `Origin` is echoed back verbatim (with
`Vary: Origin` + `Access-Control-Allow-Credentials: true`); any other
origin (including none/empty allowlist) gets **no**
`Access-Control-Allow-Origin` header at all (default-deny). It never emits
`"*"`. The old subtest asserted the pre-hardening wildcard behavior
(`Access-Control-Allow-Origin == "*"`), so it was asserting removed,
intentionally-insecure behavior. Per §11.4.120 this is a stale-test
defect, not a product defect — reconciled the test to the new correct
mechanism instead of reverting the security fix.

## RED (defect/staleness present — captured before edit)
Command:
```
cd helix_code && go test -tags=nogui ./tests/regression/ \
  -run 'TestCriticalPath_APIEndpointAvailability/CORSHeaders' -v -count=1
```
Result: **FAIL**
```
critical_paths_test.go:733:
    Error: Not equal:
       expected: "*"
       actual  : ""
    Messages: CORS origin should be set
--- FAIL: TestCriticalPath_APIEndpointAvailability/CORSHeaders (0.00s)
FAIL
```
(Full output captured at `/tmp/s1_red.txt`.)

## Fix (test reconciliation only — no product code touched)
1. Router setup now passes a real allowlist,
   `corsAllowedOrigins := []string{"http://localhost:3000"}`, into
   `server.CORSMiddleware(...)` instead of `nil` (comment added explaining
   why, citing §11.4.120 and cross-referencing `cors_security_test.go`).
2. `CORSHeaders` subtest rewritten to assert the SECURE, deterministic
   (§11.4.50) behavior:
   - Allowlisted origin `http://localhost:3000` → `204`, `Access-Control-Allow-Origin`
     echoes the exact origin (asserted equal to the origin string AND
     `assert.NotEqual(t, "*", ...)` to make the never-wildcard invariant
     explicit and future-regression-proof), `Access-Control-Allow-Methods`
     contains `GET`.
   - Non-allowlisted origin `http://evil.example.com` → `204`, but
     `Access-Control-Allow-Origin` is **empty** (default-deny), proving the
     middleware does not leak CORS-allow headers to arbitrary origins.
3. No wildcard `"*"` was re-introduced as an asserted/expected value
   anywhere in the diff — verified below.

## GREEN (fix verified — captured after edit)
Command:
```
cd helix_code && go test -tags=nogui ./tests/regression/ \
  -run 'TestCriticalPath_APIEndpointAvailability' -v -count=1
```
Result: **PASS**
```
=== RUN   TestCriticalPath_APIEndpointAvailability/HealthEndpoint
=== RUN   TestCriticalPath_APIEndpointAvailability/CORSHeaders
=== RUN   TestCriticalPath_APIEndpointAvailability/SecurityHeaders
--- PASS: TestCriticalPath_APIEndpointAvailability (0.00s)
    --- PASS: TestCriticalPath_APIEndpointAvailability/HealthEndpoint (0.00s)
    --- PASS: TestCriticalPath_APIEndpointAvailability/CORSHeaders (0.00s)
    --- PASS: TestCriticalPath_APIEndpointAvailability/SecurityHeaders (0.00s)
PASS
ok  	dev.helix.code/tests/regression	0.013s
```

## Full-package regression sweep (no collateral breakage)
Command:
```
cd helix_code && go test -tags=nogui ./tests/regression/... -v -count=1
```
Result: **PASS** — every test in the package, including
`TestCriticalPath_APIEndpointAvailability` and its three subtests
(`HealthEndpoint`, `CORSHeaders`, `SecurityHeaders`), plus all sibling
`TestCriticalPath_*` tests and `TestServerTimeoutConfiguration` /
`TestServerStability`.
```
ok  	dev.helix.code/tests/regression	1.772s
```
(Full output captured at `/tmp/s1_full_pkg.txt`.)

## Self-review (§11.4.120 / §11.4.6 anti-bluff check)
- `git diff -- tests/regression/critical_paths_test.go` shows `"*"` only in
  (a) explanatory comments describing the REMOVED insecure behavior, and
  (b) `assert.NotEqual(t, "*", ...)` — i.e. asserting the wildcard must
  NOT appear. No wildcard is asserted as expected/allowed anywhere.
- `git status --porcelain -- tests/regression/critical_paths_test.go
  internal/server/` shows exactly one modified file:
  `helix_code/tests/regression/critical_paths_test.go`. No files under
  `internal/server/` were touched.
- No `git add`/`git commit` performed (conductor commits per task
  instructions).
