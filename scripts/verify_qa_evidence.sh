#!/usr/bin/env bash
# verify_qa_evidence.sh — §11.4.83 docs/qa/ end-user evidence gate.
#
# Purpose:
#   Scans feature-shipping commits and reports when a feature commit has
#   no matching docs/qa/<run-id>/ directory carrying its end-user evidence
#   transcript (per constitution submodule Constitution.md §11.4.83 —
#   docs/qa/ end-user evidence mandate).
#
# Two modes:
#   ADVISORY  (default)  — ALWAYS exits 0. Prints a warn-mode notice for
#                          ad-hoc visibility. NOT wired into any git hook.
#   ENFORCING (--enforce) — exits 1 if any in-scope feature-shipping commit
#                          lacks its docs/qa/<run-id>/ directory. This is the
#                          §11.4.83 operative-rule-(5) release gate. The
#                          operator AUTHORISED promotion to a blocking
#                          release gate on 2026-05-28 (HXC-019). Wired into
#                          scripts/release-gate-test.sh via
#                          scripts/gates/qa_evidence_gate.sh — release-gate
#                          ONLY, never pre-commit / pre-push.
#
# Baseline scoping (--since):
#   §11.4.83 / the docs/qa convention was introduced by the commit that
#   ADDED docs/qa/README.md:
#       ed84f90e  2026-05-28T16:09:55+03:00
#       feat(qa): establish docs/qa/ end-user evidence tree + advisory gate
#                 (§11.4.83) (HXC-019)
#   Find it yourself with:
#       git log --diff-filter=A --format='%H %cI %s' -- docs/qa/README.md
#   Commits BEFORE the baseline predate the convention and are EXEMPT — the
#   tree did not exist, so they cannot be expected to carry a docs/qa entry.
#   --enforce mode therefore REQUIRES a --since baseline; without it the
#   whole repository history (thousands of legacy feature commits) would be
#   blocking and HEAD would become un-releasable. --since accepts any git
#   revision OR an approxidate (e.g. a SHA, a tag, or "2026-05-28").
#   Implementation: only commits in the range "<since>..HEAD" are evaluated
#   (i.e. descendants of the baseline), so the baseline commit itself is
#   exempt and merge-ancestry — not author-date sorting — defines scope.
#
# Per-commit opt-out:
#   A commit whose message (subject OR body) contains the literal token
#       [no-qa-evidence]
#   is EXEMPT (e.g. a pure refactor, governance-only change, or doc-only
#   change that trips the heuristic). Documented in docs/qa/README.md.
#
# Heuristic for "feature-shipping commit":
#   A commit that touches non-test production code under any of:
#     helix_code/internal/**   helix_code/cmd/**   helix_code/applications/**
#   excluding *_test.go files. Deliberately loose — false positives are
#   handled by the [no-qa-evidence] opt-out in enforcing mode.
#
# Match rule (does a commit carry its evidence?):
#   A feature commit is satisfied when its subject references the basename
#   of an existing docs/qa/<run-id>/ directory (e.g. subject mentions
#   "HXC-019" and docs/qa/HXC-019/ exists).
#
# Usage:
#   scripts/verify_qa_evidence.sh [N]
#       Advisory scan of the last N commits (default 20). Always exit 0.
#   scripts/verify_qa_evidence.sh --enforce --since <ref-or-date> [N]
#       Enforcing scan over <ref-or-date>..HEAD. Exit 1 on any violation,
#       exit 0 when clean. N caps the number of in-range commits scanned
#       (default: all commits in range).
#   scripts/verify_qa_evidence.sh --enforce            (no --since)
#       ERROR — refuses to run (exit 2). --since is mandatory in enforcing
#       mode to avoid blocking on pre-convention legacy history.
#   scripts/verify_qa_evidence.sh --help
#
# Inputs:
#   git history of the current repository; the docs/qa/ tree on disk.
#
# Outputs:
#   A human-readable report on stdout; diagnostics on stderr in --enforce.
#   Exit: advisory → always 0; enforcing → 0 clean / 1 violation / 2 misuse.
#
# Side-effects:
#   None. Read-only over git history + the working tree.
#
# Dependencies:
#   git, POSIX coreutils, bash (honest shebang — uses bash arrays).
#
# Cross-references:
#   docs/qa/README.md (the convention), docs/scripts/verify_qa_evidence.md
#   (companion guide), scripts/gates/qa_evidence_gate.sh (release-gate
#   wiring), scripts/release-gate-test.sh, constitution Constitution.md
#   §11.4.83.

set -u

# --------- Argument parsing ---------
MODE_ENFORCE=0
SINCE_REF=""
SCAN_N=""
WANT_HELP=0

