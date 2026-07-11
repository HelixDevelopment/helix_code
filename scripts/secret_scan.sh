#!/usr/bin/env bash
# scripts/secret_scan.sh
#
# Permanent secret-scan guard closing the committed-key leak class per
# §11.4.135 (standing regression-guard suite) / §11.4.138 (operator-escape
# bluff-audit + permanent guard). Forensic anchor: commit 41372967 redacted
# a real Google (Gemini) API key that had been committed in plaintext in
# docs/qa/phase1_providers_20260708T141500Z/live_probe.md — see
# docs/qa/SECURITY_INCIDENT_gemini_key_leak_20260711.md. Root cause
# (§11.4.102): scripts/scan-secrets.sh already existed and already covers
# these key shapes, but was wired ONLY into the pre-push hook
# (scripts/git_hooks/pre-push) — no pre-commit content-level secret scan
# existed, so a committed evidence .md file with a real key never hit a
# gate before landing in a local commit. This script is wired into
# scripts/git_hooks/pre-commit (see section 4 there) to close that gap at
# the earliest possible point — before the secret is even committed, not
# merely before it is pushed.
#
# Scans a set of files, the whole tracked tree, or the staged diff for
# key-shaped patterns. Exits non-zero on any unallowlisted hit and prints
# ONLY "<file>:<line>" for each hit — the matched secret VALUE is never
# printed anywhere (§11.4.10 — never print any real key value).
#
# Usage:
#   scripts/secret_scan.sh                 # scan the whole tracked git tree
#   scripts/secret_scan.sh --staged        # scan staged (about-to-commit) content
#   scripts/secret_scan.sh <file> [<file>...]  # scan an explicit file set
#
# Exit codes:
#   0 = no unallowlisted key-shaped pattern found
#   1 = one or more hits (caller must investigate, rotate if real, and
#       either remove the content or add an explicit redaction marker)
#   2 = usage / environment error (e.g. no grep found)
#
# Allowlist (content-based, NOT path-based): a matched line is treated as a
# false positive and SKIPPED if it also contains (case-insensitive) any of:
#   "redacted", "example", "..." (three literal dots — the "<...>" placeholder
#   shape). This covers redaction markers like
#   "<REDACTED-GEMINI-KEY-CONST-042-...>" and illustrative placeholders like
#   "AIzaSyEXAMPLE..." without needing a path-based allowlist file.
#
# Cross-references: §11.4.10 / §11.4.30 / §11.4.102 / §11.4.135 / §11.4.138 /
#   CONST-042; scripts/scan-secrets.sh (pre-push, broader file-type scan,
#   path-based .scan-secrets-allow — left untouched, not modified by this
#   script); scripts/git_hooks/pre-commit (wired here, section 4).

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)

# ---------------------------------------------------------------------------
# grep binary resolution — avoid a hardcoded /usr/bin/grep (breaks on macOS
# Apple Silicon + Homebrew) and avoid an injected `ugrep` wrapper that some
# CLI-agent environments put on PATH ahead of GNU/BSD grep (its -I flag
# silently skips files this scanner needs to read).
# ---------------------------------------------------------------------------
GREP_BIN="$(command -v grep 2>/dev/null || true)"
if [ -z "$GREP_BIN" ] || [ "$GREP_BIN" = "$(command -v ugrep 2>/dev/null || true)" ]; then
  for candidate in /usr/bin/grep /usr/local/bin/grep /opt/homebrew/bin/grep /bin/grep; do
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

