#!/usr/bin/env bash
# summary_clarity_gate.sh — Constitution §11.4.91 gate.
#
# Scans the summary docs (docs/Issues_Summary.md + docs/Fixed_Summary.md) and
# FAILs on anti-pattern one-liner descriptions. §11.4.91 requires every
# summary entry's one-liner description to be self-contained and meaningful:
# >= 6 words OR >= 40 chars, naming SUBJECT + PROBLEM/GOAL.
#
# Forbidden anti-patterns (case-insensitive, whole-cell match):
#   - section labels: "Composes with", "Closure criteria", "Fix direction",
#     "Cascade requirement", "Canonical authority", "Scope", "Notes"
#   - bare metadata fragments: "Critical", "High", "Medium", "Low", "Bug",
#     "Feature", "Task", "In progress", "Queued", "Open", "Done", "Blocked"
#   - a bare §-letter / section marker alone (e.g. "§11.4.90", "(A)")
#
# Which cell is the "description one-liner": the LAST column of each data row
# (the Notes column for Issues_Summary, the closure note for Fixed_Summary).
#
# Exit: 0 if all rows compliant, 1 on any violation, 2 on usage error.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCS="${1:-$ROOT/docs}"

FILES=("$DOCS/Issues_Summary.md" "$DOCS/Fixed_Summary.md")

violations=0
checked=0

for f in "${FILES[@]}"; do
    if [[ ! -f "$f" ]]; then
        echo "SKIP $f — not present"
        continue
    fi

    out="$(awk -v file="$f" '
        function trim(s) { gsub(/^[ \t]+|[ \t]+$/, "", s); return s }
        function lc(s)   { return tolower(s) }
        function wordcount(s,   n, arr) {
            n = split(trim(s), arr, /[ \t]+/)
            if (trim(s) == "") return 0
            return n
        }
        function is_antipattern(cell,   c) {
            c = lc(trim(cell))
            # Section labels (whole-cell or label-prefixed fragments).
            if (c ~ /^composes with/)         return "section-label:composes-with"
            if (c ~ /^closure criteria/)      return "section-label:closure-criteria"
            if (c ~ /^fix direction/)         return "section-label:fix-direction"
            if (c ~ /^cascade requirement/)   return "section-label:cascade-requirement"
            if (c ~ /^canonical authority/)   return "section-label:canonical-authority"
            if (c == "scope" || c == "notes") return "section-label:bare"
            # Bare metadata fragments (whole-cell).
            if (c == "critical" || c == "high" || c == "medium" || c == "low")  return "bare-metadata:severity"
            if (c == "bug" || c == "feature" || c == "task")                    return "bare-metadata:type"
            if (c == "in progress" || c == "queued" || c == "open" || c == "done" || c == "blocked" || c == "reopened" || c == "obsolete") return "bare-metadata:status"
            # Bare section-marker / paragraph-letter alone.
            if (c ~ /^\(?[a-z]\)?$/)          return "bare-section-letter"
            if (c ~ /^§?[0-9]+(\.[0-9]+)*$/)  return "bare-section-marker"
            return ""
        }
        # A data row is a table line that starts with "|" and is NOT the header
        # separator (|---|---|) and NOT inside a counts-only header block.
        /^\|/ {
            line = $0
            # Skip separator rows.
            if (line ~ /^\|[ ]*-+/) next
            # Split into cells on unescaped pipes.
            ncell = split(line, cells, /\|/)
            # cells[1] is empty (leading |). Real cells are 2..ncell-1.
            # Header row detection: skip the header (contains "ID" + "Title" or
            # "Type" + "Count" style headers). We only check data rows whose
            # first real cell looks like an ID (prefix-NNN) — that targets the
            # per-item one-liner tables and skips aggregate/count tables.
            first = trim(cells[2])
            if (first !~ /^[A-Z]+-[0-9]+$/) next
            # The description one-liner is the LAST real cell.
            desc = trim(cells[ncell - 1])
            reason = is_antipattern(desc)
            wc = wordcount(desc)
            len = length(desc)
            if (reason != "") {
                printf("VIOLATION\t%s\t%s\tanti-pattern(%s): \"%s\"\n", file, first, reason, desc)
                viol++
            } else if (wc < 6 && len < 40) {
                printf("VIOLATION\t%s\t%s\ttoo-short (%d words / %d chars, need >=6 words OR >=40 chars): \"%s\"\n", file, first, wc, len, desc)
                viol++
            } else {
                printf("OK\t%s\t%s\n", file, first)
                ok++
            }
        }
        END { printf("SUMMARY\t%d\t%d\n", ok+0, viol+0) }
    ' "$f")"

    while IFS=$'\t' read -r kind a b c; do
        case "$kind" in
            VIOLATION)
                echo "FAIL §11.4.91  [$a] $b — $c" >&2
                violations=$((violations + 1))
                ;;
            OK)
                checked=$((checked + 1))
                ;;
            SUMMARY) : ;;
        esac
    done <<< "$out"
done

echo "§11.4.91 summary-clarity gate: ${checked} compliant row(s), ${violations} violation(s)"
if [[ "$violations" -gt 0 ]]; then
    exit 1
fi
exit 0
