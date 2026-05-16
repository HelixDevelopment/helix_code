#!/bin/bash

# 🧪 HelixCode Comprehensive Test Suite Runner
# This script runs all test types: Unit, Integration, E2E, and Automation tests

set -e

echo "🌀 Starting HelixCode Comprehensive Test Suite..."
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT="30m"
COVERAGE_THRESHOLD=80
PARALLEL_TESTS=4

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    print_status $BLUE "🔍 Checking prerequisites..."
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        print_status $RED "❌ Go is not installed"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.21"
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_status $RED "❌ Go version $GO_VERSION is less than required $REQUIRED_VERSION"
        exit 1
    fi
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_status $YELLOW "⚠️  Docker not found - some integration tests will be skipped"
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_status $YELLOW "⚠️  Docker Compose not found - some integration tests will be skipped"
    fi
    
    print_status $GREEN "✅ Prerequisites check passed"
}

# Function to setup test environment
setup_test_environment() {
    print_status $BLUE "🔧 Setting up test environment..."
    
    # Generate SSH keys for testing
    if [ ! -f "test/workers/ssh-keys/id_rsa" ]; then
        print_status $BLUE "🔑 Generating SSH keys for test environment..."
        ./scripts/generate-test-keys.sh
    fi
    
    # Create test database
    print_status $BLUE "🗄️  Setting up test database..."
    createdb helixcode_test 2>/dev/null || true
    
    print_status $GREEN "✅ Test environment setup complete"
}

# Function to run unit tests
run_unit_tests() {
    print_status $BLUE "🧪 Running Unit Tests..."
    
    local start_time=$(date +%s)
    
    # Run unit tests with coverage
    go test -v -race -coverprofile=unit_coverage.out -covermode=atomic ./internal/... \
        -timeout=$TEST_TIMEOUT \
        -parallel=$PARALLEL_TESTS
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Calculate coverage
    local coverage=$(go tool cover -func=unit_coverage.out | grep total: | awk '{print $3}' | sed 's/%//')
    
    print_status $GREEN "✅ Unit Tests completed in ${duration}s with ${coverage}% coverage"
    
    # Check coverage threshold
    if (( $(echo "$coverage < $COVERAGE_THRESHOLD" | bc -l) )); then
        print_status $RED "❌ Unit test coverage ${coverage}% is below threshold ${COVERAGE_THRESHOLD}%"
        exit 1
    fi
}

# Function to run integration tests
run_integration_tests() {
    print_status $BLUE "🔗 Running Integration Tests..."
    
    if ! command -v docker-compose &> /dev/null; then
        print_status $YELLOW "⚠️  Skipping integration tests (Docker Compose not available)"
        return 0
    fi
    
    local start_time=$(date +%s)
    
    # Start test infrastructure
    docker-compose -f docker-compose.test.yml up -d test-db
    
    # Wait for database to be ready
    until docker-compose -f docker-compose.test.yml exec -T test-db pg_isready -U helix_test -d helix_test; do
        sleep 2
    done
    
    # Run integration tests
    go test -v -tags=integration ./test/integration/... \
        -timeout=$TEST_TIMEOUT
    
    # Cleanup
    docker-compose -f docker-compose.test.yml down
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_status $GREEN "✅ Integration Tests completed in ${duration}s"
}

# Function to run E2E tests
run_e2e_tests() {
    print_status $BLUE "🌐 Running End-to-End Tests..."
    
    if ! command -v docker-compose &> /dev/null; then
        print_status $YELLOW "⚠️  Skipping E2E tests (Docker Compose not available)"
        return 0
    fi
    
    local start_time=$(date +%s)
    
    # Build and run complete test environment
    docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test-runner
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_status $GREEN "✅ E2E Tests completed in ${duration}s"
}

# Function to run automation tests
run_automation_tests() {
    print_status $BLUE "🤖 Running Automation Tests..."
    
    local start_time=$(date +%s)
    
    # Run automation tests with real AI QA
    go test -v -tags=automation ./test/automation/... \
        -timeout=$TEST_TIMEOUT \
        -test.parallel=$PARALLEL_TESTS
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_status $GREEN "✅ Automation Tests completed in ${duration}s"
}

