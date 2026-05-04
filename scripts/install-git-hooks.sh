#!/usr/bin/env bash
# scripts/install-git-hooks.sh
# Idempotent installer for HelixCode git hooks.
#
# Per CONST-042: the pre-push hook is a courtesy gate; the constitutional
# clause is the actual contract.
#
# Usage: ./scripts/install-git-hooks.sh
#
# Idempotent: running this script multiple times is safe.
# A hook is only (re-)installed when the source differs from the installed copy.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

HOOKS_SRC="scripts/git-hooks"
HOOKS_DST=".git/hooks"

mkdir -p "$HOOKS_DST"

installed=0
for src in "$HOOKS_SRC"/*; do
  [ -f "$src" ] || continue
  name=$(basename "$src")
  dst="$HOOKS_DST/$name"

  # If existing hook is byte-for-byte identical, skip.
  if [ -f "$dst" ] && cmp -s "$src" "$dst"; then
    continue
  fi

  cp -p "$src" "$dst"
  chmod +x "$dst"
  installed=$((installed + 1))
  echo "  installed: $dst"
done

echo "OK: $installed hook(s) installed/updated under $HOOKS_DST"
