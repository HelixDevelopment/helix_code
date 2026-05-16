#!/usr/bin/env bash
# Challenge: P1-F02 — Permission Rule System end-to-end runtime evidence.
# Exits 0 only if all three scenarios produce the expected decisions
# AND the filesystem markers prove that denied commands really did NOT execute.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../.." && pwd)
BIN="$ROOT/bin/helixcode"

if [[ ! -x "$BIN" ]]; then
  echo "[setup] building helixcode binary..."
  (cd "$ROOT" && go build -o bin/helixcode ./cmd/cli)
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Pre-create the rules file in WORK so S2 and S3 get a Bash(rm*) deny rule.
RULES_DIR="$WORK/.helixcode"
mkdir -p "$RULES_DIR"
cat > "$RULES_DIR/permissions.yaml" <<'EOF'
apiVersion: helixcode.permissions/v1
rules:
  - pattern: "Bash(rm*)"
    action: deny
    priority: 1000
EOF

# Scenario 1: read auto-allowed under dontAsk
echo "=== S1: read auto-allowed under dontAsk ==="
S1=$(HOME="$WORK" "$BIN" permissions check Bash --command "ls -la /tmp" --permission-mode dontAsk)
echo "$S1"
if ! echo "$S1" | grep -q "decision: allow"; then
  echo "FAIL S1: expected decision: allow"
  exit 1
fi

# Scenario 2: destructive denied under default + explicit rule
echo
echo "=== S2: destructive denied under default ==="
MARKER1=/tmp/helixcode-challenge-marker
echo "present" > "$MARKER1"
S2=$(HOME="$WORK" "$BIN" permissions check Bash --command "rm -rf $MARKER1" --permission-mode default)
echo "$S2"
if ! echo "$S2" | grep -q "decision: deny"; then
  echo "FAIL S2: expected decision: deny"
  exit 1
fi
if [[ ! -e "$MARKER1" ]]; then
  echo "FAIL S2: marker $MARKER1 was deleted — deny rule did not block exec"
  exit 1
fi

# Scenario 3: smuggle via $() denied
echo
echo "=== S3: smuggle via command substitution denied ==="
MARKER2=/tmp/helixcode-smuggle-marker
echo "present" > "$MARKER2"
S3=$(HOME="$WORK" "$BIN" permissions check Bash --command "echo hi \$(rm -rf $MARKER2)" --permission-mode auto)
echo "$S3"
if ! echo "$S3" | grep -q "decision: deny"; then
  echo "FAIL S3: expected decision: deny (smuggled rm in \$())"
  exit 1
fi
if [[ ! -e "$MARKER2" ]]; then
  echo "FAIL S3: marker $MARKER2 was deleted — smuggle was not blocked"
  exit 1
fi

# Cleanup markers
rm -f "$MARKER1" "$MARKER2"

echo
echo "PASS: all three scenarios produced expected decisions and markers preserved"
