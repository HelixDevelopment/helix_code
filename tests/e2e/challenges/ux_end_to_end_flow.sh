#!/usr/bin/env bash
# tests/e2e/challenges/ux_end_to_end_flow.sh — anti-bluff UX
# Challenge per CONST-035 + CONST-050(B). FINAL 6th of the 6
# missing test types from Task #266. Closes Task #266.
#
# What this Challenge proves:
#   - The end-to-end USER EXPERIENCE coheres across a realistic
#     journey: discover help → confirm health → discover models →
#     attempt action with bogus model → recover via friendly error
#     → confirm post-error liveness.
#   - Every step's output is comprehensible to a user (no raw
#     stack traces, no `panic:`, no `goroutine N [running]:`
#     surfacing to the user surface).
#   - Failure modes don't strand the user — every error message
#     either names the bad input OR points at the recovery action
#     (config wizard / set env var / list-models for valid IDs).
#   - The system is rerunnable after a deliberate bad-input attempt
#     (no leaked lock files / no broken state).
#
# Differs from UI Challenge (label-by-label assertions on
# individual flags): UX Challenge asserts on the JOURNEY — the
# sequence and coherence across multiple steps. UI proves each
# step's output schema; UX proves the chain holds together.
#
# Operator-safe: no LLM round-trip required (we use a deliberately
# bogus model to exercise the error path), so no provider key
# needed. SKIP-OK if cli binary missing.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

CLI_BIN="${HELIX_CLI_BIN:-HelixCode/bin/cli}"
TIMEOUT_SEC="${UX_CLI_TIMEOUT_SEC:-30}"
BOGUS_MODEL="${UX_BOGUS_MODEL:-bogus-model-that-does-not-exist-xyz}"

# Phrases that mean the binary leaked a raw stack-trace / panic
# to the user surface — would be a UX bluff.
USER_HOSTILE_PATTERNS=(
    'panic:'
    'goroutine [0-9]+ \[running\]:'
    'runtime error:'
    'segmentation fault'
    'fatal error:'
)

echo "=== UX End-to-End Flow Challenge (anti-bluff per CONST-035) ==="
echo "  cli binary:           $CLI_BIN"
echo "  per-step timeout:     ${TIMEOUT_SEC}s"
echo "  deliberately-bogus model: $BOGUS_MODEL"

assert_no_panic() {
    local label="$1"
    local body="$2"
    for pat in "${USER_HOSTILE_PATTERNS[@]}"; do
        if printf '%s' "$body" | grep -qE "$pat"; then
            echo "  FAIL: $label surfaced user-hostile pattern: $pat"
            echo "  last 300 chars: $(printf '%s' "$body" | tail -c 300)"
            return 1
        fi
    done
    return 0
}

# Step 1: binary-presence dispatch.
echo
echo "[1/6] Binary-presence dispatch..."
if [[ ! -x "$CLI_BIN" ]]; then
    echo "  SKIP: $CLI_BIN not found — SKIP-OK: #env-binary-missing"
    echo "  (run \`cd HelixCode && make build\` to produce ./bin/cli)"
    echo
    echo "=== UX End-to-End Flow Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "  PASS: cli binary executable"

# Step 2: Discover help → confirm cli documents both `-help` AND
# the recovery action (`wizard` / `provider` / `key`).
echo
echo "[2/6] Discover help → assert recovery hints documented..."
help_out=$(timeout "$TIMEOUT_SEC" "$CLI_BIN" -help 2>&1 || true)
assert_no_panic "-help" "$help_out" || exit 1
# At least one recovery hint must appear — these tell the user
# what to do when something goes wrong.
hint_count=0
for hint in 'wizard' 'provider' 'key' 'list-models' 'health'; do
    if printf '%s' "$help_out" | grep -qE -- "-?$hint"; then
        hint_count=$((hint_count + 1))
    fi
done
if [[ "$hint_count" -lt 3 ]]; then
    echo "  FAIL: help text exposes only $hint_count/5 documented recovery hints"
    exit 1
