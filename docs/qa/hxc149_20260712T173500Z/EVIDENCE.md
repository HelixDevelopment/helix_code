# HXC-149 — QA evidence (§11.4.83)

**Item:** HXC-149 (Bug/Med) — stale gitlinks breaking submodule walk
**Fix commit:** helix_code `ac75ee4a` (git-index-only; pushed github+gitlab)
**Date (UTC):** 2026-07-12T17:35:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)

## Root cause
67 stale gitlinks (5 top-level + 62 dependencies/*) from a historical path-rename had no
.gitmodules mapping, causing git submodule status/foreach to abort mid-walk.

## Fix
git rm --cached on all 67 stale entries. Valid entries untouched, hashes verified correct.
Submodule walk now completes without fatal. Git-index-only change, no working-tree modification.

## Verification
git submodule status completes without fatal. comm -23 shows 0 stale entries. Spot-checked
valid gitlink hashes (submodules/containers, constitution, submodules/helix_agent — all OK).
