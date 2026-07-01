#!/usr/bin/env bash
#
# HelixCode Submodule Initialization & Latest-Tracking Script
# ==========================================================
#
# Guarantees on every run:
#   1. INTERRUPTION-SAFE — a fresh clone whose `git submodule update` was
#      interrupted leaves submodules "cloned-but-not-checked-out" (objects +
#      HEAD present, index + worktree empty; `git status` then shows every
#      file as a staged deletion). Re-running converges to a fully
#      checked-out tree instead of stranding them.
#   2. LATEST-TRACKING — every repo is advanced to the tip of its *default*
#      branch (origin/HEAD -> main/master/…, detected, never guessed),
#      FAST-FORWARD ONLY. No force-push, no history rewrite — Constitution
#      §11.4.113 (absolute no-force / merge-onto-latest) and §9 (data safety).
#
# IMPORTANT: enumeration is a FILESYSTEM WALK, not `git submodule foreach`.
# `git submodule foreach --recursive` silently skips cloned-but-empty
# submodules (verified: it does not descend into an un-checked-out worktree),
# so relying on it for recovery/verify is blind to exactly the failure this
# script must fix (a §11.4.108-class verification gap). `find -name .git`
# reliably finds every cloned repo at every depth, empty or not.
#
# Env:
#   HELIX_SUBMODULE_JOBS   parallel jobs (default 8)
#   HELIX_SUBMODULE_MAP    optional; append "path<TAB>branch" per repo advanced
#
set -uo pipefail

JOBS="${HELIX_SUBMODULE_JOBS:-8}"
MAP="${HELIX_SUBMODULE_MAP:-}"
ROOT="$(git rev-parse --show-toplevel)"
[ -n "$MAP" ] && : > "$MAP"
export ROOT MAP

# Every cloned repo under ROOT (excluding ROOT itself), pruning heavy non-repo
# dirs for speed. `.git` (dir OR gitlink file) marks a repo root at any depth.
list_repos() {
  find "$ROOT" -type d \( -name node_modules -o -name .venv -o -name vendor \
       -o -name target -o -name .gradle -o -name build \) -prune \
       -o -name .git -print 2>/dev/null \
    | while IFS= read -r g; do d="$(dirname "$g")"; [ "$d" = "$ROOT" ] || printf '%s\n' "$d"; done
}

# True iff the worktree at $1 holds nothing but .git (cloned-but-not-checked-out).
# Uses `find -print -quit` (stops at the first non-.git entry) rather than a
# stderr-suppressed `ls`, so a transient FS error on a populated dir cannot be
# misread as "empty" and trigger a force-checkout.
repo_is_empty() {
  [ -z "$(find "$1" -mindepth 1 -maxdepth 1 ! -name .git -print -quit 2>/dev/null)" ]
}
export -f repo_is_empty

# Resolve a repo's default branch, never guessing (§11.4.6): origin/HEAD symref,
# else `remote set-head --auto`, else probe main/master. Echoes "" if none.
# Shared by advance_repo AND the verify at-tip check so both agree.
resolve_def() {
  d="$1"; def="$(git -C "$d" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##')"
  if [ -z "$def" ]; then
    git -C "$d" remote set-head origin --auto >/dev/null 2>&1 || true
    def="$(git -C "$d" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##')"
  fi
  if [ -z "$def" ]; then
    for c in main master; do
      if git -C "$d" ls-remote --exit-code --heads origin "$c" >/dev/null 2>&1; then def="$c"; break; fi
    done
  fi
  printf '%s' "$def"
}
export -f resolve_def

