#!/usr/bin/env bash
# scripts/bluff-detector.sh
#
# Anti-bluff composite scanner for HelixCode production code.
# Implements the deterministic subset of the detector specified in
# docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md §5.2
# and enforces CLAUDE.md §3.3 (BLUFF-001/002/003 must stay resolved) and
# CONST-035 / Article XI §11.9 (zero-bluff mandate) and CONST-050(A)
# (no mocks imported by integration tests).
#
# Two gating checks (both deterministic, both currently clean — see the
# stream report for the calibration evidence):
#
#   Check 4 — integration-test purity (B5 / CONST-050(A)):
#       files under helix_code/tests/integration/ MUST NOT import
#       helix_code/internal/mocks. Any hit => FAIL.
#
#   Check 5 — canonical simulation-signature scan (B6 / §3.3):
#       the exact BLUFF-001/002/003 simulation phrases must NOT appear in
#       executable (non-comment) production code. Any hit => FAIL.
#
# Scope is restricted to high-confidence, executable-line signatures so the
# gate is meaningful (it fails on real bluffs) without being a tautology
# (benign i18n "placeholder" terminology, doc-comments quoting removed bluff
# strings, and code-generators emitting "TODO: implement" as DATA are NOT
# flagged — see the stream report for why each is benign).
#
# Wired into: make bluff-detector  (and transitively make verify-foundation).
# Exit 0 = clean, exit 1 = bluff hit(s), exit 2 = environment error.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

PROD_DIRS=(helix_code/internal helix_code/cmd helix_code/applications)
INTEGRATION_DIR=helix_code/tests/integration

fail=0

# --- environment sanity -----------------------------------------------------
for d in "${PROD_DIRS[@]}"; do
  if [ ! -d "$d" ]; then
    echo "ERROR: expected production directory missing: $d" >&2
    echo "  (run: git submodule update --init --recursive)" >&2
    exit 2
  fi
done

# --- Check 4: integration-test purity (no internal/mocks imports) -----------
echo "[bluff-detector] Check 4: integration-test purity (no internal/mocks imports)"
if [ -d "$INTEGRATION_DIR" ]; then
  # grep -R returns 1 (no match) on a clean tree; capture without aborting set -e.
  c4_hits="$(grep -rn --include='*.go' 'internal/mocks' "$INTEGRATION_DIR" 2>/dev/null || true)"
  if [ -n "$c4_hits" ]; then
    echo "  FAIL: integration tests import internal/mocks (CONST-050(A) violation):" >&2
    echo "$c4_hits" | sed 's/^/    /' >&2
    fail=1
  else
    echo "  OK: no internal/mocks imports under $INTEGRATION_DIR"
  fi
else
  echo "  SKIP: $INTEGRATION_DIR not present"
fi

# --- Check 5: canonical simulation-signature scan ---------------------------
# These are the EXACT phrases CLAUDE.md §3.3 declares must stay gone (the
# resolved BLUFF-001/002/003 patterns). They are bluff signatures regardless
# of surrounding code; matching is restricted to executable lines (comment
# lines starting with // are excluded, as are *_test.go files).
echo "[bluff-detector] Check 5: canonical simulation-signature scan (§3.3 / CONST-035)"
SIM_PATTERN='For now, simulate|simulate generation|This is a simulated response|In production, this would use the actual LLM|For now, just return a simulated|Generated response for: %s'

c5_hits="$(grep -rn --include='*.go' -E "$SIM_PATTERN" "${PROD_DIRS[@]}" 2>/dev/null \
  | grep -v '_test\.go:' \
  | grep -vE ':[0-9]+:[[:space:]]*//' \
  || true)"

if [ -n "$c5_hits" ]; then
  echo "  FAIL: canonical simulation bluff signature(s) found in executable production code:" >&2
  echo "$c5_hits" | sed 's/^/    /' >&2
  fail=1
else
  echo "  OK: no canonical simulation bluff signatures in executable production code"
fi

# --- verdict ----------------------------------------------------------------
if [ "$fail" -ne 0 ]; then
  echo "[bluff-detector] RESULT: FAIL — bluff signatures detected (see above)" >&2
  exit 1
fi

echo "[bluff-detector] RESULT: PASS — no bluff signatures detected"
exit 0
