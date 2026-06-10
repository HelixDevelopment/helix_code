#!/usr/bin/env bash
# caf_lib.sh — Shared helpers for the SP3 "fork-ALL cli_agents" mechanism.
#
# Sourced by:
#   fork_third_party_submodule.sh
#   update_fork_from_upstream.sh
#   resolve_recursive_fork_deps.sh
#   caf_validate.sh
#
# Design (§11.4 anti-bluff): pure helper functions, no side effects beyond
# logging. NOTHING is hardcoded — org / prefix / src-dir / providers / branch /
# depth are all parameterized via caf_parse_args (flags) or CAF_* env vars.
#
# Binding constraints cited inline:
#   §11.4.113 — absolute no-force-push (callers never emit --force / +ref).
#   §11.4.30 / CONST-053 — minimal-file commits only (callers' concern).
#   §11.4.28(B) / CONST-051 — forks stay decoupled / project-not-aware.
#   Rule 3 — SSH URLs only; this lib normalizes & re-emits SSH.
#
# This file defines functions and parameter defaults ONLY. It performs no
# external mutation when sourced.

# ---------------------------------------------------------------------------
# Parameter defaults (every one overridable by flag OR CAF_* env var).
# ---------------------------------------------------------------------------
: "${CAF_ORG:=vasic-digital}"
: "${CAF_PREFIX:=caf-}"
: "${CAF_SRC_DIR:=cli_agents}"
: "${CAF_PROVIDERS:=github,gitlab}"
: "${CAF_GITLAB_GROUP:=vasic-digital}"
: "${CAF_BRANCH:=}"            # empty → auto-detect remote HEAD (main|master)
: "${CAF_DRY_RUN:=1}"         # DEFAULT dry-run: mutate nothing unless told to
: "${CAF_ONLY:=}"
: "${CAF_EXCLUDE:=claude-code-source}"
: "${CAF_RECURSIVE:=0}"
: "${CAF_DEPTH:=3}"
: "${CAF_WORKDIR:=}"
: "${CAF_PUSH:=1}"
: "${CAF_STRATEGY:=merge}"    # merge (union-preserving) | ff-only
: "${CAF_VISIBILITY:=private}"

# Own-org owner set (CONST-051). The operator's own GitLab namespace is included
# so claude-code-source's GitLab mirror is recognised as own / skip-worthy.
: "${CAF_OWN_ORGS:=vasic-digital HelixDevelopment red-elf ATMOSphere1234321 Bear-Suite BoatOS123456 Helix-Flow Helix-Track Server-Factory milos85vasic}"

# Resolved lazily by callers that need them (kept overridable for the Challenge).
# CAF_GITMODULES / CAF_MAP_FILE / CAF_STATUS_DOC / CAF_NESTED_REPORT default
# relative to a REPO_ROOT the caller sets before sourcing OR we derive here.
if [ -z "${REPO_ROOT:-}" ]; then
    _caf_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "${_caf_self}/../.." && pwd)"
fi
: "${CAF_GITMODULES:=${REPO_ROOT}/.gitmodules}"
: "${CAF_MAP_FILE:=${REPO_ROOT}/docs/caf/map.tsv}"
: "${CAF_STATUS_DOC:=${REPO_ROOT}/docs/caf/Status.md}"
: "${CAF_NESTED_REPORT:=${REPO_ROOT}/docs/caf/nested_report.tsv}"

# ---------------------------------------------------------------------------
# caf_log <LEVEL> <args...>
# Append "TS LEVEL args" to the Status ledger + echo to stdout. Append-only.
# ---------------------------------------------------------------------------
caf_log() {
    local level="$1"; shift
    local ts; ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    local line="${ts} ${level} $*"
    if [ -n "${CAF_STATUS_DOC:-}" ]; then
        mkdir -p "$(dirname "${CAF_STATUS_DOC}")" 2>/dev/null || true
        printf '%s\n' "${line}" >> "${CAF_STATUS_DOC}" 2>/dev/null || true
    fi
    printf '%s\n' "${line}"
}

