#!/bin/bash

# HelixCode Test Infrastructure Management Script
# Manages Docker/Podman containers for integration, E2E, and automation tests
#
# Usage:
#   ./scripts/test-infra.sh start           - Start all test services
#   ./scripts/test-infra.sh stop            - Stop all test services
#   ./scripts/test-infra.sh restart         - Restart all services
#   ./scripts/test-infra.sh status          - Show service status
#   ./scripts/test-infra.sh health          - Run health checks
#   ./scripts/test-infra.sh logs [service]  - Show logs
#   ./scripts/test-infra.sh clean           - Clean up volumes and data
#   ./scripts/test-infra.sh wait            - Wait for all services to be healthy
#   ./scripts/test-infra.sh pull            - Pull latest images
#   ./scripts/test-infra.sh monitoring      - Start with monitoring profile

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.test.yml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Service health check endpoints
declare -A HEALTH_ENDPOINTS=(
    ["postgres-test"]="pg_isready"
    ["redis-test"]="redis-cli"
    ["cognee-test"]="http://localhost:8001/health"
    ["chromadb-test"]="http://localhost:8002/api/v1/heartbeat"
    ["qdrant-test"]="http://localhost:6333/healthz"
    ["ollama-test"]="http://localhost:11434/api/tags"
    ["prometheus-test"]="http://localhost:9091/-/healthy"
    ["grafana-test"]="http://localhost:3001/api/health"
)

# Service ports for external access
declare -A SERVICE_PORTS=(
    ["postgres-test"]="5433"
    ["redis-test"]="6380"
    ["cognee-test"]="8001"
    ["chromadb-test"]="8002"
    ["qdrant-test"]="6333"
    ["ollama-test"]="11434"
    ["prometheus-test"]="9091"
    ["grafana-test"]="3001"
)

# Container names
declare -A CONTAINER_NAMES=(
    ["postgres-test"]="helixcode-postgres-test"
    ["redis-test"]="helixcode-redis-test"
    ["cognee-test"]="helixcode-cognee-test"
    ["chromadb-test"]="helixcode-chromadb-test"
    ["qdrant-test"]="helixcode-qdrant-test"
    ["ollama-test"]="helixcode-ollama-test"
    ["prometheus-test"]="helixcode-prometheus-test"
    ["grafana-test"]="helixcode-grafana-test"
)

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_header() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
    echo ""
}

# Detect container runtime (Docker or Podman)
detect_runtime() {
    if command -v podman &> /dev/null; then
        if command -v podman-compose &> /dev/null; then
            COMPOSE_CMD="podman-compose"
            CONTAINER_CMD="podman"
            log_info "Using Podman with podman-compose"
            return 0
        fi
    fi

    if command -v docker &> /dev/null; then
        if docker compose version &> /dev/null 2>&1; then
            COMPOSE_CMD="docker compose"
            CONTAINER_CMD="docker"
            log_info "Using Docker with docker compose"
            return 0
        elif command -v docker-compose &> /dev/null; then
            COMPOSE_CMD="docker-compose"
            CONTAINER_CMD="docker"
            log_info "Using Docker with docker-compose"
            return 0
        fi
    fi

    log_error "No container runtime found. Please install Docker or Podman."
    exit 1
}

# Check if compose file exists
check_compose_file() {
    if [ ! -f "$COMPOSE_FILE" ]; then
        log_error "Compose file not found: $COMPOSE_FILE"
        exit 1
    fi
}

# Create required configuration files
create_config_files() {
    local config_dir="$PROJECT_ROOT/config"

    # Create Prometheus config if it doesn't exist
    if [ ! -f "$config_dir/prometheus-test.yml" ]; then
        log_info "Creating Prometheus test configuration..."
        mkdir -p "$config_dir"
        cat > "$config_dir/prometheus-test.yml" << 'EOF'
# Prometheus Test Configuration
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'helixcode'
    static_configs:
      - targets: ['host.docker.internal:8080']
    metrics_path: /metrics
    scrape_interval: 10s

  - job_name: 'cognee'
    static_configs:
      - targets: ['cognee-test:8000']
    metrics_path: /metrics
    scrape_interval: 30s

  - job_name: 'qdrant'
    static_configs:
      - targets: ['qdrant-test:6333']
    metrics_path: /metrics
    scrape_interval: 30s
EOF
        log_success "Created Prometheus configuration"
    fi
}

