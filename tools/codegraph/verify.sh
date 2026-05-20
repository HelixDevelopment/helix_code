#!/usr/bin/env bash
# =============================================================================
# Script:   tools/codegraph/verify.sh
# Purpose:  Anti-bluff end-to-end proof that CodeGraph genuinely works against
#           the real HelixCode repository (CONST-035 / Article XI §11.9).
# Task ID:  CG3 scaffold — Phase C task CG10 fills the real verification body.
# Authority: Cascaded from HelixCode root CLAUDE.md / CONSTITUTION.md.
#
# STATUS:   STUB. This file is a placeholder created in Phase A (CG3) so the
#           scaffold is complete. Phase C task CG10 replaces this body with the
#           real CG-CHALLENGE-01 layer-A verification:
#             - `codegraph status .` must report initialized:true AND
#               files>0 AND nodes>0 AND edges>0 (non-zero counts).
#             - `codegraph query Provider` must return real HelixCode symbols.
#             - `codegraph context "add a new LLM provider"` must reference
#               real HelixCode files.
#             - assert output contains no `simulated` / `placeholder` / `TODO`.
#
# Until CG10 lands, this stub exits non-zero so no caller can mistake the
# scaffold for a passing verification. An honest, loud "not implemented yet"
# is required by the anti-bluff mandate — a stub that exits 0 would be a bluff.
#
# Exit codes: 2 = verification not yet implemented (Phase C / CG10 pending).
# Usage:      tools/codegraph/verify.sh
# =============================================================================
set -euo pipefail

echo "verify.sh: NOT IMPLEMENTED YET — Phase C task CG10 fills this." >&2
echo "verify.sh: this stub exits non-zero by design (anti-bluff: a stub" >&2
echo "verify.sh: that exits 0 would falsely claim CodeGraph is verified)." >&2
exit 2
