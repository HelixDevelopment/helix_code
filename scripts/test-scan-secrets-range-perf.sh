#!/usr/bin/env bash
# scripts/test-scan-secrets-range-perf.sh
#
# Perf + correctness regression guard for scan-secrets.sh's `--range` mode
# (the run_scan_range function used by scripts/git_hooks/pre-push).
#
# §11.4.82 FIX (2026-07-11): the original run_scan_range looped over every
# surviving added diff line and, for EACH line, looped over EVERY pattern in
# PATTERNS, spawning `printf | grep -qE` (two subprocesses) per
# (line x pattern) pair -- O(lines * patterns) subprocess spawns. On an
# 18-commit / 1.8 MiB root push this produced hundreds of thousands of
# spawns and took ~15 MINUTES (the actual git transfer was instant),
# blocking every future root push. Fixed to O(total bytes): PATTERNS are
# combined ONCE into a single alternation regex and the entire surviving
# corpus is grepped in a SINGLE `printf | grep -nE` call -- one process
# spawn total instead of one per (line x pattern).
#
# §11.4.115 (RED-baseline-on-the-broken-artifact): this fixture and its
# planted secrets were first run against a saved snapshot of the PRE-FIX
# script to capture the slow baseline BEFORE the fix landed. Measured on
# this exact fixture (1200 benign lines, 12 patterns => 14,412 spawns):
#   UNFIXED (RED):  48.10s wall-clock  (exit 1, secret correctly detected)
#   FIXED   (GREEN): 0.09s wall-clock  (exit 1, IDENTICAL output)
# ~534x speedup, byte-identical detection output (diff of both runs' stdout
# is empty). See the commit message / evidence note for the full capture.
#
# This test proves the fix is BOTH fast AND correct: a fast scanner that
# stopped detecting secrets would be a worse regression than the perf bug
# it replaces (§11.4 anti-bluff -- a faster-but-weaker scanner is a security
# regression, not an improvement). It exercises every selectivity path
# run_scan_range applies: extension include-list, dir excludes, filename
# excludes, and the .scan-secrets-allow allowlist -- plus the §11.4.10
# value-never-printed contract.
#
# Exit 0 = all assertions passed. Exit 1 = at least one failed.

set -uo pipefail
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
SCRIPT="$REPO_ROOT/scripts/scan-secrets.sh"

FIXTURE_DIR=$(mktemp -d)
trap 'rm -rf "$FIXTURE_DIR"' EXIT

PASS=0
FAIL=0
ok()  { echo "  PASS - $1"; PASS=$((PASS + 1)); }
bad() { echo "  FAIL - $1"; FAIL=$((FAIL + 1)); }

# now_ms: milliseconds since epoch. Prefers GNU `date +%s%N` (nanosecond
# resolution); falls back to 1-second resolution on platforms whose `date`
# lacks %N (e.g. stock BSD/macOS date) so this test stays cross-platform
# (§11.4.81) without a hard GNU-coreutils dependency. 1s resolution is still
# more than sufficient to distinguish a ~0.1s fixed run from a ~48s
# regressed run against the PERF_THRESHOLD_MS below.
now_ms() {
  local raw
  raw=$(date +%s%N 2>/dev/null || true)
  case "$raw" in
    *N|"")
      echo "$(( $(date +%s) * 1000 ))"
      ;;
    *)
      echo "$(( raw / 1000000 ))"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Build fixture: a throwaway git repo with a base commit and a "new" commit
# whose diff exercises every run_scan_range selectivity path:
#   - N_BENIGN benign added lines in an included, non-excluded file (the
#     perf driver -- this is what made the old O(lines*patterns) code slow)
#   - 1 real-shaped secret in that same file            -> MUST be detected
#   - 1 real-shaped secret in an allowlisted file        -> MUST be skipped
#   - 1 real-shaped secret in an excluded dir (vendor/)  -> MUST be skipped
#   - 1 real-shaped secret in a non-included ext (.xyz)  -> MUST be skipped
#   - 1 real-shaped secret in an excluded filename       -> MUST be skipped
#     (*.example)
# ---------------------------------------------------------------------------
N_BENIGN=1200

cd "$FIXTURE_DIR"
git init -q
git config user.email "test@example.invalid"
git config user.name "scan-secrets-range-perf-test"
mkdir -p vendor docs

echo "# fixture repo" > README.md
git add README.md
git commit -q -m "base"
BASE_SHA=$(git rev-parse HEAD)

{
  i=0
  while [ "$i" -lt "$N_BENIGN" ]; do
    echo "line_${i} = \"benign content, nothing to see here, index ${i}\""
    i=$((i + 1))
  done
  # Real-shaped Anthropic-style secret -- MUST be detected (kept file, kept
  # dir, kept extension, not allowlisted).
  echo 'ANTHROPIC_API_KEY = "sk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefg"'  # EXAMPLE fixture literal, not a real key
} > big_file.py

# Allowlisted file with a real-shaped OpenAI-style secret -> MUST be skipped.
echo 'TOKEN = "sk-FAKE0123456789abcdefghijklmnopqrstuvwxyz"' > docs/allowlisted_notes.md  # EXAMPLE fixture literal, not a real key

