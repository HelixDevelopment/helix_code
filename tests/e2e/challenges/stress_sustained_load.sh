#!/usr/bin/env bash
# tests/e2e/challenges/stress_sustained_load.sh — anti-bluff Stress
# Challenge per CONST-035 + CONST-050(B). 2nd of the 6 missing test
# types from Task #256.
#
# Difference from ddos_health_flood.sh (sibling Challenge): DDoS is a
# burst at advertised throughput; STRESS is *sustained* load over a
# longer wall-clock window with a *higher* per-second rate target.
# DDoS asks "does the server survive a single flood?"; stress asks
# "does the server stay sane under continuous pressure for N seconds
# without leaking memory, accumulating goroutines, or dropping
# latency below SLO?".
#
# What this Challenge proves:
#   1. The server's /api/v1/health endpoint survives sustained
#      pressure for `STRESS_DURATION_SEC` seconds at
#      `STRESS_REQUESTS_PER_SEC` target rate (best-effort).
#   2. Per-second pass rate stays above threshold throughout (not
#      just average — degradation patterns are forensic).
#   3. Post-stress latency hasn't degraded by more than
#      `STRESS_MAX_LATENCY_DEGRADATION_PCT` vs the pre-stress
#      baseline (catches the "still-200-but-50x-slower" zombie
#      class that the binary pass-rate gate would miss).
#
# Anti-bluff guarantees:
#   - Per-second sampling (not just aggregate) — captures degradation
#     curves, not just end-state.
#   - Pre-stress + post-stress latency baselines compared explicitly
#     (catches degradation that pass-rate gate would mask).
#   - Honest SKIP-OK per §11.4.3 when server unreachable BEFORE
#     stress — never silently fake-PASS.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

# Configurable knobs:
HEALTH_URL="${HELIX_HEALTH_URL:-http://localhost:8080/api/v1/health}"
DURATION_SEC="${STRESS_DURATION_SEC:-30}"
REQS_PER_SEC="${STRESS_REQUESTS_PER_SEC:-100}"
CONCURRENCY="${STRESS_CONCURRENCY:-40}"
TIMEOUT_SEC="${STRESS_TIMEOUT_SEC:-5}"
MIN_PASS_PCT="${STRESS_MIN_PASS_PCT:-95}"
MAX_LATENCY_DEGRADATION_PCT="${STRESS_MAX_LATENCY_DEGRADATION_PCT:-300}"

echo "=== Sustained-Load Stress Challenge (anti-bluff per CONST-035) ==="
echo "  target:                    $HEALTH_URL"
echo "  duration:                  ${DURATION_SEC}s"
echo "  target rps:                $REQS_PER_SEC"
echo "  concurrency:               $CONCURRENCY"
echo "  pass-rate threshold:       ≥${MIN_PASS_PCT}%"
echo "  latency-degradation max:   ≤${MAX_LATENCY_DEGRADATION_PCT}%"

# Step 1: pre-stress probe + baseline latency. The baseline is the
# median of 10 quick probes — captured before we apply pressure so
# we can compare post-stress against it.
echo
echo "[1/6] Pre-stress probe + baseline latency..."
pre_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null || echo "000")
if [[ "$pre_code" != "200" ]]; then
    echo "  SKIP: server not reachable (HTTP $pre_code) — SKIP-OK: #env-server-down"
    echo
    echo "=== Sustained-Load Stress Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
BASELINE_LATENCIES=$(for i in $(seq 1 10); do
    curl -sS --max-time 5 -o /dev/null -w "%{time_total}\n" "$HEALTH_URL" 2>/dev/null
done | sort -n)
baseline_median=$(printf '%s\n' "$BASELINE_LATENCIES" | awk 'NR==5{print}')
echo "  PASS: pre-stress baseline median latency = ${baseline_median}s"

# Step 2: schema sanity (anti-bluff — catches 200-with-empty-body bluff)
echo
echo "[2/6] Pre-stress schema sanity..."
pre_body=$(curl -sS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)
if ! printf '%s' "$pre_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: pre-stress body doesn't carry a valid status field"
    echo "  body: $(printf '%s' "$pre_body" | head -c 200)"
    exit 1
fi
echo "  PASS: pre-stress body has valid status field"

# Step 3: sustained-load loop. Each second, fan out REQS_PER_SEC
# requests via xargs -P, append results to a per-second log line.
# This gives us a per-second pass-rate curve — degradation patterns
# are visible even when aggregate looks fine.
echo
echo "[3/6] Sustained load (${DURATION_SEC}s @ ${REQS_PER_SEC} rps)..."
PER_SEC_LOG=$(mktemp)
RESULTS=$(mktemp)
trap "rm -f $PER_SEC_LOG $RESULTS" EXIT

stress_start=$(date +%s.%N)
for second in $(seq 1 "$DURATION_SEC"); do
    sec_start=$(date +%s.%N)
    seq 1 "$REQS_PER_SEC" | xargs -n1 -P "$CONCURRENCY" -I{} \
        curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" \
            -w "%{http_code} %{time_total}\n" "$HEALTH_URL" \
        2>/dev/null >> "$RESULTS" || true
    sec_end=$(date +%s.%N)
    # Trim the most recent REQS_PER_SEC lines for this second's snapshot.
    sec_total=$REQS_PER_SEC
    sec_ok=$(tail -n "$REQS_PER_SEC" "$RESULTS" | awk '$1=="200"{c++} END{print c+0}')
    sec_pct=$((sec_ok * 100 / sec_total))
    sec_wall=$(awk -v s="$sec_start" -v e="$sec_end" 'BEGIN { printf "%.2f", e-s }')
    echo "  t=${second}s ok=${sec_ok}/${sec_total} (${sec_pct}%) wall=${sec_wall}s"
    echo "$second $sec_ok $sec_total $sec_pct $sec_wall" >> "$PER_SEC_LOG"
