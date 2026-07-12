#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# run-helixcode-http-banks.sh -- Drives the HelixCode-specific HelixQA
# HTTP test banks (submodules/helix_qa/banks/helixcode-*.{yaml,json})
# against a real, running HelixCode server (cmd/server), using the
# helix_qa CLI's own `http` subcommand.
#
# HXC-124 root cause + consumer-side fix (§11.4.28 -- helix_qa is a
# decoupled, project-not-aware submodule; its HTTPExecutor default of
# TokenField="session_token" is CORRECT for other consumers and MUST
# NOT be changed there):
#
#   HelixCode's own login handler
#   (helix_code/internal/server/handlers.go func (s *Server) login)
#   returns the bearer JWT under the TOP-LEVEL "token" field:
#
#       {"status":"success","user":<User>,"token":<JWT>,"session":<Session>}
#
#   "session_token" DOES appear in that body, but only nested inside
#   the distinct "session" object (helix_code/internal/auth/auth.go
#   Session.SessionToken -- a server-side session record, not the JWT
#   authMiddleware validates via VerifyJWTWithDB,
#   helix_code/internal/server/server.go ~L551). The generic
#   HTTPExecutor (submodules/helix_qa/pkg/autonomous/http_executor.go)
#   only ever inspects the TOP-LEVEL decoded JSON map for
#   decoded[h.TokenField], so with the built-in default it never finds
#   a token, no bearer header is ever attached, and every
#   AuthMode "admin" / "as:<user>" bank step against a real HelixCode
#   server 401s -- previously misdiagnosed (HXC-124) as a "JWT cannot
#   be minted" gap. Registration (HXC-035) and login both work; the
#   executor was simply configured to read the wrong response field.
#
#   The fix is CONSUMER-SIDE and CONFIG-ONLY: the standalone
#   `helixqa http` CLI (submodules/helix_qa/cmd/helixqa/http.go)
#   already exposes a `-token-field` flag
#   (`if *tokenField != "" { exec.TokenField = *tokenField }`,
#   http.go L123-124) precisely for this -- no submodule code changes
#   needed, only the CLI invocation's config value.
#
# Usage:
#   ./scripts/run-helixcode-http-banks.sh [--base-url URL] [--banks-dir DIR]
#
# Env overrides (never hardcoded secrets -- CONST-045/§11.4.10 style):
#   HELIXCODE_QA_BASE_URL   HelixCode server base URL (default http://localhost:8080)
#   HELIXCODE_QA_ADMIN_USER Existing registered HelixCode username to reuse
#   HELIXCODE_QA_ADMIN_PASS Password for HELIXCODE_QA_ADMIN_USER
#   HELIXCODE_QA_SKIP_REGISTER=1  Skip the self-register step (use an
#                                  already-registered HELIXCODE_QA_ADMIN_USER)
#
# When HELIXCODE_QA_ADMIN_USER/_PASS are not set, this script
# self-registers a fresh throwaway user via POST /api/v1/auth/register
# (registration works -- HXC-035/HXC-029 fix history) so every run is
# self-contained and re-runnable (§11.4.98) without a pre-seeded
# fixture user.
#
# Exit code: 0 only when the CLI reports zero FAILed/errored cases.
# A bank case whose only http: steps are honest `_skip_reason`
# entries (e.g. HXC-TASK-014's documented JWT-mint-in-offline-bank
# limitation) is reported SKIP, not FAIL/PASS, and does not affect
# the exit code -- Article XI §11.2.2 / §11.4.3.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELIX_CODE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${HELIX_CODE_DIR}/.." && pwd)"
HELIXQA_DIR="${REPO_ROOT}/submodules/helix_qa"
BANKS_DIR="${HELIXQA_DIR}/banks"

BASE_URL="${HELIXCODE_QA_BASE_URL:-http://localhost:8080}"
ADMIN_USER="${HELIXCODE_QA_ADMIN_USER:-}"
ADMIN_PASS="${HELIXCODE_QA_ADMIN_PASS:-}"
SKIP_REGISTER="${HELIXCODE_QA_SKIP_REGISTER:-0}"
QA_EVIDENCE_DIR="${QA_EVIDENCE_DIR:-${REPO_ROOT}/docs/qa}"
RUN_TAG="hxc124_http_banks_$(date -u +%Y%m%dT%H%M%S)"
EVIDENCE_DIR="${QA_EVIDENCE_DIR}/${RUN_TAG}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)  BASE_URL="$2"; shift 2 ;;
    --banks-dir) BANKS_DIR="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -d "${HELIXQA_DIR}" ]]; then
  echo "error: helix_qa submodule not found at ${HELIXQA_DIR}" >&2
  exit 1
