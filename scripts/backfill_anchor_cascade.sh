#!/usr/bin/env bash
# ============================================================================
# Deterministic governance-anchor backfill (CONST-047/049 cascade repair).
#
# Closes the verify-governance-cascade.sh gap by ADDITIVELY splicing the
# verbatim canonical anchor sections from the golden, already-passing
# reference submodule (helix_qa) into a target owned submodule's three
# governance files. Additive-only: never deletes/rewrites existing content
# (§11.4.122 no-silent-removal). Idempotent: re-running is a no-op once green.
#
# Usage:  scripts/backfill_anchor_cascade.sh <target-submodule-dir>
#   e.g.  scripts/backfill_anchor_cascade.sh submodules/helix_agent
#
# Golden source: submodules/helix_qa (passes verify-governance-cascade.sh).
# ============================================================================
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

GOLDEN="submodules/helix_qa"
TARGET="${1:?usage: backfill_anchor_cascade.sh <target-submodule-dir>}"
TARGET="${TARGET%/}"

[ -d "$GOLDEN" ] || { echo "FATAL: golden $GOLDEN missing"; exit 2; }
[ -d "$TARGET" ] || { echo "FATAL: target $TARGET missing"; exit 2; }

# Required CONST literals (verifier scope) + §11.9.
CONST_TOKENS="CONST-047 CONST-048 CONST-049 CONST-050 CONST-051 CONST-052 CONST-053 CONST-054 CONST-055 CONST-056 CONST-057 CONST-058 CONST-059 CONST-060"

# Extract the verbatim section for a heading line from golden file.
# A section = from the matched heading to the line before the next ^## / ^### .
extract_heading_section() {  # $1=golden_file  $2=heading_literal (regex-safe prefix)
  awk -v h="$2" '
    index($0, h)==1 && !started { started=1; print; next }
    started && (/^## / || /^### /) { exit }
    started { print }
  ' "$1"
}

# Extract a CONST-NNN section: golden "## CONST-NNN:" up to next ^## .
extract_const_section() {  # $1=golden_file  $2=CONST-NNN
  awk -v t="## $2:" '
    index($0, t)==1 && !started { started=1; print; next }
    started && /^## / { exit }
    started { print }
  ' "$1"
}

# Extract the §11.9 bold block: golden line starting "**§11.9" up to next ^## .
extract_sec119() {  # $1=golden_file
  awk '
    /^\*\*§11\.9 / && !started { started=1; print; next }
    started && /^## / { exit }
    started { print }
  ' "$1"
}

changed_any=0
for fname in CONSTITUTION.md CLAUDE.md AGENTS.md; do
  gfile="$GOLDEN/$fname"
  tfile="$TARGET/$fname"
  [ -f "$gfile" ] || { echo "WARN: golden $gfile missing — skip $fname"; continue; }
  [ -f "$tfile" ] || { echo "WARN: target $tfile missing — skip $fname"; continue; }

  appended=""
  file_changed=0

  # --- §11.9 (verifier anchor is the regex "Article XI.*11.9") ---
  if ! grep -qE 'Article XI.{0,60}11\.9' "$tfile"; then
    sec="$(extract_sec119 "$gfile")"
    title='### Article XI §11.9 — Anti-Bluff Forensic Anchor (CONST-035)'
    appended+=$'\n'"$title"$'\n\n'"$sec"$'\n'
    file_changed=1; echo "  + $fname: Article XI §11.9"
  fi

  # --- CONST-NNN ---
  for ct in $CONST_TOKENS; do
    if ! grep -qF "$ct" "$tfile"; then
      sec="$(extract_const_section "$gfile" "$ct")"
      if [ -n "$sec" ]; then appended+=$'\n'"$sec"$'\n'; file_changed=1; echo "  + $fname: $ct"; fi
    fi
  done

  # --- §11.4.NNN heading anchors (## for 69-121, ### for 122-134) ---
  # Iterate golden's own anchor heading lines so we mirror the exact literals.
  while IFS= read -r hline; do
    # hline like "## §11.4.103 — Title..." or "### §11.4.122 — ..."
    lvl="${hline%% §11.4*}"          # "##" or "###"
    rest="${hline#"$lvl" §11.4.}"     # "103 — Title"
    nnn="${rest%% *}"                 # "103"
    lit="$lvl §11.4.${nnn} —"        # literal the verifier checks
    if ! grep -qF "$lit" "$tfile"; then
      sec="$(extract_heading_section "$gfile" "$lvl §11.4.${nnn} —")"
      if [ -n "$sec" ]; then appended+=$'\n'"$sec"$'\n'; file_changed=1; echo "  + $fname: $lit"; fi
    fi
  done < <(grep -E '^(##|###) §11\.4\.[0-9]+ —' "$gfile")

  if [ "$file_changed" -eq 1 ]; then
    {
      printf '\n\n<!-- ============================================================\n'
      printf '     CASCADED GOVERNANCE ANCHORS (backfill_anchor_cascade.sh)\n'
      printf '     Additive cascade from golden reference (helix_qa) per\n'
      printf '     CONST-047/049 — universal anchors only, additive (§11.4.122).\n'
      printf '     ============================================================ -->\n'
      printf '%s\n' "$appended"
    } >> "$tfile"
    changed_any=1
  else
    echo "  = $fname: already complete"
  fi
done

echo "DONE: $TARGET (changed=$changed_any)"
