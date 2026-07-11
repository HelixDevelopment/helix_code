#!/usr/bin/env bash
# scripts/secret_scan_test.sh
#
# Hermetic test suite for scripts/secret_scan.sh (§11.4.135 / §11.4.138
# permanent secret-scan guard). Ten real-vs-allowlisted fixture cases plus
# one paired §1.1 mutation proving the guard is genuinely load-bearing (not
# a tautology): neuter the Google-pattern line, plant a real-shaped Google
# key, show the guard WRONGLY passes, restore, show it correctly fails
# again. Never echoes a real secret value to stdout in this test's own
# output beyond the deliberately-fake fixture strings written to disk
# (none of the fixture strings are real credentials — see NOTE below).
#
# NOTE: every "real-shaped" fixture below is a FABRICATED placeholder built
# to match the shape (prefix + length) of a real key, never a value that
# could authenticate against any real service. §11.4.10 governs REAL
# secrets; these are synthetic test inputs, exactly like scan-secrets.sh's
# own test suite (scripts/test-scan-secrets.sh) and the .scan-secrets-allow
# entries that document this same convention project-wide.

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

echo 'GEMINI_API_KEY=AIzaSyD-fabricated0123456789abcdef' > "$WORKDIR/1_google.txt"
check "Test 1: real-shaped Google API key" nonzero "$WORKDIR/1_google.txt"

echo 'OPENAI_API_KEY=sk-fabricated0123456789ABCDEFGHIJKLMN' > "$WORKDIR/2_openai.txt"
check "Test 2: real-shaped OpenAI API key" nonzero "$WORKDIR/2_openai.txt"

echo 'AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP' > "$WORKDIR/3_aws.txt"
check "Test 3: real-shaped AWS access key" nonzero "$WORKDIR/3_aws.txt"

echo 'GITHUB_TOKEN=ghp_fabricated0123456789ABCDEFGHIJKLMNOPQR' > "$WORKDIR/4_github.txt"
check "Test 4: real-shaped GitHub PAT" nonzero "$WORKDIR/4_github.txt"

printf -- '-----BEGIN RSA PRIVATE KEY-----\nMIIfabricatedNotARealKeyBodyAtAll\n-----END RSA PRIVATE KEY-----\n' > "$WORKDIR/5_pem.txt"
check "Test 5: private-key header (RSA)" nonzero "$WORKDIR/5_pem.txt"

printf -- '-----BEGIN PRIVATE KEY-----\nMIIfabricatedGenericPKCS8Body\n-----END PRIVATE KEY-----\n' > "$WORKDIR/6_pem_generic.txt"
check "Test 6: private-key header (generic PKCS8)" nonzero "$WORKDIR/6_pem_generic.txt"

echo 'SLACK_BOT_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx' > "$WORKDIR/7_slack_real.txt"
check "Test 7: real-shaped Slack bot token (xoxb-N-N-alnum)" nonzero "$WORKDIR/7_slack_real.txt"

# HuggingFace hf_ token — the exact class that partially leaked into the
# session transcript on 2026-07-11 (catalogue-providers stream); added to
# the guard patterns per §11.4.138 (close the class that escaped).
echo 'HF_TOKEN=hf_fabricated0123456789ABCDEFGHIJKLMNOP' > "$WORKDIR/7b_hf.txt"
check "Test 7b: real-shaped HuggingFace token (hf_...)" nonzero "$WORKDIR/7b_hf.txt"

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

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
