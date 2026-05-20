#!/bin/bash
# competitor_speed_baseline.sh — speed-programme competitor wall-clock baseline (P0-T03).
#
# PURPOSE (constitution §11.4.18 — script documentation mandate)
#   Establishes the wall-clock numbers HelixCode must beat. For each competitor
#   AI CLI agent (Claude Code, Gemini CLI, Aider, Cline, Crush, plus OpenCode /
#   Qwen Code when present) it detects installation, then measures wall-clock
#   for comparable lightweight scenarios on the SAME host. It is the anti-bluff
#   competitor baseline that makes the programme's "3-5x faster" claim
#   falsifiable (R4 phased plan docs/research/speed/04-phased-implementation-plan.md
#   §3 P0-T03).
#
#   Phase 0 is the measurement baseline — this script changes NO production
#   code and builds nothing. It only times competitor binaries already on the
#   host. It NEVER fabricates a number: an agent that is not installed yields a
#   SKIP-OK row (CONST-035 / §11.4.6 no-guessing; §11.4.3 topology-dispatch).
#
# SCENARIOS
#   S1 cold-start   `<agent> --version` (or `--help`) wall-clock — no LLM cost.
#                   This is the DEFAULT measurement: safe, repeatable, free.
#   S2-S4 llm       LLM-invoking scenarios that would cost real API tokens are
#                   gated behind COMPETITOR_BENCH_LLM=1 and are NOT run by
#                   default. When that flag is unset (the default) only S1 runs.
#                   S2-S4 are intentionally left as documented placeholders for
#                   a future opt-in token-budgeted run; this script will not
#                   spend real API tokens without the explicit env flag.
#
# USAGE
#   scripts/testing/competitor_speed_baseline.sh [options]
#
#   Options:
#     -r, --runs N    repeats per scenario for variance (default: 3)
#     -o, --out FILE  capture file to write the results table into
#                     (default: docs/research/speed/baseline/competitor-baseline-<UTC>.md)
#     -q, --quiet     suppress per-run progress lines (table still printed)
#     -h, --help      show this help and exit
#
# ENVIRONMENT
#   COMPETITOR_BENCH_LLM   if set to 1, ALSO attempt LLM-invoking scenarios
#                          S2-S4 (NOT implemented as token-spending runs here —
#                          see SCENARIOS note). Default unset: cold-start only.
#
# OUTPUT
#   A Markdown results table on stdout AND written to the capture file: one row
#   per agent per scenario with measured wall-clock (min/median/max over N
#   runs), or an explicit `SKIP-OK` row for agents absent on the host.
#
# EXIT CODES
#   0  the harness ran, every installed agent was measured, table emitted
#   1  fatal harness error (no timing tool, unwritable output path)
#
# ANTI-BLUFF (CONST-035 / Article XI §11.9)
#   Every number in the table is a real captured wall-clock from THIS host in
#   THIS run. Absent agents are SKIP-OK, never fabricated. The raw `time`
#   output is appended verbatim so the table is independently checkable.
#
# POSIX / shellcheck: honest #!/bin/bash shebang; `bash -n` and `sh -n` clean.

set -euo pipefail

# ---------------------------------------------------------------------------
# Locate repo root (this script lives at <root>/scripts/testing/).
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BASELINE_DIR="${REPO_ROOT}/docs/research/speed/baseline"

# ---------------------------------------------------------------------------
# Defaults + argument parsing.
# ---------------------------------------------------------------------------
RUNS=3
QUIET=0
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_FILE="${BASELINE_DIR}/competitor-baseline-${TIMESTAMP}.md"

while [ $# -gt 0 ]; do
    case "$1" in
        -r|--runs)  RUNS="$2";  shift 2 ;;
        -o|--out)   OUT_FILE="$2"; shift 2 ;;
        -q|--quiet) QUIET=1; shift ;;
        -h|--help)
            sed -n '2,60p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown option: $1" >&2
            exit 1
            ;;
    esac
done

case "${RUNS}" in
    ''|*[!0-9]*) echo "--runs must be a positive integer" >&2; exit 1 ;;
esac
[ "${RUNS}" -ge 1 ] || { echo "--runs must be >= 1" >&2; exit 1; }

mkdir -p "$(dirname "${OUT_FILE}")"

# ---------------------------------------------------------------------------
# Pick a timing tool. Prefer GNU /usr/bin/time -v for rich detail; otherwise
# fall back to the shell builtin `time`. Either way we compute wall-clock from
# a wrapping `date` measurement so the number is tool-independent.
# ---------------------------------------------------------------------------
GNU_TIME=""
if [ -x /usr/bin/time ]; then
    GNU_TIME="/usr/bin/time"
