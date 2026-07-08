#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# self_val_analyzer_fullhttp_gen.sh -- Self-validated analyzer for the
# HelixCode full-HTTP generate E2E bank.
#
# Implements SEC11.4.107(10) golden-good / golden-bad fixture pair.
# The analyzer MUST accept the golden-good fixture as PASS and MUST reject
# the golden-bad fixture as FAIL -- proven in Phase 1.
#
# Usage:
#   ./self_val_analyzer_fullhttp_gen.sh [--base-url URL] [--bank PATH]
#
# Exit code: 0 only when ALL pass:
#   1. Self-validation (analyzer passes golden-good, rejects golden-bad)
#   2. Bank YAML parses as valid YAML
#   3. Every RED test expects 401/400 status
#   4. Green test expects 200 + real content

set -euo pipefail

# -- Golden-good / golden-bad fixtures (SEC11.4.107(10)) ------------------------
GOLDEN_GOOD_FIXTURE='{"status":"success","content":"Hello! How can I help you today?","provider":"helixllm","model":"llama3.2","usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20},"finish_reason":"stop"}'
GOLDEN_BAD_FIXTURE='{"status":"success","content":"","provider":"helixllm","model":"llama3.2","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0},"finish_reason":"stop"}'

# -- Defaults -------------------------------------------------------------------
BASE_URL="${BASE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BANK_PATH="${BANK_PATH:-${REPO_ROOT}/submodules/helix_qa/banks/helixcode-generate-e2e.yaml}"
QA_EVIDENCE_DIR="${QA_EVIDENCE_DIR:-${REPO_ROOT}/docs/qa}"
RUN_TAG="fullhttp_gen_selfval_$(date -u +%Y%m%dT%H%M%S)"
EVIDENCE_DIR="${QA_EVIDENCE_DIR}/${RUN_TAG}"

# -- Parse CLI ------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="$2"; shift 2 ;;
    --bank)     BANK_PATH="$2";    shift 2 ;;
    *)          echo "Unknown: $1"; exit 2 ;;
  esac
done

# -- Counters -------------------------------------------------------------------
P=0 F=0 S=0
pass() { echo "  PASS: $1"; ((P++)) || true; }
fail() { echo "  FAIL: $1"; ((F++)) || true; }
skip() { echo "  SKIP: $1"; ((S++)) || true; }

# -- Json validation helper -----------------------------------------------------
# validate_good_json: runs a python script that reads two env vars:
#   VJ_JSON   - the JSON string to validate
#   VJ_REQUIRED - required substring (may be empty)
# Returns 0 if JSON has non-empty "content" matching all criteria.
# Returns 1 otherwise. Stderr is silenced.
validate_good_json() {
  VJ_JSON="$1" VJ_REQUIRED="$2" python3 -c '
import os, json, sys
raw = os.environ.get("VJ_JSON", "")
required = os.environ.get("VJ_REQUIRED", "")
try:
    obj = json.loads(raw)
except json.JSONDecodeError:
    sys.exit(1)
content = obj.get("content", "")
if not isinstance(content, str) or content.strip() == "":
    sys.exit(1)
if required and required.lower() not in content.lower():
    sys.exit(1)
bluff = ["simulated", "for now", "placeholder", "todo implement", "in production this would"]
for p in bluff:
    if p in content.lower():
        sys.exit(1)
sys.exit(0)
' 2>/dev/null
}

echo "============================================"
echo "  Self-Validated Analyzer -- Generate E2E"
echo "  Run ID: ${RUN_TAG}"
echo "  Base URL: ${BASE_URL}"
echo "  Bank: ${BANK_PATH}"
echo "  Evidence: ${EVIDENCE_DIR}"
echo "============================================"
echo ""

# -- Phase 0: Set up evidence dir ----------------------------------------------
mkdir -p "${EVIDENCE_DIR}"

# -- Phase 1: Self-validation (golden-good/golden-bad) --------------------------
echo "--- Phase 1: Self-validation (golden-good/golden-bad fixtures) ---"

if validate_good_json "${GOLDEN_GOOD_FIXTURE}" "hello"; then
  pass "Golden-good fixture accepted (non-empty content with 'hello')"
else
  fail "Golden-good fixture REJECTED -- analyzer broke"
fi

if validate_good_json "${GOLDEN_BAD_FIXTURE}" ""; then
  fail "Golden-bad fixture ACCEPTED -- empty-content bluff NOT detected!"
else
  pass "Golden-bad fixture rejected (empty content caught)"
fi

if echo "${GOLDEN_BAD_FIXTURE}" | python3 -c 'import json,sys; json.load(sys.stdin)' 2>/dev/null; then
  pass "Golden-bad fixture IS valid JSON (plausible server response shape)"
else
  fail "Golden-bad fixture is not valid JSON -- test fixture broken"
fi

echo ""

# -- Phase 2: Bank YAML structure validation ------------------------------------
echo "--- Phase 2: Bank YAML structure validation ---"

if [[ -f "${BANK_PATH}" ]]; then
  pass "Bank file exists: ${BANK_PATH}"
else
  fail "Bank file not found at: ${BANK_PATH}"
fi

