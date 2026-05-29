#!/usr/bin/env bash
# scripts/gates/summary_sync_gate.sh — CM-ISSUES-SUMMARY-SYNC + CM-FIXED-SUMMARY-SYNC
#
# §11.4.12 + §11.4.91 + §11.4.86 freshness gate. The summary docs
# (docs/Issues_Summary.md, docs/Fixed_Summary.md) MUST be a mechanical
# projection of their source trackers (docs/Issues.md, docs/Fixed.md). They
# were historically HAND-MAINTAINED, which let them drift (the open-count went
# stale by ~100 closures). This gate runs the generators in --check mode: if a
# fresh regeneration would differ from what is committed, the summary is stale
# and the build FAILS until `scripts/generate_{issues,fixed}_summary.sh` is re-run.
#
# Companion to summary_clarity_gate.sh (§11.4.91 anti-pattern content) and
# workable_items_sync_gate.sh (§11.4.93/95 md↔db). Clarity checks the rows are
# meaningful; THIS checks they are fresh.
#
# Exit 0 = both summaries in sync; non-zero = at least one stale.
set -uo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GEN_ISSUES="$ROOT/scripts/generate_issues_summary.sh"
GEN_FIXED="$ROOT/scripts/generate_fixed_summary.sh"

rc=0
for gen in "$GEN_ISSUES" "$GEN_FIXED"; do
    if [[ ! -x "$gen" ]]; then
        echo "CM-SUMMARY-SYNC: FAIL — generator missing/not executable: $gen" >&2
        rc=1; continue
    fi
    if ! "$gen" --check; then
        rc=1
    fi
done

if [[ "$rc" -eq 0 ]]; then
    echo "CM-SUMMARY-SYNC: PASS — Issues_Summary.md + Fixed_Summary.md fresh vs trackers"
fi
exit "$rc"
