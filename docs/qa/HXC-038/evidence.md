# HXC-038 — docs_chain G14 (partial: fixed+governance green; issues = operator-gated residual)
ROOT CAUSE (corrected by subagent): docs_chain transform contract is output-FILE based (runner.go execTransform reads
the engine-supplied outName), NOT stdout-capture. generate_fixed_summary.sh ignored the engine's output path + wrote
its own hardcoded file → engine read empty → fixed_summary STALE. FIX (6183b454): --stdout mode honoring the engine
output-file (default file-write/--check unchanged); fixed.yaml gen-fixed-summary → --stdout.
PROGRESS: docs_chain verify --all → `fixed` in-sync ✓, `governance` in-sync ✓ (regenerated CLAUDE.html/pdf + Fixed.html/pdf siblings); DB/Issues NOT touched (porcelain clean).
RESIDUAL (operator-gated, §11.4.95): `issues` context STALE [issues_html/pdf/summary/summary_html/pdf] — two reasons:
(a) generate_issues_summary.sh has the same output-path bug (same --stdout fix applies, deferred);
(b) the issues_md⇄items_db sync edge is a §11.4.6 CONFLICT (engine refuses silent merge; authority=items_db) — though
db-to-md is 0-diff (MD⇄DB byte-consistent), the engine won't auto-pick direction. A prior agent that forced this
CORRUPTED the tracker DB (HXC-048/043 lost — recovered). Safe resolution needs a careful db-to-md-authority baseline
refresh or an engine `--accept-authority` affordance — operator-gated to protect the tracked DB.
