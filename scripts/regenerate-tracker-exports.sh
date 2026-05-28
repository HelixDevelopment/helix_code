#!/usr/bin/env bash
# regenerate-tracker-exports.sh — render docs/{Issues,Issues_Summary,Fixed,Fixed_Summary}.md
# as HTML + PDF per Constitution §11.4.19. Idempotent — safe to re-run.
#
# Requires: pandoc (mandatory)
# Optional: xelatex / pdflatex / weasyprint / wkhtmltopdf  (any one — PDF skipped if none)
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/docs"
DST="$SRC/exports"
CSS="$SRC/_progress-style.css"            # §11.4.90 tracker styling
COLORIZE="$ROOT/scripts/gates/obsolete_colorize.sh"  # §11.4.90 colorizer
mkdir -p "$DST"

if ! command -v pandoc >/dev/null 2>&1; then
    echo "FATAL: pandoc not installed — install pandoc and re-run" >&2
    exit 2
fi

SOURCES=(Issues.md Issues_Summary.md Fixed.md Fixed_Summary.md)
EXIT=0

# Pick the first available PDF engine
PDF_ENGINE=""
for eng in xelatex pdflatex weasyprint wkhtmltopdf; do
    if command -v "$eng" >/dev/null 2>&1; then
        PDF_ENGINE="$eng"
        break
    fi
done

if [[ -z "$PDF_ENGINE" ]]; then
    echo "WARN: no PDF engine found (xelatex/pdflatex/weasyprint/wkhtmltopdf) — PDF exports skipped"
fi

for src in "${SOURCES[@]}"; do
    base="${src%.md}"
    in="$SRC/$src"
    if [[ ! -f "$in" ]]; then
        echo "SKIP $src — source missing"
        continue
    fi

    # HTML — embed §11.4.90 tracker stylesheet so the standalone export
    # carries .cell-status-obsolete (light-gray + strikethrough) styling.
    css_args=()
    if [[ -f "$CSS" ]]; then
        css_args=(--css "$CSS" --embed-resources)
    fi
    if pandoc "$in" -o "$DST/$base.html" --standalone "${css_args[@]}" --metadata title="$base"; then
        echo "OK   $base.html"
        # §11.4.90 colorizer: tag Obsolete rows post-render.
        if [[ -x "$COLORIZE" ]]; then
            "$COLORIZE" "$DST/$base.html" || { echo "FAIL $base.html colorize"; EXIT=1; }
        fi
    else
        echo "FAIL $base.html"
        EXIT=1
    fi

    # PDF
    if [[ -n "$PDF_ENGINE" ]]; then
        if pandoc "$in" -o "$DST/$base.pdf" --pdf-engine="$PDF_ENGINE"; then
            echo "OK   $base.pdf (via $PDF_ENGINE)"
        else
            echo "FAIL $base.pdf"
            EXIT=1
        fi
    else
        echo "SKIP $base.pdf — no PDF engine"
    fi
done

exit $EXIT
