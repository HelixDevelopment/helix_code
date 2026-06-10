#!/usr/bin/env bash
# ui_hits_helixcode_meta_test.sh — §1.1 paired-mutation meta-test for the
# CM-UI-HITS-HELIXCODE gate (SP7 A5).
#
# Plants a MUTATED rendered_cells.json (empty grid / found:false) and asserts the
# gate FAILs; then writes a GOOD one (real composited cells, all found) and asserts
# the gate PASSes. Exit non-zero if any assertion fails.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/ui_hits_helixcode_gate.sh"

FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ui_gate_meta.XXXXXX")"
cleanup() { rm -rf "$FIXTURE_DIR"; }
trap cleanup EXIT

fail_meta=0
assert() {
    if [[ "$2" -eq "$3" ]]; then
        echo "  ASSERT PASS — $1 (exit=$3)"
    else
        echo "  ASSERT FAIL — $1 (expected exit=$2, got exit=$3)"
        fail_meta=1
    fi
}

echo "=== meta-test: CM-UI-HITS-HELIXCODE paired mutation ==="
echo

# --- (1) MUTATED: empty grid + a found:false entry --------------------------
mkdir -p "$FIXTURE_DIR/run/ui_bad"
cat > "$FIXTURE_DIR/run/ui_bad/rendered_cells.json" <<'EOF'
{
  "name": "ui_bad",
  "width": 0,
  "height": 0,
  "asserted_strings": [
    {"text": "Main Menu", "found": false}
  ]
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out1.txt" 2>&1
rc1=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out1.txt"
assert "gate FAILs on an empty-grid / found:false render report" 1 "$rc1"
echo

# --- (2) FIXED: real composited cells, all found ----------------------------
rm -rf "$FIXTURE_DIR/run"
mkdir -p "$FIXTURE_DIR/run/ui_good"
cat > "$FIXTURE_DIR/run/ui_good/rendered_cells.json" <<'EOF'
{
  "name": "ui_good",
  "width": 80,
  "height": 25,
  "asserted_strings": [
    {"text": "Main Menu", "found": true},
    {"text": "List Models", "found": true},
    {"text": "Generate", "found": true}
  ]
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out2.txt" 2>&1
rc2=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out2.txt"
assert "gate PASSes on a real composited-cells render report" 0 "$rc2"
echo

if [[ "$fail_meta" -ne 0 ]]; then
    echo "META-TEST FAIL: paired-mutation assertions did not hold"
    exit 1
fi
echo "META-TEST PASS: CM-UI-HITS-HELIXCODE FAILs the empty-grid bluff, PASSes a real render"
exit 0
