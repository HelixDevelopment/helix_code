// Test-only export shims for cross-package telemetry tests.
//
// This file is gated by the `testing_export` build tag so it is NEVER
// compiled into a production binary. Tests in other packages (e.g.
// internal/tools) that need to capture stdout exporter output build with
// `-tags=testing_export` to reach the package-private stdoutWriter.
//
// In-package telemetry tests use the package-private captureStdout helper
// directly (see provider_test.go) and do NOT need this build tag.

//go:build testing_export

package telemetry

import "io"

// SetStdoutWriterForTest swaps the package-private stdoutWriter with w.
// Returns the previous writer so callers can restore it. Test-only:
// production code MUST NEVER call this (the build tag enforces this).
func SetStdoutWriterForTest(w io.Writer) io.Writer {
	old := stdoutWriter
	stdoutWriter = w
	return old
}