usage() {
	cat <<'EOF'
Usage:
  scripts/verify_qa_evidence.sh [N]
      Advisory scan of the last N commits (default 20). ALWAYS exits 0.

  scripts/verify_qa_evidence.sh --enforce --since <ref-or-date> [N]
      Enforcing release gate over <ref-or-date>..HEAD.
      Exit 0 when every in-range feature-shipping commit carries its
      docs/qa/<run-id>/ directory; exit 1 on any violation.
      --since is MANDATORY in enforcing mode (baseline scoping — avoids
      blocking on pre-convention legacy history). N optionally caps the
      number of in-range commits scanned.

  scripts/verify_qa_evidence.sh --help

Per-commit opt-out: a commit whose message contains the token
[no-qa-evidence] is exempt (pure refactor / governance / docs).

Exit codes:
  0  clean (advisory always; enforcing when no violations)
  1  enforcing: at least one in-range feature commit lacks docs/qa evidence
  2  misuse (e.g. --enforce without --since, unknown flag, bad --since ref)
EOF
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--enforce)        MODE_ENFORCE=1 ;;
		--since)          shift; SINCE_REF="${1:-}" ;;
		--since=*)        SINCE_REF="${1#--since=}" ;;
		-h|--help|help)   WANT_HELP=1 ;;
		--*)              echo "verify_qa_evidence.sh: unknown flag: $1" >&2; usage >&2; exit 2 ;;
		*)
			if [ -z "$SCAN_N" ]; then
				SCAN_N="$1"
			else
				echo "verify_qa_evidence.sh: unexpected argument: $1" >&2
				usage >&2
				exit 2
			fi
			;;
	esac
	shift
done

if [ "$WANT_HELP" -eq 1 ]; then
	usage
	exit 0
fi

# Resolve repo root so the script works from any cwd.
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
	if [ "$MODE_ENFORCE" -eq 1 ]; then
		echo "verify_qa_evidence.sh: not inside a git repository (--enforce requires one)." >&2
		exit 2
	fi
	echo "verify_qa_evidence.sh: not inside a git repository — nothing to scan (advisory)."
	exit 0
fi
cd "$REPO_ROOT" || exit 2

QA_DIR="docs/qa"

# --------- Enforcing-mode preconditions ---------
if [ "$MODE_ENFORCE" -eq 1 ]; then
	if [ -z "$SINCE_REF" ]; then
		echo "verify_qa_evidence.sh: --enforce requires --since <ref-or-date>." >&2
		echo "  Baseline scoping is mandatory: enforcing over the whole history would" >&2
		echo "  block on thousands of pre-convention legacy commits. Use the commit" >&2
		echo "  that added docs/qa/README.md as the baseline (see script header)." >&2
		exit 2
	fi
	# Validate that --since resolves to something git understands. A SHA / tag
	# resolves via rev-parse; an approxidate (e.g. "2026-05-28") does not, so
	# fall back to a since-date probe with rev-list.
	if ! git rev-parse --verify --quiet "${SINCE_REF}^{commit}" >/dev/null 2>&1; then
		if ! git rev-list -1 --since="$SINCE_REF" HEAD >/dev/null 2>&1; then
			echo "verify_qa_evidence.sh: --since value '$SINCE_REF' is not a valid git ref or date." >&2
			exit 2
		fi
	fi
fi

