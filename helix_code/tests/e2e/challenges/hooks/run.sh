#!/usr/bin/env bash
# Challenge: P1-F05 — Hook-Based Extensibility runtime evidence.
# Drives hooks.Manager directly through a Go test binary inside the module tree.
# Per CONST-035: runtime evidence required. Per Article XI §11.9: every
# PASS must demonstrate the feature actually works for the end user.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../.." && pwd)
WORK=$(mktemp -d -p "$ROOT/cmd")
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/driver.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/hooks"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: driver <scenario>")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "s1":
		// S1: a real before_bash hook blocks rm.
		tmp, _ := os.MkdirTemp("", "f05-s1-")
		defer os.RemoveAll(tmp)
		marker := filepath.Join(tmp, "marker")
		os.WriteFile(marker, []byte("present"), 0o644)
		scriptPath := filepath.Join(tmp, "block.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'blocked' >&2; exit 1\n"), 0o755)

		mgr := hooks.NewManager()
		mgr.Register(hooks.NewHook("blocker", hooks.HookTypeBeforeBash, hooks.NewShellRunner(scriptPath, 0)))

		event := hooks.NewEventWithContext(context.Background(), hooks.HookTypeBeforeBash)
		event.SetData("toolName", "Bash")
		event.SetData("params", map[string]interface{}{"command": "rm -rf " + marker})
		results := mgr.TriggerEventAndWait(event)
		blockers := hooks.Blockers(results)

		_, statErr := os.Stat(marker)
		fmt.Printf("blocker_count=%d\n", len(blockers))
		fmt.Printf("marker_present_after=%v\n", statErr == nil)
	case "s2":
		// S2: after_tool_call hook writes one line per call.
		tmp, _ := os.MkdirTemp("", "f05-s2-")
		defer os.RemoveAll(tmp)
		logPath := filepath.Join(tmp, "audit.log")
		scriptPath := filepath.Join(tmp, "audit.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'fired' >> "+logPath+"\n"), 0o755)

		mgr := hooks.NewManager()
		mgr.Register(hooks.NewHook("audit", hooks.HookTypeAfterToolCall, hooks.NewShellRunner(scriptPath, 0)))

		for i := 0; i < 3; i++ {
			event := hooks.NewEventWithContext(context.Background(), hooks.HookTypeAfterToolCall)
			event.SetData("toolName", "X")
			mgr.TriggerEventAndWait(event)
		}

		body, _ := os.ReadFile(logPath)
		lines := 0
		for _, b := range body {
			if b == '\n' {
				lines++
			}
		}
		fmt.Printf("log_lines=%d\n", lines)
	case "s3":
		// S3: malformed YAML → loader returns error.
		tmp, _ := os.MkdirTemp("", "f05-s3-")
		defer os.RemoveAll(tmp)
		yamlPath := filepath.Join(tmp, "hooks.yaml")
		os.WriteFile(yamlPath, []byte("not: valid: yaml: ["), 0o600)

		loader := &hooks.FileLoader{UserPath: yamlPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
		_, _, err := loader.Load(context.Background())
		fmt.Printf("validate_error_present=%v\n", err != nil)
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", os.Args[1])
		os.Exit(2)
	}
}
EOF

DRIVER_BIN="$WORK/driver"
(cd "$ROOT" && go build -o "$DRIVER_BIN" "$WORK/driver.go")

echo "=== S1: block-bash-rm ==="
S1=$("$DRIVER_BIN" s1)
echo "$S1"
if ! echo "$S1" | grep -q "^blocker_count=1$"; then
  echo "FAIL S1: expected exactly 1 blocker"
  exit 1
fi
if ! echo "$S1" | grep -q "^marker_present_after=true$"; then
  echo "FAIL S1: marker was deleted (block did not prevent operation)"
  exit 1
fi

echo
echo "=== S2: audit-after-tool ==="
S2=$("$DRIVER_BIN" s2)
echo "$S2"
if ! echo "$S2" | grep -q "^log_lines=3$"; then
  echo "FAIL S2: expected log to have exactly 3 lines"
  exit 1
fi

echo
echo "=== S3: yaml-validate-malformed ==="
S3=$("$DRIVER_BIN" s3)
echo "$S3"
if ! echo "$S3" | grep -q "^validate_error_present=true$"; then
  echo "FAIL S3: malformed YAML did not produce a load error"
  exit 1
fi

echo
echo "PASS: all three scenarios produced expected outcomes"
