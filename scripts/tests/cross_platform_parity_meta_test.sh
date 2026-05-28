#!/usr/bin/env bash
# cross_platform_parity_meta_test.sh — §1.1 paired-mutation meta-test for the
# CM-CROSS-PLATFORM-PARITY gate (§11.4.81, HXC-015).
#
# Plants a fixture script that CLAIMS `case "$(uname -s)"` dispatch but OMITS a
# manifest host-shell platform's branch with NO honest-gap citation:
#   (1) assert the gate FAILs (exit 1) on the mutated fixture;
#   (2) fix the fixture (add the missing branch);
#   (3) assert the gate PASSes (exit 0).
# Exit non-zero if any assertion fails. Cleans up the fixture dir in a trap.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$ROOT/scripts/gates/cross_platform_parity_gate.sh"

FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cpp_parity_meta.XXXXXX")"
cleanup() { rm -rf "$FIXTURE_DIR"; }
trap cleanup EXIT

# The gate reads the real manifest at $ROOT/docs/platforms/supported_platforms.yaml
# (host-shell platforms: Linux, Darwin, Windows_NT). We scan only the fixture
# dir by passing it as $1, so we never touch real repo scripts.

fail_meta=0
assert() {
    # assert <desc> <expected-exit> <actual-exit>
    if [[ "$2" -eq "$3" ]]; then
        echo "  ASSERT PASS — $1 (exit=$3)"
    else
        echo "  ASSERT FAIL — $1 (expected exit=$2, got exit=$3)"
        fail_meta=1
    fi
}

echo "=== meta-test: CM-CROSS-PLATFORM-PARITY paired mutation ==="
echo "Manifest under test: docs/platforms/supported_platforms.yaml"
echo "Fixture scan dir:    $FIXTURE_DIR"
echo

# --- (1) MUTATED fixture: dispatch present, Darwin branch MISSING, no gap ----
cat > "$FIXTURE_DIR/bad_dispatch.sh" <<'EOF'
#!/usr/bin/env bash
# A script that claims multi-platform dispatch but drops the macOS branch
# (no honest gap citation) — this is the bluff the gate must catch.
case "$(uname -s)" in
    Linux)      echo "linux path: cgroup limit" ;;
    Windows_NT) echo "windows path: job object" ;;
    *)          echo "unknown" ;;
esac
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out1.txt" 2>&1
rc1=$?
set -e
echo "--- gate output on MUTATED fixture ---"
sed 's/^/    /' "$FIXTURE_DIR/out1.txt"
assert "gate FAILs when a uname-dispatch script omits Darwin (no gap citation)" 1 "$rc1"
echo

# --- (2) FIXED fixture: add the missing Darwin branch ------------------------
cat > "$FIXTURE_DIR/bad_dispatch.sh" <<'EOF'
#!/usr/bin/env bash
# Now covers all host-shell platforms.
case "$(uname -s)" in
    Linux)      echo "linux path: cgroup limit" ;;
    Darwin)     echo "macos path: launchd / RLIMIT_CPU proxy" ;;
    Windows_NT) echo "windows path: job object" ;;
    *)          echo "unknown" ;;
esac
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out2.txt" 2>&1
rc2=$?
set -e
echo "--- gate output on FIXED fixture ---"
sed 's/^/    /' "$FIXTURE_DIR/out2.txt"
assert "gate PASSes once all host-shell platform branches present" 0 "$rc2"
echo

# --- (3) bonus: honest-gap citation also satisfies the gate ------------------
cat > "$FIXTURE_DIR/bad_dispatch.sh" <<'EOF'
#!/usr/bin/env bash
# Darwin intentionally uncovered, but with an honest kernel-gap citation.
# PARITY-GAP: Darwin — XNU does not enforce RLIMIT_AS for unprivileged procs;
#   no equivalent primitive; macOS path deferred to F14.5 Seatbelt work.
case "$(uname -s)" in
    Linux)      echo "linux path" ;;
    Windows_NT) echo "windows path" ;;
    *)          echo "unknown" ;;
esac
EOF

set +e
"$GATE" "$FIXTURE_DIR" > "$FIXTURE_DIR/out3.txt" 2>&1
rc3=$?
set -e
echo "--- gate output on honest-GAP fixture ---"
sed 's/^/    /' "$FIXTURE_DIR/out3.txt"
assert "gate PASSes when uncovered platform carries an honest PARITY-GAP citation" 0 "$rc3"
echo

if [[ "$fail_meta" -ne 0 ]]; then
    echo "META-TEST FAIL: paired-mutation assertions did not hold"
    exit 1
fi
echo "META-TEST PASS: gate FAILs on the bluff, PASSes on the fix and on honest-gap"
exit 0
