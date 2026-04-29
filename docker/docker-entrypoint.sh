#!/bin/sh
set -e

# HelixCode Container Entrypoint
# Anti-bluff: This script MUST actually start services, not just echo

# Validate required environment variables
if [ -z "$HELIX_AUTH_JWT_SECRET" ]; then
    echo "ERROR: HELIX_AUTH_JWT_SECRET is required"
    exit 1
fi

# Wait for PostgreSQL if configured
if [ -n "$HELIX_DATABASE_URL" ]; then
    echo "Waiting for PostgreSQL..."
    until pg_isready -d "$HELIX_DATABASE_URL" 2>/dev/null; do
        sleep 1
    done
    echo "PostgreSQL is ready"
fi

# Wait for Redis if configured
if [ -n "$HELIX_REDIS_URL" ]; then
    echo "Waiting for Redis..."
    until redis-cli -u "$HELIX_REDIS_URL" ping 2>/dev/null | grep -q PONG; do
        sleep 1
    done
    echo "Redis is ready"
fi

# Run database migrations if available
if [ -f "./scripts/migrate.sh" ]; then
    echo "Running database migrations..."
    ./scripts/migrate.sh || true
fi

# Start the appropriate service based on HELIX_SERVICE_TYPE
case "${HELIX_SERVICE_TYPE:-server}" in
    server)
        echo "Starting HelixCode server..."
        exec ./bin/server
        ;;
    cli)
        echo "Starting HelixCode CLI..."
        exec ./bin/cli "$@"
        ;;
    worker)
        echo "Starting HelixCode worker..."
        exec ./bin/worker "$@"
        ;;
    *)
        echo "Unknown service type: $HELIX_SERVICE_TYPE"
        exit 1
        ;;
esac
