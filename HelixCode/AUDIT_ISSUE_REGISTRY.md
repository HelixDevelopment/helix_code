# HelixCode Audit Issue Registry

**Created**: 2026-01-08
**Last Updated**: 2026-01-08
**Audit Phase**: Phase 1 - Critical Path Audit

---

## Issue Summary

| Category | Open | In Progress | Fixed | Verified | Total |
|----------|------|-------------|-------|----------|-------|
| CRITICAL | 0 | 0 | 1 | 0 | 1 |
| HIGH | 3 | 0 | 0 | 0 | 3 |
| MEDIUM | 1 | 0 | 4 | 0 | 5 |
| LOW | 3 | 0 | 1 | 0 | 4 |
| **Total** | **7** | **0** | **6** | **0** | **13** |

---

## Phase 1.1: auth/ Package Issues

### HELIX-001: Broken Argon2 Password Verification
```
ID: HELIX-001
Category: BROKEN
Severity: CRITICAL
Package: internal/auth
File: auth.go:329-398
Status: FIXED
```

**Description**: The `verifyArgon2Password` function is critically broken. It compares the hash with itself (`subtle.ConstantTimeCompare([]byte(hash), []byte(hash)) == 1`), which always returns true. This means any password would be accepted for Argon2-hashed passwords.

**Expected**: Function should properly decode Argon2 hash parameters (salt, time, memory, threads, key length) and verify the password against those parameters.

**Actual**: Function always returns true because it compares `hash` to `hash`.

**Code**:
```go
func (s *AuthService) verifyArgon2Password(password, hash string) bool {
    // This is a simplified implementation
    parts := strings.Split(hash, "$")
    if len(parts) != 6 {
        return false
    }
    // For now, just use a simple comparison
    // In a real implementation, you'd decode the parameters and verify
    return subtle.ConstantTimeCompare([]byte(hash), []byte(hash)) == 1  // BUG: Always true!
}
```

**Fix Required**: Yes - implement proper Argon2 verification or remove the fallback
**Test Required**: Yes - add test to verify Argon2 password verification

---

### HELIX-002: JWT Secret Hardcoded in Config File
```
ID: HELIX-002
Category: MOCKLEAK
Severity: HIGH
Package: config
File: config/config.yaml:27
Status: OPEN
```

**Description**: JWT secret is hardcoded in the config file that's checked into version control. This exposes the production secret.

**Expected**: JWT secret should be loaded from environment variable only.

**Actual**: Secret `QBHQ2paeBWWnOgniSQLqh1Dsd+pumKOcUTZbTXB+N0g=` is in plaintext in config.

**Fix Required**: Yes - change to `${HELIX_AUTH_JWT_SECRET}` and document requirement
**Test Required**: Yes - add test to verify secret is loaded from env var

---

### HELIX-003: VerifyJWT Returns Minimal User Object
```
ID: HELIX-003
Category: INCOMPLETE
Severity: HIGH
Package: internal/auth
File: auth.go:279-285
Status: OPEN
```

**Description**: The `VerifyJWT` function returns a minimal user object constructed from JWT claims instead of fetching the actual user from the database. The code contains a TODO comment acknowledging this.

**Expected**: After validating the JWT, fetch the complete user record from the database.

**Actual**: Returns `&User{ID, Username, Email}` only, missing `IsActive`, `IsVerified`, `MFAEnabled`, `DisplayName`, `LastLogin`, timestamps.

**Code**:
```go
// In a real implementation, you would fetch the user from the database
// For now, return a minimal user object
return &User{
    ID:       userID,
    Username: claims["username"].(string),
    Email:    claims["email"].(string),
}, nil
```

**Fix Required**: Yes - add database lookup option
**Test Required**: Yes - add test for complete user fetch

---

### HELIX-004: README AuthService Struct Mismatch
```
ID: HELIX-004
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md:32-40
Status: FIXED
```

**Description**: README shows incorrect AuthService struct definition.

**README Shows**:
```go
type AuthService struct {
    db          *database.Database
    jwtSecret   []byte
    tokenExpiry time.Duration
}
```

**Actual**:
```go
type AuthService struct {
    config AuthConfig
    db     AuthRepository
}
```

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-005: README Claims Struct Mismatch
```
ID: HELIX-005
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md (removed)
Status: FIXED
```

**Description**: README shows a custom Claims struct that doesn't exist. Code uses jwt.MapClaims.

**README Shows**:
```go
type Claims struct {
    UserID   string   `json:"user_id"`
    Username string   `json:"username"`
    Roles    []string `json:"roles"`
    jwt.RegisteredClaims
}
```

**Actual**: Code uses `jwt.MapClaims` with keys: `user_id`, `username`, `email`, `exp`, `iat`. No Roles field.

**Fix Required**: Yes - update README to match actual implementation
**Test Required**: No

---

### HELIX-006: README Session Struct Incomplete
```
ID: HELIX-006
Category: INCONSISTENT
Severity: LOW
Package: internal/auth
File: README.md:80-94
Status: FIXED
```

