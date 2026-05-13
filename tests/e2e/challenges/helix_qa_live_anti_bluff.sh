#!/usr/bin/env bash
# helix_qa_live_anti_bluff.sh — anti-bluff QA Challenge per operator
# mandate 2026-05-13:
#
#   "all tests and Challenges do work in anti-bluff manner — they
#    MUST confirm that all tested codebase really works as expected"
#
# Boots HelixCode against the mistborn-distributed Postgres+Redis
# (via SSH tunnels established by scripts/mistborn-up.sh), runs the
# helix-qa Go harness, asserts that every check PASSes with real
# captured wire evidence, and writes a per-session evidence dir
# under docs/qa_evidence/ (gitignored).
#
# Per CONST-035 / Article XI §11.9, a PASS in this Challenge means:
#   1. HelixCode actually served the documented endpoint contract
#   2. The response was captured to disk with status + body excerpt
#   3. The per-check JSON evidence file exists with body_bytes > 0
#      AND result=="PASS" (no metadata-only / absence-of-error PASS)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

echo "=== helix-qa Anti-Bluff Live-Server Challenge ==="

# Step 1: Verify mistborn tunnels are up (15432 = postgres).
echo "[1/5] Checking mistborn tunnels..."
if ! (echo > /dev/tcp/127.0.0.1/15432) 2>/dev/null; then
    echo "  SKIP: mistborn postgres tunnel not bound on :15432"
    echo "  Run \`bash scripts/mistborn-up.sh\` first; SKIP-OK: #env-mistborn-offline"
    exit 0
fi
echo "  PASS: 127.0.0.1:15432 reachable"

# Step 2: Ensure helix-qa binary is built.
echo "[2/5] Building helix-qa..."
HQ_BIN="/tmp/helix-qa-${USER}-$$"
trap 'rm -f "$HQ_BIN"' EXIT
go build -o "$HQ_BIN" scripts/qa/main.go
echo "  PASS: $HQ_BIN built"

# Step 3: Ensure HelixCode server is reachable. If not, start it briefly.
echo "[3/5] Checking HelixCode server on :8080..."
if ! curl -fsS -m 3 http://localhost:8080/health >/dev/null 2>&1; then
    echo "  HelixCode not up; booting briefly for the Challenge..."
    HC_BIN="/tmp/helixcode-server-${USER}-$$"
    (cd HelixCode && go build -o "$HC_BIN" ./cmd/server)
    HELIX_DATABASE_HOST=127.0.0.1 HELIX_DATABASE_PORT=15432 \
        HELIX_DATABASE_USER=helix HELIX_DATABASE_NAME=helixcode_prod \
        HELIX_DATABASE_PASSWORD=helixpass \
        HELIX_REDIS_HOST=127.0.0.1 HELIX_REDIS_PORT=16379 HELIX_REDIS_PASSWORD="" \
        nohup "$HC_BIN" >"/tmp/hcsrv-$$.log" 2>&1 &
    HC_PID=$!
    trap 'kill $HC_PID 2>/dev/null; rm -f "$HQ_BIN" "$HC_BIN" /tmp/hcsrv-$$.log' EXIT
    # Wait up to 30 s for /health to come up.
    for _ in $(seq 1 30); do
        sleep 1
        if curl -fsS -m 1 http://localhost:8080/health >/dev/null 2>&1; then break; fi
    done
    if ! curl -fsS -m 3 http://localhost:8080/health >/dev/null 2>&1; then
        echo "  FAIL: HelixCode never became reachable"; exit 1
    fi
fi
echo "  PASS: HelixCode /health responsive"

# Step 4: Run helix-qa with per-session evidence dir.
SESSION_TS="$(date -u +%Y%m%dT%H%M%SZ)"
SESSION_DIR="docs/qa_evidence/qa_session_${SESSION_TS}_challenge"
mkdir -p "$SESSION_DIR"
echo "[4/5] Running helix-qa..."
"$HQ_BIN" -base http://localhost:8080 -evidence-dir "$SESSION_DIR"
echo "  PASS: helix-qa exited 0"

# Step 5: Positive-evidence verification per CONST-035.
echo "[5/5] Verifying captured evidence..."
if [ ! -d "$SESSION_DIR/evidence" ]; then
    echo "  FAIL: no evidence dir produced"; exit 1
fi
EV_COUNT="$(ls "$SESSION_DIR/evidence/" | wc -l | tr -d ' ')"
if [ "$EV_COUNT" -lt 6 ]; then
    echo "  FAIL: only $EV_COUNT evidence files (expected ≥ 6)"; exit 1
fi
# Every file MUST have result==PASS AND body_bytes>0.
# `grep -l` exits 1 when nothing matches AND we have `set -o pipefail`,
# so the no-match case (which is the desired state for the empty-body
# check) would kill the script before the "ALL CHECKS PASSED" emit.
# Wrap with `|| true` to swallow the harmless non-match exit code.
PASS_COUNT="$( (grep -l '"result": "PASS"' "$SESSION_DIR/evidence/"*.json 2>/dev/null || true) | wc -l | tr -d ' ')"
EMPTY_BODY="$( (grep -l '"body_bytes": 0' "$SESSION_DIR/evidence/"*.json 2>/dev/null || true) | wc -l | tr -d ' ')"
if [ "$PASS_COUNT" -lt "$EV_COUNT" ]; then
    echo "  FAIL: $(($EV_COUNT - $PASS_COUNT)) evidence file(s) not PASS"
    exit 1
fi
if [ "$EMPTY_BODY" -gt 0 ]; then
    echo "  FAIL: $EMPTY_BODY evidence file(s) have body_bytes=0 (CONST-035 violation)"
    exit 1
fi
echo "  PASS: $EV_COUNT/$EV_COUNT evidence files all PASS with body_bytes>0"
echo
echo "=== ALL CHECKS PASSED ==="
echo "Anti-Bluff Verification (helix-qa live-server): COMPLETE"
echo "Session evidence: $SESSION_DIR"
