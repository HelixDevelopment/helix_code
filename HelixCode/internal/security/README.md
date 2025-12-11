# Security Package

The `security` package provides security utilities and validation for the HelixCode platform.

## Overview

This package handles:
- Input validation and sanitization
- Path traversal prevention
- Command injection prevention
- Security headers
- Rate limiting
- API key management

## Key Types

### SecurityManager

```go
type SecurityManager struct {
    config    *Config
    validator *Validator
    sanitizer *Sanitizer
}
```

### Validator

```go
type Validator struct {
    rules map[string]ValidationRule
}
```

## Usage

### Input Validation

```go
import "dev.helix.code/internal/security"

manager := security.NewManager(config)

// Validate input
err := manager.ValidateInput(input, &security.ValidationRules{
    MaxLength: 1000,
    AllowedChars: security.AlphanumericWithSpaces,
    NoHTML: true,
})
```

### Path Validation

```go
// Validate file path (prevent traversal)
err := manager.ValidatePath(path, basePath)
if err != nil {
    // Path contains traversal attempt
}

// Safe path joining
safePath := manager.SafeJoinPath(basePath, userPath)
```

### Command Validation

```go
// Validate shell command
err := manager.ValidateCommand(cmd, &security.CommandRules{
    AllowedCommands: []string{"git", "go", "npm"},
    BlockedPatterns: []string{";", "&&", "||", "|", "`"},
})
```

### Input Sanitization

```go
// Sanitize HTML
clean := manager.SanitizeHTML(input)

// Sanitize for SQL (use prepared statements instead)
clean := manager.SanitizeSQL(input)

// Sanitize for shell
clean := manager.SanitizeShell(input)
```

### API Key Management

```go
// Generate API key
key, err := manager.GenerateAPIKey()

// Hash API key for storage
hash := manager.HashAPIKey(key)

// Verify API key
valid := manager.VerifyAPIKey(key, hash)
```

### Security Headers

```go
// Apply security headers to response
manager.ApplySecurityHeaders(w)

// Headers applied:
// - Content-Security-Policy
// - X-Content-Type-Options
// - X-Frame-Options
// - X-XSS-Protection
// - Strict-Transport-Security
```

### Rate Limiting

```go
// Create rate limiter
limiter := security.NewRateLimiter(&security.RateLimitConfig{
    RequestsPerMinute: 60,
    BurstSize:        10,
})

// Check rate limit
if !limiter.Allow(clientIP) {
    return ErrRateLimited
}
```

## Validation Rules

### Common Rules

```go
// Email validation
err := manager.ValidateEmail(email)

// URL validation
err := manager.ValidateURL(url)

// UUID validation
err := manager.ValidateUUID(id)

// JSON validation
err := manager.ValidateJSON(data)
```

### Custom Rules

```go
// Define custom rule
rule := &security.ValidationRule{
    Pattern:   `^[a-z][a-z0-9-]{2,30}$`,
    Message:   "Invalid project name format",
    MaxLength: 31,
}

manager.AddRule("project_name", rule)
err := manager.Validate("project_name", projectName)
```

## Configuration

```yaml
security:
  input_validation:
    max_request_size: 10MB
    allowed_content_types: ["application/json", "multipart/form-data"]

  rate_limiting:
    enabled: true
    requests_per_minute: 60
    burst_size: 10

  headers:
    csp: "default-src 'self'"
    hsts: true
    hsts_max_age: 31536000
```

## Security Checks

```go
// Check for common vulnerabilities
issues := manager.SecurityAudit(request)

for _, issue := range issues {
    log.Warn("Security issue: %s", issue.Description)
}
```

## Testing

```bash
go test -v ./internal/security/...
```

## Notes

- Always validate user input
- Use prepared statements for SQL
- Sanitize output for display
- Apply security headers to all responses
- Implement rate limiting for public APIs
