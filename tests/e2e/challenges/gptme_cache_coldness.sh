#!/bin/bash
# gptme port Anti-Bluff Challenge: CacheAwareness cold-prediction
# Validates internal/llm/cache_control.go CacheAwareness port (commit 4ee0495).
# Article XI §11.9 — every PASS carries positive runtime evidence.
set -e

cd "$(dirname "$0")/../../.."  # repo root

echo "=== gptme Port Anti-Bluff Challenge: Cache Coldness ==="
echo "Verifying CacheAwareness IsCacheLikelyCold with positive runtime evidence"

# Step 1: package builds
echo "[1/4] Building internal/llm/..."
cd helix_code
go build ./internal/llm/... || (echo "FAIL: llm package build failed"; exit 1)
echo "  PASS: llm package builds"

# Step 2: 3 consecutive PASS runs (-count=3 reruns) for the cache tests
echo "[2/4] Running CacheAware tests (3 consecutive runs)..."
TEST_OUT=$(go test -v -count=3 -run 'TestCacheAware|TestCacheControlConfig_ColdThresholdDefault' ./internal/llm/ 2>&1)
if printf '%s\n' "$TEST_OUT" | grep -q '^--- FAIL:'; then
    echo "FAIL: cache awareness tests reported failure"
    printf '%s\n' "$TEST_OUT"
    exit 1
fi
PASS_COUNT=$(printf '%s\n' "$TEST_OUT" | grep -c '^--- PASS:' || true)
# 6 tests x 3 runs = 18 expected PASS lines; require at least 12 as a safety net.
if [ "$PASS_COUNT" -lt 12 ]; then
    echo "FAIL: expected >=12 PASS lines across 3 runs, got $PASS_COUNT"
    printf '%s\n' "$TEST_OUT"
    exit 1
fi
echo "  PASS: $PASS_COUNT cache awareness PASS lines across 3 runs"

# Step 3: anti-bluff grep on production source
echo "[3/4] Anti-bluff grep on cache_control.go..."
HITS=$(grep -n "simulated\|for now\|TODO implement\|placeholder" \
    internal/llm/cache_control.go || true)
if [ -n "$HITS" ]; then
    echo "FAIL: bluff markers found"
    printf '%s\n' "$HITS"
    exit 1
fi
echo "  PASS: no bluff markers in cache_control.go"

# Step 4: POSITIVE-EVIDENCE PROBE — exercise CacheAwareness state machine.
echo "[4/4] Positive-evidence probe: CacheAwareness state transitions..."
# Probe MUST live inside the dev.helix.code module tree so that
# `internal/llm` is importable. Use a unique tmpdir under cmd/.
PROBE_DIR=$(mktemp -d "$PWD/cmd/.gptme-probe-cache-coldness-XXXXXX")
trap 'rm -rf "$PROBE_DIR"' EXIT
PROBE_FILE="$PROBE_DIR/probe.go"
cat > "$PROBE_FILE" <<'EOF'
package main

import (
	"fmt"
	"os"
	"time"

	"dev.helix.code/internal/llm"
)

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "CONST-035 violation: %s\n", msg)
	os.Exit(2)
}

func main() {
	ca := llm.NewCacheAwareness()
	if ca == nil {
		fail("NewCacheAwareness() returned nil")
	}

	now := time.Now()

	// Fresh awareness with no completion: must be cold.
	if !ca.IsCacheLikelyCold(now) {
		fail("fresh CacheAwareness should report cold (no completion recorded)")
	}
	fmt.Printf("step1: fresh -> cold=true OK\n")

	// Record a completion at `now`; one second later it must be hot.
	ca.RecordCompletion(now)
	if ca.IsCacheLikelyCold(now.Add(1 * time.Second)) {
		fail("just-recorded completion should be hot at now+1s, got cold")
	}
	fmt.Printf("step2: recorded -> cold@now+1s=false OK\n")

	// Default threshold must equal 5 minutes.
	if ca.ColdThreshold() != 5*time.Minute {
		fail(fmt.Sprintf("default ColdThreshold()=%v, want %v", ca.ColdThreshold(), 5*time.Minute))
	}
	if llm.DefaultColdThreshold != 5*time.Minute {
		fail(fmt.Sprintf("DefaultColdThreshold=%v, want %v", llm.DefaultColdThreshold, 5*time.Minute))
	}
	fmt.Printf("step3: ColdThreshold=%v OK\n", ca.ColdThreshold())

	fmt.Println("cache coldness: OK")
}
EOF
PROBE_OUT=$(go run "$PROBE_FILE" 2>&1) || {
    echo "FAIL: probe binary failed"
    printf '%s\n' "$PROBE_OUT"
    exit 1
}
printf '  probe output:\n%s\n' "$PROBE_OUT"
if ! printf '%s\n' "$PROBE_OUT" | grep -q "cache coldness: OK"; then
    echo "FAIL: CONST-035 violation: cache coldness state transitions invalid"
    exit 1
fi
echo "  PASS: CacheAwareness state transitions validated"

echo ""
echo "=== ALL CHECKS PASSED ==="
echo "Anti-Bluff Verification (cache coldness port): COMPLETE"
