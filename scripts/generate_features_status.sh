#!/usr/bin/env bash
# generate_features_status.sh — docs_chain aggregate transform (§11.4.106).
# Derives docs/features/Status.md from the fixed header + every inventory slice.
# Deterministic: same inputs -> byte-identical output. --stdout prints; else writes the file.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"; cd "$root"
out() {
  cat docs/features/_status_header.md
  printf '\n## Feature inventory\n\n_Aggregated from docs/features/inventory/*.md by scripts/generate_features_status.sh (docs_chain §11.4.106)._\n\n'
  for f in $(ls docs/features/inventory/*.md | sort); do printf '\n'; cat "$f"; printf '\n'; done
  printf '\n## Inventory sources\n\n'
  for f in $(ls docs/features/inventory/*.md | sort); do printf -- '- `%s`\n' "$f"; done
}
if [ "${1:-}" = "--stdout" ]; then out; else out > docs/features/Status.md; fi
