#!/usr/bin/env bash
# scripts/tests/no_terminal_item_in_issues_meta_test.sh — §1.1 paired-mutation
# meta-test for scripts/gates/no_terminal_item_in_issues_gate.sh
# (CM-NO-TERMINAL-ITEM-IN-ISSUES).
#
# HXC-126 permanent regression guard (§11.4.135): plants a phantom item carrying
# a terminal §11.4.15 status into docs/Issues.md, asserts the gate FAILS, then
# restores and asserts the gate PASSES. Proves the gate cannot bluff (a genuine
# terminal-status-leaked-into-Issues defect is mechanically detected).
#
# The mutation is on the REAL docs/Issues.md but is GUARANTEED restored via a
# trap (even on interrupt) — the original is byte-restored from a backup.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

GATE="scripts/gates/no_terminal_item_in_issues_gate.sh"
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

echo "=== no_terminal_item_in_issues_gate §1.1 paired-mutation meta-test ==="

# Baseline: gate should PASS on the real, already-repaired tree.
bash "$GATE" >/tmp/ntii_meta_base.out 2>&1; base=$?
ck "baseline gate PASS (zero terminal items leaked into Issues.md)" 0 "$base"

# MUTATION: append a phantom item carrying a TERMINAL status under an "## <ID>"
# heading in docs/Issues.md — exactly the HXC-126 defect shape.
cat >> "$ISSUES" <<'EOF'

## HXC-META-MUTATION — paired-mutation terminal-in-Issues probe (NOT a real item)

**Status:** Fixed (→ Fixed.md)
**Type:** Bug
**Discovered:** meta-test phantom — must be detected as a terminal-status leak
EOF

bash "$GATE" >/tmp/ntii_meta_mut.out 2>&1; mut=$?
ck "mutated (terminal-status-in-Issues) gate FAILS" 1 "$mut"

# RESTORE + reconfirm PASS.
restore; trap - EXIT INT TERM
bash "$GATE" >/tmp/ntii_meta_rst.out 2>&1; rst=$?
ck "restored gate PASS again" 0 "$rst"

echo "---"
echo "Passed: $PASS  Failed: $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
echo "PASS: gate genuinely detects a terminal-status item leaked into Issues.md (§1.1 honoured)"
exit 0
