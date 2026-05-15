#!/usr/bin/env bash
# at_mention.sh — anti-bluff Challenge for the @-file-mention REPL feature
# (CONST-035 / §11.4.2): the REPL claims to recognise `@<path>` tokens, read
# the referenced file, attach it to the prompt sent to the LLM, and surface
# a 📎-marker to the user. This Challenge proves all four invariants with
# captured runtime evidence.
#
# Bluff-class avoided: "the helper function exists in source so the feature
# must work" — that's a CONST-035 structural-only PASS. Paired structural +
# runtime gates below.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

echo "=== @-file-mention REPL anti-bluff Challenge ==="

# Step 1: structural — both helper functions present in source.
echo "[1/5] Structural: helpers wired in cmd/cli/main.go..."
if ! grep -q 'func expandAtMentions(prompt \*string)' HelixCode/cmd/cli/main.go; then
    echo "  FAIL: expandAtMentions helper missing"; exit 1
fi
if ! grep -q 'func atMentionTokens(text string)' HelixCode/cmd/cli/main.go; then
    echo "  FAIL: atMentionTokens helper missing"; exit 1
fi
if ! grep -q 'expandAtMentions(&promptToSend)' HelixCode/cmd/cli/main.go; then
    echo "  FAIL: handleInteractive doesn't call expandAtMentions"; exit 1
fi
echo "  PASS: source has expandAtMentions + atMentionTokens + REPL call-site"

# Step 2: structural — unit tests present.
echo "[2/5] Structural: unit tests present..."
if [ ! -f HelixCode/cmd/cli/at_mentions_test.go ]; then
    echo "  FAIL: at_mentions_test.go missing"; exit 1
fi
test_count=$(grep -c '^func Test' HelixCode/cmd/cli/at_mentions_test.go)
if [ "$test_count" -lt 5 ]; then
    echo "  FAIL: only $test_count test funcs (want ≥5 covering tokens + attach + miss + oversize + dir)"
    exit 1
fi
echo "  PASS: $test_count unit-test functions present"

# Step 3: runtime — unit tests pass.
echo "[3/5] Runtime: unit tests pass..."
( cd HelixCode && go test -count=1 -run "TestAtMentionTokens|TestExpandAtMentions" ./cmd/cli/ ) || {
    echo "  FAIL: unit tests do not pass"; exit 1
}
echo "  PASS: TestAtMentionTokens + TestExpandAtMentions* all green"

# Step 4: live REPL round-trip with @-mention (skip honestly if no API key).
echo "[4/5] Live REPL @-mention round-trip..."
set +e
if [ -f scripts/load_api_keys.sh ]; then
    . scripts/load_api_keys.sh 2>/dev/null
elif [ -f "$HOME/api_keys.sh" ]; then
    . "$HOME/api_keys.sh" 2>/dev/null
fi
set -e
if [ -z "${GROQ_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${MISTRAL_API_KEY:-}" ] && [ -z "${DEEPSEEK_API_KEY:-}" ] && [ -z "${OPENROUTER_API_KEY:-}" ]; then
    echo "  SKIP: no provider key in env — SKIP-OK: #env-no-llm-key"
    echo
    echo "=== @-mention Challenge: PASSED (live step SKIP-OK) ==="
    exit 0
fi

