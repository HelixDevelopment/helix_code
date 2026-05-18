package cognee

// Round-51 §11.4 anti-bluff tests — wires real intel_gpu_top-based
// Intel Arc / Xe GPU telemetry as a sibling to round-43's nvidia-smi
// path, round-45's rocm-smi path, and round-49's ioreg path. Hermetic
// by design: every non-real-GPU subtest uses a fake `intel_gpu_top`
// script dropped into t.TempDir() with PATH redirected via t.Setenv.
// The fake binaries are NOT production code (CONST-050(A)) — they
// live in the test process's tmpdir and exercise the REAL
// exec.CommandContext path.
//
// Intel-specific coverage:
//   - TestProbeIntelGPU_NoIntelGpuTop_ReturnsSentinel
//   - TestProbeIntelGPU_ParsesIntelGpuTopJSON
//   - TestProbeIntelGPU_HandlesMultiEngine
//   - TestProbeIntelGPU_HandlesUnknownEngineKeys
//   - TestProbeIntelGPU_HandlesParseFailure
//   - TestProbeIntelGPU_HandlesTimeout
//   - TestProbeIntelGPU_HandlesKillFailure
//   - TestProbeIntelGPU_RealGPU (SKIP unless real intel_gpu_top works)
//
// Parser-level coverage:
//   - TestParseIntelGPUTopUtilization_OutOfRangeFails
//   - TestParseIntelGPUTopUtilization_MissingEngines
//   - TestParseIntelGPUTopUtilization_EmptyEngines
//   - TestParseIntelGPUTopUtilization_NoBusyKey
//
// Probe-chain coverage (extends round-49 chain to include Intel):
//   - TestGetGPUUsage_ProbeChain_FallsBackToIntel
//   - TestGetGPUUsage_ProbeChain_AllFourTried_ThenSentinel

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeIntelGpuTop creates an executable shell script at
// <dir>/intel_gpu_top whose stdout is the supplied body. Returns the
// directory (which the caller should put on PATH). Mirrors
// writeFakeNvidiaSmi (round-43) / writeFakeRocmSmi (round-45) /
// writeFakeIoreg (round-49) but kept distinct so the four probes can
// be exercised independently.
func writeFakeIntelGpuTop(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-WINDOWS-ROUND51 — fake intel_gpu_top script uses POSIX sh; Windows path covered separately if/when ported")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "intel_gpu_top")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return dir
}

// writeFakeIntelGpuTopInDir writes a fake intel_gpu_top into an
// existing directory (rather than allocating a fresh tmpdir). Used by
// the probe-chain tests that need MULTIPLE fake binaries on PATH
// simultaneously (nvidia-smi + rocm-smi + ioreg + intel_gpu_top).
func writeFakeIntelGpuTopInDir(t *testing.T, dir string, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-WINDOWS-ROUND51 — fake script uses POSIX sh")
	}
	path := filepath.Join(dir, "intel_gpu_top")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// scrubPathToOnlyIntel sets PATH to a single directory for the lifetime
// of the test. Renamed from round-43/45/49 helpers to avoid clash in
// the same package; behaviour identical.
func scrubPathToOnlyIntel(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

// ──────────────────────────────────────────────────────────────────
// Intel-specific tests
// ──────────────────────────────────────────────────────────────────

func TestProbeIntelGPU_NoIntelGpuTop_ReturnsSentinel(t *testing.T) {
	// PATH points at an empty dir → intel_gpu_top cannot be looked up
	// (typical for non-Intel-GPU hosts, the dominant case across
	// HelixCode's deployment topology).
	empty := t.TempDir()
	scrubPathToOnlyIntel(t, empty)

	got := queryIntelGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"missing intel_gpu_top MUST surface the sentinel, not a fabricated number (round-51 anti-bluff contract)")
}