# ---------------------------------------------------------------------------
# caf_in_csv <needle> <csv>
# Membership test for --only / --exclude / --providers (comma-separated).
# Returns 0 if present, 1 otherwise. Empty csv → not a member.
# ---------------------------------------------------------------------------
caf_in_csv() {
    local needle="$1" csv="$2" item
    [ -z "${csv}" ] && return 1
    local IFS=','
    for item in ${csv}; do
        # trim surrounding whitespace
        item="${item#"${item%%[![:space:]]*}"}"
        item="${item%"${item##*[![:space:]]}"}"
        [ "${item}" = "${needle}" ] && return 0
    done
    return 1
}

# ---------------------------------------------------------------------------
# caf_normalize_url <url>
# Strip per-machine SSH alias (org-NNNN@host), host-normalize, strip trailing
# .git, emit canonical "host/owner/repo". Handles:
#   git@github.com:owner/repo.git
#   https://github.com/owner/repo.git
#   org-14957082@github.com:owner/repo.git   (per-machine alias → github.com)
#   ssh://git@gitlab.com/owner/repo.git
# Echoes the canonical "host/owner/repo" (lowercased host only).
# ---------------------------------------------------------------------------
caf_normalize_url() {
    local url="$1" rest host ownerrepo
    # Strip ssh:// scheme if present.
    url="${url#ssh://}"
    # Strip https:// / http:// scheme.
    url="${url#https://}"
    url="${url#http://}"
    # Strip a leading "user@" (incl. per-machine alias org-NNNN@ and git@).
    # Everything up to and including the first '@' is the user portion.
    if [[ "${url}" == *"@"* ]]; then
        rest="${url#*@}"
    else
        rest="${url}"
    fi
    # rest is now host[:/]owner/repo[.git]. Separator after host is ':' (scp form)
    # or '/' (url form). Split host from the path.
    if [[ "${rest}" == *":"* ]]; then
        host="${rest%%:*}"
        ownerrepo="${rest#*:}"
    else
        host="${rest%%/*}"
        ownerrepo="${rest#*/}"
    fi
    # Strip trailing .git and any trailing slash.
    ownerrepo="${ownerrepo%.git}"
    ownerrepo="${ownerrepo%/}"
    # Lowercase host only (owner/repo case is significant for git hosts).
    host="$(printf '%s' "${host}" | tr '[:upper:]' '[:lower:]')"
    printf '%s/%s' "${host}" "${ownerrepo}"
}

# ---------------------------------------------------------------------------
# caf_url_owner <normalized>   → owner portion of host/owner/repo
# caf_url_repo  <normalized>   → repo  portion
# ---------------------------------------------------------------------------
caf_url_owner() {
    local norm="$1" path
    path="${norm#*/}"        # drop host
    printf '%s' "${path%%/*}"
}
caf_url_repo() {
    local norm="$1"
    printf '%s' "${norm##*/}"
}

# ---------------------------------------------------------------------------
# caf_is_own_org <normalized-url>
# TRUE (exit 0) if the owner ∈ CAF_OWN_ORGS. Used to SKIP top-level forks
# (e.g. claude-code-source) and to flag CONST-051 for nested own-org chains.
# ---------------------------------------------------------------------------
caf_is_own_org() {
    local norm="$1" owner org
    owner="$(caf_url_owner "${norm}")"
    for org in ${CAF_OWN_ORGS}; do
        [ "${owner}" = "${org}" ] && return 0
    done
    return 1
}

# ---------------------------------------------------------------------------
# caf_fork_name <submodule-dir-name>
# fork name = <CAF_PREFIX><dir-name>. Preserves the kebab submodule DIRECTORY
# name (per plan §3.3 naming policy / CONST-052), NOT the upstream CamelCase
# repo name.
# ---------------------------------------------------------------------------
caf_fork_name() {
    printf '%s%s' "${CAF_PREFIX}" "$1"
}

# ---------------------------------------------------------------------------
# caf_fork_ssh_url <provider> <fork-name>
# Emit the SSH (Rule 3) fork URL for a provider. provider ∈ {github,gitlab}.
# ---------------------------------------------------------------------------
caf_fork_ssh_url() {
    local provider="$1" fork="$2"
    case "${provider}" in
        github) printf 'git@github.com:%s/%s.git' "${CAF_ORG}" "${fork}" ;;
        gitlab) printf 'git@gitlab.com:%s/%s.git' "${CAF_GITLAB_GROUP}" "${fork}" ;;
        *)      printf 'git@%s:%s/%s.git' "${provider}" "${CAF_ORG}" "${fork}" ;;
    esac
}

