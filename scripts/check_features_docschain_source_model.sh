#!/usr/bin/env bash
# check_features_docschain_source_model.sh — §11.4.135 standing regression guard
# for the docs_chain Status.md-clobber defect (root-caused + fixed 2026-06-16 via
# §11.4.102 systematic-debugging).
#
# DEFECT (reproduced): .docs_chain/contexts/features.yaml modelled the HAND-AUTHORED
# docs/features/Status.md as a DERIVED `kind: status` target with an
# `aggregate-inventory` transform (scripts/generate_features_status.sh). That
# generator emits a LOSSY rev1 aggregation (859 lines), so `docs_chain sync`
# OVERWROTE the hand-authored rev7 ledger (1003 lines) — observed rev7->rev1,
# 168 deletions. Root cause = derived-vs-source mis-modelling.
#
# FIX: Status.md + Status_Summary.md are `kind: markdown` SOURCES; the
# aggregate-inventory + gen-features-summary edges are dropped (only export edges
# remain), matching the proven governance.yaml/issues.yaml pattern.
#
# This guard FAILS if the clobber-model ever returns. Deterministic, no network.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"; cd "$root"
# Optional path arg lets the §1.1 paired-mutation test point at a mutated copy.
f="${1:-.docs_chain/contexts/features.yaml}"
fail() { echo "FAIL (§11.4.135 docs_chain clobber-model regression): $1"; exit 1; }

[ -f "$f" ] || fail "$f missing"

# 1. Status.md + Status_Summary.md MUST be kind: markdown SOURCES (not status/derived).
grep -Eq 'status_md:[[:space:]]*\{[[:space:]]*kind:[[:space:]]*markdown' "$f" \
  || fail "status_md is not 'kind: markdown' — Status.md modelled as derived again (clobber risk)"
grep -Eq 'status_summary:[[:space:]]*\{[[:space:]]*kind:[[:space:]]*(markdown|status_summary)' "$f" \
  && grep -Eq 'status_summary:[[:space:]]*\{[[:space:]]*kind:[[:space:]]*markdown' "$f" \
  || fail "status_summary is not 'kind: markdown' — Status_Summary.md modelled as derived again"

# 2. Status.md / Status_Summary.md MUST NOT be a derive-from EDGE TARGET (the
#    clobber model wired `..., to: status_md, ...` / `..., to: status_summary, ...`).
#    Checking the edge-target form `, to: status_md` is comment-safe (the FIX
#    description above mentions the generator names in prose but never as an edge).
grep -Eq ',[[:space:]]*to:[[:space:]]*status_md\b' "$f" \
  && fail "an edge targets status_md (Status.md modelled as a derived target again → clobber)"
grep -Eq ',[[:space:]]*to:[[:space:]]*status_summary\b' "$f" \
  && fail "an edge targets status_summary (Status_Summary.md modelled as derived again → clobber)"

echo "PASS: features.yaml models Status.md/Status_Summary.md as hand-authored markdown sources; no clobbering derive-from edge targets them."
