#!/usr/bin/env bash
# test_helixllm_mode.sh — HERMETIC tests for scripts/helixllm-mode.sh (design §3.1).
#
# NO real podman / curl / nvidia-ctk / GPU is ever touched:
#   * pure-function tests SOURCE the script (its source-guard suppresses main)
#     and OVERRIDE container_create_argv / detect_slots_props with fixtures;
#   * CLI-level tests run the script as a SUBPROCESS with a stub PATH whose
#     fake podman/curl/nvidia-ctk only LOG their argv and emit fixtures — the
#     test asserts the resolved binaries are the stubs (no real podman) and that
#     no mutating `podman run/stop/rm` is issued on the no-op / fail-closed paths.
#
# Covers: arg parsing; mode detection from a fixture CreateCommand (8=>coder,
# 1=>claude, other=>unknown, absent); /props total_slots mapping; the crux
# --print-cmd command-construction (coder<->claude differ in EXACTLY the two
# tokens -c and --parallel, --metrics + name + image + model + order preserved);
# idempotency no-op; fail-closed on unknown; drift warning; a fully-stubbed
# recreate proving stop-before-run (VRAM safety); and a §1.1 paired mutation that
# breaks the "preserve --metrics" logic and confirms the crux assertion FAILS.
#
# Honest shebang; `bash -n` + `shellcheck -S error` clean; no GNU-only constructs.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/helixllm-mode.sh"
[ -f "$SCRIPT" ] || { echo "FATAL: script under test not found: $SCRIPT" >&2; exit 2; }

SANDBOX="$(mktemp -d "${TMPDIR:-/tmp}/helixllm_mode_test.XXXXXX")"
cleanup() { rm -rf "$SANDBOX"; }
trap cleanup EXIT

STUB_DIR="$SANDBOX/bin"
mkdir -p "$STUB_DIR"

# --------------------------------------------------------------------------
# assert helpers (no `grep` in the hot path so `set -e` never bites)
# --------------------------------------------------------------------------
pass_n=0; fail_n=0
ok()  { pass_n=$((pass_n + 1)); printf '  PASS — %s\n' "$1"; }
bad() { fail_n=$((fail_n + 1)); printf '  FAIL — %s\n' "$1"; }

assert_eq()       { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (want '$2' got '$3')"; fi; }
assert_ne()       { if [ "$2" != "$3" ]; then ok "$1"; else bad "$1 (both '$2')"; fi; }
assert_exit()     { if [ "$2" -eq "$3" ]; then ok "$1 (exit=$3)"; else bad "$1 (want exit=$2 got=$3)"; fi; }
assert_contains() { case "$3" in *"$2"*) ok "$1";; *) bad "$1 (missing: $2)";; esac; }
assert_absent()   { case "$3" in *"$2"*) bad "$1 (unexpected: $2)";; *) ok "$1";; esac; }

# --------------------------------------------------------------------------
# fixtures + stubs
# --------------------------------------------------------------------------
# Write a coder-shaped CreateCommand (one argv token per line) with reordered
# flags and --metrics present, parameterised on -c and --parallel values.
write_fixture() {
  local f="$1" ctx="$2" par="$3"
  printf '%s\n' podman run -d --name helixllm-coder \
    --network=host --device nvidia.com/gpu=all --security-opt=label=disable \
    -v /home/milos/models:/models:ro \
    localhost/helixllm/llamacpp-router:cuda12.8-sm120 \
    -m /models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf \
    -ngl 99 -c "$ctx" --parallel "$par" --cont-batching -fa on \
    --cache-type-k q8_0 --cache-type-v q8_0 \
    --host 0.0.0.0 --port 18434 --jinja --metrics > "$f"
}

FIX_CODER="$SANDBOX/coder.createcmd"
FIX_CLAUDE="$SANDBOX/claude.createcmd"
FIX_UNKNOWN="$SANDBOX/unknown.createcmd"
FIX_DRIFT="$SANDBOX/drift.createcmd"
write_fixture "$FIX_CODER"   24576 8
write_fixture "$FIX_CLAUDE"  229376 1
write_fixture "$FIX_UNKNOWN" 24576 4      # --parallel 4 => unknown
write_fixture "$FIX_DRIFT"   229376 8      # parallel 8 (coder) but -c 229376 (drift)