func TestProbeIntelGPU_ParsesIntelGpuTopJSON(t *testing.T) {
	// Fake intel_gpu_top emits a single JSON object then blocks on
	// `tail -f /dev/null` so the production-side kill path runs. We
	// use `tail -f` (not `sleep`) per the round-43 forensic finding:
	// sandboxed harnesses short-circuit sleep, leaving the process
	// reapable instantly and defeating the kill-path coverage.
	// echo (not printf) per round-45 finding: '%' in printf format
	// strings is a hazard; JSON object never contains '%' here so
	// echo is safe and simple.
	body := `echo '{"engines": {"Render/3D/0": {"busy": 42.0}}}'
exec tail -f /dev/null`
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	got := queryIntelGPUUsage()
	assert.InDelta(t, 42.0, got, 0.0001,
		"single-engine reading MUST surface as the parsed busy value")
}

func TestProbeIntelGPU_HandlesMultiEngine(t *testing.T) {
	// Multi-engine JSON sample — typical of a real Arc-A GPU under
	// load. The parser MUST aggregate via mean per round-51
	// documented choice for parity with NVIDIA / AMD / Apple paths.
	// mean(20, 60, 40, 80) = 50.
	body := `echo '{"engines": {"Render/3D/0": {"busy": 20.0}, "Blitter/0": {"busy": 60.0}, "Video/0": {"busy": 40.0}, "VideoEnhance/0": {"busy": 80.0}}}'
exec tail -f /dev/null`
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	got := queryIntelGPUUsage()
	assert.InDelta(t, 50.0, got, 0.0001,
		"multi-engine readings MUST aggregate via mean (parity with NVIDIA / AMD / Apple paths)")
}

func TestProbeIntelGPU_HandlesUnknownEngineKeys(t *testing.T) {
	// Future Intel driver may add novel engine names. Parser MUST
	// accept them as long as the "busy" field is a number.
	// mean(10, 90) = 50.
	body := `echo '{"engines": {"NewEngine/0": {"busy": 10.0}, "AnotherFutureEngine": {"busy": 90.0}}}'
exec tail -f /dev/null`
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	got := queryIntelGPUUsage()
	assert.InDelta(t, 50.0, got, 0.0001,
		"unknown engine keys MUST be tolerated as long as 'busy' is present (Intel schema-tolerance contract)")
}

func TestProbeIntelGPU_HandlesParseFailure(t *testing.T) {
	// Garbage output — not JSON at all. Decoder MUST fail and the
	// probe MUST surface the sentinel.
	body := `echo 'not json at all even close'
exec tail -f /dev/null`
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	got := queryIntelGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"unparseable output MUST surface sentinel, never a fabricated number")
}

func TestProbeIntelGPU_HandlesTimeout(t *testing.T) {
	// Fake intel_gpu_top must exist on PATH so LookPath succeeds; the
	// script body BLOCKS without ever emitting JSON — simulates the
	// CAP_PERFMON-missing case where intel_gpu_top runs but produces
	// no output. Use `tail -f /dev/null` (not `sleep`) per the
	// round-43 forensic finding that sandboxed harnesses elide sleep.
	var tailPath string
	for _, candidate := range []string{"/usr/bin/tail", "/bin/tail"} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			tailPath = candidate
			break
		}
	}
	if tailPath == "" {
		if p, err := exec.LookPath("tail"); err == nil {
			tailPath = p
		}
	}
	if tailPath == "" {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-TIMEOUT-ROUND51 — `tail` binary not available; cannot exercise blocking timeout path hermetically")
	}

	body := "exec " + tailPath + " -f /dev/null"
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	start := time.Now()
	got := queryIntelGPUUsage()
	elapsed := time.Since(start)

	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"intel_gpu_top timeout MUST surface the sentinel")
	assert.GreaterOrEqual(t, elapsed, intelGPUTopQueryTimeout-200*time.Millisecond,
		"timeout MUST honour the documented bound (round-51 bounded shell-out)")
	assert.Less(t, elapsed, intelGPUTopQueryTimeout+2*time.Second,
		"timeout MUST fire promptly, not leak the goroutine indefinitely")
}

