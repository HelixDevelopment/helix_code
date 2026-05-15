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

# 2. Owned-by-us submodules (require CONSTITUTION.md, CLAUDE.md, AGENTS.md with anchor)
echo ""
echo "--- Owned-by-us submodules ---"
if [ -f "$OWNED_FILE" ]; then
  while IFS=' |' read -r sm rest; do
    [ -z "$sm" ] && continue
    [ ! -d "$ROOT/$sm" ] && echo "SKIP: $sm (not initialized)" && continue
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
      if [ -z "$missing_anchors" ]; then
        echo "PASS: $sm/$f (§11.9 + CONST-047/048/049/050/051/052)"
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
