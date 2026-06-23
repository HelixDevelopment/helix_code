#!/usr/bin/env bash
# ============================================================================
# Governance Propagation Script — RETIRED (no-op)
# ============================================================================
#
# RETIRED 2026-06-23 per the operator's thin-inheritance governance-model
# decision (CONST-059 / CONST-051(B) / §11.4.28).
#
# WHAT THIS SCRIPT USED TO DO (and why it is now WRONG):
#   It copied the five repo-root governance files (CONSTITUTION.md, CLAUDE.md,
#   AGENTS.md, QWEN.md, GEMINI.md) verbatim INTO every owned submodule, then
#   committed + pushed them. That is "inline restatement" propagation — the
#   OLD model. Under the thin-inheritance model it is a CONST-051(B)/§11.4.28
#   VIOLATION: it re-injects project-specific inline content into submodules
#   that are supposed to be project-agnostic thin-inheritance stubs which only
#   POINT to the canonical constitution (via a `## INHERITED FROM` heading +
#   the `find_constitution.sh` resolver, never a hardcoded path).
#
# THE CURRENT MODEL (what replaces this script):
#   - Owned-submodule governance carriers are THIN-INHERITANCE STUBS. The
#     universal §11.9 / CONST-047..059 / covenant-114 anchors live ONLY in the
#     constitution submodule + the meta-root carriers; submodules inherit them
#     by reference. `scripts/verify-governance-cascade.sh` (section 2) asserts
#     the inheritance pointer is present — it no longer demands inline anchors.
#   - The canonical thin-stub CONVERSION of submodule carriers is owned by the
#     governance-migration mechanism (the process that produced e.g. the
#     `containers` thin stub) — NOT by this script. Re-implementing a converter
#     here would risk diverging from that canonical format, so this script is
#     deliberately retired rather than rewritten.
#
# This file is kept (not deleted) because `./scripts/propagate-governance.sh`
# is a documented command; it now no-ops with this notice so a stale invocation
# can never re-couple a submodule.
# ============================================================================

set -euo pipefail

cat <<'NOTICE'
=== propagate-governance.sh is RETIRED (no-op) ===

This script previously re-inlined the root governance files into every owned
submodule. That contradicts the thin-inheritance model (operator decision
2026-06-23; CONST-059 / CONST-051(B) / §11.4.28): owned-submodule carriers are
project-agnostic thin-inheritance STUBS that point to the constitution via
find_constitution.sh, NOT inline copies of the gov files.

  • Governance is verified, not propagated-by-copy:
      bash scripts/verify-governance-cascade.sh
    (section 2 asserts each owned-submodule carrier has the inheritance pointer)

  • The canonical thin-stub conversion of submodule carriers is owned by the
    governance-migration mechanism, not this script.

No files were copied, committed, or pushed. This invocation did nothing.
NOTICE

exit 0
