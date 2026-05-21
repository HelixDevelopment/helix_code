#!/usr/bin/env bash
# const052_rename_leaf.sh — CONST-052 owned-org submodule LEAF rename driver.
#
# Part of the CONST-052 (§11.4.29) lowercase-snake_case rename programme
# (HXC-001, Phase 0 pre-flight tooling). Renames ONE owned-org submodule
# leaf directory and atomically rewrites EVERY reference so no consumer
# build breaks and §11.4.29 reference-integrity holds.
#
# What it does, in order, for `<org>/<OldName>` -> `<org>/<new_name>`:
#   (a) `git mv` the submodule directory under dependencies/<org>/
#   (b) rewrite .gitmodules — both the `path =` line AND the
#       `[submodule "..."]` section-name header
#   (c) fix the .git/modules/.../  gitdir tree location, the submodule
#       worktree's `.git` gitdir: pointer file, AND the gitdir's own
#       core.worktree back-pointer. NOTE the two relative paths have
#       DIFFERENT depths: the worktree `.git` file lives at
#       dependencies/<org>/<name>/.git (DEPTH dirs below repo root) so
#       its pointer is DEPTH × '../'; the gitdir's core.worktree lives at
#       .git/modules/dependencies/<org>/<name>/config (DEPTH+2 dirs below
#       repo root — the extra '.git' and 'modules' segments) so it needs
#       (DEPTH+2) × '../' to reach the worktree. Using the SAME depth for
#       both leaves core.worktree pointing inside .git/modules/ — git then
#       sees an empty worktree, reports every tracked file as deleted, and
#       the submodule shows a spurious '-dirty' SHA. (This was the Phase
#       2-A bug.)
#   (d) rewrite the matching `replace <module> => ../dependencies/<org>/<OldName>`
#       filesystem path(s) in every consumer go.mod (helix_code, helix_agent,
#       helix_qa). Go MODULE PATHS are abstract and do NOT change — only the
#       `replace => ./path` filesystem suffix is rewritten. When a consumer
#       go.mod lives INSIDE a submodule (helix_agent, helix_qa), it cannot
#       be staged from the meta-repo (`git add` rejects a submodule-resident
#       path); the script stages it WITHIN the submodule with `git -C` and
#       prints a `SUBMODULE-COMMIT-NEEDED: <name>` line so the batch caller
#       commits + pushes that submodule and bumps its pointer. The script
#       itself never commits or pushes submodules (keeps it composable).
#   (e) rewrite docs/coverage/ ledger rows that reference the old path.
#
# Module-path note: a directory rename never changes a Go import path —
# `go list -m` output is unchanged. The only breakage class is `replace`
# filesystem-path resolution, which (d) repairs atomically.
#
# Usage:
#   scripts/const052_rename_leaf.sh <org> <OldName> <new_name> [--dry-run]
#
#   <org>      owned-org parent dir under dependencies/ (HelixDevelopment,
#              vasic-digital, helix_development, ...)
#   <OldName>  current leaf directory name (PascalCase / mixed)
#   <new_name> target lowercase snake_case leaf name
#   --dry-run  print every action without mutating anything; idempotent —
#              re-running against an already-renamed leaf reports
#              "already compliant — no change needed" and exits 0.
#
# Anti-bluff: the script makes REAL filesystem + git mutations. There is no
# simulation path. --dry-run genuinely inspects state and reports the exact
# commands it WOULD run. After a real run, pair with
# scripts/const052_verify_refs.sh to prove 0 stale references remain.
#
# Exit codes: 0 success / already-compliant ; 1 usage or precondition error ;
#             2 mutation failure.
#
# §11.4.18 in-source doc block. §11.4.67 `bash -n`-clean.

set -uo pipefail

cd "$(dirname "$0")/.." || { echo "FATAL: cannot cd to repo root" >&2; exit 1; }
ROOT="$(pwd)"

DRY_RUN=0
ARGS=()
for a in "$@"; do
	if [ "$a" = "--dry-run" ]; then
		DRY_RUN=1
	else
		ARGS+=("$a")
	fi
done

