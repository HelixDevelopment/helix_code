#!/usr/bin/env bash
# scripts/gates/workable_items_sync_gate.sh — CM-WORKABLE-ITEMS-MD-DB-IN-SYNC
#
# §11.4.93/95 (HXC-013/HXC-026): the tracked docs/workable_items.db is the
# single source of truth for workable items, kept in lockstep with docs/Issues.md
# + docs/Fixed.md. This gate FAILS the build if md and db have drifted, so a
# committer who edits Issues.md/Fixed.md without regenerating the DB (or vice
# versa) is caught before release.
#
# Invariants checked (all on TEMP COPIES — the tracked docs/workable_items.db is
# NEVER opened in-place, because SQLite WAL-mode mutates the file header even on a
# read open, which would dirty the working tree):
#   1. the committed DB validates (closed-set status/type, §11.4.91 description
#      floor, no duplicate atm_id);
#   2. md→db→md round-trip of the live Issues.md/Fixed.md is byte-identical modulo
#      trailing whitespace (the docs are self-consistent for the binary);
#   3. the committed DB's item set matches a fresh md→db of the live docs (the
#      tracked DB is not stale relative to the md).
#
# Exit 0 = in sync; non-zero = drift (release-gate blocking).
#
# Dependencies: go (CGO, for go-sqlite3), the constitution submodule's
# workable-items binary source. If the binary cannot be built (no CGO/sqlite in
# the environment), the gate SKIPs with reason (honest §11.4.3 — never a fake
# pass), exit 0, because md/db sync cannot be mechanically checked here.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

DB="docs/workable_items.db"
ISSUES="docs/Issues.md"
FIXED="docs/Fixed.md"
WI_SRC="constitution/scripts/workable-items"

fail() { echo "CM-WORKABLE-ITEMS-MD-DB-IN-SYNC: FAIL — $1" >&2; exit 1; }
skip() { echo "CM-WORKABLE-ITEMS-MD-DB-IN-SYNC: SKIP-OK — $1"; exit 0; }

[ -f "$DB" ] || fail "tracked $DB missing (§11.4.95 requires it tracked)"
[ -f "$ISSUES" ] || fail "$ISSUES missing"
[ -f "$FIXED" ] || fail "$FIXED missing"
[ -d "$WI_SRC" ] || skip "workable-items binary source not present at $WI_SRC (constitution submodule not checked out)"
command -v go >/dev/null 2>&1 || skip "go toolchain not available"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

BIN="$TMP/wi"
if ! ( cd "$WI_SRC" && go build -o "$BIN" ./cmd/workable-items ) 2>"$TMP/build.err"; then
    # CGO/sqlite unavailable is an environment limitation, not a sync defect.
    if grep -qiE 'cgo|sqlite|gcc|cc1|exec: "gcc"' "$TMP/build.err"; then
        skip "workable-items binary cannot build in this env (CGO/sqlite): $(tail -1 "$TMP/build.err")"
    fi
    fail "workable-items binary build failed: $(tail -3 "$TMP/build.err" | tr '\n' ' ')"
fi

# (1) committed DB validates — on a temp copy so the tracked file is untouched.
cp "$DB" "$TMP/committed.db"
if ! "$BIN" validate --db "$TMP/committed.db" >"$TMP/validate.out" 2>&1; then
    fail "committed DB fails validate: $(tail -2 "$TMP/validate.out" | tr '\n' ' ')"
fi

# (2) md→db→md round-trip byte-identical (modulo trailing whitespace).
"$BIN" sync md-to-db --db "$TMP/fresh.db" --issues "$ISSUES" --fixed "$FIXED" >/dev/null 2>"$TMP/sync.err" \
    || fail "md-to-db failed: $(tail -2 "$TMP/sync.err" | tr '\n' ' ')"
"$BIN" sync db-to-md --db "$TMP/fresh.db" --out-issues "$TMP/rt_issues.md" --out-fixed "$TMP/rt_fixed.md" >/dev/null 2>"$TMP/rt.err" \
    || fail "db-to-md failed: $(tail -2 "$TMP/rt.err" | tr '\n' ' ')"
norm() { sed 's/[[:space:]]*$//' "$1"; }
if ! diff -q <(norm "$ISSUES") <(norm "$TMP/rt_issues.md") >/dev/null; then
    fail "$ISSUES does not round-trip md→db→md byte-identically (regenerate docs/workable_items.db)"
fi
if ! diff -q <(norm "$FIXED") <(norm "$TMP/rt_fixed.md") >/dev/null; then
    fail "$FIXED does not round-trip md→db→md byte-identically (regenerate docs/workable_items.db)"
fi

# (3) committed DB's md projection matches the live docs (DB not stale vs md).
"$BIN" sync db-to-md --db "$TMP/committed.db" --out-issues "$TMP/committed_issues.md" --out-fixed "$TMP/committed_fixed.md" >/dev/null 2>"$TMP/cdb.err" \
    || fail "db-to-md on committed DB failed: $(tail -2 "$TMP/cdb.err" | tr '\n' ' ')"
if ! diff -q <(norm "$ISSUES") <(norm "$TMP/committed_issues.md") >/dev/null; then
    fail "committed $DB is STALE vs $ISSUES — regenerate it (sync md-to-db) + WAL-checkpoint + recommit"
fi
if ! diff -q <(norm "$FIXED") <(norm "$TMP/committed_fixed.md") >/dev/null; then
    fail "committed $DB is STALE vs $FIXED — regenerate it (sync md-to-db) + WAL-checkpoint + recommit"
fi

echo "CM-WORKABLE-ITEMS-MD-DB-IN-SYNC: PASS — committed DB validates + md⟷db byte-identical in sync"
exit 0
