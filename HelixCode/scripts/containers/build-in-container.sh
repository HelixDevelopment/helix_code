#!/usr/bin/env bash
# ============================================================================
# HelixCode Containerized Build Script
# ============================================================================
# Authority: CONST-035 — All builds MUST be containerized.
#
# This script runs HelixCode builds inside a container so NO dependencies
# need to be installed on the host machine. The only host requirement is
# Docker (or Podman with docker-compatible CLI).
#
# The builder container includes:
#   - Go 1.24 toolchain
#   - gcc, g++, musl-dev (for CGO)
#   - git, bash, make, coreutils
#   - postgresql-client, redis
#   - docker-cli, docker-compose
#   - golangci-lint
#   - Plus a PostgreSQL 15 and Redis 7 sidecar for tests
#
# Usage:
#   ./scripts/containers/build-in-container.sh build
#   ./scripts/containers/build-in-container.sh test
#   ./scripts/containers/build-in-container.sh lint
#   ./scripts/containers/build-in-container.sh shell
# ============================================================================

set -euo pipefail

COMPOSE_FILE="docker-compose.builder.yml"
BUILDER_SERVICE="builder"

# Detect compose command (docker compose vs docker-compose)
if docker compose version &>/dev/null; then
    COMPOSE_CMD="docker compose"
elif docker-compose version &>/dev/null; then
    COMPOSE_CMD="docker-compose"
else
    echo "❌ ERROR: Neither 'docker compose' nor 'docker-compose' found."
    echo "   Please install Docker or Podman with compose support."
    exit 1
fi

# Change to project root (HelixCode/ subdirectory)
cd "$(dirname "$0")/../.."

run_in_builder() {
    local cmd="$1"
    echo "🐳 Running in builder container: $cmd"
    $COMPOSE_CMD -f "$COMPOSE_FILE" run --rm "$BUILDER_SERVICE" -c "$cmd"
}

case "${1:-build}" in
    build|b)
        echo "🔨 Containerized build..."
        run_in_builder "make build"
        ;;
    test|t)
        echo "🧪 Containerized tests..."
        run_in_builder "make test"
        ;;
    test-short|ts)
        echo "🧪 Containerized short tests..."
        run_in_builder "go test -short -timeout=120s ./..."
        ;;
    lint|l)
        echo "🔍 Containerized lint..."
        run_in_builder "make lint"
        ;;
    shell|sh)
        echo "🐚 Starting interactive builder shell..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" run --rm "$BUILDER_SERVICE"
        ;;
    dev-up)
        echo "🚀 Starting containerized dev environment..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" up -d postgres redis
        echo "✅ Dev environment ready. Run '$0 shell' to enter."
        ;;
    dev-down)
        echo "🛑 Stopping containerized dev environment..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" down
        ;;
    clean)
        echo "🧹 Cleaning builder environment..."
        $COMPOSE_CMD -f "$COMPOSE_FILE" down -v --remove-orphans
        ;;
    *)
        echo "Usage: $0 {build|test|test-short|lint|shell|dev-up|dev-down|clean}"
        echo ""
        echo "Commands:"
        echo "  build       - Build the application inside container"
        echo "  test        - Run all tests inside container"
        echo "  test-short  - Run short tests inside container"
        echo "  lint        - Run linter inside container"
        echo "  shell       - Interactive shell in builder container"
        echo "  dev-up      - Start postgres + redis for dev"
        echo "  dev-down    - Stop dev environment"
        echo "  clean       - Remove all builder containers/volumes"
        exit 1
        ;;
esac