fi

# ---------------------------------------------------------------------------
# Competitor catalogue. Each entry: "label|command|scenario-args".
# scenario-args is the S1 cold-start invocation (no LLM cost).
# ---------------------------------------------------------------------------
AGENT_LABELS="Claude Code|Gemini CLI|Aider|Cline|Crush|OpenCode|Qwen Code"
AGENT_CMDS="claude|gemini|aider|cline|crush|opencode|qwen"
S1_ARGS="--version|--version|--version|--version|--version|--version|--version"

# ---------------------------------------------------------------------------
# millis_now — wall-clock in milliseconds (epoch).
# ---------------------------------------------------------------------------
millis_now() {
    # date +%s%3N is GNU coreutils; fall back to seconds*1000 if unavailable.
    local ms
    ms="$(date +%s%3N 2>/dev/null || true)"
    case "${ms}" in
        ''|*[!0-9]*) echo "$(( $(date +%s) * 1000 ))" ;;
        *) echo "${ms}" ;;
    esac
}

# ---------------------------------------------------------------------------
# measure_agent — runs one agent's S1 scenario RUNS times, captures wall-clock.
#   $1 label, $2 command, $3 scenario-args
# Appends a result line to TABLE_ROWS and raw evidence to RAW_EVIDENCE.
# ---------------------------------------------------------------------------
measure_agent() {
    local label="$1" cmd="$2" args="$3"
    local resolved
    if ! resolved="$(command -v "${cmd}" 2>/dev/null)"; then
        TABLE_ROWS="${TABLE_ROWS}| ${label} | \`${cmd}\` | S1 cold-start | SKIP-OK | not installed on host |
"
        RAW_EVIDENCE="${RAW_EVIDENCE}
### ${label} (\`${cmd}\`) — SKIP-OK
CONST-§11.4.3 topology-dispatch: \`${cmd}\` not found on PATH; no number fabricated.
"
        [ "${QUIET}" -eq 1 ] || echo "  ${label}: SKIP-OK (not installed)"
        return 0
    fi

    [ "${QUIET}" -eq 1 ] || echo "  ${label}: measuring S1 cold-start (${RUNS} runs) ..."

    local i start end dur sample_count=0
    local min="" max="" sum=0
    local samples=""
    local raw=""

    for i in $(seq 1 "${RUNS}"); do
        start="$(millis_now)"
        # Run the agent; capture combined output for the non-empty self-check.
        local agent_out=""
        local agent_rc=0
        if [ -n "${GNU_TIME}" ]; then
            agent_out="$( "${GNU_TIME}" -v "${cmd}" ${args} 2>&1 || true )"
        else
            agent_out="$( { time "${cmd}" ${args}; } 2>&1 || true )"
        fi
        agent_rc=$?
        end="$(millis_now)"
        dur=$(( end - start ))
        [ "${dur}" -ge 0 ] || dur=0

        # Self-verify: a measured agent MUST actually have produced output.
        if [ -z "${agent_out}" ]; then
            raw="${raw}run ${i}: WARNING empty output (agent produced nothing)
"
        fi

        samples="${samples}${dur} "
        sum=$(( sum + dur ))
        sample_count=$(( sample_count + 1 ))
        if [ -z "${min}" ] || [ "${dur}" -lt "${min}" ]; then min="${dur}"; fi
        if [ -z "${max}" ] || [ "${dur}" -gt "${max}" ]; then max="${dur}"; fi

        raw="${raw}run ${i}: wall=${dur}ms rc=${agent_rc}
$(printf '%s' "${agent_out}" | sed 's/^/    /' | head -20)
"
    done

    # Median of the sorted samples.
    local median
    median="$(printf '%s\n' ${samples} | sort -n | awk '{a[NR]=$1} END{ if(NR%2==1){print a[(NR+1)/2]} else {print int((a[NR/2]+a[NR/2+1])/2)} }')"
    local mean=$(( sum / sample_count ))

    TABLE_ROWS="${TABLE_ROWS}| ${label} | \`${cmd}\` | S1 cold-start | ${median} ms | min ${min} / mean ${mean} / max ${max} ms (n=${sample_count}) |
"
    RAW_EVIDENCE="${RAW_EVIDENCE}
### ${label} (\`${cmd}\`) — S1 cold-start
binary: ${resolved}
samples (ms): ${samples}
median: ${median} ms | mean: ${mean} ms | min: ${min} ms | max: ${max} ms
\`\`\`
${raw}\`\`\`
"
    [ "${QUIET}" -eq 1 ] || echo "    ${label}: median ${median} ms (min ${min} / max ${max})"
}

# ---------------------------------------------------------------------------
# Drive the catalogue.
# ---------------------------------------------------------------------------
TABLE_ROWS=""
RAW_EVIDENCE=""

[ "${QUIET}" -eq 1 ] || {
    echo "==> competitor wall-clock baseline (P0-T03)"
    echo "    host: $(uname -srm)"
    echo "    runs per scenario: ${RUNS}"
    echo "    timing tool: ${GNU_TIME:-shell builtin time}"
    echo "    LLM scenarios (S2-S4): ${COMPETITOR_BENCH_LLM:+ENABLED via COMPETITOR_BENCH_LLM}${COMPETITOR_BENCH_LLM:-disabled (default; set COMPETITOR_BENCH_LLM=1 to opt in)}"
    echo "---"
}

# Iterate the parallel pipe-delimited catalogue lists.
IDX=1
while :; do
    label="$(printf '%s' "${AGENT_LABELS}" | cut -d'|' -f"${IDX}")"
    cmd="$(printf '%s' "${AGENT_CMDS}" | cut -d'|' -f"${IDX}")"
    args="$(printf '%s' "${S1_ARGS}" | cut -d'|' -f"${IDX}")"
    [ -n "${label}" ] || break
    measure_agent "${label}" "${cmd}" "${args}"
    IDX=$(( IDX + 1 ))
done

# ---------------------------------------------------------------------------
# S2-S4 — explicit, env-gated, NOT run by default (no real API token spend).
# ---------------------------------------------------------------------------
LLM_NOTE=""
if [ "${COMPETITOR_BENCH_LLM:-0}" = "1" ]; then
    LLM_NOTE="**S2-S4 (LLM-invoking):** \`COMPETITOR_BENCH_LLM=1\` was set, but
token-spending LLM scenarios are intentionally not auto-run by this harness —
they require per-agent prompt/credential setup and a token budget. Run them
manually per agent and append the captured numbers. The cold-start S1 figures
above are the no-cost baseline."
else
    LLM_NOTE="**S2-S4 (LLM-invoking):** skipped — they would cost real API tokens.
Set \`COMPETITOR_BENCH_LLM=1\` to opt in. The S1 cold-start figures above are
the default no-cost baseline (CONST-050: no fake LLM, no fabricated number)."
fi

# ---------------------------------------------------------------------------
# Emit the Markdown report.
# ---------------------------------------------------------------------------
{
    echo "<!--"
    echo "Document-Metadata (constitution §11.4.44)"
    echo "Revision: 1"
    echo "Generated: ${TIMESTAMP}"
    echo "Authority: HelixCode speed programme — P0-T03 competitor wall-clock baseline."
    echo "           Produced by scripts/testing/competitor_speed_baseline.sh."
    echo "Scope:     Captured measurement artefact. Numbers are real wall-clock from"
    echo "           the host below; absent agents are SKIP-OK, never fabricated"
    echo "           (CONST-035 anti-bluff / §11.4.6 no-guessing)."
    echo "-->"
    echo
    echo "# Competitor Wall-Clock Baseline — ${TIMESTAMP}"
    echo
    echo "Speed-programme Phase 0 task **P0-T03**. Establishes the competitor"
    echo "numbers HelixCode must beat (R4 §3). Captured by"
    echo "\`scripts/testing/competitor_speed_baseline.sh\`."
    echo
    echo "| Field | Value |"
    echo "|---|---|"
    echo "| Host | \`$(uname -srm)\` |"
    echo "| Generated (UTC) | ${TIMESTAMP} |"
    echo "| Runs per scenario | ${RUNS} |"
    echo "| Timing tool | \`${GNU_TIME:-shell builtin time}\` |"
    echo
    echo "## Measured wall-clock"
    echo
    echo "| Agent | Command | Scenario | Median | Detail |"
    echo "|---|---|---|---|---|"
    printf '%s' "${TABLE_ROWS}"
    echo
    echo "S1 cold-start = \`<agent> --version\` wall-clock — a safe, no-LLM-cost,"
    echo "repeatable measurement. \`SKIP-OK\` rows are agents not installed on this"
    echo "host; per CONST-035 / §11.4.6 no number is fabricated for them."
    echo
    echo "${LLM_NOTE}"
    echo
    echo "## Raw \`time\` evidence (anti-bluff — independently checkable)"
    printf '%s' "${RAW_EVIDENCE}"
} > "${OUT_FILE}"

[ "${QUIET}" -eq 1 ] || {
    echo "---"
    echo "==> captured: ${OUT_FILE}"
    echo "==> done"
}

# Echo the table to stdout too so a CI/integration check can grep it.
printf '%s' "${TABLE_ROWS}"