if [ "${#ARGS[@]}" -ne 3 ]; then
	echo "usage: $0 <org> <OldName> <new_name> [--dry-run]" >&2
	exit 1
fi

ORG="${ARGS[0]}"
OLD="${ARGS[1]}"
NEW="${ARGS[2]}"

OLD_PATH="dependencies/${ORG}/${OLD}"
NEW_PATH="dependencies/${ORG}/${NEW}"

CONSUMER_GOMODS=(helix_code/go.mod helix_agent/go.mod helix_qa/go.mod)
LEDGERS=(docs/coverage/COVERAGE_LEDGER.md docs/coverage/ledger.md)

log()  { printf '%s\n' "$*"; }
act()  { if [ "$DRY_RUN" -eq 1 ]; then printf 'DRY-RUN would: %s\n' "$*"; else printf 'RUN: %s\n' "$*"; fi; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 2; }

# Set of top-level directories that are SUBMODULES (from .gitmodules). A
# consumer go.mod whose first path component is in this set is submodule-
# resident and must be staged with `git -C <submodule>` not the meta-repo
# index. helix_code/ is a TRACKED SUBDIRECTORY of the meta-repo (NOT a
# submodule) so it is absent from this set and stages normally.
SUBMODULE_TOPDIRS="$(git config --file .gitmodules --get-regexp '^submodule\..*\.path$' 2>/dev/null \
	| awk '{print $2}' | awk -F/ '{print $1}' | sort -u)"

# is_submodule_topdir <dir> — exit 0 if <dir> is a known submodule root.
is_submodule_topdir() {
	printf '%s\n' "$SUBMODULE_TOPDIRS" | grep -qxF -- "$1"
}

# --- idempotency / precondition gate -------------------------------------
if [ ! -d "$OLD_PATH" ] && [ -d "$NEW_PATH" ]; then
	log "already compliant — no change needed: ${OLD_PATH} is absent and ${NEW_PATH} exists."
	# Confirm .gitmodules also already migrated.
	if grep -q "path = ${OLD_PATH}\$" .gitmodules 2>/dev/null; then
		log "WARNING: .gitmodules still carries the OLD path line — partial prior rename; re-run NOT idempotent here." >&2
		exit 1
	fi
	exit 0
fi

if [ ! -d "$OLD_PATH" ]; then
	fail "source directory ${OLD_PATH} does not exist (and ${NEW_PATH} not present) — nothing to rename."
fi

if [ -e "$NEW_PATH" ]; then
	fail "target ${NEW_PATH} already exists — refusing to overwrite."
fi

if [ "$OLD" = "$NEW" ]; then
	log "already compliant — no change needed: OldName == new_name (${OLD})."
	exit 0
fi

if ! grep -q "path = ${OLD_PATH}\$" .gitmodules 2>/dev/null; then
	fail ".gitmodules has no 'path = ${OLD_PATH}' entry — not a tracked submodule leaf."
fi

log "=== CONST-052 leaf rename: ${OLD_PATH} -> ${NEW_PATH} ==="
[ "$DRY_RUN" -eq 1 ] && log "(dry-run: no mutations will be performed)"

# --- (a) git mv the submodule directory ----------------------------------
act "git mv ${OLD_PATH} ${NEW_PATH}"
if [ "$DRY_RUN" -eq 0 ]; then
	git mv "$OLD_PATH" "$NEW_PATH" || fail "git mv failed"
fi

# --- (b) rewrite .gitmodules: path line + section-name header -------------
act "rewrite .gitmodules path line: ${OLD_PATH} -> ${NEW_PATH}"
act "rewrite .gitmodules section header: [submodule \"${OLD_PATH}\"] -> [submodule \"${NEW_PATH}\"]"
if [ "$DRY_RUN" -eq 0 ]; then
	# Section headers and path lines both embed the path verbatim; a path
	# string is unique enough that a literal global replace is safe here.
	tmp="$(mktemp)"
	sed -e "s|\\[submodule \"${OLD_PATH}\"\\]|[submodule \"${NEW_PATH}\"]|g" \
	    -e "s|path = ${OLD_PATH}\$|path = ${NEW_PATH}|g" \
	    .gitmodules > "$tmp" || { rm -f "$tmp"; fail "sed on .gitmodules failed"; }
	mv "$tmp" .gitmodules
	git add .gitmodules || fail "git add .gitmodules failed"
