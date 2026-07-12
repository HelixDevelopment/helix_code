#!/usr/bin/env bash
# token_accounting.sh — §11.4.141 Token-efficiency measurement harness
# Computes BEFORE/AFTER token reduction from Claude API usage objects.
set -euo pipefail

usage() {
    cat <<EOF
Usage: $(basename "$0") --before <before.json> --after <after.json> [--output <report.md>]

Measures token reduction per §11.4.141. Each JSON file must contain a Claude API
usage object with: input_tokens, cache_read_input_tokens, cache_creation_input_tokens, output_tokens.

Options:
  --before FILE   JSON with BEFORE usage (required)
  --after FILE    JSON with AFTER usage (required)
  --output FILE   Write markdown report to FILE (default: stdout)
  --help          Show this help
EOF
}

BEFORE="" AFTER="" OUTPUT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --before) BEFORE="$2"; shift 2 ;;
        --after)  AFTER="$2"; shift 2 ;;
        --output) OUTPUT="$2"; shift 2 ;;
        --help)   usage; exit 0 ;;
        *)        echo "Unknown option: $1"; usage; exit 1 ;;
    esac
done

if [[ -z "$BEFORE" || -z "$AFTER" ]]; then
    echo "ERROR: --before and --after are required" >&2
    usage >&2
    exit 1
fi

# Extract values using python3 (available on all target hosts)
extract() {
    python3 -c "import json,sys; d=json.load(open('$1')); print(d.get('$2', 0))"
}

B_INPUT=$(extract "$BEFORE" input_tokens)
B_CACHE_READ=$(extract "$BEFORE" cache_read_input_tokens)
B_CACHE_CREATE=$(extract "$BEFORE" cache_creation_input_tokens)
B_OUTPUT=$(extract "$BEFORE" output_tokens)
B_TOTAL=$((B_INPUT + B_CACHE_READ + B_CACHE_CREATE))

A_INPUT=$(extract "$AFTER" input_tokens)
A_CACHE_READ=$(extract "$AFTER" cache_read_input_tokens)
A_CACHE_CREATE=$(extract "$AFTER" cache_creation_input_tokens)
A_OUTPUT=$(extract "$AFTER" output_tokens)
A_TOTAL=$((A_INPUT + A_CACHE_READ + A_CACHE_CREATE))

# Compute reductions
if [[ $B_TOTAL -gt 0 ]]; then
    TOTAL_PCT=$(python3 -c "print(f'{(1 - $A_TOTAL/$B_TOTAL) * 100:.1f}')")
else
    TOTAL_PCT="N/A"
fi

if [[ $B_INPUT -gt 0 ]]; then
    INPUT_PCT=$(python3 -c "print(f'{(1 - $A_INPUT/$B_INPUT) * 100:.1f}')")
else
    INPUT_PCT="N/A"
fi

# Warm cache check
WARM="NO"
if [[ $A_CACHE_READ -gt 0 ]]; then
    WARM="YES"
fi

# Target check (≤40% of BEFORE = ≥60% reduction)
TARGET_MET="NO"
if python3 -c "exit(0 if $A_TOTAL <= 0.4 * $B_TOTAL else 1)" 2>/dev/null; then
    TARGET_MET="YES"
fi

# Generate report
REPORT=$(cat <<EOF
# §11.4.141 Token-Efficiency Report

| Metric | BEFORE | AFTER | Reduction |
|--------|--------|-------|-----------|
| Input tokens | $B_INPUT | $A_INPUT | ${INPUT_PCT}% |
| Cache read | $B_CACHE_READ | $A_CACHE_READ | — |
| Cache creation | $B_CACHE_CREATE | $A_CACHE_CREATE | — |
| Output tokens | $B_OUTPUT | $A_OUTPUT | — |
| **Total (input+cache)** | **$B_TOTAL** | **$A_TOTAL** | **${TOTAL_PCT}%** |

## Checks
- Warm cache (cache_read > 0): **$WARM**
- Target met (AFTER ≤ 40% of BEFORE): **$TARGET_MET**
- §11.4.141 compliant: **$([ "$WARM" = "YES" ] && [ "$TARGET_MET" = "YES" ] && echo "YES" || echo "NO")**
EOF
)

if [[ -n "$OUTPUT" ]]; then
    echo "$REPORT" > "$OUTPUT"
    echo "Report written to: $OUTPUT"
else
    echo "$REPORT"
fi
