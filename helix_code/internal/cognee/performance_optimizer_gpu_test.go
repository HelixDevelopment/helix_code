package cognee

// Round-43 §11.4 anti-bluff tests — wires real nvidia-smi GPU telemetry
// to close GPUUsageUnavailableSentinel added by round 33. Hermetic by
// design: every non-real-GPU subtest uses a fake `nvidia-smi` script
// dropped into t.TempDir() with PATH redirected via t.Setenv. The fake
// binaries are NOT production code (CONST-050(A)) — they live in the
// test process's tmpdir and exercise the REAL exec.CommandContext path.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeNvidiaSmi creates an executable shell script at <dir>/nvidia-smi
// whose stdout is the supplied body. Returns the directory (which the
// caller should put on PATH).
func writeFakeNvidiaSmi(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-WINDOWS-ROUND43 — fake nvidia-smi script uses POSIX sh; Windows path covered separately if/when ported")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "nvidia-smi")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return dir
}

// scrubPathToOnly sets PATH to a single directory for the lifetime of
// the test. t.Setenv restores the prior value at teardown.
func scrubPathToOnly(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

func TestGetGPUUsage_NoNvidiaSmi_ReturnsSentinel(t *testing.T) {
	resetGPUUsageCacheForTest()
	// PATH points at an empty dir → nvidia-smi cannot be looked up.
	empty := t.TempDir()
	scrubPathToOnly(t, empty)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"missing nvidia-smi MUST surface the sentinel, not a fabricated number (round-43 anti-bluff contract)")
}

func TestGetGPUUsage_ParsesNvidiaSmiOutput(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf '42\\n'")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, 42.0, got, 0.0001, "single-GPU reading must surface as the parsed value")
}

func TestGetGPUUsage_HandlesMultiGPU_MeanAggregation(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf '30\\n60\\n'")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, 45.0, got, 0.0001,
		"multi-GPU readings MUST aggregate via mean per round-43 documented choice (representative system GPU%)")
}

func TestGetGPUUsage_HandlesMultiGPU_ThreeWay(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf '10\\n20\\n30\\n'")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, 20.0, got, 0.0001, "3-GPU mean must equal arithmetic mean")
}

func TestGetGPUUsage_HandlesParseFailure_GarbageOutput(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf 'not a number\\n'")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"unparseable output MUST surface the sentinel, never a fabricated number")
}

func TestGetGPUUsage_HandlesParseFailure_OutOfRange(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf '150\\n'")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"out-of-range driver output MUST surface the sentinel (round-43 defensive parse)")
}

func TestGetGPUUsage_HandlesExecFailure(t *testing.T) {
	resetGPUUsageCacheForTest()
	// Script exits non-zero → exec.Cmd.Output() returns *ExitError.
	dir := writeFakeNvidiaSmi(t, "exit 7")
	scrubPathToOnly(t, dir)

	got := queryNvidiaGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"nvidia-smi exec failure MUST surface the sentinel")
}

func TestGetGPUUsage_HandlesTimeout(t *testing.T) {
	resetGPUUsageCacheForTest()
	// Set up a fake nvidia-smi on PATH so LookPath succeeds (the
	// LookPath gate runs BEFORE nvidiaSmiCommand is invoked). The
	// fake's contents don't matter — we override the command builder
	// below to ensure exec-level timeout is what we exercise, not the
	// fake's body.
	dir := writeFakeNvidiaSmi(t, "printf '99\\n'")
	scrubPathToOnly(t, dir)

	// Override nvidiaSmiCommand to a blocking process that the ctx
	// cancel WILL kill. Chosen over `sleep N` because some sandboxed
	// test harnesses short-circuit sleep, defeating the timeout
	// assertion. `tail -f /dev/null` blocks on a real fd-read and
	// cannot be elided. We use absolute paths to bypass the scrubbed
	// PATH (which only contains the fake nvidia-smi dir).
	tailPath, err := exec.LookPath("tail")
	if err != nil {
		// Re-look in canonical locations because PATH is scrubbed for
		// this test.
		for _, candidate := range []string{"/usr/bin/tail", "/bin/tail"} {
			if _, statErr := os.Stat(candidate); statErr == nil {
				tailPath = candidate
				err = nil
				break
			}
		}
	}
	if err != nil || tailPath == "" {
		t.Skip("SKIP-OK: #COGNEE-GPU-TIMEOUT-ROUND43 — `tail` binary not available; cannot exercise blocking timeout path hermetically")
	}
	origCmd := nvidiaSmiCommand
	t.Cleanup(func() { nvidiaSmiCommand = origCmd })
	nvidiaSmiCommand = func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, tailPath, "-f", "/dev/null")
	}

	start := time.Now()
	got := queryNvidiaGPUUsage()
	elapsed := time.Since(start)

	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"nvidia-smi timeout MUST surface the sentinel")
	assert.GreaterOrEqual(t, elapsed, nvidiaSmiQueryTimeout-100*time.Millisecond,
		"timeout MUST honour the documented bound (round-43 bounded shell-out)")
	assert.Less(t, elapsed, nvidiaSmiQueryTimeout+1*time.Second,
		"timeout MUST fire promptly, not leak the goroutine")
}

