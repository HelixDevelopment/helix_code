# Auth Package

The `auth` package provides JWT-based authentication and session management for the HelixCode platform.

## Overview

This package handles:
- User authentication with JWT tokens
- Session management with token-based access
- Password hashing (bcrypt and Argon2)
- Token generation and validation

## Key Types

### AuthConfig

Configuration for the authentication service:

```go
type AuthConfig struct {
    JWTSecret       string
    TokenExpiry     time.Duration
    SessionExpiry   time.Duration
    BcryptCost      int
    Argon2Time      uint32
    Argon2Memory    uint32
    Argon2Threads   uint8
    Argon2KeyLength uint32
}
```

### AuthService

The main authentication service that manages user authentication:

```go
type AuthService struct {
    config AuthConfig
    db     AuthRepository
}
```

### AuthRepository

Interface for authentication data storage:

```go
type AuthRepository interface {
    CreateUser(ctx context.Context, user *User, passwordHash string) error
    GetUserByUsername(ctx context.Context, username string) (*User, string, error)
    GetUserByEmail(ctx context.Context, email string) (*User, string, error)
    GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
    UpdateUserLastLogin(ctx context.Context, id uuid.UUID) error
    CreateSession(ctx context.Context, session *Session) error
    GetSession(ctx context.Context, token string) (*Session, error)
    DeleteSession(ctx context.Context, token string) error
    DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}
```

### User

Represents an authenticated user:

```go
type User struct {
    ID          uuid.UUID `json:"id"`
    Username    string    `json:"username"`
    Email       string    `json:"email"`
    DisplayName string    `json:"display_name"`
    IsActive    bool      `json:"is_active"`
    IsVerified  bool      `json:"is_verified"`
    MFAEnabled  bool      `json:"mfa_enabled"`
    LastLogin   time.Time `json:"last_login"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

### Session

Represents an authenticated user session:

```go
type Session struct {
    ID           uuid.UUID `json:"id"`
    UserID       uuid.UUID `json:"user_id"`
    SessionToken string    `json:"session_token"`
    ClientType   string    `json:"client_type"`
    IPAddress    net.IP    `json:"ip_address"`
    UserAgent    string    `json:"user_agent"`
    ExpiresAt    time.Time `json:"expires_at"`
    CreatedAt    time.Time `json:"created_at"`
}
```

## Usage

### Creating the Auth Service

```go
import "dev.helix.code/internal/auth"

config := auth.AuthConfig{
    JWTSecret:     os.Getenv("HELIX_AUTH_JWT_SECRET"),
    TokenExpiry:   24 * time.Hour,
    SessionExpiry: 7 * 24 * time.Hour,
    BcryptCost:    12,
}

db := auth.NewAuthDB(database) // implements AuthRepository
service := auth.NewAuthService(config, db)
```

### User Registration

```go
user, err := service.Register(ctx, "username", "email@example.com", "password", "Display Name")
if err != nil {
    // Handle registration error
}
```

### Authenticating Users

```go
// Login and get session
session, user, err := service.Login(ctx, "username", "password", "web", "127.0.0.1", "Mozilla/5.0")
if err != nil {
    // Handle authentication error
}

// Verify session
user, err := service.VerifySession(ctx, session.SessionToken)
if err != nil {
    // Handle invalid session
}

// Generate JWT for API access
token, err := service.GenerateJWT(user)
if err != nil {
    // Handle token generation error
}

// Verify JWT (returns minimal user from claims - fast)
user, err := service.VerifyJWT(token)
if err != nil {
    // Handle invalid token
}

// Verify JWT with database lookup (returns complete user - slower)
// Use this when you need IsActive, IsVerified, MFAEnabled, DisplayName, etc.
user, err := service.VerifyJWTWithDB(ctx, token)
if err != nil {
    // Handle invalid token or inactive user
}
```

### Session Management

```go
// Logout single session
err := service.Logout(ctx, sessionToken)

// Logout all sessions for a user
err := service.LogoutAll(ctx, userID)
```

## Configuration

Configure authentication via `config.yaml`:

```yaml
auth:
  jwt_secret: "${HELIX_AUTH_JWT_SECRET}"  # REQUIRED: Set via environment variable
  token_expiry: 86400                      # Token expiry in seconds (24 hours)
  session_expiry: 604800                   # Session expiry in seconds (7 days)
  bcrypt_cost: 12                          # bcrypt cost factor
```

## Security Considerations

- **JWT secrets must be set via environment variables** - never commit secrets to config files
- JWT secrets should be at least 32 bytes (256 bits)
- Use HTTPS in production to protect tokens in transit
- Session tokens are cryptographically secure (32 bytes of random data)
- Passwords are hashed using bcrypt (default) or Argon2
- Constant-time comparison is used for password verification to prevent timing attacks

## Future Enhancements

The following features are not yet implemented but may be added in future versions:
- Role-based access control (RBAC)
- Token refresh for long-running sessions
- Rate limiting for login attempts
- Multi-factor authentication (MFA)

## Testing

```bash
go test -v ./internal/auth/...
```
