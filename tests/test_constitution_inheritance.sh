#!/bin/bash
set -o pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0
check() {
  local desc="$1"; shift
  local cmd="$1"
  if eval "$cmd" 2>/dev/null; then PASS=$((PASS+1)); echo "  PASS: $desc"
  else FAIL=$((FAIL+1)); echo "  FAIL: $desc"; fi
}
echo "=== Constitution Inheritance Gate ==="
check "constitution dir exists" 'test -d "$ROOT/constitution"'
check "Constitution.md has forensic anchor" 'grep -qF "End-user quality guarantee" "$ROOT/constitution/Constitution.md"'
check "constitution CLAUDE.md has anti-bluff" 'grep -qF "ANTI-BLUFF" "$ROOT/constitution/CLAUDE.md"'
check "constitution AGENTS.md exists" 'test -f "$ROOT/constitution/AGENTS.md"'
check "project inherits" 'grep -qF "INHERITED FROM constitution" "$ROOT/CLAUDE.md"'
for sub in submodules/helix_qa submodules/helix_llm submodules/llms_verifier \
           submodules/helix_agent submodules/challenges submodules/streaming submodules/watcher; do
  sp="$ROOT/$sub"
  [ -d "$sp" ] || continue
  check "$sub inherits" 'test -f "$sp/CLAUDE.md" -a -n "$(head -3 "$sp/CLAUDE.md" | grep -E "INHERITED|constitution")"'
done
echo "=== $PASS PASS, $FAIL FAIL ==="
[ "$FAIL" -eq 0 ]
