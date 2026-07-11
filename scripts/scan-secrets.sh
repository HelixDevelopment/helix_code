#!/usr/bin/env bash
# scripts/scan-secrets.sh
# Scan working tree (or a given directory) for credentials.
# Uses gitleaks if available; otherwise regex fallback for known token shapes.
# Per CONST-041: no credentials may be committed.
# Used by: make verify-foundation, pre-push hook, manual audit.
#
# Usage:
#   scripts/scan-secrets.sh                    # full tracked-tree scan (default)
#   scripts/scan-secrets.sh <dir>               # targeted directory scan (test harness)
#   scripts/scan-secrets.sh --range <base> <new> # scan ONLY lines ADDED between
#       two git revisions (a `git diff <base> <new>`), never the working tree.
#       Used by scripts/git_hooks/pre-push to scope the secret gate to the
#       COMMITS ACTUALLY BEING PUSHED (§11.4.6 fix, 2026-07-11): the prior
#       no-args tree scan also swept untracked working-tree files (scratchpad
#       reports, local notes) and blocked unrelated pushes on their content —
#       see docs/audit/bypass_events.md 2026-07-11 entry "Follow-up: ...
#       scope pre-push to pushed-commits not full working tree." A diff-scoped
#       scan structurally cannot see untracked files (git diff only ever
#       compares two committed tree states) while still catching a real
#       secret newly introduced by the pushed commits.
#
# Exit codes:
#   0 = no credential patterns found
#   1 = one or more matches (caller must rotate + git rm --cached + remediate)
#
# Allowlist:
#   Create .scan-secrets-allow at repo root (one path glob per line, # for comments).
#   Files/paths matching any allow rule are excluded from scanning (tree,
#   directory, AND --range modes — the allowlist is mode-independent).

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

# Allow scanning a specific directory (used by tests), or a --range <base>
# <new> git-revision pair (used by scripts/git_hooks/pre-push — see the
# header comment above).
MODE="dir"
SCAN_TARGET="${1:-.}"
RANGE_BASE=""
RANGE_NEW=""
if [ "${1:-}" = "--range" ]; then
  MODE="range"
  RANGE_BASE="${2:-}"
  RANGE_NEW="${3:-}"
  if [ -z "$RANGE_BASE" ] || [ -z "$RANGE_NEW" ]; then
    echo "ERROR: --range requires <base-rev> <new-rev>" >&2
    exit 2
  fi
fi

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
  --exclude-dir=dependencies
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
  --exclude='test-scan-secrets-range-perf.sh'
  --exclude='secret_scan.sh'
  --exclude='secret_scan_test.sh'
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

# Helper: check if a repo-relative-ish file PATH matches any allow pattern.
# The path may be absolute or relative (e.g. ./foo/bar.md or /tmp/something/
# bar.md). The allowlist globs are repo-relative (e.g. docs/foo.md or *.md).
# We match if the filepath ends with the glob pattern or the basename matches.
# Shared by is_allowlisted() (grep-output-line callers) and run_scan_range()
# (--range diff-scan callers, which already have a bare path, no line-number
# prefix to strip).
is_path_allowlisted() {
  local filepath="$1"
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

# Helper: check if a grep output line (file:lineno:content) matches any allow
# pattern. Extracts the file path (everything before the first colon-digit
# sequence = line number) and delegates to is_path_allowlisted().
is_allowlisted() {
  local match_line="$1"
  local filepath
  filepath="${match_line%%:*}"
  is_path_allowlisted "$filepath"
}

# ---------------------------------------------------------------------------
# --range mode helpers: mirror the EXCLUDES / EXCLUDE_FILES / FILE_INCLUDES
# filtering that the tree-mode `grep -r --exclude-dir=... --include=...`
# invocation applies, but expressed as path-string predicates — --range mode
# parses `git diff` output rather than walking the filesystem, so grep's
# --exclude-dir/--include flags don't apply; without these, --range mode
# would scan strictly MORE than tree mode ever does (e.g. a vendored
# dependencies/ path edited in a push), a behaviour drift from the
# already-audited tree-mode selectivity. Keep these three predicates in sync
# with EXCLUDES / EXCLUDE_FILES / FILE_INCLUDES above if those ever change.
# ---------------------------------------------------------------------------
is_excluded_dir_path() {
  local f="$1" seg
  local IFS='/'
  # shellcheck disable=SC2206
  local segs=($f)
  for seg in "${segs[@]}"; do
    case "$seg" in
      .git|node_modules|vendor|target|dist|build|Example_Projects|helix_agent|dependencies|Documentation)
        return 0 ;;
    esac
  done
  return 1
}

is_excluded_file_path() {
  local base="${1##*/}"
  case "$base" in
    *.example|*.template|*.sample|*-example|*-template|scan-secrets.sh|test-scan-secrets.sh|test-scan-secrets-range-perf.sh|secret_scan.sh|secret_scan_test.sh)
      return 0 ;;
  esac
  return 1
}

is_included_extension() {
  local base="${1##*/}"
  case "$base" in
    *.go|*.py|*.js|*.ts|*.tsx|*.jsx|*.kt|*.java|*.swift|*.rs|*.rb|*.php) return 0 ;;
    *.json|*.yaml|*.yml|*.toml|*.md|*.txt|*.sh|*.bash) return 0 ;;
    *.cfg|*.conf|*.ini|*.env|*.pem|*.key) return 0 ;;
    id_rsa|id_rsa.*|id_ed25519|id_ed25519.*) return 0 ;;
  esac
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

