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

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$REPO_ROOT"

# Use real GNU grep, not the ugrep wrapper that Claude Code injects into the shell.
# The ugrep wrapper adds -I (skip binary files) which prevents scanning id_rsa etc.
GREP=/usr/bin/grep
if [ ! -x "$GREP" ]; then
  GREP=$(command -v grep)
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
  --exclude-dir=HelixAgent
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

found=0

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
    # Use -e to pass the pattern — avoids patterns starting with '-' being parsed as options
    if "$GREP" -rEn "${FILE_INCLUDES[@]}" "${EXCLUDES[@]}" "${EXCLUDE_FILES[@]}" \
         -e "$pattern" -- "$SCAN_TARGET" 2>/dev/null; then
      found=1
    fi
  done
else
  # Targeted directory scan: no file-type filter (test harness uses .txt files)
  for pattern in "${PATTERNS[@]}"; do
    if "$GREP" -rEn "${EXCLUDES[@]}" "${EXCLUDE_FILES[@]}" \
         -e "$pattern" -- "$SCAN_TARGET" 2>/dev/null; then
      found=1
    fi
  done
fi

if [ "$found" -eq 0 ]; then
  echo "OK: no credential patterns found in $SCAN_TARGET"
  exit 0
fi

echo "" >&2
echo "FAIL: credential pattern(s) found. Rotate and remove BEFORE committing." >&2
exit 1
