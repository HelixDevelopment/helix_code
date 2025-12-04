#!/bin/bash

# Coverage reporting script for HelixCode project
# This script generates comprehensive coverage reports and dashboards

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_FILE="coverage.out"
HTML_COVERAGE_FILE="coverage.html"
UNIT_COVERAGE_FILE="unit_coverage.out"
INTEGRATION_COVERAGE_FILE="integration_coverage.out"
E2E_COVERAGE_FILE="e2e_coverage.out"
REPORT_DIR="coverage-reports"
MIN_COVERAGE=85

echo -e "${BLUE}🔍 HelixCode Coverage Reporting${NC}"
echo "=================================="

# Create report directory
mkdir -p "${REPORT_DIR}"

# Clean previous coverage files
echo -e "${YELLOW}🧹 Cleaning previous coverage files...${NC}"
rm -f *.out coverage.html "${REPORT_DIR}"/*

echo -e "${BLUE}📊 Running comprehensive test coverage...${NC}"

# 1. Unit Tests Coverage
echo -e "${YELLOW}Running unit tests with coverage...${NC}"
go test -v -race -coverprofile="${UNIT_COVERAGE_FILE}" -covermode=atomic ./internal/... 2>&1 | tee "${REPORT_DIR}/unit-tests.log"

# 2. Integration Tests Coverage (if integration tests exist)
if [ -d "tests/integration" ] || find . -name "*_integration_test.go" -not -path "./vendor/*" | grep -q .; then
    echo -e "${YELLOW}Running integration tests with coverage...${NC}"
    go test -v -race -coverprofile="${INTEGRATION_COVERAGE_FILE}" -covermode=atomic ./tests/... 2>&1 | tee "${REPORT_DIR}/integration-tests.log"
fi

# 3. E2E Tests Coverage (if E2E tests exist)
if [ -d "tests/e2e" ] || find . -name "*_e2e_test.go" -not -path "./vendor/*" | grep -q .; then
    echo -e "${YELLOW}Running E2E tests with coverage...${NC}"
    go test -v -race -coverprofile="${E2E_COVERAGE_FILE}" -covermode=atomic ./tests/e2e/... 2>&1 | tee "${REPORT_DIR}/e2e-tests.log"
fi

# 4. Merge coverage files
echo -e "${YELLOW}🔗 Merging coverage files...${NC}"
COVERAGE_FILES=()
if [ -f "${UNIT_COVERAGE_FILE}" ]; then
    COVERAGE_FILES+=("${UNIT_COVERAGE_FILE}")
fi
if [ -f "${INTEGRATION_COVERAGE_FILE}" ]; then
    COVERAGE_FILES+=("${INTEGRATION_COVERAGE_FILE}")
fi
if [ -f "${E2E_COVERAGE_FILE}" ]; then
    COVERAGE_FILES+=("${E2E_COVERAGE_FILE}")
fi

if [ ${#COVERAGE_FILES[@]} -gt 1 ]; then
    # Use gocovmerge to combine coverage files
    go install github.com/wadey/gocovmerge@latest
    gocovmerge "${COVERAGE_FILES[@]}" > "${COVERAGE_FILE}"
elif [ ${#COVERAGE_FILES[@]} -eq 1 ]; then
    cp "${COVERAGE_FILES[0]}" "${COVERAGE_FILE}"
else
    echo -e "${RED}❌ No coverage files generated${NC}"
    exit 1
fi

# 5. Generate coverage reports
echo -e "${YELLOW}📈 Generating coverage reports...${NC}"

# Total coverage percentage
TOTAL_COVERAGE=$(go tool cover -func="${COVERAGE_FILE}" | tail -1 | grep -o '[0-9.]*%' | tr -d '%')
echo -e "${GREEN}Total Coverage: ${TOTAL_COVERAGE}%${NC}"

# Per-package coverage
echo -e "${BLUE}📦 Coverage by Package:${NC}"
go tool cover -func="${COVERAGE_FILE}" | grep -v "^total:" | sort -k3 -nr | head -20

# Generate HTML coverage
go tool cover -html="${COVERAGE_FILE}" -o "${HTML_COVERAGE_FILE}"
echo -e "${GREEN}📄 HTML coverage report generated: ${HTML_COVERAGE_FILE}${NC}"

# 6. Coverage quality gate
echo -e "${YELLOW}🚦 Checking coverage quality gate...${NC}"
COVERAGE_CHECK=$(echo "${TOTAL_COVERAGE} >= ${MIN_COVERAGE}" | bc -l)
if [ "${COVERAGE_CHECK}" -eq 1 ]; then
    echo -e "${GREEN}✅ Coverage ${TOTAL_COVERAGE}% meets minimum threshold of ${MIN_COVERAGE}%${NC}"
else
    echo -e "${RED}❌ Coverage ${TOTAL_COVERAGE}% is below minimum threshold of ${MIN_COVERAGE}%${NC}"
    exit 1
fi

# 7. Generate detailed report
echo -e "${YELLOW}📋 Generating detailed coverage report...${NC}"
REPORT_FILE="${REPORT_DIR}/coverage-report-$(date +%Y%m%d-%H%M%S).md"

cat > "${REPORT_FILE}" << EOF
# HelixCode Coverage Report

Generated: $(date)
Minimum Coverage Threshold: ${MIN_COVERAGE}%
Total Coverage: ${TOTAL_COVERAGE}%

## Coverage Summary

| Metric | Value |
|--------|-------|
| Total Coverage | ${TOTAL_COVERAGE}% |
| Status | $([ "${COVERAGE_CHECK}" -eq 1 ] && echo "✅ PASS" || echo "❌ FAIL") |

## Coverage by Package (Top 20)

\`\`\`
$(go tool cover -func="${COVERAGE_FILE}" | grep -v "^total:" | sort -k3 -nr | head -20)
\`\`\`

## Full Coverage Details

\`\`\`
$(go tool cover -func="${COVERAGE_FILE}")
\`\`\`

## Test Results

- Unit Tests: $([ -f "${REPORT_DIR}/unit-tests.log" ] && echo "✅ Executed" || echo "❌ Missing")
- Integration Tests: $([ -f "${REPORT_DIR}/integration-tests.log" ] && echo "✅ Executed" || echo "❌ Missing")
- E2E Tests: $([ -f "${REPORT_DIR}/e2e-tests.log" ] && echo "✅ Executed" || echo "❌ Missing")

## Coverage Files Generated

- Main Coverage: ${COVERAGE_FILE}
- HTML Coverage: ${HTML_COVERAGE_FILE}
- Unit Coverage: ${UNIT_COVERAGE_FILE}
- Integration Coverage: ${INTEGRATION_COVERAGE_FILE}
- E2E Coverage: ${E2E_COVERAGE_FILE}

## Recommendations

EOF

if [ "${COVERAGE_CHECK}" -eq 0 ]; then
    cat >> "${REPORT_FILE}" << EOF
⚠️ Coverage is below the minimum threshold. Consider:

1. Adding unit tests for uncovered functions
2. Improving test coverage in low-coverage packages
3. Adding integration tests for complex workflows
4. Writing E2E tests for user-facing features

EOF
fi

# Identify functions with low coverage
echo -e "${YELLOW}🔍 Analyzing low coverage areas...${NC}"
LOW_COVERAGE_FUNCTIONS=$(go tool cover -func="${COVERAGE_FILE}" | awk -v threshold=50 '$3 != "" && substr($3, 1, length($3)-1) < threshold {print $1 " " $3}')
if [ -n "$LOW_COVERAGE_FUNCTIONS" ]; then
    echo -e "${YELLOW}Functions with coverage < 50%:${NC}"
    echo "$LOW_COVERAGE_FUNCTIONS"
    echo "" >> "${REPORT_FILE}"
    echo "### Low Coverage Functions (< 50%)" >> "${REPORT_FILE}"
    echo '```' >> "${REPORT_FILE}"
    echo "$LOW_COVERAGE_FUNCTIONS" >> "${REPORT_FILE}"
    echo '```' >> "${REPORT_FILE}"
fi

# 8. Coverage badge generation
echo -e "${YELLOW}🏷️ Generating coverage badge...${NC}"
BADGE_COLOR=""
if [ "${TOTAL_COVERAGE}" -ge 90 ]; then
    BADGE_COLOR="brightgreen"
elif [ "${TOTAL_COVERAGE}" -ge 80 ]; then
    BADGE_COLOR="green"
elif [ "${TOTAL_COVERAGE}" -ge 70 ]; then
    BADGE_COLOR="yellow"
elif [ "${TOTAL_COVERAGE}" -ge 60 ]; then
    BADGE_COLOR="orange"
else
    BADGE_COLOR="red"
fi

BADGE_SVG="${REPORT_DIR}/coverage-badge.svg"
cat > "${BADGE_SVG}" << EOF
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="20" role="img" aria-label="coverage: ${TOTAL_COVERAGE}%">
  <title>coverage: ${TOTAL_COVERAGE}%</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <mask id="s">
    <rect width="120" height="20" rx="3" fill="#fff"/>
  </mask>
  <g mask="url(#s)">
    <rect width="60" height="20" fill="#555"/>
    <rect x="60" width="60" height="20" fill="#4c1"/>
    <rect width="120" height="20" fill="url(#s)"/>
  </g>
  <g aria-hidden="true" fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text x="35" y="15" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="450">coverage</text>
    <text x="35" y="14" transform="scale(.1)" textLength="450">coverage</text>
    <text x="95" y="15" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="550">${TOTAL_COVERAGE}%</text>
    <text x="95" y="14" transform="scale(.1)">${TOTAL_COVERAGE}%</text>
  </g>
</svg>
EOF

echo -e "${GREEN}📊 Coverage report generated: ${REPORT_FILE}${NC}"
echo -e "${GREEN}🏷️ Coverage badge generated: ${BADGE_SVG}${NC}"

# 9. Summary
echo ""
echo -e "${BLUE}📊 Coverage Summary${NC}"
echo "=================="
echo -e "Total Coverage: ${GREEN}${TOTAL_COVERAGE}%${NC}"
echo -e "Status: $([ "${COVERAGE_CHECK}" -eq 1 ] && echo "${GREEN}✅ PASS${NC}" || echo "${RED}❌ FAIL${NC}")"
echo -e "HTML Report: ${GREEN}${HTML_COVERAGE_FILE}${NC}"
echo -e "Detailed Report: ${GREEN}${REPORT_FILE}${NC}"
echo -e "Coverage Badge: ${GREEN}${BADGE_SVG}${NC}"

exit $([ "${COVERAGE_CHECK}" -eq 1 ] && echo 0 || echo 1)