fi

# --- (c) fix .git/modules tree + worktree .git pointer -------------------
OLD_GITDIR=".git/modules/${OLD_PATH}"
NEW_GITDIR=".git/modules/${NEW_PATH}"
if [ -d "$OLD_GITDIR" ]; then
	act "move gitdir tree: ${OLD_GITDIR} -> ${NEW_GITDIR}"
	if [ "$DRY_RUN" -eq 0 ]; then
		mkdir -p "$(dirname "$NEW_GITDIR")" || fail "mkdir for new gitdir failed"
		mv "$OLD_GITDIR" "$NEW_GITDIR" || fail "moving gitdir tree failed"
	fi
else
	log "note: ${OLD_GITDIR} not present (submodule possibly uninitialised) — skipping gitdir move."
fi

# Worktree .git pointer file — recompute the relative path to .git/modules.
#
# Two DIFFERENT depths are at play (this is the Phase 2-A bug — fixed here):
#   * The worktree `.git` file lives at  <NEW_PATH>/.git  i.e. DEPTH dirs
#     below the repo root (DEPTH = number of '/'-separated components of
#     NEW_PATH). To name the repo-root .git/modules tree from there needs
#     DEPTH × '../'.  -> REL
#   * The gitdir's own core.worktree back-pointer lives at
#     .git/modules/<NEW_PATH>/config  i.e. DEPTH+2 dirs below the repo root
#     (the extra '.git' and 'modules' path segments). To name the worktree
#     from there needs (DEPTH+2) × '../'.  -> WT_REL
# Using REL for core.worktree (the old bug) makes it resolve to a path
# INSIDE .git/modules/ — git then sees an empty worktree and reports a
# spurious '-dirty' submodule. Both depths are computed from NEW_PATH; no
# value is hardcoded.
WT_GITFILE="${NEW_PATH}/.git"
DEPTH="$(printf '%s' "$NEW_PATH" | awk -F/ '{print NF}')"
REL=""
i=0
while [ "$i" -lt "$DEPTH" ]; do REL="../${REL}"; i=$((i + 1)); done
# core.worktree lives 2 levels deeper (.git/modules) -> DEPTH+2 of '../'.
WT_REL="$REL"
WT_REL="../../${WT_REL}"
NEW_GITDIR_REL="${REL}.git/modules/${NEW_PATH}"
CORE_WT="${WT_REL}${NEW_PATH}"
act "rewrite ${WT_GITFILE} -> 'gitdir: ${NEW_GITDIR_REL}'  (worktree-pointer depth=${DEPTH})"
act "set ${NEW_GITDIR}/config core.worktree -> '${CORE_WT}'  (back-pointer depth=$((DEPTH + 2)))"
if [ "$DRY_RUN" -eq 0 ] && [ -d "$NEW_PATH" ]; then
	if [ -f "$WT_GITFILE" ]; then
		printf 'gitdir: %s\n' "$NEW_GITDIR_REL" > "$WT_GITFILE" || fail "rewriting worktree .git pointer failed"
	fi
	# Repair the gitdir's own core.worktree back-pointer so git stays
	# consistent. Set it directly on the config file (not via `git -C
	# <NEW_PATH> config`) because a stale core.worktree can make `git -C`
	# itself resolve the wrong worktree before we have fixed it.
	if [ -f "${NEW_GITDIR}/config" ]; then
		git config --file "${NEW_GITDIR}/config" core.worktree "$CORE_WT" || \
			log "note: could not set core.worktree (non-fatal)"
	fi
fi