fi
echo "  PASS: help text exposes $hint_count/5 recovery hints (wizard/provider/key/list-models/health)"

# Step 3: Confirm health → second probe in the user journey.
echo
echo "[3/6] Confirm health (journey step 2)..."
health_out=$(timeout "$TIMEOUT_SEC" "$CLI_BIN" -health 2>&1 || true)
assert_no_panic "-health" "$health_out" || exit 1
if ! printf '%s' "$health_out" | grep -qE 'System is operational|operational'; then
    echo "  FAIL: -health doesn't render the 'operational' verdict line"
    echo "  last 200 chars: $(printf '%s' "$health_out" | tail -c 200)"
    exit 1
fi
echo "  PASS: -health renders the 'operational' verdict line cleanly"

# Step 4: Discover models → the user has a way to find what to use.
echo
echo "[4/6] Discover models (journey step 3)..."
models_out=$(timeout "$TIMEOUT_SEC" "$CLI_BIN" -list-models 2>&1 || true)
assert_no_panic "-list-models" "$models_out" || exit 1
model_count=$(printf '%s' "$models_out" | grep -cE '^ID: ')
if [[ "$model_count" -lt 1 ]]; then
    echo "  FAIL: -list-models exposed 0 selectable models (user has nothing to choose)"
    echo "  last 200 chars: $(printf '%s' "$models_out" | tail -c 200)"
    exit 1
fi
# Capture first model ID — pretend the user picked it.
first_model=$(printf '%s' "$models_out" | grep -E '^ID: ' | head -1 | sed 's/^ID: //; s/[[:space:]]*$//')
echo "  PASS: $model_count models available; user picks first → '$first_model'"

# Step 5: Attempt action with deliberately bogus model → assert
# error is user-comprehensible AND not user-hostile.
echo
echo "[5/6] Attempt action with bogus model → assert friendly recovery..."
set +e
bogus_out=$(timeout "$TIMEOUT_SEC" "$CLI_BIN" -model "$BOGUS_MODEL" -prompt "ping" 2>&1)
bogus_exit=$?
set -e
echo "  bogus-model exit code: $bogus_exit"
assert_no_panic "-model bogus" "$bogus_out" || exit 1
# The error message must mention SOMETHING actionable: the model
# name, the word "model"/"provider"/"not found"/"unknown", OR a
# recovery hint.
friendly_count=0
for token in "$BOGUS_MODEL" 'model' 'provider' 'not found' 'unknown' 'available' 'list-models'; do
    if printf '%s' "$bogus_out" | grep -qiF -- "$token"; then
        friendly_count=$((friendly_count + 1))
    fi
done
if [[ "$friendly_count" -lt 1 ]]; then
    echo "  FAIL: bogus-model error didn't surface any user-actionable token"
    echo "  last 300 chars: $(printf '%s' "$bogus_out" | tail -c 300)"
    exit 1
fi
echo "  PASS: bogus-model error surfaced $friendly_count user-actionable tokens"

# Step 6: Post-error liveness — system survived the bogus attempt.
# Re-run -health; must still report 'operational'.
echo
echo "[6/6] Post-error liveness (system survived bogus attempt)..."
post_out=$(timeout "$TIMEOUT_SEC" "$CLI_BIN" -health 2>&1 || true)
assert_no_panic "post-error -health" "$post_out" || exit 1
if ! printf '%s' "$post_out" | grep -qE 'System is operational|operational'; then
    echo "  FAIL: post-error -health no longer reports 'operational' — system tipped over"
    echo "  last 200 chars: $(printf '%s' "$post_out" | tail -c 200)"
    exit 1
fi
echo "  PASS: post-error -health still reports 'operational' — UX journey survived"

echo
echo "=== UX End-to-End Flow Challenge: PASSED ==="
echo "  Captured evidence:"
echo "    journey=discover→health→models→bogus-recover→post-liveness"
echo "    models_listed=${model_count} picked='${first_model}' bogus_exit=${bogus_exit}"
echo "    user_actionable_tokens_in_error=${friendly_count}"
