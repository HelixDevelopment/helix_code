#!/bin/bash
# pgo-refresh.sh — speed-programme Phase 4 task P4-T01.
#
# Regenerates the committed Profile-Guided-Optimization profile
# (cmd/cli/default.pgo + cmd/server/default.pgo).
#
# Go's toolchain applies PGO automatically when a `default.pgo` file sits
# next to a package's `main.go` — so refreshing the profile is the only
# action needed to re-tune code generation for the current hot paths.
#
# The merged profile combines two representative sources:
#   1. The Phase-0 baseline CPU profiles for the four canonical scenarios
#      S1-S4 (docs/research/speed/baseline/pprof/S{1..4}-cpu.pprof) —
#      filesystem walk, os.ReadFile, runtime hot paths.
#   2. Freshly captured CPU profiles from the repo-map (tree-sitter parse —
#      the CPU bottleneck) and filesystem-search (regexp grep) benchmark
#      suites — captured live by this script so the profile tracks current
#      code.
#
# `go tool pprof -proto` merges them into one proto-format profile, which
# is copied to both cmd/ directories. The profile is a committed build
# INPUT (the source of the optimization), not a regenerable artefact in
# the CONST-053 sense — it is committed to version control.
#
# PGO only changes code generation; it never changes behaviour. After a
# refresh, run the full test suite to confirm no regression before
# committing the new profile.
#
# Usage:  make pgo-refresh   (or)   bash scripts/pgo-refresh.sh
set -euo pipefail

# Resolve the inner Go module root (this script lives in <module>/scripts/).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$MODULE_ROOT/.." && pwd)"
cd "$MODULE_ROOT"

BASELINE_DIR="$REPO_ROOT/docs/research/speed/baseline/pprof"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> PGO refresh — module root: $MODULE_ROOT"

# 1. Phase-0 baseline scenario CPU profiles (S1-S4).
PHASE0_PROFILES=()
for s in S1 S2 S3 S4; do
	p="$BASELINE_DIR/${s}-cpu.pprof"
	if [ -f "$p" ]; then
		PHASE0_PROFILES+=("$p")
	else
		echo "    WARN: missing Phase-0 profile $p (skipping)"
	fi
done

# 2. Fresh benchmark CPU profiles — repo-map (tree-sitter) + filesystem search.
echo "==> Capturing repo-map benchmark CPU profile..."
go test -run='^$' -bench=. -benchtime=2x -timeout=300s \
	-cpuprofile="$WORK_DIR/repomap.pgo" ./internal/repomap/ \
	>"$WORK_DIR/repomap.log" 2>&1 || {
	echo "    WARN: repo-map benchmark failed — see $WORK_DIR/repomap.log"; }

echo "==> Capturing filesystem-search benchmark CPU profile..."
go test -run='^$' -bench=. -benchtime=1x -timeout=300s \
	-cpuprofile="$WORK_DIR/filesystem.pgo" ./internal/tools/filesystem/ \
	>"$WORK_DIR/filesystem.log" 2>&1 || {
	echo "    WARN: filesystem benchmark failed — see $WORK_DIR/filesystem.log"; }

FRESH_PROFILES=()
for f in repomap filesystem; do
	if [ -s "$WORK_DIR/$f.pgo" ]; then
		FRESH_PROFILES+=("$WORK_DIR/$f.pgo")
	fi
done

ALL_PROFILES=("${PHASE0_PROFILES[@]}" "${FRESH_PROFILES[@]}")
if [ "${#ALL_PROFILES[@]}" -eq 0 ]; then
	echo "ERROR: no input profiles available — cannot build default.pgo" >&2
	exit 1
fi

# 3. Merge into one proto-format profile.
echo "==> Merging ${#ALL_PROFILES[@]} profile(s) into default.pgo..."
go tool pprof -proto "${ALL_PROFILES[@]}" >"$WORK_DIR/default.pgo"

# 4. Install next to each main.go — Go picks it up automatically.
cp "$WORK_DIR/default.pgo" "$MODULE_ROOT/cmd/cli/default.pgo"
cp "$WORK_DIR/default.pgo" "$MODULE_ROOT/cmd/server/default.pgo"
echo "==> Installed cmd/cli/default.pgo + cmd/server/default.pgo"

echo "==> Merged profile summary:"
go tool pprof -top -nodecount=10 "$WORK_DIR/default.pgo" 2>/dev/null | sed 's/^/    /'

echo ""
echo "PGO profile refreshed. Next steps:"
echo "  1. go build ./cmd/cli/... ./cmd/server/...   (confirms PGO build is clean)"
echo "  2. go test ./...                             (confirms no regression)"
echo "  3. commit cmd/cli/default.pgo + cmd/server/default.pgo"
