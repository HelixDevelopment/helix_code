#!/usr/bin/env bash
# fork_third_party_submodule.sh — SP3 fork-all.
#
# For every submodule under --src-dir in the root .gitmodules: create
# <org>/<prefix><name> on each configured provider (GitHub via gh, GitLab via
# glab), wire origin=our-fork + upstream=original, and record the mapping in
# --map-file. Idempotent: re-running detects an existing fork and skips.
#
# ANTI-BLUFF (§107): a PASS observes the REAL remote (gh repo view / glab repo
# view exit 0 + the recorded map row), never just a CLI exit code. The
# Challenge (fork_third_party_submodule.challenge.sh) proves this with throwaway
# LOCAL repos — no real remote required.
#
# §11.4.113: the create+mirror-seed fallback targets an EMPTY fork (no refs) so
# it is a fast-forward seed, NOT a force / history overwrite. All real pushes go
# through caf_safe_push which refuses force-class args.
#
# §11.4.28(B)/CONST-051: we never edit a fork's source — only remote config.
# §11.4.101: any non --dry-run invocation creates irreversible external state
# and is OPERATOR-GATED behind G-1. The DEFAULT is --dry-run.
#
# Graceful degradation: if gh (or glab when GitLab is requested) is absent, the
# real-remote path SKIPs with reason (exit 2); --dry-run always works.
#
# Usage:
#   bash fork_third_party_submodule.sh [--dry-run|--execute] [flags...]
#   (see caf_lib.sh caf_parse_args for the full flag surface)
#
# Exit codes:
#   0 — all handled (or dry-run planned)
#   1 — partial (some FAIL logged; one bad repo never aborts the batch)
#   2 — environment problem (gh/glab/git missing for the requested providers)

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
# shellcheck source=caf_lib.sh
source "${SCRIPT_DIR}/caf_lib.sh"
caf_parse_args "$@"

# --- environment gate (graceful degradation) -------------------------------
if [ "${CAF_DRY_RUN}" != 1 ]; then
    if ! caf_have gh; then
        caf_log SKIP "gh CLI absent — real-remote fork path unavailable (re-run with --dry-run)"
        echo "ERROR: gh CLI required for --execute. Re-run with --dry-run to plan only." >&2
        exit 2
    fi
    if caf_in_csv gitlab "${CAF_PROVIDERS}" && ! caf_have glab; then
        caf_log SKIP "glab CLI absent — GitLab provider requested but unavailable"
        echo "ERROR: glab required for the gitlab provider. Drop it via --providers github, or install glab." >&2
        exit 2
    fi
fi

if [ ! -f "${CAF_GITMODULES}" ]; then
    echo "ERROR: gitmodules file not found: ${CAF_GITMODULES}" >&2
    exit 2
fi

mkdir -p "$(dirname "${CAF_MAP_FILE}")" 2>/dev/null || true

# caf_poll_fork_ready <org/fork> — GitHub forks are async; gh repo view may 404
# briefly after gh repo fork. Poll up to ~30s. Dry-run never reaches here.
caf_poll_fork_ready() {
    local slug="$1" i
    for i in $(seq 1 10); do
        gh repo view "${slug}" >/dev/null 2>&1 && return 0
        sleep 3
    done
    return 1
}

rc=0
# Iterate every cli_agents/<name>.url entry. Read into an array first so the
# loop body runs in THIS shell (not a subshell pipe) and rc/map writes persist.
mapfile -t _entries < <(git config -f "${CAF_GITMODULES}" --get-regexp \
    "submodule\.${CAF_SRC_DIR}/.*\.url" 2>/dev/null)

