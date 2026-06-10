#!/usr/bin/env bash
# resolve_recursive_fork_deps.sh — SP3 recursive nested-dep classifier.
#
# Scan a tree for nested .gitmodules and classify each nested dependency:
#   third-party nested dep  → FORK (emit a FORK:<org>/<fork> row; the actual
#                             fork is delegated to fork_third_party_submodule.sh
#                             with --src-dir = the nested manifest's dir, then
#                             the fork's .gitmodules is rewritten to the SSH
#                             fork URL — Rule 3, §11.4.30-clean minimal change).
#   own-org nested dep      → CONST-051 VIOLATION → emit a PULL_TO_ROOT
#                             remediation row. NEVER silently rewrite an own-org
#                             chain (§11.4.28/CONST-051).
#
# The one known live case: cli_agents/cline → evals/cline-bench
# (https://github.com/cline/cline-bench.git, HTTPS, third-party) → FORK +
# SSH-rewrite to git@github.com:vasic-digital/caf-cline-bench.git.
#
# ANTI-BLUFF (§107): classification is observable in --nested-report; the
# Challenge proves FORK vs PULL_TO_ROOT against fixtures + a paired §1.1
# mutation (caf_is_own_org always-false → own-org row mis-classifies → FAIL).
#
# Depth-bounded (--depth, default 3); cycle guard via a visited-set on
# normalized URLs. Report-only by default; --execute delegates the fork
# (OPERATOR-GATED G-1) only when a fork CLI is available.
#
# Usage:
#   bash resolve_recursive_fork_deps.sh --src-dir <tree> [--depth N] \
#        [--nested-report <tsv>] [--execute]
#
# Exit codes:
#   0 — scan complete, no CONST-051 violation found
#   1 — scan complete, at least one own-org nested chain flagged (PULL_TO_ROOT)
#   2 — environment problem (git missing / scan root absent)

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
# shellcheck source=caf_lib.sh
source "${SCRIPT_DIR}/caf_lib.sh"
caf_parse_args "$@"

caf_have git || { echo "ERROR: git required" >&2; exit 2; }

SCAN_ROOT="${CAF_SRC_DIR}"
[ -d "${SCAN_ROOT}" ] || SCAN_ROOT="${REPO_ROOT}/${CAF_SRC_DIR}"
if [ ! -d "${SCAN_ROOT}" ] && [ ! -f "${SCAN_ROOT}/.gitmodules" ]; then
    # Allow pointing directly at a directory that simply contains a .gitmodules.
    if [ ! -e "${CAF_SRC_DIR}/.gitmodules" ] && [ ! -d "${CAF_SRC_DIR}" ]; then
        echo "ERROR: scan root not found: ${CAF_SRC_DIR}" >&2
        exit 2
    fi
    SCAN_ROOT="${CAF_SRC_DIR}"
fi

mkdir -p "$(dirname "${CAF_NESTED_REPORT}")" 2>/dev/null || true
: > "${CAF_NESTED_REPORT}"   # fresh report each run (report is a derived artefact)

_CAF_VISITED=" "   # space-delimited visited-set of normalized URLs
rc=0

# scan_dir <dir> <depth>
scan_dir() {
    local dir="$1" depth="$2"
    [ "${depth}" -le 0 ] && return 0
    local gm="${dir}/.gitmodules"
    [ -f "${gm}" ] || return 0

    local entry key url norm owner sub fork
    while IFS= read -r entry; do
        [ -z "${entry}" ] && continue
        key="${entry%% *}"
        url="${entry#* }"
        sub="${key#submodule.}"
        sub="${sub%.url}"
        norm="$(caf_normalize_url "${url}")"

        # cycle guard
        if [[ "${_CAF_VISITED}" == *" ${norm} "* ]]; then
            caf_log CYCLE "skip already-visited ${norm}"
            continue
        fi
        _CAF_VISITED="${_CAF_VISITED}${norm} "

        if caf_is_own_org "${norm}"; then
            caf_log CONST051 "${dir} nested own-org chain: ${url}"
            printf '%s\t%s\t%s\n' "${dir}" "${url}" "PULL_TO_ROOT" >> "${CAF_NESTED_REPORT}"
            rc=1
        else
            fork="$(caf_fork_name "$(basename "${sub}")")"
            printf '%s\t%s\t%s\n' "${dir}" "${url}" "FORK:${CAF_ORG}/${fork}" >> "${CAF_NESTED_REPORT}"
            caf_log NESTED-FORK "${dir} ${url} -> ${CAF_ORG}/${fork}"
            if [ "${CAF_DRY_RUN}" != 1 ] && caf_have gh; then
                caf_log PLAN "delegate fork of ${norm} -> ${CAF_ORG}/${fork} + SSH-rewrite nested .gitmodules (G-1)"
                # Real delegation is OPERATOR-GATED; left as a logged plan so
                # this script never performs an un-gated external mutation.
            fi
        fi
    done < <(git config -f "${gm}" --get-regexp '\.url$' 2>/dev/null)

    # Recurse into each checked-out nested submodule directory one level deeper.
    local subpath
    while IFS= read -r subpath; do
        [ -z "${subpath}" ] && continue
        local child="${dir}/${subpath}"
        [ -d "${child}" ] && scan_dir "${child}" "$((depth - 1))"
    done < <(git config -f "${gm}" --get-regexp '\.path$' 2>/dev/null | awk '{print $2}')
}

# Scan every direct submodule dir under the src-dir, plus the src-dir itself.
if [ -f "${SCAN_ROOT}/.gitmodules" ]; then
    scan_dir "${SCAN_ROOT}" "${CAF_DEPTH}"
fi
# Also scan each immediate child directory (the per-agent submodule worktrees).
if [ -d "${SCAN_ROOT}" ]; then
    for child in "${SCAN_ROOT}"/*/; do
        [ -d "${child}" ] || continue
        scan_dir "${child%/}" "${CAF_DEPTH}"
    done
fi

caf_log DONE "resolve_recursive_fork_deps rc=${rc} report=${CAF_NESTED_REPORT}"
exit ${rc}
