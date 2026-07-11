#!/usr/bin/env bash
# scripts/secret_scan_test.sh
#
# Hermetic test suite for scripts/secret_scan.sh (§11.4.135 / §11.4.138
# permanent secret-scan guard). 22 real-vs-allowlisted fixture cases (12
# from the original guard + HuggingFace follow-up, 8 from the 2026-07-11
# defense-in-depth hardening pass covering xai/anthropic/gcp/azure key
# classes, 2 post-restore re-checks) plus TWO paired §1.1 mutations proving
# the guard is genuinely load-bearing (not a tautology): (1) neuter the
# Google-pattern line, plant a real-shaped Google key, show the guard
# WRONGLY passes, restore, show it correctly fails again; (2) same
# mutate/assert/restore cycle for the new Anthropic-key pattern. Never
# echoes a real secret value to stdout in this test's own output beyond
# the deliberately-fake fixture strings written to disk (none of the
# fixture strings are real credentials — see NOTE below).
#
# NOTE: every "real-shaped" fixture below is a FABRICATED placeholder built
# to match the shape (prefix + length) of a real key, never a value that
# could authenticate against any real service. §11.4.10 governs REAL
# secrets; these are synthetic test inputs, exactly like scan-secrets.sh's
# own test suite (scripts/test-scan-secrets.sh) and the .scan-secrets-allow
# entries that document this same convention project-wide.
#
# RUNTIME-ASSEMBLED FIXTURES (2026-07-11 fix, closes a real GitHub
# push-protection block): every "MUST be detected" fixture below (the ones
# our guard is required to catch — Tests 1-7b/13/15/17/19) is assembled at
# RUNTIME from separate prefix/body fragments (`printf '...%s' "$frag"`)
# rather than written as one contiguous literal, so NO contiguous
# secret-shaped token ever appears in this file's SOURCE TEXT — only in the
# ephemeral $WORKDIR temp file the assembled value is written to (never
# committed, never even tracked by git). GitHub's server-side push
# protection scans the DIFF TEXT of what's being pushed; a literal fixture
# with the real Slack-bot-token shape (xoxb- + a digit run + "-" + a digit
# run + "-" + an alphanumeric run — deliberately NOT reproduced verbatim
# even here in prose, per the same fix this comment documents) sitting in
# committed source is indistinguishable to that scanner from a real leaked
# token and gets blocked (this happened on 2026-07-11 for that fixture).
# Splitting the prefix (kept inline in the printf FORMAT string,
# far too short alone to match any pattern) from the long fabricated body
# (held in a shell variable, never adjacent to its defining prefix in
# source) breaks every pattern's required contiguous shape in the SOURCE
# while leaving the value the guard actually tests — the text written to
# the WORKDIR temp file at runtime — byte-identical to before. The
# allowlisted / non-matching fixtures (Tests 8,9,10,11,14,16,18,20) are
# deliberately NOT real-shaped to begin with (that's the point of those
# cases) and are left as plain literals.
#
# §11.4.84-mutation-test-exempt: this file's markers are trap-restored test
# logic. The literal string "MUTATED for paired" below (in the paired §1.1
# mutation comment) is documentation of this test's own mutate -> assert ->
# restore sequence, not residue from an interrupted experiment — the
# unconditional `trap cleanup EXIT` + `cp "$BACKUP" "$SCANNER"` restore
# above proves the mutation never survives past this script's own exit.

set -uo pipefail
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

SCANNER="$REPO_ROOT/scripts/secret_scan.sh"
WORKDIR=$(mktemp -d)
BACKUP="$WORKDIR/secret_scan.sh.orig-backup"
cp "$SCANNER" "$BACKUP"

