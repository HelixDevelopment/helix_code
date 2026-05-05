package sandbox_test

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/sandbox"
)

// fakeFileInfo is a minimal os.FileInfo implementation used by Stat seams.
type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// makeDetector wires a Detector with the given mock seams.
func makeDetector(
	goos string,
	bwrapPath string,
	bwrapErr error,
	procFile map[string][]byte,
	procFileErr map[string]error,
	statOK map[string]bool,
) *sandbox.Detector {
	return &sandbox.Detector{
		GOOS: goos,
		LookPath: func(name string) (string, error) {
			if name == "bwrap" {
				return bwrapPath, bwrapErr
			}
			return "", errors.New("unknown binary")
		},
		ReadFile: func(path string) ([]byte, error) {
			if err, ok := procFileErr[path]; ok {
				return nil, err
			}
			if data, ok := procFile[path]; ok {
				return data, nil
			}
			return nil, os.ErrNotExist
		},
		Stat: func(path string) (os.FileInfo, error) {
			if ok, present := statOK[path]; present && ok {
				return fakeFileInfo{name: path}, nil
			}
			return nil, os.ErrNotExist
		},
	}
}

// TestDetector_Linux_BwrapPresent_SelectsBubblewrap verifies that when bwrap
// is on PATH, the bubblewrap backend is selected regardless of userns/cgroups.
func TestDetector_Linux_BwrapPresent_SelectsBubblewrap(t *testing.T) {
	d := makeDetector(
		"linux",
		"/usr/bin/bwrap",
		nil,
		map[string][]byte{"/proc/sys/kernel/unprivileged_userns_clone": []byte("0\n")},
		nil,
		map[string]bool{"/sys/fs/cgroup/cgroup.controllers": true},
	)
	caps := d.Detect()

	assert.Equal(t, sandbox.BackendBubblewrap, caps.SelectedBackend)
	assert.Equal(t, "/usr/bin/bwrap", caps.BubblewrapPath)
	assert.Empty(t, caps.UnavailableReason, "bubblewrap selection must not carry a reason")
}

// TestDetector_Linux_NoBwrap_UnprivUserNSEnabled_SelectsNative verifies
// the native fallback when bwrap is absent but userns is enabled.
func TestDetector_Linux_NoBwrap_UnprivUserNSEnabled_SelectsNative(t *testing.T) {
	d := makeDetector(
		"linux",
		"",
		errors.New("not found"),
		map[string][]byte{"/proc/sys/kernel/unprivileged_userns_clone": []byte("1\n")},
		nil,
		map[string]bool{"/sys/fs/cgroup/cgroup.controllers": true},
	)
	caps := d.Detect()

	assert.Equal(t, sandbox.BackendNative, caps.SelectedBackend)
	assert.Empty(t, caps.BubblewrapPath)
	assert.True(t, caps.UnprivilegedUserNS)
	assert.Empty(t, caps.UnavailableReason)
}

// TestDetector_Linux_NoBwrap_UnprivUserNSDisabled_FailsClosed verifies
// the fail-closed path when neither bwrap nor userns is available.
func TestDetector_Linux_NoBwrap_UnprivUserNSDisabled_FailsClosed(t *testing.T) {
	d := makeDetector(
		"linux",
		"",
		errors.New("not found"),
		map[string][]byte{"/proc/sys/kernel/unprivileged_userns_clone": []byte("0")},
		nil,
		map[string]bool{},
	)
	caps := d.Detect()

	assert.Equal(t, sandbox.BackendNone, caps.SelectedBackend)
	assert.False(t, caps.UnprivilegedUserNS)
	assert.NotEmpty(t, caps.UnavailableReason)
	assert.Contains(t, caps.UnavailableReason, "install bubblewrap",
		"reason must mention install hint for bubblewrap")
	assert.Contains(t, caps.UnavailableReason, "user namespaces",
		"reason must mention user namespaces hint")
}

// TestDetector_Linux_NoBwrap_NoProcFile_FailsClosed verifies that when
// /proc/sys/kernel/unprivileged_userns_clone is missing entirely, we treat
// userns as disabled and fail closed.
func TestDetector_Linux_NoBwrap_NoProcFile_FailsClosed(t *testing.T) {
	d := makeDetector(
		"linux",
		"",
		errors.New("not found"),
		nil,
		map[string]error{"/proc/sys/kernel/unprivileged_userns_clone": os.ErrNotExist},
		map[string]bool{},
	)
	caps := d.Detect()

	assert.Equal(t, sandbox.BackendNone, caps.SelectedBackend)
	assert.False(t, caps.UnprivilegedUserNS)
	assert.NotEmpty(t, caps.UnavailableReason)
}

// TestDetector_NonLinux_FailsClosed verifies macOS / Windows are fail-closed
// in v1 with a reason that mentions F14.5 deferral.
func TestDetector_NonLinux_FailsClosed(t *testing.T) {
	for _, goos := range []string{"darwin", "windows"} {
		goos := goos
		t.Run(goos, func(t *testing.T) {
			d := makeDetector(
				goos,
				"/usr/bin/bwrap", // even if bwrap somehow resolves, non-linux must be none
				nil,
				nil,
				nil,
				nil,
			)
			caps := d.Detect()

			assert.Equal(t, sandbox.BackendNone, caps.SelectedBackend)
			assert.NotEmpty(t, caps.UnavailableReason)
			assert.Contains(t, caps.UnavailableReason, "Linux",
				"reason must mention Linux-only restriction")
			assert.Contains(t, caps.UnavailableReason, "F14.5",
				"reason must mention F14.5 deferral")
		})
	}
}

