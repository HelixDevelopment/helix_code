# HelixCode Configuration Documentation

## Overview

HelixCode uses a comprehensive configuration system that supports YAML configuration files, environment variables, and runtime configuration management. This guide covers all configuration options, their purposes, and how to set them up for different deployment scenarios.

## Configuration Sources

HelixCode loads configuration from multiple sources in the following priority order:

1. **Environment Variables** (highest priority)
2. **Configuration File** (`config.yaml`)
3. **Default Values** (lowest priority)

### Configuration File Locations

HelixCode searches for configuration files in the following locations:

```bash
# Primary locations (in order)
./config/config.yaml
./config.yaml
$HOME/.config/helixcode/config.yaml
/etc/helixcode/config.yaml

# Custom location via environment
export HELIX_CONFIG=/path/to/custom/config.yaml
```

## Core Configuration Sections

### Server Configuration

Controls the HTTP server behavior and network settings.

```yaml
server:
  # Network settings
  address: "0.0.0.0"          # Listen address (0.0.0.0 for all interfaces)
  port: 8080                   # Listen port

  # Timeout settings (in seconds)
  read_timeout: 30             # Maximum time to read request
  write_timeout: 30            # Maximum time to write response
  idle_timeout: 60             # Maximum idle connection time
  shutdown_timeout: 30         # Graceful shutdown timeout
```

**Environment Variables:**
```bash
HELIX_SERVER_ADDRESS=0.0.0.0
HELIX_SERVER_PORT=8080
HELIX_SERVER_READ_TIMEOUT=30
HELIX_SERVER_WRITE_TIMEOUT=30
HELIX_SERVER_IDLE_TIMEOUT=60
HELIX_SERVER_SHUTDOWN_TIMEOUT=30
```

### Database Configuration

PostgreSQL database connection settings.

```yaml
database:
  # Connection settings
  host: "localhost"            # Database host
  port: 5432                   # Database port
  user: "helixcode"            # Database user
  password: ""                 # Database password (use env var)
  dbname: "helixcode"          # Database name
  sslmode: "disable"           # SSL mode: disable, require, verify-ca, verify-full

  # Connection pool settings
  max_connections: 50          # Maximum connections
  min_connections: 5           # Minimum connections
  connection_timeout: "30s"    # Connection timeout
  connection_lifetime: "1h"    # Maximum connection lifetime
  max_idle_connections: 10     # Maximum idle connections
```

**Environment Variables:**
```bash
HELIX_DATABASE_HOST=localhost
HELIX_DATABASE_PORT=5432
HELIX_DATABASE_USER=helixcode
HELIX_DATABASE_PASSWORD=your-secure-password
HELIX_DATABASE_NAME=helixcode
HELIX_DATABASE_SSLMODE=disable
```

### Redis Configuration

Redis cache and session storage settings.

```yaml
redis:
  # Basic settings
  enabled: true                # Enable Redis
  host: "localhost"            # Redis host
  port: 6379                   # Redis port
  password: ""                 # Redis password (use env var)
  database: 0                  # Redis database number

  # Advanced settings
  max_retries: 3               # Connection retry attempts
  dial_timeout: "5s"           # Connection timeout
  read_timeout: "3s"           # Read timeout
  write_timeout: "3s"          # Write timeout
  pool_size: 10                # Connection pool size
  min_idle_conns: 2            # Minimum idle connections
```

**Environment Variables:**
```bash
HELIX_REDIS_ENABLED=true
HELIX_REDIS_HOST=localhost
HELIX_REDIS_PORT=6379
HELIX_REDIS_PASSWORD=your-redis-password
HELIX_REDIS_DATABASE=0
```

### Authentication Configuration

JWT and session authentication settings.

```yaml
auth:
  # JWT settings
  jwt_secret: ""               # JWT signing secret (REQUIRED, use env var)
  token_expiry: 86400          # Token expiry in seconds (24 hours)
  session_expiry: 604800       # Session expiry in seconds (7 days)

  # Password settings
  bcrypt_cost: 12              # Bcrypt cost factor (4-31)

  # RBAC settings
  rbac_enabled: true           # Enable role-based access control
  default_role: "user"         # Default user role

  # Multi-factor authentication
  mfa_enabled: false           # Enable MFA
  mfa_issuer: "HelixCode"      # MFA issuer name
```

