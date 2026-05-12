#!/usr/bin/env bash
# Fail if any test skip directive is present without a SKIP-OK: #<ticket> annotation.
#
# Part of the Definition of Done enforcement arm. A skipped test is invisible debt —
# this script makes that debt loud. If a skip is genuinely needed, annotate it with
# a ticket reference:
#
#   t.Skip("flake under race — SKIP-OK: #1234")
#   @Ignore("waiting on upstream — SKIP-OK: #1234")
#   it.skip('pending feature — SKIP-OK: #1234', ...)
#
# Portable across Go / Kotlin / Java / TS / JS / Python / Swift / Rust.
# Wire into: make no-silent-skips  →  make ci-validate-all
#
# Env:
#   NO_SILENT_SKIPS_WARN_ONLY=1   — log violations but exit 0 (transition mode)
#   NO_SILENT_SKIPS_EXCLUDES=...  — colon-separated extra directory names to skip

set -uo pipefail
cd "$(dirname "$0")/.."  # repo root (script installed under scripts/)

# JS test-skip variants are matched as `(it|describe|test|context|xit|xdescribe)\.skip\(`
# rather than the overly broad `\.skip\(`, which previously matched generic
# method calls like `this.skip(10)` on a video-player object in
# Github-Pages-Website/docs/courses/player.js and produced ~1500 spurious hits.
PATTERNS='t\.Skip\(|@Ignore\b|\bxit\(|(it|describe|test|context|xit|xdescribe)\.skip\(|@pytest\.mark\.skip|@unittest\.skip|#\[ignore\]|XCTSkipIf'
INCLUDES=(--include='*.go' --include='*.kt' --include='*.kts' --include='*.java'
          --include='*.ts' --include='*.tsx' --include='*.js' --include='*.jsx'
          --include='*.py' --include='*.swift' --include='*.rs')

# Default excludes — third-party/vendored/generated trees.
EXCLUDES=(--exclude-dir=.git --exclude-dir=vendor --exclude-dir=node_modules
          --exclude-dir=external --exclude-dir=target --exclude-dir=build
          --exclude-dir=.gradle --exclude-dir=.idea --exclude-dir=dist
          --exclude-dir=releases --exclude-dir=reports --exclude-dir=test-results
          --exclude-dir=.next --exclude-dir=.nuxt --exclude-dir=coverage
          --exclude-dir=.venv --exclude-dir=__pycache__
          # Anti-bluff test fixtures are intentional bare skips: they exist
          # so the scanner can prove it CATCHES the violation. Annotating
          # them defeats the test.
          --exclude-dir=fixtures
          # OS-vendored Go cache trees occasionally appear in vendored
          # toolchains; treat like other vendored dirs.
          --exclude-dir=testdata)

# Auto-exclude every third-party submodule listed in
# docs/improvements/submodule_third_party.txt. These trees are owned by
# external upstreams and we cannot annotate their skips with our
# SKIP-OK ticket format without polluting upstream. Per the
# verify-governance-cascade decision (commit 099f06a), listing in
# submodule_third_party.txt IS the governance acknowledgement; this
# script honours the same boundary.
THIRD_PARTY_FILE="docs/improvements/submodule_third_party.txt"
if [ -f "$THIRD_PARTY_FILE" ]; then
  while IFS=' |' read -r sm rest; do
    [ -z "$sm" ] && continue
    # --exclude-dir matches directory basename anywhere in the walk,
    # so the leaf name of each submodule path is enough.
    base=$(basename "$sm")
    EXCLUDES+=("--exclude-dir=$base")
  done < "$THIRD_PARTY_FILE"
fi

# Caller-provided extras (colon-separated directory names).
if [ -n "${NO_SILENT_SKIPS_EXCLUDES:-}" ]; then
  IFS=':' read -r -a extras <<< "$NO_SILENT_SKIPS_EXCLUDES"
  for d in "${extras[@]}"; do
    [ -n "$d" ] && EXCLUDES+=("--exclude-dir=$d")
  done
fi

# Accept SKIP-OK markers in any of these traceable forms:
#   - SKIP-OK: #1234                       numeric GitHub ticket
#   - SKIP-OK: #short-mode                 slug (kebab/snake)
#   - SKIP-OK: P1-F14-T10                  project task ID (no #)
#   - SKIP-OK: P1-F16-T10 — OTEL ...       project task ID + rationale
# Any of the above counts as an explicit, documented skip. The earlier
# regex accepted only `#<digits>` and silently misflagged ~5500 valid
# skips across the codebase, defeating the gate's purpose entirely.
#
# Path-based post-filter: grep's --exclude-dir matches BASENAME only,
# but vendored trees nested under our own dirs (e.g.
# HelixAgent/MCP/submodules/python-sdk/, HelixQA/tools/opensource/...)
# slip through because the basename of the leaf is unique-looking but
# the PARENT path tags the file as not-ours-to-annotate. Strip those
# paths after the grep.
VENDORED_PATH_REGEX='HelixAgent/MCP/submodules/|HelixQA/tools/opensource/|/python-sdk/|/llama-index/|/llama_index/|/chroma[/_]|/unstructured/|/browser-use/|/atlassian-mcp/|/opensource/'

violations=$(grep -rnE "$PATTERNS" "${INCLUDES[@]}" "${EXCLUDES[@]}" . 2>/dev/null \
             | grep -v -E 'SKIP-OK: #?[A-Za-z0-9][A-Za-z0-9_-]*' \
             | grep -v -E "$VENDORED_PATH_REGEX" || true)

if [ -n "$violations" ]; then
  count=$(printf '%s\n' "$violations" | wc -l | tr -d ' ')
  echo "⚠️  $count silent-skip violation(s) detected." >&2
  echo "" >&2
  printf '%s\n' "$violations" | head -30 >&2
  if [ "$count" -gt 30 ]; then
    echo "... ($((count - 30)) more — re-run '$0' without head)" >&2
  fi
  echo "" >&2
  echo "Annotate each with a trailing '// SKIP-OK: #<ticket>' (or '# SKIP-OK: #<ticket>')" >&2
  echo "comment so the skip is tracked, or remove the skip if it is no longer needed." >&2
  if [ "${NO_SILENT_SKIPS_WARN_ONLY:-0}" = "1" ]; then
    echo "" >&2
    echo "(warn-only mode — set NO_SILENT_SKIPS_WARN_ONLY=0 to fail the build)" >&2
    exit 0
  fi
  exit 1
fi

echo "no-silent-skips: OK (no unannotated skip directives found)"
