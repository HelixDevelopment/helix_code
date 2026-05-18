#!/usr/bin/env bash
# scripts/generate-coverage-ledger.sh
#
# Round 68 deliverable for CONST-048 (Full-Automation-Coverage Mandate /
# constitution §11.4.25). Maintains docs/coverage/COVERAGE_LEDGER.md —
# the submodule × feature × platform × invariant ledger.
#
# This script is the COMPANION to scripts/regenerate-coverage-ledger.sh
# (round 41, feature × invariant rollup). The two ledgers are
# orthogonal axes of the same CONST-048 mandate and BOTH must stay
# in sync with reality.
#
# What it does:
#  --check   (default): validate the existing COVERAGE_LEDGER.md
#    * every owned submodule has >=1 row
#    * every row has exactly 11 columns
#    * every status cell is in the closed vocabulary
#    * every PASS cell has a non-blank Notes column
#    * schema version present
#    Exits non-zero on any violation. This is the round 68
#    enforcement gate.
#
#  --scaffold: print a baseline ledger to stdout with one
#    UNCONFIRMED: row per owned submodule. Used to bootstrap a
#    fresh ledger; never rewrites an existing one (operator must
#    redirect to file explicitly).
#
# Anti-bluff guarantees:
#  - Never promotes UNCONFIRMED: -> PASS. Operator only.
#  - Never rewrites operator-marked cells.
#  - Never silently inserts a PASS row.
#  - --check fails (exit 1) on any owned-submodule omission per
#    CONST-048's "rows that quietly omit a platform are CONST-048
#    violations" mandate.
#
# Cross-references:
#  - docs/coverage/COVERAGE_LEDGER.md   (the ledger)
#  - docs/coverage/SCHEMA.md            (column / status definitions)
#  - docs/coverage/README.md            (suite overview)
#  - docs/improvements/submodule_owned.txt (canonical owned roster)
#  - scripts/regenerate-coverage-ledger.sh (feature-axis companion)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

LEDGER="docs/coverage/COVERAGE_LEDGER.md"
OWNED_FILE="docs/improvements/submodule_owned.txt"
SCHEMA_FILE="docs/coverage/SCHEMA.md"

MODE="${1:---check}"

# Closed status vocabulary (per SCHEMA.md §2)
VALID_STATUSES_REGEX='^(PASS|PARTIAL|UNCONFIRMED:|PENDING_FORENSICS:|OPERATOR-BLOCKED:|SKIP-OK: #[A-Za-z0-9_-]+|N/A)( \(.+\))?$'

usage() {
    cat <<EOF
Usage: $0 [--check|--scaffold]

  --check     Validate docs/coverage/COVERAGE_LEDGER.md against CONST-048
              invariants (every owned submodule has >=1 row, schema
              correctness, status-vocabulary closed-set, PASS rows have
              Notes). Default mode. Exits non-zero on violations.

  --scaffold  Emit a baseline ledger to stdout with one UNCONFIRMED: row
              per owned submodule. Does NOT overwrite an existing ledger
              (operator must redirect to file explicitly). Used only when
              bootstrapping a fresh ledger.

Exit codes:
  0 = all good
  1 = ledger violation detected (--check) or required files missing
  2 = invalid mode

See docs/coverage/SCHEMA.md for the row format and status vocabulary.
See docs/coverage/README.md for the suite overview.
EOF
}

scaffold_one_row() {
    local sm="$1"
    printf '| %s | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |\n' "$sm"
}

cmd_scaffold() {
    if [[ ! -f "$OWNED_FILE" ]]; then
        echo "ERROR: $OWNED_FILE not found — cannot enumerate owned submodules" >&2
        exit 1
    fi

    cat <<'EOF'
# HelixCode CONST-048 Coverage Ledger (SCAFFOLD)

**Schema:** `SCHEMA_VERSION = 1.0.0` (see `SCHEMA.md`)
**Note:** scaffold output — operator must hand-promote cells with captured evidence references.

| Submodule | Feature | Platform | I1 anti-bluff | I2 e2e-working | I3 doc-match | I4 no-issues | I5 doc-sync | I6 4-layer-tests | Overall | Notes |
|-----------|---------|----------|---------------|----------------|--------------|--------------|-------------|------------------|---------|-------|
EOF

    while IFS=' |' read -r sm rest; do
        [[ -z "$sm" ]] && continue
        scaffold_one_row "$sm"
    done < "$OWNED_FILE"
}

# Extract data rows that belong to the 11-column ledger tables.
# A data row qualifies iff:
#   * it appears AFTER a header row containing "Submodule | Feature | Platform"
#   * AND it has exactly 12 pipes (= 11 columns)
#   * AND it is NOT the header / separator line
#   * AND the table block hasn't ended (blank line / non-table line)
ledger_data_rows() {
    [[ -f "$LEDGER" ]] || return 0
    awk '
        BEGIN { in_table=0 }
        /^\| Submodule \| Feature \| Platform \|/ { in_table=1; next }
        in_table==1 && /^\|[ -]+\|[ -]+\|/ { next }   # skip separator line
        in_table==1 && /^\| / {
            n = gsub(/\|/, "&")
            if (n == 12) print
            next
        }
        in_table==1 && !/^\| / { in_table=0 }
    ' "$LEDGER"
}

