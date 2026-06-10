#!/usr/bin/env bash
# scaling_hits_helixcode_gate.sh — anti-delegation gate CM-SCALING-HITS-HELIXCODE (SP7 A5).
#
# WHAT THIS GATE GUARDS
#   The scaling coverage row must be backed by a HelixCode-LOCAL harness that
#   exercised the real internal/worker.WorkerPool across a worker-count sweep and
#   captured a real throughput table — NOT a config-only / delegated-shell PASS
#   (CONST-050(B)). It asserts the scaling harness emitted a scaling_throughput.json
#   whose shape proves real scale-out:
#     - a `steps` array with multiple distinct n_workers values (a real sweep),
#     - `gain_at_max_n` >= `min_gain_threshold` (throughput actually scaled, not flat),
#     - at least one step with non-zero throughput_tps.
#
# GATE POLICY
#   PASS  at least one scaling_throughput.json satisfies all invariants above.
#   FAIL  a scaling_throughput.json exists but is flat/empty (single step, gain
#         below threshold, or zero throughput).
#   SOFT  no scaling_throughput.json found (harness not run this cycle). Run
#         `make test-scaling` / `make test-scaling-meta`.
#
#   Exit 0 when zero FAIL. Exit 1 on any FAIL. Exit 2 on usage error.
#
# SCOPE: scans qa-results/ under repo root by default; $1 overrides (meta-test
# passes a fixture dir).
#
# Honest shebang, `bash -n` clean (CONST-068). PARITY: linux-only — JSON text
# analysis only; no OS-specific primitives.

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SCAN_DIR="${1:-$ROOT/qa-results}"

echo "CM-SCALING-HITS-HELIXCODE (SP7 A5) — scanning for scaling evidence under: ${SCAN_DIR#"$ROOT"/}"
echo

if [[ ! -d "$SCAN_DIR" ]]; then
    echo "SOFT  no qa-results dir ($SCAN_DIR) — scaling harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

mapfile -t REPORTS < <(find "$SCAN_DIR" -type f -name 'scaling_throughput.json' 2>/dev/null | sort)

if [[ "${#REPORTS[@]}" -eq 0 ]]; then
    echo "SOFT  no scaling_throughput.json found — scaling harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

pass=0
fail=0
for rep in "${REPORTS[@]}"; do
    rel="${rep#"$ROOT"/}"
    body="$(cat "$rep")"

    # distinct n_workers values
    distinct_n="$(printf '%s' "$body" | grep -oE '"n_workers"[[:space:]]*:[[:space:]]*[0-9]+' | grep -oE '[0-9]+$' | sort -un | wc -l | tr -d ' ')"
    # any non-zero throughput
    nonzero_tps=0
    printf '%s' "$body" | grep -oE '"throughput_tps"[[:space:]]*:[[:space:]]*[0-9.]+' | grep -oE '[0-9.]+$' | grep -qE '[1-9]' && nonzero_tps=1
    gain="$(printf '%s' "$body" | grep -oE '"gain_at_max_n"[[:space:]]*:[[:space:]]*[0-9.]+' | head -1 | grep -oE '[0-9.]+$' || echo 0)"
    thr="$(printf '%s' "$body" | grep -oE '"min_gain_threshold"[[:space:]]*:[[:space:]]*[0-9.]+' | head -1 | grep -oE '[0-9.]+$' || echo 0)"

    problem=""
    [[ "${distinct_n:-0}" -lt 2 ]] && problem="only $distinct_n distinct n_workers value(s) (not a sweep)"
    [[ -z "$problem" && "$nonzero_tps" -eq 0 ]] && problem="no non-zero throughput_tps (no real work)"
    if [[ -z "$problem" ]]; then
        # gain >= threshold, using awk for float compare
        below="$(awk -v g="$gain" -v t="$thr" 'BEGIN{print (g+0 < t+0) ? 1 : 0}')"
        [[ "$below" -eq 1 ]] && problem="gain_at_max_n=$gain below min_gain_threshold=$thr (flat throughput)"
    fi

    if [[ -n "$problem" ]]; then
        echo "FAIL  $rel — $problem"
        fail=$((fail + 1))
    else
        echo "PASS  $rel — sweep over $distinct_n worker counts, gain=$gain >= threshold=$thr, real throughput"
        pass=$((pass + 1))
    fi
done

echo
echo "=== CM-SCALING-HITS-HELIXCODE summary === PASS=$pass FAIL=$fail"
if [[ "$fail" -gt 0 ]]; then
    echo "FAIL: $fail scaling_throughput.json file(s) are flat/empty, not real scale-out"
    exit 1
fi
echo "PASS: scaling evidence shows a real worker-count sweep with genuine scale-out"
exit 0