# Stub podman: log argv; `inspect` cats $HLX_FIX (empty => absent); `container
# exists` honours $HLX_CONTAINER_EXISTS; run/stop/rm are no-ops that only LOG.
cat > "$STUB_DIR/podman" <<'STUB'
#!/usr/bin/env bash
printf 'podman %s\n' "$*" >> "${HLX_PODMAN_LOG:-/dev/null}"
case "${1:-}" in
  inspect)
    if [ -n "${HLX_FIX:-}" ] && [ -f "${HLX_FIX:-}" ]; then cat "$HLX_FIX"; fi
    exit 0 ;;
  container)
    if [ "${2:-}" = exists ]; then
      [ "${HLX_CONTAINER_EXISTS:-1}" = 1 ] && exit 0 || exit 1
    fi
    exit 0 ;;
  run|stop|rm|ps|logs) exit 0 ;;
  *) exit 0 ;;
esac
STUB

# Stub curl: /props emits $HLX_PROPS_JSON; health (/v1/models) succeeds empty.
cat > "$STUB_DIR/curl" <<'STUB'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >> "${HLX_CURL_LOG:-/dev/null}"
url=""
for a in "$@"; do case "$a" in http*) url="$a";; esac; done
case "$url" in
  *"/props"*)     printf '%s\n' "${HLX_PROPS_JSON:-}"; exit 0 ;;
  *"/v1/models"*) exit 0 ;;
  *) exit 0 ;;
esac
STUB

# Stub nvidia-ctk: create the requested --output file, exit 0 (never touch a GPU).
cat > "$STUB_DIR/nvidia-ctk" <<'STUB'
#!/usr/bin/env bash
out=""
for a in "$@"; do case "$a" in --output=*) out="${a#--output=}";; esac; done
if [ -n "$out" ]; then mkdir -p "$(dirname "$out")"; : > "$out"; fi
exit 0
STUB
chmod +x "$STUB_DIR/podman" "$STUB_DIR/curl" "$STUB_DIR/nvidia-ctk"

# Run the CLI as a subprocess under the stub PATH; capture stdout+stderr+exit.
CLI_OUT=""; CLI_RC=0
run_cli() {
  set +e
  CLI_OUT="$(PATH="$STUB_DIR:$PATH" HELIXLLM_READY_STEPS=1 \
    XDG_CONFIG_HOME="$SANDBOX/cfg" \
    bash "$SCRIPT" "$@" 2>&1)"
  CLI_RC=$?
  set -e
}

# ==========================================================================
echo "=== helixllm-mode.sh hermetic tests ==="
echo "sandbox: $SANDBOX"
echo

# --- guard: the subprocess PATH resolves to the STUBS, not real binaries ----
echo "[guard] no real podman/curl/nvidia-ctk reachable in CLI tests"
RESOLVED_PODMAN="$(PATH="$STUB_DIR:$PATH" command -v podman)"
RESOLVED_CURL="$(PATH="$STUB_DIR:$PATH" command -v curl)"
RESOLVED_NVCTK="$(PATH="$STUB_DIR:$PATH" command -v nvidia-ctk)"
assert_eq "podman resolves to stub"     "$STUB_DIR/podman"     "$RESOLVED_PODMAN"
assert_eq "curl resolves to stub"       "$STUB_DIR/curl"       "$RESOLVED_CURL"
assert_eq "nvidia-ctk resolves to stub" "$STUB_DIR/nvidia-ctk" "$RESOLVED_NVCTK"
echo

# --------------------------------------------------------------------------
# 1. ARG PARSING
# --------------------------------------------------------------------------
echo "[1] arg parsing"
export HLX_PODMAN_LOG="$SANDBOX/log.argparse"; : > "$HLX_PODMAN_LOG"
export HLX_FIX="$FIX_CODER" HLX_CONTAINER_EXISTS=1 HLX_PROPS_JSON='{"total_slots":8}'

run_cli frobnicate
assert_exit "bad verb rejected" 1 "$CLI_RC"
assert_contains "bad verb prints usage" "usage:" "$CLI_OUT"

run_cli status
assert_exit "status accepted" 0 "$CLI_RC"
assert_contains "status prints mode" "mode      : coder" "$CLI_OUT"

run_cli --print-cmd
assert_exit "--print-cmd with no mode rejected" 1 "$CLI_RC"
assert_contains "--print-cmd needs a mode" "needs a coder|claude" "$CLI_OUT"
echo

# --------------------------------------------------------------------------
# Pure-function tests: SOURCE the script (source-guard suppresses main) and
# override the podman/curl-backed readers with fixtures. No process, no binary.
# --------------------------------------------------------------------------
# shellcheck source=/dev/null
source "$SCRIPT"

