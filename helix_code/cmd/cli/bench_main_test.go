package main

import (
	"io"
	"log"
	"os"
	"testing"
)

// bench_main_test.go — speed-programme Phase 0 task P0-T02.
//
// TestMain silences the standard `log` package for the duration of the test
// binary so that `NewCLI()`'s bootstrap logging (Ollama-discovery warnings,
// Raft consensus chatter, SSH known-hosts notices) does not interleave into and
// corrupt the `go test -bench` result lines. This keeps the captured baseline
// (docs/research/speed/baseline/benchmarks-2026-05-20.txt) clean and machine-
// parseable — a prerequisite for falsifiable before/after comparison
// (CONST-035 / Rule 9). It changes NO production code: log output is a test-
// process concern only and `cmd/cli` had no prior TestMain.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
