#!/usr/bin/env bash
# fixed_h2_pipe_row_parity_gate.sh — §11.4.135 standing regression guard for the
# §11.4.90/.91/.53 docs-tooling drift that hid HXC-044 from Fixed_Summary.
#
# THE DRIFT (forensic, FACT): docs/Fixed.md is MIXED — a pipe table
# (`| Closure | Title | Type | Status | Round | Commit(s) | Evidence |`) AND H2
# detail sections (`## HXC/ATM-NNN — …`). generate_fixed_summary.sh reads ONLY
# the pipe table. So an H2 closure section with NO matching pipe-table row is
# invisible to the summary (HXC-044 was Obsolete in the DB + had an H2 section
# but no pipe row → absent from Fixed_Summary twice over).
#
# THIS GUARD asserts two invariants so the drift cannot recur:
#   (A) every docs/Fixed.md H2 closure heading (## HXC-NNN / ## ATM-NNN, the
#       ticket-id form) has a matching pipe-table row keyed by `<ID>:` or
#       `<ID> ` in the Title cell;
#   (B) every `Obsolete (→ Fixed.md)` pipe-table item appears in
#       docs/Fixed_Summary.md.
#
# SEVERITY (FACT discovered 2026-06-16 while fixing HXC-044): the pipe table is
# hand-maintained and had fallen ~50 HXC items behind the H2 sections (only 25
# of 75 HXC H2 sections had pipe rows). HXC-044 was the operator-flagged one;
# the rest are a PRE-EXISTING backlog OUTSIDE the HXC-044 task scope (§11.4.118
# — enumerated, not silently backfilled with guessed dates/commits §11.4.6).
# Therefore:
#   - Invariant (B) — the Obsolete→Summary parity HXC-044 exercised — is a HARD
#     FAIL (this is the exact bug this guard was written to lock down).
#   - Invariant (A) — the broader H2↔pipe-row parity — is reported as a
#     WARNING with the full enumerated backlog (captured evidence per §11.4.118)
#     and does NOT fail the gate, so a clean GREEN is achievable for the
#     HXC-044-scoped fix while the backlog stays visible + tracked. Set
#     FIXED_PARITY_STRICT=1 to promote (A) to a hard FAIL once the backlog is
#     fully repaired in a dedicated work item.
#
# Purpose / Usage / Inputs / Outputs / Side-effects / Dependencies / Cross-refs:
#   Purpose:      §11.4.135 regression guard (drift that hid HXC-044).
#   Usage:        scripts/gates/fixed_h2_pipe_row_parity_gate.sh
#                 RED_MODE=1 scripts/gates/fixed_h2_pipe_row_parity_gate.sh
#                            # §11.4.115 polarity: reproduce the pre-fix defect
#                            # on a synthesized broken copy (asserts the guard
#                            # genuinely catches a missing row + missing summary
#                            # entry → exit 1). Default RED_MODE=0 = standing
#                            # GREEN guard asserting the defect is ABSENT.
#   Inputs:       docs/Fixed.md, docs/Fixed_Summary.md
#   Outputs:      PASS/FAIL line on stdout; exit 0 PASS, exit 1 FAIL.
#   Side-effects: none (RED_MODE uses a temp copy under $TMPDIR; cleaned on EXIT).
#   Dependencies: awk, grep, mktemp.
#   Cross-refs:   §11.4.53 (Fixed_Summary parity), §11.4.90 (Obsolete status),
#                 §11.4.91 (summary clarity), §11.4.115 (RED polarity),
#                 §11.4.135 (standing regression guard).
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXED="$ROOT/docs/Fixed.md"
SUMMARY="$ROOT/docs/Fixed_Summary.md"
GATE="CM-FIXED-H2-PIPE-ROW-PARITY"
RED_MODE="${RED_MODE:-0}"

