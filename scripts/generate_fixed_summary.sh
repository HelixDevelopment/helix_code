#!/usr/bin/env bash
# generate_fixed_summary.sh — mechanically regenerate docs/Fixed_Summary.md
# from the docs/Fixed.md closure table per Constitution §11.4.19 + §11.4.57
# (Type-aware closure vocabulary) + §11.4.91.
#
# Fixed.md is a pipe table:  | Closure | Title | Type | Status | Round | Commit(s) | Evidence |
# This generator counts closures by Type and emits the aggregate-counts block.
#
# Idempotent. Usage: scripts/generate_fixed_summary.sh [--check]
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/docs/Fixed.md"
OUT="$ROOT/docs/Fixed_Summary.md"
[[ -f "$SRC" ]] || { echo "FATAL: $SRC missing" >&2; exit 2; }

# Count data rows by the Type column (3rd pipe field). Skip the header row
# ("| Closure |") and the separator ("|---|"). A data row's col-1 is a date.
read -r BUG FEAT TASK TOTAL <<<"$(
  awk -F'|' '
    /^\|/ {
      c1=$2; gsub(/^[ \t]+|[ \t]+$/, "", c1)
      if (c1 !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) next      # only YYYY-MM-DD data rows
      t=$4; gsub(/^[ \t]+|[ \t]+$/, "", t)
      if (t=="Bug") bug++; else if (t=="Feature") feat++; else if (t=="Task") task++
      total++
    }
    END { printf "%d %d %d %d\n", bug, feat, task, total }
  ' "$SRC"
)"

TODAY="$(date +%Y-%m-%d)"
NEW="$(cat <<EOF
# HelixCode — Fixed Items Summary

> Generated **mechanically** from \`docs/Fixed.md\` by \`scripts/generate_fixed_summary.sh\` per Constitution §11.4.19 + §11.4.57 (Type-aware closure vocabulary). Counts only — do not hand-edit, re-run the generator.

## Aggregate counts

| Type | Count | Closure vocabulary (CONST-057) |
|---|---|---|
| Bug | $BUG | \`Fixed (→ Fixed.md)\` |
| Feature | $FEAT | \`Implemented (→ Fixed.md)\` |
| Task | $TASK | \`Completed (→ Fixed.md)\` |

**Total closed items**: $TOTAL (counted directly from the \`docs/Fixed.md\` closure table).

*Last regenerated: $TODAY by \`scripts/generate_fixed_summary.sh\`. HTML/PDF exports via \`scripts/regenerate-tracker-exports.sh\`.*
EOF
)"

if [[ "${1:-}" == "--check" ]]; then
  if ! diff -u <(printf '%s\n' "$NEW") "$OUT" >/tmp/fixed_summary.diff 2>/dev/null; then
    echo "CM-FIXED-SUMMARY-SYNC: FAIL — docs/Fixed_Summary.md is stale; run scripts/generate_fixed_summary.sh" >&2
    cat /tmp/fixed_summary.diff >&2
    exit 1
  fi
  echo "CM-FIXED-SUMMARY-SYNC: PASS — summary in sync with Fixed.md"
  exit 0
fi

printf '%s\n' "$NEW" > "$OUT"
echo "wrote $OUT (Bug=$BUG Feature=$FEAT Task=$TASK total=$TOTAL)"