# --- (d) rewrite consumer go.mod `replace` filesystem paths --------------
# Track submodules whose go.mod we rewrote — the caller must commit + push
# each and bump its pointer (the script never commits submodules itself).
SUBMODULE_COMMITS_NEEDED=()
for gm in "${CONSUMER_GOMODS[@]}"; do
	[ -f "$gm" ] || continue
	if grep -q "=> \\.\\./${OLD_PATH}\\(/\\|\$\\)" "$gm"; then
		# Determine whether this consumer go.mod is submodule-resident.
		gm_topdir="${gm%%/*}"
		if is_submodule_topdir "$gm_topdir"; then
			act "rewrite ${gm}: replace path ../${OLD_PATH} -> ../${NEW_PATH}  (submodule-resident — stage WITHIN ${gm_topdir})"
		else
			act "rewrite ${gm}: replace path ../${OLD_PATH} -> ../${NEW_PATH}"
		fi
		# Record submodule-resident consumers in BOTH dry-run and real
		# runs so the SUBMODULE-COMMIT-NEEDED hand-off prints either way.
		if is_submodule_topdir "$gm_topdir"; then
			_already=0
			for s in "${SUBMODULE_COMMITS_NEEDED[@]:-}"; do
				[ "$s" = "$gm_topdir" ] && _already=1
			done
			[ "$_already" -eq 0 ] && SUBMODULE_COMMITS_NEEDED+=("$gm_topdir")
		fi
		if [ "$DRY_RUN" -eq 0 ]; then
			tmp="$(mktemp)"
			# Anchor on '../OLD_PATH' followed by a slash or end-of-line so a
			# leaf rename does not accidentally hit a longer sibling path.
			sed -E "s|=> \\.\\./${OLD_PATH}(/\| *\$)|=> ../${NEW_PATH}\\1|g" \
				"$gm" > "$tmp" || { rm -f "$tmp"; fail "sed on ${gm} failed"; }
			mv "$tmp" "$gm"
			if is_submodule_topdir "$gm_topdir"; then
				# Submodule-resident: stage within the submodule worktree.
				# `git add <submodule>/go.mod` from the meta-repo fails with
				# "Pathspec is in submodule" — must use `git -C`.
				gm_rel="${gm#${gm_topdir}/}"
				git -C "$gm_topdir" add "$gm_rel" || \
					fail "git -C ${gm_topdir} add ${gm_rel} failed"
			else
				# Meta-repo tracked subdirectory (helix_code) — stages
				# normally into the meta-repo index.
				git add "$gm" || fail "git add ${gm} failed"
			fi
		fi
	fi
done

# --- (e) rewrite coverage-ledger rows ------------------------------------
for ledger in "${LEDGERS[@]}"; do
	[ -f "$ledger" ] || continue
	if grep -q "${OLD_PATH}\\(/\\| \\||\\)" "$ledger"; then
		act "rewrite ${ledger}: ledger rows ${OLD_PATH} -> ${NEW_PATH}"
		if [ "$DRY_RUN" -eq 0 ]; then
			tmp="$(mktemp)"
			sed "s|${OLD_PATH}|${NEW_PATH}|g" "$ledger" > "$tmp" || { rm -f "$tmp"; fail "sed on ${ledger} failed"; }
			mv "$tmp" "$ledger"
			git add "$ledger" || fail "git add ${ledger} failed"
		fi
	fi
done

# --- caller hand-off: submodule-resident go.mod edits need their own commit
if [ "${#SUBMODULE_COMMITS_NEEDED[@]}" -gt 0 ]; then
	for sm in "${SUBMODULE_COMMITS_NEEDED[@]}"; do
		# Printed for BOTH dry-run and real runs so a batch caller can plan.
		log "SUBMODULE-COMMIT-NEEDED: ${sm}"
	done
fi

if [ "$DRY_RUN" -eq 1 ]; then
	log "=== dry-run complete: 0 mutations performed ==="
else
	log "=== rename complete: ${OLD_PATH} -> ${NEW_PATH} ==="
	log "next: run scripts/const052_verify_refs.sh ${OLD_PATH} to confirm 0 stale references."
	if [ "${#SUBMODULE_COMMITS_NEEDED[@]}" -gt 0 ]; then
		log "note: ${#SUBMODULE_COMMITS_NEEDED[@]} submodule(s) above have a staged go.mod — commit + push them and bump their pointer."
	fi
fi
exit 0
