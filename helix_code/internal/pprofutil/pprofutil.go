// Package pprofutil provides opt-in pprof capture wiring for the HelixCode CLI
// and an opt-in net/http/pprof debug mount for the HTTP server.
//
// It is measurement infrastructure for the speed programme (R4 phased plan
// P0-T01, docs/research/speed/04-phased-implementation-plan.md §3). When the
// CLI is started without the --pprof flag and HELIX_PPROF is unset, NONE of
// this code runs — there is zero behaviour change to the CLI's normal path.
//
// Constitutional anchors: Phase 0 is the measurement-baseline phase — no
// production behaviour changes beyond an opt-in profiling flag (CONST-035:
// the captured .pprof profiles are the anti-bluff proof the harness profiled
// real code paths). CONST-046: the strings here are developer-facing
// diagnostics, not localized end-user content.
package pprofutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// EnvVar is the environment variable that, when set to a non-empty directory
// path, enables CPU + heap profiling for the process — the env equivalent of
// the --pprof <dir> CLI flag. The flag takes precedence when both are set.
const EnvVar = "HELIX_PPROF"

// Capture is a live profiling session. It is created by Start and closed by
// Stop. The zero value is inert: Stop on a nil *Capture is a safe no-op, so
// callers can write `defer cap.Stop()` unconditionally.
type Capture struct {
	dir     string
	cpuFile *os.File
	started time.Time
}

// ResolveDir returns the profiling output directory: flagValue if non-empty,
// otherwise the value of the HELIX_PPROF env var (looked up via getenv).
// It returns "" when profiling is not requested. getenv is injected so unit
// tests do not depend on the real process environment.
func ResolveDir(flagValue string, getenv func(string) string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	return strings.TrimSpace(getenv(EnvVar))
}

// Start begins CPU profiling and returns a *Capture. dir is created if it does
// not exist. The CPU profile is written to <dir>/cpu.pprof. If dir is empty
// Start returns (nil, nil) — profiling was not requested and the caller's
// deferred Stop is a safe no-op.
//
// label is an optional run identifier used to namespace the profile files
// (e.g. "S1"); when empty the bare names cpu.pprof / heap.pprof are used.
func Start(dir, label string) (*Capture, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("pprofutil: create dir %s: %w", dir, err)
	}
	cpuPath := filepath.Join(dir, profileName(label, "cpu"))
	f, err := os.Create(cpuPath)
	if err != nil {
		return nil, fmt.Errorf("pprofutil: create %s: %w", cpuPath, err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("pprofutil: start CPU profile: %w", err)
	}
	return &Capture{dir: dir, cpuFile: f, started: time.Now()}, nil
}

// Stop ends CPU profiling and writes a heap profile to <dir>/heap.pprof. It is
// safe to call on a nil *Capture (no-op). It returns the elapsed profiling
// duration and the heap-profile path, or an error if writing failed.
func (c *Capture) Stop(label string) (time.Duration, string, error) {
	if c == nil {
		return 0, "", nil
	}
	pprof.StopCPUProfile()
	elapsed := time.Since(c.started)
	if c.cpuFile != nil {
		if err := c.cpuFile.Close(); err != nil {
			return elapsed, "", fmt.Errorf("pprofutil: close CPU profile: %w", err)
		}
	}
	heapPath := filepath.Join(c.dir, profileName(label, "heap"))
	hf, err := os.Create(heapPath)
	if err != nil {
		return elapsed, "", fmt.Errorf("pprofutil: create %s: %w", heapPath, err)
	}
	defer hf.Close()
	runtime.GC() // force an up-to-date heap snapshot before writing
	if err := pprof.WriteHeapProfile(hf); err != nil {
		return elapsed, heapPath, fmt.Errorf("pprofutil: write heap profile: %w", err)
	}
	return elapsed, heapPath, nil
}

// CPUPath returns the path of the CPU profile being written by this Capture.
func (c *Capture) CPUPath() string {
	if c == nil || c.cpuFile == nil {
		return ""
	}
	return c.cpuFile.Name()
}

// profileName returns "<label>-<kind>.pprof" when label is non-empty, else
// "<kind>.pprof".
func profileName(label, kind string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return kind + ".pprof"
	}
	return label + "-" + kind + ".pprof"
}
