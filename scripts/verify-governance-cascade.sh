#!/usr/bin/env bash
# scripts/verify-governance-cascade.sh
# Governance cascade verification — exits non-zero on any deficiency.
# Version: 2.0.0
# Author: HelixCode Integration Plan

set -euo pipefail

REQUIRED_FILES=("CONSTITUTION.md" "CLAUDE.md" "AGENTS.md")

# Mandatory text strings that must appear in every governance file.
MANDATORY_PATTERNS=(
  "We had been in position that all tests do execute"
  "bar for shipping is not"
  "CONST-035"
  "Reproduction-Before-Fix"
  "Host Power Management is Forbidden"
  "CONST-042"
  "CONST-043"
)

REPORT_FILE="governance-cascade-report-$(date +%Y%m%d-%H%M%S).txt"
FAILURES=0

# Return true if the submodule path is HelixCode-owned (not third-party).
is_helixcode_owned() {
  local path="$1"
  case "$path" in
    Challenges|Security|Containers|HelixQA)
      return 0
      ;;
    HelixAgent|HelixAgent/HelixLLM|HelixAgent/HelixMemory|HelixAgent/HelixSpecifier)
      return 0
      ;;
    Dependencies/HelixDevelopment/*)
      return 0
      ;;
    # Assets and Github-Pages-Website are ours but excluded (no code/governance mandate):
    Assets|Github-Pages-Website)
      return 1
      ;;
    *)
      return 1
      ;;
  esac
}

# Submodules are discovered dynamically from .gitmodules.
read_submodules() {
  local gitmodules="${1:-.gitmodules}"
  if [[ -f "$gitmodules" ]]; then
    grep '^\s*path = ' "$gitmodules" | sed 's/^\s*path = //'
  fi
}

# Catalogizer submodules are discovered dynamically from .gitmodules.
read_catalogizer_submodules() {
  local gitmodules="${1:-../Catalogizer/.gitmodules}"
  if [[ -f "$gitmodules" ]]; then
    grep '^\s*path = ' "$gitmodules" | sed 's/^\s*path = //'
  fi
}

verify_submodule() {
  local subpath="$1"
  local subname
  subname=$(basename "$subpath")

  echo "--- Submodule: $subname ($subpath) ---" >> "$REPORT_FILE"

  for file in "${REQUIRED_FILES[@]}"; do
    local filepath="$subpath/$file"
    if [[ ! -f "$filepath" ]]; then
      echo "MISSING_FILE: $filepath" >> "$REPORT_FILE"
      ((FAILURES++)) || true
      continue
    fi

    for pattern in "${MANDATORY_PATTERNS[@]}"; do
      if ! grep -q "$pattern" "$filepath"; then
        echo "MISSING_TEXT: $filepath | pattern: $pattern" >> "$REPORT_FILE"
        ((FAILURES++)) || true
      fi
    done
  done
}

# Main execution
echo "Governance Cascade Verification Report — $(date -Iseconds)" > "$REPORT_FILE"
echo "Repo: $(pwd)" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Verify parent repo governance files first
for file in "${REQUIRED_FILES[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "MISSING_FILE: ./$file" >> "$REPORT_FILE"
    ((FAILURES++)) || true
    continue
  fi
  for pattern in "${MANDATORY_PATTERNS[@]}"; do
    if ! grep -q "$pattern" "$file"; then
      echo "MISSING_TEXT: ./$file | pattern: $pattern" >> "$REPORT_FILE"
      ((FAILURES++)) || true
    fi
  done
done

# Verify HelixCode-owned submodules only
while IFS= read -r sub; do
  if is_helixcode_owned "$sub" && [[ -d "$sub" ]]; then
    verify_submodule "$sub"
  fi
done < <(read_submodules)



# Catalogizer cascade (optional — run only when Catalogizer is sibling checkout)
while IFS= read -r sub; do
  if is_helixcode_owned "$sub" && [[ -d "$sub" ]]; then
    verify_submodule "$sub"
  fi
done < <(read_catalogizer_submodules)

echo "" >> "$REPORT_FILE"
echo "TOTAL_FAILURES: $FAILURES" >> "$REPORT_FILE"

if [[ $FAILURES -gt 0 ]]; then
  echo "GOVERNANCE_CASCADE: FAILED ($FAILURES deficiencies)"
  cat "$REPORT_FILE"
  exit 1
else
  echo "GOVERNANCE_CASCADE: PASSED"
  exit 0
fi
