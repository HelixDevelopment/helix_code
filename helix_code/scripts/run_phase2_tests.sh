#!/bin/bash
# Phase 2 Test Runner — Real-infra-only verification
# Runs ALL unit tests + challenge harnesses for Phase 2 features (F21-F30).
# Exit 0 = all pass. Exit 1 = any failure detected.
# Article XI §11.9: every PASS carries positive runtime evidence.

set -e
cd "$(dirname "$0")/.."
FAILURES=0
PASSED=0

log() { echo "[$(date +%H:%M:%S)] $*"; }

# ── Unit tests (all Phase 2 packages) ──
log "=== Phase 2 unit tests ==="
PKGS=(
    "./internal/approval/..."
    "./internal/autocommit/..."
    "./internal/tools/browser/..."
    "./internal/projectmemory/..."
    "./internal/plantree/..."
    "./internal/workspace/..."
    "./internal/planner/..."
    "./internal/voice/..."
    "./internal/kilocode/..."
    "./internal/roocode/..."
    "./internal/continua/..."
)
for pkg in "${PKGS[@]}"; do
    log "Testing $pkg..."
    if go test -count=1 -race "$pkg" 2>&1 > /tmp/p2_test.log; then
        PASSED=$((PASSED+1))
        log "  PASS: $pkg"
    else
        FAILURES=$((FAILURES+1))
        log "  FAIL: $pkg"
        cat /tmp/p2_test.log | tail -5
    fi
done

# ── Challenge harnesses ──
log "=== Phase 2 challenge harnesses ==="
CHALLENGES=(
    "p2f25_challenge:./tests/integration/cmd/p2f25_challenge/"
    "p2f26_challenge:./tests/integration/cmd/p2f26_challenge/"
    "p2f27_challenge:./tests/integration/cmd/p2f27_challenge/"
    "p2f28_challenge:./tests/integration/cmd/p2f28_challenge/"
    "p2f29_challenge:./tests/integration/cmd/p2f29_challenge/"
    "p2f30_challenge:./tests/integration/cmd/p2f30_challenge/"
)
for entry in "${CHALLENGES[@]}"; do
    name="${entry%%:*}"
    path="${entry##*:}"
    log "Building $name..."
    if go build -o "/tmp/$name" "$path" 2>&1 > /tmp/p2_build.log; then
        log "Running $name..."
        if timeout 120 "/tmp/$name" 2>&1 > /tmp/p2_run.log; then
            PASSED=$((PASSED+1))
            log "  PASS: $name ($(tail -2 /tmp/p2_run.log | tr '\n' ' '))"
        else
            FAILURES=$((FAILURES+1))
            log "  FAIL: $name"
            tail -5 /tmp/p2_run.log
        fi
        rm -f "/tmp/$name"
    else
        FAILURES=$((FAILURES+1))
        log "  BUILD FAIL: $name"
        cat /tmp/p2_build.log | tail -3
    fi
done

# ── Anti-bluff smoke ──
log "=== Anti-bluff smoke ==="
BLUFF_COUNT=$(grep -rn "simulated\|for now\|TODO implement\|placeholder" \
    internal/approval internal/autocommit internal/tools/browser \
    internal/projectmemory internal/plantree internal/workspace \
    internal/planner internal/voice internal/kilocode \
    internal/roocode internal/continua \
    --include="*.go" 2>/dev/null | grep -v "_test.go" | wc -l)

if [ "$BLUFF_COUNT" -eq 0 ]; then
    log "  Anti-bluff: clean"
else
    log "  Anti-bluff: $BLUFF_COUNT matches found"
    FAILURES=$((FAILURES+1))
fi

# ── Summary ──
log "========================================="
log "Phase 2 Test Results: $PASSED passed, $FAILURES failed"
log "========================================="

if [ "$FAILURES" -eq 0 ]; then
    log "ALL PHASE 2 TESTS AND CHALLENGES PASS"
    exit 0
else
    log "FAILURES DETECTED"
    exit 1
fi
