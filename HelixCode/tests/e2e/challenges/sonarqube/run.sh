#!/usr/bin/env bash
# HelixCode Challenge — SonarQube Security Scanning Configuration
# Ported from HelixAgent's challenges/scripts/sonarqube_automated_scanning_challenge.sh
#
# Validates that SonarQube security scanning infrastructure is correctly configured
# for HelixCode: compose files exist, env-var references not hardcoded credentials,
# services defined, resource limits set, health checks wired, volumes declared.
#
# This Challenge validates CONFIGURATION CORRECTNESS — it does NOT run actual scans.
# Running live scans requires real credentials (SONAR_TOKEN must be rotated first).
# See docs/improvements/05_phase_0_evidence.md § P0-T08.7 for credential rotation notes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../../../.." && pwd)"
HELIXCODE_ROOT="${PROJECT_ROOT}/HelixCode"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

record_result() {
    local test_name="$1"
    local status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "${GREEN}[PASS]${NC} $test_name"
    else
        FAILED=$((FAILED + 1))
        echo -e "${RED}[FAIL]${NC} $test_name"
    fi
}

echo "==========================================="
echo "  SonarQube Security Scanning Challenge"
echo "  (HelixCode — configuration correctness)"
echo "==========================================="
echo ""

ROOT_PROPS="${HELIXCODE_ROOT}/sonar-project.properties"
DOCKER_COMPOSE="${HELIXCODE_ROOT}/docker/security/sonarqube/docker-compose.yml"
DOCKER_PROPS="${HELIXCODE_ROOT}/docker/security/sonarqube/sonar-project.properties"
SNYK_POLICY="${HELIXCODE_ROOT}/.snyk"
SECURITY_SCAN="${HELIXCODE_ROOT}/scripts/security-scan.sh"
BOOT_BINARY="${HELIXCODE_ROOT}/cmd/security-scan/main.go"

# ============================================================================
# SECTION 1: FILE EXISTENCE
# ============================================================================
echo -e "${BLUE}--- Section 1: File Existence ---${NC}"

[ -f "$ROOT_PROPS" ]       && record_result "Root sonar-project.properties exists"     "PASS" || record_result "Root sonar-project.properties exists" "FAIL"
[ -f "$DOCKER_COMPOSE" ]   && record_result "SonarQube docker-compose.yml exists"      "PASS" || record_result "SonarQube docker-compose.yml exists" "FAIL"
[ -f "$DOCKER_PROPS" ]     && record_result "SonarQube sonar-project.properties (docker) exists" "PASS" || record_result "SonarQube sonar-project.properties (docker) exists" "FAIL"
[ -f "$SNYK_POLICY" ]      && record_result "Root .snyk policy exists"                 "PASS" || record_result "Root .snyk policy exists" "FAIL"
[ -f "$SECURITY_SCAN" ]    && record_result "scripts/security-scan.sh exists"          "PASS" || record_result "scripts/security-scan.sh exists" "FAIL"
[ -x "$SECURITY_SCAN" ]    && record_result "security-scan.sh is executable"           "PASS" || record_result "security-scan.sh is executable" "FAIL"
[ -f "$BOOT_BINARY" ]      && record_result "cmd/security-scan/main.go exists"         "PASS" || record_result "cmd/security-scan/main.go exists" "FAIL"

# ============================================================================
# SECTION 2: NO HARDCODED CREDENTIALS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 2: No Hardcoded Credentials ---${NC}"

# Compose files must NOT contain raw token values — must use ${VAR} pattern
if grep -qE '\$\{SONAR_TOKEN' "$DOCKER_COMPOSE" && ! grep -qE 'SONAR_TOKEN\s*=\s*[a-z0-9]{16,}' "$DOCKER_COMPOSE"; then
    record_result "SONAR_TOKEN uses env-var reference (not hardcoded)" "PASS"
else
    record_result "SONAR_TOKEN uses env-var reference (not hardcoded)" "FAIL"
fi

# Root properties must use ${...} for project key, not a hardcoded value like 'helixagent' or 'helixcode-prod-key-123'
if grep -q 'sonar.projectKey=\${' "$ROOT_PROPS"; then
    record_result "sonar.projectKey uses env-var reference in root properties" "PASS"
else
    record_result "sonar.projectKey uses env-var reference in root properties" "FAIL"
fi

if grep -q 'sonar.projectKey=\${' "$DOCKER_PROPS"; then
    record_result "sonar.projectKey uses env-var reference in docker properties" "PASS"
else
    record_result "sonar.projectKey uses env-var reference in docker properties" "FAIL"
fi

# No helixagent-specific references leaked
if ! grep -qi "helixagent" "$DOCKER_COMPOSE" && ! grep -qi "helixagent" "$ROOT_PROPS"; then
    record_result "No HelixAgent-specific references in HelixCode configs" "PASS"
else
    record_result "No HelixAgent-specific references in HelixCode configs" "FAIL"
fi

# ============================================================================
# SECTION 3: REQUIRED SERVICES
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 3: Required Services in Compose ---${NC}"

