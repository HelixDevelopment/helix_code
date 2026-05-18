package cognee

// Round-49 §11.4 anti-bluff tests — wires real ioreg-based Apple
// Silicon GPU telemetry as a sibling to round-43's nvidia-smi path
// and round-45's rocm-smi path. Hermetic by design: every non-real-GPU
// subtest uses a fake `ioreg` script dropped into t.TempDir() with
// PATH redirected via t.Setenv. The fake binaries are NOT production
// code (CONST-050(A)) — they live in the test process's tmpdir and
// exercise the REAL exec.CommandContext path.
//
// Apple-specific coverage:
//   - TestProbeAppleGPU_NoIoreg_ReturnsSentinel
//   - TestProbeAppleGPU_ParsesIoregOutput
//   - TestProbeAppleGPU_HandlesMultipleGPUs
//   - TestProbeAppleGPU_HandlesWhitespaceVariants
//   - TestProbeAppleGPU_HandlesNoMatch
//   - TestProbeAppleGPU_HandlesParseFailure
//   - TestProbeAppleGPU_HandlesExecFailure
//   - TestProbeAppleGPU_HandlesTimeout
//   - TestProbeAppleGPU_RealGPU (SKIP on non-macOS hosts)
//
// Parser-level coverage:
//   - TestParseAppleIoregUtilization_OutOfRangeFails
//   - TestParseAppleIoregUtilization_EmptyOutput
//   - TestParseAppleIoregUtilization_NoMatches
//
// Probe-chain coverage (extends round-45 chain to include Apple):
//   - TestGetGPUUsage_ProbeChain_FallsBackToApple
//   - TestGetGPUUsage_ProbeChain_AllVendorsTried_ThenSentinel

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

// writeFakeIoreg creates an executable shell script at <dir>/ioreg
// whose stdout is the supplied body. Returns the directory (which the
// caller should put on PATH). Mirrors writeFakeNvidiaSmi (round-43) /
// writeFakeRocmSmi (round-45) but kept distinct so the three probes
// can be exercised independently.
func writeFakeIoreg(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-WINDOWS-ROUND49 — fake ioreg script uses POSIX sh; Windows path covered separately if/when ported")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ioreg")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return dir
}

// writeFakeIoregInDir writes a fake ioreg into an existing directory
// (rather than allocating a fresh tmpdir). Used by the probe-chain
// tests that need MULTIPLE fake binaries on PATH simultaneously
// (nvidia-smi + rocm-smi + ioreg).
func writeFakeIoregInDir(t *testing.T, dir string, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-WINDOWS-ROUND49 — fake script uses POSIX sh")
	}
	path := filepath.Join(dir, "ioreg")
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// scrubPathToOnlyApple sets PATH to a single directory for the lifetime
// of the test. Renamed from round-43/45 helpers to avoid clash in the
// same package; behaviour identical.
func scrubPathToOnlyApple(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

// ──────────────────────────────────────────────────────────────────
// Apple-specific tests
// ──────────────────────────────────────────────────────────────────

func TestProbeAppleGPU_NoIoreg_ReturnsSentinel(t *testing.T) {
	// PATH points at an empty dir → ioreg cannot be looked up
	// (typical Linux/Windows host, which is the dominant case for
	// HelixCode's deployment topology).
	empty := t.TempDir()
	scrubPathToOnlyApple(t, empty)

	got := queryAppleGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"missing ioreg MUST surface the sentinel, not a fabricated number (round-49 anti-bluff contract)")
}

func TestProbeAppleGPU_ParsesIoregOutput(t *testing.T) {
	// echo (not printf) — printf would interpret '%"' as a format
	// specifier and abort. Sample ioreg snippet with the canonical
	// "Device Utilization %" line.
	body := `echo '    "PerformanceStatistics" = {"Device Utilization %" = 42}'`
	dir := writeFakeIoreg(t, body)
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	assert.InDelta(t, 42.0, got, 0.0001,
		"single-GPU reading MUST surface as the parsed value")
}

func TestProbeAppleGPU_HandlesMultipleGPUs(t *testing.T) {
	// Two "Device Utilization %" lines (rare on real macOS — only on
	// Intel Macs with both integrated + discrete AGX-aware drivers —
	// but the parser MUST aggregate via mean per round-49 documented
	// choice for parity with NVIDIA + AMD paths). Use two echo
	// statements rather than a heredoc — heredocs are fragile inside
	// the fake-script-body string when interpreted via sh -c chain.
	body := `echo '    "Device Utilization %" = 20'
echo '    "Device Utilization %" = 60'`
	dir := writeFakeIoreg(t, body)
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	assert.InDelta(t, 40.0, got, 0.0001,
		"multi-GPU readings MUST aggregate via mean (parity with NVIDIA / AMD path)")
}