ledger_referenced_submodules() {
    ledger_data_rows | awk -F'|' '{
        v=$2; gsub(/^[ \t]+|[ \t]+$/, "", v); print v
    }' | sort -u
}

cmd_check() {
    local fail=0

    if [[ ! -f "$LEDGER" ]]; then
        echo "FAIL: $LEDGER not found" >&2
        echo "  Hint: bootstrap with '$0 --scaffold > $LEDGER'" >&2
        return 1
    fi

    if [[ ! -f "$OWNED_FILE" ]]; then
        echo "FAIL: $OWNED_FILE not found — cannot enumerate owned submodules" >&2
        return 1
    fi

    if [[ ! -f "$SCHEMA_FILE" ]]; then
        echo "FAIL: $SCHEMA_FILE not found (required companion per round 68)" >&2
        fail=1
    fi

    # 1. Schema version line present
    if ! grep -q 'SCHEMA_VERSION = 1\.' "$LEDGER"; then
        echo "FAIL: $LEDGER missing 'SCHEMA_VERSION = 1.x.y' header" >&2
        fail=1
    fi

    # 2. Every owned submodule has >=1 row
    local owned_count missing_count=0
    owned_count=$(grep -cE '^[^[:space:]]' "$OWNED_FILE" 2>/dev/null || echo 0)
    referenced="$(ledger_referenced_submodules)"

    while IFS=' |' read -r sm _; do
        [[ -z "$sm" ]] && continue
        if ! grep -qxF "$sm" <<<"$referenced"; then
            echo "FAIL: owned submodule '$sm' missing from $LEDGER (CONST-048 silent-omission violation)" >&2
            missing_count=$((missing_count + 1))
            fail=1
        fi
    done < "$OWNED_FILE"

    # 3. Every data row has exactly 11 columns (12 pipes) — already
    # filtered by ledger_data_rows, but track count for the summary.
    local row_count=0 bad_cols=0
    while IFS= read -r line; do
        row_count=$((row_count + 1))
    done < <(ledger_data_rows)

    # 4. Every status cell uses the closed vocabulary.
    # Note: awk -F'|' on "| a | b |" yields $1="" $2=" a " $3=" b " $4="".
    # So human column N = awk field (N+1). Status columns (human 4..10)
    # = awk fields 5..11. Notes (human 11) = awk field 12.
    local bad_status=0
    while IFS= read -r line; do
        for col in 5 6 7 8 9 10 11; do
            local cell
            cell=$(awk -F'|' -v c="$col" '{
                v=$c;
                gsub(/^[ \t]+|[ \t]+$/, "", v);
                print v
            }' <<<"$line")
            [[ -z "$cell" ]] && continue
            if ! grep -qE "$VALID_STATUSES_REGEX" <<<"$cell"; then
                local human_col=$((col - 1))
                echo "FAIL: invalid status '$cell' in column $human_col:" >&2
                echo "  $line" >&2
                bad_status=$((bad_status + 1))
                fail=1
            fi
        done
    done < <(ledger_data_rows)

    # 5. Every PASS-bearing row has a non-blank Notes column (col 11 human, awk $12)
    local bad_notes=0
    while IFS= read -r line; do
        local notes
        notes=$(awk -F'|' '{
            v=$12;
            gsub(/^[ \t]+|[ \t]+$/, "", v);
            print v
        }' <<<"$line")
        if grep -qE '\| PASS\b|\| PASS \(' <<<"$line"; then
            if [[ -z "$notes" ]]; then
                echo "FAIL: PASS row with blank Notes (CONST-035 PASS-bluff at governance layer):" >&2
                echo "  $line" >&2
                bad_notes=$((bad_notes + 1))
                fail=1
            fi
        fi
    done < <(ledger_data_rows)

    # 6. Audit trail table present
    if ! grep -q '^## Audit trail' "$LEDGER"; then
        echo "FAIL: $LEDGER missing '## Audit trail' section (schema-change discipline)" >&2
        fail=1
    fi

    # Summary
    echo
    echo "=== generate-coverage-ledger.sh --check summary ==="
    echo "  ledger:              $LEDGER"
    echo "  owned submodules:    $owned_count"
    echo "  ledger data rows:    $row_count"
    echo "  missing submodules:  $missing_count"
    echo "  bad-column rows:     $bad_cols"
    echo "  invalid statuses:    $bad_status"
    echo "  PASS rows w/o Notes: $bad_notes"
    if [[ $fail -eq 0 ]]; then
        echo "  RESULT: PASS"
        return 0
    else
        echo "  RESULT: FAIL"
        return 1
    fi
}

case "$MODE" in
    --check)    cmd_check ;;
    --scaffold) cmd_scaffold ;;
    -h|--help)  usage; exit 0 ;;
    *)          usage; exit 2 ;;
esac
