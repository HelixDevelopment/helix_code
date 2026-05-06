#!/usr/bin/env bash
# scripts/test_load_api_keys.sh
# Self-test for scripts/load_api_keys.sh (P1.5-WP4).
# Exercises both code branches: $HOME/api_keys.sh and .env fallback.
#
# Usage: bash scripts/test_load_api_keys.sh

set -u

LOADER="$(cd "$(dirname "$0")" && pwd)/load_api_keys.sh"
TMPROOT="$(mktemp -d)"
PASS=0
FAIL=0

cleanup() {
    rm -rf "$TMPROOT"
}
trap cleanup EXIT

assert_eq() {
    local name="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "PASS: $name"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $name (expected=$expected, actual=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

# Branch 1: $HOME/api_keys.sh present -> shell file is sourced, .env ignored
T1_HOME="$TMPROOT/home1"
T1_CWD="$TMPROOT/cwd1"
mkdir -p "$T1_HOME" "$T1_CWD"
echo 'export TEST_LOADER_KEY=from_sh' > "$T1_HOME/api_keys.sh"
echo 'TEST_LOADER_KEY=from_env_should_lose' > "$T1_CWD/.env"
touch "$T1_CWD/.gitmodules"
RESULT=$(HOME="$T1_HOME" bash -c "cd '$T1_CWD' && . '$LOADER' && printf '%s' \"\$TEST_LOADER_KEY\"")
assert_eq "branch1_prefers_api_keys_sh" "from_sh" "$RESULT"

# Branch 2: $HOME/api_keys.sh absent -> .env fallback used
T2_HOME="$TMPROOT/home2"
T2_CWD="$TMPROOT/cwd2"
mkdir -p "$T2_HOME" "$T2_CWD"
echo 'TEST_LOADER_KEY2=from_env' > "$T2_CWD/.env"
touch "$T2_CWD/.gitmodules"
RESULT=$(HOME="$T2_HOME" bash -c "cd '$T2_CWD' && . '$LOADER' && printf '%s' \"\$TEST_LOADER_KEY2\"")
assert_eq "branch2_falls_back_to_env" "from_env" "$RESULT"

# Branch 3: both absent -> loader is silent and does not fail the source
T3_HOME="$TMPROOT/home3"
T3_CWD="$TMPROOT/cwd3"
mkdir -p "$T3_HOME" "$T3_CWD"
RESULT=$(HOME="$T3_HOME" bash -c "cd '$T3_CWD' && . '$LOADER'; printf 'OK'")
assert_eq "branch3_neither_present_is_silent" "OK" "$RESULT"

# Branch 4: HELIXCODE_LOAD_API_KEYS=0 -> opt-out (no auto-load)
T4_HOME="$TMPROOT/home4"
T4_CWD="$TMPROOT/cwd4"
mkdir -p "$T4_HOME" "$T4_CWD"
echo 'export TEST_LOADER_KEY3=from_sh' > "$T4_HOME/api_keys.sh"
RESULT=$(HOME="$T4_HOME" HELIXCODE_LOAD_API_KEYS=0 bash -c "cd '$T4_CWD' && . '$LOADER' && printf '%s' \"\${TEST_LOADER_KEY3:-unset}\"")
assert_eq "branch4_opt_out_respected" "unset" "$RESULT"

echo
echo "Results: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
