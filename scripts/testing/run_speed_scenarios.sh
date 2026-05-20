#!/bin/bash
# run_speed_scenarios.sh — speed-programme canonical-scenario runner (P0-T04).
#
# PURPOSE (constitution §11.4.18 — script documentation mandate)
#   Executes the four canonical speed-programme scenarios S1-S4 against a
#   deterministic large-repo fixture and emits per-scenario timing plus a
#   3-run variance summary. It is the reproducible measurement entry point
#   that makes every later phase's speedup claim falsifiable (R4 phased plan
#   docs/research/speed/04-phased-implementation-plan.md §3 P0-T04).
#
#   Phase 0 is the measurement baseline — this script changes NO production
#   code. It builds nothing in the product; it only times scenarios.
#
# SCENARIOS (defined in the shared manifest, see MANIFEST below)
#   S1 cold-start      CLI process spawn to ready
#   S2 llm-dispatch    single LLM generate-request dispatch
#   S3 repomap-build   repo-map build over the fixture tree
#   S4 content-search  grep over the fixture tree
#
# USAGE
#   scripts/testing/run_speed_scenarios.sh [options]
#
#   Options:
#     -r, --runs N       repeats per scenario for variance (default: 3)
#     -f, --files N      fixture file count (default: manifest default, 2000)
#     -s, --seed N       fixture seed (default: manifest default, deterministic)
#     -o, --out DIR      directory to write captured results into
#                        (default: docs/research/speed/baseline)
#     -k, --keep         keep the generated fixture directory after the run
#     -j, --json         also emit a JSON results file into --out
#     -h, --help         show this help and exit
#
# ENVIRONMENT
#   HELIX_SPEED_LLM_URL  if set, S2 dispatches against this real provider URL;
#                        if unset, S2 reports SKIP-OK (CONST-050: no fake LLM).
#
# OUTPUT
#   A human-readable table on stdout, plus a timestamped capture file under
#   the --out directory: speed-scenarios-<UTC-timestamp>.txt (and .json with -j).
#
# EXIT CODES
#   0  all non-skipped scenarios ran and the harness produced a result table
#   1  build failure, missing toolchain, or a fatal runner error
#
# ANTI-BLUFF (CONST-035 / Article XI §11.9)
#   The 3-run variance line IS the proof the harness is stable enough to
#   detect a 1.3x change. A run with no captured numbers is a bluff.
#
# POSIX / shellcheck: honest #!/bin/bash shebang; `bash -n` and `sh -n` clean.

set -euo pipefail

# ---------------------------------------------------------------------------
# Locate repo root (this script lives at <root>/scripts/testing/).
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
INNER_MODULE="${REPO_ROOT}/helix_code"
MANIFEST="${INNER_MODULE}/tests/performance/scenarios/scenarios.json"

# ---------------------------------------------------------------------------
# Defaults.
# ---------------------------------------------------------------------------
RUNS=3
FILES=0          # 0 => manifest default
SEED=0           # 0 => manifest default
OUT_DIR="${REPO_ROOT}/docs/research/speed/baseline"
KEEP_FIXTURE=0
EMIT_JSON=0

usage() {
    sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
}

# ---------------------------------------------------------------------------
# Argument parsing.
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
    case "$1" in
        -r|--runs)  RUNS="$2"; shift 2 ;;
        -f|--files) FILES="$2"; shift 2 ;;
        -s|--seed)  SEED="$2"; shift 2 ;;
        -o|--out)   OUT_DIR="$2"; shift 2 ;;
        -k|--keep)  KEEP_FIXTURE=1; shift ;;
        -j|--json)  EMIT_JSON=1; shift ;;
        -h|--help)  usage; exit 0 ;;
        *) echo "run_speed_scenarios: unknown argument: $1" >&2; usage; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Preconditions.
# ---------------------------------------------------------------------------
if ! command -v go >/dev/null 2>&1; then
    echo "run_speed_scenarios: 'go' toolchain not found on PATH" >&2
    exit 1
fi
if [ ! -f "${MANIFEST}" ]; then
    echo "run_speed_scenarios: scenario manifest missing: ${MANIFEST}" >&2
    exit 1
fi

mkdir -p "${OUT_DIR}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
CAPTURE_TXT="${OUT_DIR}/speed-scenarios-${TIMESTAMP}.txt"
CAPTURE_JSON="${OUT_DIR}/speed-scenarios-${TIMESTAMP}.json"

# ---------------------------------------------------------------------------
# Build the Go scenario runner binary.
# ---------------------------------------------------------------------------
RUNNER_BIN="$(mktemp -d)/speed_runner"
echo "==> building scenario runner"
( cd "${INNER_MODULE}" && go build -o "${RUNNER_BIN}" ./tests/performance/scenarios/cmd/runner )

# ---------------------------------------------------------------------------
# Generate the deterministic fixture.
# ---------------------------------------------------------------------------
FIXTURE_DIR="$(mktemp -d)/speed_fixture"
cleanup() {
    rm -rf "$(dirname "${RUNNER_BIN}")"
    if [ "${KEEP_FIXTURE}" -eq 0 ]; then
        rm -rf "$(dirname "${FIXTURE_DIR}")"
    else
        echo "==> fixture kept at: ${FIXTURE_DIR}"
    fi
}
trap cleanup EXIT

GEN_ARGS="-gen-fixture ${FIXTURE_DIR}"
[ "${SEED}" -ne 0 ]  && GEN_ARGS="${GEN_ARGS} -seed ${SEED}"
[ "${FILES}" -ne 0 ] && GEN_ARGS="${GEN_ARGS} -files ${FILES}"

echo "==> generating deterministic fixture"
# shellcheck disable=SC2086
( cd "${INNER_MODULE}" && "${RUNNER_BIN}" ${GEN_ARGS} )

# ---------------------------------------------------------------------------
# Run the scenarios and capture output.
# ---------------------------------------------------------------------------
echo "==> running scenarios S1-S4 (${RUNS} run(s) each)"
{
    echo "speed-programme canonical scenarios — capture ${TIMESTAMP}"
    echo "repo-root: ${REPO_ROOT}"
    echo "runs-per-scenario: ${RUNS}"
    echo "fixture: ${FIXTURE_DIR}"
    echo "go: $(go version)"
    echo "---"
    ( cd "${INNER_MODULE}" && "${RUNNER_BIN}" -run -runs "${RUNS}" -fixture "${FIXTURE_DIR}" )
} | tee "${CAPTURE_TXT}"

if [ "${EMIT_JSON}" -eq 1 ]; then
    ( cd "${INNER_MODULE}" && "${RUNNER_BIN}" -run -runs "${RUNS}" -fixture "${FIXTURE_DIR}" -json ) > "${CAPTURE_JSON}"
    echo "==> JSON results: ${CAPTURE_JSON}"
fi

echo "==> captured: ${CAPTURE_TXT}"
echo "==> done"
