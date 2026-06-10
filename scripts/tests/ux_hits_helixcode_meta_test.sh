#!/usr/bin/env bash
# ux_hits_helixcode_meta_test.sh — §1.1 paired-mutation meta-test for the
# CM-UX-HITS-HELIXCODE gate (SP7 A5).
#
# Plants a MUTATED journey_transcript.jsonl (one-sided / canned) and asserts the
# gate FAILs; then writes a GOOD bidirectional transcript and asserts the gate
# PASSes. Exit non-zero if any assertion fails.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/ux_hits_helixcode_gate.sh"

FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ux_gate_meta.XXXXXX")"
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

echo "=== meta-test: CM-UX-HITS-HELIXCODE paired mutation ==="
echo

# --- (1) MUTATED: one-sided + canned-bluff transcript -----------------------
mkdir -p "$FIXTURE_DIR/run/ux_bad"
cat > "$FIXTURE_DIR/run/ux_bad/journey_transcript.jsonl" <<'EOF'
{"step":"canned","command_sent":"cli generate","response_received":"This is a simulated response","assertion":"real output","verdict":"PASS"}
{"step":"oneside","command_sent":"","response_received":"only a response, no command","assertion":"x","verdict":"PASS"}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out1.txt" 2>&1
rc1=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out1.txt"
assert "gate FAILs on a canned-bluff response" 1 "$rc1"
echo

# --- (2) FIXED: real bidirectional transcript -------------------------------
rm -rf "$FIXTURE_DIR/run"
mkdir -p "$FIXTURE_DIR/run/ux_good"
cat > "$FIXTURE_DIR/run/ux_good/journey_transcript.jsonl" <<'EOF'
{"step":"command_exec","command_sent":"cli -command 'echo helixcode-ux-probe-123'","response_received":"stdout: helixcode-ux-probe-123\nexit code: 0","assertion":"real echoed stdout","verdict":"PASS"}
{"step":"health_check","command_sent":"cli -health","response_received":"server: healthy","assertion":"real non-empty output","verdict":"PASS"}
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out2.txt" 2>&1
rc2=$?
set -e
sed 's/^/    /' "$FIXTURE_DIR/out2.txt"
assert "gate PASSes on a real bidirectional CLI journey transcript" 0 "$rc2"
echo

if [[ "$fail_meta" -ne 0 ]]; then
    echo "META-TEST FAIL: paired-mutation assertions did not hold"
    exit 1
fi
echo "META-TEST PASS: CM-UX-HITS-HELIXCODE FAILs the canned/one-sided bluff, PASSes a real journey"
exit 0
