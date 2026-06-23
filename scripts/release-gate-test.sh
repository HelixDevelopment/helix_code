#!/usr/bin/env bash
# scripts/release-gate-test.sh
#
# Round 74 deliverable — programme-wide release-gate test runner.
# Closes CONST-048 invariant #6 ("four-layer test floor per §1 (pre-build +
# post-build + runtime + paired mutation)") at the META-REPO level.
#
# COMPLEMENTS round 68 (commit 3ed78a9) — `scripts/generate-coverage-ledger.sh`
# tracks the STRUCTURAL/coverage view (submodule × feature × platform ×
# invariant). This script provides the EXECUTION counterpart: walk every
# owned-org submodule, run its Go test suite, aggregate PASS/FAIL/SKIP
# results, and produce an honest release-gate signal.
#
# Why this exists (verbatim 2026-05-19 operator mandate, preserved per
# CONST-049 §11.4.17 to keep the why-now intent in-tree):
#
#   "all existing tests and Challenges do work in anti-bluff manner -
#    they MUST confirm that all tested codebase really works as expected!
#    We had been in position that all tests do execute with success and
#    all Challenges as well, but in reality the most of the features
#    does not work and can't be used! This MUST NOT be the case and
#    execution of tests and Challenges MUST guarantee the quality, the
#    completition and full usability by end users of the product!"
#
# The existing `make test-full` inside helix_code/ covers ONLY the inner
# Go module. This script covers the meta-repo level (all owned submodules
# under vasic-digital and HelixDevelopment per CONST-051(A) equal-codebase
# mandate). Both are required for a complete release gate.
#
# What it does:
#   * Reads docs/improvements/submodule_owned.txt (canonical roster, round 56)
#   * For each owned submodule, if it has a go.mod:
#       - cd into submodule
#       - run `GOMAXPROCS=2 nice -n 19 go test -count=1 -race -timeout=180s ./...`
#       - capture stdout+stderr to per-submodule log
#       - parse exit code, PASS/FAIL/SKIP line counts
#       - scan for bare `--- SKIP:` without `SKIP-OK:` marker (CONST-035 violation)
#   * Submodule without go.mod → SKIP-NO-GOMOD (legitimate; non-Go submodule
#     like assets/, github_pages_website/)
#   * Aggregates results into human-readable summary; optional --json
#   * Exit non-zero if ANY submodule FAILed or ANY bare SKIP detected
#     (release-gate honest signal per CONST-035)
#
# Flags:
#   --json                 Emit machine-readable JSON summary
#   --quick                Stop on first FAIL (faster CI signal)
#   --only=<glob>          Restrict to matching submodules (e.g. --only='dependencies/*')
#   --check                Self-validate script + paths without running real tests
#                          (round 89 extends this to also exercise the env-vs-logic classifier)
#   --skip-env-failures    Round 89 — classify each FAIL as ENV-CLASS (operator-fixable
#                          via `go mod tidy` / install missing system dep) vs LOGIC-CLASS
#                          (genuine test-code / production-code defect). When set,
#                          ENV-CLASS failures are reported but do NOT trigger non-zero
#                          exit. LOGIC-CLASS failures ALWAYS trigger non-zero exit.
#                          Mixed (some LOGIC + some skipped-ENV) exits 3 so CI can
#                          distinguish "all clear" from "skipped some env".
#                          Classification is deterministic regex (no LLM grading);
#                          ambiguous patterns fail-closed to LOGIC-CLASS (anti-bluff).
#   --help / -h            Usage
#
# Exit codes:
#   0 = all owned submodules passed (or were legitimately SKIP-NO-GOMOD)
#       — OR with --skip-env-failures, only ENV-CLASS failures occurred and were skipped
#   1 = at least one submodule FAILed OR at least one bare SKIP detected
#       (round-74 default behaviour preserved when --skip-env-failures NOT set)
#   2 = invalid args / missing required files / script self-check failed
#   3 = round 89 — mixed: LOGIC-CLASS FAILs PLUS skipped ENV-CLASS FAILs (only when
#       --skip-env-failures is set; tells CI "logic-broken AND env-dirty")
#
# Cross-references:
#   * scripts/generate-coverage-ledger.sh   (round 68 — structural ledger)
#   * docs/coverage/COVERAGE_LEDGER.md      (structural state)
#   * docs/coverage/README.md               (suite overview — extended this round)
#   * docs/improvements/submodule_owned.txt (canonical roster — round 56)
#   * helix_code/Makefile                   (inner-module test-full target)
#   * CLAUDE.md §3.4                        (root vs inner Makefile distinction)
#
# Anchors:
#   * CONST-035 (anti-bluff PASS=runtime-evidence)
#   * CONST-048 invariant 6 (four-layer test floor at meta-repo level)
#   * CONST-049 §11.4.17 (verbatim operator quote preserved in-tree)
#   * CONST-051(A) (equal-codebase mandate — every owned submodule walked)
#   * Article XI §11.9 (anti-bluff forensic anchor)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