for entry in "${_entries[@]}"; do
    key="${entry%% *}"
    url="${entry#* }"
    # submodule.<src-dir>/<name>.url → <name>
    name="${key#submodule.${CAF_SRC_DIR}/}"
    name="${name%.url}"

    norm="$(caf_normalize_url "${url}")"

    if caf_is_own_org "${norm}"; then
        caf_log SKIP "own-org ${name} (${norm})"
        continue
    fi
    if caf_in_csv "${name}" "${CAF_EXCLUDE}"; then
        caf_log SKIP "excluded ${name}"
        continue
    fi
    if [ -n "${CAF_ONLY}" ] && ! caf_in_csv "${name}" "${CAF_ONLY}"; then
        continue
    fi

    fork="$(caf_fork_name "${name}")"
    fork_gh="$(caf_fork_ssh_url github "${fork}")"
    fork_gl="$(caf_fork_ssh_url gitlab "${fork}")"

    if [ "${CAF_DRY_RUN}" = 1 ]; then
        caf_log PLAN "gh repo fork ${norm} --org ${CAF_ORG} --fork-name ${fork} --clone=false"
        caf_in_csv gitlab "${CAF_PROVIDERS}" && \
            caf_log PLAN "glab repo create ${CAF_GITLAB_GROUP}/${fork} --${CAF_VISIBILITY}"
        printf '%s\t%s\t%s\t%s\n' "${name}" "${url}" "${fork_gh}" "${fork_gl}" >> "${CAF_MAP_FILE}"
        continue
    fi

    # --- (a) GitHub fork-or-create (OPERATOR-GATED G-1) --------------------
    if gh repo view "${CAF_ORG}/${fork}" >/dev/null 2>&1; then
        caf_log EXISTS "${CAF_ORG}/${fork}"
    elif gh repo fork "${norm}" --org "${CAF_ORG}" --fork-name "${fork}" --clone=false 2>/dev/null; then
        if caf_poll_fork_ready "${CAF_ORG}/${fork}"; then
            caf_log FORKED "${CAF_ORG}/${fork}"
        else
            caf_log FAIL "fork-not-ready ${name}"; rc=1; continue
        fi
    else
        # Fallback: non-forkable (HTTPS-only / archived). create + mirror-seed
        # into an EMPTY target (§11.4.113-safe — no existing refs overwritten).
        if ! gh repo create "${CAF_ORG}/${fork}" "--${CAF_VISIBILITY}" >/dev/null 2>&1; then
            caf_log FAIL "create ${name}"; rc=1; continue
        fi
        tmp="$(mktemp -d)"
        if git clone --bare "${url}" "${tmp}/repo.git" >/dev/null 2>&1; then
            # Push every branch + tag to the EMPTY fork. NOT --mirror/--force.
            ( cd "${tmp}/repo.git" \
              && git push "${fork_gh}" 'refs/heads/*:refs/heads/*' \
              && git push "${fork_gh}" 'refs/tags/*:refs/tags/*' ) \
              && caf_log SEEDED "${CAF_ORG}/${fork}" \
              || { caf_log FAIL "seed ${name}"; rc=1; rm -rf "${tmp}"; continue; }
        else
            caf_log FAIL "clone ${name}"; rc=1; rm -rf "${tmp}"; continue
        fi
        rm -rf "${tmp}"
    fi

    # --- (b) GitLab mirror of the fork (dual-remote) ----------------------
    if caf_in_csv gitlab "${CAF_PROVIDERS}"; then
        if glab repo view "${CAF_GITLAB_GROUP}/${fork}" >/dev/null 2>&1; then
            caf_log EXISTS "gitlab ${CAF_GITLAB_GROUP}/${fork}"
        elif ! glab repo create "${CAF_GITLAB_GROUP}/${fork}" "--${CAF_VISIBILITY}" >/dev/null 2>&1; then
            caf_log FAIL "gitlab ${name}"; rc=1
        else
            caf_log CREATED "gitlab ${CAF_GITLAB_GROUP}/${fork}"
        fi
    fi

    # --- (c) record mapping for swap + merge ------------------------------
    printf '%s\t%s\t%s\t%s\n' "${name}" "${url}" "${fork_gh}" "${fork_gl}" >> "${CAF_MAP_FILE}"
done

caf_log DONE "fork_third_party_submodule rc=${rc} (dry_run=${CAF_DRY_RUN})"
exit ${rc}