# Pull latest images
pull_images() {
    log_header "Pulling Latest Images"
    $COMPOSE_CMD -f "$COMPOSE_FILE" pull
    log_success "All images pulled successfully"
}

# Start services
start_services() {
    local profile="${1:-}"

    log_header "Starting Test Infrastructure"

    create_config_files

    if [ "$profile" = "monitoring" ]; then
        log_info "Starting services with monitoring profile..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring up -d
    else
        log_info "Starting core services..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" up -d
    fi

    log_success "Services started"

    # Wait for services to be healthy
    wait_for_services
}

# Stop services
stop_services() {
    log_header "Stopping Test Infrastructure"

    $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring down

    log_success "All services stopped"
}

# Restart services
restart_services() {
    stop_services
    start_services "$1"
}

# Show service status
show_status() {
    log_header "Service Status"

    $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring ps -a

    echo ""
    log_info "Service Endpoints:"
    for service in "${!SERVICE_PORTS[@]}"; do
        local container="${CONTAINER_NAMES[$service]}"
        local port="${SERVICE_PORTS[$service]}"
        local status="$($CONTAINER_CMD inspect -f '{{.State.Status}}' "$container" 2>/dev/null || echo "not found")"

        if [ "$status" = "running" ]; then
            echo -e "  ${GREEN}●${NC} $service: localhost:$port"
        else
            echo -e "  ${RED}○${NC} $service: localhost:$port (${status})"
        fi
    done
}

# Check service health
check_postgres_health() {
    local container="${CONTAINER_NAMES["postgres-test"]}"
    $CONTAINER_CMD exec "$container" pg_isready -U helix_test -d helix_test &> /dev/null
}

check_redis_health() {
    local container="${CONTAINER_NAMES["redis-test"]}"
    $CONTAINER_CMD exec "$container" redis-cli -a test_redis_password_123 ping 2>/dev/null | grep -q PONG
}

check_http_health() {
    local url="$1"
    curl -sf "$url" &> /dev/null
}

# Run health checks
run_health_checks() {
    log_header "Health Checks"

    local all_healthy=true

    # PostgreSQL
    if check_postgres_health; then
        log_success "PostgreSQL: Healthy"
    else
        log_error "PostgreSQL: Unhealthy"
        all_healthy=false
    fi

    # Redis
    if check_redis_health; then
        log_success "Redis: Healthy"
    else
        log_error "Redis: Unhealthy"
        all_healthy=false
    fi

    # HTTP services
    local http_services=("cognee-test" "chromadb-test" "qdrant-test" "ollama-test")
    for service in "${http_services[@]}"; do
        local url="${HEALTH_ENDPOINTS[$service]}"
        if check_http_health "$url"; then
            log_success "$service: Healthy"
        else
            log_warning "$service: Unhealthy or starting"
            all_healthy=false
        fi
    done

    # Monitoring services (optional)
    local container="${CONTAINER_NAMES["prometheus-test"]}"
    if $CONTAINER_CMD inspect "$container" &> /dev/null; then
        if check_http_health "${HEALTH_ENDPOINTS["prometheus-test"]}"; then
            log_success "Prometheus: Healthy"
        else
            log_warning "Prometheus: Unhealthy"
        fi

        if check_http_health "${HEALTH_ENDPOINTS["grafana-test"]}"; then
            log_success "Grafana: Healthy"
        else
            log_warning "Grafana: Unhealthy"
        fi
    fi

    echo ""
    if [ "$all_healthy" = true ]; then
        log_success "All core services are healthy!"
        return 0
    else
        log_warning "Some services are not healthy yet"
        return 1
    fi
}

