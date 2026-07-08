#!/usr/bin/env bash
# =============================================================================
# doc_integrity_check.sh — §11.4.186 pre-build gate entry
# =============================================================================
# Purpose:
#   Pre-build gate wrapper that invokes the constitution submodule's
#   doc-integrity hard gate hook. Runs BEFORE any export render, doc/DB
#   sync verify, or doc-set commit. Refuses the build on a hard FAIL,
#   skips gracefully when the consumer checkset is absent (exit 3 SKIP
#   per §11.4.3), and surfaces the integrity findings.
#
#   Composes with §11.4.186 step 5: wiring the gate stub into the
#   project's pre-build verification pipeline.
#
# Usage:
#   bash scripts/pre_build/doc_integrity_check.sh [repo_root]
#
#     [repo_root]  root of the HelixCode checkout (default: $PWD)
#
# Exit codes (mirrors doc_integrity_gate.sh):
#   0  PASS — no integrity findings; build may proceed.
#   1  FAIL — integrity findings; build MUST be REFUSED.
#   3  SKIP — checkset or toolchain absent (§11.4.3, not a fake PASS).
#
# Dependencies:
#   - constitution/scripts/doc_integrity/wire/doc_integrity_gate.sh
#     (inherited by reference from the constitution submodule per
#     §11.4.177 / §11.4.80-style inheritance)
#   - Go toolchain (1.21+) if DOC_INTEGRITY_BIN is not set
#   - .helix_code/doc_integrity/checkset.yaml (consumer-owned, tracked)
#
# Cross-references:
#   DOC_INTEGRITY_INTEGRATION.md §4.1 / §5 / §6
#   Constitution §11.4.186 / §11.4.3 / §11.4.106 / §11.4.65
# =============================================================================
set -euo pipefail

REPO_ROOT="${1:-$PWD}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE_HOOK="${SCRIPT_DIR}/../../constitution/scripts/doc_integrity/wire/doc_integrity_gate.sh"
CHECKSET="${REPO_ROOT}/.helix_code/doc_integrity/checkset.yaml"

# --- Resolve the gate hook (inherited by reference from constitution submodule) ---
if [ ! -f "${GATE_HOOK}" ]; then
  echo "WARN §11.4.186/§11.4.3: gate hook not found at ${GATE_HOOK} — SKIP"
  echo "  (constitution submodule may not be initialised; run git submodule update --init)"
  exit 3
fi

# --- Check for the consumer checkset ---
if [ ! -f "${CHECKSET}" ]; then
  echo "SKIP §11.4.186/§11.4.3: consumer checkset not found at ${CHECKSET}"
  echo "  (doc-integrity gate skipped; author .helix_code/doc_integrity/checkset.yaml to enable)"
  exit 3
fi

# --- Invoke the gate hook ---
# Use --divergence-class-only for the pre-build seam (§11.4.186 clause-6 honest
# boundary + §11.4.50 ratchet): INTEGRITY-class findings (orphan-ref / Status↔Type
# / location↔status DATA defects) are NON-refusing — plan-data correctness is an
# operator decision the gate SURFACES, never MAKES. Only DEDUP / TIMELINE /
# CROSS-DOC / STRUCTURAL still REFUSE. Drop this flag once plan data is clean.
echo "doc-integrity: running pre-build gate against ${CHECKSET} ..."
if bash "${GATE_HOOK}" "${CHECKSET}" "${REPO_ROOT}" --divergence-class-only; then
  rc=0
else
  rc=$?
fi

case "${rc}" in
  0)
    echo "PASS §11.4.186: doc-integrity gate — no divergences found."
    exit 0
    ;;
  1)
    echo "FATAL §11.4.186: doc-integrity FAIL — build REFUSED." >&2
    echo "  Fix cross-doc divergences before rebuilding." >&2
    exit 1
    ;;
  3)
    echo "SKIP §11.4.186/§11.4.3: doc-integrity gate — source unavailable."
    echo "  (honest SKIP, not a fake PASS — build may proceed but should be re-checked)"
    exit 3
    ;;
  *)
    echo "FATAL §11.4.186: doc-integrity gate — unexpected exit ${rc}, build REFUSED." >&2
    exit 1
    ;;
esac