done
stress_end=$(date +%s.%N)
stress_seconds=$(awk -v s="$stress_start" -v e="$stress_end" 'BEGIN { printf "%.2f", e-s }')

# Step 4: aggregate analysis + per-second min-pass-rate check.
echo
echo "[4/6] Aggregate + per-second analysis..."
total=$(wc -l < "$RESULTS" | tr -d ' ')
ok_count=$(awk '$1=="200"{c++} END{print c+0}' "$RESULTS")
fail_count=$((total - ok_count))
overall_pct=$((ok_count * 100 / total))
worst_sec_pct=$(awk 'BEGIN{m=100} {if($4<m){m=$4}} END{print m}' "$PER_SEC_LOG")
worst_sec_t=$(awk -v target="$worst_sec_pct" '$4==target{print $1; exit}' "$PER_SEC_LOG")
nonok_codes=$(awk '$1!="200"{print $1}' "$RESULTS" | sort -u | tr '\n' ',' | sed 's/,$//')

sorted_latencies=$(awk '{print $2}' "$RESULTS" | sort -n)
p50=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.5){print; exit}')
p95=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.95){print; exit}')
p99=$(printf '%s\n' "$sorted_latencies" | awk -v n="$total" 'NR==int(n*0.99){print; exit}')

echo "  total requests:        $total"
echo "  HTTP 200 count:        $ok_count"
echo "  failures:              $fail_count (non-200 codes: ${nonok_codes:-none})"
echo "  overall pass rate:     ${overall_pct}%"
echo "  worst-second pass:     ${worst_sec_pct}% (at t=${worst_sec_t}s)"
echo "  stress wall-clock:     ${stress_seconds}s"
echo "  latency p50/p95/p99:   ${p50:-N/A}s / ${p95:-N/A}s / ${p99:-N/A}s"

if [[ "$overall_pct" -lt "$MIN_PASS_PCT" ]]; then
    echo
    echo "  FAIL: overall pass rate ${overall_pct}% < threshold ${MIN_PASS_PCT}%"
    exit 1
fi
if [[ "$worst_sec_pct" -lt "$MIN_PASS_PCT" ]]; then
    echo
    echo "  FAIL: worst-second pass rate ${worst_sec_pct}% < threshold ${MIN_PASS_PCT}%"
    echo "  → server degraded mid-stress at t=${worst_sec_t}s; investigate"
    exit 1
fi
echo "  PASS: aggregate + per-second pass-rate both ≥${MIN_PASS_PCT}%"

# Step 5: post-stress latency check. Compare post-stress median
# against pre-stress baseline; allow up to MAX_LATENCY_DEGRADATION_PCT
# degradation. This catches the "still-200-but-zombie-slow" class.
echo
echo "[5/6] Post-stress latency baseline check..."
POST_LATENCIES=$(for i in $(seq 1 10); do
    curl -sS --max-time 5 -o /dev/null -w "%{time_total}\n" "$HEALTH_URL" 2>/dev/null
done | sort -n)
post_median=$(printf '%s\n' "$POST_LATENCIES" | awk 'NR==5{print}')

# Compute degradation percentage. Bash doesn't do floating-point, so
# we use awk and round to int.
degradation_pct=$(awk -v b="$baseline_median" -v p="$post_median" \
    'BEGIN { if (b > 0) printf "%d", (p - b) * 100 / b; else print 0 }')
echo "  baseline median:    ${baseline_median}s"
echo "  post-stress median: ${post_median}s"
echo "  degradation:        ${degradation_pct}% (threshold: ≤${MAX_LATENCY_DEGRADATION_PCT}%)"
if [[ "$degradation_pct" -gt "$MAX_LATENCY_DEGRADATION_PCT" ]]; then
    echo
    echo "  FAIL: latency degraded ${degradation_pct}% > ${MAX_LATENCY_DEGRADATION_PCT}%"
    echo "  → server is zombie-slow after stress; investigate goroutine / connection leaks"
    exit 1
fi
echo "  PASS: latency degradation within threshold"

# Step 6: post-stress liveness + schema (catches fell-over-after-stress)
echo
echo "[6/6] Post-stress liveness + schema..."
post_code=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null || echo "000")
if [[ "$post_code" != "200" ]]; then
    echo "  FAIL: post-stress probe HTTP $post_code — server tipped over"
    exit 1
fi
post_body=$(curl -sS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)
if ! printf '%s' "$post_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: post-stress body missing valid status field"
    exit 1
fi
echo "  PASS: post-stress HTTP 200 + valid status — server stable"

echo
echo "=== Sustained-Load Stress Challenge: PASSED ==="
echo "  Captured evidence:"
echo "    duration=${stress_seconds}s requests=${total} target_rps=${REQS_PER_SEC} concurrency=${CONCURRENCY}"
echo "    overall_pass=${overall_pct}% worst_sec_pass=${worst_sec_pct}% (t=${worst_sec_t}s)"
echo "    p50=${p50:-N/A}s p95=${p95:-N/A}s p99=${p99:-N/A}s"
echo "    baseline_med=${baseline_median}s post_med=${post_median}s degradation=${degradation_pct}%"
