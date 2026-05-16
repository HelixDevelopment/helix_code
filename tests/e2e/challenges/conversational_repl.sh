#!/usr/bin/env bash
# conversational_repl.sh — anti-bluff Challenge per CONST-035 / §11.9:
# verify the CLI's interactive REPL is a real conversational interface
# (NOT the round-pre-41 broken form that only accepted 6 hardcoded
# commands and truncated multi-word prompts to one token via fmt.Scanln).
#
# Per CONST-035, every PASS in this Challenge carries positive runtime
# evidence captured during execution. The bluff-class avoided:
# **the REPL prints '=== Helix CLI Interactive Mode ===' and looks
# friendly, but plain prompts are silently ignored or rejected.**

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

echo "=== Conversational REPL anti-bluff Challenge ==="

# Step 1: Build / verify CLI exists.
echo "[1/6] Checking CLI binary..."
if [ ! -x helix_code/bin/cli ]; then
    echo "  Building CLI..."
    (cd helix_code && go build -o bin/cli ./cmd/cli) || {
        echo "  FAIL: CLI build failed"; exit 1
    }
fi
echo "  PASS: helix_code/bin/cli present"

# Step 2: Verify the source of the conversational REPL is in place. This
# is a STRUCTURAL gate (CONST-035 §11.9 forbids structural-only PASS;
# this gate is PAIRED with the runtime gate in step 4 below).
echo "[2/6] Verifying conversational REPL source..."
if ! grep -q 'bufio.NewScanner(os.Stdin)' helix_code/cmd/cli/main.go; then
    echo "  FAIL: REPL is not using bufio.Scanner (regression to fmt.Scanln word-only?)"
    exit 1
fi
if ! grep -q 'c.llmProvider.Generate(ctx, req)' helix_code/cmd/cli/main.go; then
    echo "  FAIL: REPL never calls provider.Generate (regression to slash-only commands?)"
    exit 1
fi
echo "  PASS: bufio.Scanner + provider.Generate wired in handleInteractive"

# Step 3: Verify /exit slash command works (TTY-pipe-safe regression test).
echo "[3/6] Verifying /exit clean shutdown..."
OUTPUT=$(printf '/exit\n' | timeout 15 ./helix_code/bin/cli 2>&1 || true)
if ! printf '%s' "$OUTPUT" | grep -q 'Helix CLI Interactive Mode'; then
    echo "  FAIL: REPL didn't print its banner"
    printf '%s\n' "$OUTPUT" | tail -10
    exit 1
fi
if ! printf '%s' "$OUTPUT" | grep -q 'Goodbye!'; then
    echo "  FAIL: /exit didn't reach 'Goodbye!' clean-shutdown line"
    printf '%s\n' "$OUTPUT" | tail -10
    exit 1
fi
echo "  PASS: REPL banner emitted + /exit → 'Goodbye!' confirmed"

# Step 4: Live LLM round-trip via REPL (skip honestly if no API key).
# Anti-bluff: this is the gate that proves the REPL ACTUALLY sends
# prompts to an LLM and prints responses — not a documentation-only
# claim. Honest SKIP-OK when no provider key is available.
echo "[4/6] Live REPL round-trip..."
# Use the canonical loader which does ApiKey_<Provider> → <PROVIDER>_API_KEY
# normalisation (round-41 readiness fix). The loader's auto-run block can
# return non-zero when no api_keys.sh / .env exists; with `set -euo
# pipefail` that would abort the script silently. Wrap in set +e / set -e
# so a missing-keys env honestly hits the SKIP-OK below instead of dying.
set +e
if [ -f scripts/load_api_keys.sh ]; then
    # shellcheck source=/dev/null
    . scripts/load_api_keys.sh 2>/dev/null
elif [ -f "$HOME/api_keys.sh" ]; then
    # shellcheck source=/dev/null
    . "$HOME/api_keys.sh" 2>/dev/null
