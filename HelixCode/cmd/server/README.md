# HelixCode Server

The HelixCode Server is the core backend component that provides REST API, WebSocket, and MCP (Model Context Protocol) endpoints for distributed AI-powered development workflows.

## Overview

The server provides:
- REST API for project, task, and worker management
- WebSocket endpoint for real-time communication and MCP
- LLM provider integration (local and cloud)
- SSH-based distributed worker management
- PostgreSQL-backed persistence
- Redis caching and real-time state

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     HelixCode Server                         │
├─────────────────────────────────────────────────────────────┤
│  HTTP Server (Gin)                                           │
│  ├── REST API (/api/v1/*)                                   │
│  ├── WebSocket (/ws)                                        │
│  └── Static Files (/)                                       │
├─────────────────────────────────────────────────────────────┤
│  Core Services                                               │
│  ├── Auth Service (JWT + Session)                           │
│  ├── Task Manager                                           │
│  ├── Worker Manager                                         │
│  ├── Project Manager                                        │
│  ├── Workflow Engine                                        │
│  └── LLM Provider Manager                                   │
├─────────────────────────────────────────────────────────────┤
│  Data Layer                                                  │
│  ├── PostgreSQL (persistence)                               │
│  └── Redis (caching, real-time)                             │
└─────────────────────────────────────────────────────────────┘
```

## Building

```bash
cd HelixCode

# Development build
make build
# Binary at: bin/helixcode

# Production build (cross-platform)
make prod
# Creates: bin/helixcode-linux, bin/helixcode-macos, bin/helixcode-windows.exe
```

## Configuration

### Configuration File

Create `config/config.yaml`:

```yaml
version: "1.0.0"

application:
  name: "helixcode"
  environment: "development"

server:
  address: "127.0.0.1"
  port: 8080
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 120
  shutdown_timeout: 15

database:
  host: "localhost"
  port: 5432
  user: "helixcode"
  password: "your_password"
  dbname: "helixcode"
  sslmode: "disable"

redis:
  enabled: true
  host: "localhost"
  port: 6379
  password: ""
  database: 0

auth:
  jwt_secret: "your-secure-jwt-secret-minimum-32-chars"
  token_expiry: 3600
  session_expiry: 86400
  bcrypt_cost: 12

logging:
  level: "info"

llm:
  default_provider: "ollama"
  default_model: "llama2"
  max_tokens: 4096
  temperature: 0.7

workers:
  health_check_interval: 30
  health_ttl: 90
  max_concurrent_tasks: 5

tasks:
  max_retries: 3
  checkpoint_interval: 300
  cleanup_interval: 3600
```

### Environment Variables

Environment variables override config file settings:

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIX_SERVER_ADDRESS` | Bind address | 127.0.0.1 |
| `HELIX_SERVER_PORT` | Server port | 8080 |
| `HELIX_DATABASE_HOST` | PostgreSQL host | localhost |
| `HELIX_DATABASE_PORT` | PostgreSQL port | 5432 |
| `HELIX_DATABASE_USER` | Database user | helixcode |
| `HELIX_DATABASE_PASSWORD` | Database password | (required) |
| `HELIX_DATABASE_NAME` | Database name | helixcode |
| `HELIX_REDIS_ENABLED` | Enable Redis | true |
| `HELIX_REDIS_HOST` | Redis host | localhost |
| `HELIX_REDIS_PORT` | Redis port | 6379 |
| `HELIX_AUTH_JWT_SECRET` | JWT signing secret | (required) |
| `HELIX_LOGGING_LEVEL` | Log level | info |

## Running

### Development Mode

```bash
make dev
# or
./bin/helixcode --config config/dev/config.yaml
```

### Production Mode

```bash
./bin/helixcode --config config/config.yaml
```

### Docker

```bash
docker build -t helixcode:latest .
docker run -p 8080:8080 -v $(pwd)/config:/app/config helixcode:latest
```

### Docker Compose

```bash
docker compose up -d
```

## API Endpoints

### Health & Status

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/server/info` | Server information |
| GET | `/api/v1/metrics` | System metrics |

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register user |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/logout` | Logout |
| POST | `/api/v1/auth/refresh` | Refresh token |

### Projects

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/projects` | List projects |
| POST | `/api/v1/projects` | Create project |
| GET | `/api/v1/projects/:id` | Get project |
| PUT | `/api/v1/projects/:id` | Update project |
| DELETE | `/api/v1/projects/:id` | Delete project |

### Tasks

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tasks` | List tasks |
| POST | `/api/v1/tasks` | Create task |
| GET | `/api/v1/tasks/:id` | Get task |
| PUT | `/api/v1/tasks/:id` | Update task |
| DELETE | `/api/v1/tasks/:id` | Delete task |

### Workers

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/workers` | List workers |
| GET | `/api/v1/workers/:id` | Get worker |

### Workflows

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/projects/:id/workflows/planning` | Execute planning |
| POST | `/api/v1/projects/:id/workflows/building` | Execute building |
| POST | `/api/v1/projects/:id/workflows/testing` | Execute testing |
| POST | `/api/v1/projects/:id/workflows/refactoring` | Execute refactoring |

### LLM

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/llm/providers` | List providers |
| GET | `/api/v1/llm/models` | List models |

### WebSocket

| Endpoint | Description |
|----------|-------------|
| `/ws` | WebSocket for MCP and real-time updates |

## Database Setup

### PostgreSQL

Create the database and user:

```sql
CREATE USER helixcode WITH PASSWORD 'your_password';
CREATE DATABASE helixcode OWNER helixcode;
GRANT ALL PRIVILEGES ON DATABASE helixcode TO helixcode;
```

The server automatically initializes the schema on first startup.

### Redis (Optional)

Redis is optional but recommended for:
- Session caching
- Real-time worker state
- Task queue caching

## Middleware

The server includes the following middleware:

- **CORS**: Cross-Origin Resource Sharing headers
- **Security**: X-Content-Type-Options, X-Frame-Options, HSTS
- **Authentication**: JWT token validation for protected routes
- **Logging**: Request/response logging
- **Recovery**: Panic recovery

## Health Checks

The `/health` endpoint checks:
1. Database connectivity
2. Redis connectivity (if enabled)

Returns:
- `200 OK` - All systems operational
- `503 Service Unavailable` - One or more systems down

## Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` signals for graceful shutdown:
1. Stops accepting new connections
2. Waits for in-flight requests (configurable timeout)
3. Closes database and Redis connections
4. Exits cleanly

## Monitoring

### Metrics Endpoint

`GET /api/v1/metrics` returns:
- Request count and latency
- Active connections
- Task statistics
- Worker statistics
- Memory usage

### Logging

Configure log level via `logging.level`:
- `debug` - Verbose debugging information
- `info` - Standard operational logs
- `warn` - Warnings and potential issues
- `error` - Errors only

## Troubleshooting

### Server won't start

1. Check configuration file syntax:
   ```bash
   ./bin/helixcode config validate
   ```

2. Verify database connectivity:
   ```bash
   psql -h localhost -U helixcode -d helixcode
   ```

3. Check port availability:
   ```bash
   lsof -i :8080
   ```

### Database connection errors

- Verify PostgreSQL is running
- Check credentials in config
- Ensure database exists and user has permissions

### Redis connection errors

- Verify Redis is running
- Check Redis password if authentication is enabled
- Set `redis.enabled: false` to disable Redis

### Authentication errors

- Ensure `auth.jwt_secret` is at least 32 characters
- Check token expiry settings
- Verify client is sending `Authorization: Bearer <token>` header

## Related Documentation

- [CLI Documentation](../cli/README.md)
- [API Reference](../../docs/COMPLETE_API_REFERENCE.md)
- [Configuration Guide](../../docs/CONFIGURATION.md)
- [Deployment Guide](../../docs/DEPLOYMENT.md)
