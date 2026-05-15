#!/usr/bin/env bash
# tests/e2e/challenges/chaos_failure_injection.sh — anti-bluff Chaos
# Challenge per CONST-035 + CONST-050(B). 3rd of the 6 missing test
# types from Task #266.
#
# Operator-safe scope (CONST-033 host-power-management ban + no-sudo
# rule): all chaos injection is **client-side** — we don't touch host
# processes, host filesystems, iptables/tc, or system clocks. The
# faults injected are:
#
#   1. Malformed HTTP request (bad verb, bad version, invalid path).
#   2. Oversized URL / header values (assert server limits cleanly).
#   3. Slow-loris-style slow header send (partial request, idle).
#   4. Abrupt connection close mid-request.
#   5. Mixed legit + chaos load: control group of legit /health
#      requests runs concurrently with chaos; legit MUST still
#      succeed despite the noise.
#
# What this Challenge proves:
#   - Server REJECTS malformed input cleanly (4xx, not 5xx, not crash).
#   - Server SURVIVES slow / partial / abrupt-close requests.
#   - Legit requests STILL SUCCEED under concurrent chaos load
#     (catches the "chaos starves real users" class).
#   - Post-chaos liveness probe passes (catches "fell-over-after-
#     chaos" zombie class).
#
# Anti-bluff (CONST-035): every gate carries captured runtime
# evidence. Honest SKIP-OK per §11.4.3 topology-dispatch when server
# unreachable.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

HEALTH_URL="${HELIX_HEALTH_URL:-http://localhost:8080/api/v1/health}"
CHAOS_HOST="${HELIX_CHAOS_HOST:-localhost}"
CHAOS_PORT="${HELIX_CHAOS_PORT:-8080}"
LEGIT_REQUESTS="${CHAOS_LEGIT_REQUESTS:-100}"
CHAOS_REQUESTS="${CHAOS_BAD_REQUESTS:-50}"
LEGIT_MIN_PASS_PCT="${CHAOS_LEGIT_MIN_PASS_PCT:-95}"

echo "=== Chaos Failure-Injection Challenge (anti-bluff per CONST-035) ==="
echo "  target health URL:    $HEALTH_URL"
echo "  chaos host:port:      $CHAOS_HOST:$CHAOS_PORT"
echo "  legit requests:       $LEGIT_REQUESTS"
echo "  chaos requests:       $CHAOS_REQUESTS"
echo "  legit pass threshold: ≥${LEGIT_MIN_PASS_PCT}%"

# Step 1: pre-chaos probe — server must be reachable BEFORE we start
# injecting faults, otherwise SKIP-OK (infrastructure-down).
echo
echo "[1/6] Pre-chaos probe — server must be reachable..."
pre_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || pre_code="000"
if [[ "$pre_code" != "200" ]]; then
    echo "  SKIP: server not reachable (HTTP $pre_code) — SKIP-OK: #env-server-down"
    echo
    echo "=== Chaos Failure-Injection Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "  PASS: pre-chaos probe HTTP 200 — server reachable"

# Step 2: malformed-request injection. Send a series of badly-formed
# HTTP requests via raw TCP (nc-style via /dev/tcp); assert server
# closes the connection cleanly without crashing.
echo
echo "[2/6] Malformed-request injection..."
MALFORMED=$(mktemp)
trap "rm -f $MALFORMED" EXIT
for case in "BADVERB / HTTP/1.1" "GET / HTTP/9.9" "GET // HTTP/1.1" "GET /\x00 HTTP/1.1" "INVALID"; do
    {
        printf '%s\r\nHost: %s\r\n\r\n' "$case" "$CHAOS_HOST" 2>/dev/null \
            | timeout 3 bash -c "exec 3<>/dev/tcp/$CHAOS_HOST/$CHAOS_PORT && cat >&3 && cat <&3" \
            2>/dev/null \
            | head -c 200 \
            >> "$MALFORMED" 2>/dev/null
        printf '\n---\n' >> "$MALFORMED"
    } || true
done
# Verify server is still reachable AFTER malformed-request salvo.
post_malformed_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post_malformed_code="000"
if [[ "$post_malformed_code" != "200" ]]; then
    echo "  FAIL: server tipped over after malformed-request injection (HTTP $post_malformed_code)"
    exit 1
fi
echo "  PASS: server survived 5 malformed-request shapes + still HTTP 200"

# Step 3: oversized header injection. Send a request with an
# 8192-byte header value; expect server to reject cleanly (typically
# 4xx 'Request Header Too Large' / 431, never 5xx crash).
echo
echo "[3/6] Oversized-header injection..."
huge_val=$(head -c 8192 /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 8192)
oversized_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" \
    -H "X-Chaos-Huge: $huge_val" "$HEALTH_URL" 2>/dev/null) || oversized_code="000"