func TestProbeAppleGPU_HandlesWhitespaceVariants(t *testing.T) {
	// Real ioreg output has varied indentation depending on tree depth
	// (some lines deeply indented, some flush-left). The regex MUST
	// tolerate every reasonable whitespace permutation around the '='.
	// Avoid printf (round-45 finding: '%' in printf format strings is
	// a hazard) — use echo throughout with embedded literal tab
	// characters (\t in the Go raw-string is a literal tab byte).
	body := "echo '\"Device Utilization %\"=33'\n" +
		"echo '          \"Device Utilization %\"   =   77'\n" +
		"echo '\t\"Device Utilization %\"\t=\t10'"
	dir := writeFakeIoreg(t, body)
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	// mean(33, 77, 10) = 40.0
	assert.InDelta(t, 40.0, got, 0.0001,
		"whitespace variants (no spaces / many spaces / tabs) MUST all be parsed correctly")
}

func TestProbeAppleGPU_HandlesNoMatch(t *testing.T) {
	// Intel Mac with no Apple Silicon GPU and no IOAccelerator-compatible
	// integrated GPU — ioreg returns IORegistry output WITHOUT any
	// "Device Utilization %" lines. This is the dominant case on Intel
	// Macs without a discrete GPU; MUST surface sentinel cleanly.
	// Use multiple echo statements (heredocs are fragile inside fake
	// script bodies).
	body := `echo '+-o IOPlatformExpertDevice  <class IOPlatformExpertDevice>'
echo '  | {'
echo '  |   "compatible" = <"MacBookPro15,4">'
echo '  |   "model" = <"MacBookPro15,4">'
echo '  | }'`
	dir := writeFakeIoreg(t, body)
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"ioreg output without 'Device Utilization %' MUST surface sentinel (Intel Mac no-GPU edge case)")
}

func TestProbeAppleGPU_HandlesParseFailure(t *testing.T) {
	// Garbage value after the '=' — regex requires \d+ so this will
	// simply not match (rather than match and fail strconv). Either
	// way the contract is sentinel-on-failure.
	body := `echo '"Device Utilization %" = not-a-number'`
	dir := writeFakeIoreg(t, body)
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"unparseable value MUST surface sentinel, never a fabricated number")
}

func TestProbeAppleGPU_HandlesExecFailure(t *testing.T) {
	dir := writeFakeIoreg(t, "exit 7")
	scrubPathToOnlyApple(t, dir)

	got := queryAppleGPUUsage()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"ioreg exec failure MUST surface the sentinel")
}

func TestProbeAppleGPU_HandlesTimeout(t *testing.T) {
	// Fake ioreg must exist on PATH so LookPath succeeds; the body is
	// irrelevant because we override appleIoregCommand below. Use echo
	// (not printf) to avoid the format-specifier hazard.
	dir := writeFakeIoreg(t, `echo '"Device Utilization %" = 99'`)
	scrubPathToOnlyApple(t, dir)

	// Same approach as round-43 NVIDIA / round-45 AMD timeout tests:
	// override the command builder with `tail -f /dev/null` so the
	// context-cancel is what we exercise, not the fake script body.
	// `tail -f` blocks on real fd-read and cannot be elided by
	// sandboxed harnesses (round-43 forensic finding — `sleep` was
	// short-circuited in some sandboxes).
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
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-TIMEOUT-ROUND49 — `tail` binary not available; cannot exercise blocking timeout path hermetically")
	}

	origCmd := appleIoregCommand
	t.Cleanup(func() { appleIoregCommand = origCmd })
	appleIoregCommand = func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, tailPath, "-f", "/dev/null")
	}

	start := time.Now()
	got := queryAppleGPUUsage()
	elapsed := time.Since(start)

	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"ioreg timeout MUST surface the sentinel")
	assert.GreaterOrEqual(t, elapsed, appleIoregQueryTimeout-100*time.Millisecond,
		"timeout MUST honour the documented bound (round-49 bounded shell-out)")
	assert.Less(t, elapsed, appleIoregQueryTimeout+1*time.Second,
		"timeout MUST fire promptly, not leak the goroutine")
}