func TestGetGPUUsage_CacheReusesRecentSuccess(t *testing.T) {
	resetGPUUsageCacheForTest()
	// First call: fake returns 42.
	dir := writeFakeNvidiaSmi(t, "printf '42\\n'")
	scrubPathToOnly(t, dir)

	first := queryNvidiaGPUUsage()
	require.InDelta(t, 42.0, first, 0.0001)

	// Swap PATH to an EMPTY dir mid-test. Cache should still serve 42
	// because nvidiaSmiCacheTTL (1s) has not elapsed.
	empty := t.TempDir()
	scrubPathToOnly(t, empty)

	second := queryNvidiaGPUUsage()
	assert.InDelta(t, 42.0, second, 0.0001,
		"cache MUST elide the second nvidia-smi call within TTL (round-43 cache contract)")
}

func TestGetGPUUsage_CacheExpires(t *testing.T) {
	resetGPUUsageCacheForTest()
	dir := writeFakeNvidiaSmi(t, "printf '42\\n'")
	scrubPathToOnly(t, dir)

	first := queryNvidiaGPUUsage()
	require.InDelta(t, 42.0, first, 0.0001)

	// Backdate the cache timestamp to simulate TTL expiry. This avoids
	// relying on wall-clock sleep (some sandboxed harnesses elide
	// sleeps, defeating the assertion).
	gpuUsageCache.mu.Lock()
	gpuUsageCache.taken = time.Now().Add(-2 * nvidiaSmiCacheTTL)
	gpuUsageCache.mu.Unlock()

	// Swap PATH to a directory with no nvidia-smi — fresh query MUST
	// hit LookPath which now fails, surfacing the sentinel.
	empty := t.TempDir()
	scrubPathToOnly(t, empty)

	second := queryNvidiaGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, second, 0.0001,
		"after TTL expiry cache MUST NOT serve stale value; sentinel surfaces when LookPath fails")
}

func TestGetGPUUsage_SentinelNotPoisoningCache(t *testing.T) {
	resetGPUUsageCacheForTest()
	// First call: LookPath fails → sentinel, MUST NOT poison cache.
	empty := t.TempDir()
	scrubPathToOnly(t, empty)
	first := queryNvidiaGPUUsage()
	require.InDelta(t, GPUUsageUnavailableSentinel, first, 0.0001)

	// Swap PATH to a working fake. Cache should be empty, real shell-out
	// happens, 73 surfaces.
	dir := writeFakeNvidiaSmi(t, "printf '73\\n'")
	scrubPathToOnly(t, dir)
	second := queryNvidiaGPUUsage()
	assert.InDelta(t, 73.0, second, 0.0001,
		"sentinel returns MUST NOT poison the cache (round-43 cache contract: only success populates)")
}

func TestParseNvidiaSmiUtilization_HandlesBlankLines(t *testing.T) {
	v, ok := parseNvidiaSmiUtilization([]byte("\n42\n\n58\n\n"))
	require.True(t, ok)
	assert.InDelta(t, 50.0, v, 0.0001, "blank lines must be skipped, mean computed across non-blank")
}

func TestParseNvidiaSmiUtilization_EmptyOutput(t *testing.T) {
	_, ok := parseNvidiaSmiUtilization([]byte(""))
	assert.False(t, ok, "empty output MUST yield !ok so caller surfaces sentinel")
}

func TestParseNvidiaSmiUtilization_AllWhitespace(t *testing.T) {
	_, ok := parseNvidiaSmiUtilization([]byte("\n\n   \n"))
	assert.False(t, ok, "whitespace-only output MUST yield !ok")
}

// TestGetGPUUsage_RealGPU is the integration test — it only runs if
// nvidia-smi is actually on the test host's PATH. CONST-050(A) compliant
// (no fakes); SKIP-OK marker for the typical CPU-only dev host.
func TestGetGPUUsage_RealGPU(t *testing.T) {
	resetGPUUsageCacheForTest()
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		t.Skip("SKIP-OK: #COGNEE-GPU-REAL-ROUND43 — requires real NVIDIA GPU + driver on the test host")
	}
	got := queryNvidiaGPUUsage()
	if got == GPUUsageUnavailableSentinel {
		t.Fatalf("real-GPU host returned sentinel — nvidia-smi present but query failed; investigate driver state")
	}
	assert.GreaterOrEqual(t, got, 0.0, "GPU utilisation MUST be >= 0")
	assert.LessOrEqual(t, got, 100.0, "GPU utilisation MUST be <= 100")
}
