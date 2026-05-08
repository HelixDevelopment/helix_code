#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ANCHOR="Article XI.*11.9"
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
      if [ -f "$ROOT/$sm/$f" ] && grep -q "$ANCHOR" "$ROOT/$sm/$f" 2>/dev/null; then
        echo "PASS: $sm/$f"
      elif [ -f "$ROOT/$sm/$f" ]; then
        echo "FAIL: $sm/$f — no anchor"; FAILURES=$((FAILURES+1))
      else
        echo "FAIL: $sm/$f — file missing"; FAILURES=$((FAILURES+1))
      fi
    done
  done < "$OWNED_FILE"
else
  echo "SKIP: $OWNED_FILE not found (run P1-T01 first)"
fi

# 3. Third-party submodules (require .helix-governance marker)
echo ""
echo "--- Third-party submodules ---"
if [ -f "$THIRD_PARTY_FILE" ]; then
  while IFS=' |' read -r sm rest; do
    [ -z "$sm" ] && continue
    [ ! -d "$ROOT/$sm" ] && echo "SKIP: $sm (not initialized)" && continue
    if [ -f "$ROOT/$sm/.helix-governance" ]; then
      echo "PASS: $sm/.helix-governance"
    else
      echo "FAIL: $sm/.helix-governance — missing"; FAILURES=$((FAILURES+1))
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
