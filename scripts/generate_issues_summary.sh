#!/usr/bin/env bash
# generate_issues_summary.sh — mechanically regenerate docs/Issues_Summary.md
# from docs/Issues.md per Constitution §11.4.91 (summary-doc clarity) +
# §11.4.12 (CM-ISSUES-SUMMARY-SYNC) + §11.4.15/16 (Status/Type closed-sets).
#
# Closes the long-standing "TODO: create" gap noted in docs/Issues.md's footer
# (the summary was previously hand-maintained, which let it drift from the
# tracker — see HXC-018 §11.4.91 tooling). The generator is the source of
# freshness: it extracts ONLY from each item's H2 heading line + the
# **Status:**/**Type:**/**Discovered:** field lines + the first descriptive
# body line (Closure/Evidence/Resolution), never from arbitrary prose, so a
# regenerated row can never carry a §11.4.91 anti-pattern note.
#
# Idempotent. Usage: scripts/generate_issues_summary.sh [--check]
#   (no args)  rewrite docs/Issues_Summary.md
#   --check    exit 1 (and print a unified diff) if the on-disk summary differs
#              from a fresh regeneration — the CM-ISSUES-SUMMARY-SYNC gate body.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/docs/Issues.md"
OUT="$ROOT/docs/Issues_Summary.md"
[[ -f "$SRC" ]] || { echo "FATAL: $SRC missing" >&2; exit 2; }

render() {
  awk '
    function flush(   notes) {
      if (id == "") return
      notes = note
      # §11.4.91 floor: Notes MUST be self-contained (>=6 words OR >=40 chars).
      # Migrated tombstones often carry no Closure/Evidence body line — give them
      # a meaningful pointer instead of a bare "—" (which the clarity gate FAILs).
      if (notes == "") {
        if (status ~ /\(→ Fixed\.md\)/) notes = "Closed — full closure record in docs/Fixed.md (" status ")"
        else notes = title
      }
      # collapse pipes/newlines so the row stays a single table cell
      gsub(/\|/, "/", title);  gsub(/\|/, "/", notes)
      gsub(/\r/, "", title);   gsub(/\r/, "", notes)
      printf "| %s | %s | %s | %s | %s | %s |\n", id, title, type, status, disc, notes
      # tally
      total++
      if (status ~ /(Fixed|Implemented|Completed|Obsolete) \(→/) closed++
      else open++
      id=""; title=""; type=""; status=""; disc=""; note=""
    }
    # H2 item heading: "## <ID> (...) — <title>"  (ID like HXC-001 / HXC-014b)
    /^## [A-Z]+-[0-9]+[a-z]? / {
      flush()
      line=$0; sub(/^## /, "", line)
      # id = first whitespace-delimited token
      id=line; sub(/[ \t].*$/, "", id)
      # title = text after the FIRST em-dash " — " ([^—]* stops at the first
      # em-dash, so this is non-greedy); fall back to dropping the ID token.
      title=line
      if (title ~ / — /) sub(/^[^—]*— /, "", title)
      else               sub(/^[^ ]+ +/, "", title)
      sub(/ — CLOSED.*$/, "", title)            # strip trailing migration marker
      sub(/ — [Mm]igrated.*$/, "", title)
      next
    }
    id != "" && /^\*\*Status:\*\*/   { status=$0; sub(/^\*\*Status:\*\* */, "", status); sub(/ —.*$/, "", status); next }
    id != "" && /^\*\*Type:\*\*/     { type=$0;   sub(/^\*\*Type:\*\* */, "", type);     next }
    id != "" && /^\*\*Discovered:\*\*/ {
      disc=$0; sub(/^\*\*Discovered:\*\* */, "", disc)
      # keep the leading ISO date only (drop any "(round …)" qualifier)
      if (disc ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}/) { sub(/ .*$/, "", disc) }
      next
    }
    # first descriptive body line → Notes (strip the **Label:** prefix, truncate)
    id != "" && note == "" && /^\*\*(Closure|Evidence|Resolution|Closure progress|Obsolete-Details)/ {
      note=$0
      sub(/^\*\*[^*]*\*\* */, "", note)
      sub(/^\([^)]*\) */, "", note)            # drop a leading "(date)" tag
      if (length(note) > 160) note=substr(note,1,157) "…"
      next
    }
    END { flush(); printf "@@COUNTS@@ %d %d %d\n", total, open, closed }
  ' "$SRC"
}

BODY="$(render)"
COUNTS_LINE="$(printf '%s\n' "$BODY" | grep '^@@COUNTS@@ ')"
read -r _ TOTAL OPEN CLOSED <<<"$COUNTS_LINE"
ROWS="$(printf '%s\n' "$BODY" | grep -v '^@@COUNTS@@ ')"
TODAY="$(date +%Y-%m-%d)"

NEW="$(cat <<EOF
# HelixCode — Issues Summary

> Generated **mechanically** from \`docs/Issues.md\` by \`scripts/generate_issues_summary.sh\` per Constitution §11.4.91 (summary clarity) + §11.4.12 (CM-ISSUES-SUMMARY-SYNC). Do not hand-edit — re-run the generator. Title column carries each item's H2 heading (self-contained, ≥40 chars per §11.4.91); Notes is the first Closure/Evidence/Resolution line.
>
> **Prefix convention:** IDs are scope-prefixed (\`HXC\`=root project; \`HXA\`=HelixAgent; \`HXL\`=HelixLLM; \`HXQ\`=HelixQA; \`HXV\`=LLMsVerifier; \`VEN\`=VisionEngine; \`PAN\`=panoptic; \`OPS\`=LLMOps). See \`docs/Issues.md\` "Prefix convention" for the legacy \`ISSUE-NNN\` mapping.

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
$ROWS

**Counts**: $TOTAL tracked item-sections in \`docs/Issues.md\` — **$OPEN open** (non-terminal status) / **$CLOSED closed** (terminal \`(→ Fixed.md)\` status; retained as §11.4.19 migration tombstones).

*Last regenerated: $TODAY by \`scripts/generate_issues_summary.sh\`. HTML/PDF exports via \`scripts/regenerate-tracker-exports.sh\`.*
EOF
)"

if [[ "${1:-}" == "--check" ]]; then
  if ! diff -u <(printf '%s\n' "$NEW") "$OUT" >/tmp/issues_summary.diff 2>/dev/null; then
    echo "CM-ISSUES-SUMMARY-SYNC: FAIL — docs/Issues_Summary.md is stale; run scripts/generate_issues_summary.sh" >&2
    cat /tmp/issues_summary.diff >&2
    exit 1
  fi
  echo "CM-ISSUES-SUMMARY-SYNC: PASS — summary in sync with Issues.md"
  exit 0
fi

printf '%s\n' "$NEW" > "$OUT"
echo "wrote $OUT ($TOTAL items: $OPEN open / $CLOSED closed)"