# ---------------------------------------------------------------------------
# Key-shaped patterns (extended regex). Each entry: "label|pattern".
# ---------------------------------------------------------------------------
PATTERNS=(
  "Google API key|AIza[0-9A-Za-z_-]{20,}"
  "OpenAI API key|sk-[A-Za-z0-9]{20,}"
  "AWS access key|AKIA[0-9A-Z]{16}"
  "GitHub personal access token|ghp_[A-Za-z0-9]{30,}"
  "Private key header|-----BEGIN (RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----"
  "Private key header (generic)|-----BEGIN PRIVATE KEY-----"
  "Slack bot token|xoxb-[0-9]{9,}-[0-9]{9,}-[A-Za-z0-9]{20,}"
  "HuggingFace token|hf_[A-Za-z0-9]{30,}"
  # ---------------------------------------------------------------------
  # Defense-in-depth hardening pass (§11.4.138 — closes additional key
  # classes real .env files in this project could hold; the project ships
  # ~30 provider .env aliases including xai, gcp, azure). Each pattern
  # below was verified against the whole tracked tree before being added:
  # a low-detection-threshold version would have false-positived on
  # existing legitimate doc placeholders (e.g. "sk-ant-your-key") and Go
  # unit-test fixtures (e.g. "sk-ant-realvalue-1234567890", 20 chars) —
  # thresholds were tuned to clear the longest observed in-repo fixture
  # with a safety margin while staying far below real key entropy length.
  # See scratchpad/r41_guard_hardening.md for the full false-positive
  # audit this session performed before landing these patterns.
  # ---------------------------------------------------------------------
  # xAI (Grok) API key: "xai-" + a pure-alphanumeric body (no hyphens in
  # the real key body). Longest in-repo placeholder observed was
  # "xai-secret-key-12345" (16 alnum-run chars, broken by hyphens); the
  # 20-char pure-alnum-run requirement clears every such placeholder.
  "xAI API key|xai-[A-Za-z0-9]{20,}"
  # Anthropic API key, explicit label (diagnostic aid). NOTE: the existing
  # generic "OpenAI API key|sk-[A-Za-z0-9]{20,}" pattern does NOT already
  # cover this shape — real/leaked Anthropic keys are "sk-ant-api03-..."
  # and the hyphen after "ant" breaks the OpenAI pattern's required
  # 20-char pure-alnum run at only 3 characters ("ant"), so this is a
  # genuine coverage gap being closed, not merely a redundant label.
  # Threshold is 30 (not 20) specifically because the longest sk-ant-
  # fixture already committed in this repo's Go unit tests is
  # "sk-ant-realvalue-1234567890" (exactly 20 chars after "sk-ant-");
  # 30 clears it with a 10-char margin while staying far below real
  # Anthropic key length (~100+ chars for the sk-ant-api03-... format).
  "Anthropic API key (explicit)|sk-ant-[A-Za-z0-9_-]{30,}"
  # GCP service-account JSON credential marker: the literal
  # "type": "service_account" key-value pair is unique to GCP
  # service-account key files and does not occur in ordinary prose
  # (verified: doc files in this repo that merely discuss "service
  # account" in prose do not match this exact quoted-JSON shape).
  "GCP service-account JSON marker (type)|\"type\"[[:space:]]*:[[:space:]]*\"service_account\""
  # GCP service-account JSON credential marker: the "private_key_id" JSON
  # key is likewise unique to GCP service-account key files. Matching on
  # the key name + following colon (not a specific value shape) keeps
  # this simple and low-FP since the exact literal never occurs outside
  # that JSON shape.
  "GCP service-account JSON marker (private_key_id)|\"private_key_id\"[[:space:]]*:"
  # Azure key/secret, env-assignment shape: an AZURE_*KEY or AZURE_*SECRET
  # env-var name, an assignment operator, and a 32+ char hex value (the
  # shape used by Azure Cognitive Services / Azure OpenAI resource keys).
  # Deliberately narrow: Azure also issues base64 Storage-account keys and
  # mixed-charset AD client secrets, whose shapes are not "cleanly
  # definable" without materially raising false-positive risk (see
  # scratchpad/r41_guard_hardening.md for the honest skip note) — those
  # are NOT covered by this pattern.
  "Azure key/secret (env-assignment, hex)|AZURE_[A-Z0-9_]*(KEY|SECRET)[[:space:]]*[:=][[:space:]]*[\"']?[A-Fa-f0-9]{32,}[\"']?"
)

# Content-based allowlist markers (case-insensitive substrings). A matched
# line containing any of these is a redaction marker / illustrative
# placeholder, not a real leaked secret.
ALLOW_MARKER_RE='redacted|example|\.\.\.'

# Files this scanner itself must never flag (it legitimately quotes the
# pattern literals and label text above, and its own test plants fixture
# secrets in a temp dir, not in this file).
is_self() {
  case "$1" in
    */scripts/secret_scan.sh|scripts/secret_scan.sh) return 0 ;;
    */scripts/secret_scan_test.sh|scripts/secret_scan_test.sh) return 0 ;;
  esac
  return 1
}

