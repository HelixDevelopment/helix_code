#!/usr/bin/env bash
# update_fork_from_upstream.sh — SP3 auto-merge unit of work.
#
# Keep every fork current by merging upstream main/master INTO the fork,
# fast-forward-preferred, NEVER force (§11.4.113). Reads the --map-file rows
# (name \t upstream \t fork_gh \t fork_gl) and, for each:
#   clone fork origin → add upstream remote → fetch → merge upstream/<base>
#   → fast-forward push to ALL provider remotes.
#
# §11.4.113 merge-onto-latest-main: base = the fork's own default branch; we
# merge upstream ON TOP of it, so every push descends from the fork tip and is
# a fast-forward — no force is ever needed. All pushes go through caf_safe_push,
# which REFUSES force-class args. Conflicts are NEVER auto-resolved (no -s ours,
# no -X theirs): a conflicted fork is logged + parked for operator review and
# the loop continues (zero-idle §11.4.94).
#
# ANTI-BLUFF (§107): a PASS observes the REAL post-merge state — the upstream
# SHA present in the fork's branch + the push being fast-forward — captured via
# GIT_TRACE. The Challenge proves this against LOCAL bare repos (no real remote).
#
# §11.4.101: the first non --dry-run push is OPERATOR-GATED (G-1-push). DEFAULT
# is --dry-run (plan only).
#
# Graceful degradation: needs only git (no gh/glab). Map rows whose fork remote
# is unreachable are logged CLONEFAIL + skipped.
#
# Usage:
#   bash update_fork_from_upstream.sh --map-file <tsv> [--workdir <dir>] \
#        [--only <name>] [--strategy merge|ff-only] [--no-push] [--execute]
#
# Exit codes:
#   0 — all forks updated (or dry-run planned) with no conflicts
#   1 — partial: at least one CONFLICT parked or PUSHFAIL logged
#   2 — environment problem (git missing / map-file absent)

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
# shellcheck source=caf_lib.sh
source "${SCRIPT_DIR}/caf_lib.sh"
caf_parse_args "$@"

caf_have git || { echo "ERROR: git required" >&2; exit 2; }
if [ ! -f "${CAF_MAP_FILE}" ]; then
    echo "ERROR: map-file not found: ${CAF_MAP_FILE}" >&2
    exit 2
fi

WORKDIR="${CAF_WORKDIR:-$(mktemp -d)}"
mkdir -p "${WORKDIR}"
_CAF_OWN_WORKDIR=0
[ -z "${CAF_WORKDIR}" ] && _CAF_OWN_WORKDIR=1
cleanup() { [ "${_CAF_OWN_WORKDIR}" = 1 ] && rm -rf "${WORKDIR}"; }
trap cleanup EXIT

CONFLICT_DIR="${REPO_ROOT}/docs/caf/conflicts"
mkdir -p "${CONFLICT_DIR}" 2>/dev/null || true

caf_record_conflict() {
    local fork="$1" repo="$2"
    local ts; ts="$(date -u +%Y%m%dT%H%M%SZ)"
    local f="${CONFLICT_DIR}/${fork}_${ts}.md"
    {
        echo "# CONFLICT — ${fork} — ${ts}"
        echo
        echo "Upstream merge produced conflicts; parked for operator review (NEVER auto-resolved per §11.4.113)."
        echo
        echo '## Conflicting paths'
        ( cd "${repo}" && git diff --name-only --diff-filter=U 2>/dev/null ) || true
    } > "${f}" 2>/dev/null || true
    caf_log CONFLICT "${fork} → ${f}"
}

rc=0
while IFS=$'\t' read -r name upstream fork_gh fork_gl; do
    [ -z "${name:-}" ] && continue
    case "${name}" in '#'*) continue ;; esac
    if [ -n "${CAF_ONLY}" ] && ! caf_in_csv "${name}" "${CAF_ONLY}"; then
        continue
    fi

    fork="$(caf_fork_name "${name}")"
    repo="${WORKDIR}/${fork}"

    # Clone the fork's origin (first provider URL = GitHub by convention).
    if ! git clone "${fork_gh}" "${repo}" >/dev/null 2>&1; then
        caf_log CLONEFAIL "${fork} (${fork_gh})"; rc=1; continue
    fi

    (
        cd "${repo}" || exit 1
        git remote add upstream "${upstream}" 2>/dev/null \
            || git remote set-url upstream "${upstream}"
        git fetch --all --prune --tags >/dev/null 2>&1 || true

        base="$(caf_detect_default_branch upstream)"
        # Make sure the fork has a local checkout of base.
        git checkout "${base}" >/dev/null 2>&1 \
            || git checkout -b "${base}" "origin/${base}" >/dev/null 2>&1 || exit 1

        pre_tip="$(git rev-parse HEAD)"

        # §11.4.113: merge-onto-latest-main. Prefer fast-forward; fall back to a
        # union-preserving merge. NEVER -s ours / -X theirs.
        if git merge --ff-only "upstream/${base}" >/dev/null 2>&1; then
            caf_log FF "${fork} ${base}"
        elif [ "${CAF_STRATEGY}" = "ff-only" ]; then
            # ff-only requested but not possible → park (do not force a merge).
            caf_log NONFF "${fork} ${base} (ff-only requested, divergent — parked)"
            exit 3
        elif git merge --no-edit "upstream/${base}" >/dev/null 2>&1; then
            caf_log MERGE "${fork} ${base}"
        else
            git merge --abort >/dev/null 2>&1 || true
            # Re-run to capture conflict paths for the report.
            git merge --no-commit --no-ff "upstream/${base}" >/dev/null 2>&1 || true
            exit 4   # signal conflict to the parent
        fi

        post_tip="$(git rev-parse HEAD)"
        caf_log TIP "${fork} ${pre_tip} -> ${post_tip}"

        # Verify the upstream tip is now an ancestor (real post-state, anti-bluff).
        up_tip="$(git rev-parse "upstream/${base}")"
        if ! git merge-base --is-ancestor "${up_tip}" HEAD; then
            caf_log VERIFYFAIL "${fork} upstream tip not reachable post-merge"
            exit 5
        fi

        if [ "${CAF_PUSH}" = 1 ]; then
            while IFS= read -r remote; do
                [ -z "${remote}" ] && continue
                if caf_safe_push "${remote}" "${base}" >/dev/null 2>&1; then
                    caf_log PUSHED "${fork} -> ${remote}"
                else
                    caf_log PUSHFAIL "${fork} -> ${remote}"
                    exit 6
                fi
            done < <(caf_provider_remotes "${fork}")
        fi
        exit 0
    )
    sub_rc=$?
    case "${sub_rc}" in
        0) : ;;
        4) caf_record_conflict "${fork}" "${repo}"; rc=1 ;;
        3) rc=1 ;;                         # non-ff parked
        *) caf_log FAIL "${fork} (sub_rc=${sub_rc})"; rc=1 ;;
    esac
done < "${CAF_MAP_FILE}"

caf_log DONE "update_fork_from_upstream rc=${rc} (dry_run=${CAF_DRY_RUN}, push=${CAF_PUSH})"
exit ${rc}
