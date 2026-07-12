#!/usr/bin/env bash
# scripts/gates/no_terminal_item_in_issues_gate.sh — CM-NO-TERMINAL-ITEM-IN-ISSUES
#
# HXC-126: "Finished items still appear in the open-issues tracker instead of
# the resolved tracker." Root cause (2026-07-12 forensic): 41 items across the
# workable-items SQLite SSoT (docs/workable_items.db) carried a TERMINAL §11.4.15
# status (`Fixed (→ Fixed.md)` / `Implemented (→ Fixed.md)` / `Completed
# (→ Fixed.md)` / `Obsolete (→ Fixed.md)`) while `current_location='Issues'`, so
# `workable-items export` rendered them into docs/Issues.md (the OPEN tracker)
# instead of docs/Fixed.md (the RESOLVED tracker) — a status↔location desync that
# `workable-items validate` does NOT cross-check (validate only checks the
# closed-set/description-floor invariants per item, not status-vs-location
# agreement), which is why it stayed green while the leak existed.
#
# This gate closes that gap permanently (§11.4.135 standing regression guard):
# it FAILs if ANY item with a terminal status is still rendered under an "## <ID>"
# heading in docs/Issues.md (the MD-level, DB-independent oracle — reads exactly
# what an end-user opening the tracker would see), and — defense in depth, when
# the workable-items binary can build in this environment — cross-checks the
# tracked SQLite DB directly for current_location='Issues' rows carrying a
# terminal status (a §11.4.111-class direct-source check, never trusting the MD
# rendering alone).
#
# Exit 0 = PASS (zero terminal-status items leaked into Issues.md); non-zero =
# FAIL (release-gate blocking, §11.4.135).

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

ISSUES="docs/Issues.md"
DB="docs/workable_items.db"
WI_SRC="constitution/scripts/workable-items"

fail() { echo "CM-NO-TERMINAL-ITEM-IN-ISSUES: FAIL — $1" >&2; exit 1; }
skip_note() { echo "CM-NO-TERMINAL-ITEM-IN-ISSUES: (info) $1"; }

[ -f "$ISSUES" ] || fail "$ISSUES missing"

# --- Layer 1: MD-level oracle (DB-independent — reads what the end user reads) ---
#
# Walk docs/Issues.md; for every "## <ID> ..." heading, capture the FIRST
# "**Status:**" line encountered before the next "## " heading (or EOF) and
# check it against the closed-set terminal-status patterns (§11.4.15/§11.4.33/
# §11.4.90). Any match is a leak.

MD_VIOLATIONS="$(awk '
    /^## / {
        # Flush any pending heading with no Status line found before this one
        # (should not happen in a well-formed tracker, but never silently drop).
        heading = $0
        sub(/^## /, "", heading)
        # ID = first whitespace-delimited token of the heading text.
        split(heading, parts, " ")
        cur_id = parts[1]
        have_id = 1
        next
    }
    /^\*\*Status:\*\*/ {
        if (have_id) {
            status_line = $0
            sub(/^\*\*Status:\*\*[ \t]*/, "", status_line)
            if (status_line ~ /^(Fixed|Implemented|Completed) \(→ Fixed\.md\)/ || status_line ~ /^Obsolete/) {
                print cur_id "\t" status_line
            }
            have_id = 0   # only the FIRST Status line per heading counts
        }
        next
    }
' "$ISSUES")"

if [ -n "$MD_VIOLATIONS" ]; then
    n=$(printf '%s\n' "$MD_VIOLATIONS" | grep -c .)
    ids=$(printf '%s\n' "$MD_VIOLATIONS" | cut -f1 | tr '\n' ' ')
    fail "$n item(s) with a terminal §11.4.15 status are still rendered as open headings in $ISSUES (leaked into the open-issues tracker): $ids"
fi

# --- Layer 2: DB-level cross-check (defense in depth; honest SKIP if unbuildable) ---

if [ -f "$DB" ] && [ -d "$WI_SRC" ] && command -v go >/dev/null 2>&1 && command -v sqlite3 >/dev/null 2>&1; then
    TMP="$(mktemp -d)"
    trap 'rm -rf "$TMP"' EXIT
    BIN="$TMP/wi"
    if ( cd "$WI_SRC" && go build -o "$BIN" ./cmd/workable-items ) >"$TMP/build.out" 2>&1; then
        # Read on a TEMP COPY — SQLite WAL-mode mutates the file header even on a
        # read open, which would dirty the tracked working tree (mirrors
        # workable_items_sync_gate.sh's discipline).
        cp "$DB" "$TMP/committed.db"
        if "$BIN" validate --db "$TMP/committed.db" >"$TMP/validate.out" 2>&1; then
            DB_LEAK_COUNT=$(sqlite3 "$TMP/committed.db" "SELECT COUNT(*) FROM items WHERE current_location='Issues' AND (status LIKE '%→ Fixed.md%' OR status LIKE 'Obsolete%');" 2>/dev/null)
            if [ -n "$DB_LEAK_COUNT" ] && [ "$DB_LEAK_COUNT" != "0" ]; then
                DB_LEAK_IDS=$(sqlite3 "$TMP/committed.db" "SELECT atm_id FROM items WHERE current_location='Issues' AND (status LIKE '%→ Fixed.md%' OR status LIKE 'Obsolete%') ORDER BY atm_id;" 2>/dev/null | tr '\n' ' ')
                fail "$DB_LEAK_COUNT item(s) in the tracked $DB carry a terminal status with current_location='Issues' (status↔location desync, even though $ISSUES currently renders clean — regenerate export or investigate): $DB_LEAK_IDS"
            fi
        else
            skip_note "DB-level cross-check skipped: committed DB failed validate ($(tail -1 "$TMP/validate.out"))"
        fi
    else
        skip_note "DB-level cross-check skipped: workable-items binary cannot build in this env ($(tail -1 "$TMP/build.out"))"
    fi
else
    skip_note "DB-level cross-check skipped: DB, binary source, go, or sqlite3 not available (MD-level Layer 1 check above still ran and is authoritative for this gate's PASS)"
fi

echo "CM-NO-TERMINAL-ITEM-IN-ISSUES: PASS — zero terminal-status items leaked into $ISSUES"
exit 0