**Environment Variables:**
```bash
HELIX_AUTH_JWT_SECRET=your-super-secure-jwt-secret-min-32-chars
HELIX_AUTH_TOKEN_EXPIRY=86400
HELIX_AUTH_SESSION_EXPIRY=604800
HELIX_AUTH_BCRYPT_COST=12
```

### Worker Configuration

Distributed worker pool settings.

```yaml
workers:
  # Health monitoring
  health_check_interval: 30    # Health check interval (seconds)
  health_ttl: 120              # Health TTL (seconds)
  max_concurrent_tasks: 10     # Maximum concurrent tasks per worker

  # Auto-scaling
  auto_scaling_enabled: true   # Enable auto-scaling
  min_workers: 1               # Minimum worker count
  max_workers: 50              # Maximum worker count
  scale_up_threshold: 0.8      # Scale up when utilization > 80%
  scale_down_threshold: 0.2    # Scale down when utilization < 20%

  # Resource limits
  cpu_limit: 4.0               # CPU cores per worker
  memory_limit: "8GB"          # Memory limit per worker
  disk_limit: "100GB"          # Disk limit per worker

  # Worker types
  specialized_workers:
    gpu:                        # GPU workers
      enabled: true
      gpu_memory: "16GB"
      cuda_version: "11.8"
    high_memory:                # High memory workers
      enabled: true
      memory_limit: "64GB"
```

### Task Configuration

Task execution and management settings.

```yaml
tasks:
  # Retry settings
  max_retries: 3               # Maximum retry attempts
  retry_delay: "5s"            # Delay between retries
  exponential_backoff: true    # Use exponential backoff

  # Checkpoint settings
  checkpoint_enabled: true     # Enable checkpointing
  checkpoint_interval: 300     # Checkpoint interval (seconds)
  checkpoint_storage: "redis"  # Checkpoint storage: redis, file, database

  # Cleanup settings
  cleanup_enabled: true        # Enable cleanup
  cleanup_interval: 3600       # Cleanup interval (seconds)
  max_age: "24h"               # Maximum task age

  # Priority settings
  priority_levels: 5           # Number of priority levels (1-10)
  default_priority: 3          # Default task priority

  # Resource allocation
  cpu_weight: 1.0              # CPU allocation weight
  memory_weight: 1.0           # Memory allocation weight
  io_weight: 1.0               # I/O allocation weight

  # Timeout settings
  execution_timeout: "1h"      # Maximum execution time
  queue_timeout: "10m"         # Maximum queue time
```

### LLM Configuration

AI model and provider settings.

```yaml
llm:
  # Default settings
  default_provider: "anthropic"  # Default provider
  default_model: "claude-3-sonnet"  # Default model
  max_tokens: 4096              # Maximum tokens per request
  temperature: 0.7              # Generation temperature (0.0-2.0)
  timeout: "30s"                # Request timeout

  # Provider configurations
  providers:
    # Local providers (no API keys required)
    local:
      enabled: true
      base_url: "http://localhost:11434"
      models: ["llama-3.2-3b", "codellama"]

    # Cloud providers (require API keys)
    anthropic:
      enabled: true
      api_key: ""               # Set via ANTHROPIC_API_KEY
      models: ["claude-3-sonnet", "claude-3-haiku"]
      max_tokens: 200000
      timeout: "60s"

    openai:
      enabled: true
      api_key: ""               # Set via OPENAI_API_KEY
      models: ["gpt-4", "gpt-3.5-turbo"]
      organization: ""          # Optional organization ID

    gemini:
      enabled: false
      api_key: ""               # Set via GEMINI_API_KEY
      models: ["gemini-pro", "gemini-pro-vision"]

    # Free providers
    xai:
      enabled: true
      models: ["grok-3-fast-beta"]

    openrouter:
      enabled: true
      api_key: ""               # Set via OPENROUTER_API_KEY
      models: ["auto"]          # Auto-select best model

  # Request settings
  retry_attempts: 3             # Retry failed requests
  retry_delay: "1s"             # Delay between retries
  rate_limiting: true           # Enable rate limiting

  # Response caching
  caching_enabled: true         # Enable response caching
  cache_ttl: "1h"               # Cache TTL
  cache_size: 1000              # Maximum cache entries

  # Fallback settings
  fallback_enabled: true        # Enable provider fallback
  fallback_providers: ["openai", "xai", "local"]
```

