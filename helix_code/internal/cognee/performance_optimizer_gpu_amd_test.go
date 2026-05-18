package cognee

// Round-45 §11.4 anti-bluff tests — wires real rocm-smi AMD GPU
// telemetry as a sibling to round-43's nvidia-smi path. Hermetic by
// design: every non-real-GPU subtest uses a fake `rocm-smi` script
// dropped into t.TempDir() with PATH redirected via t.Setenv. The fake
// binaries are NOT production code (CONST-050(A)) — they live in the
// test process's tmpdir and exercise the REAL exec.CommandContext path.
//
// Probe-chain coverage:
//   - TestGetGPUUsage_ProbeChain_NvidiaPreferred
//   - TestGetGPUUsage_ProbeChain_FallsBackToAMD
//   - TestGetGPUUsage_ProbeChain_BothMissing_ReturnsSentinel
//
// AMD-specific coverage:
//   - TestProbeAMDGPU_NoRocmSmi_ReturnsSentinel
//   - TestProbeAMDGPU_ParsesRocmSmiJSON
//   - TestProbeAMDGPU_HandlesMultiCard
//   - TestProbeAMDGPU_HandlesAltKeyName
//   - TestProbeAMDGPU_HandlesAltKeyName_GpuUtilization
//   - TestProbeAMDGPU_HandlesParseFailure
//   - TestProbeAMDGPU_HandlesTimeout
//   - TestParseRocmSmiUtilization_SkipsNonCardKeys
//   - TestParseRocmSmiUtilization_OutOfRangeFails
//   - TestParseRocmSmiUtilization_EmptyOutput

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

// writeFakeRocmSmi creates an executable shell script at <dir>/rocm-smi
// whose stdout is the supplied body. Returns the directory (which the
// caller should put on PATH). Mirrors writeFakeNvidiaSmi from the
// round-43 test file but kept distinct so the two probes can be
// exercised independently.
func writeFakeRocmSmi(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-AMD-WINDOWS-ROUND45 — fake rocm-smi script uses POSIX sh; Windows path covered separately if/when ported")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "rocm-smi")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return dir
}

// writeFakeNvidiaSmiInDir writes a fake nvidia-smi into an existing
// directory (rather than allocating a fresh tmpdir as the round-43
// helper does). Used by the probe-chain tests that need BOTH binaries
// on PATH simultaneously.
func writeFakeNvidiaSmiInDir(t *testing.T, dir string, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-AMD-WINDOWS-ROUND45 — fake script uses POSIX sh")
	}
	path := filepath.Join(dir, "nvidia-smi")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// writeFakeRocmSmiInDir is the rocm-smi equivalent of writeFakeNvidiaSmiInDir.
func writeFakeRocmSmiInDir(t *testing.T, dir string, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-AMD-WINDOWS-ROUND45 — fake script uses POSIX sh")
	}
	path := filepath.Join(dir, "rocm-smi")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// scrubPathToOnlyAMD sets PATH to a single directory for the lifetime
// of the test. Renamed from round-43's scrubPathToOnly to avoid clash
// in the same package; behaviour identical.
func scrubPathToOnlyAMD(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

// ──────────────────────────────────────────────────────────────────
// AMD-specific tests
// ──────────────────────────────────────────────────────────────────

func TestProbeAMDGPU_NoRocmSmi_ReturnsSentinel(t *testing.T) {
	// PATH points at an empty dir → rocm-smi cannot be looked up.
	empty := t.TempDir()
	scrubPathToOnlyAMD(t, empty)

	got := queryAMDGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"missing rocm-smi MUST surface the sentinel, not a fabricated number (round-45 anti-bluff contract)")
}

func TestProbeAMDGPU_ParsesRocmSmiJSON(t *testing.T) {
	// echo (not printf) — printf would interpret '%)' as a format
	// specifier and abort with "invalid format character".
	dir := writeFakeRocmSmi(t, `echo '{"card0": {"GPU use (%)": "42"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, 42.0, got, 0.0001,
		"single-card reading MUST surface as the parsed value")
}

func TestProbeAMDGPU_HandlesMultiCard(t *testing.T) {
	dir := writeFakeRocmSmi(t, `echo '{"card0": {"GPU use (%)": "20"}, "card1": {"GPU use (%)": "60"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, 40.0, got, 0.0001,
		"multi-card readings MUST aggregate via mean per round-45 documented choice (parity with NVIDIA path)")
}