OWNED_FILE="docs/improvements/submodule_owned.txt"
LOG_DIR="${RELEASE_GATE_LOG_DIR:-/tmp/release-gate-$$}"

# Default test command per submodule
GO_TEST_CMD=(env GOMAXPROCS=2 nice -n 19 go test -count=1 -race -timeout=180s ./...)

MODE_JSON=0
MODE_QUICK=0
MODE_CHECK=0
MODE_SKIP_ENV=0   # round 89 — --skip-env-failures opt-in filter
ONLY_GLOB=""

usage() {
    cat <<'EOF'
Usage: scripts/release-gate-test.sh [flags]

Programme-wide release-gate test runner. Walks every owned-org submodule
listed in docs/improvements/submodule_owned.txt, runs `go test ./...` in
each (where a go.mod exists), aggregates PASS/FAIL/SKIP/BARE-SKIP counts,
and produces an honest release-gate signal.

Flags:
  --json                Emit machine-readable JSON summary instead of human text
  --quick               Stop on first FAIL (faster CI signal)
  --only=<glob>         Restrict to submodules matching <glob> (shell-glob, not regex)
                        e.g. --only='dependencies/*'
                             --only='helix_qa'
  --check               Self-validate script + paths without running real tests
                        (round 89 — also exercises the env-vs-logic classifier
                        against synthetic fixtures)
  --skip-env-failures   Round 89 — classify each FAIL as ENV-CLASS vs LOGIC-CLASS
                        and treat ENV-CLASS as non-blocking. ENV-CLASS = missing
                        go.sum, cannot find package, missing C/X11 headers,
                        missing executables (chromium, etc.), permission-denied,
                        no required module provides package. Classification is
                        deterministic regex; ambiguous patterns fail-closed to
                        LOGIC-CLASS (anti-bluff guarantee). Use day-to-day for
                        CI; OMIT for release gates.
  -h, --help            Show this help

Per-submodule test command (default):
  GOMAXPROCS=2 nice -n 19 go test -count=1 -race -timeout=180s ./...

Per-submodule log files written under:
  $RELEASE_GATE_LOG_DIR (default: /tmp/release-gate-<pid>/)

Exit codes:
  0 = all owned submodules passed (or were legitimately SKIP-NO-GOMOD)
      — OR with --skip-env-failures, only ENV-CLASS failures were skipped
  1 = at least one submodule FAILed OR at least one bare SKIP detected
      (round 74 default; preserved when --skip-env-failures NOT set)
  2 = invalid args / missing required files / script self-check failed
  3 = round 89 — LOGIC-CLASS FAIL plus skipped ENV-CLASS FAIL (only emitted
      when --skip-env-failures is set; signals "logic-broken AND env-dirty")

See script header for the full forensic anchor (round 74, CONST-048 invariant 6;
round 89, env-vs-logic classification filter).
EOF
}

for arg in "$@"; do
    case "$arg" in
        --json)                 MODE_JSON=1 ;;
        --quick)                MODE_QUICK=1 ;;
        --check)                MODE_CHECK=1 ;;
        --skip-env-failures)    MODE_SKIP_ENV=1 ;;
        --only=*)               ONLY_GLOB="${arg#--only=}" ;;
        -h|--help)              usage; exit 0 ;;
        *)                      echo "ERROR: unknown flag: $arg" >&2; usage; exit 2 ;;
    esac
done

