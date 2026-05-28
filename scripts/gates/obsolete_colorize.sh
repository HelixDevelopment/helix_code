#!/usr/bin/env bash
# obsolete_colorize.sh — Constitution §11.4.90 colorizer.
#
# Post-processes a pandoc-rendered tracker HTML file: for every table row
# (<tr>...</tr>) whose Status cell reads "Obsolete (→ Fixed.md)", adds the
# class="cell-status-obsolete" attribute to the opening <tr> tag so the
# .cell-status-obsolete CSS rule (docs/_progress-style.css) renders the row
# with a light-gray (#E0E0E0) background + strikethrough description.
#
# Idempotent: a row already carrying the class is left untouched.
#
# Usage: obsolete_colorize.sh <file.html> [<file.html> ...]
# Exit:  0 on success, 2 on usage error, 1 if any file could not be written.
set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "usage: $0 <file.html> [<file.html> ...]" >&2
    exit 2
fi

# The literal Status value §11.4.90 mandates for obsolete items.
OBSOLETE_MARKER='Obsolete (→ Fixed.md)'

EXIT=0
for html in "$@"; do
    if [[ ! -f "$html" ]]; then
        echo "SKIP $html — not a file"
        continue
    fi

    tmp="$(mktemp)"
    # awk pass: buffer each <tr>...</tr> block; if the block contains the
    # obsolete marker and the opening <tr> lacks the class, inject it.
    if awk -v marker="$OBSOLETE_MARKER" '
        BEGIN { in_row = 0; buf = ""; close_tag = "</" "tr>" }
        {
            line = $0
            if (in_row == 0 && line ~ /<tr[ >]/) {
                # Start buffering a row.
                in_row = 1
                buf = line
                if (index(line, close_tag) > 0) {
                    # Single-line row.
                    flush_row()
                }
                next
            }
            if (in_row == 1) {
                buf = buf "\n" line
                if (index(line, close_tag) > 0) {
                    flush_row()
                }
                next
            }
            print line
        }
        function flush_row(   out) {
            in_row = 0
            out = buf
            if (index(out, marker) > 0 && out !~ /class="cell-status-obsolete"/) {
                # Add the class to the opening <tr> tag only. The two forms
                # (<tr> bare vs <tr ...attrs>) are mutually exclusive, so try
                # the bare form first and fall back to the attributed form.
                if (sub(/<tr>/, "<tr class=\"cell-status-obsolete\">", out) == 0) {
                    sub(/<tr /, "<tr class=\"cell-status-obsolete\" ", out)
                }
            }
            print out
            buf = ""
        }
        END {
            if (in_row == 1) { print buf }
        }
    ' "$html" > "$tmp"; then
        if mv "$tmp" "$html"; then
            n=$(grep -c 'class="cell-status-obsolete"' "$html" || true)
            echo "OK   $html — ${n} obsolete row(s) tagged"
        else
            echo "FAIL $html — could not write"
            rm -f "$tmp"
            EXIT=1
        fi
    else
        echo "FAIL $html — colorize pass errored"
        rm -f "$tmp"
        EXIT=1
    fi
done

exit $EXIT