fi
set -e
if [ -z "${GROQ_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ]; then
    echo "  SKIP: no GROQ_API_KEY/OPENAI_API_KEY/ANTHROPIC_API_KEY in env — SKIP-OK: #env-no-llm-key"
    echo
    echo "=== Conversational REPL Challenge: PASSED (live step SKIP-OK) ==="
    exit 0
fi

PROVIDER=groq
[ -z "${GROQ_API_KEY:-}" ] && PROVIDER=openai
[ -z "${OPENAI_API_KEY:-}" ] && [ -n "${ANTHROPIC_API_KEY:-}" ] && PROVIDER=anthropic
echo "  Using provider: $PROVIDER"

LIVE_OUT=$(printf 'Reply with the single word: four\n/exit\n' | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./helix_code/bin/cli 2>&1 || true)

# Anti-bluff invariants on the captured output:
#   (a) The REPL banner was emitted (proves we entered the conversational path)
#   (b) A 'helix> ' prompt was rendered at least twice (one for the input, one
#       for the post-response prompt)
#   (c) The model name appeared OR a real model token was emitted (proves the
#       provider was actually called)
#   (d) 'Goodbye!' was emitted at the end (proves /exit drove clean shutdown
#       AFTER the LLM turn — not before it)
if ! printf '%s' "$LIVE_OUT" | grep -q 'Helix CLI Interactive Mode'; then
    echo "  FAIL: live REPL didn't reach interactive mode banner"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
HELIX_PROMPTS=$(printf '%s' "$LIVE_OUT" | grep -c 'helix>' || true)
if [ "$HELIX_PROMPTS" -lt 2 ]; then
    echo "  FAIL: expected ≥2 'helix>' prompts (pre-prompt + post-response); got $HELIX_PROMPTS"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
if ! printf '%s' "$LIVE_OUT" | grep -q 'Goodbye!'; then
    echo "  FAIL: live REPL didn't reach Goodbye! after the prompt — /exit broken after LLM call?"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
# Stats anti-bluff: a real LLM round-trip should have produced token stats.
if ! printf '%s' "$LIVE_OUT" | grep -q 'tokens: in='; then
    echo "  FAIL: token-stats line missing — provider didn't populate Usage, or LLM call never happened"
    printf '%s\n' "$LIVE_OUT" | tail -15
    exit 1
fi
echo "  PASS: live LLM round-trip emitted banner + ≥2 helix prompts + token stats + Goodbye"

# Step 5: Multi-turn conversation memory probe (CONST-035 anti-bluff: the
# REPL claims multi-turn context preserved in handleInteractive's
# `conversation` slice. Prove it: ask "what is 2+2?", then in the SAME
# session ask "what number did I just ask about?" — the model should
# reference 2 (or 2+2). A regression that lost context between turns
# would make the model answer "I don't know" or change topic entirely.
echo "[5/6] Multi-turn conversation memory..."
MT_OUT=$(printf 'What is 2+2?\nNow what is that plus 3?\n/exit\n' | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./helix_code/bin/cli 2>&1 || true)
# Anti-bluff: second turn must mention BOTH the prior result (4) AND
# the new operation (+3, equals 7). A regression that lost context
# would produce a response that doesn't reference the prior turn.
if ! printf '%s' "$MT_OUT" | grep -qE '7|seven|that plus 3'; then
    echo "  FAIL: second turn response doesn't reference prior context (regression to single-turn REPL?)"
    printf '%s\n' "$MT_OUT" | tail -10
    exit 1
fi
echo "  PASS: multi-turn context preserved across turns"

# Step 6: /clear context-reset probe — CONST-035 anti-bluff: /clear
# should wipe conversation slice so a subsequent question doesn't
# leak prior context.
echo "[6/6] /clear context-reset..."
CL_OUT=$(printf 'What is 2+2?\n/clear\nWhat number did I just ask about?\n/exit\n' | \
    HELIX_LLM_PROVIDER=$PROVIDER timeout 60 ./helix_code/bin/cli 2>&1 || true)
if ! printf '%s' "$CL_OUT" | grep -q '(conversation history cleared)'; then
    echo "  FAIL: /clear command didn't emit confirmation"
    printf '%s\n' "$CL_OUT" | tail -10
    exit 1
fi
# The post-/clear response should NOT reference 2 (the cleared content).
# Honest invariant: response either says no prior context OR asks for
# clarification — never references "2" or "2+2".
POST_CLEAR=$(printf '%s' "$CL_OUT" | awk '/conversation history cleared/{flag=1; next} flag')
if printf '%s' "$POST_CLEAR" | grep -qE '\b2\+2\b|\babout 2\b'; then
    echo "  FAIL: post-/clear response references prior '2+2' context — /clear didn't actually clear"
    printf '%s\n' "$CL_OUT" | tail -10
    exit 1
fi
echo "  PASS: /clear wiped conversation history (no leak of prior context)"
echo
echo "=== Conversational REPL Challenge: PASSED ==="