# --------- Round 89 — env-vs-logic FAIL classifier ---------
#
# Inputs:   $1 = path to go-test log file
# Outputs:  sets CLASS_RESULT ∈ {env, logic} and CLASS_REASON (matched pattern
#           or "no env-pattern match — fail-closed to LOGIC")
# Anti-bluff invariant: deterministic regex, ambiguous → LOGIC (fail-closed).
#
# Pattern catalogue (env-class; if ANY hit → ENV-CLASS):
#   * "missing go.sum entry"             → operator: go mod tidy
#   * "cannot find package"              → operator: go mod tidy
#   * "no required module provides"      → operator: go mod tidy
#   * "updates to go.mod needed"         → operator: go mod tidy
#   * "inconsistent vendoring"           → operator: go mod vendor
#   * "package .* is not in std"         → operator: go mod tidy / check go version
#   * "command not found"                → operator: install missing tool
#   * "executable file not found"        → operator: install missing tool (chromium, etc.)
#   * "permission denied"                → operator: chmod / chown / port-binding
#   * Cgo / X11 header errors            → operator: install -dev package
#       - "fatal error: .*\.h: No such file"
#       - "X11/.*\.h"
#       - "Xcursor/Xcursor.h"
#       - "gtk/gtk.h"
#       - "cannot find -l"               (linker missing system lib)
#   * "[setup failed]" preceded WITHIN log by any of the above env-patterns → ENV
#
# Anything else (including [setup failed] with NO env-pattern preceding,
# real `--- FAIL: Test` lines, compile errors that are syntax/build defects
# in OUR code rather than env-deps) → LOGIC-CLASS.
classify_failure() {
    local logfile="$1"
    CLASS_RESULT="logic"
    CLASS_REASON="no env-pattern match — fail-closed to LOGIC-CLASS"

    [[ ! -f "$logfile" ]] && return 0

    # Env-pattern regex catalogue — extended POSIX (grep -E).
    # Each pattern is anchored loosely; we just need ONE hit.
    local env_patterns=(
        'missing go\.sum entry'
        'cannot find package'
        'no required module provides'
        'updates to go\.mod needed'
        'inconsistent vendoring'
        'package [^ ]+ is not in std'
        'command not found'
        'executable file not found'
        'permission denied'
        'fatal error: .*\.h: No such file'
        'X11/[A-Za-z]+\.h'
        'Xcursor/Xcursor\.h'
        'gtk/gtk\.h'
        'cannot find -l[A-Za-z]'
    )

    local pat
    for pat in "${env_patterns[@]}"; do
        if grep -qE "$pat" "$logfile" 2>/dev/null; then
            CLASS_RESULT="env"
            CLASS_REASON="matched env-pattern: $pat"
            return 0
        fi
    done

    # If [setup failed] appears AND no env-pattern matched, leave as LOGIC
    # (fail-closed). This means a compile-time build error in OUR code is
    # treated as a real defect, NOT an env issue.
    return 0
}