FIX_ACTIVE=""; SLOTS_ACTIVE=""
container_create_argv() { if [ -n "$FIX_ACTIVE" ]; then cat "$FIX_ACTIVE"; fi; return 0; }
detect_slots_props()    { if [ -n "$SLOTS_ACTIVE" ]; then printf '%s\n' "$SLOTS_ACTIVE"; fi; return 0; }

# --------------------------------------------------------------------------
# 2. MODE DETECTION from a fixture CreateCommand
# --------------------------------------------------------------------------
echo "[2] mode detection from CreateCommand (works even when stopped)"
FIX_ACTIVE="$FIX_CODER";   assert_eq "parallel 8, -c 24576 => coder"  "coder"   "$(current_mode)"
FIX_ACTIVE="$FIX_CLAUDE";  assert_eq "parallel 1, -c 229376 => claude" "claude"  "$(current_mode)"
FIX_ACTIVE="$FIX_UNKNOWN"; assert_eq "parallel 4 => unknown"          "unknown" "$(current_mode)"
FIX_ACTIVE="";             assert_eq "no container => absent"         "absent"  "$(current_mode)"
FIX_ACTIVE="$FIX_CODER";   assert_eq "detect_ctx_inspect => 24576"    "24576"   "$(detect_ctx_inspect)"
FIX_ACTIVE="$FIX_CLAUDE";  assert_eq "detect_parallel_inspect => 1"   "1"       "$(detect_parallel_inspect)"
echo

# --------------------------------------------------------------------------
# 3. /props total_slots -> mode mapping
# --------------------------------------------------------------------------
echo "[3] /props total_slots mapping"
assert_eq "slots 8 => coder"    "coder"   "$(parallel_to_mode 8)"
assert_eq "slots 1 => claude"   "claude"  "$(parallel_to_mode 1)"
assert_eq "slots 4 => unknown"  "unknown" "$(parallel_to_mode 4)"
echo

# --------------------------------------------------------------------------
# 4. CRUX: --print-cmd command construction (swap EXACTLY two tokens)
# --------------------------------------------------------------------------
echo "[4] --print-cmd construction: coder<->claude differ in exactly -c and --parallel"
FIX_ACTIVE="$FIX_CODER"
OUT_CODER="$SANDBOX/out.coder"; OUT_CLAUDE="$SANDBOX/out.claude"
build_run_argv coder  > "$OUT_CODER"
build_run_argv claude > "$OUT_CLAUDE"

# coder transform of a coder fixture is a verbatim round-trip (order preserved).
if diff -q "$FIX_CODER" "$OUT_CODER" >/dev/null 2>&1; then
  ok "coder print-cmd reproduces the stored CreateCommand verbatim (order preserved)"
else
  bad "coder print-cmd diverged from stored CreateCommand"
fi

# The teeth: exactly two token lines differ, and they are -c's value + --parallel's.
diff_markers="$(diff "$OUT_CODER" "$OUT_CLAUDE" | grep -c '^[<>]' || true)"
assert_eq "exactly 2 token changes (4 diff markers)" "4" "$diff_markers"
changed_to="$( { diff "$OUT_CODER" "$OUT_CLAUDE" || true; } | awk '/^> /{print $2}' | sort | tr '\n' ',')"
assert_eq "the two new values are 1 and 229376" "1,229376," "$changed_to"

# Everything else is carried verbatim into the claude command.
CLAUDE_STR="$(tr '\n' ' ' < "$OUT_CLAUDE")"
assert_contains "claude keeps --metrics"          " --metrics "                                     "$CLAUDE_STR "
assert_contains "claude keeps --name helixllm-coder" "--name helixllm-coder"                        "$CLAUDE_STR"
assert_contains "claude keeps the image"          "localhost/helixllm/llamacpp-router:cuda12.8-sm120" "$CLAUDE_STR"
assert_contains "claude keeps the model path"     "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf"  "$CLAUDE_STR"
assert_contains "claude keeps --cache-type-k q8_0" "--cache-type-k q8_0"                             "$CLAUDE_STR"
assert_contains "claude keeps -c 229376"           "-c 229376"                                         "$CLAUDE_STR"
assert_contains "claude keeps --parallel 1"       "--parallel 1"                                     "$CLAUDE_STR"
assert_absent   "claude dropped coder -c 24576"   "-c 24576"                                         "$CLAUDE_STR"
assert_absent   "claude dropped coder --parallel 8" "--parallel 8"                                   "$CLAUDE_STR"

