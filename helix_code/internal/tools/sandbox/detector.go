package sandbox

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// procUnprivilegedUserNSPath is the kernel sysctl entry that gates
// unprivileged user namespace creation on Linux. Reading "1" means the
// in-process userns fallback is usable; "0" or absent means it is not.
const procUnprivilegedUserNSPath = "/proc/sys/kernel/unprivileged_userns_clone"

// cgroupV2ControllersPath is the canonical hierarchy file present on
// systems running cgroup-v2 (and cgroup-v2-only / unified hierarchy hosts).
// We use its existence as the cheapest reliable v2 signal — no parse, no
// mount-table inspection.
const cgroupV2ControllersPath = "/sys/fs/cgroup/cgroup.controllers"

// nonLinuxReason is the verbatim fail-closed message produced when the
// host is not Linux. F14 v1 only ships Linux primitives; macOS Seatbelt
// (sandbox-exec) and Windows Job Object integration are tracked under
// F14.5 per the spec.
const nonLinuxReason = "sandboxing only available on Linux in v1; macOS Seatbelt + Windows Job Object deferred to F14.5"

// noBackendReason is the verbatim fail-closed message produced when the
// host is Linux but neither bubblewrap nor unprivileged user namespaces
// are available. The text is part of the user-facing contract: it lists
// both remediations so a non-root user can pick whichever is feasible.
const noBackendReason = "sandboxing unavailable: install bubblewrap (apt install bubblewrap) OR enable unprivileged user namespaces (echo 1 > /proc/sys/kernel/unprivileged_userns_clone)"

// Detector probes the host for sandboxing capabilities.
//
// The detector is a thin hexagonal seam over four side-effecting OS
// primitives — `exec.LookPath`, `os.ReadFile`, `os.Stat`, and
// `runtime.GOOS`. Every primitive is injectable as a function field so
// tests can simulate every realistic host configuration without touching
// the real kernel. Production wiring uses `NewDetector` which fills in
// the real OS calls.
//
// `Detect` is total: it always returns a populated `SandboxCapabilities`.
// When no usable backend is found, the result has
// `SelectedBackend == BackendNone` AND a non-empty `UnavailableReason`
// — never one without the other (CONST-033 fail-closed contract).
type Detector struct {
	// LookPath resolves a binary name to an absolute path. Defaults to
	// `exec.LookPath`. Tests inject a fake to simulate "bwrap missing"
	// or "bwrap at /opt/bin/bwrap".
	LookPath func(name string) (string, error)

	// ReadFile reads a file's contents. Defaults to `os.ReadFile`. Tests
	// inject fakes to simulate `/proc/sys/kernel/unprivileged_userns_clone`
	// returning "0", "1", or os.ErrNotExist.
	ReadFile func(path string) ([]byte, error)

	// Stat reports file metadata. Defaults to `os.Stat`. Tests inject
	// fakes to simulate `/sys/fs/cgroup/cgroup.controllers` present or
	// missing.
	Stat func(path string) (os.FileInfo, error)

	// GOOS is the target operating system label. Defaults to
	// `runtime.GOOS`. Tests inject "darwin" / "windows" to exercise the
	// non-Linux fail-closed path.
	GOOS string
}

// NewDetector returns a Detector wired to real OS primitives:
// `exec.LookPath`, `os.ReadFile`, `os.Stat`, and `runtime.GOOS`. This is
// what production code uses; only tests should construct a Detector
// literal with custom seams.
func NewDetector() *Detector {
	return &Detector{
		LookPath: exec.LookPath,
		ReadFile: os.ReadFile,
		Stat:     os.Stat,
		GOOS:     runtime.GOOS,
	}
}

// Detect probes the host and returns a populated SandboxCapabilities.
//
// Probe order (cheap → cheap → cheap, all independent):
//  1. `GOOS` — short-circuit fail-closed for non-Linux.
//  2. `bwrap` via LookPath → BubblewrapPath.
//  3. `/proc/sys/kernel/unprivileged_userns_clone` → UnprivilegedUserNS.
//  4. `/sys/fs/cgroup/cgroup.controllers` → CGroupsV2.
//  5. Apply `SelectBackend` to determine SelectedBackend + reason.
//
// All four probes always run on Linux, even when bwrap is found, so that
// `/sandbox status` can report the full capability picture, not just the
// winning backend.
func (d *Detector) Detect() SandboxCapabilities {
	caps := SandboxCapabilities{
		GOOS: d.GOOS,
	}

	// Probe 2: bwrap binary on PATH. We always probe this so that the
	// status output can show "bwrap available but on non-Linux host".
	if path, err := d.LookPath("bwrap"); err == nil {
		caps.BubblewrapPath = path
	}

	// Probe 3: unprivileged userns clone. File-missing OR non-"1"
	// content both mean "not usable". We do not attempt the more
	// elaborate `/proc/self/uid_map` probe in v1 — that is an F14.5
	// item if real-world hosts demand it.
	if data, err := d.ReadFile(procUnprivilegedUserNSPath); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			caps.UnprivilegedUserNS = true
		}
	}

	// Probe 4: cgroup-v2 unified hierarchy. Existence of cgroup.controllers
	// is the canonical v2 signal. We do not currently use the controller
	// list (memory/cpu) — that is reserved for the bubblewrap backend's
	// resource-limit wiring in T04.
	if _, err := d.Stat(cgroupV2ControllersPath); err == nil {
		caps.CGroupsV2 = true
	} else if !errors.Is(err, os.ErrNotExist) {
		// Permission errors or other unexpected stat failures: still
		// treat as "no v2 visible to us". CGroupsV2 stays false.
		caps.CGroupsV2 = false
	}

	// Probe 5: apply selection precedence.
	caps.SelectedBackend, caps.UnavailableReason = SelectBackend(caps)

	return caps
}

// SelectBackend applies the v1 backend precedence rules:
//
//	1. GOOS != "linux" → BackendNone (with F14.5 deferral reason).
//	2. bubblewrap path present → BackendBubblewrap.
//	3. unprivileged user namespaces enabled → BackendNative.
//	4. otherwise → BackendNone (with install-hint reason).
//
// SelectBackend is a pure function: it does no I/O and no logging. It
// reads only the fields of `caps` that the rules above require. The
// returned reason is non-empty if and only if the returned kind is
// BackendNone.
//
// CONST-033 contract: when this function returns BackendNone, downstream
// code MUST surface the reason verbatim in `ErrSandboxUnavailable` so
// the user sees an actionable remediation, not a generic "unavailable"
// message.
func SelectBackend(caps SandboxCapabilities) (BackendKind, string) {
	if caps.GOOS != "linux" {
		return BackendNone, nonLinuxReason
	}
	if caps.BubblewrapPath != "" {
		return BackendBubblewrap, ""
	}
	if caps.UnprivilegedUserNS {
		return BackendNative, ""
	}
	return BackendNone, noBackendReason
}