**Description**: README shows simplified Session struct missing several fields.

**README Shows**: `ID, UserID, Token, ExpiresAt, CreatedAt`

**Actual Has**: `ID, UserID, SessionToken, ClientType, IPAddress, UserAgent, ExpiresAt, CreatedAt`

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-007: README NewAuthService Constructor Mismatch
```
ID: HELIX-007
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md:99-113
Status: FIXED
```

**Description**: README shows incorrect constructor signature.

**README Shows**: `auth.NewAuthService(db, jwtSecret, 24*time.Hour)`

**Actual**: `NewAuthService(config AuthConfig, db AuthRepository)`

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-008: RBAC Not Implemented
```
ID: HELIX-008
Category: MISSING
Severity: HIGH
Package: internal/auth
File: README.md:11
Status: OPEN
```

**Description**: README claims "Role-based access control (RBAC)" but no RBAC implementation exists. User struct has no Roles field, JWT claims have no roles.

**Expected**: Either implement RBAC or remove claim from README.

**Fix Required**: Yes - either implement or document as not implemented
**Test Required**: Yes - if implementing, add comprehensive RBAC tests

---

### HELIX-009: Token Refresh Not Implemented
```
ID: HELIX-009
Category: MISSING
Severity: MEDIUM
Package: internal/auth
File: README.md:110
Status: OPEN
```

**Description**: README Security section mentions "Implement token refresh for long-running sessions" but no token refresh functionality exists.

**Fix Required**: Yes - either implement or update README
**Test Required**: Yes - if implementing

---

### HELIX-010: Rate Limiting Not Implemented
```
ID: HELIX-010
Category: MISSING
Severity: MEDIUM
Package: internal/auth
File: README.md:111
Status: OPEN
```

**Description**: README mentions "Implement rate limiting for login attempts" but no rate limiting exists.

**Fix Required**: Yes - either implement or update README
**Test Required**: Yes - if implementing

---

### HELIX-011: MFA Field Unused
```
ID: HELIX-011
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth.go:36
Status: OPEN
```

**Description**: User struct has `MFAEnabled bool` field but no MFA implementation exists. The field is always set to false during registration.

**Fix Required**: Yes - either implement MFA or remove field
**Test Required**: Yes - if implementing MFA

---

### HELIX-012: DisplayName Not Stored in DB
```
ID: HELIX-012
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth_db.go:72,111
Status: OPEN
```

**Description**: In `GetUserByUsername` and `GetUserByEmail`, DisplayName is hardcoded to empty string with comment "Not stored in DB", but `GetUserByID` tries to read it from DB. Schema inconsistency.

**Code**:
```go
user.DisplayName = "" // Not stored in DB
```

But in `GetUserByID`:
```go
&user.DisplayName,  // Tries to read from DB
```

**Fix Required**: Yes - ensure DisplayName is stored and retrieved consistently
**Test Required**: Yes - add test for DisplayName persistence

---

### HELIX-013: Test Coverage Gap - verifyArgon2Password Edge Cases
```
ID: HELIX-013
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth_test.go:163-243
Status: FIXED
```

**Description**: No test for `verifyArgon2Password` edge cases. Current tests only use bcrypt. The broken Argon2 implementation (HELIX-001) was not caught by tests.

**Resolution**: Added comprehensive `TestAuthService_verifyArgon2Password` test with 9 test cases covering:
- Valid Argon2id hash verification
- Wrong password rejection
- Invalid hash format detection
- Invalid algorithm detection
- Invalid version format detection
- Invalid parameters format detection
- Invalid base64 salt detection
- Invalid base64 hash detection
- bcrypt hash rejection in Argon2 verification

**Fix Required**: Yes - add tests for Argon2 password flow
**Test Required**: Yes - COMPLETED

---

## Phase 1.2: task/ Package Issues

*To be populated during task/ audit*

---

## Phase 1.3: worker/ Package Issues

*To be populated during worker/ audit*

---

## Phase 1.4: llm/ Package Issues

*To be populated during llm/ audit*

---

## Phase 1.5: workflow/ Package Issues

*To be populated during workflow/ audit*

---

## Changelog

| Date | Issue | Action | By |
|------|-------|--------|-----|
| 2026-01-08 | HELIX-001 to HELIX-013 | Created | Audit |
| 2026-01-08 | HELIX-001 | FIXED - Implemented proper Argon2 password verification | Audit |
| 2026-01-08 | HELIX-004 | FIXED - Updated README AuthService struct | Audit |
| 2026-01-08 | HELIX-005 | FIXED - Removed incorrect Claims struct from README | Audit |
| 2026-01-08 | HELIX-006 | FIXED - Updated README Session struct | Audit |
| 2026-01-08 | HELIX-007 | FIXED - Updated README constructor example | Audit |
| 2026-01-08 | HELIX-013 | FIXED - Added Argon2 verification tests | Audit |
