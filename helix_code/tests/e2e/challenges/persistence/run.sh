#!/usr/bin/env bash
# Challenge: P1-F03 — Tool Result Persistence end-to-end runtime evidence.
# Drives the persistence.Manager directly through a Go driver that
# emits machine-readable evidence (the persisted path + file size) to stdout.
#
# The driver is written into a temp dir under cmd/ so that Go's internal-package
# restriction is satisfied (the file lives inside the dev.helix.code module tree).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)/HelixCode"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Driver must live inside the Go module so internal/ imports are allowed.
DRIVER_DIR=$(mktemp -d -p cmd)
trap 'rm -rf "$DRIVER_DIR"' EXIT
DRIVER="$DRIVER_DIR/main.go"

cat > "$DRIVER" <<'EOF'
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/tools/persistence"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: driver <projectRoot> <scenario> <inputSize>")
		os.Exit(2)
	}
	projectRoot := os.Args[1]
	scenario := os.Args[2]
	var size int
	if _, err := fmt.Sscanf(os.Args[3], "%d", &size); err != nil {
		fmt.Fprintf(os.Stderr, "bad size: %v\n", err)
		os.Exit(2)
	}

	m := persistence.NewManager(projectRoot)
	output := strings.Repeat("X", size)

	switch scenario {
	case "single":
		res, err := m.MaybePersist("Bash", "call-1", output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MaybePersist error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("was_persisted=%v\n", res.WasPersisted)
		fmt.Printf("path=%s\n", res.PersistedOutputPath)
		fmt.Printf("size=%d\n", res.PersistedOutputSize)
		fmt.Printf("dir_exists=%v\n", dirExists(filepath.Join(projectRoot, persistence.PersistDir)))
	case "twice":
		r1, err := m.MaybePersist("Bash", "call-1", output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "first MaybePersist error: %v\n", err)
			os.Exit(1)
		}
		r2, err := m.MaybePersist("Bash", "call-2", output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "second MaybePersist error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("first_path=%s\n", r1.PersistedOutputPath)
		fmt.Printf("second_path=%s\n", r2.PersistedOutputPath)
		base1 := filepath.Base(r1.PersistedOutputPath)
		base2 := filepath.Base(r2.PersistedOutputPath)
		parts1 := strings.SplitN(base1, "_", 3)
		parts2 := strings.SplitN(base2, "_", 3)
		hashesMatch := len(parts1) >= 2 && len(parts2) >= 2 && parts1[1] == parts2[1]
		fmt.Printf("hashes_match=%v\n", hashesMatch)
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", scenario)
		os.Exit(2)
	}
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
EOF

# Scenario 1: below threshold → inline, dir not created
echo "=== S1: below-threshold inline ==="
S1_ROOT="$WORK/s1"
mkdir -p "$S1_ROOT"
S1_OUT=$(go run "$DRIVER" "$S1_ROOT" single 49999)
echo "$S1_OUT"
if ! echo "$S1_OUT" | grep -q "^was_persisted=false$"; then
  echo "FAIL S1: expected was_persisted=false"
  exit 1
fi
if ! echo "$S1_OUT" | grep -q "^dir_exists=false$"; then
  echo "FAIL S1: persistence dir was created for below-threshold output"
  exit 1
fi

# Scenario 2: above threshold → persisted, file exists, byte count matches
echo
echo "=== S2: above-threshold persisted ==="
S2_ROOT="$WORK/s2"
mkdir -p "$S2_ROOT"
S2_OUT=$(go run "$DRIVER" "$S2_ROOT" single 50001)
echo "$S2_OUT"
if ! echo "$S2_OUT" | grep -q "^was_persisted=true$"; then
  echo "FAIL S2: expected was_persisted=true"
  exit 1
fi
S2_PATH=$(echo "$S2_OUT" | grep '^path=' | sed 's/^path=//')
if [[ ! -f "$S2_PATH" ]]; then
  echo "FAIL S2: file not at $S2_PATH"
  exit 1
fi
S2_BYTES=$(wc -c < "$S2_PATH" | tr -d ' ')
if [[ "$S2_BYTES" != "50001" ]]; then
  echo "FAIL S2: byte count $S2_BYTES != 50001"
  exit 1
fi

# Scenario 3: hash idempotence — same content twice → same hash prefix
echo
echo "=== S3: hash idempotence ==="
S3_ROOT="$WORK/s3"
mkdir -p "$S3_ROOT"
S3_OUT=$(go run "$DRIVER" "$S3_ROOT" twice 60000)
echo "$S3_OUT"
if ! echo "$S3_OUT" | grep -q "^hashes_match=true$"; then
  echo "FAIL S3: identical content produced different hash filenames"
  exit 1
fi

echo
echo "PASS: all three scenarios produced expected outcomes"
