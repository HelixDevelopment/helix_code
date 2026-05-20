#!/usr/bin/env bash
# =============================================================================
# Script:   tools/codegraph/install.sh
# Purpose:  Idempotent installer for the pinned CodeGraph runtime
#           (npm package @colbymchenry/codegraph) into a project-local
#           tools/codegraph/node_modules/ prefix — no global npm mutation.
# Task ID:  CG3 (Phase A — CodeGraph incorporation)
# Authority: Cascaded from HelixCode root CLAUDE.md / CONSTITUTION.md.
#            Honours CONST-035 (anti-bluff: real install, real output),
#            CONST-051 (decoupling: project-local prefix, no host pollution),
#            CONST-053 (.gitignore: node_modules/ is gitignored).
#
# Behaviour:
#   1. Pre-checks Node.js (>=18 <25) and npm are on PATH.
#   2. Reads the pinned version from tools/codegraph/codegraph.version.
#   3. Runs `npm install --prefix <tools/codegraph> @colbymchenry/codegraph@<ver>`
#      — idempotent: re-running with the same pin is a no-op upgrade-check.
#   4. Resolves the `codegraph` binary to an absolute path and verifies it
#      executes (`codegraph --version`).
#
# Exit codes: 0 = installed + binary verified; non-zero = a precheck or the
#             install or the binary verification failed.
# Usage:      tools/codegraph/install.sh
# =============================================================================
set -euo pipefail

# --- Resolve this script's directory (absolute) ------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
VERSION_FILE="${SCRIPT_DIR}/codegraph.version"

# --- Precheck: codegraph.version exists --------------------------------------
if [ ! -f "${VERSION_FILE}" ]; then
  echo "ERROR: pinned version file not found: ${VERSION_FILE}" >&2
  exit 1
fi
CG_VERSION="$(tr -d '[:space:]' < "${VERSION_FILE}")"
if [ -z "${CG_VERSION}" ]; then
  echo "ERROR: ${VERSION_FILE} is empty — no version pinned" >&2
  exit 1
fi

# --- Precheck: node + npm on PATH --------------------------------------------
if ! command -v node >/dev/null 2>&1; then
  echo "ERROR: node is not on PATH. CodeGraph requires Node.js >=18 <25." >&2
  exit 1
fi
if ! command -v npm >/dev/null 2>&1; then
  echo "ERROR: npm is not on PATH." >&2
  exit 1
fi

# --- Precheck: Node major version is in the supported window (>=18 <25) ------
NODE_RAW="$(node --version)"                 # e.g. v22.19.0
NODE_MAJOR="$(echo "${NODE_RAW}" | sed 's/^v//' | cut -d. -f1)"
if ! echo "${NODE_MAJOR}" | grep -Eq '^[0-9]+$'; then
  echo "ERROR: could not parse Node major version from '${NODE_RAW}'." >&2
  exit 1
fi
if [ "${NODE_MAJOR}" -lt 18 ] || [ "${NODE_MAJOR}" -ge 25 ]; then
  echo "ERROR: Node ${NODE_RAW} unsupported. CodeGraph needs >=18 <25." >&2
  exit 1
fi

echo "==> CodeGraph install"
echo "    pinned version : ${CG_VERSION}"
echo "    node           : ${NODE_RAW}"
echo "    npm            : $(npm --version)"
echo "    prefix         : ${SCRIPT_DIR}"

# --- Install (idempotent: pinned spec; re-run is a no-op if already at pin) ---
echo "==> npm install --prefix ${SCRIPT_DIR} @colbymchenry/codegraph@${CG_VERSION}"
npm install --prefix "${SCRIPT_DIR}" \
  --no-audit --no-fund \
  "@colbymchenry/codegraph@${CG_VERSION}"

# --- Resolve the binary to an absolute path ----------------------------------
CG_BIN="${SCRIPT_DIR}/node_modules/.bin/codegraph"
if [ ! -x "${CG_BIN}" ]; then
  echo "ERROR: codegraph binary not found / not executable at ${CG_BIN}" >&2
  exit 1
fi

# --- Verify the binary actually runs -----------------------------------------
echo "==> Resolved binary : ${CG_BIN}"
echo "==> Binary self-check: codegraph --version"
"${CG_BIN}" --version

echo "==> CodeGraph ${CG_VERSION} installed OK."
echo "    binary (absolute path for MCP config): ${CG_BIN}"