// TestProbeIntelGPU_HandlesKillFailure exercises the kill-path on a
// script that traps SIGTERM but does NOT trap SIGKILL. Process.Kill
// sends SIGKILL on POSIX so even a SIGTERM-ignoring child is reaped.
// The test asserts the probe still returns sentinel within the
// timeout (no goroutine leak, no zombie).
func TestProbeIntelGPU_HandlesKillFailure(t *testing.T) {
	// Script traps SIGTERM (ignored), emits no JSON, blocks on read.
	// SIGKILL from Process.Kill cannot be trapped — kernel reaps the
	// process. Use `tail -f /dev/null` for the block (not sleep).
	var tailPath string
	for _, candidate := range []string{"/usr/bin/tail", "/bin/tail"} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			tailPath = candidate
			break
		}
	}
	if tailPath == "" {
		if p, err := exec.LookPath("tail"); err == nil {
			tailPath = p
		}
	}
	if tailPath == "" {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-KILL-ROUND51 — `tail` binary not available; cannot exercise kill-path hermetically")
	}

	// trap '' TERM ignores SIGTERM forever; exec tail blocks until
	// SIGKILL arrives.
	body := "trap '' TERM\nexec " + tailPath + " -f /dev/null"
	dir := writeFakeIntelGpuTop(t, body)
	scrubPathToOnlyIntel(t, dir)

	start := time.Now()
	got := queryIntelGPUUsage()
	elapsed := time.Since(start)

	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"SIGTERM-ignoring child MUST still return sentinel via SIGKILL kill-path")
	// Should still complete within ~timeout + a small grace period for
	// kill+wait reaping. If this assertion fires, the process is leaking.
	assert.Less(t, elapsed, intelGPUTopQueryTimeout+3*time.Second,
		"kill+wait MUST complete promptly even when child ignores SIGTERM (Process.Kill sends SIGKILL on POSIX)")
}

// ──────────────────────────────────────────────────────────────────
// Parser-level unit tests (no exec involved)
// ──────────────────────────────────────────────────────────────────

func TestParseIntelGPUTopUtilization_OutOfRangeFails(t *testing.T) {
	doc := map[string]interface{}{
		"engines": map[string]interface{}{
			"Render/3D/0": map[string]interface{}{"busy": 150.0},
		},
	}
	_, ok := parseIntelGPUTopUtilization(doc)
	assert.False(t, ok, "out-of-range readings MUST fail-closed (parity with NVIDIA + AMD + Apple defensive parse)")

	doc2 := map[string]interface{}{
		"engines": map[string]interface{}{
			"Render/3D/0": map[string]interface{}{"busy": -5.0},
		},
	}
	_, ok = parseIntelGPUTopUtilization(doc2)
	assert.False(t, ok, "negative readings MUST fail-closed")
}

func TestParseIntelGPUTopUtilization_MissingEngines(t *testing.T) {
	doc := map[string]interface{}{
		"period": map[string]interface{}{"duration": 1000.0},
	}
	_, ok := parseIntelGPUTopUtilization(doc)
	assert.False(t, ok, "missing top-level 'engines' MUST yield !ok so caller surfaces sentinel")
}

func TestParseIntelGPUTopUtilization_EmptyEngines(t *testing.T) {
	doc := map[string]interface{}{
		"engines": map[string]interface{}{},
	}
	_, ok := parseIntelGPUTopUtilization(doc)
	assert.False(t, ok, "empty 'engines' object MUST yield !ok")
}

func TestParseIntelGPUTopUtilization_NoBusyKey(t *testing.T) {
	doc := map[string]interface{}{
		"engines": map[string]interface{}{
			"Render/3D/0": map[string]interface{}{"sema": 0.0, "wait": 0.0},
		},
	}
	_, ok := parseIntelGPUTopUtilization(doc)
	assert.False(t, ok, "engines without any 'busy' key MUST yield !ok (all skipped → count=0)")
}

