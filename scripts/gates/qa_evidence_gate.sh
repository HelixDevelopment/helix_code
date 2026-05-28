#!/usr/bin/env bash
# scripts/gates/qa_evidence_gate.sh
#
# §11.4.83 docs/qa/ end-user-evidence RELEASE GATE wrapper.
#
# Purpose:
#   Invokes scripts/verify_qa_evidence.sh in ENFORCING mode, scoped to the
#   convention baseline, and propagates its exit code. This is the
#   release-gate seam for §11.4.83 operative rule (5): "release gates MUST
#   refuse to tag a version that has any feature-shipping commit without its
#   matching docs/qa/<run-id>/ directory." The operator authorised promotion
#   to a blocking release gate on 2026-05-28 (HXC-019).
#
#   RELEASE-GATE ONLY. This wrapper is invoked by scripts/release-gate-test.sh.
#   It is deliberately NOT wired into any pre-commit / pre-push git hook —
#   the §11.4.83 mandate scopes enforcement to "release gates".
#
# Baseline:
#   The docs/qa convention was introduced by the commit that ADDED
#   docs/qa/README.md:
#       ed84f90e  2026-05-28  feat(qa): establish docs/qa/ end-user evidence
#                             tree + advisory gate (§11.4.83) (HXC-019)
#   Commits at-or-before the baseline predate the convention and are exempt.
#   Override the baseline via the QA_EVIDENCE_BASELINE env var (e.g. for a
#   future re-baseline) or pass an explicit ref as $1.
#
# Usage:
#   scripts/gates/qa_evidence_gate.sh [<baseline-ref-or-date>]
#       Default baseline: $QA_EVIDENCE_BASELINE or the hardcoded SHA below.
#
# Exit codes:
#   0  PASS — no in-range feature commit lacks docs/qa evidence
#   1  FAIL — at least one violation (release gate blocks)
#   2  misuse / scanner could not run
#
# Cross-references:
#   scripts/verify_qa_evidence.sh, scripts/release-gate-test.sh,
#   docs/qa/README.md, constitution Constitution.md §11.4.83.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT" || exit 2

# Convention baseline (commit that added docs/qa/README.md). Overridable.
DEFAULT_BASELINE="ed84f90e7471fb683f7779bac80cdfd169620159"
BASELINE="${1:-${QA_EVIDENCE_BASELINE:-$DEFAULT_BASELINE}}"

SCANNER="scripts/verify_qa_evidence.sh"
if [ ! -x "$SCANNER" ] && [ ! -f "$SCANNER" ]; then
	echo "qa_evidence_gate.sh: $SCANNER not found" >&2
	exit 2
fi

echo "=== §11.4.83 docs/qa/ evidence release gate (baseline: ${BASELINE}) ==="
bash "$SCANNER" --enforce --since "$BASELINE"
rc=$?

case "$rc" in
	0) echo "qa_evidence_gate.sh: PASS (release gate green)" ;;
	1) echo "qa_evidence_gate.sh: FAIL (§11.4.83 release gate red — see above)" >&2 ;;
	*) echo "qa_evidence_gate.sh: ERROR (scanner exit $rc)" >&2 ;;
esac

exit "$rc"
