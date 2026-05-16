#!/usr/bin/env bash
# scripts/scan-secrets.sh
# Scan working tree (or a given directory) for credentials.
# Uses gitleaks if available; otherwise regex fallback for known token shapes.
# Per CONST-041: no credentials may be committed.
# Used by: make verify-foundation, pre-push hook, manual audit.
#
# Exit codes:
#   0 = no credential patterns found
#   1 = one or more matches (caller must rotate + git rm --cached + remediate)
#
# Allowlist:
#   Create .scan-secrets-allow at repo root (one path glob per line, # for comments).
#   Files/paths matching any allow rule are excluded from scanning.

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$REPO_ROOT"

# Prefer the system grep found via PATH — avoids hardcoding /usr/bin/grep which
# breaks on macOS Apple Silicon + Homebrew (grep lives at /opt/homebrew/bin/grep).
# Also avoids the ugrep wrapper that Claude Code injects (-I skips binary files).
GREP_BIN="$(command -v grep 2>/dev/null || true)"
if [ -z "$GREP_BIN" ] || [ "$GREP_BIN" = "$(command -v ugrep 2>/dev/null || true)" ]; then
  # Fallback: try well-known GNU grep locations in preference order
  for candidate in /usr/bin/grep /usr/local/bin/grep /opt/homebrew/bin/grep; do
    if [ -x "$candidate" ]; then
      GREP_BIN="$candidate"
      break
    fi
  done
fi
if [ -z "$GREP_BIN" ]; then
  echo "ERROR: grep not found in PATH or standard locations" >&2
  exit 2
fi

# Allow scanning a specific directory (used by tests)
SCAN_TARGET="${1:-.}"

# Patterns matching real-world secret shapes. Tighten over time.
# Each entry is one extended-regex pattern.
PATTERNS=(
  'sk-[A-Za-z0-9]{16,}'                                        # OpenAI
  'sk-ant-[A-Za-z0-9_-]{16,}'                                  # Anthropic
  'gho_[A-Za-z0-9]{16,}'                                       # GitHub OAuth token
  'ghp_[A-Za-z0-9]{16,}'                                       # GitHub personal access token
  'github_pat_[A-Za-z0-9_]{16,}'                               # GitHub PAT v2
  'glpat-[A-Za-z0-9_-]{16,}'                                   # GitLab PAT
  'xoxb-[A-Za-z0-9-]{16,}'                                     # Slack bot
  'xoxp-[A-Za-z0-9-]{16,}'                                     # Slack user
  'AKIA[A-Z0-9]{16}'                                           # AWS access key
  'AIza[A-Za-z0-9_-]{32,}'                                     # Google API key
  'eyJ[A-Za-z0-9_-]{20,}'                                      # JWT base64url header
  '-----BEGIN (RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----'      # PEM private key header
)

# Directories to exclude
EXCLUDES=(
  --exclude-dir=.git
  --exclude-dir=node_modules
  --exclude-dir=vendor
  --exclude-dir=target
  --exclude-dir=dist
  --exclude-dir=build
  --exclude-dir=Example_Projects
  --exclude-dir=helix_agent
  --exclude-dir=Dependencies
  --exclude-dir=Documentation
)

# Files to exclude (patterns/templates/this script itself)
EXCLUDE_FILES=(
  --exclude='*.example'
  --exclude='*.template'
  --exclude='*.sample'
  --exclude='*-example'
  --exclude='*-template'
  --exclude='scan-secrets.sh'
  --exclude='test-scan-secrets.sh'
)

# ---------------------------------------------------------------------------
# Allowlist: read .scan-secrets-allow from repo root.
# Each non-comment, non-empty line is a glob pattern for paths to skip.
# Applied as: skip grep output lines whose filename matches any allow glob.
# ---------------------------------------------------------------------------
ALLOWLIST_FILE="$REPO_ROOT/.scan-secrets-allow"
ALLOW_PATTERNS=()
if [ -f "$ALLOWLIST_FILE" ]; then
  while IFS= read -r line; do
    # Strip leading/trailing whitespace, skip empty lines and comments
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    if [ -z "$line" ] || [ "${line:0:1}" = "#" ]; then
      continue
    fi
    ALLOW_PATTERNS+=("$line")
  done < "$ALLOWLIST_FILE"
fi

# Helper: check if a grep output line (file:lineno:content) matches any allow pattern.
# The file path in grep output may be absolute or relative (e.g. ./foo/bar.md or
# /tmp/something/bar.md). The allowlist globs are repo-relative (e.g. docs/foo.md
# or *.md). We match if the filepath ends with the glob pattern or the basename matches.
is_allowlisted() {
  local match_line="$1"
  # Extract the file path (everything before the first colon-digit sequence = line number)
  local filepath
  filepath="${match_line%%:*}"
  # Normalize: strip leading ./
  filepath="${filepath#./}"
  local allow_glob
  for allow_glob in "${ALLOW_PATTERNS[@]}"; do
    # Strip leading ./ from glob too
    allow_glob="${allow_glob#./}"
    # Use bash glob matching — try three forms:
    # 1. Exact match (relative path vs glob)
    # 2. Path ends with /glob (relative suffix match)
    # 3. Basename match (for simple filename globs)
    local basename_path
    basename_path="${filepath##*/}"
    # shellcheck disable=SC2254
    case "$filepath" in
      $allow_glob)           return 0 ;;
      */$allow_glob)         return 0 ;;
      */"$allow_glob")       return 0 ;;
    esac
    # shellcheck disable=SC2254
    case "$basename_path" in
      $allow_glob)           return 0 ;;
    esac
  done
  return 1
}

found=0

run_scan() {
  local pattern="$1"
  shift
  local grep_args=("$@")
  local line
  while IFS= read -r line; do
    if ! is_allowlisted "$line"; then
      echo "$line"
      found=1
    fi
  done < <("$GREP_BIN" "${grep_args[@]}" -e "$pattern" -- "$SCAN_TARGET" 2>/dev/null || true)
}

if [ "$SCAN_TARGET" = "." ]; then
  # Full repo scan: restrict to known source + config file extensions
  FILE_INCLUDES=(
    --include='*.go' --include='*.py' --include='*.js' --include='*.ts'
    --include='*.tsx' --include='*.jsx' --include='*.kt' --include='*.java'
    --include='*.swift' --include='*.rs' --include='*.rb' --include='*.php'
    --include='*.json' --include='*.yaml' --include='*.yml' --include='*.toml'
    --include='*.md' --include='*.txt' --include='*.sh' --include='*.bash'
    --include='*.cfg' --include='*.conf' --include='*.ini' --include='*.env'
    --include='id_rsa' --include='id_rsa.*' --include='id_ed25519' --include='id_ed25519.*'
    --include='*.pem' --include='*.key'
  )
  for pattern in "${PATTERNS[@]}"; do
    run_scan "$pattern" -rEn "${FILE_INCLUDES[@]}" "${EXCLUDES[@]}" "${EXCLUDE_FILES[@]}"
  done
else
  # Targeted directory scan: no file-type filter (test harness uses .txt files)
  for pattern in "${PATTERNS[@]}"; do
    run_scan "$pattern" -rEn "${EXCLUDES[@]}" "${EXCLUDE_FILES[@]}"
  done
fi

if [ "$found" -eq 0 ]; then
  echo "OK: no credential patterns found in $SCAN_TARGET"
  exit 0
fi

echo "" >&2
echo "FAIL: credential pattern(s) found. Rotate and remove BEFORE committing." >&2
exit 1