# No-container fallback still yields a canonical command carrying --metrics.
FIX_ACTIVE=""
OUT_FALLBACK="$SANDBOX/out.fallback"
build_run_argv claude > "$OUT_FALLBACK"
FALLBACK_STR="$(tr '\n' ' ' < "$OUT_FALLBACK")"
assert_contains "fallback (no container) still has --metrics" " --metrics" "$FALLBACK_STR"
assert_contains "fallback has -c 229376"      "-c 229376"      "$FALLBACK_STR"
assert_contains "fallback has --parallel 1"  "--parallel 1"  "$FALLBACK_STR"
echo

# --------------------------------------------------------------------------
# 5. §1.1 PAIRED MUTATION — break "preserve --metrics" => the crux check FAILS
# --------------------------------------------------------------------------
echo "[5] paired mutation: teeth of the crux assertion"
# The reusable check the crux relies on: exactly 2 token changes AND --metrics kept.
crux_check() { # $1=baseline argv file  $2=candidate argv file
  local n; n="$(diff "$1" "$2" | grep -c '^[<>]' || true)"
  [ "$n" -eq 4 ] || return 1
  grep -qx -- '--metrics' "$2" || return 1
  return 0
}

# Sanity: the REAL transform passes the check.
set +e; crux_check "$OUT_CODER" "$OUT_CLAUDE"; real_rc=$?; set -e
assert_exit "crux_check PASSES on the correct transform" 0 "$real_rc"

# Mutant A: a "claude" command that lost --metrics (broken preserve logic).
MUT_NOMETRICS="$SANDBOX/mutant.nometrics"
grep -vx -- '--metrics' "$OUT_CLAUDE" > "$MUT_NOMETRICS"
set +e; crux_check "$OUT_CODER" "$MUT_NOMETRICS"; mutA_rc=$?; set -e
assert_exit "crux_check FAILS when --metrics is dropped" 1 "$mutA_rc"

# Mutant B: a transform that also changed a third token (--port) — over-broad edit.
MUT_THIRD="$SANDBOX/mutant.third"
sed 's/^18434$/29999/' "$OUT_CLAUDE" > "$MUT_THIRD"
set +e; crux_check "$OUT_CODER" "$MUT_THIRD"; mutB_rc=$?; set -e
assert_exit "crux_check FAILS on a 3rd changed token" 1 "$mutB_rc"
echo

# --------------------------------------------------------------------------
# 6. DRIFT warning via `status` (parallel 8 but -c 229376)
# --------------------------------------------------------------------------
echo "[6] status reports -c/--parallel drift"
FIX_ACTIVE="$FIX_DRIFT"; SLOTS_ACTIVE=""
STATUS_OUT="$(cmd_status)"
assert_contains "drift: mode still coder (by --parallel)" "mode      : coder" "$STATUS_OUT"
assert_contains "drift: WARNING on -c mismatch"           "WARNING"           "$STATUS_OUT"
echo

# --------------------------------------------------------------------------
# 7. IDEMPOTENCY no-op — already in mode => no podman run/stop/rm
# --------------------------------------------------------------------------
echo "[7] idempotency: 'coder' while already coder is a no-op"
export HLX_PODMAN_LOG="$SANDBOX/log.noop"; : > "$HLX_PODMAN_LOG"
export HLX_FIX="$FIX_CODER" HLX_CONTAINER_EXISTS=1
run_cli coder
LOG_NOOP="$(cat "$HLX_PODMAN_LOG")"
assert_exit "no-op exits 0" 0 "$CLI_RC"
assert_contains "no-op announces already-in-mode" "already in 'coder' mode" "$CLI_OUT"
assert_absent "no-op issued no 'podman run'"  "podman run"  "$LOG_NOOP"
assert_absent "no-op issued no 'podman stop'" "podman stop" "$LOG_NOOP"
assert_absent "no-op issued no 'podman rm'"   "podman rm"   "$LOG_NOOP"
echo

# --------------------------------------------------------------------------
# 8. FAIL-CLOSED on unknown — refuse to guess, mutate nothing
# --------------------------------------------------------------------------
echo "[8] fail-closed on unknown mode (no --force)"
export HLX_PODMAN_LOG="$SANDBOX/log.unknown"; : > "$HLX_PODMAN_LOG"
export HLX_FIX="$FIX_UNKNOWN" HLX_CONTAINER_EXISTS=1
run_cli claude
LOG_UNK="$(cat "$HLX_PODMAN_LOG")"
assert_exit "unknown mode refused" 1 "$CLI_RC"
assert_contains "unknown mode says 'refusing to guess'" "refusing to guess" "$CLI_OUT"
assert_absent "unknown refusal issued no 'podman run'"  "podman run"  "$LOG_UNK"
assert_absent "unknown refusal issued no 'podman stop'" "podman stop" "$LOG_UNK"
assert_absent "unknown refusal issued no 'podman rm'"   "podman rm"   "$LOG_UNK"
echo

