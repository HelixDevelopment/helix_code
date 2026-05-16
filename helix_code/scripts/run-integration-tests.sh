#!/bin/bash

# HelixCode Integration Test Runner
# Automatically starts test infrastructure before running integration tests
#
# Usage:
#   ./scripts/run-integration-tests.sh                  - Run all integration tests
#   ./scripts/run-integration-tests.sh -v               - Run with verbose output
#   ./scripts/run-integration-tests.sh -cover           - Run with coverage
#   ./scripts/run-integration-tests.sh -run TestName    - Run specific test
#   ./scripts/run-integration-tests.sh --no-infra       - Skip infrastructure setup

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
INFRA_SCRIPT="$SCRIPT_DIR/test-infra.sh"
COVERAGE_FILE="$PROJECT_ROOT/coverage-integration.out"
COVERAGE_HTML="$PROJECT_ROOT/coverage-integration.html"

# Flags
START_INFRA=true
STOP_INFRA=false
VERBOSE=false
COVERAGE=false
TEST_ARGS=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-infra)
            START_INFRA=false
            shift
            ;;
        --stop-infra)
            STOP_INFRA=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            TEST_ARGS="$TEST_ARGS -v"
            shift
            ;;
        -cover|--cover)
            COVERAGE=true
            TEST_ARGS="$TEST_ARGS -coverprofile=$COVERAGE_FILE"
            shift
            ;;
        -run)
            TEST_ARGS="$TEST_ARGS -run $2"
            shift 2
            ;;
        -timeout)
            TEST_ARGS="$TEST_ARGS -timeout $2"
            shift 2
            ;;
        *)
            TEST_ARGS="$TEST_ARGS $1"
            shift
            ;;
    esac
done

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
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

cleanup() {
    if [ "$STOP_INFRA" = true ]; then
        log_info "Stopping test infrastructure..."
        "$INFRA_SCRIPT" stop || true
    fi
}

trap cleanup EXIT

log_header "HelixCode Integration Tests"

# Check prerequisites
if ! command -v go &> /dev/null; then
    log_error "Go is not installed"
    exit 1
fi

log_info "Go version: $(go version)"

# Start infrastructure if requested
if [ "$START_INFRA" = true ]; then
    if [ -x "$INFRA_SCRIPT" ]; then
        log_info "Starting test infrastructure..."

        # Check if already running
        if "$INFRA_SCRIPT" health &> /dev/null; then
            log_success "Infrastructure already running"
        else
            "$INFRA_SCRIPT" start

            log_info "Waiting for infrastructure to be ready..."
            "$INFRA_SCRIPT" wait
        fi
    else
        log_error "Infrastructure script not found: $INFRA_SCRIPT"
        log_info "Running tests without infrastructure (some tests may be skipped)"
    fi
fi

# Export environment variables for tests
export HELIX_TEST_INFRA=true
export HELIX_TEST_DB_HOST=localhost
export HELIX_TEST_DB_PORT=5433
export HELIX_TEST_DB_NAME=helix_test
export HELIX_TEST_DB_USER=helix_test
export HELIX_TEST_DB_PASSWORD=test_password_secure_123
export HELIX_TEST_REDIS_HOST=localhost
export HELIX_TEST_REDIS_PORT=6380
export HELIX_TEST_REDIS_PASSWORD=test_redis_password_123
export HELIX_TEST_COGNEE_HOST=localhost
export HELIX_TEST_COGNEE_PORT=8001
export HELIX_TEST_COGNEE_URL="http://localhost:8001"
export HELIX_TEST_COGNEE_API_KEY=test_cognee_key_123
export HELIX_TEST_CHROMADB_HOST=localhost
export HELIX_TEST_CHROMADB_PORT=8002
export HELIX_TEST_CHROMADB_URL="http://localhost:8002"
export HELIX_TEST_QDRANT_HOST=localhost
export HELIX_TEST_QDRANT_PORT=6333
export HELIX_TEST_QDRANT_URL="http://localhost:6333"
export HELIX_TEST_OLLAMA_HOST=localhost
export HELIX_TEST_OLLAMA_PORT=11434
export HELIX_TEST_OLLAMA_URL="http://localhost:11434"

log_header "Running Integration Tests"

cd "$PROJECT_ROOT"

# Set default timeout if not provided
if [[ ! "$TEST_ARGS" =~ "-timeout" ]]; then
    TEST_ARGS="$TEST_ARGS -timeout 10m"
fi

# Run tests
log_info "Running: go test $TEST_ARGS ./tests/integration/..."

if [ "$VERBOSE" = true ]; then
    go test $TEST_ARGS ./tests/integration/...
else
    go test $TEST_ARGS ./tests/integration/... 2>&1 | tee test-output.log

    # Parse results
    PASSED=$(grep -c "^--- PASS" test-output.log 2>/dev/null || echo "0")
    FAILED=$(grep -c "^--- FAIL" test-output.log 2>/dev/null || echo "0")
    SKIPPED=$(grep -c "^--- SKIP" test-output.log 2>/dev/null || echo "0")

    rm -f test-output.log
fi

# Generate coverage report if requested
if [ "$COVERAGE" = true ] && [ -f "$COVERAGE_FILE" ]; then
    log_info "Generating coverage report..."
    go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"
    log_success "Coverage report: $COVERAGE_HTML"

    # Show coverage summary
    go tool cover -func="$COVERAGE_FILE" | tail -1
fi

log_header "Test Results"

if [ "$VERBOSE" = false ]; then
    echo "  Passed:  $PASSED"
    echo "  Failed:  $FAILED"
    echo "  Skipped: $SKIPPED"
fi

if [ "${FAILED:-0}" -gt 0 ]; then
    log_error "Some tests failed"
    exit 1
else
    log_success "All tests passed!"
fi
