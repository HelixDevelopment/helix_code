#!/usr/bin/env bash
# scaling_hits_helixcode_meta_test.sh — §1.1 paired-mutation meta-test for the
# CM-SCALING-HITS-HELIXCODE gate (SP7 A5).
#
# Plants a MUTATED scaling_throughput.json (flat: gain below threshold) and asserts
# the gate FAILs; then writes a GOOD one (real scale-out) and asserts the gate
# PASSes. Exit non-zero if any assertion fails.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/scaling_hits_helixcode_gate.sh"

FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/scaling_gate_meta.XXXXXX")"
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

echo "=== meta-test: CM-SCALING-HITS-HELIXCODE paired mutation ==="
echo

# --- (1) MUTATED: flat throughput (gain below threshold) --------------------
mkdir -p "$FIXTURE_DIR/run/scaling_bad"
cat > "$FIXTURE_DIR/run/scaling_bad/scaling_throughput.json" <<'EOF'
{
  "name": "scaling_bad",
  "steps": [
    {"n_workers": 1, "throughput_tps": 1000.0},
    {"n_workers": 2, "throughput_tps": 1010.0},
    {"n_workers": 4, "throughput_tps": 1005.0},
    {"n_workers": 8, "throughput_tps": 1002.0}
  ],
  "gain_at_max_n": 1.0,
  "min_gain_threshold": 1.5
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out1.txt" 2>&1
rc1=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out1.txt"
assert "gate FAILs on a flat (gain below threshold) scaling report" 1 "$rc1"
echo

# --- (2) FIXED: real scale-out ----------------------------------------------
rm -rf "$FIXTURE_DIR/run"
mkdir -p "$FIXTURE_DIR/run/scaling_good"
cat > "$FIXTURE_DIR/run/scaling_good/scaling_throughput.json" <<'EOF'
{
  "name": "scaling_good",
  "steps": [
    {"n_workers": 1, "throughput_tps": 536.0},
    {"n_workers": 2, "throughput_tps": 1224.0},
    {"n_workers": 4, "throughput_tps": 2082.0},
    {"n_workers": 8, "throughput_tps": 3905.0}
  ],
  "gain_at_max_n": 7.28,
  "min_gain_threshold": 1.5
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out2.txt" 2>&1
rc2=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out2.txt"
assert "gate PASSes on a real scale-out scaling report" 0 "$rc2"
echo

if [[ "$fail_meta" -ne 0 ]]; then
    echo "META-TEST FAIL: paired-mutation assertions did not hold"
    exit 1
fi
echo "META-TEST PASS: CM-SCALING-HITS-HELIXCODE FAILs the flat bluff, PASSes real scale-out"
exit 0
