# Auth Package

The `auth` package provides JWT-based authentication and authorization for the HelixCode platform.

## Overview

This package handles:
- User authentication with JWT tokens
- Session management
- Role-based access control (RBAC)
- Password hashing and verification
- Token generation and validation

## Key Types

### AuthService

The main authentication service that manages user authentication:

```go
type AuthService struct {
    db          *database.Database
    jwtSecret   []byte
    tokenExpiry time.Duration
}
```

### Claims

JWT claims structure for token validation:

```go
type Claims struct {
    UserID   string   `json:"user_id"`
    Username string   `json:"username"`
    Roles    []string `json:"roles"`
    jwt.RegisteredClaims
}
```

### Session

Represents an authenticated user session:

```go
type Session struct {
    ID        string
    UserID    string
    Token     string
    ExpiresAt time.Time
    CreatedAt time.Time
}
```

## Usage

### Creating the Auth Service

```go
import "dev.helix.code/internal/auth"

service := auth.NewAuthService(db, jwtSecret, 24*time.Hour)
```

### Authenticating Users

```go
// Login and get token
token, err := service.Login(ctx, username, password)
if err != nil {
    // Handle authentication error
}

// Validate token
claims, err := service.ValidateToken(token)
if err != nil {
    // Handle invalid token
}
```

### Session Management

```go
// Create session
session, err := service.CreateSession(ctx, userID)

// Get session
session, err := service.GetSession(ctx, sessionID)

// Invalidate session (logout)
err := service.InvalidateSession(ctx, sessionID)
```

## Configuration

Configure authentication via `config.yaml`:

```yaml
auth:
  jwt_secret: "${HELIX_AUTH_JWT_SECRET}"
  token_expiry: 24h
  session_timeout: 1h
  max_sessions_per_user: 5
```

## Security Considerations

- JWT secrets should be at least 32 bytes
- Store secrets in environment variables, not config files
- Implement token refresh for long-running sessions
- Use HTTPS in production
- Implement rate limiting for login attempts

## Testing

```bash
go test -v ./internal/auth/...
```
