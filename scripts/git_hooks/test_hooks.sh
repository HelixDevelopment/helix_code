#!/usr/bin/env bash
# scripts/git_hooks/test_hooks.sh
# Hermetic anti-bluff test suite for the §11.4.75 git hooks.
#
# Runs every hook against a THROWAWAY temp git repo (mktemp -d), exercising
# real `git add` / `git commit` so the assertions reflect actual hook
# behaviour, not a re-grep of the source. Captures PASS/FAIL per case.
#
# Includes a paired §1.1 mutation: one hook assertion is deliberately
# broken, a case that should now FAIL is asserted to FAIL, then the hook is
# restored — proving the test genuinely exercises the invariant.
#
# Usage:   scripts/git_hooks/test_hooks.sh
# Exit:    0 iff every case passed; 1 otherwise.
# Side-effects: creates + removes a temp dir; touches no real repo state,
#   installs NO hooks into the real .git/hooks.
# Dependencies: git, bash, mktemp.
# Cross-references: §11.4.75 / §11.4.30 / §11.4.10 / §11.4.84 / §1.1;
#   scripts/git_hooks/{pre-commit,pre-push,post-commit,commit-msg}.

set -uo pipefail

HOOKS_SRC_DIR=$(cd "$(dirname "$0")" && pwd)

PASS=0
FAIL=0
declare -a FAILED_CASES=()