# Wait for all services to be healthy
wait_for_services() {
    log_header "Waiting for Services"

    local max_attempts=60
    local attempt=0
    local wait_interval=5

    log_info "Waiting for services to be healthy (max ${max_attempts} attempts)..."

    while [ $attempt -lt $max_attempts ]; do
        attempt=$((attempt + 1))

        echo -n "  Attempt $attempt/$max_attempts: "

        local all_ready=true

        # Check PostgreSQL
        if ! check_postgres_health; then
            all_ready=false
        fi

        # Check Redis
        if ! check_redis_health; then
            all_ready=false
        fi

        # Check core HTTP services (excluding Cognee which may take longer)
        if ! check_http_health "${HEALTH_ENDPOINTS["chromadb-test"]}"; then
            all_ready=false
        fi

        if ! check_http_health "${HEALTH_ENDPOINTS["qdrant-test"]}"; then
            all_ready=false
        fi

        if [ "$all_ready" = true ]; then
            echo -e "${GREEN}Ready!${NC}"
            log_success "Core services are ready"

            # Additional wait for Cognee and Ollama (they take longer)
            log_info "Waiting for additional services (Cognee, Ollama)..."
            local extra_attempts=0
            while [ $extra_attempts -lt 12 ]; do
                extra_attempts=$((extra_attempts + 1))

                local cognee_ready=false
                local ollama_ready=false

                if check_http_health "${HEALTH_ENDPOINTS["cognee-test"]}"; then
                    cognee_ready=true
                fi

                if check_http_health "${HEALTH_ENDPOINTS["ollama-test"]}"; then
                    ollama_ready=true
                fi

                if [ "$cognee_ready" = true ] && [ "$ollama_ready" = true ]; then
                    log_success "All services are ready!"
                    return 0
                fi

                sleep 5
            done

            log_warning "Some optional services may still be starting"
            return 0
        fi

        echo -e "${YELLOW}Waiting...${NC}"
        sleep $wait_interval
    done

    log_error "Timeout waiting for services to be ready"
    return 1
}

# Show logs
show_logs() {
    local service="$1"

    if [ -z "$service" ]; then
        log_header "All Service Logs"
        $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring logs --tail=100
    else
        log_header "Logs for $service"
        $COMPOSE_CMD -f "$COMPOSE_FILE" logs --tail=100 "$service"
    fi
}

# Follow logs
follow_logs() {
    local service="$1"

    if [ -z "$service" ]; then
        $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring logs -f
    else
        $COMPOSE_CMD -f "$COMPOSE_FILE" logs -f "$service"
    fi
}

# Clean up volumes and data
cleanup() {
    log_header "Cleaning Up Test Infrastructure"

    log_info "Stopping all services..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" --profile monitoring down -v

    log_info "Removing named volumes..."
    local volumes=(
        "helix-test-postgres-data"
        "helix-test-redis-data"
        "helix-test-cognee-data"
        "helix-test-chromadb-data"
        "helix-test-qdrant-data"
        "helix-test-ollama-data"
        "helix-test-prometheus-data"
        "helix-test-grafana-data"
    )

    for vol in "${volumes[@]}"; do
        $CONTAINER_CMD volume rm "$vol" 2>/dev/null && log_info "Removed volume: $vol" || true
    done

    log_info "Removing test network..."
    $CONTAINER_CMD network rm helix-test-network 2>/dev/null || true

    log_success "Cleanup completed"
}