// ──────────────────────────────────────────────────────────────────
// Parser-level unit tests (no exec involved)
// ──────────────────────────────────────────────────────────────────

func TestParseAppleIoregUtilization_OutOfRangeFails(t *testing.T) {
	raw := []byte(`"Device Utilization %" = 150`)
	_, ok := parseAppleIoregUtilization(raw)
	assert.False(t, ok, "out-of-range readings MUST fail-closed (parity with NVIDIA + AMD defensive parse)")
}

func TestParseAppleIoregUtilization_EmptyOutput(t *testing.T) {
	_, ok := parseAppleIoregUtilization([]byte(""))
	assert.False(t, ok, "empty output MUST yield !ok so caller surfaces sentinel")

	_, ok = parseAppleIoregUtilization([]byte("\n  \n"))
	assert.False(t, ok, "whitespace-only output MUST yield !ok")
}

func TestParseAppleIoregUtilization_NoMatches(t *testing.T) {
	// Plausible ioreg output that lacks any "Device Utilization %" line.
	raw := []byte(`+-o IOPlatformExpertDevice
  "model" = <"MacBookPro15,4">
  "compatible" = <"MacBookPro15,4">`)
	_, ok := parseAppleIoregUtilization(raw)
	assert.False(t, ok, "output with no 'Device Utilization %' matches MUST yield !ok")
}

func TestParseAppleIoregUtilization_SingleMatch(t *testing.T) {
	raw := []byte(`    "Device Utilization %" = 55`)
	v, ok := parseAppleIoregUtilization(raw)
	require.True(t, ok)
	assert.InDelta(t, 55.0, v, 0.0001)
}

// ──────────────────────────────────────────────────────────────────
// Probe-chain tests — exercise runGPUUsageProbeChain end-to-end with
// Apple as the fallback vendor
// ──────────────────────────────────────────────────────────────────

func TestGetGPUUsage_ProbeChain_FallsBackToApple(t *testing.T) {
	resetGPUUsageCacheForTest()
	// nvidia-smi + rocm-smi missing → both probes return sentinel →
	// chain falls through to Apple probe which returns 50.
	dir := t.TempDir()
	writeFakeIoregInDir(t, dir, `echo '"Device Utilization %" = 50'`)
	scrubPathToOnlyApple(t, dir)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, 50.0, got, 0.0001,
		"probe chain MUST fall back to Apple when NVIDIA + AMD probes both return sentinel (round-49 chain contract)")
}

func TestGetGPUUsage_ProbeChain_AllVendorsTried_ThenSentinel(t *testing.T) {
	resetGPUUsageCacheForTest()
	// All three probes missing → chain MUST return sentinel and the
	// telemetry-gap log SHOULD be emitted (cannot easily intercept the
	// log channel here, but the sentinel return is the load-bearing
	// contract).
	empty := t.TempDir()
	scrubPathToOnlyApple(t, empty)

	got := runGPUUsageProbeChain()
	assert.InDelta(t, GPUUsageUnavailableSentinel, got, 0.0001,
		"probe chain MUST return sentinel when no vendor probe (nvidia + amd + apple) yields data (round-49 honest sentinel)")
}

// TestProbeAppleGPU_RealGPU is the integration test — only runs if a
// real ioreg binary is on the test host's PATH AND we're on darwin.
// CONST-050(A) compliant (no fakes); SKIP-OK marker for the typical
// Linux dev host.
func TestProbeAppleGPU_RealGPU(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-REAL-ROUND49 — ioreg + IOAccelerator are macOS-only")
	}
	if _, err := exec.LookPath("ioreg"); err != nil {
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-REAL-ROUND49 — ioreg not found on PATH despite darwin host (unusual)")
	}
	got := queryAppleGPUUsage()
	if got == GPUUsageUnavailableSentinel {
		// On Intel Macs without an AGX-aware integrated GPU the
		// sentinel is the correct return; we cannot distinguish "no
		// such Mac" from "probe broken" without external introspection,
		// so SKIP rather than FAIL.
		t.Skip("SKIP-OK: #COGNEE-GPU-APPLE-REAL-ROUND49 — ioreg present but no IOAccelerator device emits 'Device Utilization %' (Intel Mac without AGX-aware GPU)")
	}
	assert.GreaterOrEqual(t, got, 0.0, "GPU utilisation MUST be >= 0")
	assert.LessOrEqual(t, got, 100.0, "GPU utilisation MUST be <= 100")
}
