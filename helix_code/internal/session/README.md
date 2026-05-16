# Session Package

The `session` package provides session tracking and context management for the HelixCode platform.

## Overview

This package handles:
- User session management
- Context persistence across interactions
- Session state tracking
- Session timeout and cleanup
- Multi-device session support

## Key Types

### Session

```go
type Session struct {
    ID        string
    UserID    string
    ProjectID string
    Context   *Context
    Status    Status
    CreatedAt time.Time
    UpdatedAt time.Time
    ExpiresAt time.Time
}
```

### Context

```go
type Context struct {
    Messages     []*Message
    RecentFiles  []string
    CurrentTask  string
    Variables    map[string]interface{}
    Preferences  map[string]interface{}
}
```

### Status

```go
type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
    StatusExpired  Status = "expired"
)
```

## Usage

### Creating the Session Manager

```go
import "dev.helix.code/internal/session"

manager := session.NewManager(db, config)
```

### Creating Sessions

```go
// Create new session
sess, err := manager.CreateSession(ctx, userID, projectID)

// Create with options
sess, err := manager.CreateSessionWithOptions(ctx, &session.CreateOptions{
    UserID:    userID,
    ProjectID: projectID,
    TTL:       24 * time.Hour,
    Context: &session.Context{
        Preferences: map[string]interface{}{
            "theme": "dark",
        },
    },
})
```

### Session Operations

```go
// Get session
sess, err := manager.GetSession(ctx, sessionID)

// Update session
sess.Context.CurrentTask = taskID
err := manager.UpdateSession(ctx, sess)

// End session
err := manager.EndSession(ctx, sessionID)

// Refresh session (extend TTL)
err := manager.RefreshSession(ctx, sessionID)
```

### Context Management

```go
// Add to context
sess.Context.AddMessage(&session.Message{
    Role:    "user",
    Content: "Fix the bug in auth",
})

// Get recent files
files := sess.Context.RecentFiles

// Set variable
sess.Context.SetVariable("last_command", "build")

// Get variable
cmd := sess.Context.GetVariable("last_command")
```

### Session Lookup

```go
// Get active sessions for user
sessions, err := manager.GetUserSessions(ctx, userID)

// Get session by project
sess, err := manager.GetProjectSession(ctx, projectID)

// Find sessions with filter
sessions, err := manager.FindSessions(ctx, &session.Filter{
    UserID:    userID,
    Status:    session.StatusActive,
    CreatedAfter: time.Now().Add(-24 * time.Hour),
})
```

## Configuration

```yaml
session:
  default_ttl: 24h
  max_per_user: 5
  cleanup_interval: 1h
  context_max_messages: 100
  context_max_size: 1MB
```

## Session Lifecycle

```
Created -> Active -> Inactive -> Expired
              |
              v
           Ended
```

## Multi-Device Support

```go
// Get all active sessions for user
sessions, err := manager.GetActiveSessions(ctx, userID)

// End all sessions (logout everywhere)
err := manager.EndAllSessions(ctx, userID)

// End all sessions except current
err := manager.EndOtherSessions(ctx, userID, currentSessionID)
```

## Persistence

Sessions are automatically persisted to the database:

```go
// Sessions auto-save on update
sess.Context.CurrentTask = taskID
manager.UpdateSession(ctx, sess)  // Saved to DB

// Manual save
err := manager.SaveSession(ctx, sess)
```

## Testing

```bash
go test -v ./internal/session/...
```

## Notes

- Sessions expire after configured TTL
- Context is preserved across interactions
- Implement session cleanup for expired sessions
- Monitor active sessions per user