// TestDetector_CGroupsV2_DetectedWhenStatSucceeds verifies cgroups-v2 is
// reported true when the cgroup.controllers file exists.
func TestDetector_CGroupsV2_DetectedWhenStatSucceeds(t *testing.T) {
	d := makeDetector(
		"linux",
		"/usr/bin/bwrap",
		nil,
		nil,
		nil,
		map[string]bool{"/sys/fs/cgroup/cgroup.controllers": true},
	)
	caps := d.Detect()

	assert.True(t, caps.CGroupsV2)
}

// TestDetector_CGroupsV2_FalseWhenStatFails verifies cgroups-v2 is reported
// false when the cgroup.controllers file is missing.
func TestDetector_CGroupsV2_FalseWhenStatFails(t *testing.T) {
	d := makeDetector(
		"linux",
		"/usr/bin/bwrap",
		nil,
		nil,
		nil,
		map[string]bool{},
	)
	caps := d.Detect()

	assert.False(t, caps.CGroupsV2)
}

// TestDetector_PopulatesGOOS verifies the GOOS field is set in the result.
func TestDetector_PopulatesGOOS(t *testing.T) {
	d := makeDetector("linux", "/usr/bin/bwrap", nil, nil, nil, nil)
	caps := d.Detect()
	assert.Equal(t, "linux", caps.GOOS)
}

// TestDetector_BubblewrapPath_PopulatedWhenFound verifies BubblewrapPath
// is set to whatever LookPath returned.
func TestDetector_BubblewrapPath_PopulatedWhenFound(t *testing.T) {
	d := makeDetector("linux", "/opt/bin/bwrap", nil, nil, nil, nil)
	caps := d.Detect()
	assert.Equal(t, "/opt/bin/bwrap", caps.BubblewrapPath)
}

// TestSelectBackend_PrecedenceTable exercises the four selection rules.
func TestSelectBackend_PrecedenceTable(t *testing.T) {
	cases := []struct {
		name        string
		caps        sandbox.SandboxCapabilities
		wantKind    sandbox.BackendKind
		wantReasonContains string
	}{
		{
			name: "non_linux_fails_closed",
			caps: sandbox.SandboxCapabilities{
				GOOS:           "darwin",
				BubblewrapPath: "/usr/bin/bwrap",
			},
			wantKind:           sandbox.BackendNone,
			wantReasonContains: "Linux",
		},
		{
			name: "linux_bwrap_wins",
			caps: sandbox.SandboxCapabilities{
				GOOS:               "linux",
				BubblewrapPath:     "/usr/bin/bwrap",
				UnprivilegedUserNS: false,
			},
			wantKind:           sandbox.BackendBubblewrap,
			wantReasonContains: "",
		},
		{
			name: "linux_no_bwrap_userns_native",
			caps: sandbox.SandboxCapabilities{
				GOOS:               "linux",
				UnprivilegedUserNS: true,
			},
			wantKind:           sandbox.BackendNative,
			wantReasonContains: "",
		},
		{
			name: "linux_no_bwrap_no_userns_none",
			caps: sandbox.SandboxCapabilities{
				GOOS: "linux",
			},
			wantKind:           sandbox.BackendNone,
			wantReasonContains: "install bubblewrap",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotKind, gotReason := sandbox.SelectBackend(tc.caps)
			assert.Equal(t, tc.wantKind, gotKind)
			if tc.wantReasonContains == "" {
				assert.Empty(t, gotReason, "non-None backends must have empty reason")
			} else {
				assert.Contains(t, gotReason, tc.wantReasonContains)
			}
		})
	}
}

// TestNewDetector_RealHost runs the unmocked detector against the actual
// host. It does not assert specific capabilities (those depend on the test
// host), only that Detect() doesn't panic and GOOS matches runtime.GOOS.
func TestNewDetector_RealHost(t *testing.T) {
	d := sandbox.NewDetector()
	require.NotNil(t, d)
	caps := d.Detect()
	assert.Equal(t, runtime.GOOS, caps.GOOS)
	// SelectedBackend must be one of the three known kinds.
	switch caps.SelectedBackend {
	case sandbox.BackendBubblewrap, sandbox.BackendNative, sandbox.BackendNone:
		// ok
	default:
		t.Fatalf("unexpected backend kind: %q", caps.SelectedBackend)
	}
	// If it is None, a reason must be present.
	if caps.SelectedBackend == sandbox.BackendNone {
		assert.NotEmpty(t, caps.UnavailableReason)
	}
}

// TestSelectBackend_NoneCarriesReason verifies that whenever SelectBackend
// returns BackendNone, the reason is non-empty AND mentions one of the
// expected hint phrases.
func TestSelectBackend_NoneCarriesReason(t *testing.T) {
	cases := []sandbox.SandboxCapabilities{
		{GOOS: "darwin"},
		{GOOS: "windows"},
		{GOOS: "linux"}, // no bwrap, no userns
	}
	for i, c := range cases {
		c := c
		t.Run(c.GOOS, func(t *testing.T) {
			kind, reason := sandbox.SelectBackend(c)
			require.Equal(t, sandbox.BackendNone, kind, "case %d expected None", i)
			require.NotEmpty(t, reason, "case %d must carry a reason", i)
			lower := strings.ToLower(reason)
			ok := strings.Contains(lower, "install bubblewrap") ||
				strings.Contains(lower, "user namespaces") ||
				strings.Contains(lower, "linux")
			assert.True(t, ok,
				"reason %q must mention install bubblewrap / user namespaces / Linux", reason)
		})
	}
}
