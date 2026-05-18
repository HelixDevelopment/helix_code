# Tracker Exports

Generated via `scripts/regenerate-tracker-exports.sh`. Per Constitution §11.4.19,
Issues + Fixed trackers must be available in Markdown + HTML + PDF formats kept
in sync via `CM-DOCS-EXPORT-SYNC` discipline.

Source files are `docs/{Issues,Issues_Summary,Fixed,Fixed_Summary}.md` —
regenerate exports after any source edit.

## Regenerate

```bash
bash scripts/regenerate-tracker-exports.sh
```

Requires `pandoc`. PDF generation additionally requires one of:
`xelatex` / `pdflatex` / `weasyprint` / `wkhtmltopdf` (first available is picked).

## Exemption from CONST-053

Build derivatives are normally excluded from version control (CONST-053).
Tracker exports here are an **explicit exception** required by §11.4.19 —
they ship as committed exports so PR reviewers and operators without pandoc
can still consume the Markdown / HTML / PDF artefacts as a synchronised set.
