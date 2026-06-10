#!/usr/bin/env bash
# ui_hits_helixcode_gate.sh — anti-delegation gate CM-UI-HITS-HELIXCODE (SP7 A5).
#
# WHAT THIS GATE GUARDS
#   The ui coverage row must be backed by a HelixCode-LOCAL TUI render that asserted
#   on REAL composited cells (tcell.SimulationScreen) — NOT a config-only /
#   delegated-shell PASS (CONST-050(B)). It asserts the ui harness emitted a
#   rendered_cells.json whose shape proves a real render:
#     - non-zero `width` AND `height` (the screen was actually composited),
#     - an `asserted_strings` array with at least one entry,
#     - EVERY asserted_strings entry has `"found":true` (the expected text really
#       appeared in the rendered cells — an empty cell grid is not a PASS).
#
# GATE POLICY
#   PASS  at least one rendered_cells.json satisfies all invariants above.
#   FAIL  a rendered_cells.json exists but is empty-grid / has a `"found":false`
#         entry (expected text absent from the real render).
#   SOFT  no rendered_cells.json found (harness not run this cycle). Run
#         `make test-ui` / `make test-ui-meta`.
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

echo "CM-UI-HITS-HELIXCODE (SP7 A5) — scanning for ui rendered-cell evidence under: ${SCAN_DIR#"$ROOT"/}"
echo

if [[ ! -d "$SCAN_DIR" ]]; then
    echo "SOFT  no qa-results dir ($SCAN_DIR) — ui harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

mapfile -t REPORTS < <(find "$SCAN_DIR" -type f -name 'rendered_cells.json' 2>/dev/null | sort)

if [[ "${#REPORTS[@]}" -eq 0 ]]; then
    echo "SOFT  no rendered_cells.json found — ui harness not run this cycle"
    echo "=== summary === PASS=0 FAIL=0 SOFT=1"
    exit 0
fi

pass=0
fail=0
for rep in "${REPORTS[@]}"; do
    rel="${rep#"$ROOT"/}"
    body="$(cat "$rep")"

    width="$(printf '%s' "$body" | grep -oE '"width"[[:space:]]*:[[:space:]]*[0-9]+' | head -1 | grep -oE '[0-9]+$' 2>/dev/null || echo 0)"
    height="$(printf '%s' "$body" | grep -oE '"height"[[:space:]]*:[[:space:]]*[0-9]+' | head -1 | grep -oE '[0-9]+$' 2>/dev/null || echo 0)"
    n_asserted="$( { printf '%s' "$body" | grep -coE '"text"[[:space:]]*:'; } 2>/dev/null || echo 0)"
    n_found_false="$( { printf '%s' "$body" | grep -coE '"found"[[:space:]]*:[[:space:]]*false'; } 2>/dev/null || echo 0)"

    problem=""
    { [[ "${width:-0}" -le 0 ]] || [[ "${height:-0}" -le 0 ]]; } && problem="empty grid (width=$width height=$height)"
    [[ -z "$problem" && "${n_asserted:-0}" -lt 1 ]] && problem="no asserted_strings entries"
    [[ -z "$problem" && "${n_found_false:-0}" -gt 0 ]] && problem="$n_found_false expected string(s) NOT found in render"

    if [[ -n "$problem" ]]; then
        echo "FAIL  $rel — $problem"
        fail=$((fail + 1))
    else
        echo "PASS  $rel — ${width}x${height} grid, $n_asserted asserted strings all found"
        pass=$((pass + 1))
    fi
done

echo
echo "=== CM-UI-HITS-HELIXCODE summary === PASS=$pass FAIL=$fail"
if [[ "$fail" -gt 0 ]]; then
    echo "FAIL: $fail rendered_cells.json file(s) are empty-grid / missing expected strings, not a real render"
    exit 1
fi
echo "PASS: ui evidence shows real composited cells with all expected strings present"
exit 0