**Environment Variables for LLM Providers:**
```bash
# Required for premium providers
ANTHROPIC_API_KEY=sk-ant-your-key
OPENAI_API_KEY=sk-your-openai-key
GEMINI_API_KEY=your-gemini-key

# Optional for free providers
XAI_API_KEY=xai-your-key
OPENROUTER_API_KEY=sk-or-your-key
GITHUB_TOKEN=ghp_your-github-token
```

## LLMsVerifier Configuration

The LLMsVerifier subsystem is the **single source of truth** for all model and provider metadata (CONST-036). When enabled, HelixCode fetches live model data from the verifier service instead of using hardcoded lists.

```yaml
verifier:
  enabled: false              # Master enable/disable
  mode: remote                # "remote" (REST API) or "embedded"
  endpoint: http://localhost:8081  # Verifier REST API URL
  api_key: ""                 # Optional authentication key
  timeout: 30s                # Request timeout
  cache_ttl: 5m               # In-memory cache TTL
  polling_interval: 60s       # Background poll interval

  scoring:
    weights:
      code_capability: 0.40   # Weight for code generation performance
      responsiveness: 0.20    # Weight for latency
      reliability: 0.20       # Weight for uptime/consistency
      feature_richness: 0.15  # Weight for feature breadth
      value_proposition: 0.05 # Weight for price/performance
    min_acceptable_score: 6.0 # Minimum score to include model

  health:
    failure_threshold: 5      # Failures before opening circuit
    recovery_threshold: 3     # Successes before closing circuit
    circuit_breaker:
      enabled: true
      half_open_timeout: 60s  # Time in Open state before retry

  events:
    enabled: true             # Enable change event publishing
    websocket: false          # WebSocket real-time events
    websocket_path: /ws/verifier/events
```

**Environment Variables:**
```bash
HELIX_VERIFIER_ENABLED=true
HELIX_VERIFIER_ENDPOINT=http://localhost:8081
HELIX_VERIFIER_API_KEY=
HELIX_VERIFIER_TIMEOUT=30s
HELIX_VERIFIER_CACHE_TTL=5m
HELIX_VERIFIER_POLLING_INTERVAL=60s
HELIX_VERIFIER_MIN_SCORE=6.0
```

**Provider API Keys** (used by verifier for live discovery):
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
DEEPSEEK_API_KEY=...
GROQ_API_KEY=...
MISTRAL_API_KEY=...
XAI_API_KEY=...
OPENROUTER_API_KEY=...
```

### Verifier Fallback Models

When the verifier is unavailable, HelixCode uses a constitutional fallback list of 7 verified models:

| Model | Provider | Context | Score | Tier |
|-------|----------|---------|-------|------|
| Llama 3.2 3B | Ollama | 128K | 6.0 | 3 |
| GPT-4o | OpenAI | 128K | 9.1 | 1 |
| Claude 3.5 Sonnet | Anthropic | 200K | 8.9 | 1 |
| Mistral Large | Mistral | 128K | 7.8 | 2 |
| Gemini 2.5 Pro | Gemini | 1M | 8.7 | 1 |
| DeepSeek Chat | DeepSeek | 64K | 8.3 | 2 |
| Grok-3 Fast Beta | xAI | 128K | 8.0 | 1 |

## Memory Provider Configurations

### Cognee Configuration

Advanced memory and knowledge graph system.

```yaml
cognee:
  # Basic settings
  enabled: true
  auto_start: true
  host: "localhost"
  port: 8001
  mode: "local"                # local, cloud, hybrid

  # Remote API (for cloud/hybrid mode)
  remote_api:
    service_endpoint: "https://api.cognee.ai"
    api_key: "your-cognee-api-key"
    timeout: "30s"

  # Dynamic configuration
  dynamic_config: true

  # Repository settings (for local mode)
  source: "https://github.com/cognee-ai/cognee"
  branch: "main"
  build_path: "./build"

  # Optimization settings
  optimization:
    host_aware: true
    cpu_optimization: true
    gpu_optimization: false
    memory_optimization: true
    host_specific:
      gpu_memory: "16GB"
      cuda_version: "11.8"

  # Feature settings
  features:
    knowledge_graph: true
    semantic_search: true
    real_time_processing: true
    multi_modal_support: true
    graph_analytics: true
    advanced_insights: true
    auto_optimization: true

  # Provider integrations
  providers:
    openai:
      enabled: true
      integration: "embeddings"
      priority: 1
      features: ["embeddings", "completion"]
    anthropic:
      enabled: true
      integration: "reasoning"
      priority: 2
      features: ["analysis", "reasoning"]

  # API configuration
  api:
    enabled: true
    host: "localhost"
    port: 8001
    cors_enabled: true
    auth_required: false
    rate_limiting: true
    request_timeout: "30s"
    max_request_size: "10MB"

  # Performance configuration
  performance:
    max_concurrent_requests: 10
    worker_pool_size: 4
    cache_enabled: true
    cache_size: "1GB"
    metrics_enabled: true
    profiling_enabled: false

  # Cache configuration
  cache:
    enabled: true
    type: "redis"              # redis, memory, disk
    ttl: "1h"
    max_size: "2GB"
    compression: true

  # Monitoring configuration
  monitoring:
    enabled: true
    metrics_endpoint: "/metrics"
    health_endpoint: "/health"
    log_level: "info"
    tracing_enabled: false