# ---------------------------------------------------------------------------
# caf_provider_remotes <fork-name>
# Expand the --providers csv to the set of SSH fork remote URLs (one per line).
# ---------------------------------------------------------------------------
caf_provider_remotes() {
    local fork="$1" p
    local IFS=','
    for p in ${CAF_PROVIDERS}; do
        p="${p#"${p%%[![:space:]]*}"}"; p="${p%"${p##*[![:space:]]}"}"
        [ -z "${p}" ] && continue
        caf_fork_ssh_url "${p}" "${fork}"
    done
}

# ---------------------------------------------------------------------------
# caf_detect_default_branch <remote-name-or-url>
# Resolve the default branch (main|master|...) via the remote's symbolic HEAD.
# Falls back to "main" only if the remote is unreachable (logged by caller).
# Echoes the branch name; exit 0 on resolve, 1 on fallback.
# ---------------------------------------------------------------------------
caf_detect_default_branch() {
    local target="$1" symref branch
    if [ -n "${CAF_BRANCH}" ]; then
        printf '%s' "${CAF_BRANCH}"; return 0
    fi
    symref="$(git ls-remote --symref "${target}" HEAD 2>/dev/null | awk '/^ref:/{print $2}')"
    if [ -n "${symref}" ]; then
        branch="${symref#refs/heads/}"
        printf '%s' "${branch}"; return 0
    fi
    printf 'main'; return 1
}

# ---------------------------------------------------------------------------
# caf_assert_no_force <args...>
# §11.4.113 guard: scan a command's args for any force/overwrite flag and
# REFUSE (exit non-zero) if present. Callers run pushes THROUGH this so a
# force can never slip in. Used by the anti-bluff mutation in the Challenge.
# Returns 0 if safe, 2 if a force flag is detected.
# ---------------------------------------------------------------------------
caf_assert_no_force() {
    local a
    for a in "$@"; do
        case "${a}" in
            --force|-f|--force-with-lease|--force-with-lease=*|--mirror)
                caf_log FORCE-BLOCKED "§11.4.113 refusing force-class arg: ${a}"
                return 2 ;;
            +*:*|+refs/*)
                caf_log FORCE-BLOCKED "§11.4.113 refusing refspec overwrite: ${a}"
                return 2 ;;
        esac
    done
    return 0
}

# ---------------------------------------------------------------------------
# caf_safe_push <remote> <refspec...>
# §11.4.113-safe push wrapper: blocks force-class args, then performs a plain
# fast-forward push. Honours CAF_DRY_RUN.
# ---------------------------------------------------------------------------
caf_safe_push() {
    local remote="$1"; shift
    if ! caf_assert_no_force "$@"; then
        return 2
    fi
    if [ "${CAF_DRY_RUN}" = 1 ]; then
        caf_log PLAN "git push ${remote} $*"
        return 0
    fi
    git push "${remote}" "$@"
}

# ---------------------------------------------------------------------------
# caf_parse_args <args...>
# Fills the CAF_* globals from flags. Unknown flags are passed through to
# CAF_REST (positional remainder) so callers can extend.
# ---------------------------------------------------------------------------
caf_parse_args() {
    CAF_REST=()
    while [ $# -gt 0 ]; do
        case "$1" in
            --org)         CAF_ORG="$2"; shift 2 ;;
            --prefix)      CAF_PREFIX="$2"; shift 2 ;;
            --src-dir)     CAF_SRC_DIR="$2"; shift 2 ;;
            --gitmodules)  CAF_GITMODULES="$2"; shift 2 ;;
            --providers)   CAF_PROVIDERS="$2"; shift 2 ;;
            --gitlab-group) CAF_GITLAB_GROUP="$2"; shift 2 ;;
            --branch)      CAF_BRANCH="$2"; shift 2 ;;
            --only)        CAF_ONLY="$2"; shift 2 ;;
            --exclude)     CAF_EXCLUDE="$2"; shift 2 ;;
            --depth)       CAF_DEPTH="$2"; shift 2 ;;
            --workdir)     CAF_WORKDIR="$2"; shift 2 ;;
            --map-file)    CAF_MAP_FILE="$2"; shift 2 ;;
            --status-doc)  CAF_STATUS_DOC="$2"; shift 2 ;;
            --nested-report) CAF_NESTED_REPORT="$2"; shift 2 ;;
            --strategy)    CAF_STRATEGY="$2"; shift 2 ;;
            --visibility)  CAF_VISIBILITY="$2"; shift 2 ;;
            --recursive)   CAF_RECURSIVE=1; shift ;;
            --dry-run)     CAF_DRY_RUN=1; shift ;;
            --no-dry-run|--execute) CAF_DRY_RUN=0; shift ;;
            --push)        CAF_PUSH=1; shift ;;
            --no-push)     CAF_PUSH=0; shift ;;
            --)            shift; while [ $# -gt 0 ]; do CAF_REST+=("$1"); shift; done ;;
            *)             CAF_REST+=("$1"); shift ;;
        esac
    done
}

# ---------------------------------------------------------------------------
# caf_have <cmd>  → 0 if on PATH, else 1
# ---------------------------------------------------------------------------
caf_have() { command -v "$1" >/dev/null 2>&1; }
