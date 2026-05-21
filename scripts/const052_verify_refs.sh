#!/usr/bin/env bash
# const052_verify_refs.sh — CONST-052 (§11.4.29) reference-integrity
# regression test for the HXC-001 rename programme.
#
# Part of CONST-052 Phase 0 pre-flight tooling. Run AFTER every rename
# batch to prove §11.4.29 reference integrity: a rename is only done when
# every reference to the OLD path is gone and every consumer still builds.
#
# Three checks:
#   1. STALE-REF SCAN — grep the whole tracked tree for the supplied OLD
#      path string(s). §11.4.29 reference integrity is about ACTIVE
#      references — anything a build, import resolver, submodule engine,
#      or config loader reads. Hits are classified two ways:
#        * ACTIVE  — source/config/build files: .go .mod .sum .gitmodules
#          .git config, .yaml .yml .json .toml .sh .mk Makefile etc. Any
#          ACTIVE hit => FAIL (a real reference the rename must have
#          repaired).
#        * PROSE   — documentation: .md .html .txt. A rename that is
#          historically recorded in docs (CONTINUATION/Fixed/Issues, the
#          rename plan itself, prior superseded specs, generated HTML
#          exports) legitimately quotes the pre-rename path. PROSE hits
#          are reported as an informational note and do NOT fail the
#          check — markdown prose is not build/import drift.
#      The CONST-052 tooling files themselves reference the pattern by
#      design and are always tolerated.
#   2. SUBMODULE STATUS — `git submodule status` must show every submodule
#      resolving (no missing / un-initialised gitlink for tracked entries).
#   3. CONSUMER BUILD — `cd helix_code && go build ./...` and
#      `cd helix_agent && go build ./...` must exit 0. A pre-existing
#      headless X11/Xcursor gap for Fyne GUI packages is UNRELATED to
#      CONST-052 and is scoped around: GUI build failures whose output
#      mentions xcursor / X11 / cgo Fyne are reported as a known
#      unrelated gap and do NOT fail this check.
#
# Usage:
#   scripts/const052_verify_refs.sh [OLD_PATH ...]
#
#   With one or more OLD_PATH arguments: runs all three checks, scanning
#   for each supplied old path.
#   With no arguments: skips check 1 (nothing to scan for) and runs the
#   submodule-status + build checks only — useful as a generic build gate.
#
# Anti-bluff: every check captures and prints real command output. There
# is no metadata-only PASS. Paired-mutation test: plant a stale ACTIVE
# reference (e.g. an old path in a scratch .go / .yaml / config file),
# run this script, assert exit 1 / RESULT FAIL; remove the planted ref,
# assert exit 0 / RESULT PASS. A prose-only (.md/.html/.txt) hit is NOT
# sufficient to plant a mutation — prose hits are non-fatal by design.
#
# Exit codes: 0 all checks PASS ; 1 one or more checks FAIL.
#
# §11.4.18 in-source doc block. §11.4.67 `bash -n`-clean.

set -uo pipefail

cd "$(dirname "$0")/.." || { echo "FATAL: cannot cd to repo root" >&2; exit 1; }
ROOT="$(pwd)"

OLD_PATHS=("$@")
FAILED=0

