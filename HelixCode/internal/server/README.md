# Server Package

The `server` package provides HTTP server, routing, and API handlers for the HelixCode platform.

## Overview

This package handles:
- HTTP/HTTPS server management
- REST API routing
- WebSocket support
- Middleware (auth, logging, CORS)
- Request validation
- Error handling

## Key Types

### Server

The main HTTP server:

```go
type Server struct {
    router     *gin.Engine
    config     *Config
    auth       *auth.AuthService
    db         *database.Database
    httpServer *http.Server
}
```

### Config

Server configuration:

```go
type Config struct {
    Address      string
    Port         int
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
    TLSCert      string
    TLSKey       string
}
```

## Usage

### Creating the Server

```go
import "dev.helix.code/internal/server"

config := &server.Config{
    Address:      "0.0.0.0",
    Port:         8080,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}

srv, err := server.New(config, db, authService)
if err != nil {
    log.Fatal(err)
}
```

### Starting the Server

```go
// Start HTTP server
err := srv.Start(ctx)

// Start HTTPS server
config.TLSCert = "/path/to/cert.pem"
config.TLSKey = "/path/to/key.pem"
err := srv.StartTLS(ctx)
```

### Graceful Shutdown

```go
// Setup signal handling
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

// Server shuts down gracefully on interrupt
err := srv.Start(ctx)
```

## API Endpoints

### Health

```
GET /health              - Server health check
GET /api/v1/health       - API health check
```

### Authentication

```
POST /api/v1/auth/login    - User login
POST /api/v1/auth/logout   - User logout
POST /api/v1/auth/refresh  - Refresh token
GET  /api/v1/auth/me       - Current user info
```

### Projects

```
GET    /api/v1/projects           - List projects
POST   /api/v1/projects           - Create project
GET    /api/v1/projects/:id       - Get project
PUT    /api/v1/projects/:id       - Update project
DELETE /api/v1/projects/:id       - Delete project
```

### Tasks

```
GET    /api/v1/tasks              - List tasks
POST   /api/v1/tasks              - Create task
GET    /api/v1/tasks/:id          - Get task
PUT    /api/v1/tasks/:id/status   - Update task status
DELETE /api/v1/tasks/:id          - Cancel task
```

### Workers

```
GET    /api/v1/workers            - List workers
POST   /api/v1/workers            - Add worker
GET    /api/v1/workers/:id        - Get worker
DELETE /api/v1/workers/:id        - Remove worker
GET    /api/v1/workers/:id/health - Worker health
```

### WebSocket

```
WS /api/v1/ws                     - WebSocket connection
```

## Middleware

### Authentication Middleware

```go
// Protected routes require valid JWT
router.Use(server.AuthMiddleware(authService))
```

### CORS Middleware

```go
// Configure CORS
corsConfig := &server.CORSConfig{
    AllowOrigins: []string{"*"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders: []string{"Authorization", "Content-Type"},
}
router.Use(server.CORSMiddleware(corsConfig))
```

### Logging Middleware

```go
// Request logging
router.Use(server.LoggingMiddleware(logger))
```

### Rate Limiting

```go
// Rate limit by IP
router.Use(server.RateLimitMiddleware(100, time.Minute))
```

## Error Handling

```go
// Standard error response format
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Code    int    `json:"code"`
}

// Return errors consistently
c.JSON(http.StatusBadRequest, &ErrorResponse{
    Error:   "validation_error",
    Message: "Invalid project name",
    Code:    400,
})
```

## Configuration

```yaml
server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  tls_cert: ""
  tls_key: ""
  cors:
    enabled: true
    allow_origins: ["*"]
```

## Testing

```bash
go test -v ./internal/server/...
```

## Notes

- Use HTTPS in production
- Enable CORS only for trusted origins in production
- Implement rate limiting to prevent abuse
- All routes except health require authentication
