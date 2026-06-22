#!/usr/bin/env bash
# ============================================================================
# Deterministic governance-anchor backfill (CONST-047/049 cascade repair).
#
# Closes the verify-governance-cascade.sh gap by ADDITIVELY splicing the
# verbatim canonical anchor sections into a target owned submodule's three
# governance files. Additive-only: never deletes/rewrites existing content
# (§11.4.122 no-silent-removal). Idempotent: re-running is a no-op once green.
#
# TWO SOURCES, by anchor format (the gate greps for the EXACT literal of each):
#   1. helix_qa golden submodule  — §11.9 + CONST-047..060 + the heading-format
#      covenant-114 anchors (## / ### §11.4.NN —, ranges 69..134). helix_qa
#      passes the verifier for THOSE anchors, so it is a valid golden for them.
#   2. constitution submodule carriers (CANONICAL per §11.4.35) — the
#      BOLD-INLINE covenant-114 band `**§11.4.N —` (range 135..165, the gate
#      ceiling) PLUS the blockquote-prefixed §11.4.140 (`> **§11.4.140 —`, whose
#      verifier literal is the BARE form `§11.4.140 —`). helix_qa does NOT carry
#      this band, so it cannot be a golden for it; the canonical constitution
#      carriers do. The gate only checks the literal STRING per file, so all
#      three target carriers are satisfied by sourcing the band from a
#      constitution carrier that actually holds the bold-inline literals.
#      constitution/Constitution.md carries this band in a NON-bold table/prose
#      form (0 bold-inline literals), so it is NOT a valid source for the gate
#      literal; constitution/CLAUDE.md and constitution/AGENTS.md each carry the
#      full bold-inline band.
#      Map: target CLAUDE.md + CONSTITUTION.md  <- constitution/CLAUDE.md band
#           target AGENTS.md                    <- constitution/AGENTS.md band
#      NOTE: the band lower bound was 142 (bug): a repo lagging at ≤§11.4.139 was
#      left with 135..141 unfixed (silent FAIL) — fleet-wide success masked it
#      because every other repo already carried 135..141 from a prior cascade.
#      CONST-058/CONST-059 fall back to the meta-repo ROOT CONSTITUTION.md when
#      the helix_qa golden condensed them into an inline reference (no heading).
#
# Usage:  scripts/backfill_anchor_cascade.sh <target-submodule-dir>
#   e.g.  scripts/backfill_anchor_cascade.sh submodules/helix_agent
# ============================================================================
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

GOLDEN="submodules/helix_qa"
# Canonical bold-inline band source (§11.4.35). constitution/Constitution.md is
# intentionally NOT used here: it lacks the bold-inline `**§11.4.N —` literals.
CONST_CLAUDE="constitution/CLAUDE.md"
CONST_AGENTS="constitution/AGENTS.md"
# Meta-repo ROOT CONSTITUTION.md — used ONLY as a fallback CONST-NNN section
# source when the helix_qa golden condensed a CONST anchor into an inline
# reference (no `## CONST-NNN:` heading) so extract_const_section() yields empty
# for that file. The root CONSTITUTION.md carries `## CONST-058:` / `## CONST-059:`
# heading sections the golden CONSTITUTION.md lacks. (§11.4.6 — no faked
# completeness: a CONST literal that extracts empty MUST be sourced where it
# genuinely exists, not silently skipped.)
ROOT_CONSTITUTION="CONSTITUTION.md"
# Bold-inline covenant-114 band the verifier requires: 135..165 (gate ceiling;
# §11.4.166 REPEALED 2026-06-22, retired from the verifier's anchor list).
# Lowered from 142 to 135 so a repo lagging at ≤§11.4.139 (e.g. doc_processor at
# §11.4.97) gets 135..141 spliced from the SAME canonical constitution carriers
# the 142..165 band already uses — previously the loop started at 142 and NEVER
# spliced 135..141, so a lagging repo was left with those anchors unfixed (silent
# FAIL); fleet-wide success was only because every other repo already carried
# 135..141 from a prior cascade. §11.4.140 is special-cased below: its verifier
# literal is the BARE form `§11.4.140 —` and its opener line is blockquote-
# prefixed (`> **§11.4.140 —`), so the col0 extract_bold_anchor() cannot match it.
BOLD_BAND_LO=135
BOLD_BAND_HI=165
# Anchors in the band whose opener is NOT a col0 `**§11.4.N —` and so need a
# dedicated extractor (see the band loop below). §11.4.140's canonical-carrier
# opener is `> **§11.4.140 —` (inside a blockquote).
BOLD_BAND_BLOCKQUOTE_ANCHORS=" 140 "
TARGET="${1:?usage: backfill_anchor_cascade.sh <target-submodule-dir>}"
TARGET="${TARGET%/}"