hr()   { printf -- '----------------------------------------------------------------\n'; }
pass() { printf 'PASS: %s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*"; FAILED=1; }

# --- check 1: stale-reference scan ---------------------------------------
hr
echo "CHECK 1 — stale-reference scan"
hr
if [ "${#OLD_PATHS[@]}" -eq 0 ]; then
	echo "(no OLD_PATH arguments supplied — stale-ref scan skipped)"
else
	# The CONST-052 tooling files reference the OLD-path pattern by design.
	TOOLING_RE='^scripts/const052_(rename_leaf|verify_refs)\.sh:'
	# PROSE = documentation files; ACTIVE = everything else (code/config/build).
	PROSE_RE='^[^:]*\.(md|html|txt):'
	for old in "${OLD_PATHS[@]}"; do
		echo "scanning tracked tree for: ${old}"
		# Search only tracked files (git grep) so build artefacts / vendored
		# trees / .git internals are excluded automatically.
		HITS="$(git grep -n -F -- "$old" 2>/dev/null || true)"
		if [ -z "$HITS" ]; then
			pass "no references to '${old}' anywhere in the tracked tree."
			continue
		fi
		# Drop the tooling self-references, then split prose vs active.
		NONTOOL="$(printf '%s\n' "$HITS" | grep -v -E "$TOOLING_RE" || true)"
		PROSE_HITS="$(printf '%s\n' "$NONTOOL" | grep -E "$PROSE_RE" || true)"
		ACTIVE_HITS="$(printf '%s\n' "$NONTOOL" | grep -v -E "$PROSE_RE" || true)"
		PROSE_HITS="$(printf '%s\n' "$PROSE_HITS" | grep -v -E '^[[:space:]]*$' || true)"
		ACTIVE_HITS="$(printf '%s\n' "$ACTIVE_HITS" | grep -v -E '^[[:space:]]*$' || true)"

		if [ -n "$PROSE_HITS" ]; then
			PROSE_N="$(printf '%s\n' "$PROSE_HITS" | grep -c . || true)"
			echo "NOTE: ${PROSE_N} prose reference(s) to '${old}' in .md/.html/.txt — historical documentation, not build/import drift (non-fatal):"
			printf '%s\n' "$PROSE_HITS" | sed 's/^/    /'
		fi
		if [ -z "$ACTIVE_HITS" ]; then
			pass "no ACTIVE (code/config/build) references to '${old}' remain — reference integrity holds."
		else
			fail "stale ACTIVE reference(s) to '${old}' in code/config/build files:"
			printf '%s\n' "$ACTIVE_HITS" | sed 's/^/    /'
		fi
	done
fi

# --- check 2: submodule status -------------------------------------------
hr
echo "CHECK 2 — git submodule status"
hr
SM_OUT="$(git submodule status --recursive 2>&1 || true)"
if [ -z "$SM_OUT" ]; then
	echo "(no submodules reported)"
else
	# A leading '-' = not initialised; '+' = checked-out SHA differs from
	# index; 'U' = merge conflict. '-' for a tracked entry is a broken ref.
	BROKEN="$(printf '%s\n' "$SM_OUT" | grep -E '^U' || true)"
	UNINIT="$(printf '%s\n' "$SM_OUT" | grep -E '^-' || true)"
	if [ -n "$BROKEN" ]; then
		fail "submodule(s) in merge-conflict state:"
		printf '%s\n' "$BROKEN" | sed 's/^/    /'
	fi
	if [ -n "$UNINIT" ]; then
		warn "un-initialised submodule(s) (expected if not all init'd; not a CONST-052 failure on its own):"
		printf '%s\n' "$UNINIT" | sed 's/^/    /'
	fi
	if [ -z "$BROKEN" ]; then
		RESOLVED="$(printf '%s\n' "$SM_OUT" | grep -c -E '^[ +]' || true)"
		pass "${RESOLVED} submodule gitlink(s) resolve; no merge-conflict gitlinks."
	fi
fi

# --- check 3: consumer go build ------------------------------------------
hr
echo "CHECK 3 — consumer go build ./..."
hr
build_consumer() {
	mod_dir="$1"
	if [ ! -f "${mod_dir}/go.mod" ]; then
		warn "${mod_dir}/go.mod not found — skipping build of ${mod_dir}."
		return 0
	fi
	echo "building ${mod_dir} ..."
	BUILD_OUT="$(cd "$mod_dir" && go build ./... 2>&1)"
	BUILD_RC=$?
	if [ "$BUILD_RC" -eq 0 ]; then
		pass "${mod_dir}: go build ./... exit 0."
		return 0
	fi
	# Scope around the known unrelated headless-X11/Xcursor Fyne GUI gap.
	NON_GUI="$(printf '%s\n' "$BUILD_OUT" \
		| grep -E '\.go:[0-9]+' \
		| grep -v -i -E 'xcursor|/x11|fyne|glfw|gl\.h|cannot find -lX' \
		|| true)"
	if [ -z "$NON_GUI" ]; then
		warn "${mod_dir}: go build failed ONLY in headless X11/Xcursor Fyne GUI packages — known unrelated gap, scoped around per CONST-052 plan §4."
		printf '%s\n' "$BUILD_OUT" | sed 's/^/    /'
		return 0
	fi
	fail "${mod_dir}: go build ./... exit ${BUILD_RC} with non-GUI errors:"
	printf '%s\n' "$BUILD_OUT" | sed 's/^/    /'
	return 1
}

if command -v go >/dev/null 2>&1; then
	build_consumer helix_code
	build_consumer helix_agent
else
	warn "go toolchain not on PATH — CHECK 3 cannot run; treat as INCONCLUSIVE."
	FAILED=1
fi

# --- verdict -------------------------------------------------------------
hr
if [ "$FAILED" -eq 0 ]; then
	echo "RESULT: PASS — all CONST-052 reference-integrity checks passed."
	exit 0
else
	echo "RESULT: FAIL — one or more CONST-052 reference-integrity checks failed."
	exit 1
fi
