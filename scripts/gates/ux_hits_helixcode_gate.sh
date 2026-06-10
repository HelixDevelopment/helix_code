#!/usr/bin/env bash
# ux_hits_helixcode_gate.sh — anti-delegation gate CM-UX-HITS-HELIXCODE (SP7 A5).
#
# WHAT THIS GATE GUARDS
#   The ux coverage row must be backed by a HelixCode-LOCAL journey that drove the
#   real CLI/server with a captured BIDIRECTIONAL transcript (§11.4.83) — NOT a
#   config-only / delegated-shell PASS (CONST-050(B)). It asserts the ux harness
#   emitted a journey_transcript.jsonl whose shape proves a real journey:
#     - at least one JSON line carrying BOTH a non-empty `command_sent` AND a
#       non-empty `response_received` (both directions present — one-sided is not a
#       transcript),
#     - at least one line with `"verdict":"PASS"` (a real-output assertion passed),
#     - NO line whose response is a canned-bluff constant.
#
# GATE POLICY
#   PASS  at least one journey_transcript.jsonl satisfies all invariants above.
#   FAIL  a journey_transcript.jsonl exists but is one-sided (missing a direction)
#         or carries a canned-bluff response.
#   SOFT  no journey_transcript.jsonl found (harness not run this cycle). Run
#         `make test-ux` / `make test-ux-meta`.
#
#   Exit 0 when zero FAIL. Exit 1 on any FAIL. Exit 2 on usage error.
#
# SCOPE: scans qa-results/ under repo root by default; $1 overrides (meta-test
# passes a fixture dir).
#
# Honest shebang, `bash -n` clean (CONST-068). PARITY: linux-only — JSONL text
# analysis only; no OS-specific primitives.

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SCAN_DIR="${1:-$ROOT/qa-results}"

echo "CM-UX-HITS-HELIXCODE (SP7 A5) — scanning for ux journey transcripts under: ${SCAN_DIR#"$ROOT"/}"
echo

if [[ ! -d "$SCAN_DIR" ]]; then
    echo "SOFT  no qa-results dir ($SCAN_DIR) — ux harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

mapfile -t REPORTS < <(find "$SCAN_DIR" -type f -name 'journey_transcript.jsonl' 2>/dev/null | sort)

if [[ "${#REPORTS[@]}" -eq 0 ]]; then
    echo "SOFT  no journey_transcript.jsonl found — ux harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

# Canned-bluff response markers the gate refuses (the BLUFF-001 family).
CANNED_RE='This is a simulated response|simulated response|canned constant|For now, simulate'

pass=0
fail=0
for rep in "${REPORTS[@]}"; do
    rel="${rep#"$ROOT"/}"

    bidir=0
    haspass=0
    canned=0
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        has_cmd=0; has_resp=0
        printf '%s' "$line" | grep -qE '"command_sent"[[:space:]]*:[[:space:]]*"[^"]+"' && has_cmd=1
        printf '%s' "$line" | grep -qE '"response_received"[[:space:]]*:[[:space:]]*"[^"]+"' && has_resp=1
        [[ "$has_cmd" -eq 1 && "$has_resp" -eq 1 ]] && bidir=1
        printf '%s' "$line" | grep -qE '"verdict"[[:space:]]*:[[:space:]]*"PASS"' && haspass=1
        printf '%s' "$line" | grep -qiE "$CANNED_RE" && canned=1
    done < "$rep"

    problem=""
    [[ "$bidir" -eq 0 ]] && problem="no bidirectional line (command_sent + response_received both present)"
    [[ -z "$problem" && "$haspass" -eq 0 ]] && problem="no PASS verdict (no real-output assertion passed)"
    [[ -z "$problem" && "$canned" -eq 1 ]] && problem="canned-bluff response present (simulated/canned constant)"

    if [[ -n "$problem" ]]; then
        echo "FAIL  $rel — $problem"
        fail=$((fail + 1))
    else
        echo "PASS  $rel — bidirectional transcript with PASS verdict, no canned responses"
        pass=$((pass + 1))
    fi
done

echo
echo "=== CM-UX-HITS-HELIXCODE summary === PASS=$pass FAIL=$fail"
if [[ "$fail" -gt 0 ]]; then
    echo "FAIL: $fail journey_transcript.jsonl file(s) are one-sided / canned, not a real CLI journey"
    exit 1
fi
echo "PASS: ux evidence is a real bidirectional CLI/server journey transcript"
exit 0