[ -d "$GOLDEN" ] || { echo "FATAL: golden $GOLDEN missing"; exit 2; }
[ -d "$TARGET" ] || { echo "FATAL: target $TARGET missing"; exit 2; }
[ -f "$CONST_CLAUDE" ] || { echo "FATAL: canonical band source $CONST_CLAUDE missing"; exit 2; }
[ -f "$CONST_AGENTS" ] || { echo "FATAL: canonical band source $CONST_AGENTS missing"; exit 2; }
[ -f "$ROOT_CONSTITUTION" ] || { echo "FATAL: CONST fallback source $ROOT_CONSTITUTION missing"; exit 2; }

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

# Extract one BOLD-INLINE covenant-114 anchor section (`**§11.4.N —` band).
# A section = from the line whose first chars are the anchor literal, up to the
# line BEFORE the next bold-inline anchor opener (`**§11.4.`) or EOF. This
# absorbs wrapped prose AND any standalone `**Canonical authority:**` blocks
# that belong to the SAME anchor (they open with `**Canonical`, not `**§11.4.`),
# so a multi-block anchor is captured whole. Match is by string index (the
# literal contains §/— so a regex would need escaping); robust against the long
# single-line paragraphs the canonical carriers use.
extract_bold_anchor() {  # $1=source_file  $2=anchor_literal (e.g. "**§11.4.142 —")
  awk -v h="$2" '
    index($0, h)==1 && !started { started=1; print; next }
    started && index($0, "**§11.4.")==1 { exit }
    started { print }
  ' "$1"
}