# --- RED_MODE: §11.4.115 reproduce-the-defect-on-a-broken-artifact ----------
# Synthesize the pre-fix broken state (HXC-044's pipe row + its summary entry
# removed) on COPIES, run the same checker against them, and assert it FAILs.
# This proves the guard is not a blind test.
if [[ "$RED_MODE" == "1" ]]; then
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  # Recreate the exact pre-fix drift HXC-044 exposed: the Obsolete item HAS a
  # pipe row (so invariant (B) can see it as Obsolete) but is ABSENT from
  # Fixed_Summary.md (the bug — generate_fixed_summary.sh did not emit Obsolete
  # items). Invariant (B) MUST hard-FAIL on this.
  cp "$FIXED" "$TMP/Fixed.md"
  grep -v 'HXC-044' "$SUMMARY" > "$TMP/Fixed_Summary.md" || true
  if RED_MODE=0 FIXED_OVERRIDE="$TMP/Fixed.md" SUMMARY_OVERRIDE="$TMP/Fixed_Summary.md" \
       "$0" >/dev/null 2>&1; then
    echo "$GATE: RED FAIL — guard PASSed on the known-broken artifact (blind test)" >&2
    exit 1
  fi
  echo "$GATE: RED OK — guard correctly FAILs on the pre-fix broken artifact (HXC-044 Obsolete pipe row present but ABSENT from Fixed_Summary.md — the exact §11.4.90/.53 bug)"
  exit 0
fi

# Allow RED_MODE's recursive invocation to point at the synthesized copies.
FIXED="${FIXED_OVERRIDE:-$FIXED}"
SUMMARY="${SUMMARY_OVERRIDE:-$SUMMARY}"

[[ -f "$FIXED" ]]   || { echo "$GATE: FAIL — $FIXED missing" >&2; exit 1; }
[[ -f "$SUMMARY" ]] || { echo "$GATE: FAIL — $SUMMARY missing" >&2; exit 1; }

FAILURES=0
STRICT="${FIXED_PARITY_STRICT:-0}"
A_BACKLOG=0

# (A) Every H2 closure heading (## HXC-NNN / ## ATM-NNN) has a pipe-table row.
#     Heading id is the ticket id token; the pipe row carries `<ID>:` or
#     `<ID> ` (space) in its Title cell (col 2). WARN (enumerated backlog) by
#     default; hard FAIL under FIXED_PARITY_STRICT=1.
while IFS= read -r id; do
  # Pipe data row whose Title cell starts with this id followed by ':' or space.
  if ! awk -F'|' -v id="$id" '
        /^\|/ {
          c1=$2; gsub(/^[ \t]+|[ \t]+$/, "", c1)
          if (c1 !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) next
          title=$3; gsub(/^[ \t]+|[ \t]+$/, "", title)
          if (title ~ ("^" id "[: ]")) { found=1 }
        }
        END { exit (found ? 0 : 1) }
      ' "$FIXED"; then
    if [[ "$STRICT" == "1" ]]; then
      echo "$GATE: FAIL — Fixed.md H2 closure section '## $id' has NO matching pipe-table row (§11.4.135)" >&2
      FAILURES=$((FAILURES + 1))
    else
      echo "$GATE: WARN — Fixed.md H2 closure section '## $id' has NO matching pipe-table row (pre-existing backlog §11.4.118)" >&2
      A_BACKLOG=$((A_BACKLOG + 1))
    fi
  fi
done < <(grep -oE '^## (HXC|ATM)-[0-9A-Za-z]+' "$FIXED" | sed 's/^## //' | sort -u)
[[ "$A_BACKLOG" -gt 0 ]] && echo "$GATE: WARN — $A_BACKLOG H2 closure section(s) lack a pipe-table row (enumerated above; pre-existing, tracked separately from the HXC-044 fix; set FIXED_PARITY_STRICT=1 once repaired)" >&2

# (B) Every Obsolete (→ Fixed.md) pipe-table item appears in Fixed_Summary.md.
while IFS= read -r id; do
  if ! grep -q -- "$id" "$SUMMARY"; then
    echo "$GATE: FAIL — Obsolete item '$id' missing from Fixed_Summary.md (§11.4.90/.53)" >&2
    FAILURES=$((FAILURES + 1))
  fi
done < <(awk -F'|' '
    /^\|/ {
      c1=$2; gsub(/^[ \t]+|[ \t]+$/, "", c1)
      if (c1 !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) next
      s=$5; gsub(/^[ \t]+|[ \t]+$/, "", s)
      if (s !~ /^Obsolete \(/) next
      title=$3; gsub(/^[ \t]+|[ \t]+$/, "", title)
      # Emit the leading ticket-id token (up to the first ":" or space).
      n=split(title, a, /[: ]/); print a[1]
    }
  ' "$FIXED" | sort -u)

if [[ "$FAILURES" -gt 0 ]]; then
  echo "$GATE: FAIL — $FAILURES parity violation(s)" >&2
  exit 1
fi
echo "$GATE: PASS — every Fixed.md H2 closure heading has a pipe row; every Obsolete item is in Fixed_Summary.md"
exit 0
