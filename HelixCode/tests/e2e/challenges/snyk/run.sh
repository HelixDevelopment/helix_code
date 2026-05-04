#!/usr/bin/env bash
# HelixCode Challenge — Snyk Security Scanning Configuration
#
# Validates that Snyk security scanning infrastructure is correctly configured
# for HelixCode: compose file exists, env-var credential references (not hardcoded),
# Dockerfile is valid, policy file (.snyk) is correct, script wiring is correct.
#
# This Challenge validates CONFIGURATION CORRECTNESS — it does NOT run actual scans.
# Running live scans requires a real SNYK_TOKEN (must be rotated first per P0-T08.5).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../../../.." && pwd)"
HELIXCODE_ROOT="${PROJECT_ROOT}/HelixCode"

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
echo "  Snyk Security Scanning Challenge"
echo "  (HelixCode — configuration correctness)"
echo "==========================================="
echo ""

SNYK_COMPOSE="${HELIXCODE_ROOT}/docker/security/snyk/docker-compose.yml"
SNYK_DOCKERFILE="${HELIXCODE_ROOT}/docker/security/snyk/Dockerfile"
SNYK_POLICY="${HELIXCODE_ROOT}/.snyk"
SECURITY_SCAN="${HELIXCODE_ROOT}/scripts/security-scan.sh"
BOOT_BINARY="${HELIXCODE_ROOT}/cmd/security-scan/main.go"

# ============================================================================
# SECTION 1: FILE EXISTENCE
# ============================================================================
echo -e "${BLUE}--- Section 1: File Existence ---${NC}"

[ -f "$SNYK_COMPOSE" ]     && record_result "Snyk docker-compose.yml exists"         "PASS" || record_result "Snyk docker-compose.yml exists" "FAIL"
[ -f "$SNYK_DOCKERFILE" ]  && record_result "Snyk Dockerfile exists"                  "PASS" || record_result "Snyk Dockerfile exists" "FAIL"
[ -f "$SNYK_POLICY" ]      && record_result "Root .snyk policy file exists"           "PASS" || record_result "Root .snyk policy file exists" "FAIL"
[ -f "$SECURITY_SCAN" ]    && record_result "scripts/security-scan.sh exists"         "PASS" || record_result "scripts/security-scan.sh exists" "FAIL"
[ -f "$BOOT_BINARY" ]      && record_result "cmd/security-scan/main.go exists"        "PASS" || record_result "cmd/security-scan/main.go exists" "FAIL"

# ============================================================================
# SECTION 2: NO HARDCODED CREDENTIALS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 2: No Hardcoded Credentials ---${NC}"

# Compose file must use env-var for SNYK_TOKEN
if grep -qE '\$\{SNYK_TOKEN' "$SNYK_COMPOSE" && ! grep -qE 'SNYK_TOKEN\s*=\s*[a-z0-9]{16,}' "$SNYK_COMPOSE"; then
    record_result "SNYK_TOKEN uses env-var reference (not hardcoded)" "PASS"
else
    record_result "SNYK_TOKEN uses env-var reference (not hardcoded)" "FAIL"
fi

# No helixagent-specific container names leaked
if ! grep -qi "helixagent" "$SNYK_COMPOSE"; then
    record_result "No HelixAgent-specific container names in Snyk compose" "PASS"
else
    record_result "No HelixAgent-specific container names in Snyk compose" "FAIL"
fi

# Dockerfile uses helixcode references (not helixagent)
if ! grep -qi "helixagent" "$SNYK_DOCKERFILE"; then
    record_result "Snyk Dockerfile has no HelixAgent-specific references" "PASS"
else
    record_result "Snyk Dockerfile has no HelixAgent-specific references" "FAIL"
fi

# ============================================================================
# SECTION 3: SNYK COMPOSE SERVICES
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 3: Snyk Compose Services ---${NC}"