func TestProbeAMDGPU_HandlesAltKeyName(t *testing.T) {
	dir := writeFakeRocmSmi(t, `echo '{"card0": {"GPU%": "33"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, 33.0, got, 0.0001,
		"older-ROCm 'GPU%' key MUST be honoured by the parser (round-45 key fallback)")
}

func TestProbeAMDGPU_HandlesAltKeyName_GpuUtilization(t *testing.T) {
	dir := writeFakeRocmSmi(t, `echo '{"card0": {"gpu_utilization": "77"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, 77.0, got, 0.0001,
		"packaging-variant 'gpu_utilization' key MUST be honoured by the parser (round-45 key fallback)")
}

func TestProbeAMDGPU_HandlesParseFailure(t *testing.T) {
	dir := writeFakeRocmSmi(t, "printf 'not json at all\\n'")
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"unparseable output MUST surface the sentinel, never a fabricated number")
}

func TestProbeAMDGPU_HandlesExecFailure(t *testing.T) {
	dir := writeFakeRocmSmi(t, "exit 7")
	scrubPathToOnlyAMD(t, dir)

	got := queryAMDGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"rocm-smi exec failure MUST surface the sentinel")
}

func TestProbeAMDGPU_HandlesTimeout(t *testing.T) {
	// Fake rocm-smi must exist on PATH so LookPath succeeds; the body
	// is irrelevant because we override rocmSmiCommand below. Use echo
	// (not printf) to avoid the '%)' format-specifier hazard.
	dir := writeFakeRocmSmi(t, `echo '{"card0": {"GPU use (%)": "99"}}'`)
	scrubPathToOnlyAMD(t, dir)

	// Same approach as round-43 NVIDIA timeout test: override the
	// command builder with `tail -f /dev/null` so the context-cancel
	// is what we exercise, not the fake script body. `tail -f` blocks
	// on real fd-read and cannot be elided by sandboxed harnesses.
	var tailPath string
	for _, candidate := range []string{"/usr/bin/tail", "/bin/tail"} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			tailPath = candidate
			break
		}
	}
	if tailPath == "" {
		// Last resort: try the (scrubbed) PATH — usually empty in this test.
		if p, err := exec.LookPath("tail"); err == nil {
			tailPath = p
		}
	}
	if tailPath == "" {
		t.Skip("SKIP-OK: #COGNEE-GPU-AMD-TIMEOUT-ROUND45 — `tail` binary not available; cannot exercise blocking timeout path hermetically")
	}

	origCmd := rocmSmiCommand
	t.Cleanup(func() { rocmSmiCommand = origCmd })
	rocmSmiCommand = func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, tailPath, "-f", "/dev/null")
	}

	start := time.Now()
	got := queryAMDGPUUsage()
	elapsed := time.Since(start)

	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"rocm-smi timeout MUST surface the sentinel")
	assert.GreaterOrEqual(t, elapsed, rocmSmiQueryTimeout-100*time.Millisecond,
		"timeout MUST honour the documented bound (round-45 bounded shell-out)")
	assert.Less(t, elapsed, rocmSmiQueryTimeout+1*time.Second,
		"timeout MUST fire promptly, not leak the goroutine")
}

// ──────────────────────────────────────────────────────────────────
// Parser-level unit tests (no exec involved)
// ──────────────────────────────────────────────────────────────────

func TestParseRocmSmiUtilization_SkipsNonCardKeys(t *testing.T) {
	// rocm-smi versions sometimes emit "system" alongside cardN entries.
	raw := []byte(`{"system": {"driver": "5.7.0"}, "card0": {"GPU use (%)": "50"}}`)
	v, ok := parseRocmSmiUtilization(raw)
	require.True(t, ok, "non-card keys MUST be skipped, not treated as parse failure")
	assert.InDelta(t, 50.0, v, 0.0001)
}

