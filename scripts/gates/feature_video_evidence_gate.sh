#!/usr/bin/env bash
# scripts/gates/feature_video_evidence_gate.sh
#
# §11.4.153 / §11.4.86 — Feature-Status video-evidence DURABILITY gate.
#
# Purpose
#   The HXC-107 audit (docs/qa/HXC-107_ledger_audit.md) found docs/features/Status.md
#   marking rows `confirmed` while citing recordings from the ROTATABLE, git-ignored
#   raw corpus (/Volumes/T7/Downloads/Recordings/, §11.4.128). The §11.4.154
#   fresh-corpus rotation deleted those files → every confirmation was orphaned
#   (a §11.4.153 PASS-bluff: "confirmed" with a missing evidence path).
#
#   This gate makes that bluff MECHANICALLY IMPOSSIBLE:
#     1. It walks every `confirmed` row of docs/features/Status.md, extracts each
#        DURABLE evidence path cited in the `📹 Video` column, and FAILs if any
#        cited path does not exist on disk.
#     2. It rejects citations that point at the rotatable raw corpus
#        (/Volumes/T7/Downloads/Recordings or any path NOT under the repo) — a
#        confirmed row MUST cite committed §11.4.83 evidence (docs/qa/... or
#        docs/features/...), never the rotatable corpus.
#     3. It maintains a §11.4.86 drift-proof fingerprint sidecar
#        (docs/features/.video_evidence_roster.sha256) = sha256 of the sorted
#        cited-evidence-path roster. If the live roster differs from the sidecar
#        (a citation added/removed/renamed), the gate FAILs until the sidecar is
#        refreshed (`--update`) — so a silent change to the confirmed-evidence set
#        cannot pass unnoticed.
#
# Usage
#   feature_video_evidence_gate.sh            # verify (gate mode; exit 1 on any failure)
#   feature_video_evidence_gate.sh --update   # refresh the fingerprint sidecar after an intended change
#
# Fingerprint is sha256 of the newline-joined sorted UNIQUE cited durable paths.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"; cd "$root"
STATUS="docs/features/Status.md"
SIDECAR="docs/features/.video_evidence_roster.sha256"
MODE="${1:-verify}"

sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }

[ -f "$STATUS" ] || { echo "FAIL: $STATUS not found"; exit 1; }

# Extract durable evidence paths cited inside `📹 Video` cells of `confirmed` rows.
# A confirmed row is a table line ending in `| confirmed |`. The `📹 Video` cell is
# the 9th pipe-delimited field. We pull every docs/qa/... or docs/features/... token
# (file or dir path) cited anywhere in that cell.
mapfile -t CITED < <(
  awk -F'|' '/\| confirmed \|$/ {
    # field layout: "" Area Component Feature Dev Wired Real-use Tests V&V Video Analysis Origin Overall ""
    print $10
  }' "$STATUS" \
  | grep -oE 'docs/(qa|features)/[A-Za-z0-9._/-]+' \
  | sort -u
)

# Reject rotatable-corpus / non-repo citations in any confirmed row's Video cell.
ROTATABLE=0
while IFS= read -r badcell; do
  echo "FAIL (rotatable/non-durable citation): $badcell"
  ROTATABLE=1
done < <(
  awk -F'|' '/\| confirmed \|$/ {print $10}' "$STATUS" \
  | grep -oE '/Volumes/T7/Downloads/Recordings[A-Za-z0-9._/-]*' || true
)

MISSING=0
for p in "${CITED[@]:-}"; do
  [ -n "$p" ] || continue
  if [ ! -e "$p" ]; then
    echo "FAIL (cited durable evidence MISSING): $p"
    MISSING=1
  fi
done

# §11.4.86 drift-proof fingerprint over the sorted cited roster.
LIVE_FP="$(printf '%s\n' "${CITED[@]:-}" | sha256)"

if [ "$MODE" = "--update" ]; then
  printf '%s\n' "$LIVE_FP" > "$SIDECAR"
  echo "UPDATED $SIDECAR = $LIVE_FP  (${#CITED[@]} cited durable paths)"
  exit 0
fi

DRIFT=0
if [ ! -f "$SIDECAR" ]; then
  echo "FAIL (no fingerprint sidecar — run with --update to establish): $SIDECAR"
  DRIFT=1
else
  STORED_FP="$(tr -d '[:space:]' < "$SIDECAR")"
  if [ "$STORED_FP" != "$LIVE_FP" ]; then
    echo "FAIL (§11.4.86 roster drift): sidecar=$STORED_FP live=$LIVE_FP"
    echo "       cited-evidence roster changed; re-run with --update if intended."
    DRIFT=1
  fi
fi

if [ "$ROTATABLE" -ne 0 ] || [ "$MISSING" -ne 0 ] || [ "$DRIFT" -ne 0 ]; then
  echo "GATE FAIL: feature_video_evidence_gate (§11.4.153/§11.4.86)"
  exit 1
fi

echo "GATE PASS: ${#CITED[@]} cited durable evidence paths all exist; no rotatable-corpus citation; fingerprint matches ($LIVE_FP)"
exit 0