found=0
hits=""

# scan_file_content <label> <pattern> <display-path> <content>
# Greps $content for $pattern via a temp buffer (process substitution),
# applies the content-based allowlist per matched line, and records
# "<display-path>:<line>" for any unallowlisted hit. NEVER echoes the
# matched line/value.
scan_file_content() {
  local label="$1" pattern="$2" display_path="$3" content="$4"
  local lineno rest
  while IFS=: read -r lineno rest; do
    [ -z "$lineno" ] && continue
    case "$rest" in
      *[Rr][Ee][Dd][Aa][Cc][Tt][Ee][Dd]*|*[Ee][Xx][Aa][Mm][Pp][Ll][Ee]*|*'...'*)
        continue
        ;;
    esac
    hits="${hits}\n  ${display_path}:${lineno}  (${label})"
    found=1
  # -e explicitly disambiguates the pattern from an option: several of our
  # patterns (the PEM private-key headers) start with "-----", which some
  # grep-compatible implementations (e.g. a ugrep shim providing a bare
  # "grep" on PATH) otherwise misparse as an unrecognized flag rather than
  # a positional PATTERN argument.
  done < <(printf '%s\n' "$content" | "$GREP_BIN" -nE -e "$pattern" 2>/dev/null || true)
}

# scan_disk_file <path>  — grep a real file on disk, path also used as the
# display path. Used for whole-tree scans and explicit-file-arg scans.
scan_disk_file() {
  local f="$1"
  is_self "$f" && return 0
  [ -f "$f" ] || return 0
  # Skip binary files: a key-shaped ASCII secret cannot meaningfully live in
  # one, and reading NUL-containing content into a shell variable emits noisy
  # "ignored null byte" warnings on every commit. `grep -Iq .` reports a
  # binary file as not-matching (-I treats binary as no match), so a non-zero
  # exit here means "binary" → skip.
  "$GREP_BIN" -Iq . -- "$f" 2>/dev/null || return 0
  local content
  content=$(cat "$f" 2>/dev/null) || return 0
  local entry label pattern
  for entry in "${PATTERNS[@]}"; do
    label="${entry%%|*}"
    pattern="${entry#*|}"
    scan_file_content "$label" "$pattern" "$f" "$content"
  done
}

# scan_staged_file <path> — grep the STAGED BLOB (git index), not the
# working tree, so the guard checks exactly what is about to be committed
# (mirrors the §11.4.84 mutation-residue section of scripts/git_hooks/pre-commit).
scan_staged_file() {
  local f="$1"
  is_self "$f" && return 0
  local content
  content=$(git show ":$f" 2>/dev/null) || return 0
  local entry label pattern
  for entry in "${PATTERNS[@]}"; do
    label="${entry%%|*}"
    pattern="${entry#*|}"
    scan_file_content "$label" "$pattern" "$f" "$content"
  done
}

mode="tree"
files=()
if [ "${1:-}" = "--staged" ]; then
  mode="staged"
elif [ "$#" -gt 0 ]; then
  mode="files"
  files=("$@")
fi

case "$mode" in
  tree)
    cd "$REPO_ROOT" || exit 2
    while IFS= read -r f; do
      [ -n "$f" ] && scan_disk_file "$f"
    done < <(git ls-files 2>/dev/null || true)
    ;;
  staged)
    cd "$REPO_ROOT" || exit 2
    while IFS= read -r f; do
      [ -n "$f" ] && scan_staged_file "$f"
    done < <(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null || true)
    ;;
  files)
    for f in "${files[@]}"; do
      scan_disk_file "$f"
    done
    ;;
esac

if [ "$found" -eq 0 ]; then
  echo "OK: no unallowlisted key-shaped secret pattern found (mode=$mode)"
  exit 0
fi

{
  echo ""
  echo "FAIL: key-shaped secret pattern(s) found (value never printed):"
  printf '%b\n' "$hits"
  echo ""
  echo "If real: rotate immediately, git rm --cached the file, and follow"
  echo "the CONST-042 / §11.4.10 post-mortem procedure before re-committing."
  echo "If a false positive (documentation example): add a redaction marker"
  echo "such as REDACTED / EXAMPLE / ... to the line so the allowlist"
  echo "recognizes it as intentional, non-secret content."
} >&2
exit 1