# run_scan_range <base-rev> <new-rev>
#
# Scans ONLY the lines ADDED between two git revisions (never the working
# tree, never untracked files — `git diff` structurally cannot see either).
# Parses unified diff output: tracks the current file from each "+++ b/<path>"
# hunk header, then pattern-matches every "+"-prefixed content line against
# PATTERNS, applying the SAME selectivity as tree mode (extension include
# list, dir/file excludes, .scan-secrets-allow) so --range mode never flags
# something tree mode would have silently skipped.
#
# NEVER prints the matched secret value itself — only "<file>: <label>",
# mirroring scripts/secret_scan.sh's §11.4.10 value-never-printed contract.
#
# PERFORMANCE (§11.4.82 fix, 2026-07-11): the original implementation looped
# over every surviving added line and, for EACH line, looped over EVERY
# pattern in PATTERNS, spawning `printf | grep -qE` (two subprocesses) per
# (line × pattern) pair — O(lines × patterns) subprocess spawns. On an
# 18-commit / 1.8 MiB push this produced hundreds of thousands of spawns and
# took ~15 minutes (the actual git transfer was instant), blocking every
# root push. Fixed to O(total bytes): the per-line filtering (dir/file
# excludes, extension include-list, allowlist — all pure-bash, no subprocess
# cost) still runs once per line as before, but SURVIVING line content is
# collected into parallel arrays instead of grepped inline. All PATTERNS are
# combined ONCE into a single alternation regex, and the entire surviving
# corpus is grepped in a SINGLE `printf | grep -nE` invocation — one process
# spawn total instead of one per (line × pattern). The alternation
# `(pat1)|(pat2)|...` is logically equivalent to "does any individual
# pattern match this line" (each PATTERNS entry is a well-formed,
# self-contained ERE, so wrapping each in its own group and joining with `|`
# changes nothing about what matches — only how many processes it costs to
# find out), so detection coverage is byte-for-byte identical to before.
run_scan_range() {
  local base="$1" new="$2"
  local current_file="" line content pattern diff_out
  diff_out=$(git diff --unified=0 --no-color "$base" "$new" 2>/dev/null) || return 0

  # Build the combined alternation ONCE (not per line). Each PATTERNS entry
  # is wrapped in its own group so alternation precedence can never bleed
  # across pattern boundaries.
  local combined_pattern=""
  for pattern in "${PATTERNS[@]}"; do
    if [ -z "$combined_pattern" ]; then
      combined_pattern="(${pattern})"
    else
      combined_pattern="${combined_pattern}|(${pattern})"
    fi
  done

  # Single pass over the diff: apply the SAME per-line filtering as before
  # (dir exclude / file exclude / extension include / allowlist — all
  # subprocess-free bash predicates), but instead of grepping inline,
  # collect surviving (file, content) pairs into parallel arrays.
  local -a kept_files=()
  local -a kept_contents=()
  while IFS= read -r line; do
    case "$line" in
      '+++ '*)
        current_file="${line#+++ }"
        current_file="${current_file#b/}"
        continue
        ;;
      '+++')
        current_file=""
        continue
        ;;
    esac
    case "$line" in
      '+'*)
        [ -z "$current_file" ] && continue
        is_excluded_dir_path "$current_file" && continue
        is_excluded_file_path "$current_file" && continue
        is_included_extension "$current_file" || continue
        is_path_allowlisted "$current_file" && continue
        content="${line#+}"
        kept_files+=("$current_file")
        kept_contents+=("$content")
        ;;
    esac
  done <<< "$diff_out"

  [ "${#kept_contents[@]}" -eq 0 ] && return 0

  # ONE grep invocation over the ENTIRE surviving corpus, matching ANY
  # pattern. `printf '%s\n' "${kept_contents[@]}"` emits exactly one line
  # per array element in array order (each element is passed as a %s
  # argument, never interpreted as a format string, so arbitrary content —
  # backslashes, %, etc. — is safe); grep -n's 1-based line number therefore
  # maps 1:1 onto (index - 1) in kept_files/kept_contents.
  local match_line idx
  while IFS= read -r match_line; do
    idx="${match_line%%:*}"
    echo "${kept_files[$((idx - 1))]}: possible credential pattern in pushed range (value not printed)"
    found=1
  done < <(printf '%s\n' "${kept_contents[@]}" | "$GREP_BIN" -nE -e "$combined_pattern" 2>/dev/null || true)
}

if [ "$MODE" = "range" ]; then
  run_scan_range "$RANGE_BASE" "$RANGE_NEW"
elif [ "$SCAN_TARGET" = "." ]; then
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
  if [ "$MODE" = "range" ]; then
    echo "OK: no credential patterns found in pushed range ${RANGE_BASE}..${RANGE_NEW}"
  else
    echo "OK: no credential patterns found in $SCAN_TARGET"
  fi
  exit 0
fi

echo "" >&2
echo "FAIL: credential pattern(s) found. Rotate and remove BEFORE committing." >&2
exit 1