ok()   { PASS=$((PASS+1)); printf 'PASS  %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); FAILED_CASES+=("$1"); printf 'FAIL  %s\n' "$1"; }

# assert_block <case-name> <expected: BLOCK|ALLOW> <actual-exit>
assert_block() {
  local name="$1" expect="$2" rc="$3"
  if [ "$expect" = "BLOCK" ]; then
    if [ "$rc" -ne 0 ]; then ok "$name (blocked as expected)"; else bad "$name (expected BLOCK, hook allowed)"; fi
  else
    if [ "$rc" -eq 0 ]; then ok "$name (allowed as expected)"; else bad "$name (expected ALLOW, hook blocked rc=$rc)"; fi
  fi
}

# ---------------------------------------------------------------------------
# Build a throwaway repo with the hooks installed as real .git/hooks.
# ---------------------------------------------------------------------------
TMP=$(mktemp -d 2>/dev/null) || { echo "mktemp failed"; exit 1; }
cleanup() { rm -rf "$TMP" 2>/dev/null || true; }
trap cleanup EXIT

cd "$TMP"
git init -q
git config user.email "test@example.com"
git config user.name "Hook Test"
git config commit.gpgsign false

mkdir -p .git/hooks docs/audit
for h in pre-commit pre-push post-commit commit-msg; do
  cp "$HOOKS_SRC_DIR/$h" ".git/hooks/$h"
  chmod +x ".git/hooks/$h"
done

# Seed an initial commit so HEAD exists (post-commit / diffs need it).
echo "seed" > README.txt
git add README.txt
git commit -q -m "seed" 2>/dev/null

# Helper: attempt a commit, return rc.
try_commit() {
  git commit -q -m "$1" >/tmp/hk_out 2>&1
  echo $?
}

# ===========================================================================
# pre-commit — §11.4.75 governance-doc sibling check
# ===========================================================================

# 1. governance md WITH both siblings -> ALLOW
printf 'x\n' > CLAUDE.md; printf '<html>' > CLAUDE.html; printf '%%PDF' > CLAUDE.pdf
git add CLAUDE.md CLAUDE.html CLAUDE.pdf
assert_block "gov-md-with-both-siblings" ALLOW "$(try_commit 'add CLAUDE with siblings')"

# 2. governance md WITHOUT siblings -> BLOCK
printf 'x\n' > AGENTS.md
git add AGENTS.md
assert_block "gov-md-without-siblings" BLOCK "$(try_commit 'add AGENTS no siblings')"
git reset -q HEAD AGENTS.md; rm -f AGENTS.md

# 3. governance md with only .html (missing .pdf) -> BLOCK
printf 'x\n' > CONSTITUTION.md; printf '<html>' > CONSTITUTION.html
git add CONSTITUTION.md CONSTITUTION.html
assert_block "gov-md-missing-pdf-only" BLOCK "$(try_commit 'CONSTITUTION missing pdf')"
git reset -q HEAD CONSTITUTION.md CONSTITUTION.html; rm -f CONSTITUTION.md CONSTITUTION.html

# 4. tracker doc (docs/Issues.md) without siblings -> BLOCK
mkdir -p docs; printf 'x\n' > docs/Issues.md
git add docs/Issues.md
assert_block "tracker-issues-no-siblings" BLOCK "$(try_commit 'Issues no siblings')"
git reset -q HEAD docs/Issues.md; rm -f docs/Issues.md

# 5. tracker doc WITH siblings -> ALLOW
printf 'x\n' > docs/Fixed.md; printf '<html>' > docs/Fixed.html; printf '%%PDF' > docs/Fixed.pdf
git add docs/Fixed.md docs/Fixed.html docs/Fixed.pdf
assert_block "tracker-fixed-with-siblings" ALLOW "$(try_commit 'Fixed with siblings')"

# 6. Status.md under docs/<domain>/ without siblings -> BLOCK
mkdir -p docs/audio; printf 'x\n' > docs/audio/Status.md
git add docs/audio/Status.md
assert_block "status-doc-no-siblings" BLOCK "$(try_commit 'Status no siblings')"
git reset -q HEAD docs/audio/Status.md; rm -f docs/audio/Status.md

# 7. WORKING-SPEC md (docs/superpowers/**) without siblings -> ALLOW (md-only by convention)
mkdir -p docs/superpowers/plans; printf 'x\n' > docs/superpowers/plans/plan.md
git add docs/superpowers/plans/plan.md
assert_block "working-spec-superpowers-md-only" ALLOW "$(try_commit 'superpowers plan md-only')"

# 8. WORKING-SPEC md (docs/research/**) without siblings -> ALLOW
mkdir -p docs/research; printf 'x\n' > docs/research/note.md
git add docs/research/note.md
assert_block "working-spec-research-md-only" ALLOW "$(try_commit 'research note md-only')"

# 9. WORKING-SPEC md (docs/guides/**) without siblings -> ALLOW
mkdir -p docs/guides; printf 'x\n' > docs/guides/guide.md
git add docs/guides/guide.md
assert_block "working-spec-guides-md-only" ALLOW "$(try_commit 'guide md-only')"

# 9a. §11.4.65/CONST-066 reconciliation — working-spec md that ALREADY ships
#     a sibling (export-opted-in) but is missing the OTHER sibling -> BLOCK.
#     This is the widened scope: an export-opted-in doc updated without
#     regenerating both siblings is a sync regression and must be caught.
mkdir -p docs/research/sub
printf 'x\n' > docs/research/sub/opted.md
printf '<html>' > docs/research/sub/opted.html   # has .html, missing .pdf
git add docs/research/sub/opted.md docs/research/sub/opted.html
assert_block "export-optin-working-spec-missing-pdf" BLOCK "$(try_commit 'opted research missing pdf')"
git reset -q HEAD docs/research/sub/opted.md docs/research/sub/opted.html
rm -rf docs/research/sub

# 9b. §11.4.65/CONST-066 reconciliation — export-opted-in working-spec md
#     WITH BOTH siblings present -> ALLOW.
mkdir -p docs/research/sub2
printf 'x\n' > docs/research/sub2/full.md
printf '<html>' > docs/research/sub2/full.html
printf '%%PDF' > docs/research/sub2/full.pdf
git add docs/research/sub2/full.md docs/research/sub2/full.html docs/research/sub2/full.pdf
assert_block "export-optin-working-spec-with-both" ALLOW "$(try_commit 'opted research with both')"

# 10. ordinary source file (.go) -> ALLOW
printf 'package main\n' > main.go
git add main.go
assert_block "ordinary-source-file" ALLOW "$(try_commit 'add main.go')"

# 11. clean docs-only commit (non-governed README in subdir) -> ALLOW
mkdir -p pkg; printf '# pkg\n' > pkg/README.md
git add pkg/README.md
assert_block "non-governed-readme-md-only" ALLOW "$(try_commit 'pkg readme')"

# ===========================================================================
# pre-commit — §11.4.30 / §11.4.10 forbidden-class
# ===========================================================================

# 12. staged real .env -> BLOCK
printf 'SECRET=abc\n' > .env
git add -f .env
assert_block "staged-real-env" BLOCK "$(try_commit 'add env')"
git reset -q HEAD .env; rm -f .env

# 13. staged .env.example -> ALLOW (placeholder)
printf 'SECRET=\n' > .env.example
git add .env.example
assert_block "staged-env-example" ALLOW "$(try_commit 'add env.example')"

# 14. staged private key id_rsa -> BLOCK
printf 'PRIVATE\n' > id_rsa
git add -f id_rsa
assert_block "staged-id-rsa" BLOCK "$(try_commit 'add id_rsa')"
git reset -q HEAD id_rsa; rm -f id_rsa

# 15. staged *.pem credential -> BLOCK
printf 'CERT\n' > server.pem
git add -f server.pem
assert_block "staged-pem" BLOCK "$(try_commit 'add pem')"
git reset -q HEAD server.pem; rm -f server.pem

# 16. staged build artifact (*.so) -> BLOCK
printf 'ELF\n' > libfoo.so
git add -f libfoo.so
assert_block "staged-build-artifact-so" BLOCK "$(try_commit 'add so')"
git reset -q HEAD libfoo.so; rm -f libfoo.so

# ===========================================================================
# pre-commit — §11.4.84 mutation residue
# ===========================================================================

# 17. staged file with mutation marker -> BLOCK
printf 'func verify() bool { return true // always pass\n}\n' > auth.go
git add auth.go
assert_block "staged-mutation-residue" BLOCK "$(try_commit 'add mutated auth')"
git reset -q HEAD auth.go; rm -f auth.go

# 18. staged clean file (no marker) -> ALLOW
printf 'func verify() bool { return realCheck() }\n' > auth.go
git add auth.go
assert_block "staged-clean-no-residue" ALLOW "$(try_commit 'add clean auth')"

# ===========================================================================
# pre-commit — §11.4.84 mutation-test exemption (false-positive fix,
# 2026-07-11). A mutation-TEST script legitimately embeds a residue-marker
# string as its own trap-restored test logic and must NOT be blocked, while
# a real file that merely CLAIMS the exemption without the proven restore
# idiom must still be blocked (the exemption is explicit + auditable, not
# an abusable heuristic).
# ===========================================================================

# 18a. mutation-test file WITH exemption header + marker + proven trap+cp
#      restore idiom -> ALLOW (this is the real-world secret_scan_test.sh
#      shape reproduced as a minimal fixture).
cat > mutation_test_fixture.sh <<'FIXTURE'
#!/usr/bin/env bash
# §11.4.84-mutation-test-exempt: this file's markers are trap-restored test logic
BACKUP="$WORKDIR/target.orig-backup"
cleanup() { cp "$BACKUP" "$TARGET"; }
trap cleanup EXIT
# Replace the pattern (MUTATED for paired §1.1 mutation test — restored
# unconditionally by the EXIT trap above before this script returns).
FIXTURE
git add mutation_test_fixture.sh
assert_block "mutation-test-exempt-header-plus-idiom" ALLOW "$(try_commit 'add exempt mutation test fixture')"

# 18b. NON-test file with a BARE residue marker (no header, no idiom) ->
#      still BLOCKS (regression proof: the exemption never widens beyond
#      explicitly-marked files; unrelated real files keep tripping the
#      original detector unchanged).
printf 'func verify() bool { return true // always pass\n}\n' > auth_bare.go
git add auth_bare.go
assert_block "non-test-bare-marker-no-header-still-blocks" BLOCK "$(try_commit 'add bare-marker file')"
git reset -q HEAD auth_bare.go; rm -f auth_bare.go

# 18c. file CLAIMING the exemption header but WITHOUT the proven trap+cp
#      restore idiom -> still BLOCKS (closes the "just type the magic
#      comment" abuse case; the header alone is not sufficient).
cat > fake_exempt.sh <<'FIXTURE'
#!/usr/bin/env bash
# §11.4.84-mutation-test-exempt: this file's markers are trap-restored test logic
echo "MUTATED for paired residue with no real restore idiom"
FIXTURE
git add fake_exempt.sh
assert_block "fake-exempt-header-without-idiom-still-blocks" BLOCK "$(try_commit 'add fake-exempt file')"
git reset -q HEAD fake_exempt.sh; rm -f fake_exempt.sh

# 18d. §1.1 PAIRED MUTATION — neuter is_mutation_test_exempt() in a COPY of
#      the pre-commit hook (mutated hook always exempts, i.e. the idiom
#      check is bypassed), stage a fresh fake-exempt fixture (header
#      WITHOUT the real idiom), and assert it now WRONGLY passes -> proves
#      the idiom requirement in case 18c is load-bearing, not a tautology.
#      Then restore and assert a fresh fake-exempt fixture blocks again.
#      Distinct filenames are used for the pre-/post-restore fixtures so
#      each assertion is against a genuinely new staged file, never
#      ambiguous with "nothing to commit" from a prior identical blob.
MUT_HOOK84=".git/hooks/pre-commit"
cp "$MUT_HOOK84" "$MUT_HOOK84.bak84"
sed 's/^is_mutation_test_exempt() {/is_mutation_test_exempt() { return 0 #MUTATED for paired §1.1 mutation test/' \
  "$MUT_HOOK84.bak84" > "$MUT_HOOK84"
chmod +x "$MUT_HOOK84"

cat > fake_exempt_mut.sh <<'FIXTURE'
#!/usr/bin/env bash
# §11.4.84-mutation-test-exempt: this file's markers are trap-restored test logic
echo "MUTATED for paired residue with no real restore idiom"
FIXTURE
git add fake_exempt_mut.sh
mut84_rc=$(try_commit 'mutated: fake-exempt should now wrongly pass')
if [ "$mut84_rc" -eq 0 ]; then
  ok "paired-mutation-114-84: broken idiom-check no longer blocks fake-exempt (mutation detected)"
else
  bad "paired-mutation-114-84: hook still blocked despite broken idiom-check (mutation NOT detected -> test is blind)"
fi
git reset -q HEAD fake_exempt_mut.sh >/dev/null 2>&1 || true
rm -f fake_exempt_mut.sh
mv "$MUT_HOOK84.bak84" "$MUT_HOOK84"
chmod +x "$MUT_HOOK84"

cat > fake_exempt_check.sh <<'FIXTURE'
#!/usr/bin/env bash
# §11.4.84-mutation-test-exempt: this file's markers are trap-restored test logic
echo "MUTATED for paired residue with no real restore idiom"
FIXTURE
git add fake_exempt_check.sh
restored84_rc=$(try_commit 'restored: fresh fake-exempt should block again')
if [ "$restored84_rc" -ne 0 ]; then
  ok "paired-mutation-114-84: restored hook blocks fake-exempt again (invariant intact)"
else
  bad "paired-mutation-114-84: restored hook did NOT block fake-exempt (restore failed)"
fi
git reset -q HEAD fake_exempt_check.sh >/dev/null 2>&1 || true
rm -f fake_exempt_check.sh

# ===========================================================================
# commit-msg — §11.4.75 bypass-rationale footer
#
# NOTE on git semantics: `git commit --no-verify` skips ALL client hooks,
# commit-msg INCLUDED. So the enforceable bypass-audit path is a commit
# WRAPPER that sets HELIX_COMMIT_NO_VERIFY=1 (or the marker-absent case the
# pre-commit/commit-msg pair handles). We therefore drive the commit-msg
# hook the way a wrapper / the marker mechanism does, and also exercise it
# directly so the assertion reflects real hook behaviour, not a guess.
# ===========================================================================

run_commitmsg() {
  # $1 = message body, $2 = HELIX_COMMIT_NO_VERIFY value ("" = unset).
  # Returns the hook exit code. Ensures the one-shot marker is absent so the
  # marker-based bypass detection fires.
  printf '%s\n' "$1" > "$TMP/.cm_msg"
  rm -f .git/ATMO_PRECOMMIT_RAN 2>/dev/null || true
  if [ -n "$2" ]; then
    HELIX_COMMIT_NO_VERIFY="$2" bash .git/hooks/commit-msg "$TMP/.cm_msg" >/tmp/hk_cm 2>&1
  else
    bash .git/hooks/commit-msg "$TMP/.cm_msg" >/tmp/hk_cm 2>&1
  fi
  echo $?
}

# 19. bypass (env override) WITHOUT footer -> BLOCK
assert_block "bypass-without-footer" BLOCK "$(run_commitmsg 'bypass no footer' 1)"

# 20. bypass (env override) WITH footer -> ALLOW
assert_block "bypass-with-footer" ALLOW "$(run_commitmsg 'bypass with footer

Bypass-rationale: emergency mirror reconciliation approved by operator' 1)"

# 21. marker-absent (pre-commit skipped) WITHOUT footer -> BLOCK
assert_block "marker-absent-without-footer" BLOCK "$(run_commitmsg 'no footer, marker gone' '')"

# 22. normal commit (pre-commit ran -> marker present) needs no footer -> ALLOW
printf 'normal\n' > normal.txt
git add normal.txt
assert_block "normal-commit-no-footer-needed" ALLOW "$(try_commit 'normal commit')"

# ===========================================================================
# post-commit — log-only, never blocks (advisory)
# ===========================================================================

# 23. governed md committed via --no-verify (bypassing pre-commit sibling
#     gate; --no-verify skips commit-msg too so no footer is demanded) ->
#     commit lands; post-commit STILL runs (post-commit is NOT skipped by
#     --no-verify) and LOGS the orphan, exits 0, repo OK.
printf 'x\n' > QWEN.md
git add QWEN.md
git commit -q --no-verify -m "QWEN no siblings (post-commit logs)" >/tmp/hk_out4 2>&1
rc23=$?
if [ "$rc23" -eq 0 ] && git rev-parse HEAD >/dev/null 2>&1; then
  ok "post-commit-advisory-never-blocks (commit landed, exit 0)"
else
  bad "post-commit-advisory-never-blocks (commit rc=$rc23)"
fi
# Confirm post-commit actually logged the orphan
if grep -q "QWEN.md" docs/audit/postcommit_sibling_log.md 2>/dev/null; then
  ok "post-commit-logged-orphan-sibling"
else
  bad "post-commit-logged-orphan-sibling (no log entry)"
fi

# ===========================================================================
# install_git_hooks.sh --dry-run — run from the REAL repo root (the installer
# resolves its own repo via `git rev-parse`, so it must be invoked there, not
# inside this temp repo whose scripts/git_hooks does not exist).
# ===========================================================================
INSTALLER="$HOOKS_SRC_DIR/../install_git_hooks.sh"
REAL_ROOT=$(cd "$HOOKS_SRC_DIR" && git rev-parse --show-toplevel 2>/dev/null || echo "")
if [ -f "$INSTALLER" ] && [ -n "$REAL_ROOT" ]; then
  out=$(cd "$REAL_ROOT" && bash "$INSTALLER" --dry-run 2>&1)
  drc=$?
  if [ "$drc" -eq 0 ] && printf '%s' "$out" | grep -qi 'DRY-RUN'; then
    ok "installer-dry-run-prints-plan-no-mutation"
  else
    bad "installer-dry-run-prints-plan-no-mutation (rc=$drc out=$out)"
  fi
else
  bad "installer-dry-run-prints-plan-no-mutation (installer missing at $INSTALLER)"
fi

# ===========================================================================
# §1.1 PAIRED MUTATION — break the sibling assertion, prove a case FAILs,
# then restore. We mutate a COPY of the pre-commit hook installed in the
# temp repo so the real source is never touched.
# ===========================================================================
MUT_HOOK=".git/hooks/pre-commit"
cp "$MUT_HOOK" "$MUT_HOOK.bak"
# Mutation: make is_governed_md always return 1 (nothing is governed) so the
# governance-md-without-siblings case would (wrongly) ALLOW.
#   replace the function body's first `case` with an unconditional `return 1`.
sed 's/^is_governed_md() {/is_governed_md() { return 1 #MUTATED/' "$MUT_HOOK.bak" > "$MUT_HOOK"
chmod +x "$MUT_HOOK"

# Re-run case 2 under the mutated hook: governance md without siblings.
printf 'x\n' > MUTCHK.md
# MUTCHK is not in the governed set anyway; use a real governed name.
rm -f MUTCHK.md
printf 'x\n' > AGENTS.md
git add AGENTS.md
mut_rc=$(try_commit 'mutated: AGENTS no siblings should now wrongly pass')
# Under the mutation the hook NO LONGER blocks -> rc 0. The paired-mutation
# assertion: with the assertion broken, the BLOCK case is NOT blocked.
if [ "$mut_rc" -eq 0 ]; then
  ok "paired-mutation: broken sibling-assertion no longer blocks (mutation detected)"
else
  bad "paired-mutation: hook still blocked despite broken assertion (mutation NOT detected -> test is blind)"
fi
# Restore the hook + repo state.
git reset -q --hard HEAD >/dev/null 2>&1
mv "$MUT_HOOK.bak" "$MUT_HOOK"
chmod +x "$MUT_HOOK"

# Sanity: restored hook blocks again (the GREEN side of the pair).
printf 'x\n' > AGENTS.md
git add AGENTS.md
restored_rc=$(try_commit 'restored: AGENTS no siblings should block again')
if [ "$restored_rc" -ne 0 ]; then
  ok "paired-mutation: restored hook blocks again (invariant intact)"
else
  bad "paired-mutation: restored hook did NOT block (restore failed)"
fi
git reset -q HEAD AGENTS.md >/dev/null 2>&1; rm -f AGENTS.md

# ===========================================================================
# Summary
# ===========================================================================
echo "----------------------------------------------------------------"
echo "RESULT: $PASS PASS / $FAIL FAIL"
if [ "$FAIL" -gt 0 ]; then
  echo "Failed cases:"
  for c in "${FAILED_CASES[@]}"; do echo "  - $c"; done
  exit 1
fi
exit 0