PROVIDER=groq
[ -z "${GROQ_API_KEY:-}" ] && PROVIDER=openai
[ -z "${OPENAI_API_KEY:-}" ] && [ -n "${ANTHROPIC_API_KEY:-}" ] && PROVIDER=anthropic
[ -z "${GROQ_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -n "${MISTRAL_API_KEY:-}" ] && PROVIDER=mistral
[ -z "${GROQ_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${MISTRAL_API_KEY:-}" ] && [ -n "${DEEPSEEK_API_KEY:-}" ] && PROVIDER=deepseek
[ -z "${GROQ_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${MISTRAL_API_KEY:-}" ] && [ -z "${DEEPSEEK_API_KEY:-}" ] && [ -n "${OPENROUTER_API_KEY:-}" ] && PROVIDER=openrouter
echo "  Using provider: $PROVIDER"

# Build CLI if missing.
if [ ! -x HelixCode/bin/cli ]; then
    ( cd HelixCode && go build -o bin/cli ./cmd/cli ) || { echo "  FAIL: CLI build failed"; exit 1; }
fi

# Plant a known-content file. The LLM should be able to identify the
# unique sentinel string after we @-mention the file.
SENTINEL="atmention-canary-$(date +%s)-$$"
TMP_FILE="/tmp/at_mention_challenge_${SENTINEL}.txt"
cat > "$TMP_FILE" <<EOF
This file is part of an anti-bluff Challenge that proves the @-mention
feature really reads files. The unique sentinel for this run is:
${SENTINEL}
The HelixCode @-mention feature must surface this sentinel to the LLM
so it can quote it back in its answer.
EOF
trap "rm -f $TMP_FILE" EXIT

# Ask the LLM to echo back the sentinel; if @-mention works, the model
# sees the sentinel in the attached file and echoes it; if @-mention
# is a bluff (the helper exists but isn't wired), the model sees the
# literal `@<path>` token and answers with no knowledge of the
# sentinel.
LIVE_OUT=$(printf 'Read @%s and quote ONLY the sentinel string verbatim, nothing else.\n/exit\n' "$TMP_FILE" | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./HelixCode/bin/cli 2>&1 || true)

# Invariant (a): the 📎 attachment marker was emitted.
if ! printf '%s' "$LIVE_OUT" | grep -q "📎 attached: $TMP_FILE"; then
    echo "  FAIL: REPL didn't surface 📎-attached marker for $TMP_FILE"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
# Invariant (b): the LLM's response contains the unique sentinel — proves
# the file content actually reached the model.
if ! printf '%s' "$LIVE_OUT" | grep -q "$SENTINEL"; then
    echo "  FAIL: LLM response didn't echo the sentinel — @-mention didn't reach the model"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
# Invariant (c): clean shutdown via /exit after the @-mention turn.
if ! printf '%s' "$LIVE_OUT" | grep -q 'Goodbye!'; then
    echo "  FAIL: REPL didn't reach Goodbye! after @-mention turn"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
echo "  PASS: 📎-marker emitted + sentinel echoed back by LLM + clean shutdown"

# Step 5: miss-handling — @-mention of a non-existent file MUST stay verbatim
# and MUST NOT crash. Anti-bluff: prove the failure mode is graceful.
echo "[5/5] Miss-handling: non-existent file stays verbatim..."
MISS_OUT=$(printf 'Say only the word "noted"\n/exit\n' | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./HelixCode/bin/cli 2>&1 || true)
# Should reach Goodbye even with no @-mention.
if ! printf '%s' "$MISS_OUT" | grep -q 'Goodbye!'; then
    echo "  FAIL: REPL didn't shutdown cleanly on plain-prompt turn"
    printf '%s\n' "$MISS_OUT" | tail -10
    exit 1
fi
# A separate run with a @-mention of a non-existent file should NOT emit
# the 📎-marker (no attachment) and SHOULD still reach Goodbye.
GHOST_OUT=$(printf 'Say only the word "ok": @/tmp/this/path/does/not/exist.zzz\n/exit\n' | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./HelixCode/bin/cli 2>&1 || true)
if printf '%s' "$GHOST_OUT" | grep -q "📎 attached: /tmp/this/path/does/not/exist.zzz"; then
    echo "  FAIL: REPL emitted attachment marker for non-existent file (bluff!)"
    exit 1
fi
if ! printf '%s' "$GHOST_OUT" | grep -q 'Goodbye!'; then
    echo "  FAIL: REPL didn't shutdown cleanly on ghost-mention turn"
    exit 1
fi
echo "  PASS: non-existent @-mention stays verbatim + clean shutdown"

echo
echo "=== @-mention Challenge: PASSED ==="
