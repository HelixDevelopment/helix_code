package sandbox

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// spyBackend is a hexagonal seam (interface-driven testability) used to
// observe whether Manager.Execute reaches the backend's Run method. It is
// NOT a mock-bluff: it stands in for a real backend in tests where the
// concern is dispatch ordering (deny BEFORE Run), not execution semantics.
//
// Run-counter is atomic so concurrent tests stay race-clean.
type spyBackend struct {
	mu             sync.Mutex
	kind           BackendKind
	calls          atomic.Int32
	lastCommand    string
	lastPolicy     SandboxPolicy
	resultStdout   string
	resultExitCode int
}

func newSpyBackend(kind BackendKind) *spyBackend {
	return &spyBackend{
		kind:           kind,
		resultStdout:   "spy-ok",
		resultExitCode: 0,
	}
}

func (s *spyBackend) Kind() BackendKind { return s.kind }

func (s *spyBackend) Run(_ context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls.Add(1)
	s.lastCommand = command
	s.lastPolicy = policy
	return &SandboxResult{
		Stdout:   s.resultStdout,
		ExitCode: s.resultExitCode,
		Backend:  s.kind,
		Duration: 1 * time.Millisecond,
	}, nil
}

func (s *spyBackend) Calls() int32 {
	return s.calls.Load()
}

func (s *spyBackend) LastCommand() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCommand
}

func (s *spyBackend) LastPolicy() SandboxPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPolicy
}

// newTestManagerWithSpy builds a manager whose only backend is the spy.
// We install the spy into both backend slots through the
// SandboxManagerOption seam (set bubblewrap nil + native nil and override
// dispatch via the test seam).
//
// Implementation detail: since the production manager dispatches based on
// caps.SelectedBackend, we set caps.SelectedBackend = BackendBubblewrap
// (or whichever) and stash the spy as the bubblewrap backend by setting
// the test-only `backendOverride` field through NewSandboxManager.
func newTestManagerWithSpy(t *testing.T, kind BackendKind, cfg SandboxConfig) (*SandboxManager, *spyBackend) {
	t.Helper()
	spy := newSpyBackend(kind)
	caps := SandboxCapabilities{
		GOOS:            "linux",
		SelectedBackend: kind,
	}
	mgr := NewSandboxManager(caps, nil, nil, cfg, zap.NewNop())
	// Install the spy via the test seam. The manager looks up backendOverride
	// first when present (production code never sets it).
	mgr.backendOverride = spy
	require.NoError(t, mgr.compileUserDeny())
	return mgr, spy
}

func TestSandboxManager_RejectsConstitutionalDenyList_BeforeSpawn(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	res, err := mgr.Execute(context.Background(), "systemctl suspend", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls(), "backend.Run must NOT be called when CONST-033 matches")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.True(t, errors.Is(err, ErrCommandDenied))
	require.Contains(t, denyErr.MatchedRule, "CONST-033")
}

func TestSandboxManager_RejectsConstitutionalDeny_NestedBashC(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	res, err := mgr.Execute(context.Background(), "bash -c 'systemctl suspend'", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls(), "nested form must be rejected before spawn")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.Contains(t, denyErr.MatchedRule, "CONST-033")
}

func TestSandboxManager_RejectsConstitutionalDeny_ChainedSemicolon(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	res, err := mgr.Execute(context.Background(), "ls; systemctl suspend", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls(), "chained form must be rejected before spawn")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.Contains(t, denyErr.MatchedRule, "CONST-033")
}

func TestSandboxManager_RejectsConstitutionalDeny_Whitespace(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	res, err := mgr.Execute(context.Background(), "systemctl   poweroff", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls(), "extra-whitespace form must be rejected before spawn")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.Contains(t, denyErr.MatchedRule, "CONST-033")
}

func TestSandboxManager_RejectsUserDeny_AfterCompile(t *testing.T) {
	cfg := SandboxConfig{
		DefaultPolicy: DefaultSandboxPolicy(),
		UserDenyList:  []string{`^rm -rf /`},
	}
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, cfg)

	res, err := mgr.Execute(context.Background(), "rm -rf / --no-preserve-root", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls(), "user-deny match must reject before spawn")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.True(t, errors.Is(err, ErrCommandDenied))
	require.Contains(t, denyErr.MatchedRule, "user-deny")
	require.Equal(t, `^rm -rf /`, denyErr.Pattern)
}

func TestSandboxManager_UserDenyCannotSubtractConst033(t *testing.T) {
	// User tries to add an unrelated benign pattern; CONST-033 still applies.
	cfg := SandboxConfig{
		DefaultPolicy: DefaultSandboxPolicy(),
		UserDenyList:  []string{`^benign$`},
	}
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, cfg)

	res, err := mgr.Execute(context.Background(), "systemctl suspend", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 0, spy.Calls())

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	// MUST be CONST-033, not user-deny.
	require.Contains(t, denyErr.MatchedRule, "CONST-033")
	require.NotContains(t, denyErr.MatchedRule, "user-deny")
}

func TestSandboxManager_FailClosed_WhenBackendIsNone(t *testing.T) {
	caps := SandboxCapabilities{
		GOOS:              runtime.GOOS,
		SelectedBackend:   BackendNone,
		UnavailableReason: "test fail-closed",
	}
	mgr := NewSandboxManager(caps, nil, nil, DefaultSandboxConfig(), zap.NewNop())

	res, err := mgr.Execute(context.Background(), "echo hi", DefaultSandboxPolicy())

	require.Nil(t, res)
	require.Error(t, err)

	var fcErr *FailClosedError
	require.ErrorAs(t, err, &fcErr)
	require.True(t, errors.Is(err, ErrSandboxUnavailable))
	require.Contains(t, fcErr.Reason, "test fail-closed")
}