# Export environment variables for tests
export_env() {
    log_header "Test Environment Variables"

    cat << EOF
# Add these to your test environment or source this output

# PostgreSQL
export HELIX_TEST_DB_HOST=localhost
export HELIX_TEST_DB_PORT=5433
export HELIX_TEST_DB_NAME=helix_test
export HELIX_TEST_DB_USER=helix_test
export HELIX_TEST_DB_PASSWORD=test_password_secure_123
export HELIX_TEST_DB_URL="postgresql://helix_test:test_password_secure_123@localhost:5433/helix_test?sslmode=disable"

# Redis
export HELIX_TEST_REDIS_HOST=localhost
export HELIX_TEST_REDIS_PORT=6380
export HELIX_TEST_REDIS_PASSWORD=test_redis_password_123
export HELIX_TEST_REDIS_URL="redis://:test_redis_password_123@localhost:6380"

# Cognee
export HELIX_TEST_COGNEE_HOST=localhost
export HELIX_TEST_COGNEE_PORT=8001
export HELIX_TEST_COGNEE_URL="http://localhost:8001"
export HELIX_TEST_COGNEE_API_KEY=test_cognee_key_123

# ChromaDB
export HELIX_TEST_CHROMADB_HOST=localhost
export HELIX_TEST_CHROMADB_PORT=8002
export HELIX_TEST_CHROMADB_URL="http://localhost:8002"

# Qdrant
export HELIX_TEST_QDRANT_HOST=localhost
export HELIX_TEST_QDRANT_PORT=6333
export HELIX_TEST_QDRANT_URL="http://localhost:6333"

# Ollama
export HELIX_TEST_OLLAMA_HOST=localhost
export HELIX_TEST_OLLAMA_PORT=11434
export HELIX_TEST_OLLAMA_URL="http://localhost:11434"

# Prometheus (if monitoring profile enabled)
export HELIX_TEST_PROMETHEUS_URL="http://localhost:9091"

# Grafana (if monitoring profile enabled)
export HELIX_TEST_GRAFANA_URL="http://localhost:3001"
export HELIX_TEST_GRAFANA_USER=admin
export HELIX_TEST_GRAFANA_PASSWORD=admin123
EOF
}

# Show usage
usage() {
    echo "HelixCode Test Infrastructure Management"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  start [monitoring]  Start test infrastructure"
    echo "                      Use 'monitoring' to include Prometheus/Grafana"
    echo "  stop                Stop all services"
    echo "  restart             Restart all services"
    echo "  status              Show service status"
    echo "  health              Run health checks"
    echo "  wait                Wait for all services to be healthy"
    echo "  logs [service]      Show logs (optionally for specific service)"
    echo "  follow [service]    Follow logs in real-time"
    echo "  clean               Remove all containers and volumes"
    echo "  pull                Pull latest images"
    echo "  env                 Show environment variables for tests"
    echo "  help                Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start            # Start core services"
    echo "  $0 start monitoring # Start with Prometheus/Grafana"
    echo "  $0 wait             # Wait for services to be ready"
    echo "  $0 logs cognee-test # Show Cognee logs"
    echo "  $0 env              # Show test environment variables"
    echo ""
    echo "Services:"
    echo "  postgres-test    - PostgreSQL 16 (port 5433)"
    echo "  redis-test       - Redis 7 (port 6380)"
    echo "  cognee-test      - Cognee AI (port 8001)"
    echo "  chromadb-test    - ChromaDB (port 8002)"
    echo "  qdrant-test      - Qdrant (ports 6333, 6334)"
    echo "  ollama-test      - Ollama (port 11434)"
    echo "  prometheus-test  - Prometheus (port 9091) [monitoring profile]"
    echo "  grafana-test     - Grafana (port 3001) [monitoring profile]"
    echo ""
}

# Main script logic
main() {
    detect_runtime
    check_compose_file

    case "${1:-help}" in
        "start")
            start_services "$2"
            ;;
        "stop")
            stop_services
            ;;
        "restart")
            restart_services "$2"
            ;;
        "status")
            show_status
            ;;
        "health")
            run_health_checks
            ;;
        "wait")
            wait_for_services
            ;;
        "logs")
            show_logs "$2"
            ;;
        "follow")
            follow_logs "$2"
            ;;
        "clean")
            cleanup
            ;;
        "pull")
            pull_images
            ;;
        "env")
            export_env
            ;;
        "help"|*)
            usage
            ;;
    esac
}

main "$@"