fi

mkdir -p "${EVIDENCE_DIR}"

echo "============================================================"
echo " HXC-124 -- HelixCode HelixQA HTTP banks (fixed TokenField)"
echo "============================================================"
echo " Base URL:   ${BASE_URL}"
echo " Banks dir:  ${BANKS_DIR}"
echo " Evidence:   ${EVIDENCE_DIR}"
echo "============================================================"
echo ""

# -- Self-register a throwaway QA user when no credentials supplied ---------
if [[ "${SKIP_REGISTER}" != "1" && -z "${ADMIN_USER}" ]]; then
  ADMIN_USER="helixqa_hxc124_$(date -u +%Y%m%d%H%M%S)"
  ADMIN_PASS="HelixQA-$(date -u +%s)-Pw1!"
  echo "--- Self-registering throwaway QA user: ${ADMIN_USER} ---"
  register_body="$(printf '{"username":"%s","email":"%s@example.invalid","password":"%s"}' \
    "${ADMIN_USER}" "${ADMIN_USER}" "${ADMIN_PASS}")"
  register_status="$(curl -sS -o "${EVIDENCE_DIR}/register_response.json" -w '%{http_code}' \
    -X POST "${BASE_URL}/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "${register_body}" || echo "000")"
  echo "  POST /api/v1/auth/register -> HTTP ${register_status}"
  if [[ "${register_status}" != "200" && "${register_status}" != "201" ]]; then
    echo "  WARNING: registration did not return 200/201 (status=${register_status})." >&2
    echo "  Falling back to \$HELIXCODE_QA_ADMIN_USER/_PASS if set, else the" >&2
    echo "  executor's built-in admin/admin123 default." >&2
    echo "  See ${EVIDENCE_DIR}/register_response.json for the real response body." >&2
    ADMIN_USER="${HELIXCODE_QA_ADMIN_USER:-}"
    ADMIN_PASS="${HELIXCODE_QA_ADMIN_PASS:-}"
  fi
  echo ""
fi

# -- Select only the HelixCode-specific banks (never the other
#    consumers' banks, e.g. catalog_*/nexus-*/ocu-*, which correctly
#    rely on the upstream "session_token" default for THEIR server) --
bank_list=""
shopt -s nullglob
for f in "${BANKS_DIR}"/helixcode-*.yaml "${BANKS_DIR}"/helixcode-*.json; do
  if [[ -z "${bank_list}" ]]; then
    bank_list="${f}"
  else
    bank_list="${bank_list},${f}"
  fi
done
shopt -u nullglob

if [[ -z "${bank_list}" ]]; then
  echo "error: no helixcode-*.yaml / helixcode-*.json banks found under ${BANKS_DIR}" >&2
  exit 1
fi

echo "--- Banks selected ---"
IFS=',' read -ra bank_array <<< "${bank_list}"
for b in "${bank_array[@]}"; do
  echo "  $(basename "${b}")"
done
echo ""

# -- Run the CLI's http subcommand with the HelixCode-correct
#    -token-field (the fix; see the header comment) ------------------
cli_args=(
  http
  --banks "${bank_list}"
  --base-url "${BASE_URL}"
  --token-field token
  --verbose
  --json
)
if [[ -n "${ADMIN_USER}" ]]; then
  cli_args+=(--admin-user "${ADMIN_USER}")
fi
if [[ -n "${ADMIN_PASS}" ]]; then
  cli_args+=(--admin-pass "${ADMIN_PASS}")
fi

echo "--- Running: go run ./cmd/helixqa ${cli_args[*]} (base-url redacted-creds) ---"
(
  cd "${HELIXQA_DIR}"
  go run ./cmd/helixqa "${cli_args[@]}"
) | tee "${EVIDENCE_DIR}/run_output.txt"
status=${PIPESTATUS[0]}

echo ""
echo "Evidence written to: ${EVIDENCE_DIR}"
exit "${status}"