func TestSandboxManager_PassesThroughToSpyBackend(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	policy := DefaultSandboxPolicy()
	res, err := mgr.Execute(context.Background(), "echo hi", policy)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.EqualValues(t, 1, spy.Calls(), "benign command must dispatch exactly once")
	require.Equal(t, "echo hi", spy.LastCommand())
	require.Equal(t, "spy-ok", res.Stdout)
	require.Equal(t, BackendBubblewrap, res.Backend)
}

func TestSandboxManager_NetworkAllowedRespected(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	policy := DefaultSandboxPolicy()
	policy.NetworkAllowed = true

	_, err := mgr.Execute(context.Background(), "curl https://example.com", policy)
	require.NoError(t, err)

	require.True(t, spy.LastPolicy().NetworkAllowed, "manager must propagate NetworkAllowed=true")
}

func TestSandboxManager_DefaultPolicy_NetworkDeniedByDefault(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	// Caller passes the zero policy. Manager must substitute defaults
	// (network DENY).
	_, err := mgr.Execute(context.Background(), "echo hi", SandboxPolicy{})
	require.NoError(t, err)

	got := spy.LastPolicy()
	require.False(t, got.NetworkAllowed, "zero policy must become default-DENY")
	require.Equal(t, 30*time.Second, got.Timeout, "zero policy must inherit default timeout")
	require.True(t, got.ReadOnlyRoot, "zero policy must inherit default RO-root")
}

func TestSandboxManager_UpdateConfig_RecompilesUserDeny(t *testing.T) {
	mgr, spy := newTestManagerWithSpy(t, BackendBubblewrap, DefaultSandboxConfig())

	// Phase 1: no user-deny → forbidden-cmd passes through.
	_, err := mgr.Execute(context.Background(), "forbidden-cmd arg", DefaultSandboxPolicy())
	require.NoError(t, err)
	require.EqualValues(t, 1, spy.Calls())

	// Phase 2: install user-deny that matches it.
	require.NoError(t, mgr.UpdateConfig(SandboxConfig{
		DefaultPolicy: DefaultSandboxPolicy(),
		UserDenyList:  []string{`^forbidden-cmd`},
	}))

	res, err := mgr.Execute(context.Background(), "forbidden-cmd arg", DefaultSandboxPolicy())
	require.Nil(t, res)
	require.Error(t, err)
	require.EqualValues(t, 1, spy.Calls(), "user-deny added at runtime must take effect; no second Run call")

	var denyErr *DenyError
	require.ErrorAs(t, err, &denyErr)
	require.Contains(t, denyErr.MatchedRule, "user-deny")
}

func TestSandboxManager_BackendKindReportsCorrect(t *testing.T) {
	caps := SandboxCapabilities{GOOS: "linux", SelectedBackend: BackendBubblewrap}
	mgr := NewSandboxManager(caps, nil, nil, DefaultSandboxConfig(), zap.NewNop())
	require.Equal(t, BackendBubblewrap, mgr.SelectedBackend())

	caps.SelectedBackend = BackendNative
	mgr = NewSandboxManager(caps, nil, nil, DefaultSandboxConfig(), zap.NewNop())
	require.Equal(t, BackendNative, mgr.SelectedBackend())

	caps.SelectedBackend = BackendNone
	caps.UnavailableReason = "no usable backend"
	mgr = NewSandboxManager(caps, nil, nil, DefaultSandboxConfig(), zap.NewNop())
	require.Equal(t, BackendNone, mgr.SelectedBackend())
}

func TestSandboxManager_MergedDenyList_FormatsCorrectly(t *testing.T) {
	cfg := SandboxConfig{
		DefaultPolicy: DefaultSandboxPolicy(),
		UserDenyList:  []string{`^rm -rf`, `forkbomb`},
	}
	mgr, _ := newTestManagerWithSpy(t, BackendBubblewrap, cfg)

	constList, userList := mgr.MergedDenyList()

	require.NotEmpty(t, constList, "CONST-033 list must be non-empty")
	// Spot-check: at least one description must mention systemctl power-management.
	foundSystemctl := false
	for _, d := range constList {
		if strings.Contains(d, "systemctl power-management") {
			foundSystemctl = true
			break
		}
	}
	require.True(t, foundSystemctl, "expected systemctl power-management description in CONST-033 list")

	require.Equal(t, []string{`^rm -rf`, `forkbomb`}, userList)
}

func TestNewSandboxManagerFromDetector_OnRealHost(t *testing.T) {
	// Real-host probe: must hit the real Detector.
	cwd := t.TempDir()

	mgr, caps, err := NewSandboxManagerFromDetector(cwd, DefaultSandboxConfig(), zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.Equal(t, runtime.GOOS, caps.GOOS)

	// Surface what the detector picked so the test report line shows it.
	t.Logf("on-real-host backend resolved to: %s (bwrap=%q userns=%v cg2=%v reason=%q)",
		caps.SelectedBackend, caps.BubblewrapPath, caps.UnprivilegedUserNS, caps.CGroupsV2, caps.UnavailableReason)

	// On the development machine bubblewrap is installed; assert it.
	if runtime.GOOS == "linux" && caps.BubblewrapPath != "" {
		assert.Equal(t, BackendBubblewrap, caps.SelectedBackend)
		assert.Equal(t, BackendBubblewrap, mgr.SelectedBackend())
	}
}

// Compile-time assertion that spyBackend satisfies SandboxBackend so the
// "interface-seam, not a mock-bluff" intent is enforced by the type system.
var _ SandboxBackend = (*spyBackend)(nil)
