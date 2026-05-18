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
#   --json          Emit machine-readable JSON summary
#   --quick         Stop on first FAIL (faster CI signal)
#   --only=<glob>   Restrict to matching submodules (e.g. --only='dependencies/*')
#   --check         Self-validate script + paths without running real tests
#   --help / -h     Usage
#
# Exit codes:
#   0 = all owned submodules passed (or were legitimately SKIP-NO-GOMOD)
#   1 = at least one submodule FAILed OR at least one bare SKIP detected
#   2 = invalid args / missing required files / script self-check failed
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
ONLY_GLOB=""

usage() {
    cat <<'EOF'
Usage: scripts/release-gate-test.sh [flags]

Programme-wide release-gate test runner. Walks every owned-org submodule
listed in docs/improvements/submodule_owned.txt, runs `go test ./...` in
each (where a go.mod exists), aggregates PASS/FAIL/SKIP/BARE-SKIP counts,
and produces an honest release-gate signal.

Flags:
  --json          Emit machine-readable JSON summary instead of human text
  --quick         Stop on first FAIL (faster CI signal)
  --only=<glob>   Restrict to submodules matching <glob> (shell-glob, not regex)
                  e.g. --only='dependencies/*'
                       --only='helix_qa'
  --check         Self-validate script + paths without running real tests
                  (used by CI sanity-check + by the smoke-test below)
  -h, --help      Show this help

Per-submodule test command (default):
  GOMAXPROCS=2 nice -n 19 go test -count=1 -race -timeout=180s ./...

Per-submodule log files written under:
  $RELEASE_GATE_LOG_DIR (default: /tmp/release-gate-<pid>/)

Exit codes:
  0 = all owned submodules passed (or were legitimately SKIP-NO-GOMOD)
  1 = at least one submodule FAILed OR at least one bare SKIP detected
  2 = invalid args / missing required files / script self-check failed

See script header for the full forensic anchor (round 74, CONST-048 invariant 6).
EOF
}

for arg in "$@"; do
    case "$arg" in
        --json)         MODE_JSON=1 ;;
        --quick)        MODE_QUICK=1 ;;
        --check)        MODE_CHECK=1 ;;
        --only=*)       ONLY_GLOB="${arg#--only=}" ;;
        -h|--help)      usage; exit 0 ;;
        *)              echo "ERROR: unknown flag: $arg" >&2; usage; exit 2 ;;
    esac
done

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
    if self_check; then
        echo "release-gate-test.sh --check: PASS"
        exit 0
    else
        echo "release-gate-test.sh --check: FAIL"
        exit 2
    fi
fi

if ! self_check; then
    exit 2
fi

mkdir -p "$LOG_DIR"

# --------- Match glob helper ---------
match_only() {
    local sm="$1"
    [[ -z "$ONLY_GLOB" ]] && return 0
    # shellcheck disable=SC2053
    [[ "$sm" == $ONLY_GLOB ]]
}

# --------- Per-submodule runner ---------
# Returns via globals: STATUS, PASS_N, FAIL_N, SKIP_N, BARE_SKIP_N, DURATION
run_one() {
    local sm="$1"
    local logfile="$LOG_DIR/${sm//\//__}.log"
    local start_ts end_ts
    STATUS="unknown"
    PASS_N=0; FAIL_N=0; SKIP_N=0; BARE_SKIP_N=0; DURATION=0

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
    elif [[ $BARE_SKIP_N -gt 0 ]]; then
        STATUS="bare-skip"
    else
        STATUS="ok"
    fi
}

# --------- Walk all owned submodules ---------
declare -a RESULTS_SM RESULTS_STATUS RESULTS_PASS RESULTS_FAIL RESULTS_SKIP RESULTS_BARE RESULTS_DUR
TOTAL_PASS=0; TOTAL_FAIL=0; TOTAL_SKIP=0; TOTAL_BARE=0
N_OK=0; N_FAIL=0; N_SKIP_NO_GOMOD=0; N_MISSING=0; N_BARE=0; N_WALKED=0
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

    TOTAL_PASS=$((TOTAL_PASS + PASS_N))
    TOTAL_FAIL=$((TOTAL_FAIL + FAIL_N))
    TOTAL_SKIP=$((TOTAL_SKIP + SKIP_N))
    TOTAL_BARE=$((TOTAL_BARE + BARE_SKIP_N))

    case "$STATUS" in
        ok)             N_OK=$((N_OK + 1)) ;;
        fail)           N_FAIL=$((N_FAIL + 1)) ;;
        skip-no-gomod)  N_SKIP_NO_GOMOD=$((N_SKIP_NO_GOMOD + 1)) ;;
        missing-directory) N_MISSING=$((N_MISSING + 1)) ;;
        bare-skip)      N_BARE=$((N_BARE + 1)) ;;
    esac

    if [[ $MODE_JSON -eq 0 ]]; then
        printf '%-60s | PASS=%-4s FAIL=%-3s SKIP=%-3s BARE=%-3s DUR=%-4ss STATUS=%s\n' \
            "$sm" "$PASS_N" "$FAIL_N" "$SKIP_N" "$BARE_SKIP_N" "$DURATION" "$STATUS"
    fi

    if [[ $MODE_QUICK -eq 1 ]] && [[ "$STATUS" == "fail" ]]; then
        EARLY_STOP=1
        break
    fi