```

### Other Memory Providers

```yaml
providers:
  # Mem0 - Advanced memory management
  mem0:
    api_key: "your-mem0-key"
    base_url: "https://api.mem0.ai"
    timeout: "30s"

  # Zep - Long-term memory
  zep:
    api_key: "your-zep-key"
    base_url: "https://api.zep.ai"
    collection: "helixcode"

  # Memonto - Knowledge graphs
  memonto:
    api_key: "your-memonto-key"
    base_url: "https://api.memonto.ai"
    graph_id: "helixcode-graph"

  # BaseAI - Comprehensive memory
  baseai:
    api_key: "your-baseai-key"
    base_url: "https://api.baseai.ai"
    project_id: "helixcode"
```

## Application Configuration

### Basic Application Settings

```yaml
application:
  # Basic information
  name: "HelixCode"
  version: "1.0.0"
  description: "Enterprise AI Development Platform"
  environment: "production"     # development, staging, production

  # Workspace settings
  workspace:
    auto_save: true
    default_path: "./workspace"
    auto_save_interval: 300     # seconds
    backup_enabled: true
    backup_location: "./backups"
    backup_retention: 30        # days

  # Session management
  session:
    timeout: 3600               # 1 hour
    auto_save: true
    max_history: 1000
    persist_context: true
    context_retention: 7        # days
    max_history_size: "100MB"
    auto_resume: true

    # Context compression
    context_compression:
      enabled: true
      threshold: 1000           # Compress after 1000 items
      strategy: "gzip"
      compression_ratio: 0.7
      retention_policy: "lru"

  # Logging configuration
  logging:
    level: "info"               # debug, info, warn, error
    format: "json"              # json, text, logfmt
    output: "stdout"            # stdout, stderr, file
    file:
      path: "/var/log/helixcode/app.log"
      max_size: "100MB"
      max_age: "30d"
      max_backups: 10
      compress: true

  # Telemetry (optional)
  telemetry:
    enabled: false
    level: "info"
    data_retention: 30          # days
    metrics:
      enabled: true
      interval: "30s"
    tracing:
      enabled: false
      sampling_rate: 0.1
```

## Performance Configuration

### CPU and Memory Optimization

```yaml
performance:
  # CPU optimization
  cpu_optimization: true
  target_cpu_utilization: 70.0
  max_goroutines: 10000
  goroutine_pool_size: 100

  # Memory optimization
  memory_optimization: true
  target_memory_usage: 1073741824  # 1GB in bytes
  gc_target_percentage: 100
  max_heap_size: "2GB"

  # Garbage collection tuning
  garbage_collection:
    enabled: true
    target_pause_time: "10ms"
    max_pause_time: "50ms"

  # Concurrency optimization
  concurrency_optimization: true
  max_concurrent_requests: 1000
  worker_pool_size: 50

  # Cache optimization
  cache_optimization: true
  cache_size: 10000
  cache_ttl: "1h"
  min_cache_hit_rate: 0.95

  # Network optimization
  network_optimization: true
  connection_pool_size: 100
  connection_timeout: "30s"
  keep_alive: true

  # Database optimization
  database_optimization: true
  max_db_connections: 50
  db_connection_timeout: "30s"
  query_timeout: "30s"

  # Worker optimization
  worker_optimization: true
  worker_cpu_affinity: true
  worker_scaling_enabled: true
  min_workers: 2
  max_workers: 20

  # LLM optimization
  llm_optimization: true
  request_batching: true
  batch_size: 10
  response_caching: true
  cache_strategy: "lru"
