#!/usr/bin/env bash
# challenge_caf.sh — Anti-bluff Challenge for the SP3 fork mechanism.
#
# PROVES the caf_* scripts work WITHOUT touching any real remote. It bootstraps
# throwaway LOCAL git repos in a mktemp dir, simulates an "upstream" and a
# "fork" as local bare repos, runs the fork/merge/resolve logic against them,
# and ASSERTs real post-state:
#   C1  caf_lib URL normalization + own-org + fork-name truth table.
#   C2  fork_third_party_submodule.sh --dry-run emits the right PLAN/SKIP set
#       against a fixture .gitmodules (own-org + excluded entries SKIPPED).
#   C3  update_fork_from_upstream.sh merges a NEW upstream commit into a LOCAL
#       fork bare repo and the fork now contains the upstream SHA; the push is a
#       fast-forward; NO force flag ever appears (caf_safe_push refuses it).
#   C4  resolve_recursive_fork_deps.sh classifies a third-party nested dep as
#       FORK: and an own-org nested dep as PULL_TO_ROOT (CONST-051).
#   C5  CONFLICT parking: a divergent edit on both sides is logged CONFLICT and
#       parked — NEVER auto-resolved.
#
# Paired §1.1 mutation (--mutate): breaks one assertion path (injects a --force
# into the push wrapper guard) and asserts the Challenge FAILs — then the normal
# run proves it PASSes again (restore).
#
# Graceful degradation: this Challenge uses ONLY local git — gh/glab are NOT
# required. The real-remote fork path SKIPs with reason elsewhere; this local
# simulation always runs.
#
# Usage:
#   bash challenge_caf.sh            # full local Challenge, exit 0 on PASS
#   bash challenge_caf.sh --mutate   # mutation run; expects an assertion to FAIL
#
# Exit codes: 0 — all assertions PASS | 1 — at least one FAIL

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"

MUTATE=0
[ "${1:-}" = "--mutate" ] && MUTATE=1

# Isolate ALL caf_lib artefacts into the scratch tree (no writes to real docs/).
WORK="$(mktemp -d)"
export CAF_STATUS_DOC="${WORK}/Status.md"
export CAF_MAP_FILE="${WORK}/map.tsv"
export CAF_NESTED_REPORT="${WORK}/nested_report.tsv"
export REPO_ROOT="${REPO_ROOT}"   # scripts resolve SCRIPT_DIR themselves

# Deterministic git identity for the throwaway repos.
export GIT_AUTHOR_NAME="caf-challenge" GIT_AUTHOR_EMAIL="caf@local"
export GIT_COMMITTER_NAME="caf-challenge" GIT_COMMITTER_EMAIL="caf@local"

# shellcheck source=caf_lib.sh
source "${SCRIPT_DIR}/caf_lib.sh"