func TestParseIntelGPUTopUtilization_SingleEngine(t *testing.T) {
	doc := map[string]interface{}{
		"engines": map[string]interface{}{
			"Render/3D/0": map[string]interface{}{"busy": 33.0},
		},
	}
	v, ok := parseIntelGPUTopUtilization(doc)
	require.True(t, ok)
	assert.InDelta(t, 33.0, v, 0.0001)
}

func TestParseIntelGPUTopUtilization_MixedValidAndNoBusy(t *testing.T) {
	// Some engines have busy, some don't — the without-busy ones are
	// skipped (tolerant), aggregation runs over the rest.
	doc := map[string]interface{}{
		"engines": map[string]interface{}{
			"Render/3D/0":      map[string]interface{}{"busy": 40.0},
			"MetadataOnly":     map[string]interface{}{"sema": 0.0},
			"Blitter/0":        map[string]interface{}{"busy": 60.0},
			"AnotherNoneBusy":  map[string]interface{}{"foo": "bar"},
		},
	}
	v, ok := parseIntelGPUTopUtilization(doc)
	require.True(t, ok)
	assert.InDelta(t, 50.0, v, 0.0001,
		"engines missing 'busy' MUST be skipped, aggregation over the remainder")
}

// ──────────────────────────────────────────────────────────────────
// Probe-chain tests — exercise runGPUUsageProbeChain end-to-end with
// Intel as the fallback vendor
// ──────────────────────────────────────────────────────────────────

func TestGetGPUUsage_ProbeChain_FallsBackToIntel(t *testing.T) {
	resetGPUUsageCacheForTest()
	// nvidia-smi + rocm-smi + ioreg missing → all three probes return
	// sentinel → chain falls through to Intel probe which returns 33.
	dir := t.TempDir()
	writeFakeIntelGpuTopInDir(t, dir, `echo '{"engines": {"Render/3D/0": {"busy": 33.0}}}'
exec tail -f /dev/null`)
	scrubPathToOnlyIntel(t, dir)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, 33.0, got, 0.0001,
		"probe chain MUST fall back to Intel when NVIDIA + AMD + Apple probes all return sentinel (round-51 chain contract)")
}

func TestGetGPUUsage_ProbeChain_AllFourTried_ThenSentinel(t *testing.T) {
	resetGPUUsageCacheForTest()
	// All four probes missing → chain MUST return sentinel and the
	// telemetry-gap log SHOULD be emitted (cannot easily intercept the
	// log channel here, but the sentinel return is the load-bearing
	// contract).
	empty := t.TempDir()
	scrubPathToOnlyIntel(t, empty)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"probe chain MUST return sentinel when no vendor probe (nvidia + amd + apple + intel) yields data (round-51 honest sentinel)")
}

// TestProbeIntelGPU_RealGPU is the integration test — only runs if a
// real intel_gpu_top binary is on the test host's PATH AND it can
// produce a JSON sample within the timeout (i.e. has CAP_PERFMON or
// is running as root, AND an Intel GPU is present). CONST-050(A)
// compliant (no fakes); SKIP-OK marker for the typical non-Intel
// dev host.
func TestProbeIntelGPU_RealGPU(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-REAL-ROUND51 — intel_gpu_top is Linux-only")
	}
	if _, err := exec.LookPath("intel_gpu_top"); err != nil {
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-REAL-ROUND51 — intel_gpu_top not found on PATH (intel-gpu-tools not installed)")
	}
	got := queryIntelGPUUsage()
	if got == GPUUsageUnavailableSentinel {
		// On hosts without CAP_PERFMON / without an Intel GPU,
		// sentinel is the correct return; we cannot distinguish
		// "intel_gpu_top works but no GPU" from "permission denied"
		// without external introspection, so SKIP rather than FAIL.
		t.Skip("SKIP-OK: #COGNEE-GPU-INTEL-REAL-ROUND51 — intel_gpu_top present but did not emit JSON within timeout (no Intel GPU OR missing CAP_PERFMON)")
	}
	assert.GreaterOrEqual(t, got, 0.0, "GPU utilisation MUST be >= 0")
	assert.LessOrEqual(t, got, 100.0, "GPU utilisation MUST be <= 100")
}