func TestParseRocmSmiUtilization_OutOfRangeFails(t *testing.T) {
	raw := []byte(`{"card0": {"GPU use (%)": "150"}}`)
	_, ok := parseRocmSmiUtilization(raw)
	assert.False(t, ok, "out-of-range readings MUST fail-closed (parity with NVIDIA defensive parse)")
}

func TestParseRocmSmiUtilization_EmptyOutput(t *testing.T) {
	_, ok := parseRocmSmiUtilization([]byte(""))
	assert.False(t, ok, "empty output MUST yield !ok so caller surfaces sentinel")

	_, ok = parseRocmSmiUtilization([]byte("\n  \n"))
	assert.False(t, ok, "whitespace-only output MUST yield !ok")
}

func TestParseRocmSmiUtilization_NoCardEntries(t *testing.T) {
	// JSON is well-formed but contains zero card entries.
	raw := []byte(`{"system": {"driver": "5.7.0"}}`)
	_, ok := parseRocmSmiUtilization(raw)
	assert.False(t, ok, "zero parseable cards MUST yield !ok so caller surfaces sentinel")
}

func TestParseRocmSmiUtilization_UnknownKey(t *testing.T) {
	// Card present but utilisation key absent from rocmUtilisationKeys.
	raw := []byte(`{"card0": {"completely_unknown_key": "42"}}`)
	_, ok := parseRocmSmiUtilization(raw)
	assert.False(t, ok, "card with no recognised utilisation key MUST yield !ok so caller surfaces sentinel")
}

// ──────────────────────────────────────────────────────────────────
// Probe-chain tests — exercise runGPUUsageProbeChain end-to-end
// ──────────────────────────────────────────────────────────────────

func TestGetGPUUsage_ProbeChain_NvidiaPreferred(t *testing.T) {
	resetGPUUsageCacheForTest()
	// Both probes available: nvidia returns 50, rocm returns 80.
	// Chain MUST return 50 (nvidia preferred).
	dir := t.TempDir()
	writeFakeNvidiaSmiInDir(t, dir, "printf '50\\n'")
	writeFakeRocmSmiInDir(t, dir, `echo '{"card0": {"GPU use (%)": "80"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, 50.0, got, 0.0001,
		"probe chain MUST prefer NVIDIA when both probes succeed (round-45 documented order)")
}

func TestGetGPUUsage_ProbeChain_FallsBackToAMD(t *testing.T) {
	resetGPUUsageCacheForTest()
	// nvidia-smi missing → NVIDIA probe returns sentinel → chain falls
	// through to AMD probe which returns 70.
	dir := t.TempDir()
	writeFakeRocmSmiInDir(t, dir, `echo '{"card0": {"GPU use (%)": "70"}}'`)
	scrubPathToOnlyAMD(t, dir)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, 70.0, got, 0.0001,
		"probe chain MUST fall back to AMD when NVIDIA probe returns sentinel (round-45 chain contract)")
}

func TestGetGPUUsage_ProbeChain_BothMissing_ReturnsSentinel(t *testing.T) {
	resetGPUUsageCacheForTest()
	// Both probes missing → chain MUST return sentinel.
	empty := t.TempDir()
	scrubPathToOnlyAMD(t, empty)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"probe chain MUST return sentinel when no vendor probe yields data (round-45 honest sentinel)")
}

// TestProbeAMDGPU_RealGPU is the integration test — only runs if a real
// rocm-smi binary is on the test host's PATH. CONST-050(A) compliant
// (no fakes); SKIP-OK marker for the typical CPU-only / NVIDIA-only
// dev host.
func TestProbeAMDGPU_RealGPU(t *testing.T) {
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		t.Skip("SKIP-OK: #COGNEE-GPU-AMD-REAL-ROUND45 — requires real AMD GPU + ROCm stack on the test host")
	}
	got := queryAMDGPUUsage()
	if got == GPUUsageUnavailableSentinel {
		t.Fatalf("real-AMD host returned sentinel — rocm-smi present but query failed; investigate ROCm driver state")
	}
	assert.GreaterOrEqual(t, got, 0.0, "GPU utilisation MUST be >= 0")
	assert.LessOrEqual(t, got, 100.0, "GPU utilisation MUST be <= 100")
}
