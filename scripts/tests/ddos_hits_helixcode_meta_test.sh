#!/usr/bin/env bash
# ddos_hits_helixcode_meta_test.sh — §1.1 paired-mutation meta-test for the
# CM-DDOS-HITS-HELIXCODE gate (SP7 A5).
#
# Plants a MUTATED flood_report.json (config-only: zero served responses + a 5xx
# storm) and asserts the gate FAILs; then writes a GOOD flood_report.json and
# asserts the gate PASSes. Exit non-zero if any assertion fails.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/ddos_hits_helixcode_gate.sh"

FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ddos_gate_meta.XXXXXX")"
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

echo "=== meta-test: CM-DDOS-HITS-HELIXCODE paired mutation ==="
echo

# --- (1) MUTATED: config-only / 5xx-storm flood report ----------------------
mkdir -p "$FIXTURE_DIR/run/ddos_bad"
cat > "$FIXTURE_DIR/run/ddos_bad/flood_report.json" <<'EOF'
{
  "name": "ddos_bad",
  "endpoint": "",
  "requests_sent": 0,
  "status_2xx": 0,
  "status_5xx": 500,
  "body_marker_hits": 0,
  "p99_under_flood_ms": 0
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out1.txt" 2>&1
rc1=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out1.txt"
assert "gate FAILs on a config-only / 5xx-storm flood report" 1 "$rc1"
echo

# --- (2) FIXED: real flood report -------------------------------------------
rm -rf "$FIXTURE_DIR/run"
mkdir -p "$FIXTURE_DIR/run/ddos_good"
cat > "$FIXTURE_DIR/run/ddos_good/flood_report.json" <<'EOF'
{
  "name": "ddos_good",
  "endpoint": "http://127.0.0.1:8080/health",
  "requests_sent": 1600,
  "status_2xx": 1600,
  "status_5xx": 0,
  "body_marker_hits": 1600,
  "p99_under_flood_ms": 3.14
}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out2.txt" 2>&1
rc2=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out2.txt"
assert "gate PASSes on a real HelixCode flood report" 0 "$rc2"
echo

if [[ "$fail_meta" -ne 0 ]]; then
    echo "META-TEST FAIL: paired-mutation assertions did not hold"
    exit 1
fi
echo "META-TEST PASS: CM-DDOS-HITS-HELIXCODE FAILs the bluff, PASSes the real evidence"
exit 0