# Extract a BLOCKQUOTE-PREFIXED covenant-114 anchor (canonical opener form
# `> **§11.4.N —`, e.g. §11.4.140 which lives inside a Markdown blockquote).
# A section = from the line whose first chars are `> **§11.4.N —`, up to the
# line BEFORE the next anchor opener (either a col0 `**§11.4.` bold opener OR a
# blockquote `> **§11.4.` opener) or EOF. The spliced text contains the BARE
# `§11.4.N —` substring the verifier greps for (the gate uses grep -qF anywhere
# in the line, so the `> **§11.4.140 — …` line satisfies it). Distinct from
# extract_bold_anchor because that matches `**…` at col0 and the blockquote
# opener has the `> ` prefix.
extract_blockquote_anchor() {  # $1=source_file  $2=bq_anchor_literal (e.g. "> **§11.4.140 —")
  awk -v h="$2" '
    index($0, h)==1 && !started { started=1; print; next }
    started && (index($0, "**§11.4.")==1 || index($0, "> **§11.4.")==1) { exit }
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
  # Source each CONST section from the helix_qa golden by its `## CONST-NNN:`
  # heading. When the golden CONDENSED a CONST anchor into an inline reference
  # (no heading — true for CONST-058/CONST-059 in the golden CONSTITUTION.md),
  # extract_const_section() returns EMPTY; fall back to the meta-repo ROOT
  # CONSTITUTION.md which carries the `## CONST-NNN:` heading section. Without
  # this fallback those two anchors silently extracted empty (NOT appended) for
  # CONSTITUTION.md targets, leaving the literal absent — a verifier FAIL the
  # cascade pretended to fix. (§11.4.6 — source the literal where it exists.)
  for ct in $CONST_TOKENS; do
    if ! grep -qF "$ct" "$tfile"; then
      sec="$(extract_const_section "$gfile" "$ct")"
      if [ -z "$sec" ]; then
        sec="$(extract_const_section "$ROOT_CONSTITUTION" "$ct")"
        [ -n "$sec" ] && echo "  ~ $fname: $ct sourced from $ROOT_CONSTITUTION (golden lacks heading)"
      fi
      if [ -n "$sec" ]; then
        appended+=$'\n'"$sec"$'\n'; file_changed=1; echo "  + $fname: $ct"
      else
        echo "  ! $fname: $ct missing from target but extracted empty from BOTH golden and $ROOT_CONSTITUTION (NOT appended)" >&2
      fi
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

  # --- §11.4.135..165 BOLD-INLINE band (`**§11.4.N —`, plus the blockquote-
  #     prefixed §11.4.140) from the CANONICAL constitution carriers (§11.4.35).
  #     The gate checks the literal STRING per file, so AGENTS.md sources the
  #     band from constitution/AGENTS.md and CLAUDE.md + CONSTITUTION.md source
  #     it from constitution/CLAUDE.md (the carriers that actually hold the
  #     bold-inline literals; Constitution.md does not). Append in ascending
  #     order, missing anchors only (idempotent). Lower bound is 135 so a repo
  #     lagging at ≤§11.4.139 receives 135..141 too (the original bug: band
  #     started at 142, never splicing 135..141).
  case "$fname" in
    AGENTS.md) bfile="$CONST_AGENTS" ;;
    *)         bfile="$CONST_CLAUDE" ;;   # CLAUDE.md and CONSTITUTION.md
  esac
  n="$BOLD_BAND_LO"
  while [ "$n" -le "$BOLD_BAND_HI" ]; do
    # §11.4.140 is BLOCKQUOTE-prefixed in the canonical carriers: the verifier
    # greps the BARE form `§11.4.140 —` and the opener line is `> **§11.4.140 —`,
    # so the col0 extract_bold_anchor() cannot match it. Use the bare-literal
    # gate-check + the dedicated extract_blockquote_anchor() for these anchors.
    if [ "${BOLD_BAND_BLOCKQUOTE_ANCHORS#* $n }" != "$BOLD_BAND_BLOCKQUOTE_ANCHORS" ]; then
      gate_lit="§11.4.${n} —"            # bare form the verifier greps
      src_lit="> **§11.4.${n} —"          # blockquote opener in the canonical carrier
      if grep -qF -- "$src_lit" "$bfile"; then
        if ! grep -qF -- "$gate_lit" "$tfile"; then
          sec="$(extract_blockquote_anchor "$bfile" "$src_lit")"
          if [ -n "$sec" ]; then
            appended+=$'\n'"$sec"$'\n'; file_changed=1; echo "  + $fname: $gate_lit (blockquote)"
          else
            echo "  ! $fname: $src_lit present in $bfile but extracted empty (NOT appended)" >&2
          fi
        fi
      else
        echo "  ! $fname: canonical source $bfile is MISSING blockquote opener $src_lit" >&2
      fi
      n=$((n+1)); continue
    fi
    lit="**§11.4.${n} —"
    if grep -qF -- "$lit" "$bfile"; then
      if ! grep -qF -- "$lit" "$tfile"; then
        sec="$(extract_bold_anchor "$bfile" "$lit")"
        if [ -n "$sec" ]; then
          appended+=$'\n'"$sec"$'\n'; file_changed=1; echo "  + $fname: $lit"
        else
          echo "  ! $fname: $lit present in $bfile but extracted empty (NOT appended)" >&2
        fi
      fi
    else
      # The verifier requires this literal; if the canonical source lacks it we
      # must surface it, not silently skip (§11.4.6 — no faked completeness).
      echo "  ! $fname: canonical source $bfile is MISSING required literal $lit" >&2
    fi
    n=$((n+1))
  done

  if [ "$file_changed" -eq 1 ]; then
    {
      printf '\n\n<!-- ============================================================\n'
      printf '     CASCADED GOVERNANCE ANCHORS (backfill_anchor_cascade.sh)\n'
      printf '     Additive cascade per CONST-047/049 — universal anchors only,\n'
      printf '     additive (§11.4.122). Sources: helix_qa golden (heading-format\n'
      printf '     anchors) + canonical constitution carriers (§11.4.35) for the\n'
      printf '     bold-inline §11.4.142..165 band.\n'
      printf '     ============================================================ -->\n'
      printf '%s\n' "$appended"
    } >> "$tfile"
    changed_any=1
  else
    echo "  = $fname: already complete"
  fi
done

echo "DONE: $TARGET (changed=$changed_any)"
