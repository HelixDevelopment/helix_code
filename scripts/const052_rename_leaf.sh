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
#   (c) fix the .git/modules/.../  gitdir tree location and the
#       submodule worktree's `.git` gitdir: pointer file
#   (d) rewrite the matching `replace <module> => ../dependencies/<org>/<OldName>`
#       filesystem path(s) in every consumer go.mod (helix_code, helix_agent,
#       helix_qa). Go MODULE PATHS are abstract and do NOT change — only the
#       `replace => ./path` filesystem suffix is rewritten.
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
WT_GITFILE="${NEW_PATH}/.git"
DEPTH="$(printf '%s' "$NEW_PATH" | awk -F/ '{print NF}')"
REL=""
i=0
while [ "$i" -lt "$DEPTH" ]; do REL="../${REL}"; i=$((i + 1)); done
NEW_GITDIR_REL="${REL}.git/modules/${NEW_PATH}"
act "rewrite ${WT_GITFILE} -> 'gitdir: ${NEW_GITDIR_REL}'"
if [ "$DRY_RUN" -eq 0 ] && [ -d "$NEW_PATH" ]; then
	if [ -f "$WT_GITFILE" ]; then
		printf 'gitdir: %s\n' "$NEW_GITDIR_REL" > "$WT_GITFILE" || fail "rewriting worktree .git pointer failed"
	fi
	# Repair the gitdir's own core.worktree back-pointer so git stays consistent.
	if [ -f "${NEW_GITDIR}/config" ]; then
		git -C "$NEW_PATH" config --local core.worktree "${REL}${NEW_PATH}" 2>/dev/null || \
			log "note: could not set core.worktree (non-fatal)"
	fi
fi

# --- (d) rewrite consumer go.mod `replace` filesystem paths --------------
for gm in "${CONSUMER_GOMODS[@]}"; do
	[ -f "$gm" ] || continue
	if grep -q "=> \\.\\./${OLD_PATH}\\(/\\|\$\\)" "$gm"; then
		act "rewrite ${gm}: replace path ../${OLD_PATH} -> ../${NEW_PATH}"
		if [ "$DRY_RUN" -eq 0 ]; then
			tmp="$(mktemp)"
			# Anchor on '../OLD_PATH' followed by a slash or end-of-line so a
			# leaf rename does not accidentally hit a longer sibling path.
			sed -E "s|=> \\.\\./${OLD_PATH}(/\| *\$)|=> ../${NEW_PATH}\\1|g" \
				"$gm" > "$tmp" || { rm -f "$tmp"; fail "sed on ${gm} failed"; }
			mv "$tmp" "$gm"
			git add "$gm" || fail "git add ${gm} failed"
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

if [ "$DRY_RUN" -eq 1 ]; then
	log "=== dry-run complete: 0 mutations performed ==="
else
	log "=== rename complete: ${OLD_PATH} -> ${NEW_PATH} ==="
	log "next: run scripts/const052_verify_refs.sh ${OLD_PATH} to confirm 0 stale references."
fi
exit 0