```

## Security Configuration

### Authentication and Authorization

```yaml
security:
  # Zero-tolerance policy
  zero_tolerance: true
  max_critical_issues: 0
  max_high_issues: 0

  # Scanning configuration
  scanning:
    enabled: true
    interval: "24h"
    timeout: "1h"
    sonar_host: "http://localhost:9000"
    snyk_token: ""             # Set via SNYK_TOKEN

  # Input validation
  input_validation:
    enabled: true
    max_request_size: "10MB"
    allowed_content_types:
      - "application/json"
      - "multipart/form-data"
    sanitize_html: true
    sql_injection_protection: true
    xss_protection: true

  # Rate limiting
  rate_limiting:
    enabled: true
    requests_per_minute: 60
    burst_size: 10
    cleanup_interval: "5m"

  # Security headers
  headers:
    enabled: true
    csp: "default-src 'self'; script-src 'self' 'unsafe-inline'"
    hsts:
      enabled: true
      max_age: 31536000
      include_subdomains: true
    x_frame_options: "DENY"
    x_content_type_options: "nosniff"
    referrer_policy: "strict-origin-when-cross-origin"

  # Encryption
  encryption:
    enabled: true
    algorithm: "AES-256-GCM"
    key_rotation: "30d"

  # Audit logging
  audit:
    enabled: true
    log_file: "/var/log/helixcode/audit.log"
    max_size: "500MB"
    retention: "1y"
    compress: true
```

## Notification Configuration

```yaml
notifications:
  # Slack integration
  slack:
    enabled: true
    webhook_url: ""            # Set via HELIX_SLACK_WEBHOOK_URL
    channel: "#devops"
    username: "HelixCode"
    icon_emoji: ":robot:"

  # Email notifications
  email:
    enabled: true
    smtp_server: "smtp.gmail.com"
    smtp_port: 587
    username: ""               # Set via HELIX_EMAIL_USERNAME
    password: ""               # Set via HELIX_EMAIL_PASSWORD
    from_address: "noreply@helixcode.dev"
    tls: true

  # Discord integration
  discord:
    enabled: false
    webhook_url: ""            # Set via HELIX_DISCORD_WEBHOOK_URL
    username: "HelixCode"
    avatar_url: ""

  # Telegram bot
  telegram:
    enabled: false
    bot_token: ""              # Set via HELIX_TELEGRAM_BOT_TOKEN
    chat_id: ""                # Set via HELIX_TELEGRAM_CHAT_ID

  # Webhook notifications
  webhook:
    enabled: false
    url: "https://api.example.com/webhooks/helixcode"
    method: "POST"
    headers:
      Authorization: "Bearer your-token"
      Content-Type: "application/json"
    timeout: "10s"
    retry_attempts: 3
```

## Backup and Recovery Configuration

```yaml
backup:
  # Basic settings
  enabled: true
  schedule: "0 2 * * *"        # Daily at 2 AM
  timeout: "2h"
  compression: true
  encryption: true

  # Storage configuration
  storage:
    type: "s3"                 # s3, gcs, azure, local
    bucket: "helixcode-backups"
    region: "us-east-1"
    access_key: ""             # Set via AWS_ACCESS_KEY_ID
    secret_key: ""             # Set via AWS_SECRET_ACCESS_KEY
    path: "/backups"           # For local storage

  # Retention policy
  retention:
    daily: 7                   # Keep 7 daily backups
    weekly: 4                  # Keep 4 weekly backups
    monthly: 12                # Keep 12 monthly backups
    yearly: 5                  # Keep 5 yearly backups

  # Backup contents
  include:
    - database: true
    - files: true
    - configuration: true
    - logs: false
  exclude:
    - "*/tmp/*"
    - "*/cache/*"
    - "*.log"

  # Verification
  verification:
    enabled: true
    integrity_check: true
    restore_test: false       # Dangerous in production