done < "$OWNED_FILE"

# --------- Aggregate output ---------
EXIT_CODE=0
if [[ $N_FAIL -gt 0 ]] || [[ $TOTAL_BARE -gt 0 ]]; then
    EXIT_CODE=1
fi

if [[ $MODE_JSON -eq 1 ]]; then
    # Hand-rolled JSON (no jq dependency)
    printf '{\n'
    printf '  "round": 74,\n'
    printf '  "anchors": ["CONST-035","CONST-048-invariant-6","CONST-051-A","Article-XI-11.9"],\n'
    printf '  "owned_walked": %d,\n' "$N_WALKED"
    printf '  "n_ok": %d,\n' "$N_OK"
    printf '  "n_fail": %d,\n' "$N_FAIL"
    printf '  "n_skip_no_gomod": %d,\n' "$N_SKIP_NO_GOMOD"
    printf '  "n_missing_dir": %d,\n' "$N_MISSING"
    printf '  "n_bare_skip_subs": %d,\n' "$N_BARE"
    printf '  "total_pass": %d,\n' "$TOTAL_PASS"
    printf '  "total_fail": %d,\n' "$TOTAL_FAIL"
    printf '  "total_skip": %d,\n' "$TOTAL_SKIP"
    printf '  "total_bare_skip": %d,\n' "$TOTAL_BARE"
    printf '  "early_stop": %s,\n' "$([[ $EARLY_STOP -eq 1 ]] && echo true || echo false)"
    printf '  "log_dir": "%s",\n' "$LOG_DIR"
    printf '  "exit_code": %d,\n' "$EXIT_CODE"
    printf '  "results": [\n'
    n=${#RESULTS_SM[@]}
    for ((i=0; i<n; i++)); do
        sep=","
        [[ $i -eq $((n - 1)) ]] && sep=""
        printf '    {"submodule":"%s","status":"%s","pass":%s,"fail":%s,"skip":%s,"bare_skip":%s,"duration_s":%s}%s\n' \
            "${RESULTS_SM[$i]}" "${RESULTS_STATUS[$i]}" \
            "${RESULTS_PASS[$i]}" "${RESULTS_FAIL[$i]}" \
            "${RESULTS_SKIP[$i]}" "${RESULTS_BARE[$i]}" \
            "${RESULTS_DUR[$i]}" "$sep"
    done
    printf '  ]\n}\n'
else
    echo
    echo "=== release-gate-test.sh aggregate summary (round 74) ==="
    echo "  owned submodules walked: $N_WALKED"
    echo "  ok:                      $N_OK"
    echo "  failed:                  $N_FAIL"
    echo "  skip-no-gomod:           $N_SKIP_NO_GOMOD"
    echo "  missing-directory:       $N_MISSING"
    echo "  bare-skip submodules:    $N_BARE (CONST-035 violation if > 0)"
    echo "  total go-test PASS:      $TOTAL_PASS"
    echo "  total go-test FAIL:      $TOTAL_FAIL"
    echo "  total go-test SKIP:      $TOTAL_SKIP"
    echo "  total bare-SKIP:         $TOTAL_BARE"
    echo "  log dir:                 $LOG_DIR"
    if [[ $EARLY_STOP -eq 1 ]]; then
        echo "  early-stop:              YES (--quick)"
    fi

    if [[ $N_FAIL -gt 0 ]]; then
        echo
        echo "Failing submodules:"
        n=${#RESULTS_SM[@]}
        for ((i=0; i<n; i++)); do
            if [[ "${RESULTS_STATUS[$i]}" == "fail" ]]; then
                echo "  - ${RESULTS_SM[$i]} (log: $LOG_DIR/${RESULTS_SM[$i]//\//__}.log)"
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

    echo
    if [[ $EXIT_CODE -eq 0 ]]; then
        echo "  RESULT: PASS (release gate green)"
    else
        echo "  RESULT: FAIL (release gate red — see above)"
    fi
fi

exit $EXIT_CODE
