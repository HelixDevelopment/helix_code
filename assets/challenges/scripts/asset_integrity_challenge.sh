#!/usr/bin/env bash
# asset_integrity_challenge.sh — anti-bluff file-integrity Challenge for Assets
# per CONST-035 + CONST-050(B). Round-305 Template-C minimal enrichment
# (owned-static-content: only verifiable invariant is asset presence + integrity).
#
# Verbatim 2026-05-19 operator mandate: "all existing tests and Challenges do
# work in anti-bluff manner - they MUST confirm that all tested codebase really
# works as expected!"
#
# Wire evidence: PNG magic-number check, non-zero size, expected-asset list
# presence. No false-success via grep-only / metadata-only PASS.

set -uo pipefail

ASSETS_ROOT="${ASSETS_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
EXPECTED_ASSETS=("Logo.png" "Wide_Black.png")
MIN_SIZE_BYTES=1024  # asset must be > 1 KiB (sanity floor for a real PNG)
# PNG magic number as hex (89 50 4E 47 0D 0A 1A 0A) — compared via xxd
# because bash variable expansion mangles raw \x89 bytes.
PNG_MAGIC_HEX="89504e470d0a1a0a"

echo "=== Assets File-Integrity Challenge ==="
echo "  root=$ASSETS_ROOT"

fail=0
for asset in "${EXPECTED_ASSETS[@]}"; do
    path="$ASSETS_ROOT/$asset"

    # [1] Existence
    if [[ ! -f "$path" ]]; then
        echo "  FAIL: $asset — missing at $path"
        fail=1
        continue
    fi

    # [2] Non-zero, above sanity floor
    size=$(stat -c%s "$path" 2>/dev/null || stat -f%z "$path" 2>/dev/null || echo 0)
    if [[ "$size" -lt "$MIN_SIZE_BYTES" ]]; then
        echo "  FAIL: $asset — size=$size below floor=$MIN_SIZE_BYTES"
        fail=1
        continue
    fi

    # [3] PNG magic-number wire evidence (not just extension)
    header_hex=$(head -c 8 "$path" 2>/dev/null | xxd -p -c 8 2>/dev/null || echo "")
    if [[ "$header_hex" != "$PNG_MAGIC_HEX" ]]; then
        echo "  FAIL: $asset — PNG magic number mismatch (got=$header_hex want=$PNG_MAGIC_HEX)"
        fail=1
        continue
    fi

    echo "  PASS: $asset — size=$size bytes, PNG magic=$header_hex"
done

# [4] Governance files present (CONST-051(A) equal-codebase enforcement)
for gov in "CLAUDE.md" "AGENTS.md" "CONSTITUTION.md"; do
    if [[ ! -f "$ASSETS_ROOT/$gov" ]]; then
        echo "  FAIL: governance file missing — $gov"
        fail=1
    else
        echo "  PASS: governance file present — $gov"
    fi
done

# [5] Anti-bluff: paired-mutation self-test. If MUTATION_TEST=1, expect
# a known-bad sentinel file to fail; absence of FAIL would be a meta-bluff.
if [[ "${MUTATION_TEST:-0}" == "1" ]]; then
    sentinel="$ASSETS_ROOT/.mutation_sentinel.png"
    printf 'NOT_A_PNG' > "$sentinel"
    mut_header_hex=$(head -c 8 "$sentinel" 2>/dev/null | xxd -p -c 8 2>/dev/null || echo "")
    if [[ "$mut_header_hex" == "$PNG_MAGIC_HEX" ]]; then
        echo "  META-FAIL: mutation test sentinel was accepted — anti-bluff regression!"
        rm -f "$sentinel"
        exit 2
    fi
    echo "  PASS: mutation-test sentinel correctly rejected"
    rm -f "$sentinel"
fi

echo
if [[ "$fail" -ne 0 ]]; then
    echo "=== Assets File-Integrity Challenge: FAILED ==="
    exit 1
fi

echo "=== Assets File-Integrity Challenge: PASSED ==="
echo "  evidence: $(ls -la "$ASSETS_ROOT"/*.png 2>/dev/null | wc -l) PNG asset(s) verified, all governance files present"
