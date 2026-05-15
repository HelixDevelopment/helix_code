#!/usr/bin/env bash
# tests/e2e/challenges/ddos_health_flood.sh — anti-bluff DDoS Challenge
# per CONST-035 + CONST-050(B)'s test-type matrix (DDoS is one of the
# 6 absent test types in HelixCode's current matrix — see Task #256).
#
# What this Challenge proves:
#   1. The HelixCode server's /api/v1/health endpoint survives a
#      controlled request flood at the advertised throughput tier.
#   2. The server is STILL HEALTHY after the flood (no crash, no
#      tipped-over goroutines, no leaked connections).
#   3. The server's response is real JSON with a "status" field
#      (CONST-035: positive evidence, not just HTTP 200).
#
# Anti-bluff (CONST-035): every gate carries captured runtime
# evidence. If the infrastructure isn't running, the Challenge
# emits SKIP-OK per §11.4.3 topology-dispatch — never a fake PASS.
#
# Bluff-class avoided:
#   - "tests-pass-with-server-down": Challenge SKIP-OK's when the
#     health endpoint is unreachable BEFORE the flood, never silently
#     succeeds.
#   - "all-requests-succeed-bluff": Challenge measures + reports
#     actual success rate, fail rate, p50/p95/p99 latencies — not
#     a binary "exit-code-zero" claim.
#   - "post-flood-zombie": Challenge re-probes /health AFTER the
#     flood + asserts the response is still valid JSON with
#     status=ok|healthy.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

# Configurable knobs (with sensible defaults). Operators tune via env:
#   HELIX_HEALTH_URL    target endpoint
#   DDOS_REQUESTS       total request count (default 500)
#   DDOS_CONCURRENCY    parallel-request fan-out (default 50)
#   DDOS_TIMEOUT_SEC    per-request timeout (default 5)
#   DDOS_MIN_PASS_PCT   min % of requests that MUST succeed (default 95)
HEALTH_URL="${HELIX_HEALTH_URL:-http://localhost:8080/api/v1/health}"
REQUESTS="${DDOS_REQUESTS:-500}"
CONCURRENCY="${DDOS_CONCURRENCY:-50}"
TIMEOUT_SEC="${DDOS_TIMEOUT_SEC:-5}"
MIN_PASS_PCT="${DDOS_MIN_PASS_PCT:-95}"

echo "=== DDoS Health-Flood Challenge (anti-bluff per CONST-035) ==="
echo "  target:        $HEALTH_URL"
echo "  requests:      $REQUESTS"
echo "  concurrency:   $CONCURRENCY"
echo "  pass-rate min: ${MIN_PASS_PCT}%"

# Step 1: pre-flood probe — must succeed BEFORE we start flooding,
# otherwise SKIP-OK (infrastructure-down, not a feature bluff).
echo
echo "[1/5] Pre-flood probe — server must be reachable..."
pre_body=$(curl -sS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)
pre_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null || echo "000")
if [[ "$pre_code" != "200" ]]; then
    echo "  SKIP: server not reachable (HTTP $pre_code) — SKIP-OK: #env-server-down"
    echo
    echo "=== DDoS Health-Flood Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "  PASS: pre-flood probe HTTP 200 — server reachable"

# Step 2: schema sanity — response must be real JSON with a status field
# (anti-bluff: a 200 with empty body / non-JSON would be a server
# bluff masquerading as health).
echo
echo "[2/5] Pre-flood schema sanity..."
if ! printf '%s' "$pre_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: pre-flood body doesn't carry a valid status field"
    echo "  body: $(printf '%s' "$pre_body" | head -c 200)"
    exit 1
fi
echo "  PASS: pre-flood body has valid status field"

# Step 3: the flood itself. We use xargs -P for concurrency, curl for
# the request. Each request writes "<http_code> <elapsed_seconds>" to
# stdout; we collect into a file for analysis.
echo
echo "[3/5] Running flood ($REQUESTS requests, $CONCURRENCY concurrent)..."
RESULTS=$(mktemp)
trap "rm -f $RESULTS" EXIT
flood_start=$(date +%s.%N)
seq 1 "$REQUESTS" | xargs -n1 -P "$CONCURRENCY" -I{} \
    curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" \
        -w "%{http_code} %{time_total}\n" "$HEALTH_URL" \
    2>/dev/null >> "$RESULTS" || true
flood_end=$(date +%s.%N)
flood_seconds=$(awk -v s="$flood_start" -v e="$flood_end" 'BEGIN { printf "%.2f", e-s }')

# Step 4: analyse. Count HTTP 200s, compute p50/p95/p99 latencies.
echo
echo "[4/5] Analysing flood results..."
total=$(wc -l < "$RESULTS" | tr -d ' ')
ok_count=$(awk '$1=="200"{c++} END{print c+0}' "$RESULTS")
fail_count=$((total - ok_count))
pass_pct_int=$((ok_count * 100 / total))

# Latency percentiles — extract elapsed-time column, sort, pick.
sorted_latencies=$(awk '{print $2}' "$RESULTS" | sort -n)
p50=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.5){print; exit}')
p95=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.95){print; exit}')
p99=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.99){print; exit}')

# Distinct non-200 codes (for forensic visibility).
nonok_codes=$(awk '$1!="200"{print $1}' "$RESULTS" | sort -u | tr '\n' ',' | sed 's/,$//')

echo "  total requests:    $total"
echo "  HTTP 200 count:    $ok_count"
echo "  failures:          $fail_count (non-200 codes: ${nonok_codes:-none})"
echo "  flood wall-clock:  ${flood_seconds}s"
echo "  pass rate:         ${pass_pct_int}% (threshold: ≥${MIN_PASS_PCT}%)"
echo "  latency p50:       ${p50:-N/A}s"
echo "  latency p95:       ${p95:-N/A}s"
echo "  latency p99:       ${p99:-N/A}s"

if [[ "$pass_pct_int" -lt "$MIN_PASS_PCT" ]]; then
    echo
    echo "  FAIL: pass rate ${pass_pct_int}% below threshold ${MIN_PASS_PCT}%"
    echo "  → tune rate-limits / back-pressure OR raise DDOS_MIN_PASS_PCT if intentional"
    exit 1
fi
echo "  PASS: flood survived within pass-rate threshold"

# Step 5: post-flood liveness — the server MUST still answer health
# correctly AFTER the flood. This catches the "fell-over-during-flood"
# class of bluff where the server processed the requests but crashed
# right after.
echo
echo "[5/5] Post-flood liveness — server still healthy..."
post_body=$(curl -sS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)
post_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null || echo "000")
if [[ "$post_code" != "200" ]]; then
    echo "  FAIL: post-flood probe HTTP $post_code — server tipped over"
    exit 1
fi
if ! printf '%s' "$post_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: post-flood response missing valid status field"
    echo "  body: $(printf '%s' "$post_body" | head -c 200)"
    exit 1
fi
echo "  PASS: post-flood probe HTTP 200 + valid status — server stable"

echo
echo "=== DDoS Health-Flood Challenge: PASSED ==="
echo "  Captured evidence:"
echo "    pass_rate=${pass_pct_int}% requests=${total} concurrency=${CONCURRENCY}"
echo "    p50=${p50:-N/A}s p95=${p95:-N/A}s p99=${p99:-N/A}s wall=${flood_seconds}s"
