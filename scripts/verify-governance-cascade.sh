#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ANCHOR="Article XI.*11.9"
CONST047_ANCHOR="CONST-047"
CONST048_ANCHOR="CONST-048"
CONST049_ANCHOR="CONST-049"
CONST050_ANCHOR="CONST-050"
CONST051_ANCHOR="CONST-051"
CONST052_ANCHOR="CONST-052"
CONST053_ANCHOR="CONST-053"
CONST054_ANCHOR="CONST-054"
CONST055_ANCHOR="CONST-055"
CONST056_ANCHOR="CONST-056"
CONST057_ANCHOR="CONST-057"
CONST058_ANCHOR="CONST-058"
CONST059_ANCHOR="CONST-059"

# Covenant-114 propagation anchors (§11.4.69, §11.4.75..§11.4.97) — see §11.4.32
# / CONST-055. Literal form = "## §11.4.NN —" (the H2 HEADING marker). The
# leading "## " is MANDATORY: it ensures we match the anchor's own HEADING, not
# a cross-reference to §11.4.NN inside another anchor's body (e.g. the §11.4.93
# block body cites "§11.4.95 —" — without the "## " prefix that would falsely
# satisfy the §11.4.95 check). The trailing " — " also guards prefix collisions
# (§11.4.8 vs §11.4.84/87, §11.4.9 vs §11.4.90..97, §11.4.7 vs §11.4.75..78).
# Grep is fixed-string (-F) so § (U+00A7) and — (U+2014) match literally.
COVENANT114_ANCHORS=(
  "## §11.4.69 —" "## §11.4.75 —" "## §11.4.76 —" "## §11.4.77 —" "## §11.4.78 —"
  "## §11.4.79 —" "## §11.4.80 —" "## §11.4.81 —" "## §11.4.82 —" "## §11.4.83 —"
  "## §11.4.84 —" "## §11.4.85 —" "## §11.4.86 —" "## §11.4.87 —" "## §11.4.88 —"
  "## §11.4.89 —" "## §11.4.90 —" "## §11.4.91 —" "## §11.4.92 —" "## §11.4.93 —"
  "## §11.4.94 —" "## §11.4.95 —" "## §11.4.96 —" "## §11.4.97 —"
)

# Map "## §11.4.NN —" -> CM-COVENANT-114-NN-PROPAGATION (exact gate ID in FAILs).
covenant114_gate_id() {
  local lit="$1" nn
  nn="${lit##* §11.4.}"; nn="${nn%% —}"
  printf 'CM-COVENANT-114-%s-PROPAGATION' "$nn"
}

# Append every MISSING covenant-114 anchor for one file to $missing_anchors.
check_covenant114_anchors() {
  local f="$1" lit gid
  for lit in "${COVENANT114_ANCHORS[@]}"; do
    if ! grep -qF -- "$lit" "$f" 2>/dev/null; then
      gid="$(covenant114_gate_id "$lit")"
      missing_anchors+=" ${gid}(${lit% —})"
    fi
  done
}

FAILURES=0
OWNED_FILE="$ROOT/docs/improvements/submodule_owned.txt"
THIRD_PARTY_FILE="$ROOT/docs/improvements/submodule_third_party.txt"

echo "=== Governance Cascade Verification ==="
echo "Repo: $ROOT"
echo ""

# 1. Root governance files
echo "--- Root governance ---"
for f in CONSTITUTION.md AGENTS.md; do
  if grep -q "$ANCHOR" "$ROOT/$f" 2>/dev/null; then
    echo "PASS: root/$f"
  else
    echo "FAIL: root/$f"; FAILURES=$((FAILURES+1))
  fi
done

# 1b. Root govfiles — covenant-114 propagation (§11.4.69, §11.4.75..97).
#     All 5 consumer-extension govfiles must carry every cascaded anchor.
echo ""
echo "--- Root govfiles — covenant-114 propagation (§11.4.69, §11.4.75..97) ---"
for f in CLAUDE.md AGENTS.md QWEN.md CRUSH.md CONSTITUTION.md; do
  if [ ! -f "$ROOT/$f" ]; then
    echo "FAIL: root/$f — file missing (covenant-114 scope)"; FAILURES=$((FAILURES+1))
    continue
  fi
  missing_anchors=""
  check_covenant114_anchors "$ROOT/$f"
  if [ -z "$missing_anchors" ]; then
    echo "PASS: root/$f (all 24 covenant-114 anchors present)"
  else
    echo "FAIL: root/$f — missing:$missing_anchors"; FAILURES=$((FAILURES+1))
  fi
