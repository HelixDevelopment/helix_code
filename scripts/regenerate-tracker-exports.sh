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

    # HTML
    if pandoc "$in" -o "$DST/$base.html" --standalone --metadata title="$base"; then
        echo "OK   $base.html"
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