grep -q "snyk-deps:" "$SNYK_COMPOSE"  && record_result "snyk-deps service defined"  "PASS" || record_result "snyk-deps service defined" "FAIL"
grep -q "snyk-code:" "$SNYK_COMPOSE"  && record_result "snyk-code service defined"  "PASS" || record_result "snyk-code service defined" "FAIL"
grep -q "snyk-iac:"  "$SNYK_COMPOSE"  && record_result "snyk-iac service defined"   "PASS" || record_result "snyk-iac service defined" "FAIL"
grep -q "snyk-full:" "$SNYK_COMPOSE"  && record_result "snyk-full service defined"  "PASS" || record_result "snyk-full service defined" "FAIL"

# ============================================================================
# SECTION 4: RESOURCE LIMITS IN SNYK COMPOSE
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 4: Resource Limits ---${NC}"

grep -q "mem_limit: 2g"  "$SNYK_COMPOSE"  && record_result "Snyk containers have 2g memory limit" "PASS" || record_result "Snyk containers have 2g memory limit" "FAIL"
grep -q "pids_limit:"    "$SNYK_COMPOSE"  && record_result "Snyk containers have pids_limit"       "PASS" || record_result "Snyk containers have pids_limit" "FAIL"

# ============================================================================
# SECTION 5: DOCKERFILE CORRECTNESS
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 5: Dockerfile Correctness ---${NC}"

grep -q "FROM snyk/snyk-cli" "$SNYK_DOCKERFILE"     && record_result "Dockerfile uses official snyk/snyk-cli base image" "PASS" || record_result "Dockerfile uses official snyk/snyk-cli base image" "FAIL"
grep -q "go1.24" "$SNYK_DOCKERFILE"                  && record_result "Dockerfile installs Go 1.24"                       "PASS" || record_result "Dockerfile installs Go 1.24" "FAIL"
grep -q "/scripts/scan-all.sh" "$SNYK_DOCKERFILE"    && record_result "Dockerfile creates scan-all.sh"                    "PASS" || record_result "Dockerfile creates scan-all.sh" "FAIL"
grep -q "chmod +x /scripts" "$SNYK_DOCKERFILE"       && record_result "Dockerfile makes scripts executable"               "PASS" || record_result "Dockerfile makes scripts executable" "FAIL"
grep -q "HelixCode" "$SNYK_DOCKERFILE"               && record_result "Dockerfile references HelixCode project"           "PASS" || record_result "Dockerfile references HelixCode project" "FAIL"

# ============================================================================
# SECTION 6: SNYK POLICY FILE
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 6: Snyk Policy File ---${NC}"

grep -q "version: v1" "$SNYK_POLICY"                           && record_result ".snyk has policy version"        "PASS" || record_result ".snyk has policy version" "FAIL"
grep -q "language-settings:" "$SNYK_POLICY"                    && record_result ".snyk has language settings"     "PASS" || record_result ".snyk has language settings" "FAIL"
grep -q "  go:" "$SNYK_POLICY"                                 && record_result ".snyk configures Go language"    "PASS" || record_result ".snyk configures Go language" "FAIL"

# ============================================================================
# SECTION 7: SCRIPT WIRING
# ============================================================================
echo ""
echo -e "${BLUE}--- Section 7: Script and BootManager Wiring ---${NC}"

if "$SECURITY_SCAN" --help 2>&1 | grep -q "snyk"; then
    record_result "security-scan.sh --help shows snyk mode" "PASS"
else
    record_result "security-scan.sh --help shows snyk mode" "FAIL"
fi

if grep -q "SNYK_TOKEN" "$SECURITY_SCAN"; then
    record_result "security-scan.sh reads SNYK_TOKEN from env" "PASS"
else
    record_result "security-scan.sh reads SNYK_TOKEN from env" "FAIL"
fi

if grep -q "digital.vasic.containers/pkg/boot" "$BOOT_BINARY"; then
    record_result "cmd/security-scan imports Containers BootManager" "PASS"
else
    record_result "cmd/security-scan imports Containers BootManager" "FAIL"
fi

if grep -q '"snyk"' "$BOOT_BINARY"; then
    record_result "cmd/security-scan handles snyk scanner" "PASS"
else
    record_result "cmd/security-scan handles snyk scanner" "FAIL"
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
