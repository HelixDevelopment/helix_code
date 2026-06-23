#!/usr/bin/env bash
# helix_code/scripts/tests/run_challenge_matrix_test.sh
#
# §11.4.135 STANDING REGRESSION GUARD for ../run-challenge-matrix.sh
# (the §6.X Android-emulator gate runner — HXC-108 / HXC-112).
#
# PURPOSE
#   Pin the runner's per-OS-dispatch + preflight contract so it cannot
#   silently regress. The single most dangerous regression class for this
#   runner is the honest-SKIP path (§11.4.112): if the macOS / no-kvm path
#   ever starts exiting 0 (PASS) instead of 77 (SKIP), a naive caller would
#   read an honest "can't run here" as a successful recording — a §11.4
#   PASS-bluff at the gate-dispatch layer. This guard makes that regression
#   loud.
#
# CONTRACT ASSERTED (read from the runner header + source, 2026-06-23):
#   missing mode            -> die,  exit 2
#   --help                  -> usage, exit 0
#   unknown mode            -> die,  exit 2
#   unknown option          -> die,  exit 2
#   --record-seconds 0      -> die,  exit 2  (preflight numeric validation)
#   --record-seconds abc    -> die,  exit 2
#   --boot-timeout -5       -> die,  exit 2
#   record (valid) @ Darwin -> SKIP, exit 77, message contains "SKIP:" + "macOS"
#                              + the cited evidence path (§11.4.112).
#
#   Linux-specific KVM cases (no /dev/kvm -> 77; kvm-not-rw -> die 2) are
#   NOT mechanically exercisable on this macOS host. They are asserted-by-
#   honest-SKIP here (§11.4.3) rather than faked. When this guard runs on a
#   Linux x86_64 host WITHOUT /dev/kvm it additionally exercises the
#   no-kvm -> 77 path for real.
#
# ANTI-BLUFF (§1.1 / §11.4.135)
#   This guard is paired with a self-mutation proof: mutate a COPY of the
#   runner so its macOS path exits 0, run the guard against the copy, and
#   show it FAILs. See run_challenge_matrix_mutation_proof.sh sibling, OR
#   run this file with HELIX_RCM_TARGET=<path-to-mutated-copy> to point the
#   guard at an alternate runner (the mutation harness uses exactly that).
#
# USAGE
#   bash run_challenge_matrix_test.sh                 # guard the real runner
#   HELIX_RCM_TARGET=/path/to/runner.sh bash run_challenge_matrix_test.sh
#
# EXIT: 0 = all assertions held; 1 = at least one assertion failed.

set -u

# ---------------------------------------------------------------------------
# Locate the runner under test (override via HELIX_RCM_TARGET for §1.1 proof).
# ---------------------------------------------------------------------------
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNNER="${HELIX_RCM_TARGET:-${SELF_DIR}/../run-challenge-matrix.sh}"

if [ ! -f "$RUNNER" ]; then
  printf 'FATAL: runner under test not found: %s\n' "$RUNNER" >&2
  exit 1
fi

PASS=0
FAIL=0
HOST_OS="$(uname -s)"
HOST_ARCH="$(uname -m)"

# Run the runner, capture combined output + exit code.
# Sets globals RC_EXIT and RC_OUT.
_run() {
  RC_OUT="$(bash "$RUNNER" "$@" 2>&1)"
  RC_EXIT=$?
}

# assert_exit <expected-code> <label> -- args...
assert_exit() {
  local want="$1" label="$2"; shift 2
  _run "$@"
  if [ "$RC_EXIT" -eq "$want" ]; then
    printf '  PASS  [exit %s] %s\n' "$want" "$label"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  [want exit %s, got %s] %s\n' "$want" "$RC_EXIT" "$label" >&2
    printf '        output: %s\n' "$(printf '%s' "$RC_OUT" | tr '\n' ' ' | cut -c1-200)" >&2
    FAIL=$((FAIL + 1))
  fi
}

# assert_exit_and_msg <expected-code> <substr> <label> -- args...
assert_exit_and_msg() {
  local want="$1" substr="$2" label="$3"; shift 3
  _run "$@"
  local ok_exit=0 ok_msg=0
  [ "$RC_EXIT" -eq "$want" ] && ok_exit=1
  case "$RC_OUT" in *"$substr"*) ok_msg=1 ;; esac
  if [ "$ok_exit" -eq 1 ] && [ "$ok_msg" -eq 1 ]; then
    printf '  PASS  [exit %s + "%s"] %s\n' "$want" "$substr" "$label"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  [want exit %s + "%s"; got exit %s, msg-match=%s] %s\n' \
      "$want" "$substr" "$RC_EXIT" "$ok_msg" "$label" >&2
    printf '        output: %s\n' "$(printf '%s' "$RC_OUT" | tr '\n' ' ' | cut -c1-200)" >&2
    FAIL=$((FAIL + 1))
  fi
}