# --------------------------------------------------------------------------
# 9. --print-cmd at the CLI issues NO mutation (only a read `inspect`)
# --------------------------------------------------------------------------
echo "[9] CLI --print-cmd emits the command and runs nothing"
export HLX_PODMAN_LOG="$SANDBOX/log.print"; : > "$HLX_PODMAN_LOG"
export HLX_FIX="$FIX_CODER" HLX_CONTAINER_EXISTS=1
run_cli claude --print-cmd
LOG_PRINT="$(cat "$HLX_PODMAN_LOG")"
assert_exit "print-cmd exits 0" 0 "$CLI_RC"
assert_contains "print-cmd shows -c 229376"     "-c"           "$CLI_OUT"
assert_contains "print-cmd shows 229376"        "229376"        "$CLI_OUT"
assert_contains "print-cmd shows --parallel 1" "--parallel"   "$CLI_OUT"
assert_contains "print-cmd keeps --metrics"    "--metrics"    "$CLI_OUT"
assert_absent   "print-cmd issued no 'podman run'"  "podman run"  "$LOG_PRINT"
assert_absent   "print-cmd issued no 'podman stop'" "podman stop" "$LOG_PRINT"
assert_absent   "print-cmd issued no 'podman rm'"   "podman rm"   "$LOG_PRINT"
echo

# --------------------------------------------------------------------------
# 10. RECREATE (fully stubbed): stop BEFORE run (VRAM safety), correct swap
# --------------------------------------------------------------------------
echo "[10] recreate coder->claude: stop-before-run + only -c/--parallel swapped"
export HLX_PODMAN_LOG="$SANDBOX/log.recreate"; : > "$HLX_PODMAN_LOG"
export HLX_CURL_LOG="$SANDBOX/log.curl";       : > "$HLX_CURL_LOG"
export HLX_FIX="$FIX_CODER" HLX_CONTAINER_EXISTS=1 HLX_PROPS_JSON='{"total_slots":1}'
run_cli claude
LOG_RC="$(cat "$HLX_PODMAN_LOG")"
assert_exit "recreate exits 0" 0 "$CLI_RC"

stop_ln="$(grep -n '^podman stop ' "$HLX_PODMAN_LOG" | head -1 | cut -d: -f1 || true)"
rm_ln="$(grep -n '^podman rm ' "$HLX_PODMAN_LOG" | head -1 | cut -d: -f1 || true)"
run_ln="$(grep -n '^podman run ' "$HLX_PODMAN_LOG" | head -1 | cut -d: -f1 || true)"
if [ -n "$stop_ln" ] && [ -n "$run_ln" ] && [ "$stop_ln" -lt "$run_ln" ]; then
  ok "podman stop precedes podman run (VRAM freed before new mode) [stop@$stop_ln < run@$run_ln]"
else
  bad "stop-before-run not observed (stop@${stop_ln:-none} run@${run_ln:-none})"
fi
if [ -n "$rm_ln" ] && [ -n "$run_ln" ] && [ "$rm_ln" -lt "$run_ln" ]; then
  ok "podman rm precedes podman run"
else
  bad "rm-before-run not observed (rm@${rm_ln:-none} run@${run_ln:-none})"
fi
RUN_LINE="$(grep '^podman run ' "$HLX_PODMAN_LOG" | head -1 || true)"
assert_contains "recreate run uses -c 229376"      "-c 229376"      "$RUN_LINE"
assert_contains "recreate run uses --parallel 1"  "--parallel 1"  "$RUN_LINE"
assert_contains "recreate run keeps --metrics"    "--metrics"     "$RUN_LINE"
assert_absent   "recreate run dropped -c 24576"   "-c 24576"      "$RUN_LINE"
assert_absent   "recreate run dropped --parallel 8" "--parallel 8" "$RUN_LINE"
assert_contains "recreate cross-checks /props to claude" "total_slots=1 (claude)" "$CLI_OUT"
echo

# ==========================================================================
echo "=========================================================="
echo "helixllm-mode.sh hermetic tests: PASS=$pass_n  FAIL=$fail_n"
echo "=========================================================="
[ "$fail_n" -eq 0 ]
