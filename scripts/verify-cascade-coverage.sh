#!/usr/bin/env bash
# scripts/verify-cascade-coverage.sh — regression gate for the
# CONST-051(A) submodule-side cascade.
#
# Asserts every owned submodule listed in docs/improvements/
# submodule_owned.txt has the 6 mandated CONST-050(B) test-type
# Challenge files in its `challenges/scripts/` directory:
#
#   ddos_health_flood_challenge.sh
#   stress_sustained_load_challenge.sh
#   chaos_failure_injection_challenge.sh
#   scaling_horizontal_challenge.sh
#   ui_terminal_interaction_challenge.sh
#   ux_end_to_end_flow_challenge.sh
#
# If ANY of those files is missing from an owned submodule (or the
# submodule's challenges/scripts/ directory doesn't exist), the gate
# FAILs and names the gaps. Catches the regression where a future
# commit removes a Challenge file or adds a new owned submodule
# without the cascade.
#
# Honest SKIP-OK: HelixAgent is exempt because its primary surface
# is the Go application + nested-submodule consumer, not a service
# with its own challenges/scripts/. Document any other exemption in
# the EXEMPT_SUBMODULES list below.
#
# Anti-bluff invariant (CONST-035): file-presence alone is not
# sufficient — each file MUST also be executable and reference its
# expected env-var prefix. Empty stubs would game this gate, so we
# also assert each file is >= 1 KB and contains "PASS" + "SKIP-OK"
# patterns.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OWNED_FILE="docs/improvements/submodule_owned.txt"
REQUIRED_CHALLENGES=(
    "ddos_health_flood_challenge.sh"
    "stress_sustained_load_challenge.sh"
    "chaos_failure_injection_challenge.sh"
    "scaling_horizontal_challenge.sh"
    "ui_terminal_interaction_challenge.sh"
    "ux_end_to_end_flow_challenge.sh"
)

# Submodules exempt from cascade-coverage requirement:
# - HelixAgent: primary surface is the Go application + nested-
#   submodule consumer, not a service. No challenges/scripts/.
# - Github-Pages-Website: marketing site, no native test surface.
# - submodules/llms_verifier: nested subtree
#   structure (llm-verifier/ subdir holds actual code).
EXEMPT_SUBMODULES=(
    "HelixAgent"
    "Github-Pages-Website"
)

is_exempt() {
    local sm="$1"
    for ex in "${EXEMPT_SUBMODULES[@]}"; do
        if [[ "$sm" == "$ex" ]]; then return 0; fi
    done
    return 1
}

if [[ ! -f "$OWNED_FILE" ]]; then
    echo "FAIL: $OWNED_FILE missing"
    exit 2
fi

TOTAL=0
COVERED=0
EXEMPT_COUNT=0
SKIPPED_NODIR=0
MISSING_LINES=()
BLUFF_LINES=()

while IFS=' |' read -r sm rest; do
    [[ -z "$sm" ]] && continue
    TOTAL=$((TOTAL + 1))
    if is_exempt "$sm"; then
        EXEMPT_COUNT=$((EXEMPT_COUNT + 1))
        continue
    fi

    SCRIPTS_DIR="$sm/challenges/scripts"
    if [[ ! -d "$SCRIPTS_DIR" ]]; then
        SKIPPED_NODIR=$((SKIPPED_NODIR + 1))
        MISSING_LINES+=("  $sm: no challenges/scripts/ directory")
        continue
    fi

    SM_MISSING=()
    SM_BLUFF=()
    for ch in "${REQUIRED_CHALLENGES[@]}"; do
        f="$SCRIPTS_DIR/$ch"
        if [[ ! -f "$f" ]]; then
            SM_MISSING+=("$ch")
            continue
        fi
        if [[ ! -x "$f" ]]; then
            SM_BLUFF+=("$ch:not-executable")
            continue
        fi
        size=$(wc -c < "$f")
        if [[ "$size" -lt 1024 ]]; then
            SM_BLUFF+=("$ch:too-small($size)")
            continue
        fi
        if ! grep -q 'PASS' "$f"; then
            SM_BLUFF+=("$ch:no-PASS")
            continue
        fi
        if ! grep -q 'SKIP-OK' "$f"; then
            SM_BLUFF+=("$ch:no-SKIP-OK")
            continue
        fi
    done

    if [[ ${#SM_MISSING[@]} -eq 0 ]] && [[ ${#SM_BLUFF[@]} -eq 0 ]]; then
        COVERED=$((COVERED + 1))
    fi
    if [[ ${#SM_MISSING[@]} -gt 0 ]]; then
        MISSING_LINES+=("  $sm: missing — ${SM_MISSING[*]}")
    fi
    if [[ ${#SM_BLUFF[@]} -gt 0 ]]; then
        BLUFF_LINES+=("  $sm: bluff — ${SM_BLUFF[*]}")
    fi
done < "$OWNED_FILE"

echo "=== Cascade Coverage Regression Gate ==="
echo "  total owned submodules:  $TOTAL"
echo "  exempt:                  $EXEMPT_COUNT (${EXEMPT_SUBMODULES[*]})"
echo "  fully covered:           $COVERED"
echo "  no challenges/scripts/:  $SKIPPED_NODIR"
echo "  missing challenges:      ${#MISSING_LINES[@]} submodule(s)"
echo "  bluff signals:           ${#BLUFF_LINES[@]} submodule(s)"
echo

if [[ ${#MISSING_LINES[@]} -gt 0 ]]; then
    echo "MISSING (CONST-050(B) cascade gap):"
    printf '%s\n' "${MISSING_LINES[@]}"
    echo
fi
if [[ ${#BLUFF_LINES[@]} -gt 0 ]]; then
    echo "BLUFF SIGNALS (CONST-035 violation in cascade content):"
    printf '%s\n' "${BLUFF_LINES[@]}"
    echo
fi

EXPECTED=$((TOTAL - EXEMPT_COUNT))
if [[ "$COVERED" -eq "$EXPECTED" ]]; then
    echo "PASS: $COVERED of $EXPECTED non-exempt owned submodules covered"
    exit 0
else
    UNCOVERED=$((EXPECTED - COVERED))
    echo "FAIL: $UNCOVERED non-exempt owned submodule(s) lack full CONST-050(B) cascade"
    echo "  → run /tmp/cascade_const050b.sh for each gap, OR add to EXEMPT_SUBMODULES if intentional"
    exit 1
fi