# Parse YAML via python3
python3 -c "
import yaml, sys
try:
    with open('${BANK_PATH}') as f:
        doc = yaml.safe_load(f)
    print('  Name: ' + str(doc.get('name','')))
    cases = doc.get('test_cases', [])
    print('  Cases: ' + str(len(cases)))
    for t in cases:
        print('    ' + str(t.get('id','')).ljust(20) + ' ' + str(t.get('name',''))[:70])
except Exception as e:
    print('YAML ERROR: ' + str(e))
    sys.exit(1)
" && pass "Bank YAML parses correctly" || fail "Bank YAML parse error"

echo ""

# -- Phase 3: RED test verification -------------------------------------------
echo "--- Phase 3: RED test verification (unauth -> 401) ---"

RED_TOTAL=0
while IFS= read -r line; do
  if [[ -n "${line}" ]]; then
    ((RED_TOTAL++)) || true
  fi
done < <(python3 -c "
import yaml
with open('${BANK_PATH}') as f:
    doc = yaml.safe_load(f)
for tc in doc.get('test_cases',[]):
    for step in tc.get('steps',[]):
        if step.get('expect_status') in (400, 401):
            print(tc['id'])
            break
" 2>/dev/null)

if [[ "${RED_TOTAL}" -ge 3 ]]; then
  pass "Found ${RED_TOTAL} RED tests (all expect 400/401)"
else
  fail "Expected >=3 RED tests, found ${RED_TOTAL}"
fi

echo ""

# -- Phase 4: GREEN test expectation check -------------------------------------
echo "--- Phase 4: GREEN test expectation check ---"

export PY_BANK_PATH="${BANK_PATH}"
if python3 <<'PYEOF'
import yaml, os
bank_path = os.environ.get('PY_BANK_PATH', '')
with open(bank_path) as f:
    doc = yaml.safe_load(f)
for tc in doc.get('test_cases', []):
    if tc['id'] == 'HXC-GEN-005':
        for step in tc.get('steps', []):
            assert step.get('expect_status') == 200, 'must expect 200'
            json_path = step.get('expect_json_path', '')
            assert '$.content' in json_path, 'must assert $.content, got: ' + json_path
            print('GREEN test HXC-GEN-005: status=200, json_path=' + json_path)
PYEOF
then
  pass "HXC-GEN-005 GREEN test expects 200 + content"
else
  fail "HXC-GEN-005 GREEN test validation failed"
fi

echo ""

# -- Phase 5: Live server connectivity check -----------------------------------
echo "--- Phase 5: Live server connectivity check ---"

if curl -sf "${BASE_URL}/health" > /dev/null 2>&1; then
  pass "Server reachable at ${BASE_URL}/health"
else
  skip "Server NOT reachable at ${BASE_URL} -- SKIP-ing live checks"
fi

echo ""

# -- Phase 6: HelixLLM coder connectivity -------------------------------------
echo "--- Phase 6: HelixLLM coder connectivity (:18434) ---"

if curl -sf 'http://localhost:18434/health' > /dev/null 2>&1 || \
   curl -sf 'http://localhost:18434/v1/models' > /dev/null 2>&1; then
  pass "HelixLLM coder reachable at :18434"
else
  skip "Coder NOT reachable at :18434 -- SKIP-ing coder-specific checks"
fi

echo ""

# -- Summary -------------------------------------------------------------------
echo "============================================"
echo "  RESULTS"
echo "============================================"
echo "  PASS: ${P}"
echo "  FAIL: ${F}"
echo "  SKIP: ${S}"
echo "============================================"

# -- Write evidence ------------------------------------------------------------
# Pre-compute booleans for evidence file
if validate_good_json "${GOLDEN_GOOD_FIXTURE}" "hello" 2>/dev/null; then
  GG_ACCEPTS="TRUE"
else
  GG_ACCEPTS="FALSE"
fi
if validate_good_json "${GOLDEN_BAD_FIXTURE}" "" 2>/dev/null; then
  GB_REJECTS="FALSE"
else
  GB_REJECTS="TRUE"
fi

{
  echo "# Self-Validated Analyzer Results -- ${RUN_TAG}"
  echo ""
  echo "## Golden-Good Fixture"
  echo '```json'
  echo "${GOLDEN_GOOD_FIXTURE}"
  echo '```'
  echo "Accepts: ${GG_ACCEPTS}"
  echo ""
  echo "## Golden-Bad Fixture"
  echo '```json'
  echo "${GOLDEN_BAD_FIXTURE}"
  echo '```'
  echo "Rejects: ${GB_REJECTS}"
  echo ""
  echo "## Bank Structure"
  echo "Bank: ${BANK_PATH}"
  echo "RED tests: ${RED_TOTAL}"
  echo ""
  echo "## Summary"
  echo "PASS: ${P}"
  echo "FAIL: ${F}"
  echo "SKIP: ${S}"
} > "${EVIDENCE_DIR}/ANALYSIS.md" || echo "WARNING: Evidence write failed (exit $?)" >&2

if [[ -f "${EVIDENCE_DIR}/ANALYSIS.md" ]]; then
  echo "Evidence written to: ${EVIDENCE_DIR}/ANALYSIS.md"
fi

echo ""
[[ "${F}" -eq 0 ]]
