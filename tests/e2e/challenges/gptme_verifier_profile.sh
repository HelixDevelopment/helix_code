#!/bin/bash
# gptme port Anti-Bluff Challenge: Verifier profile
# Validates internal/agent/profiles/ verifier profile port (commit aa1489f).
# Article XI §11.9 — every PASS carries positive runtime evidence.
set -e

cd "$(dirname "$0")/../../.."  # repo root

echo "=== gptme Port Anti-Bluff Challenge: Verifier Profile ==="
echo "Verifying verifier profile resolution with positive runtime evidence"

# Step 1: package builds
echo "[1/4] Building internal/agent/profiles/..."
cd helix_code
go build ./internal/agent/profiles/... || (echo "FAIL: profiles package build failed"; exit 1)
echo "  PASS: profiles package builds"

# Step 2: profile unit tests pass (>=4 PASS lines)
echo "[2/4] Running profiles tests..."
TEST_OUT=$(go test -v -count=1 ./internal/agent/profiles/... 2>&1)
PASS_COUNT=$(printf '%s\n' "$TEST_OUT" | grep -c '^--- PASS:' || true)
if [ "$PASS_COUNT" -lt 4 ]; then
    echo "FAIL: expected >=4 PASS lines, got $PASS_COUNT"
    printf '%s\n' "$TEST_OUT"
    exit 1
fi
echo "  PASS: $PASS_COUNT profile tests passed"

# Step 3: anti-bluff grep on production source
echo "[3/4] Anti-bluff grep on profile.go..."
HITS=$(grep -n "simulated\|for now\|TODO implement\|placeholder" \
    internal/agent/profiles/profile.go || true)
if [ -n "$HITS" ]; then
    echo "FAIL: bluff markers found"
    printf '%s\n' "$HITS"
    exit 1
fi
echo "  PASS: no bluff markers in profile.go"

# Step 4: POSITIVE-EVIDENCE PROBE — fetch verifier profile, validate fields.
echo "[4/4] Positive-evidence probe: Get(\"verifier\") fields..."
# Probe MUST live inside the dev.helix.code module tree so that
# `internal/agent/profiles` is importable. Use a unique tmpdir under cmd/.
PROBE_DIR=$(mktemp -d "$PWD/cmd/.gptme-probe-verifier-profile-XXXXXX")
trap 'rm -rf "$PROBE_DIR"' EXIT
PROBE_FILE="$PROBE_DIR/probe.go"
cat > "$PROBE_FILE" <<'EOF'
package main

import (
	"fmt"
	"os"
	"strings"

	"dev.helix.code/internal/agent/profiles"
)

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "CONST-035 violation: %s\n", msg)
	os.Exit(2)
}

func main() {
	p, ok := profiles.Get("verifier")
	if !ok || p == nil {
		fail("Get(\"verifier\") returned nil or not-found")
	}
	prompt := strings.ToLower(p.SystemPrompt)
	hasReviewOrVerify := strings.Contains(prompt, "review") || strings.Contains(prompt, "verify")
	if !hasReviewOrVerify {
		fail("SystemPrompt does not mention review or verify")
	}
	if p.Temperature != 0.1 {
		fail(fmt.Sprintf("Temperature=%v, want 0.1", p.Temperature))
	}
	deniedSet := map[string]bool{}
	for _, d := range p.DeniedToolNames {
		deniedSet[strings.ToLower(d)] = true
	}
	hasGuardrail := deniedSet["fs_write"] || deniedSet["multiedit"] || deniedSet["shell"]
	if !hasGuardrail {
		fail(fmt.Sprintf("DeniedToolNames missing required guardrail tool: %v", p.DeniedToolNames))
	}
	fmt.Printf("name=%s temperature=%v review_or_verify=%v denied_count=%d\n",
		p.Name, p.Temperature, hasReviewOrVerify, len(p.DeniedToolNames))
	fmt.Println("verifier profile: OK")
}
EOF
PROBE_OUT=$(go run "$PROBE_FILE" 2>&1) || {
    echo "FAIL: probe binary failed"
    printf '%s\n' "$PROBE_OUT"
    exit 1
}
printf '  probe output: %s\n' "$PROBE_OUT"
if ! printf '%s\n' "$PROBE_OUT" | grep -q "verifier profile: OK"; then
    echo "FAIL: CONST-035 violation: verifier profile did not pass field checks"
    exit 1
fi
echo "  PASS: verifier profile fields validated"

echo ""
echo "=== ALL CHECKS PASSED ==="
echo "Anti-Bluff Verification (verifier profile port): COMPLETE"
