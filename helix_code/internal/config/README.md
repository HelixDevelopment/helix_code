# Config Package

The `config` package provides configuration management for the HelixCode platform using Viper.

## Overview

This package handles:
- Loading configuration from YAML files
- Environment variable overrides
- Configuration validation
- Dynamic configuration updates
- Default value management

## Key Types

### Config

The main configuration structure:

```go
type Config struct {
    Server       ServerConfig       `mapstructure:"server"`
    Database     DatabaseConfig     `mapstructure:"database"`
    Redis        RedisConfig        `mapstructure:"redis"`
    Auth         AuthConfig         `mapstructure:"auth"`
    LLM          LLMConfig          `mapstructure:"llm"`
    Workers      WorkersConfig      `mapstructure:"workers"`
    Tasks        TasksConfig        `mapstructure:"tasks"`
    Notifications NotificationConfig `mapstructure:"notifications"`
    Logging      LoggingConfig      `mapstructure:"logging"`
}
```

### ServerConfig

HTTP server configuration:

```go
type ServerConfig struct {
    Address      string        `mapstructure:"address"`
    Port         int           `mapstructure:"port"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
}
```

## Usage

### Loading Configuration

```go
import "dev.helix.code/internal/config"

// Load from default locations
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Load from specific file
cfg, err := config.LoadFromFile("/path/to/config.yaml")
```

### Accessing Configuration

```go
// Get server address
addr := cfg.Server.Address

// Get database connection string
dsn := cfg.Database.GetDSN()

// Check if Redis is enabled
if cfg.Redis.Enabled {
    // Initialize Redis
}
```

### Environment Variable Overrides

All configuration values can be overridden with environment variables:

| Config Key | Environment Variable |
|------------|---------------------|
| server.port | HELIX_SERVER_PORT |
| database.host | HELIX_DATABASE_HOST |
| database.password | HELIX_DATABASE_PASSWORD |
| auth.jwt_secret | HELIX_AUTH_JWT_SECRET |
| redis.host | HELIX_REDIS_HOST |

## Configuration File Locations

The package searches for configuration in these locations (in order):

1. Path specified via command-line flag
2. `./config/config.yaml`
3. `./config.yaml`
4. `$HOME/.config/helixcode/config.yaml`
5. `/etc/helixcode/config.yaml`

## Example Configuration

```yaml
server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: "localhost"
  port: 5432
  user: "helixcode"
  name: "helixcode"

redis:
  enabled: true
  host: "localhost"
  port: 6379

auth:
  jwt_secret: "${HELIX_AUTH_JWT_SECRET}"
  token_expiry: 24h

llm:
  selection:
    strategy: "performance"
    fallback_enabled: true

logging:
  level: "info"
  format: "json"
```

## Validation

The package validates configuration on load:

```go
func (c *Config) Validate() error {
    if c.Server.Port < 1 || c.Server.Port > 65535 {
        return errors.New("invalid server port")
    }
    // ... additional validations
}
```

## Testing

```bash
go test -v ./internal/config/...
```