grep -q "sonarqube:" "$DOCKER_COMPOSE"   && record_result "SonarQube service defined"          "PASS" || record_result "SonarQube service defined" "FAIL"
grep -q "postgres:"  "$DOCKER_COMPOSE"   && record_result "PostgreSQL service defined"          "PASS" || record_result "PostgreSQL service defined" "FAIL"
grep -q "sonar-scanner:" "$DOCKER_COMPOSE" && record_result "Sonar scanner service defined"    "PASS" || record_result "Sonar scanner service defined" "FAIL"
grep -q "depends_on" "$DOCKER_COMPOSE"   && record_result "depends_on configured"              "PASS" || record_result "depends_on configured" "FAIL"

# ============================================================================
# SECTION 4: PINNED IMAGE VERSIONS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 4: Pinned Image Versions ---${NC}"

grep -qE "sonarqube:[0-9].*-community" "$DOCKER_COMPOSE" && record_result "SonarQube uses pinned community edition image" "PASS" || record_result "SonarQube uses pinned community edition image" "FAIL"
grep -q "postgres:.*alpine"  "$DOCKER_COMPOSE"           && record_result "PostgreSQL uses pinned alpine version"         "PASS" || record_result "PostgreSQL uses pinned alpine version" "FAIL"
grep -q "sonarsource/sonar-scanner-cli" "$DOCKER_COMPOSE" && record_result "Scanner uses official sonarsource image"     "PASS" || record_result "Scanner uses official sonarsource image" "FAIL"

# ============================================================================
# SECTION 5: RESOURCE LIMITS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 5: Resource Limits ---${NC}"

grep -q "mem_limit"  "$DOCKER_COMPOSE"  && record_result "Memory limits configured"  "PASS" || record_result "Memory limits configured" "FAIL"
grep -q "cpus:"      "$DOCKER_COMPOSE"  && record_result "CPU limits configured"      "PASS" || record_result "CPU limits configured" "FAIL"
grep -q "ulimits:"   "$DOCKER_COMPOSE"  && record_result "ulimits configured"         "PASS" || record_result "ulimits configured" "FAIL"

# ============================================================================
# SECTION 6: HEALTH CHECKS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 6: Health Checks ---${NC}"

grep -q "healthcheck:" "$DOCKER_COMPOSE"    && record_result "Health checks configured"                         "PASS" || record_result "Health checks configured" "FAIL"
grep -q "api/system/status" "$DOCKER_COMPOSE" && record_result "SonarQube health check uses /api/system/status" "PASS" || record_result "SonarQube health check uses /api/system/status" "FAIL"
grep -q "pg_isready"  "$DOCKER_COMPOSE"     && record_result "PostgreSQL health check uses pg_isready"          "PASS" || record_result "PostgreSQL health check uses pg_isready" "FAIL"

# ============================================================================
# SECTION 7: NETWORK & VOLUMES
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 7: Network and Volumes ---${NC}"

grep -q "security-network:" "$DOCKER_COMPOSE" && record_result "Security network defined"            "PASS" || record_result "Security network defined" "FAIL"
grep -q "driver: bridge"    "$DOCKER_COMPOSE" && record_result "Security network uses bridge driver"  "PASS" || record_result "Security network uses bridge driver" "FAIL"
grep -q "subnet:"           "$DOCKER_COMPOSE" && record_result "Network has IPAM subnet config"        "PASS" || record_result "Network has IPAM subnet config" "FAIL"
grep -q "sonarqube_data:"   "$DOCKER_COMPOSE" && record_result "SonarQube data volume defined"        "PASS" || record_result "SonarQube data volume defined" "FAIL"
grep -q "postgres_data:"    "$DOCKER_COMPOSE" && record_result "PostgreSQL data volume defined"        "PASS" || record_result "PostgreSQL data volume defined" "FAIL"

# ============================================================================
# SECTION 8: SCAN SCRIPT & BOOT BINARY
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 8: Scan Script and BootManager ---${NC}"

# security-scan.sh supports --help without docker invocation
if "$SECURITY_SCAN" --help 2>&1 | grep -q "sonarqube"; then
    record_result "security-scan.sh --help shows sonarqube mode" "PASS"
else
    record_result "security-scan.sh --help shows sonarqube mode" "FAIL"
fi

# security-scan.sh supports snyk mode
if "$SECURITY_SCAN" --help 2>&1 | grep -q "snyk"; then
    record_result "security-scan.sh --help shows snyk mode" "PASS"
else
    record_result "security-scan.sh --help shows snyk mode" "FAIL"
fi

# Boot binary references Containers BootManager imports
if grep -q "digital.vasic.containers/pkg/boot" "$BOOT_BINARY"; then
    record_result "cmd/security-scan imports Containers BootManager" "PASS"
else
    record_result "cmd/security-scan imports Containers BootManager" "FAIL"
fi

# Boot binary references runtime.AutoDetect
if grep -q "runtime.AutoDetect" "$BOOT_BINARY"; then
    record_result "cmd/security-scan uses runtime.AutoDetect" "PASS"
else
    record_result "cmd/security-scan uses runtime.AutoDetect" "FAIL"
fi

# ============================================================================
# SUMMARY
# ============================================================================
echo ""
echo "==========================================="
echo "  Results: $PASSED/$TOTAL passed, $FAILED failed"
echo "==========================================="

if [ $FAILED -gt 0 ]; then
    exit 1
fi
exit 0