# Resolve default branch (never guess), fetch, recover empty worktree, ff to tip.
advance_repo() {
  d="$1"
  def="$(resolve_def "$d")"
  [ -z "$def" ] && { echo "?? ${d#"$ROOT"/}: could not resolve default branch — skipped"; return 0; }

  git -C "$d" fetch --quiet origin "$def" 2>/dev/null || git -C "$d" fetch --quiet origin 2>/dev/null || true

  if repo_is_empty "$d" && git -C "$d" rev-parse --verify -q HEAD >/dev/null 2>&1; then
    # empty worktree of an initialized repo (interrupted checkout): nothing to
    # lose — force-populate the branch from its tip. -B is safe here precisely
    # because the worktree is empty AND HEAD exists (objects/reflog retained).
    git -C "$d" checkout -f -B "$def" "origin/$def" >/dev/null 2>&1 || true
  else
    # NON-empty: land on the default branch WITHOUT resetting it, so any
    # local-ahead (unpushed) commits are preserved. NEVER use `-B`/`branch -f`
    # here — that would drop unpushed work (§9 / §11.4.113). `merge --ff-only`
    # below is the SOLE advance mechanism: it advances when local is behind,
    # and safely no-ops when local is ahead or has diverged.
    if git -C "$d" show-ref --verify --quiet "refs/heads/$def"; then
      git -C "$d" checkout -q "$def" >/dev/null 2>&1 || true
    else
      git -C "$d" checkout -q -b "$def" --track "origin/$def" >/dev/null 2>&1 || true
    fi
  fi
  git -C "$d" merge --ff-only "origin/$def" >/dev/null 2>&1 || true

  echo "ok ${d#"$ROOT"/} -> $def @ $(git -C "$d" rev-parse --short HEAD 2>/dev/null)"
  [ -n "$MAP" ] && printf '%s\t%s\n' "${d#"$ROOT"/}" "$def" >> "$MAP"
}
export -f advance_repo

# For every submodule declared across every .gitmodules in the tree that is NOT
# populated, emit "<class>\t<abs-path>":
#   gitlink — a real registered gitlink (mode 160000 in its parent's index)
#             whose clone/checkout never completed  → a genuine failure (fatal)
#   orphan  — a .gitmodules entry with NO gitlink in the parent's tree
#             (stale/incomplete config; nothing to clone) → hygiene warning
# Recurses .gitmodules at all depths.
declared_unpopulated() {
  find "$ROOT" -type d \( -name node_modules -o -name .venv -o -name vendor \
       -o -name target -o -name .gradle -o -name build \) -prune -o -name .gitmodules -print 2>/dev/null \
  | while IFS= read -r gm; do
      base="$(dirname "$gm")"
      git config -f "$gm" --get-regexp '^submodule\..*\.path$' 2>/dev/null | awk '{print $2}' \
      | while IFS= read -r rel; do
          [ -n "$rel" ] || continue
          p="$base/$rel"
          { [ -e "$p/.git" ] && ! repo_is_empty "$p"; } && continue   # populated → ok
          if [ "$(git -C "$base" ls-files --stage -- "$rel" 2>/dev/null | awk '{print $1}')" = "160000" ]; then
            printf 'gitlink\t%s\n' "$p"
          else
            printf 'orphan\t%s\n' "$p"
          fi
        done
    done
}

echo "==> [1/4] sync + init/clone (resumable, --jobs $JOBS)"
git submodule sync --recursive >/dev/null 2>&1 || true
git submodule update --init --recursive --jobs "$JOBS" || echo "   (deferred — recovery loop follows)"

echo "==> [2/4] recover interrupted clones/checkouts (bounded retry until stable)"
# Clones/checkouts can fail transiently (network) or leave empty worktrees when
# interrupted. Retry the recursive init a few times; each round also re-reveals
# nested submodules exposed by newly-populated worktrees. Converges quickly.
for round in 1 2 3; do
  missing="$(declared_unpopulated | awk -F'\t' '$1=="gitlink"{print $2}')"
  [ -z "$missing" ] && break
  echo "   round $round: $(printf '%s\n' "$missing" | grep -c .) gitlink(s) still unpopulated — retrying"
  git submodule update --init --recursive --jobs "$JOBS" >/dev/null 2>&1 || true
done

