#!/usr/bin/env bash
# verify_qa_evidence.sh — ADVISORY (warn-mode) §11.4.83 docs/qa/ evidence gate.
#
# Purpose:
#   Scans recent feature-shipping commits and WARNs when a feature commit
#   has no matching docs/qa/<run-id>/ directory carrying its end-user
#   evidence transcript (per constitution submodule Constitution.md
#   §11.4.83 — docs/qa/ end-user evidence mandate).
#
#   ADVISORY ONLY. This script ALWAYS exits 0. It never blocks a commit,
#   push, or build. The operator has NOT authorised a hard gate; promoting
#   this scanner to a blocking commit/release gate (per §11.4.83 operative
#   rule (5)) is a FUTURE OPERATOR DECISION. Until then the script raises
#   visibility without halting work, and it is NOT wired into any
#   pre-commit / pre-push hook.
#
# Heuristic for "feature-shipping commit":
#   A commit that touches non-test production code under any of:
#     helix_code/internal/**   helix_code/cmd/**   helix_code/applications/**
#   excluding *_test.go files. This is a deliberately-loose advisory
#   heuristic — false positives (e.g. a pure refactor) are acceptable for
#   a warn-mode notice.
#
# Usage:
#   scripts/verify_qa_evidence.sh [N]
#     N — number of most-recent commits to scan (default 20).
#
# Inputs:
#   git history of the current repository; the docs/qa/ tree on disk.
#
# Outputs:
#   A human-readable advisory report on stdout. Exit code ALWAYS 0.
#
# Side-effects:
#   None. Read-only over git history + the working tree.
#
# Dependencies:
#   git, POSIX coreutils, bash (honest shebang — uses bash arrays).
#
# Cross-references:
#   docs/qa/README.md (the convention), docs/scripts/verify_qa_evidence.md
#   (companion guide), constitution Constitution.md §11.4.83.

set -u

SCAN_N="${1:-20}"

# Resolve repo root so the script works from any cwd.
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
	echo "verify_qa_evidence.sh: not inside a git repository — nothing to scan (advisory)."
	exit 0
fi
cd "$REPO_ROOT" || exit 0

QA_DIR="docs/qa"

echo "==========================================================================="
echo " §11.4.83 docs/qa/ end-user evidence — ADVISORY scan (warn-mode, exit 0)"
echo "==========================================================================="
echo " Scanning the last ${SCAN_N} commit(s) for feature-shipping commits"
echo " lacking a matching docs/qa/<run-id>/ directory."
echo " NOTE: advisory only — never blocks. Hard-gate promotion is a future"
echo "       operator decision (§11.4.83 operative rule (5))."
echo "---------------------------------------------------------------------------"

# Build the set of existing run-id directories under docs/qa/.
existing_runids=""
if [ -d "$QA_DIR" ]; then
	for d in "$QA_DIR"/*/; do
		[ -d "$d" ] || continue
		rid="$(basename "$d")"
		existing_runids="${existing_runids} ${rid}"
	done
fi

warn_count=0
feature_commit_count=0

# Iterate recent commits (oldest-to-newest within the window for readability).
commits="$(git rev-list --max-count="$SCAN_N" HEAD 2>/dev/null || true)"
for sha in $commits; do
	# Feature-shipping heuristic: non-test prod code under the watched roots.
	touched="$(git show --no-patch --format= --name-only "$sha" 2>/dev/null \
		| grep -E '^helix_code/(internal|cmd|applications)/' \
		| grep -v '_test\.go$' || true)"
	[ -n "$touched" ] || continue

	feature_commit_count=$((feature_commit_count + 1))

	subject="$(git show --no-patch --format='%s' "$sha" 2>/dev/null || true)"
	short="$(git rev-parse --short "$sha" 2>/dev/null || echo "$sha")"

	# Does the commit subject reference a known run-id directory?
	matched=""
	for rid in $existing_runids; do
		case "$subject" in
			*"$rid"*) matched="$rid"; break ;;
		esac
	done

	if [ -n "$matched" ]; then
		echo "  ok   ${short}  ${subject}"
		echo "         -> docs/qa/${matched}/ present"
	else
		warn_count=$((warn_count + 1))
		echo "  WARN ${short}  ${subject}"
		echo "         -> no matching docs/qa/<run-id>/ directory found"
	fi
done

echo "---------------------------------------------------------------------------"
echo " Feature-shipping commits scanned : ${feature_commit_count}"
echo " Advisory warnings                : ${warn_count}"
if [ "$warn_count" -gt 0 ]; then
	echo
	echo " ADVISORY: the commits above appear to ship a feature but have no"
	echo " matching docs/qa/<run-id>/ evidence directory. Per §11.4.83 each"
	echo " shipped feature SHOULD carry a recorded end-to-end transcript +"
	echo " materials. Add one under docs/qa/<run-id>/ (see docs/qa/README.md)."
	echo " This is a notice only — no action is enforced."
fi
echo "==========================================================================="
echo " RESULT: advisory scan complete (exit 0 — warn-mode)."

# §11.4.83 operative rule (5) promotion to a blocking gate is a future
# operator decision. Until then this script ALWAYS succeeds.
exit 0