# Function to run performance tests
run_performance_tests() {
    print_status $BLUE "⚡ Running Performance Tests..."
    
    local start_time=$(date +%s)
    
    # Run benchmarks
    go test -bench=. -benchmem ./internal/worker/... ./internal/task/... \
        -timeout=$TEST_TIMEOUT
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_status $GREEN "✅ Performance Tests completed in ${duration}s"
}

# Function to run security tests
run_security_tests() {
    print_status $BLUE "🔒 Running Security Tests..."
    
    local start_time=$(date +%s)
    
    # Run security scanning
    if command -v gosec &> /dev/null; then
        gosec ./...
    else
        print_status $YELLOW "⚠️  gosec not installed - skipping security scanning"
    fi
    
    # Run dependency vulnerability scanning
    if command -v govulncheck &> /dev/null; then
        govulncheck ./...
    else
        print_status $YELLOW "⚠️  govulncheck not installed - skipping vulnerability scanning"
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_status $GREEN "✅ Security Tests completed in ${duration}s"
}

# Function to generate test reports
generate_test_reports() {
    print_status $BLUE "📊 Generating Test Reports..."
    
    # Combine coverage reports
    if [ -f "unit_coverage.out" ]; then
        go tool cover -html=unit_coverage.out -o coverage.html
        print_status $GREEN "📄 Coverage report: coverage.html"
    fi
    
    # Generate test summary
    local total_tests=$(find . -name "*_test.go" | wc -l)
    local total_packages=$(go list ./... | wc -l)
    
    cat > test-report.md << EOF
# HelixCode Test Report

## Summary
- **Total Test Files**: $total_tests
- **Total Packages**: $total_packages
- **Test Types**: Unit, Integration, E2E, Automation, Performance, Security
- **Coverage Threshold**: ${COVERAGE_THRESHOLD}%
- **Parallel Execution**: ${PARALLEL_TESTS} threads

## Test Results
- ✅ Unit Tests: Completed with coverage
- ✅ Integration Tests: Docker-based testing
- ✅ E2E Tests: Full system validation
- ✅ Automation Tests: AI-driven QA
- ✅ Performance Tests: Benchmarking
- ✅ Security Tests: Vulnerability scanning

## Next Steps
- Review coverage reports
- Address any failing tests
- Update documentation
- Prepare for deployment
EOF
    
    print_status $GREEN "📄 Test report: test-report.md"
}

# Function to cleanup test environment
cleanup_test_environment() {
    print_status $BLUE "🧹 Cleaning up test environment..."
    
    # Stop any running containers
    docker-compose -f docker-compose.test.yml down 2>/dev/null || true
    
    # Remove test database
    dropdb helixcode_test 2>/dev/null || true
    
    # Remove coverage files
    rm -f unit_coverage.out coverage.html
    
    print_status $GREEN "✅ Test environment cleaned up"
}

# Main test execution
main() {
    local test_type="${1:-all}"
    
    case $test_type in
        "unit")
            check_prerequisites
            run_unit_tests
            ;;
        "integration")
            check_prerequisites
            run_integration_tests
            ;;
        "e2e")
            check_prerequisites
            run_e2e_tests
            ;;
        "automation")
            check_prerequisites
            run_automation_tests
            ;;
        "performance")
            check_prerequisites
            run_performance_tests
            ;;
        "security")
            check_prerequisites
            run_security_tests
            ;;
        "all")
            check_prerequisites
            setup_test_environment
            run_unit_tests
            run_integration_tests
            run_e2e_tests
            run_automation_tests
            run_performance_tests
            run_security_tests
            generate_test_reports
            cleanup_test_environment
            ;;
        *)
            echo "Usage: $0 [unit|integration|e2e|automation|performance|security|all]"
            exit 1
            ;;
    esac
    
    print_status $GREEN "🎉 All tests completed successfully!"
}

# Handle signals for graceful shutdown
trap cleanup_test_environment EXIT

# Run main function
main "$@"