#!/bin/bash
# gptme port Anti-Bluff Challenge: Subagent Role-typed posture
# Validates internal/agent/subagent/ Role + ApplyRoleDefaults port (commit aa1489f).
# Article XI §11.9 — every PASS carries positive runtime evidence.
set -e

cd "$(dirname "$0")/../../.."  # repo root

echo "=== gptme Port Anti-Bluff Challenge: Subagent Role ==="
echo "Verifying Role-typed posture (ApplyRoleDefaults) with positive runtime evidence"

# Step 1: package builds
echo "[1/4] Building internal/agent/subagent/..."
cd HelixCode
go build ./internal/agent/subagent/... || (echo "FAIL: subagent package build failed"; exit 1)
echo "  PASS: subagent package builds"

# Step 2: Role-related unit tests pass (>=4 PASS lines)
echo "[2/4] Running Role / ApplyRoleDefaults tests..."
TEST_OUT=$(go test -v -count=1 -run 'TestSubagent.*Role|TestApplyRoleDefaults' ./internal/agent/subagent/... 2>&1)
PASS_COUNT=$(printf '%s\n' "$TEST_OUT" | grep -c '^--- PASS:' || true)
if [ "$PASS_COUNT" -lt 4 ]; then
    echo "FAIL: expected >=4 PASS lines, got $PASS_COUNT"
    printf '%s\n' "$TEST_OUT"
    exit 1
fi
echo "  PASS: $PASS_COUNT Role/ApplyRoleDefaults tests passed"

# Step 3: anti-bluff grep on production source
echo "[3/4] Anti-bluff grep on types.go and manager.go..."
HITS=$(grep -n "simulated\|for now\|TODO implement\|placeholder" \
    internal/agent/subagent/types.go internal/agent/subagent/manager.go || true)
if [ -n "$HITS" ]; then
    echo "FAIL: bluff markers found"
    printf '%s\n' "$HITS"
    exit 1
fi
echo "  PASS: no bluff markers in types.go / manager.go"

# Step 4: POSITIVE-EVIDENCE PROBE — construct SubagentTask{Role: RoleVerify},
# call ApplyRoleDefaults, assert resulting Isolation is "none".
echo "[4/4] Positive-evidence probe: ApplyRoleDefaults() output..."
# Probe MUST live inside the dev.helix.code module tree so that
# `internal/agent/subagent` is importable. Use a unique tmpdir under cmd/.
PROBE_DIR=$(mktemp -d "$PWD/cmd/.gptme-probe-subagent-role-XXXXXX")
trap 'rm -rf "$PROBE_DIR"' EXIT
PROBE_FILE="$PROBE_DIR/probe.go"
cat > "$PROBE_FILE" <<'EOF'
package main

import (
	"fmt"
	"os"

	"dev.helix.code/internal/agent/subagent"
)

func main() {
	t := subagent.SubagentTask{Role: subagent.RoleVerify}
	t.ApplyRoleDefaults()
	fmt.Printf("role=%s isolation=%s read_only=%v\n", t.Role, t.Isolation, t.ReadOnlyByDefault)
	if t.Isolation != subagent.IsolationNone {
		fmt.Fprintf(os.Stderr, "CONST-035 violation: role policy not applied (isolation=%q, want %q)\n",
			t.Isolation, subagent.IsolationNone)
		os.Exit(2)
	}
}
EOF
PROBE_OUT=$(go run "$PROBE_FILE" 2>&1) || {
    echo "FAIL: probe binary failed"
    printf '%s\n' "$PROBE_OUT"
    exit 1
}
printf '  probe output: %s\n' "$PROBE_OUT"
if ! printf '%s\n' "$PROBE_OUT" | grep -q "isolation=none"; then
    echo "FAIL: CONST-035 violation: role policy not applied (expected isolation=none)"
    exit 1
fi
echo "  PASS: ApplyRoleDefaults() sets isolation=none for RoleVerify"

echo ""
echo "=== ALL CHECKS PASSED ==="
echo "Anti-Bluff Verification (subagent Role port): COMPLETE"