```

## Monitoring Configuration

```yaml
monitoring:
  # Basic settings
  enabled: true
  interval: "30s"
  timeout: "10s"

  # Prometheus integration
  prometheus:
    enabled: true
    path: "/metrics"
    namespace: "helixcode"
    subsystem: "app"

  # Health checks
  health:
    enabled: true
    path: "/health"
    database_check: true
    redis_check: true
    worker_check: true
    llm_check: true

  # Metrics collection
  metrics:
    system: true              # CPU, memory, disk
    application: true         # Requests, errors, latency
    business: true            # Tasks, users, projects
    custom: true              # Application-specific metrics

  # Alerting
  alerts:
    enabled: true
    rules:
      - name: "high_cpu"
        condition: "cpu_usage_percent > 80"
        severity: "warning"
        cooldown: "5m"
      - name: "high_memory"
        condition: "memory_usage_percent > 90"
        severity: "critical"
        cooldown: "2m"
      - name: "db_connection_error"
        condition: "db_connections_failed > 0"
        severity: "error"
        cooldown: "1m"

  # Logging
  logging:
    level: "info"
    format: "json"
    outputs:
      - type: "stdout"
      - type: "file"
        path: "/var/log/helixcode/monitoring.log"
        max_size: "100MB"
        max_files: 5
```

## Configuration Examples

### Development Configuration

```yaml
# config/development.yaml
version: "1.0.0"
application:
  environment: "development"
  logging:
    level: "debug"
    format: "text"

server:
  port: 8080
  read_timeout: 60
  write_timeout: 60

database:
  host: "localhost"
  sslmode: "disable"

redis:
  enabled: false

auth:
  jwt_secret: "dev-secret-key-change-in-production"
  token_expiry: 3600          # 1 hour for development

performance:
  cpu_optimization: false
  memory_optimization: false

security:
  scanning:
    enabled: false
  zero_tolerance: false
```

### Production Configuration

```yaml
# config/production.yaml
version: "1.0.0"
application:
  environment: "production"
  logging:
    level: "info"
    format: "json"
    output: "file"
    file:
      path: "/var/log/helixcode/app.log"

server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 30
  write_timeout: 30
  shutdown_timeout: 60

database:
  host: "postgres-prod"
  sslmode: "require"
  max_connections: 100
  min_connections: 10

redis:
  enabled: true
  host: "redis-prod"
  pool_size: 50

auth:
  jwt_secret: "${HELIX_AUTH_JWT_SECRET}"
  token_expiry: 7200          # 2 hours
  session_expiry: 604800      # 7 days

performance:
  cpu_optimization: true
  memory_optimization: true
  target_cpu_utilization: 70.0
  target_memory_usage: 2147483648  # 2GB

security:
  scanning:
    enabled: true
    zero_tolerance: true
  rate_limiting:
    enabled: true
    requests_per_minute: 1000

backup:
  enabled: true
  schedule: "0 2 * * *"
  storage:
    type: "s3"
    bucket: "helixcode-prod-backups"

monitoring:
  enabled: true
  prometheus:
    enabled: true
```

### High-Availability Configuration

```yaml
# config/ha.yaml
version: "1.0.0"
application:
  environment: "production"

# Load balancer configuration
server:
  address: "0.0.0.0"
  port: 8080
  health_check_path: "/health"

# Database cluster
database:
  host: "postgres-cluster"
  sslmode: "require"
  max_connections: 200
  connection_timeout: "10s"

# Redis cluster
redis:
  enabled: true
  cluster:
    enabled: true
    addresses:
      - "redis-1:6379"
      - "redis-2:6379"
      - "redis-3:6379"

# Distributed workers
workers:
  auto_scaling_enabled: true
  min_workers: 10
  max_workers: 100
  health_check_interval: 15

# High availability settings
ha:
  enabled: true
  leader_election: true
  session_store: "redis"
  lock_timeout: "30s"
  failover_timeout: "60s"

# Monitoring for HA
monitoring:
  alerts:
    - name: "leader_down"
      condition: "leader_election_status != 'leader'"
      severity: "critical"
    - name: "node_unhealthy"
      condition: "node_health_status != 'healthy'"
      severity: "warning"
```

## Configuration Management

### Using the Configuration CLI

```bash
# Show current configuration
helix-config show

# Get specific value
helix-config get server.port