done

# 2. Owned-by-us submodules (require CONSTITUTION.md, CLAUDE.md, AGENTS.md with anchor)
#
# Canonical-path convention (CONST-052 snake_case + CONST-051(C) dependencies layout):
#  - Owned-by-us submodule paths in docs/improvements/submodule_owned.txt MUST be
#    the canonical on-disk paths matching the current submodule layout.
#  - Top-level submodules: lowercase snake_case (`challenges`, `containers`,
#    `github_pages_website`, `helix_agent`, `helix_qa`, `panoptic`, `security`).
#  - Nested-dependency submodules: `dependencies/<org>/<name>` per CONST-051(C).
#    Own-org `<name>` segment is lowercase snake_case per CONST-052 (§11.4.29);
#    the path column tracks the on-disk dir, the URL column keeps the (unchanged)
#    remote repo name. Only genuine third-party submodules keep upstream casing.
#  - Anti-regression: if a listed path does NOT exist on disk, the verifier now
#    FAILS loudly (was previously a silent SKIP, which masked the round-56
#    blemish where 7 stale capitalized entries hid behind "not initialized").
#    A genuinely-uninitialized submodule MUST be initialized BEFORE the cascade
#    can be verified — there is no honest middle state.
echo ""
echo "--- Owned-by-us submodules ---"
if [ -f "$OWNED_FILE" ]; then
  while IFS=' |' read -r sm rest; do
    [ -z "$sm" ] && continue
    if [ ! -d "$ROOT/$sm" ]; then
      echo "FAIL: $sm — path does not exist on disk (verifier path list out of sync; see submodule_owned.txt and CONST-052 / CONST-051(C))"
      FAILURES=$((FAILURES+1))
      continue
    fi
    for f in CONSTITUTION.md CLAUDE.md AGENTS.md; do
      if [ ! -f "$ROOT/$sm/$f" ]; then
        echo "FAIL: $sm/$f — file missing"; FAILURES=$((FAILURES+1))
        continue
      fi
      missing_anchors=""
      grep -q "$ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" §11.9"
      grep -q "$CONST047_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-047"
      grep -q "$CONST048_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-048"
      grep -q "$CONST049_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-049"
      grep -q "$CONST050_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-050"
      grep -q "$CONST051_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-051"
      grep -q "$CONST052_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-052"
      grep -q "$CONST053_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-053"
      grep -q "$CONST054_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-054"
      grep -q "$CONST055_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-055"
      grep -q "$CONST056_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-056"
      grep -q "$CONST057_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-057"
      grep -q "$CONST058_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-058"
      grep -q "$CONST059_ANCHOR" "$ROOT/$sm/$f" 2>/dev/null || missing_anchors+=" CONST-059"
      if [ -z "$missing_anchors" ]; then
        echo "PASS: $sm/$f (§11.9 + CONST-047..059)"
      else
        echo "FAIL: $sm/$f — missing:$missing_anchors"; FAILURES=$((FAILURES+1))
      fi
    done
  done < "$OWNED_FILE"
else
  echo "SKIP: $OWNED_FILE not found (run P1-T01 first)"
fi

# 3. Third-party submodules — acknowledgement is presence in
#    docs/improvements/submodule_third_party.txt (meta-repo-tracked,
#    manually curated). An optional in-submodule `.helix-governance`
#    file is still accepted as a stronger per-submodule ACK.
#
# Earlier revisions required the per-submodule marker file unconditionally,
# but that file cannot be committed to a third-party submodule's own tree
# without polluting upstream, and a meta-repo cannot track files inside a
# submodule path either — so the marker was unreachable in practice. The
# curated third-party list IS the deliberate acknowledgement.
echo ""
echo "--- Third-party submodules ---"
if [ -f "$THIRD_PARTY_FILE" ]; then
  while IFS=' |' read -r sm rest; do
    [ -z "$sm" ] && continue
    [ ! -d "$ROOT/$sm" ] && echo "SKIP: $sm (not initialized)" && continue
    if [ -f "$ROOT/$sm/.helix-governance" ]; then
      echo "PASS: $sm (in-submodule .helix-governance marker)"
    else
      echo "PASS: $sm (listed in submodule_third_party.txt)"
    fi
  done < "$THIRD_PARTY_FILE"
else
  echo "SKIP: $THIRD_PARTY_FILE not found (run P1-T01 first)"
fi

echo ""
echo "=== Result: $FAILURES failures ==="
if [ "$FAILURES" -eq 0 ]; then
  echo "PASS"
  exit 0
else
  echo "FAIL"
  exit 1
fi