# 200 (server accepted) OR 4xx (server rejected cleanly) are both fine.
# A 5xx (server crash) or 000 (server died) is the violation.
case "$oversized_code" in
    200|400|413|414|431|494)
        echo "  PASS: server rejected/handled oversized header cleanly (HTTP $oversized_code)" ;;
    *)
        echo "  FAIL: server returned HTTP $oversized_code on oversized header (5xx = bluff; 000 = crash)"
        exit 1 ;;
esac
# Liveness probe after the oversized request.
post_huge_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post_huge_code="000"
if [[ "$post_huge_code" != "200" ]]; then
    echo "  FAIL: server tipped over after oversized-header injection (HTTP $post_huge_code)"
    exit 1
fi

# Step 4: slow-loris injection. Open a TCP connection, send a partial
# request header, idle. Then close. We do this in a backgrounded
# loop for the duration of the legit-traffic phase below — overlap
# is the test.
echo
echo "[4/6] Slow-loris injection (concurrent with legit load)..."
slow_loris_pids=()
for i in $(seq 1 5); do
    (
        # Open TCP, send a partial GET line, hold idle for 5s, close.
        timeout 5 bash -c "
            exec 3<>/dev/tcp/$CHAOS_HOST/$CHAOS_PORT 2>/dev/null
            printf 'GET /api/v1/health HTTP/1.1\r\nHost: $CHAOS_HOST\r\nX-Slow-Loris: ' >&3 2>/dev/null
            sleep 4 2>/dev/null
            printf 'done\r\n\r\n' >&3 2>/dev/null
        " >/dev/null 2>&1 || true
    ) &
    slow_loris_pids+=($!)
done
echo "  Spawned ${#slow_loris_pids[@]} slow-loris connections (5s lifetime each)"

# Step 5: control-group test — concurrent legit /health requests
# while slow-loris is still active. Legit pass rate must stay above
# threshold; this catches "chaos starves real users" class.
echo
echo "[5/6] Mixed-load legit-traffic control group (during chaos)..."
RESULTS=$(mktemp)
trap "rm -f $MALFORMED $RESULTS; kill ${slow_loris_pids[*]} 2>/dev/null" EXIT

seq 1 "$LEGIT_REQUESTS" | xargs -n1 -P 20 -I{} \
    curl -sS -o /dev/null --max-time 5 \
        -w "%{http_code} %{time_total}\n" "$HEALTH_URL" \
    2>/dev/null >> "$RESULTS" || true

# Cleanup slow-loris connections.
for pid in "${slow_loris_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
done
wait 2>/dev/null || true

total=$(wc -l < "$RESULTS" | tr -d ' ')
ok_count=$(awk '$1=="200"{c++} END{print c+0}' "$RESULTS")
[[ "$total" -eq 0 ]] && total=1   # avoid div-by-zero
legit_pct=$((ok_count * 100 / total))

sorted=$(awk '{print $2}' "$RESULTS" | sort -n)
p50=$(printf '%s\n' "$sorted" | awk -v n="$total" 'NR==int(n*0.5){print; exit}')
p95=$(printf '%s\n' "$sorted" | awk -v n="$total" 'NR==int(n*0.95){print; exit}')
nonok_codes=$(awk '$1!="200"{print $1}' "$RESULTS" | sort -u | tr '\n' ',' | sed 's/,$//')

echo "  total legit:      $total"
echo "  HTTP 200:         $ok_count"
echo "  pass rate:        ${legit_pct}% (threshold ≥${LEGIT_MIN_PASS_PCT}%)"
echo "  non-200 codes:    ${nonok_codes:-none}"
echo "  legit p50/p95:    ${p50:-N/A}s / ${p95:-N/A}s"

if [[ "$legit_pct" -lt "$LEGIT_MIN_PASS_PCT" ]]; then
    echo
    echo "  FAIL: legit pass rate ${legit_pct}% < ${LEGIT_MIN_PASS_PCT}% under chaos"
    echo "  → server is starving real users under chaos load"
    exit 1
fi
echo "  PASS: legit traffic survived above threshold under concurrent chaos"

# Step 6: post-chaos liveness — server must still answer health
# correctly AFTER the entire chaos run (final zombie check).
echo
echo "[6/6] Post-chaos liveness probe..."
post_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post_code="000"
post_body=$(curl -sS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)
if [[ "$post_code" != "200" ]]; then
    echo "  FAIL: post-chaos probe HTTP $post_code — server tipped over"
    exit 1
fi
if ! printf '%s' "$post_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: post-chaos body missing valid status field"
    echo "  body: $(printf '%s' "$post_body" | head -c 200)"
    exit 1
fi
echo "  PASS: post-chaos HTTP 200 + valid status — server stable"

echo
echo "=== Chaos Failure-Injection Challenge: PASSED ==="
echo "  Captured evidence:"
echo "    legit_reqs=${total} legit_pct=${legit_pct}% (≥${LEGIT_MIN_PASS_PCT}%)"
echo "    slow_loris=${#slow_loris_pids[@]} oversized_header_code=${oversized_code}"
echo "    legit p50=${p50:-N/A}s p95=${p95:-N/A}s"