# Set configuration value
helix-config set server.port 9090

# Validate configuration
helix-config validate

# Export configuration
helix-config export config-backup.yaml

# Import configuration
helix-config import new-config.yaml

# Create backup
helix-config backup pre-deploy-$(date +%Y%m%d)

# Watch for changes
helix-config watch --command="systemctl reload helixcode"
```

### Environment Variable Overrides

```bash
# Override database settings
export HELIX_DATABASE_HOST=prod-db.example.com
export HELIX_DATABASE_PASSWORD=secure-password

# Override Redis settings
export HELIX_REDIS_ENABLED=true
export HELIX_REDIS_HOST=redis-cluster

# Override authentication
export HELIX_AUTH_JWT_SECRET=your-production-jwt-secret

# Override LLM providers
export ANTHROPIC_API_KEY=sk-ant-production-key
export OPENAI_API_KEY=sk-production-openai-key
```

### Configuration Validation

```bash
# Validate current configuration
helix-config validate

# Validate with strict mode
helix-config validate --strict

# Check for deprecated settings
helix-config validate --check-deprecated

# Validate against schema
helix-config validate --schema config-schema.json
```

### Configuration Migration

```bash
# Migrate from v0.9 to v1.0
helix-config migrate --from=0.9.0 --to=1.0.0

# Preview migration changes
helix-config migrate --dry-run

# Migrate with backup
helix-config migrate --backup
```

## Troubleshooting Configuration Issues

### Common Configuration Problems

#### Configuration File Not Found
```bash
# Check file locations
ls -la config/config.yaml
ls -la $HOME/.config/helixcode/config.yaml

# Create default configuration
helix-config create-default /path/to/config.yaml

# Set custom config path
export HELIX_CONFIG=/path/to/config.yaml
```

#### Invalid YAML Syntax
```bash
# Validate YAML syntax
yamllint config/config.yaml

# Check for tabs vs spaces
cat -A config/config.yaml | grep -P '\t'

# Use YAML validator
python3 -c "import yaml; yaml.safe_load(open('config/config.yaml'))"
```

#### Environment Variables Not Set
```bash
# Check required environment variables
echo $HELIX_AUTH_JWT_SECRET | wc -c  # Should be > 32
echo $HELIX_DATABASE_PASSWORD
echo $HELIX_REDIS_PASSWORD

# Load environment file
source .env.production

# Check variable precedence
helix-config show --show-sources
```

#### Permission Issues
```bash
# Check file permissions
ls -la config/config.yaml

# Fix permissions
chmod 600 config/config.yaml

# Check directory permissions
ls -ld config/
chmod 755 config/
```

#### Configuration Conflicts
```bash
# Check for conflicting settings
helix-config validate --check-conflicts

# Show configuration sources
helix-config show --show-precedence

# Resolve conflicts
helix-config set conflicting.key resolved-value
```

This comprehensive configuration documentation covers all aspects of HelixCode configuration management, from basic setup to advanced enterprise deployments. The configuration system is designed to be flexible, secure, and production-ready.</content>
<parameter name="filePath">docs/COMPLETE_CONFIGURATION_DOCUMENTATION.md

## Sources verified 2026-05-29: https://www.postgresql.org/support/versioning/ , https://github.com/redis/redis/releases , https://go.dev/dl/ , https://platform.claude.com/docs/en/docs/about-claude/models/overview

Verified against latest official sources on 2026-05-29. PostgreSQL 15 remains a supported major (latest minor 15.18; PG 18 newest) — the doc's PostgreSQL connection settings are version-agnostic and compatible with the project's 15+ pin (CLAUDE.md §3.1). Redis config examples are compatible with Redis 7+ (latest stable 8.8.0). Default Redis port 6379 and database-number semantics confirmed current.

Negative findings: the `llm.providers` example config IDs (`default_model: claude-3-sonnet`, `models: [claude-3-sonnet, claude-3-haiku]`, `models: [llama-3.2-3b, codellama]`) reference the now-retired Claude 3 family per Anthropic's current model overview. Per CONST-036 these are placeholders only — model/provider metadata is LLMsVerifier-sourced at runtime, never hardcoded. Operators should populate from `helixcode llm models list`. Left unmodified to avoid guessing live IDs. OpenAI docs returned HTTP 403 to automated fetch.
