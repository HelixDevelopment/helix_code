#!/usr/bin/env bash
# scripts/install_git_hooks.sh
# §11.4.75 canonical git-hook installer (symlink-based, idempotent).
#
# Symlinks every hook body under scripts/git_hooks/ into .git/hooks/ so a
# single source of truth (the tracked scripts/git_hooks/ dir) drives the
# live hooks — editing the source updates the installed hook with no
# re-install. Idempotent: re-running is a no-op when symlinks already
# point at the right targets.
#
# This is the §11.4.75-named installer (underscore). A sibling
# scripts/install-git-hooks.sh (dash) predates it and copies instead of
# symlinks; this script is the canonical one the §11.4.75 mandate names
# (`scripts/install_git_hooks.sh`) and is symlink-based per the reference
# design. Both can coexist; prefer this one.
#
# Usage:
#   scripts/install_git_hooks.sh            # install/refresh symlinks
#   scripts/install_git_hooks.sh --dry-run  # print what WOULD be installed
#   scripts/install_git_hooks.sh --help
#
# Inputs:   scripts/git_hooks/{pre-commit,pre-push,post-commit,commit-msg}.
# Outputs:  symlinks under .git/hooks/ (unless --dry-run); status to stdout.
# Side-effects: writes symlinks into .git/hooks/ (mutates the live hook
#   workflow). NOT run automatically — operator-gated.
# Dependencies: git, ln, POSIX sh.
# Cross-references: §11.4.75; scripts/git_hooks/*; scripts/setup.sh.

set -euo pipefail

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run|-n) DRY_RUN=1 ;;
    --help|-h)
      sed -n '2,30p' "$0"
      exit 0
      ;;
    *)
      echo "unknown argument: $arg (try --help)" >&2
      exit 2
      ;;
  esac
done

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "ERROR: not inside a git repository" >&2
  exit 1
}
cd "$REPO_ROOT"

HOOKS_SRC="scripts/git_hooks"
HOOKS_DST=".git/hooks"

# Honour a non-default core.hooksPath if the operator configured one.
custom_path=$(git config --get core.hooksPath 2>/dev/null || true)
if [ -n "$custom_path" ]; then
  HOOKS_DST="$custom_path"
fi

if [ ! -d "$HOOKS_SRC" ]; then
  echo "ERROR: hook source dir $HOOKS_SRC not found" >&2
  exit 1
fi

# Only the §11.4.75 canonical hook names + the legacy pre-push are managed.
MANAGED="pre-commit pre-push post-commit commit-msg"

if [ "$DRY_RUN" -eq 0 ]; then
  mkdir -p "$HOOKS_DST"
fi

installed=0
skipped=0
for name in $MANAGED; do
  src="$HOOKS_SRC/$name"
  [ -f "$src" ] || continue
  dst="$HOOKS_DST/$name"

  # Compute a path from $dst back to $src. .git/hooks is two levels below
  # repo root, so a relative link is portable across repo relocations.
  rel_target="../../$src"
  # If a custom hooksPath is in use we cannot assume depth; fall back to an
  # absolute target in that case.
  if [ -n "$custom_path" ]; then
    rel_target="$REPO_ROOT/$src"
  fi

  # Already a correct symlink? -> skip (idempotent).
  if [ -L "$dst" ]; then
    cur=$(readlink "$dst" 2>/dev/null || true)
    if [ "$cur" = "$rel_target" ]; then
      skipped=$((skipped + 1))
      continue
    fi
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    if [ -e "$dst" ] && [ ! -L "$dst" ]; then
      echo "  WOULD replace (non-symlink): $dst -> $rel_target"
    else
      echo "  WOULD install: $dst -> $rel_target"
    fi
    installed=$((installed + 1))
    continue
  fi

  ln -sf "$rel_target" "$dst"
  chmod +x "$src" 2>/dev/null || true
  echo "  installed: $dst -> $rel_target"
  installed=$((installed + 1))
done

if [ "$DRY_RUN" -eq 1 ]; then
  echo "DRY-RUN: $installed hook(s) would be installed/updated, $skipped already current ($HOOKS_DST)"
else
  echo "OK: $installed hook(s) installed/updated, $skipped already current under $HOOKS_DST"
fi