# --------- Build the set of existing run-id directories ---------
existing_runids=""
if [ -d "$QA_DIR" ]; then
	for d in "$QA_DIR"/*/; do
		[ -d "$d" ] || continue
		rid="$(basename "$d")"
		existing_runids="${existing_runids} ${rid}"
	done
fi

# --------- Determine the commit window ---------
# Advisory  : last N commits (default 20) reachable from HEAD.
# Enforcing : commits in <since>..HEAD (descendants of the baseline), so the
#             baseline commit and everything before it is exempt by
#             merge-ancestry — NOT by author-date sorting. N optionally caps.
if [ "$MODE_ENFORCE" -eq 1 ]; then
	# Resolve the range. If --since is a ref, use range syntax; if it is a
	# date, use --since=. Prefer ref-range (ancestry-accurate) when possible.
	if git rev-parse --verify --quiet "${SINCE_REF}^{commit}" >/dev/null 2>&1; then
		if [ -n "$SCAN_N" ]; then
			commits="$(git rev-list --max-count="$SCAN_N" "${SINCE_REF}..HEAD" 2>/dev/null || true)"
		else
			commits="$(git rev-list "${SINCE_REF}..HEAD" 2>/dev/null || true)"
		fi
	else
		if [ -n "$SCAN_N" ]; then
			commits="$(git rev-list --max-count="$SCAN_N" --since="$SINCE_REF" HEAD 2>/dev/null || true)"
		else
			commits="$(git rev-list --since="$SINCE_REF" HEAD 2>/dev/null || true)"
		fi
	fi
else
	commits="$(git rev-list --max-count="${SCAN_N:-20}" HEAD 2>/dev/null || true)"
fi

# --------- Report header ---------
echo "==========================================================================="
if [ "$MODE_ENFORCE" -eq 1 ]; then
	echo " §11.4.83 docs/qa/ end-user evidence — ENFORCING release gate"
	echo "==========================================================================="
	echo " Scope     : ${SINCE_REF}..HEAD  (commits before the baseline are exempt)"
	echo " Opt-out   : commits whose message contains [no-qa-evidence] are exempt"
	echo " Exit      : 0 when clean, 1 on any violation (release-gate blocking)."
else
	echo " §11.4.83 docs/qa/ end-user evidence — ADVISORY scan (warn-mode, exit 0)"
	echo "==========================================================================="
	echo " Scanning the last ${SCAN_N:-20} commit(s) for feature-shipping commits"
	echo " lacking a matching docs/qa/<run-id>/ directory."
	echo " NOTE: advisory only — never blocks. Run with --enforce --since <ref>"
	echo "       for the §11.4.83 operative-rule-(5) blocking release gate."
fi
echo "---------------------------------------------------------------------------"

violation_count=0
feature_commit_count=0
exempt_count=0

for sha in $commits; do
	# Feature-shipping heuristic: non-test prod code under the watched roots.
	# Use diff-tree plumbing (robust across git versions; `git show
	# --no-patch --name-only` errors in modern git as -s conflicts with
	# --name-only).
	touched="$(git diff-tree --no-commit-id --name-only -r "$sha" 2>/dev/null \
		| grep -E '^helix_code/(internal|cmd|applications)/' \
		| grep -v '_test\.go$' || true)"
	[ -n "$touched" ] || continue

	short="$(git rev-parse --short "$sha" 2>/dev/null || echo "$sha")"
	subject="$(git show --no-patch --format='%s' "$sha" 2>/dev/null || true)"

	# Per-commit opt-out: [no-qa-evidence] anywhere in subject OR body.
	full_msg="$(git show --no-patch --format='%B' "$sha" 2>/dev/null || true)"
	case "$full_msg" in
		*'[no-qa-evidence]'*)
			exempt_count=$((exempt_count + 1))
			echo "  exempt ${short}  ${subject}"
			echo "         -> [no-qa-evidence] opt-out token present"
			continue
			;;
	esac

	feature_commit_count=$((feature_commit_count + 1))

	# Does the commit subject reference a known run-id directory?
	matched=""
	for rid in $existing_runids; do
		case "$subject" in
			*"$rid"*) matched="$rid"; break ;;
		esac
	done

	if [ -n "$matched" ]; then
		echo "  ok     ${short}  ${subject}"
		echo "         -> docs/qa/${matched}/ present"
	else
		violation_count=$((violation_count + 1))
		if [ "$MODE_ENFORCE" -eq 1 ]; then
			echo "  VIOL   ${short}  ${subject}" >&2
			echo "         -> no matching docs/qa/<run-id>/ directory found" >&2
		else
			echo "  WARN   ${short}  ${subject}"
			echo "         -> no matching docs/qa/<run-id>/ directory found"
		fi
	fi
done

echo "---------------------------------------------------------------------------"
echo " Feature-shipping commits evaluated : ${feature_commit_count}"
echo " Opt-out exempt commits             : ${exempt_count}"
if [ "$MODE_ENFORCE" -eq 1 ]; then
	echo " Violations                         : ${violation_count}"
else
	echo " Advisory warnings                  : ${violation_count}"
fi

if [ "$MODE_ENFORCE" -eq 1 ]; then
	if [ "$violation_count" -gt 0 ]; then
		echo >&2
		echo " RELEASE-GATE FAIL: the commit(s) above ship a feature but carry no" >&2
		echo " matching docs/qa/<run-id>/ evidence directory. Per §11.4.83 each" >&2
		echo " shipped feature MUST carry a recorded end-to-end transcript +" >&2
		echo " materials under docs/qa/<run-id>/ (see docs/qa/README.md). Either" >&2
		echo " add the evidence directory, or — for a non-feature change that" >&2
		echo " tripped the heuristic — annotate the commit with [no-qa-evidence]." >&2
		echo "===========================================================================" >&2
		echo " RESULT: FAIL (enforcing — ${violation_count} violation(s))." >&2
		exit 1
	fi
	echo "==========================================================================="
	echo " RESULT: PASS (enforcing — no violations in ${SINCE_REF}..HEAD)."
	exit 0
fi

if [ "$violation_count" -gt 0 ]; then
	echo
	echo " ADVISORY: the commits above appear to ship a feature but have no"
	echo " matching docs/qa/<run-id>/ evidence directory. Per §11.4.83 each"
	echo " shipped feature SHOULD carry a recorded end-to-end transcript +"
	echo " materials. Add one under docs/qa/<run-id>/ (see docs/qa/README.md)."
	echo " This is a notice only — no action is enforced. Use --enforce --since"
	echo " <ref> for the blocking release gate."
fi
echo "==========================================================================="
echo " RESULT: advisory scan complete (exit 0 — warn-mode)."
exit 0
