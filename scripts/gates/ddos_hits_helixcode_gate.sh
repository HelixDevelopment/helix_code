#!/usr/bin/env bash
# ddos_hits_helixcode_gate.sh — anti-delegation gate CM-DDOS-HITS-HELIXCODE (SP7 A5).
#
# WHAT THIS GATE GUARDS
#   The ddos coverage row must be backed by a HelixCode-LOCAL harness that actually
#   FLOODED a live HelixCode endpoint and captured real per-status-code + latency
#   evidence — NOT a config-only / delegated-shell PASS (CONST-050(B)). It asserts
#   the ddos harness emitted a flood_report.json whose shape proves a real flood:
#     - an `endpoint` field referencing a real URL (http... host:port),
#     - `requests_sent` > 0,
#     - a `body_marker_hits` > 0 (the server actually served real responses),
#     - `status_5xx` == 0 (graceful degradation — no server-error storm),
#     - a numeric `p99_under_flood_ms` (latency was actually measured).
#
# GATE POLICY
#   PASS  at least one flood_report.json under the evidence root satisfies all
#         invariants above.
#   FAIL  a flood_report.json exists but is config-only / placeholder (no real
#         endpoint, zero requests, zero served responses, or a 5xx storm).
#   SOFT  no flood_report.json found at all (the harness has not been run this
#         cycle) — reported, non-fatal, so a fresh checkout is not blocked before
#         the harness runs. Run `make test-ddos-meta` (fast) or `make test-ddos`
#         (real infra) to produce evidence.
#
#   Exit 0 when there are zero FAIL classifications. Exit 1 on any FAIL. Exit 2 on
#   usage error.
#
# SCOPE
#   Scans qa-results/ under the repo root by default; pass a dir as $1 to override
#   (the paired meta-test passes a fixture dir so it never touches real evidence).
#
# Honest shebang, `bash -n` clean (CONST-068). PARITY: linux-only — pure shell text
# analysis of JSON evidence; no OS-specific primitives.

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SCAN_DIR="${1:-$ROOT/qa-results}"

echo "CM-DDOS-HITS-HELIXCODE (SP7 A5) — scanning for ddos flood evidence under: ${SCAN_DIR#"$ROOT"/}"
echo

if [[ ! -d "$SCAN_DIR" ]]; then
    echo "SOFT  no qa-results dir ($SCAN_DIR) — ddos harness not run this cycle"
    echo "      run 'make test-ddos-meta' (fast) or 'make test-ddos' (real infra) to produce evidence"
    echo
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

mapfile -t REPORTS < <(find "$SCAN_DIR" -type f -name 'flood_report.json' 2>/dev/null | sort)

if [[ "${#REPORTS[@]}" -eq 0 ]]; then
    echo "SOFT  no flood_report.json found — ddos harness not run this cycle"
    echo
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

pass=0
fail=0
for rep in "${REPORTS[@]}"; do
    rel="${rep#"$ROOT"/}"
    body="$(cat "$rep")"

    endpoint="$(printf '%s' "$body" | grep -oE '"endpoint"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed -E 's/.*"endpoint"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/')"
    sent="$(printf '%s' "$body" | grep -oE '"requests_sent"[[:space:]]*:[[:space:]]*[0-9]+' | head -1 | grep -oE '[0-9]+$' || echo 0)"
    marker="$(printf '%s' "$body" | grep -oE '"body_marker_hits"[[:space:]]*:[[:space:]]*[0-9]+' | head -1 | grep -oE '[0-9]+$' || echo 0)"
    s5xx="$(printf '%s' "$body" | grep -oE '"status_5xx"[[:space:]]*:[[:space:]]*[0-9]+' | head -1 | grep -oE '[0-9]+$' || echo 0)"
    has_p99=0
    printf '%s' "$body" | grep -qE '"p99_under_flood_ms"[[:space:]]*:[[:space:]]*[0-9]' && has_p99=1

    problem=""
    case "$endpoint" in
        http*://*) : ;;  # real URL
        *) problem="endpoint not a real URL ('$endpoint')" ;;
    esac
    [[ -z "$problem" && "$sent" -le 0 ]] && problem="requests_sent=$sent (no real flood)"
    [[ -z "$problem" && "$marker" -le 0 ]] && problem="body_marker_hits=$marker (no real served responses)"
    [[ -z "$problem" && "$s5xx" -gt 0 ]] && problem="status_5xx=$s5xx (server-error storm)"
    [[ -z "$problem" && "$has_p99" -eq 0 ]] && problem="no numeric p99_under_flood_ms (latency not measured)"

    if [[ -n "$problem" ]]; then
        echo "FAIL  $rel — $problem"
        fail=$((fail + 1))
    else
        echo "PASS  $rel — flooded $endpoint sent=$sent served=$marker 5xx=$s5xx p99 measured"
        pass=$((pass + 1))
    fi
done

echo
echo "=== CM-DDOS-HITS-HELIXCODE summary === PASS=$pass FAIL=$fail"
if [[ "$fail" -gt 0 ]]; then
    echo "FAIL: $fail flood_report.json file(s) are config-only / placeholder, not a real HelixCode flood"
    exit 1
fi
echo "PASS: ddos evidence references a live HelixCode endpoint with real flood + latency"
exit 0
