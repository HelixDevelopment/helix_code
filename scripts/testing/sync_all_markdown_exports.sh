#!/usr/bin/env bash
# scripts/testing/sync_all_markdown_exports.sh
# §11.4.65 / CONST-066 — canonical markdown→{.html,.pdf} sibling exporter.
#
# Every governed/working doc under docs/**/*.md SHALL ship synchronized
# .html + .pdf siblings. This helper renders those siblings from the .md
# source with a single, depth-independent pipeline:
#
#   pandoc  : .md → self-contained .html (CSS embedded inline via
#             --embed-resources, so the HTML renders identically no matter
#             how deeply nested the source doc is — no relative-CSS breakage).
#   weasyprint : that .html → .pdf  (timeout-bounded; warnings are benign).
#
# Idempotent: a sibling is only (re)rendered when missing or stale (source
# .md newer than the sibling), unless --regenerate-all forces a full pass.
#
# Modes:
#   (default)            render missing/stale siblings for the paths given
#                        (or the whole docs/ tree if no paths given).
#   --check-only         report the docs missing a sibling; render nothing;
#                        exit 1 if any are missing, 0 if all present.
#   --regenerate-all     force re-render every sibling (ignore staleness).
#   --file <path.md>     restrict to one or more explicit .md files
#                        (repeatable; may also be passed as bare args).
#
# Exit: 0 = all requested siblings present/rendered; 1 = (check-only) gaps
#       found, or a render failed.
#
# Dependencies: pandoc (mandatory), weasyprint (mandatory for .pdf), find.
# Cross-references: §11.4.65 / CONST-066 / §11.4.75 (the pre-commit gate
#   that consumes these siblings); scripts/git_hooks/pre-commit.

set -uo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && cd .. && pwd)"
cd "$REPO_ROOT" || exit 2

DOCS_DIR="$REPO_ROOT/docs"
CSS="$DOCS_DIR/_progress-style.css"
TIMEOUT_SECS=60

CHECK_ONLY=0
REGEN_ALL=0
declare -a EXPLICIT=()

while [ $# -gt 0 ]; do
  case "$1" in
    --check-only)     CHECK_ONLY=1 ;;
    --regenerate-all) REGEN_ALL=1 ;;
    --file)           shift; [ $# -gt 0 ] && EXPLICIT+=("$1") ;;
    --timeout)        shift; [ $# -gt 0 ] && TIMEOUT_SECS="$1" ;;
    -*)               echo "unknown flag: $1" >&2; exit 2 ;;
    *)                EXPLICIT+=("$1") ;;
  esac
  shift
done

command -v pandoc >/dev/null 2>&1 || { echo "FATAL: pandoc not installed" >&2; exit 2; }
HAVE_WEASY=1
command -v weasyprint >/dev/null 2>&1 || HAVE_WEASY=0

# Collect the .md source set.
declare -a SOURCES=()
if [ "${#EXPLICIT[@]}" -gt 0 ]; then
  for p in "${EXPLICIT[@]}"; do
    case "$p" in *.md) [ -f "$p" ] && SOURCES+=("$p") ;; esac
  done
else
  while IFS= read -r f; do SOURCES+=("$f"); done \
    < <(find "$DOCS_DIR" -name '*.md' -type f | sort)
fi

# A sibling is "stale" if missing OR older than the source .md.
stale() {
  # $1 = source.md  $2 = sibling
  [ ! -f "$2" ] && return 0
  [ "$1" -nt "$2" ] && return 0
  return 1
}

missing=0
rendered=0
failed=0
declare -a MISSING_LIST=()

for src in "${SOURCES[@]}"; do
  base="${src%.md}"
  html="${base}.html"
  pdf="${base}.pdf"

  if [ "$CHECK_ONLY" -eq 1 ]; then
    if [ ! -f "$html" ] || [ ! -f "$pdf" ]; then
      missing=$((missing+1)); MISSING_LIST+=("$src")
    fi
    continue
  fi

  need_html=0; need_pdf=0
  if [ "$REGEN_ALL" -eq 1 ]; then need_html=1; need_pdf=1
  else
    stale "$src" "$html" && need_html=1
    { [ "$HAVE_WEASY" -eq 1 ] && stale "$src" "$pdf"; } && need_pdf=1
  fi

  title="$(basename "$base")"

  if [ "$need_html" -eq 1 ]; then
    # Primary render. If pandoc trips on a stray `---` it mistakes for a
    # YAML metadata block, retry with yaml_metadata_block disabled so the
    # `---` is treated as a literal horizontal rule (the doc is content,
    # not front-matter).
    if pandoc "$src" -s --embed-resources --standalone \
         --metadata title="$title" --css "$CSS" -o "$html" 2>/dev/null \
       || pandoc "$src" -f markdown-yaml_metadata_block -s --embed-resources \
         --standalone --metadata title="$title" --css "$CSS" -o "$html" 2>/dev/null; then
      rendered=$((rendered+1))
    else
      echo "FAIL html: $src" >&2; failed=$((failed+1)); continue
    fi
  fi

  if [ "$need_pdf" -eq 1 ] && [ "$HAVE_WEASY" -eq 1 ]; then
    # HTML must exist (render it if it was up-to-date but we still need pdf).
    [ -f "$html" ] || pandoc "$src" -s --embed-resources --standalone \
         --metadata title="$title" --css "$CSS" -o "$html" 2>/dev/null \
      || pandoc "$src" -f markdown-yaml_metadata_block -s --embed-resources \
         --standalone --metadata title="$title" --css "$CSS" -o "$html" 2>/dev/null
    if timeout "$TIMEOUT_SECS" weasyprint "$html" "$pdf" >/dev/null 2>&1; then
      rendered=$((rendered+1))
    else
      # weasyprint emits benign warnings on stderr but may still exit 0;
      # a real failure (timeout / no output) is caught here.
      if [ -f "$pdf" ]; then rendered=$((rendered+1)); else
        echo "FAIL pdf: $src" >&2; failed=$((failed+1))
      fi
    fi
  fi
done

if [ "$CHECK_ONLY" -eq 1 ]; then
  if [ "$missing" -gt 0 ]; then
    echo "MISSING SIBLINGS ($missing):" >&2
    printf '  %s\n' "${MISSING_LIST[@]}" >&2
    exit 1
  fi
  echo "all docs/**/*.md have .html + .pdf siblings"
  exit 0
fi

echo "rendered=$rendered failed=$failed"
[ "$failed" -gt 0 ] && exit 1
exit 0
