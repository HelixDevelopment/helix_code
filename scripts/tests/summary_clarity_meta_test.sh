#!/usr/bin/env bash
# summary_clarity_meta_test.sh — paired-mutation meta-test (§1.1) for the
# §11.4.91 summary-clarity gate (scripts/gates/summary_clarity_gate.sh).
#
# Plants known anti-pattern one-liners in a temp fixture summary doc, asserts
# the gate FAILs; then repairs them, asserts the gate PASSes. Exits non-zero
# if any assertion fails.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/summary_clarity_gate.sh"

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

mkdir -p "$TMP/docs"
# Fixed_Summary.md is in the gate's file list; keep it minimal + compliant.
cat > "$TMP/docs/Fixed_Summary.md" <<'EOF'
# Fixed Summary
EOF

fail=0
assert_fail() {
    if "$GATE" "$TMP/docs" >/dev/null 2>&1; then
        echo "ASSERT-FAIL: gate PASSED but should have FAILED ($1)"
        fail=1
    else
        echo "ASSERT-OK:   gate FAILED as expected ($1)"
    fi
}
assert_pass() {
    if "$GATE" "$TMP/docs" >/dev/null 2>&1; then
        echo "ASSERT-OK:   gate PASSED as expected ($1)"
    else
        echo "ASSERT-FAIL: gate FAILED but should have PASSED ($1)"
        fail=1
    fi
}

write_summary() {
    # $1 = the one-liner description cell for the data row under test.
    cat > "$TMP/docs/Issues_Summary.md" <<EOF
# Issues Summary

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
| XXX-001 | A planted summary row for meta-testing | Bug | Open | 2026-05-28 | $1 |
EOF
}

# --- Mutation 1: section-label anti-pattern → FAIL ---
write_summary "Composes with"
assert_fail "section-label 'Composes with'"

# --- Mutation 2: bare-metadata anti-pattern → FAIL ---
write_summary "Critical"
assert_fail "bare-metadata 'Critical'"

# --- Mutation 3: bare section-letter anti-pattern → FAIL ---
write_summary "(A)"
assert_fail "bare section-letter '(A)'"

# --- Mutation 4: too-short description → FAIL ---
write_summary "tiny note"
assert_fail "too-short (< 6 words AND < 40 chars)"

# --- Repair: self-contained meaningful description → PASS ---
write_summary "Closed: race in load_balancer stat-collector goroutine fixed via mutex snapshot, -race test green"
assert_pass "self-contained meaningful description"

if [[ "$fail" -ne 0 ]]; then
    echo "META-TEST FAIL: §11.4.91 summary-clarity gate did not behave as specified"
    exit 1
fi
echo "META-TEST PASS: §11.4.91 summary-clarity gate paired-mutation verified"
exit 0
