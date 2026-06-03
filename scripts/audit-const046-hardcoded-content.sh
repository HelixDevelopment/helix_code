#!/usr/bin/env bash
# audit-const046-hardcoded-content.sh — orchestrates the CONST-046
# audit walker over HelixCode's scannable roots. Round 92: SOFT-WARN
# (always exits 0). Round 99b: BASELINE-AWARE — pre-existing findings
# are accepted via baseline snapshot, but NEW violations beyond the
# baseline cause exit 1 in --fail-on-new mode.
# Usage:
#   bash scripts/audit-const046-hardcoded-content.sh                    # soft-warn (default)
#   bash scripts/audit-const046-hardcoded-content.sh --fail-on-new      # gate mode
#   bash scripts/audit-const046-hardcoded-content.sh --update-baseline  # refresh snapshot
#   bash scripts/audit-const046-hardcoded-content.sh --json --quiet     # JSON soft-warn
set -euo pipefail

# Resolve repo root (location of this script's parent's parent).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

AUDITOR_DIR="${REPO_ROOT}/scripts/audit_const046"
ALLOWLIST="${AUDITOR_DIR}/.allowlist"
BASELINE="${AUDITOR_DIR}/.baseline.json"

# Default roots — the set of trees CONST-046 actively governs in this
# repo. Third-party reference projects (cli_agents/, cli_agents_resources/)
# are excluded by the walker's directory-prune list.
DEFAULT_ROOTS=(
  "${REPO_ROOT}/helix_code"
)

# Add every own-org submodule under the flat submodules/<project> layout
# (post-CONST-052/Phase-2 flatten: helix_agent, helix_qa, challenges, panoptic,
# and all former dependencies/<org>/* now live at submodules/<leaf>).
if [[ -d "${REPO_ROOT}/submodules" ]]; then
  for d in "${REPO_ROOT}"/submodules/*/; do
    [[ -d "${d}" ]] && DEFAULT_ROOTS+=("${d%/}")
  done
fi

# Filter to only existing directories (some submodules may not be initialised).
ROOTS=()
for r in "${DEFAULT_ROOTS[@]}"; do
  [[ -d "${r}" ]] && ROOTS+=("${r}")
done

if [[ ${#ROOTS[@]} -eq 0 ]]; then
  echo "audit-const046: no scannable roots found under ${REPO_ROOT}" >&2
  exit 0
fi

# Join with commas for the auditor's --roots flag.
ROOTS_CSV=$(IFS=','; echo "${ROOTS[*]}")

# Build the auditor binary once into a tmp path. Avoid polluting bin/.
BUILD_TMP=$(mktemp -d)
trap 'rm -rf "${BUILD_TMP}"' EXIT

BIN="${BUILD_TMP}/audit_const046"
(
  cd "${AUDITOR_DIR}"
  go build -o "${BIN}" .
)

# Pass through any flags from the caller (--json, --quiet, --fail-on-new,
# --update-baseline). The --baseline path defaults to ${BASELINE} unless
# the caller supplies their own --baseline flag.
exec "${BIN}" --roots "${ROOTS_CSV}" --allowlist "${ALLOWLIST}" --baseline "${BASELINE}" "$@"
