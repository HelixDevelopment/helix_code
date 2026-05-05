#!/usr/bin/env bash
# Challenge: P1-F04 — Git Worktree Agent Isolation end-to-end runtime evidence.
# Drives the worktree.Manager directly through a Go test binary.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../.." && pwd)
WORK=$(mktemp -d -p "$ROOT/cmd")
trap 'rm -rf "$WORK"' EXIT

# Build a tiny Go driver. It must live inside the module tree because Go's
# internal/-package rules forbid imports from outside.
cat > "$WORK/driver.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/tools/worktree"
)

func mustRun(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git %v in %s: %v\n%s\n", args, dir, err, out)
		os.Exit(1)
	}
}

func initRepo() string {
	tmp, err := os.MkdirTemp("", "f04-driver-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mustRun(tmp, "init", "-b", "main")
	mustRun(tmp, "config", "user.email", "x@y")
	mustRun(tmp, "config", "user.name", "x")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mustRun(tmp, "add", ".")
	mustRun(tmp, "commit", "-m", "seed")
	return tmp
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: driver <scenario>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "s1":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		mainHEADBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
		wtPath, err := m.EnterWorktree(context.Background(), "feature-x", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "EnterWorktree:", err)
			os.Exit(1)
		}
		_ = os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("isolated"), 0o644)
		mustRun(wtPath, "add", ".")
		mustRun(wtPath, "commit", "-m", "isolated work")
		mainHEADAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
		_, statErr := os.Stat(filepath.Join(repo, "new.txt"))
		fmt.Printf("main_head_unchanged=%v\n", strings.TrimSpace(string(mainHEADBefore)) == strings.TrimSpace(string(mainHEADAfter)))
		fmt.Printf("new_file_not_in_main=%v\n", os.IsNotExist(statErr))
	case "s2":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		first, err := m.EnterWorktree(context.Background(), "feature-y", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "first enter:", err)
			os.Exit(1)
		}
		second, err := m.EnterWorktree(context.Background(), "feature-y", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "second enter:", err)
			os.Exit(1)
		}
		fmt.Printf("first_path_equals_second_path=%v\n", first == second)
	case "s3":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		bads := []string{"../etc", "", "name with spaces", strings.Repeat("a", 65)}
		allRejected := true
		for _, name := range bads {
			if _, err := m.EnterWorktree(context.Background(), name, ""); err == nil {
				fmt.Fprintf(os.Stderr, "name %q was accepted but should be rejected\n", name)
				allRejected = false
			}
		}
		fmt.Printf("all_rejected=%v\n", allRejected)
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", os.Args[1])
		os.Exit(2)
	}
}
EOF

DRIVER_BIN="$WORK/driver"
(cd "$ROOT" && go build -o "$DRIVER_BIN" "$WORK/driver.go")

echo "=== S1: isolation preserves main ==="
S1_OUT=$("$DRIVER_BIN" s1)
echo "$S1_OUT"
if ! echo "$S1_OUT" | grep -q "^main_head_unchanged=true$"; then
  echo "FAIL S1: main HEAD changed after worktree commit"
  exit 1
fi
if ! echo "$S1_OUT" | grep -q "^new_file_not_in_main=true$"; then
  echo "FAIL S1: new file leaked into main worktree"
  exit 1
fi

echo
echo "=== S2: clean re-entry idempotent ==="
S2_OUT=$("$DRIVER_BIN" s2)
echo "$S2_OUT"
if ! echo "$S2_OUT" | grep -q "^first_path_equals_second_path=true$"; then
  echo "FAIL S2: re-entry returned different path"
  exit 1
fi

echo
echo "=== S3: invalid names rejected ==="
S3_OUT=$("$DRIVER_BIN" s3)
echo "$S3_OUT"
if ! echo "$S3_OUT" | grep -q "^all_rejected=true$"; then
  echo "FAIL S3: at least one invalid name was accepted"
  exit 1
fi

echo
echo "PASS: all three scenarios produced expected outcomes"