echo "==> [3/4] advance every repo to latest default-branch tip (FINAL mutation, ff-only)"
# MUST be the last tree mutation: a later `git submodule update` would re-pin
# nested submodules to their parent's recorded SHA and undo latest-tracking.
list_repos | xargs -P "$JOBS" -I{} bash -c 'advance_repo "$@"' _ {}

echo "==> [4/4] verify: (a) no empty worktree, (b) every declared path populated, (c) at branch tip"
fail=0

# (a) EMPTY worktrees = interrupted checkout not recovered — HARD failure.
empty="$(list_repos | while IFS= read -r d; do repo_is_empty "$d" && printf '%s\n' "${d#"$ROOT"/}"; done)"
if [ -n "$empty" ]; then
  echo "ERROR: cloned-but-not-checked-out submodule(s) remain — re-run to recover:" >&2
  printf '   !! empty: %s\n' $empty >&2
  fail=1
fi

# (b) DECLARED-but-missing — a .gitmodules path with no populated worktree means
# its clone failed entirely (SSH denied / URL down); it is invisible to the
# filesystem walk, so reconcile against the declared set (§11.4.6 — do not
# report all-green while a submodule is absent). Recurse into every .gitmodules.
unpop="$(declared_unpopulated)"
gitlink_missing="$(printf '%s\n' "$unpop" | awk -F'\t' '$1=="gitlink"{print $2}' | while IFS= read -r p; do [ -n "$p" ] && printf '%s\n' "${p#"$ROOT"/}"; done)"
orphan_entries="$(printf '%s\n' "$unpop" | awk -F'\t' '$1=="orphan"{print $2}'  | while IFS= read -r p; do [ -n "$p" ] && printf '%s\n' "${p#"$ROOT"/}"; done)"
if [ -n "$gitlink_missing" ]; then
  echo "ERROR: registered gitlink(s) not populated — clone/checkout failed, re-run to recover:" >&2
  printf '   !! missing: %s\n' $gitlink_missing >&2
  fail=1
fi
if [ -n "$orphan_entries" ]; then
  # A .gitmodules entry with no matching gitlink in the tree: stale/incomplete
  # config in the owning repo, not this script's failure. Surfaced (§11.4.6),
  # not fatal — resolving it (add the gitlink or drop the entry) is a repo-
  # hygiene decision for that submodule's owner.
  echo "WARNING: orphan .gitmodules entr(y/ies) — declared but no gitlink in the owning tree:" >&2
  printf '   ~~ orphan: %s\n' $orphan_entries >&2
fi

# (c) NOT-AT-TIP — populated but the fast-forward to origin/<default> did not
# land (diverged / fetch failed / default unresolved). Guarantee #2 is
# "advanced to latest tip", so a repo whose tip is NOT contained in HEAD is
# reported. This is a WARNING (owned repos may legitimately be ahead of a
# mirror), not a hard failure — but it is surfaced, never hidden (M1).
behind="$(list_repos | while IFS= read -r d; do
  repo_is_empty "$d" && continue
  def="$(resolve_def "$d")"          # same 3-tier resolution as advance_repo
  [ -z "$def" ] && continue
  git -C "$d" rev-parse --verify --quiet "refs/remotes/origin/$def" >/dev/null 2>&1 || continue
  # not-at-tip iff origin tip is NOT an ancestor of HEAD (i.e. HEAD is behind)
  git -C "$d" merge-base --is-ancestor "origin/$def" HEAD 2>/dev/null \
    || printf '%s (origin/%s not reached)\n' "${d#"$ROOT"/}" "$def"
done)"
if [ -n "$behind" ]; then
  echo "WARNING: repo(s) not at their latest default-branch tip (surfaced, not fatal):" >&2
  printf '%s\n' "$behind" | while IFS= read -r line; do [ -n "$line" ] && printf '   ~~ %s\n' "$line" >&2; done
fi

[ "$fail" -ne 0 ] && exit 1
echo "All submodules initialized, populated, and tracking their latest default branch. ✓"