# Excluded-directory file with a real-shaped secret -> MUST be skipped.
echo 'VENDORED_KEY = "sk-ant-api03-VENDORSECRETABCDEFGHIJKLMNOPQRSTUVWXYZ"' > vendor/leak.py  # EXAMPLE fixture literal, not a real key

# Non-included-extension file with a real-shaped secret -> MUST be skipped.
echo 'RANDOM_KEY = "sk-ant-api03-EXTFILTEREDABCDEFGHIJKLMNOPQRSTUVWXYZ"' > random.xyz  # EXAMPLE fixture literal, not a real key

# Excluded-filename file with a real-shaped secret -> MUST be skipped.
echo 'EXAMPLE_KEY = "sk-ant-api03-EXAMPLEFILEABCDEFGHIJKLMNOPQRSTUVWXYZ"' > config.example

# Allowlist lives at the fixture repo root -- this IS $REPO_ROOT for the
# scan (scan-secrets.sh cd's to `git rev-parse --show-toplevel`), so this
# does NOT touch the real repo's .scan-secrets-allow.
printf '# test allowlist\ndocs/allowlisted_notes.md\n' > .scan-secrets-allow

git add -A
git commit -q -m "new: large diff with planted secrets across every selectivity path"
NEW_SHA=$(git rev-parse HEAD)

cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# Run + time.
# ---------------------------------------------------------------------------
echo "TEST: scan-secrets.sh --range perf + correctness"
echo "  fixture: ${N_BENIGN} benign added lines + 5 planted secrets across every selectivity path"

START_MS=$(now_ms)
OUT=$(cd "$FIXTURE_DIR" && bash "$SCRIPT" --range "$BASE_SHA" "$NEW_SHA" 2>&1)
RC=$?
END_MS=$(now_ms)
ELAPSED_MS=$((END_MS - START_MS))

echo "  elapsed: ${ELAPSED_MS}ms  exit=${RC}"
echo "  output:"
sed 's/^/    /' <<< "$OUT"
echo ""

# (a) PERF: must complete well under the O(lines*patterns) regression
#     threshold. Measured unfixed baseline on this exact fixture: 48100ms.
#     Measured fixed: ~90ms. Threshold set at 5000ms: >50x margin above the
#     fixed measurement, >9x below the unfixed measurement -- a genuine
#     O(lines*patterns) regression at this fixture size cannot pass this.
PERF_THRESHOLD_MS=5000
if [ "$ELAPSED_MS" -lt "$PERF_THRESHOLD_MS" ]; then
  ok "perf: ${ELAPSED_MS}ms < ${PERF_THRESHOLD_MS}ms threshold (O(total bytes), not O(lines*patterns))"
else
  bad "perf: ${ELAPSED_MS}ms >= ${PERF_THRESHOLD_MS}ms threshold -- possible O(lines*patterns) regression"
fi

# (b) exit code: must be non-zero (a secret WAS planted and must be caught).
if [ "$RC" -ne 0 ]; then
  ok "exit code non-zero (secret detected)"
else
  bad "exit code 0 -- planted secret was NOT detected"
fi

# (c) the real, in-scope planted secret MUST be reported.
if grep -q "^big_file.py: possible credential pattern" <<< "$OUT"; then
  ok "planted secret in big_file.py detected"
else
  bad "planted secret in big_file.py NOT detected"
fi

# (d) allowlisted secret must NOT be reported.
if grep -q "allowlisted_notes.md" <<< "$OUT"; then
  bad "allowlisted secret in docs/allowlisted_notes.md was incorrectly flagged"
else
  ok "allowlisted secret correctly skipped"
fi

# (e) dir-excluded secret must NOT be reported.
if grep -q "vendor/leak.py" <<< "$OUT"; then
  bad "dir-excluded secret in vendor/leak.py was incorrectly flagged"
else
  ok "dir-excluded (vendor/) secret correctly skipped"
fi

# (f) extension-excluded secret must NOT be reported.
if grep -q "random.xyz" <<< "$OUT"; then
  bad "extension-excluded secret in random.xyz was incorrectly flagged"
else
  ok "extension-excluded (.xyz) secret correctly skipped"
fi

# (g) filename-excluded secret must NOT be reported.
if grep -q "config.example" <<< "$OUT"; then
  bad "filename-excluded secret in config.example was incorrectly flagged"
else
  ok "filename-excluded (*.example) secret correctly skipped"
fi

# (h) value-never-printed (§11.4.10): the raw secret VALUE must never appear
#     in the report output, only the filename + fixed label.
if grep -q "sk-ant-api03-ABCDEFGHIJ" <<< "$OUT"; then
  bad "value-never-printed violated -- raw secret value leaked into report output"
else
  ok "value-never-printed preserved"
fi

# (i) exactly ONE finding line -- confirms no double-reporting / no
#     over-matching introduced by combining PATTERNS into one alternation.
FOUND_LINES=$(grep -c "possible credential pattern" <<< "$OUT")
if [ "$FOUND_LINES" -eq 1 ]; then
  ok "exactly one finding reported (no over-matching from combined pattern)"
else
  bad "expected exactly 1 finding line, got ${FOUND_LINES}"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