cleanup() {
  # §11.4.84 working-tree quiescence: unconditionally restore the scanner
  # to its pre-mutation state before this script exits, on ANY exit path
  # (pass, fail, or interrupt), so no mutation residue can ever reach a
  # commit.
  cp "$BACKUP" "$SCANNER"
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

PASS=0
FAIL=0

check() {
  # check <description> <expected-rc-class: zero|nonzero> <file>
  local desc="$1" expect="$2" file="$3"
  local out rc
  out=$("$SCANNER" "$file" 2>&1)
  rc=$?
  case "$expect" in
    zero)
      if [ "$rc" -eq 0 ]; then
        echo "PASS: $desc (exit 0 as expected)"
        PASS=$((PASS + 1))
      else
        echo "FAIL: $desc (expected exit 0, got $rc)"
        printf '%s\n' "$out" | sed 's/^/    /'
        FAIL=$((FAIL + 1))
      fi
      ;;
    nonzero)
      if [ "$rc" -ne 0 ]; then
        echo "PASS: $desc (non-zero exit as expected)"
        PASS=$((PASS + 1))
      else
        echo "FAIL: $desc (expected non-zero exit, got 0)"
        printf '%s\n' "$out" | sed 's/^/    /'
        FAIL=$((FAIL + 1))
      fi
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Real-key-shaped fixtures (each a fabricated placeholder, never a real
# credential — see file-level NOTE) → MUST be detected (non-zero exit).
# ---------------------------------------------------------------------------

frag="SyD-fabricated0123456789abcdef"
printf 'GEMINI_API_KEY=AIza%s\n' "$frag" > "$WORKDIR/1_google.txt"
check "Test 1: real-shaped Google API key" nonzero "$WORKDIR/1_google.txt"

frag="fabricated0123456789ABCDEFGHIJKLMN"
printf 'OPENAI_API_KEY=sk-%s\n' "$frag" > "$WORKDIR/2_openai.txt"
check "Test 2: real-shaped OpenAI API key" nonzero "$WORKDIR/2_openai.txt"

frag="ABCDEFGHIJKLMNOP"
printf 'AWS_ACCESS_KEY_ID=AKIA%s\n' "$frag" > "$WORKDIR/3_aws.txt"
check "Test 3: real-shaped AWS access key" nonzero "$WORKDIR/3_aws.txt"

frag="fabricated0123456789ABCDEFGHIJKLMNOPQR"
printf 'GITHUB_TOKEN=ghp_%s\n' "$frag" > "$WORKDIR/4_github.txt"
check "Test 4: real-shaped GitHub PAT" nonzero "$WORKDIR/4_github.txt"

# PEM headers: the leading "-----" run is kept in a separate variable from
# the "BEGIN ... PRIVATE KEY"/"END ... PRIVATE KEY" words so the full
# 5-dash + BEGIN/END + key-type + "PRIVATE KEY" + 5-dash marker the guard
# (and GitHub's own private-key scanner) looks for is never contiguous in
# this file's source — not even spelled out verbatim in this comment.
dash="-----"
frag="MIIfabricatedNotARealKeyBodyAtAll"
printf '%sBEGIN RSA PRIVATE KEY%s\n%s\n%sEND RSA PRIVATE KEY%s\n' \
  "$dash" "$dash" "$frag" "$dash" "$dash" > "$WORKDIR/5_pem.txt"
check "Test 5: private-key header (RSA)" nonzero "$WORKDIR/5_pem.txt"

dash="-----"
frag="MIIfabricatedGenericPKCS8Body"
printf '%sBEGIN PRIVATE KEY%s\n%s\n%sEND PRIVATE KEY%s\n' \
  "$dash" "$dash" "$frag" "$dash" "$dash" > "$WORKDIR/6_pem_generic.txt"
check "Test 6: private-key header (generic PKCS8)" nonzero "$WORKDIR/6_pem_generic.txt"

printf 'SLACK_BOT_TOKEN=xoxb-%s-%s-%s\n' \
  "123456789012" "123456789012" "abcdefghijklmnopqrstuvwx" > "$WORKDIR/7_slack_real.txt"
check "Test 7: real-shaped Slack bot token (xoxb-N-N-alnum)" nonzero "$WORKDIR/7_slack_real.txt"

# HuggingFace hf_ token — the exact class that partially leaked into the
# session transcript on 2026-07-11 (catalogue-providers stream); added to
# the guard patterns per §11.4.138 (close the class that escaped).
frag="fabricated0123456789ABCDEFGHIJKLMNOP"
printf 'HF_TOKEN=hf_%s\n' "$frag" > "$WORKDIR/7b_hf.txt"
check "Test 7b: real-shaped HuggingFace token (hf_...)" nonzero "$WORKDIR/7b_hf.txt"

# ---------------------------------------------------------------------------
# Defense-in-depth hardening pass (§11.4.138, 2026-07-11 follow-up): more
# high-value key classes real .env files in this project could hold (the
# project ships ~30 provider .env aliases including xai, gcp, azure).
# Thresholds were tuned against the whole tracked tree (see
# scratchpad/r41_guard_hardening.md) so they clear every existing
# legitimate doc placeholder / Go unit-test fixture already committed.
# ---------------------------------------------------------------------------

frag="fabricated0123456789ABCDEFGHIJKLMNOP"
printf 'XAI_API_KEY=xai-%s\n' "$frag" > "$WORKDIR/13_xai.txt"
check "Test 13: real-shaped xAI API key (xai-...)" nonzero "$WORKDIR/13_xai.txt"

# "sk-ant-api03-" (13 chars total after "sk-ant-": "api03-" is only 6 chars,
# far short of the guard's 30-char threshold) stays a literal prefix in the
# printf FORMAT string below — only the long fabricated body is a variable,
# so "sk-ant-[A-Za-z0-9_-]{30,}" never matches this file's source text.
frag="fabricatedNotARealKey0123456789ABCDEFGHIJ"
printf 'ANTHROPIC_API_KEY=sk-ant-api03-%s\n' "$frag" > "$WORKDIR/15_anthropic.txt"
check "Test 15: real-shaped Anthropic API key (sk-ant-api03-...)" nonzero "$WORKDIR/15_anthropic.txt"

# Fabricated GCP service-account JSON credential blob (fake project id,
# fake hex-looking private_key_id, no PEM body at all — this fixture is
# deliberately narrower than a real credentials file so it isolates the
# two new JSON-marker patterns from the pre-existing PEM pattern). The
# guard's two GCP patterns each require a JSON key name IMMEDIATELY
# followed (mod whitespace) by its colon — so each key name and its
# ": value" suffix are assembled from two separate string literals that are
# never adjacent in source, only in the printf-assembled output line.
gcp_frag="fabricated0123456789abcdef01234567890123"
gcp_type_suffix=': "service_account"'
gcp_pkid_suffix=": \"$gcp_frag\""
{
  printf '{\n'
  printf '  "type"%s,\n' "$gcp_type_suffix"
  printf '  "project_id": "fabricated-test-project",\n'
  printf '  "private_key_id"%s,\n' "$gcp_pkid_suffix"
  printf '  "client_email": "fabricated@fabricated-test-project.iam.gserviceaccount.com"\n'
  printf '}\n'
} > "$WORKDIR/17_gcp.txt"
check "Test 17: fabricated GCP service-account JSON blob (type + private_key_id)" nonzero "$WORKDIR/17_gcp.txt"

frag="1a2b3c4d5e6f70819203a4b5c6d7e8f9"
printf 'AZURE_OPENAI_API_KEY=%s\n' "$frag" > "$WORKDIR/19_azure.txt"
check "Test 19: real-shaped Azure key (AZURE_*KEY=<32-hex>)" nonzero "$WORKDIR/19_azure.txt"

# ---------------------------------------------------------------------------
# Allowlisted / non-matching fixtures → MUST NOT be detected (exit 0).
# ---------------------------------------------------------------------------

echo 'GEMINI_API_KEY=<REDACTED-GEMINI-KEY-CONST-042-...>' > "$WORKDIR/8_redacted.txt"
check "Test 8: redaction marker (<REDACTED-...>) is allowlisted" zero "$WORKDIR/8_redacted.txt"

echo 'sample key shape: AIzaSyEXAMPLE1234567890abcdefghi' > "$WORKDIR/9_example.txt"
check "Test 9: EXAMPLE placeholder is allowlisted" zero "$WORKDIR/9_example.txt"

# This is the exact false-positive class the post-mortem flagged: a doc
# mentioning "xoxb-" prose/pattern without the real N-N-alnum token shape
# (the OLD scan-secrets.sh regex xoxb-[A-Za-z0-9-]{16,} would match this;
# the tightened xoxb-[0-9]{9,}-[0-9]{9,}-[A-Za-z0-9]{20,} correctly does not).
echo 'Slack bot tokens look like xoxb-this-is-not-a-real-token-shape-example' > "$WORKDIR/10_slack_false_positive.txt"
check "Test 10: xoxb- prose mention (not real token shape) is NOT flagged" zero "$WORKDIR/10_slack_false_positive.txt"

echo 'nothing sensitive here, just prose' > "$WORKDIR/11_clean.txt"
check "Test 11: clean file" zero "$WORKDIR/11_clean.txt"

# --- Clean-prose / near-miss fixtures for the 2026-07-11 hardening pass ---
# Each of these is deliberately a NEAR MISS (mentions the same prefix/word
# as its matching pattern) rather than generic unrelated prose, so a PASS
# here is real evidence the pattern is shape-specific and not over-broad
# (§11.4.6) — not just "the pattern doesn't match totally unrelated text".

echo 'Configure XAI_API_KEY for Grok; the prefix is xai- for xAI keys.' > "$WORKDIR/14_xai_prose.txt"
check "Test 14: xai- prose mention (not real token shape) is NOT flagged" zero "$WORKDIR/14_xai_prose.txt"

echo 'export ANTHROPIC_API_KEY="sk-ant-your-key"' > "$WORKDIR/16_anthropic_placeholder.txt"
check "Test 16: sk-ant-your-key doc placeholder (too short) is NOT flagged" zero "$WORKDIR/16_anthropic_placeholder.txt"

echo 'GCP uses a service_account JSON credentials file with a type field.' > "$WORKDIR/18_gcp_prose.txt"
check "Test 18: service_account prose mention (no JSON key:value shape) is NOT flagged" zero "$WORKDIR/18_gcp_prose.txt"

echo 'AZURE_OPENAI_API_KEY=mock-azure-key-for-testing' > "$WORKDIR/20_azure_placeholder.txt"
check "Test 20: AZURE_*_KEY=mock-... placeholder (non-hex value) is NOT flagged" zero "$WORKDIR/20_azure_placeholder.txt"

# ---------------------------------------------------------------------------
# Paired §1.1 mutation: neuter the Google-pattern line in secret_scan.sh,
# re-run Test 1's fixture, and assert it now WRONGLY passes (proving Test 1
# is load-bearing — it genuinely depends on that pattern line, not a
# tautology). Then restore and assert Test 1 passes again.
# ---------------------------------------------------------------------------

echo ""
echo "--- Paired mutation: neuter Google-key pattern, expect Test 1 fixture to WRONGLY pass ---"

# Replace the Google pattern's regex with one that can never match anything
# (MUTATED for paired §1.1 mutation test — restored unconditionally by the
# EXIT trap above before this script returns).
sed -i.bak 's#Google API key|AIza\[0-9A-Za-z_-\]{20,}#Google API key|ZZZ_THIS_PATTERN_CAN_NEVER_MATCH_ANYTHING_ZZZ#' "$SCANNER"
rm -f "${SCANNER}.bak"

mutation_out=$("$SCANNER" "$WORKDIR/1_google.txt" 2>&1)
mutation_rc=$?
if [ "$mutation_rc" -eq 0 ]; then
  echo "PASS (mutation confirmed load-bearing): with the Google pattern neutered, the real-shaped Google key fixture WRONGLY passed (exit 0) — Test 1 genuinely depends on that pattern line."
  PASS=$((PASS + 1))
else
  echo "FAIL (mutation did NOT flip the result): neutering the Google pattern still produced exit $mutation_rc — Test 1 may not be exercising the pattern this mutation targets, or another pattern independently caught it."
  printf '%s\n' "$mutation_out" | sed 's/^/    /'
  FAIL=$((FAIL + 1))
fi

# Restore (also happens unconditionally in the EXIT trap; done explicitly
# here too so the very next check runs against the restored scanner).
cp "$BACKUP" "$SCANNER"

echo "--- Paired mutation: scanner restored, expect Test 1 fixture to correctly fail again ---"
check "Test 12: post-restore, real-shaped Google key correctly detected again" nonzero "$WORKDIR/1_google.txt"

# ---------------------------------------------------------------------------
# Second paired §1.1 mutation (2026-07-11 hardening pass): neuter the new
# "Anthropic API key (explicit)" pattern line, re-run Test 15's fixture,
# and assert it now WRONGLY passes — proving Test 15 genuinely depends on
# that new pattern line (not merely on the pre-existing generic OpenAI
# sk- pattern, which was independently confirmed NOT to match sk-ant-...
# shapes during this hardening pass — see scratchpad/r41_guard_hardening.md).
# Then restore and assert Test 15 passes again.
# ---------------------------------------------------------------------------

echo ""
echo "--- Paired mutation: neuter Anthropic-key pattern, expect Test 15 fixture to WRONGLY pass ---"

# (MUTATED for paired §1.1 mutation test — restored unconditionally by the
# EXIT trap above before this script returns).
sed -i.bak 's#Anthropic API key (explicit)|sk-ant-\[A-Za-z0-9_-\]{30,}#Anthropic API key (explicit)|ZZZ_THIS_PATTERN_CAN_NEVER_MATCH_ANYTHING_ZZZ#' "$SCANNER"
rm -f "${SCANNER}.bak"

mutation2_out=$("$SCANNER" "$WORKDIR/15_anthropic.txt" 2>&1)
mutation2_rc=$?
if [ "$mutation2_rc" -eq 0 ]; then
  echo "PASS (mutation confirmed load-bearing): with the Anthropic pattern neutered, the real-shaped Anthropic key fixture WRONGLY passed (exit 0) — Test 15 genuinely depends on that pattern line."
  PASS=$((PASS + 1))
else
  echo "FAIL (mutation did NOT flip the result): neutering the Anthropic pattern still produced exit $mutation2_rc — Test 15 may not be exercising the pattern this mutation targets, or another pattern independently caught it."
  printf '%s\n' "$mutation2_out" | sed 's/^/    /'
  FAIL=$((FAIL + 1))
fi

# Restore (also happens unconditionally in the EXIT trap; done explicitly
# here too so the very next check runs against the restored scanner).
cp "$BACKUP" "$SCANNER"

echo "--- Paired mutation: scanner restored, expect Test 15 fixture to correctly fail again ---"
check "Test 21: post-restore, real-shaped Anthropic key correctly detected again" nonzero "$WORKDIR/15_anthropic.txt"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