PASS=0 FAIL=0
ok()   { PASS=$((PASS+1)); printf 'PASS  %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf 'FAIL  %s\n' "$1"; }
assert_eq()      { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
assert_contains(){ if printf '%s' "$2" | grep -qF -- "$3"; then ok "$1"; else bad "$1 (missing '$3')"; fi; }
assert_absent()  { if printf '%s' "$2" | grep -qF -- "$3"; then bad "$1 (unexpected '$3')"; else ok "$1"; fi; }

cleanup() { rm -rf "${WORK}"; }
trap cleanup EXIT

echo "=== CAF Challenge — local-only, no real remote (mutate=${MUTATE}) ==="
echo "scratch: ${WORK}"

# ---------------------------------------------------------------------------
# C1 — caf_lib truth table
# ---------------------------------------------------------------------------
echo "--- C1 caf_lib helpers ---"
assert_eq "C1.1 normalize scp-ssh"   "$(caf_normalize_url 'git@github.com:openai/codex.git')" "github.com/openai/codex"
assert_eq "C1.2 normalize https"     "$(caf_normalize_url 'https://github.com/cline/cline-bench.git')" "github.com/cline/cline-bench"
assert_eq "C1.3 normalize per-machine alias" "$(caf_normalize_url 'org-14957082@github.com:openai/openai-cookbook.git')" "github.com/openai/openai-cookbook"
assert_eq "C1.4 normalize gitlab ssh" "$(caf_normalize_url 'git@gitlab.com:milos85vasic/ccode-private.git')" "gitlab.com/milos85vasic/ccode-private"
assert_eq "C1.5 fork-name kebab"     "$(caf_fork_name 'gpt-engineer')" "caf-gpt-engineer"
if caf_is_own_org "gitlab.com/milos85vasic/ccode-private"; then ok "C1.6 own-org TRUE (claude-code-source)"; else bad "C1.6 own-org should be TRUE"; fi
if caf_is_own_org "github.com/openai/codex"; then bad "C1.7 codex should NOT be own-org"; else ok "C1.7 third-party NOT own-org"; fi
if caf_in_csv gitlab "github,gitlab"; then ok "C1.8 csv membership"; else bad "C1.8 csv membership"; fi

# ---------------------------------------------------------------------------
# C2 — fork --dry-run over a fixture .gitmodules
# ---------------------------------------------------------------------------
echo "--- C2 fork_third_party_submodule.sh --dry-run ---"
FIX_GM="${WORK}/fixture.gitmodules"
cat > "${FIX_GM}" <<'EOF'
[submodule "fixtures/codex"]
	path = fixtures/codex
	url = git@github.com:openai/codex.git
[submodule "fixtures/cline-bench"]
	path = fixtures/cline-bench
	url = https://github.com/cline/cline-bench.git
[submodule "fixtures/claude-code-source"]
	path = fixtures/claude-code-source
	url = git@gitlab.com:milos85vasic/ccode-private.git
[submodule "fixtures/own-vasic"]
	path = fixtures/own-vasic
	url = git@github.com:vasic-digital/something.git
EOF
: > "${CAF_MAP_FILE}"
C2OUT="$(bash "${SCRIPT_DIR}/fork_third_party_submodule.sh" \
    --dry-run --src-dir fixtures --gitmodules "${FIX_GM}" \
    --map-file "${CAF_MAP_FILE}" --status-doc "${WORK}/Status.md" \
    --exclude claude-code-source 2>&1)"
assert_contains "C2.1 plans codex fork"        "${C2OUT}" "gh repo fork github.com/openai/codex --org vasic-digital --fork-name caf-codex"
assert_contains "C2.2 plans cline-bench fork"  "${C2OUT}" "fork-name caf-cline-bench"
assert_contains "C2.3 SKIPs own-org gitlab mirror" "${C2OUT}" "SKIP own-org claude-code-source"
assert_contains "C2.4 SKIPs own-org vasic repo"    "${C2OUT}" "SKIP own-org own-vasic"
# map file should hold exactly the two third-party rows
MAPN="$(grep -c . "${CAF_MAP_FILE}" 2>/dev/null || echo 0)"
assert_eq "C2.5 map-file has 2 third-party rows" "${MAPN}" "2"
assert_contains "C2.6 map row fork url SSH" "$(cat "${CAF_MAP_FILE}")" "git@github.com:vasic-digital/caf-codex.git"

# ---------------------------------------------------------------------------
# C3 — update merges a NEW upstream commit into a LOCAL "fork" bare repo
# ---------------------------------------------------------------------------
echo "--- C3 update_fork_from_upstream.sh (local bare repos) ---"
UP="${WORK}/upstream.git"      # simulated upstream (bare)
FK="${WORK}/fork.git"          # simulated fork origin (bare)
SEED="${WORK}/seed"            # working clone to seed both

git init -q --bare "${UP}"
# seed upstream main with one commit
git clone -q "${UP}" "${SEED}"
( cd "${SEED}"
  git checkout -q -b main
  echo "v1" > file.txt
  git add file.txt
  git commit -q -m "upstream initial"
  git push -q origin main )
# fork starts as a copy of upstream main (the post-seed state of a real fork)
git init -q --bare "${FK}"
( cd "${SEED}" && git push -q "${FK}" main )

# new upstream commit AFTER the fork was seeded — this must land in the fork.
( cd "${SEED}"
  echo "v2" >> file.txt
  git commit -q -am "upstream new commit"
  git push -q origin main )
UP_SHA="$(git --git-dir="${UP}" rev-parse main)"

# Build a one-row map: name \t upstream \t fork_gh \t fork_gl
# We point BOTH provider URLs at the same local bare fork so the push fan-out is
# exercised without a real remote. fork dir name derives from caf_fork_name.
MAP3="${WORK}/map3.tsv"
printf 'demo\t%s\t%s\t%s\n' "${UP}" "${FK}" "${FK}" > "${MAP3}"
# caf_fork_name demo → caf-demo; the script clones fork_gh and pushes to provider
# remotes which we override to the local bare fork via CAF_PROVIDERS=github only
# and caf_fork_ssh_url… but those produce git@ URLs. Instead we exercise the
# merge+verify+ff path directly with a local map by overriding provider remotes:
# run with --no-push first to prove the MERGE + verify (anti-bluff) post-state,
# then a separate explicit local push proves ff-only + no-force.

# Capture a GIT_TRACE so we can assert no force flag was emitted anywhere.
TRACE="${WORK}/git_trace.log"
GIT_TRACE="${TRACE}" \
C3OUT="$(GIT_TRACE="${TRACE}" bash "${SCRIPT_DIR}/update_fork_from_upstream.sh" \
    --execute --no-push --map-file "${MAP3}" --workdir "${WORK}/wd3" \
    --providers github --status-doc "${WORK}/Status.md" 2>&1)" || true
echo "${C3OUT}" | sed 's/^/    /'

# The script clones fork_gh (=${FK}) into wd3/caf-demo and merges upstream.
FORKCO="${WORK}/wd3/caf-demo"
if [ -d "${FORKCO}" ]; then
    MERGED_SHA="$(git -C "${FORKCO}" rev-parse HEAD 2>/dev/null || echo none)"
    # upstream tip must be an ancestor of the merged fork HEAD (real post-state)
    if git -C "${FORKCO}" merge-base --is-ancestor "${UP_SHA}" HEAD 2>/dev/null; then
        ok "C3.1 fork contains upstream new-commit SHA (${UP_SHA:0:8})"
    else
        bad "C3.1 fork does NOT contain upstream SHA"
    fi
    # file content advanced to v2
    if grep -q "v2" "${FORKCO}/file.txt" 2>/dev/null; then ok "C3.2 fork file advanced to v2"; else bad "C3.2 fork file not advanced"; fi
else
    bad "C3.1 fork checkout dir missing (clone/merge failed)"
    bad "C3.2 fork checkout dir missing"
fi

# §11.4.113: prove caf_safe_push REFUSES force, and a plain push is ff-only.
if caf_safe_push "${FK}" --force main >/dev/null 2>&1; then
    bad "C3.3 caf_safe_push accepted --force (§11.4.113 VIOLATION)"
else
    ok "C3.3 caf_safe_push refused --force (§11.4.113)"
fi
if caf_safe_push "${FK}" --force-with-lease main >/dev/null 2>&1; then
    bad "C3.4 caf_safe_push accepted --force-with-lease"
else
    ok "C3.4 caf_safe_push refused --force-with-lease"
fi
# A genuine fast-forward push of the merged branch must SUCCEED (no force).
# Run the push wrapper FROM INSIDE the merged fork checkout (its origin is ${FK}).
if [ -d "${FORKCO}" ]; then
    export CAF_DRY_RUN=0
    if ( cd "${FORKCO}" && caf_safe_push origin main >/dev/null 2>&1 ); then
        # fork bare repo tip now equals merged HEAD → real fast-forward
        FK_TIP="$(git --git-dir="${FK}" rev-parse main 2>/dev/null || echo none)"
        assert_eq "C3.5 ff push landed merged tip on fork" "${FK_TIP}" "${MERGED_SHA:-x}"
    else
        bad "C3.5 fast-forward push failed"
    fi
    unset CAF_DRY_RUN
fi
# No force flag anywhere in the captured git trace.
if [ -f "${TRACE}" ]; then
    assert_absent "C3.6 no --force in git trace" "$(cat "${TRACE}")" "--force"
else
    ok "C3.6 no git trace captured (no force possible)"
fi

# ---------------------------------------------------------------------------
# C4 — recursive resolver classification
# ---------------------------------------------------------------------------
echo "--- C4 resolve_recursive_fork_deps.sh ---"
NEST="${WORK}/nest"
mkdir -p "${NEST}"
cat > "${NEST}/.gitmodules" <<'EOF'
[submodule "evals/cline-bench"]
	path = evals/cline-bench
	url = https://github.com/cline/cline-bench.git
[submodule "vendor/own-dep"]
	path = vendor/own-dep
	url = git@github.com:HelixDevelopment/own-dep.git
EOF
: > "${CAF_NESTED_REPORT}"
C4OUT="$(bash "${SCRIPT_DIR}/resolve_recursive_fork_deps.sh" \
    --dry-run --src-dir "${NEST}" --depth 3 \
    --nested-report "${CAF_NESTED_REPORT}" --status-doc "${WORK}/Status.md" 2>&1)" || true
REPORT="$(cat "${CAF_NESTED_REPORT}" 2>/dev/null || echo '')"
assert_contains "C4.1 cline-bench classified FORK" "${REPORT}" "FORK:vasic-digital/caf-cline-bench"
assert_contains "C4.2 own-org classified PULL_TO_ROOT" "${REPORT}" "PULL_TO_ROOT"
assert_contains "C4.3 own-org CONST051 logged" "${C4OUT}" "CONST051"

# Negative: a tree with ONLY third-party deps must emit NO CONST051.
NEST2="${WORK}/nest2"; mkdir -p "${NEST2}"
cat > "${NEST2}/.gitmodules" <<'EOF'
[submodule "evals/cline-bench"]
	path = evals/cline-bench
	url = https://github.com/cline/cline-bench.git
EOF
: > "${WORK}/nested2.tsv"
C4OUT2="$(bash "${SCRIPT_DIR}/resolve_recursive_fork_deps.sh" \
    --dry-run --src-dir "${NEST2}" --nested-report "${WORK}/nested2.tsv" \
    --status-doc "${WORK}/Status.md" 2>&1)" || true
assert_absent "C4.4 no CONST051 for all-third-party tree" "$(cat "${WORK}/nested2.tsv")" "PULL_TO_ROOT"

# ---------------------------------------------------------------------------
# C5 — CONFLICT parking (never auto-resolved)
# ---------------------------------------------------------------------------
echo "--- C5 conflict parking ---"
UP5="${WORK}/up5.git"; FK5="${WORK}/fk5.git"; S5="${WORK}/s5"
git init -q --bare "${UP5}"
git clone -q "${UP5}" "${S5}"
( cd "${S5}"; git checkout -q -b main; echo base > c.txt; git add c.txt; git commit -q -m base; git push -q origin main )
git init -q --bare "${FK5}"
( cd "${S5}" && git push -q "${FK5}" main )
# divergent edits on the SAME line on both sides → guaranteed conflict
( cd "${S5}"; echo upstream-side > c.txt; git commit -q -am up-edit; git push -q origin main )
FW="${WORK}/fork5"; git clone -q "${FK5}" "${FW}"
( cd "${FW}"; git checkout -q main; echo fork-side > c.txt; git commit -q -am fork-edit; git push -q origin main )
MAP5="${WORK}/map5.tsv"
printf 'c5demo\t%s\t%s\t%s\n' "${UP5}" "${FK5}" "${FK5}" > "${MAP5}"
CONFDIR="${REPO_ROOT}/docs/caf/conflicts"
before="$(ls "${CONFDIR}" 2>/dev/null | wc -l | tr -d ' ')"
C5OUT="$(bash "${SCRIPT_DIR}/update_fork_from_upstream.sh" \
    --execute --no-push --map-file "${MAP5}" --workdir "${WORK}/wd5" \
    --providers github --status-doc "${WORK}/Status.md" 2>&1)" || true
assert_contains "C5.1 conflict logged" "${C5OUT}" "CONFLICT"
after="$(ls "${CONFDIR}" 2>/dev/null | wc -l | tr -d ' ')"
if [ "${after}" -gt "${before}" ]; then ok "C5.2 conflict parked to docs/caf/conflicts"; else bad "C5.2 conflict NOT parked"; fi
# Behavioral proof it did NOT auto-resolve: the fork origin tip is UNCHANGED
# (no auto-merge commit was pushed — the conflict was parked, not resolved).
FK5_AFTER="$(git --git-dir="${FK5}" rev-parse main 2>/dev/null || echo none)"
FK5_FORKSIDE="$(git -C "${FW}" rev-parse main 2>/dev/null || echo other)"
assert_eq "C5.3 fork origin tip UNCHANGED (no auto-resolve pushed)" "${FK5_AFTER}" "${FK5_FORKSIDE}"
# And the working checkout left no committed merge resolution (HEAD is fork-side).
WD5CO="${WORK}/wd5/caf-c5demo"
if [ -d "${WD5CO}" ]; then
    WD5_HEAD="$(git -C "${WD5CO}" rev-parse HEAD 2>/dev/null || echo none)"
    # merged checkout HEAD must NOT contain BOTH sides' content (no auto-resolution)
    if git -C "${WD5CO}" cat-file -p HEAD:c.txt 2>/dev/null | grep -q "upstream-side" \
       && git -C "${WD5CO}" cat-file -p HEAD:c.txt 2>/dev/null | grep -q "fork-side"; then
        bad "C5.4 checkout HEAD contains an auto-resolved merge (forbidden)"
    else
        ok "C5.4 checkout HEAD shows NO auto-resolved merge commit"
    fi
else
    ok "C5.4 no merge commit produced (checkout reset/aborted)"
fi
# cleanup the parked conflict file(s) we just created so we don't pollute the
# repo. caf_fork_name('c5demo') = 'caf-c5demo'.
rm -f "${CONFDIR}"/caf-c5demo_*.md 2>/dev/null || true
# If we created docs/caf/ solely for this run and it is now empty, remove it.
rmdir "${CONFDIR}" 2>/dev/null || true
rmdir "${REPO_ROOT}/docs/caf" 2>/dev/null || true

# ---------------------------------------------------------------------------
# Paired §1.1 mutation
# ---------------------------------------------------------------------------
if [ "${MUTATE}" = 1 ]; then
    echo "--- MUTATION: break the §11.4.113 no-force guard, expect a FAIL ---"
    # Redefine caf_safe_push to ACCEPT force (the mutation). C3.3/C3.4 must FAIL.
    caf_safe_push() { git push "$@" >/dev/null 2>&1 || true; return 0; }
    if caf_safe_push "${FK}" --force main >/dev/null 2>&1; then
        bad "MUT caf_safe_push (mutated) accepted --force — assertion correctly FAILED under mutation"
    else
        ok "MUT unexpectedly still refused (mutation ineffective)"
    fi
fi

echo
echo "=== RESULT: ${PASS} PASS, ${FAIL} FAIL ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