printf '=== run-challenge-matrix.sh regression guard (§11.4.135) ===\n'
printf 'runner : %s\n' "$RUNNER"
printf 'host   : OS=%s ARCH=%s\n\n' "$HOST_OS" "$HOST_ARCH"

# ---------------------------------------------------------------------------
# Arg-parsing / preflight contract — exercisable on ANY host (these paths
# return before the container/OS-accelerator work).
# ---------------------------------------------------------------------------
printf -- '--- arg-parsing + preflight contract (host-agnostic) ---\n'
assert_exit       2 "no mode given -> die 2"                                          # (no args)
assert_exit       0 "--help -> exit 0"                              --help
assert_exit       0 "-h -> exit 0"                                  -h
assert_exit       2 "unknown mode -> die 2"                         frobnicate
assert_exit       2 "unknown option -> die 2"                       record --bogus
assert_exit       2 "--record-seconds 0 (non-positive) -> die 2"    record --record-seconds 0
assert_exit       2 "--record-seconds abc (non-numeric) -> die 2"   record --record-seconds abc
assert_exit       2 "--boot-timeout -5 (negative) -> die 2"         record --boot-timeout -5
assert_exit       2 "--boot-timeout 0 -> die 2"                     record --boot-timeout 0

# ---------------------------------------------------------------------------
# Per-OS dispatch contract (§11.4.81) — branch depends on the host.
# ---------------------------------------------------------------------------
printf -- '\n--- per-OS dispatch contract (§11.4.81 / §11.4.112) ---\n'
case "$HOST_OS" in
  Darwin)
    # The load-bearing anti-bluff assertion: macOS MUST honest-SKIP (77),
    # NEVER PASS (0). Also confirm the SKIP message is honest + cites
    # evidence (§11.4.112 / §11.4.3) rather than a bare exit.
    assert_exit_and_msg 77 "SKIP:"  "macOS record -> SKIP line printed"     record --apk /tmp/nonexistent.apk
    assert_exit_and_msg 77 "macOS"  "macOS record -> message names macOS"   record --apk /tmp/nonexistent.apk
    assert_exit_and_msg 77 "feasibility.md" \
        "macOS record -> SKIP cites §11.4.112 evidence path"                record --apk /tmp/nonexistent.apk
    # Cross-check: the SKIP must NOT be exit 0 (the exact regression we guard).
    _run record --apk /tmp/nonexistent.apk
    if [ "$RC_EXIT" -eq 0 ]; then
      printf '  FAIL  [macOS path returned PASS(0) — honest-SKIP regressed to a bluff!]\n' >&2
      FAIL=$((FAIL + 1))
    else
      printf '  PASS  [macOS path is NOT exit 0 — honest-SKIP intact]\n'
      PASS=$((PASS + 1))
    fi
    printf '  SKIP-OK: Linux no-kvm->77 and kvm-not-rw->die2 paths are not\n'
    printf '           mechanically exercisable on macOS (§11.4.3 honest gap).\n'
    ;;
  Linux)
    if [ "$HOST_ARCH" != "x86_64" ] && [ "$HOST_ARCH" != "amd64" ]; then
      assert_exit_and_msg 77 "SKIP:" "Linux non-x86_64 record -> SKIP 77" record --apk /tmp/nonexistent.apk
    elif [ ! -e /dev/kvm ]; then
      # Real exercise of the no-kvm SKIP path on a Linux host without KVM.
      assert_exit_and_msg 77 "SKIP:" "Linux x86_64 no-/dev/kvm -> SKIP 77" record --apk /tmp/nonexistent.apk
    else
      # /dev/kvm present: cannot deterministically assert 0/2/3 without a
      # built emulator image + APK + rw kvm; honest SKIP of this assertion.
      printf '  SKIP-OK: /dev/kvm present — full record-flow assertion needs a\n'
      printf '           built §6.X image + APK + rw kvm (§11.4.3 honest gap).\n'
    fi
    ;;
  *)
    assert_exit_and_msg 77 "SKIP:" "unsupported OS record -> SKIP 77" record --apk /tmp/nonexistent.apk
    ;;
esac

# ---------------------------------------------------------------------------
# Verdict
# ---------------------------------------------------------------------------
printf '\n=== RESULT: %d passed, %d failed ===\n' "$PASS" "$FAIL"
if [ "$FAIL" -ne 0 ]; then
  exit 1
fi
exit 0
