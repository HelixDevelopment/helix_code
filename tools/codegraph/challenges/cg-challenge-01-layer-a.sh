#!/usr/bin/env bash
# =============================================================================
# Challenge:   CG-CHALLENGE-01 — Layer A: CodeGraph works on the real repo
# Bank:        codegraph (helix_qa bank: codegraph-integration)
# Task ID:     CG10 (Phase C — anti-bluff verification).
# Authority:   HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035,
#              Article XI §11.9 (every PASS carries captured runtime evidence).
#
# WHAT IT PROVES:
#   CodeGraph genuinely indexed the real HelixCode repository and answers
#   real queries. It runs the full Layer-A proof (tools/codegraph/verify.sh):
#     - `codegraph status` reports non-zero files/nodes/edges,
#     - `codegraph query Provider` returns real HelixCode Go symbols,
#     - `codegraph context` references real repo files.
#
# ANTI-BLUFF:  This Challenge FAILS LOUDLY if codegraph is broken — a
#              zero-node graph, an empty query result, or a simulated/
#              placeholder result value all produce a non-zero exit. A PASS
#              here is impossible unless codegraph really works.
#
# Exit codes:  0 = PASS (real graph data returned, evidence captured)
#              1 = FAIL (codegraph empty/broken/error)
#              2 = preconditions unmet
# Usage:       tools/codegraph/challenges/cg-challenge-01-layer-a.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
EVID_DIR="$REPO_ROOT/docs/research/codegraph/evidence/phase-c"

echo "### CG-CHALLENGE-01 — Layer A (codegraph works on the real HelixCode repo)"
exec "$REPO_ROOT/tools/codegraph/verify.sh" "$EVID_DIR"
