#!/usr/bin/env bash
# scripts/tests/workable_items_sync_meta_test.sh — §1.1 paired-mutation meta-test
# for scripts/gates/workable_items_sync_gate.sh (CM-WORKABLE-ITEMS-MD-DB-IN-SYNC).
#
# Plants a known md↔db drift (a phantom item appended to docs/Issues.md that the
# committed DB does not contain), asserts the gate FAILS, then restores and asserts
# the gate PASSES. Proves the gate cannot bluff (a real drift is detected).
#
# The mutation is on the REAL docs/Issues.md but is GUARANTEED restored via a trap
# (even on interrupt) — the original is byte-restored from a backup.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

GATE="scripts/gates/workable_items_sync_gate.sh"
ISSUES="docs/Issues.md"
BACKUP="$(mktemp)"
PASS=0
FAIL=0

cp "$ISSUES" "$BACKUP"
restore() { cp "$BACKUP" "$ISSUES"; rm -f "$BACKUP"; }
trap restore EXIT INT TERM

ck() { # ck <desc> <expected-exit> <actual-exit>
    if [ "$2" -eq "$3" ]; then echo "  PASS: $1 (exit $3)"; PASS=$((PASS+1));
    else echo "  FAIL: $1 (expected exit $2, got $3)"; FAIL=$((FAIL+1)); fi
}

echo "=== workable_items_sync_gate §1.1 paired-mutation meta-test ==="

# Baseline: gate should PASS (or SKIP if the binary can't build — then this
# environment can't run the meta-test meaningfully).
bash "$GATE" >/tmp/wi_meta_base.out 2>&1; base=$?
if grep -q 'SKIP-OK' /tmp/wi_meta_base.out; then
    echo "  SKIP-OK: gate skipped in this env ($(tail -1 /tmp/wi_meta_base.out)) — cannot run paired mutation"
    exit 0
fi
ck "baseline gate PASS (md⟷db in sync)" 0 "$base"

# MUTATION: append a phantom workable item to docs/Issues.md (drift — the
# committed DB does not contain it, and md no longer matches the DB projection).
cat >> "$ISSUES" <<'EOF'

## HXC-META-MUTATION — paired-mutation drift probe (NOT a real item)

**Status:** Queued
**Type:** Bug
**Discovered:** meta-test phantom — must be detected as md↔db drift
EOF

bash "$GATE" >/tmp/wi_meta_mut.out 2>&1; mut=$?
ck "mutated (drift) gate FAILS" 1 "$mut"

# RESTORE + reconfirm PASS.
restore; trap - EXIT INT TERM
bash "$GATE" >/tmp/wi_meta_rst.out 2>&1; rst=$?
ck "restored gate PASS again" 0 "$rst"

echo "---"
echo "Passed: $PASS  Failed: $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
echo "PASS: gate genuinely detects md↔db drift (§1.1 honoured)"
exit 0