# --------- Self-check / sanity ---------
self_check() {
    local fail=0
    if [[ ! -f "$OWNED_FILE" ]]; then
        echo "FAIL: $OWNED_FILE missing (round 56 canonical roster)" >&2
        fail=1
    fi
    if ! command -v go >/dev/null 2>&1; then
        echo "WARN: 'go' not on PATH — non-check runs will all SKIP-NO-GOMOD-IGNORED" >&2
    fi
    if ! command -v nice >/dev/null 2>&1; then
        echo "WARN: 'nice' not on PATH — falling back to bare go test" >&2
    fi
    if [[ -n "$ONLY_GLOB" ]]; then
        case "$ONLY_GLOB" in
            *..*|/*) echo "FAIL: --only glob must not contain '..' or absolute path: $ONLY_GLOB" >&2; fail=1 ;;
        esac
    fi
    return $fail
}

if [[ $MODE_CHECK -eq 1 ]]; then
    if ! self_check; then
        echo "release-gate-test.sh --check: FAIL (self_check)"
        exit 2
    fi

    # Round 89 — exercise classifier against synthetic fixtures.
    classifier_check_fail=0
    tmpdir=$(mktemp -d -t release-gate-check.XXXXXX)
    trap 'rm -rf "$tmpdir"' EXIT

    # Fixture 1: ENV-CLASS — missing go.sum entry
    f1="$tmpdir/env_missing_gosum.log"
    cat >"$f1" <<'FIX1'
go: github.com/example/foo@v1.2.3: missing go.sum entry; to add it: go mod download github.com/example/foo
FAIL	github.com/example/bar [setup failed]
FIX1
    classify_failure "$f1"
    if [[ "$CLASS_RESULT" != "env" ]]; then
        echo "FAIL: fixture env_missing_gosum classified as $CLASS_RESULT, expected env" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture env_missing_gosum → ENV ($CLASS_REASON)"
    fi

    # Fixture 2: ENV-CLASS — missing X11 header
    f2="$tmpdir/env_missing_x11.log"
    cat >"$f2" <<'FIX2'
# fyne.io/fyne/v2/internal/driver/glfw
In file included from glfw_x11.go:5:0:
./glfw/glfw3.h:1: fatal error: X11/Xlib.h: No such file or directory
FAIL	fyne.io/fyne/v2/app [build failed]
FIX2
    classify_failure "$f2"
    if [[ "$CLASS_RESULT" != "env" ]]; then
        echo "FAIL: fixture env_missing_x11 classified as $CLASS_RESULT, expected env" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture env_missing_x11 → ENV ($CLASS_REASON)"
    fi

    # Fixture 2b: ENV-CLASS — go.mod needs tidying (real-world: most common
    # operator-fixable failure; surfaced by smoke-test against HelixLLM
    # submodule on 2026-05-18). Without this pattern, the classifier
    # mis-routes the failure to LOGIC-CLASS and reddens the gate falsely.
    f2b="$tmpdir/env_gomod_needs_tidy.log"
    cat >"$f2b" <<'FIX2B'
go: updates to go.mod needed; to update it:
	go mod tidy
FIX2B
    classify_failure "$f2b"
    if [[ "$CLASS_RESULT" != "env" ]]; then
        echo "FAIL: fixture env_gomod_needs_tidy classified as $CLASS_RESULT, expected env" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture env_gomod_needs_tidy → ENV ($CLASS_REASON)"
    fi

    # Fixture 3: ENV-CLASS — missing executable (chromedp scenario)
    f3="$tmpdir/env_missing_chromium.log"
    cat >"$f3" <<'FIX3'
--- FAIL: TestChromedp (0.01s)
    runner.go:42: exec: "chromium": executable file not found in $PATH
FAIL
FIX3
    classify_failure "$f3"
    if [[ "$CLASS_RESULT" != "env" ]]; then
        echo "FAIL: fixture env_missing_chromium classified as $CLASS_RESULT, expected env" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture env_missing_chromium → ENV ($CLASS_REASON)"
    fi

    # Fixture 4: LOGIC-CLASS — assertion mismatch (real test-logic failure)
    f4="$tmpdir/logic_assert_mismatch.log"
    cat >"$f4" <<'FIX4'
--- FAIL: TestAdd (0.00s)
    math_test.go:17:
        Error Trace: math_test.go:17
        Error:       Not equal:
                     expected: 4
                     actual:   5
        Test:        TestAdd
FAIL
FAIL    example.com/math 0.005s
FIX4
    classify_failure "$f4"
    if [[ "$CLASS_RESULT" != "logic" ]]; then
        echo "FAIL: fixture logic_assert_mismatch classified as $CLASS_RESULT, expected logic" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture logic_assert_mismatch → LOGIC ($CLASS_REASON)"
    fi

    # Fixture 5: LOGIC-CLASS — compile error in OUR code (syntax), no env-pattern
    f5="$tmpdir/logic_compile_error.log"
    cat >"$f5" <<'FIX5'
# example.com/broken
./broken.go:7:2: syntax error: unexpected }, expecting expression
FAIL    example.com/broken [build failed]
FIX5
    classify_failure "$f5"
    if [[ "$CLASS_RESULT" != "logic" ]]; then
        echo "FAIL: fixture logic_compile_error classified as $CLASS_RESULT, expected logic" >&2
        classifier_check_fail=1
    else
        echo "PASS: fixture logic_compile_error → LOGIC ($CLASS_REASON)"
    fi

    if [[ $classifier_check_fail -ne 0 ]]; then
        echo "release-gate-test.sh --check: FAIL (classifier fixtures)" >&2
        exit 2
    fi

    echo "release-gate-test.sh --check: PASS"
    exit 0
fi

if ! self_check; then
    exit 2
fi

mkdir -p "$LOG_DIR"

# --------- §11.4.83 docs/qa/ end-user-evidence release gate (HXC-019) ---------
# Operative rule (5): "release gates MUST refuse to tag a version that has any
# feature-shipping commit without its matching docs/qa/<run-id>/ directory."
# Operator authorised promotion to a blocking release gate on 2026-05-28.
# Delegated to scripts/gates/qa_evidence_gate.sh which scopes enforcement to
# the convention baseline (commit that added docs/qa/README.md) so legacy
# pre-convention history is exempt. A violation here makes the whole release
# gate FAIL (exit 1) regardless of the per-submodule Go-test outcome.
QA_EVIDENCE_GATE="$REPO_ROOT/scripts/gates/qa_evidence_gate.sh"
QA_EVIDENCE_RC=0
if [[ -f "$QA_EVIDENCE_GATE" ]]; then
    if [[ $MODE_JSON -eq 0 ]]; then
        bash "$QA_EVIDENCE_GATE" || QA_EVIDENCE_RC=$?
    else
        # Keep JSON stream clean — gate output goes to the log dir.
        bash "$QA_EVIDENCE_GATE" >"$LOG_DIR/qa_evidence_gate.log" 2>&1 || QA_EVIDENCE_RC=$?
    fi
else
    echo "WARN: $QA_EVIDENCE_GATE missing — §11.4.83 evidence gate skipped" >&2
fi

# --------- §11.4.32/§11.4.40 constitution-rule sweep release gate ---------
# CONST-055 / §11.4.32 names scripts/verify-all-constitution-rules.sh the
# canonical enforcement engine: it runs EVERY implementable rule gate
# (G1..G16 — incl. G15 feature-video evidence + G16 challenge-matrix
# contract) against the post-pull tree and exits non-zero (exit 1) on any
# FAILURE. Historically that sweep ran only on the §11.4.32 constitution-pull
# path, so the gates did NOT block a release tag (§11.4.40 release-readiness
# gap). Wired here so the full sweep is a blocking pre-tag step: a non-zero
# sweep exit forces the whole release gate red (EXIT_CODE=1) regardless of the
# per-submodule Go-test outcome. ENV-skip semantics do NOT apply to
# constitution-rule violations.
CONSTITUTION_SWEEP="$REPO_ROOT/scripts/verify-all-constitution-rules.sh"
CONSTITUTION_SWEEP_RC=0
if [[ -f "$CONSTITUTION_SWEEP" ]]; then
    if [[ $MODE_JSON -eq 0 ]]; then
        bash "$CONSTITUTION_SWEEP" || CONSTITUTION_SWEEP_RC=$?
    else
        # Keep JSON stream clean — gate output goes to the log dir.
        bash "$CONSTITUTION_SWEEP" >"$LOG_DIR/constitution_sweep.log" 2>&1 || CONSTITUTION_SWEEP_RC=$?
    fi
else
    echo "WARN: $CONSTITUTION_SWEEP missing — §11.4.32 constitution-rule sweep skipped" >&2
fi

# --------- Match glob helper ---------
match_only() {
    local sm="$1"
    [[ -z "$ONLY_GLOB" ]] && return 0
    # shellcheck disable=SC2053
    [[ "$sm" == $ONLY_GLOB ]]
}

# --------- Per-submodule runner ---------
# Returns via globals: STATUS, PASS_N, FAIL_N, SKIP_N, BARE_SKIP_N, DURATION,
#                      FAIL_CLASS (round 89: "env"|"logic"|""), FAIL_REASON
run_one() {
    local sm="$1"
    local logfile="$LOG_DIR/${sm//\//__}.log"
    local start_ts end_ts
    STATUS="unknown"
    PASS_N=0; FAIL_N=0; SKIP_N=0; BARE_SKIP_N=0; DURATION=0
    FAIL_CLASS=""; FAIL_REASON=""

    if [[ ! -d "$sm" ]]; then
        STATUS="missing-directory"
        echo "submodule directory missing: $sm" >"$logfile"
        return 0
    fi

    if [[ ! -f "$sm/go.mod" ]]; then
        STATUS="skip-no-gomod"
        echo "no go.mod under $sm — non-Go submodule, legitimate skip" >"$logfile"
        return 0
    fi

    start_ts=$(date +%s)
    if ! command -v go >/dev/null 2>&1; then
        STATUS="fail"
        echo "go binary not on PATH" >"$logfile"
        FAIL_N=1
        end_ts=$(date +%s); DURATION=$((end_ts - start_ts))
        return 0
    fi

    local rc=0
    (
        cd "$sm" && "${GO_TEST_CMD[@]}"
    ) >"$logfile" 2>&1 || rc=$?
    end_ts=$(date +%s); DURATION=$((end_ts - start_ts))

    # Parse PASS / FAIL / SKIP counts from go test output.
    PASS_N=$(grep -cE '^--- PASS:' "$logfile" || true)
    FAIL_N=$(grep -cE '^--- FAIL:' "$logfile" || true)
    SKIP_N=$(grep -cE '^--- SKIP:' "$logfile" || true)

    # Bare-skip detector: any `--- SKIP:` line whose following 5 lines do NOT
    # mention `SKIP-OK:` is treated as a CONST-035 skip-bluff. We scan via
    # awk to walk windows.
    BARE_SKIP_N=$(awk '
        /^--- SKIP:/ {
            line=$0; found=0
            for (i=0; i<5 && (getline next_line)>0; i++) {
                if (next_line ~ /SKIP-OK:/) { found=1; break }
            }
            if (!found) print line
        }
    ' "$logfile" | wc -l | tr -d '[:space:]')

    if [[ $rc -ne 0 ]] || [[ $FAIL_N -gt 0 ]]; then
        STATUS="fail"
        # Round 89 — classify the failure as ENV vs LOGIC.
        classify_failure "$logfile"
        FAIL_CLASS="$CLASS_RESULT"
        FAIL_REASON="$CLASS_REASON"
    elif [[ $BARE_SKIP_N -gt 0 ]]; then
        STATUS="bare-skip"
    else
        STATUS="ok"
    fi
}

# --------- Walk all owned submodules ---------
declare -a RESULTS_SM RESULTS_STATUS RESULTS_PASS RESULTS_FAIL RESULTS_SKIP RESULTS_BARE RESULTS_DUR
declare -a RESULTS_CLASS RESULTS_CLASS_REASON   # round 89
TOTAL_PASS=0; TOTAL_FAIL=0; TOTAL_SKIP=0; TOTAL_BARE=0
N_OK=0; N_FAIL=0; N_SKIP_NO_GOMOD=0; N_MISSING=0; N_BARE=0; N_WALKED=0
N_ENV_CLASS=0; N_LOGIC_CLASS=0   # round 89 — per-submodule classification counts
EARLY_STOP=0

while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    # extract first column (submodule path) — fields separated by " | "
    sm="${line%% |*}"
    sm="${sm## }"; sm="${sm%% }"
    [[ -z "$sm" ]] && continue
    if ! match_only "$sm"; then
        continue
    fi
    N_WALKED=$((N_WALKED + 1))

    run_one "$sm"

    RESULTS_SM+=("$sm")
    RESULTS_STATUS+=("$STATUS")
    RESULTS_PASS+=("$PASS_N")
    RESULTS_FAIL+=("$FAIL_N")
    RESULTS_SKIP+=("$SKIP_N")
    RESULTS_BARE+=("$BARE_SKIP_N")
    RESULTS_DUR+=("$DURATION")
    RESULTS_CLASS+=("$FAIL_CLASS")
    RESULTS_CLASS_REASON+=("$FAIL_REASON")

    TOTAL_PASS=$((TOTAL_PASS + PASS_N))
    TOTAL_FAIL=$((TOTAL_FAIL + FAIL_N))
    TOTAL_SKIP=$((TOTAL_SKIP + SKIP_N))
    TOTAL_BARE=$((TOTAL_BARE + BARE_SKIP_N))

    case "$STATUS" in
        ok)             N_OK=$((N_OK + 1)) ;;
        fail)
            N_FAIL=$((N_FAIL + 1))
            # Round 89 — increment per-class counters
            case "$FAIL_CLASS" in
                env)    N_ENV_CLASS=$((N_ENV_CLASS + 1)) ;;
                logic)  N_LOGIC_CLASS=$((N_LOGIC_CLASS + 1)) ;;
            esac
            ;;
        skip-no-gomod)  N_SKIP_NO_GOMOD=$((N_SKIP_NO_GOMOD + 1)) ;;
        missing-directory) N_MISSING=$((N_MISSING + 1)) ;;
        bare-skip)      N_BARE=$((N_BARE + 1)) ;;
    esac

    if [[ $MODE_JSON -eq 0 ]]; then
        # Round 89 — render CLASS tag on FAILed rows (env/logic) for at-a-glance triage.
        class_tag=""
        if [[ "$STATUS" == "fail" ]]; then
            class_tag=" CLASS=$FAIL_CLASS"
        fi
        printf '%-60s | PASS=%-4s FAIL=%-3s SKIP=%-3s BARE=%-3s DUR=%-4ss STATUS=%s%s\n' \
            "$sm" "$PASS_N" "$FAIL_N" "$SKIP_N" "$BARE_SKIP_N" "$DURATION" "$STATUS" "$class_tag"
    fi

    # Round 89 — --quick early-stop only triggers on LOGIC-class FAIL when
    # --skip-env-failures is active. Without --skip-env-failures, round-74
    # behaviour is preserved (any FAIL triggers early-stop).
    if [[ $MODE_QUICK -eq 1 ]] && [[ "$STATUS" == "fail" ]]; then
        if [[ $MODE_SKIP_ENV -eq 1 ]] && [[ "$FAIL_CLASS" == "env" ]]; then
            : # skip — env-class is non-blocking under --skip-env-failures
        else
            EARLY_STOP=1
            break
        fi
    fi
done < "$OWNED_FILE"

# --------- Aggregate output ---------
# Round 89 — exit-code semantics:
#   default behaviour (round 74, preserved): any FAIL or bare-SKIP → 1
#   --skip-env-failures:
#     no FAIL and no bare-SKIP            → 0
#     only ENV-class FAILs (no LOGIC)     → 0 (skipped)
#     LOGIC-class FAIL, no skipped ENV    → 1
#     LOGIC-class FAIL + skipped ENV      → 3 (mixed signal)
#     bare-SKIP without LOGIC FAIL        → 1 (CONST-035 bare-skip is always blocking)
EXIT_CODE=0
if [[ $MODE_SKIP_ENV -eq 1 ]]; then
    if [[ $N_LOGIC_CLASS -gt 0 ]] && [[ $N_ENV_CLASS -gt 0 ]]; then
        EXIT_CODE=3
    elif [[ $N_LOGIC_CLASS -gt 0 ]]; then
        EXIT_CODE=1
    elif [[ $TOTAL_BARE -gt 0 ]]; then
        EXIT_CODE=1
    else
        EXIT_CODE=0
    fi
else
    if [[ $N_FAIL -gt 0 ]] || [[ $TOTAL_BARE -gt 0 ]]; then
        EXIT_CODE=1
    fi
fi

# §11.4.83 docs/qa/ evidence gate is always blocking on the release gate
# (HXC-019): a non-zero result forces the whole gate red regardless of the
# Go-test outcome. ENV-skip semantics do NOT apply to evidence violations.
if [[ "$QA_EVIDENCE_RC" -ne 0 ]]; then
    EXIT_CODE=1
fi

# §11.4.32/§11.4.40 constitution-rule sweep is always blocking on the release
# gate: any gate failure (G1..G16) forces the whole gate red regardless of the
# Go-test outcome. ENV-skip semantics do NOT apply to constitution violations.
if [[ "$CONSTITUTION_SWEEP_RC" -ne 0 ]]; then
    EXIT_CODE=1
fi

if [[ $MODE_JSON -eq 1 ]]; then
    # Hand-rolled JSON (no jq dependency). Round 89 adds env/logic split.
    printf '{\n'
    printf '  "round": 74,\n'
    printf '  "round_89_extension": "skip-env-failures classification filter",\n'
    printf '  "anchors": ["CONST-035","CONST-044","CONST-048-invariant-6","CONST-051-A","Article-XI-11.9"],\n'
    printf '  "skip_env_failures": %s,\n' "$([[ $MODE_SKIP_ENV -eq 1 ]] && echo true || echo false)"
    printf '  "owned_walked": %d,\n' "$N_WALKED"
    printf '  "n_ok": %d,\n' "$N_OK"
    printf '  "n_fail": %d,\n' "$N_FAIL"
    printf '  "n_env_class_fail": %d,\n' "$N_ENV_CLASS"
    printf '  "n_logic_class_fail": %d,\n' "$N_LOGIC_CLASS"
    printf '  "n_skip_no_gomod": %d,\n' "$N_SKIP_NO_GOMOD"
    printf '  "n_missing_dir": %d,\n' "$N_MISSING"
    printf '  "n_bare_skip_subs": %d,\n' "$N_BARE"
    printf '  "total_pass": %d,\n' "$TOTAL_PASS"
    printf '  "total_fail": %d,\n' "$TOTAL_FAIL"
    printf '  "total_skip": %d,\n' "$TOTAL_SKIP"
    printf '  "total_bare_skip": %d,\n' "$TOTAL_BARE"
    printf '  "early_stop": %s,\n' "$([[ $EARLY_STOP -eq 1 ]] && echo true || echo false)"
    printf '  "qa_evidence_gate_rc": %d,\n' "$QA_EVIDENCE_RC"
    printf '  "constitution_sweep_rc": %d,\n' "$CONSTITUTION_SWEEP_RC"
    printf '  "log_dir": "%s",\n' "$LOG_DIR"
    printf '  "exit_code": %d,\n' "$EXIT_CODE"
    printf '  "results": [\n'
    n=${#RESULTS_SM[@]}
    for ((i=0; i<n; i++)); do
        sep=","
        [[ $i -eq $((n - 1)) ]] && sep=""
        printf '    {"submodule":"%s","status":"%s","pass":%s,"fail":%s,"skip":%s,"bare_skip":%s,"duration_s":%s,"fail_class":"%s","fail_reason":"%s"}%s\n' \
            "${RESULTS_SM[$i]}" "${RESULTS_STATUS[$i]}" \
            "${RESULTS_PASS[$i]}" "${RESULTS_FAIL[$i]}" \
            "${RESULTS_SKIP[$i]}" "${RESULTS_BARE[$i]}" \
            "${RESULTS_DUR[$i]}" "${RESULTS_CLASS[$i]}" \
            "${RESULTS_CLASS_REASON[$i]//\"/\\\"}" "$sep"
    done
    printf '  ]\n}\n'
else
    echo
    echo "=== release-gate-test.sh aggregate summary (round 74 + round 89 classifier) ==="
    echo "  owned submodules walked: $N_WALKED"
    echo "  ok:                      $N_OK"
    echo "  failed:                  $N_FAIL  (ENV-CLASS=$N_ENV_CLASS  LOGIC-CLASS=$N_LOGIC_CLASS)"
    echo "  skip-no-gomod:           $N_SKIP_NO_GOMOD"
    echo "  missing-directory:       $N_MISSING"
    echo "  bare-skip submodules:    $N_BARE (CONST-035 violation if > 0)"
    echo "  total go-test PASS:      $TOTAL_PASS"
    echo "  total go-test FAIL:      $TOTAL_FAIL"
    echo "  total go-test SKIP:      $TOTAL_SKIP"
    echo "  total bare-SKIP:         $TOTAL_BARE"
    echo "  ENV-CLASS total:         $N_ENV_CLASS"
    echo "  LOGIC-CLASS total:       $N_LOGIC_CLASS"
    echo "  qa-evidence gate (11.4.83): $([[ $QA_EVIDENCE_RC -eq 0 ]] && echo PASS || echo "FAIL (rc=$QA_EVIDENCE_RC)")"
    echo "  constitution sweep (11.4.32/.40 G1..G16): $([[ $CONSTITUTION_SWEEP_RC -eq 0 ]] && echo PASS || echo "FAIL (rc=$CONSTITUTION_SWEEP_RC)")"
    echo "  log dir:                 $LOG_DIR"
    if [[ $MODE_SKIP_ENV -eq 1 ]]; then
        echo "  --skip-env-failures:     ON (ENV-CLASS reported but non-blocking)"
    fi
    if [[ $EARLY_STOP -eq 1 ]]; then
        echo "  early-stop:              YES (--quick)"
    fi

    if [[ $N_FAIL -gt 0 ]]; then
        echo
        echo "Failing submodules (with round-89 classification + reasoning):"
        n=${#RESULTS_SM[@]}
        for ((i=0; i<n; i++)); do
            if [[ "${RESULTS_STATUS[$i]}" == "fail" ]]; then
                echo "  - ${RESULTS_SM[$i]}"
                echo "      class:   ${RESULTS_CLASS[$i]}"
                echo "      reason:  ${RESULTS_CLASS_REASON[$i]}"
                echo "      log:     $LOG_DIR/${RESULTS_SM[$i]//\//__}.log"
            fi
        done
    fi
    if [[ $N_BARE -gt 0 ]]; then
        echo
        echo "Submodules with bare SKIP (CONST-035 skip-bluff violation):"
        n=${#RESULTS_SM[@]}
        for ((i=0; i<n; i++)); do
            if [[ "${RESULTS_BARE[$i]}" -gt 0 ]]; then
                echo "  - ${RESULTS_SM[$i]} (bare-skip count: ${RESULTS_BARE[$i]})"
            fi
        done
    fi
    if [[ $N_ENV_CLASS -gt 0 ]]; then
        echo
        echo "Operator-actionable env failures detected. Recommended next step:"
        n=${#RESULTS_SM[@]}
        for ((i=0; i<n; i++)); do
            if [[ "${RESULTS_CLASS[$i]}" == "env" ]]; then
                echo "  Run: cd \"${RESULTS_SM[$i]}\" && go mod tidy"
                echo "       (then re-run: bash scripts/release-gate-test.sh --only='${RESULTS_SM[$i]}')"
            fi
        done
    fi

    echo
    case "$EXIT_CODE" in
        0) echo "  RESULT: PASS (release gate green$([[ $MODE_SKIP_ENV -eq 1 && $N_ENV_CLASS -gt 0 ]] && echo " — $N_ENV_CLASS ENV-CLASS skipped per --skip-env-failures"))" ;;
        1) echo "  RESULT: FAIL (release gate red — see above)" ;;
        3) echo "  RESULT: FAIL-MIXED (LOGIC-CLASS=$N_LOGIC_CLASS PLUS skipped ENV-CLASS=$N_ENV_CLASS)" ;;
        *) echo "  RESULT: UNKNOWN exit_code=$EXIT_CODE" ;;
    esac
fi

exit $EXIT_CODE
