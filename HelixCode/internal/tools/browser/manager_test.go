package browser

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newStubManager returns a BrowserManager whose sessionFactory builds
// a fake session without spawning chromium (avoids real subprocess in
// unit tests).
func newStubManager(t *testing.T) *BrowserManager {
	t.Helper()
	m := NewBrowserManager(nil, zap.NewNop())
	m.SetSessionFactory(func(_ context.Context, _ *BrowserManager, opts BrowserOptions) (*BrowserSession, error) {
		return &BrowserSession{
			ctx:           context.Background(),
			cancel:        func() {},
			screenshotDir: t.TempDir(),
			chromiumPath:  "/fake/chromium",
			headed:        !opts.Headless,
			createdAt:     time.Now(),
			log:           zap.NewNop(),
		}, nil
	})
	return m
}

func TestManager_EnsureSession_LazyCreates(t *testing.T) {
	m := newStubManager(t)
	require.Nil(t, m.current.Load())
	s, err := m.EnsureSession(context.Background())
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, s, m.current.Load())
}

func TestManager_EnsureSession_DoubleCallSamePointer(t *testing.T) {
	m := newStubManager(t)
	s1, err1 := m.EnsureSession(context.Background())
	s2, err2 := m.EnsureSession(context.Background())
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Same(t, s1, s2)
}

func TestManager_RequireSession_NilReturnsErr(t *testing.T) {
	m := newStubManager(t)
	_, err := m.RequireSession()
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestManager_CloseSession_FollowedByRequireFails(t *testing.T) {
	m := newStubManager(t)
	_, err := m.EnsureSession(context.Background())
	require.NoError(t, err)
	require.NoError(t, m.CloseSession())
	_, err = m.RequireSession()
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestManager_CloseSession_Idempotent(t *testing.T) {
	m := newStubManager(t)
	_, err := m.EnsureSession(context.Background())
	require.NoError(t, err)
	require.NoError(t, m.CloseSession())
	require.NoError(t, m.CloseSession())
	require.NoError(t, m.CloseSession())
}

func TestManager_CloseSession_NoActiveSession_NoOp(t *testing.T) {
	m := newStubManager(t)
	require.NoError(t, m.CloseSession())
}

func TestManager_Concurrent_EnsureSession_SamePointer(t *testing.T) {
	m := newStubManager(t)
	const N = 10
	var wg sync.WaitGroup
	seen := make([]*BrowserSession, N)
	errs := make([]error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s, err := m.EnsureSession(context.Background())
			seen[idx] = s
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for i := 0; i < N; i++ {
		require.NoError(t, errs[i])
		require.NotNil(t, seen[i])
	}
	for i := 1; i < N; i++ {
		require.Same(t, seen[0], seen[i], "concurrent EnsureSession must yield identical pointer")
	}
}

func TestManager_Status_NoActiveSession(t *testing.T) {
	m := newStubManager(t)
	st := m.Status()
	require.False(t, st.Active)
	require.Empty(t, st.ChromiumPath)
}

func TestManager_Status_ActiveSession(t *testing.T) {
	m := newStubManager(t)
	_, err := m.EnsureSession(context.Background())
	require.NoError(t, err)
	st := m.Status()
	require.True(t, st.Active)
	require.Equal(t, "/fake/chromium", st.ChromiumPath)
}

func TestManager_DefaultScreenshotRoot_NonEmpty(t *testing.T) {
	m := NewBrowserManager(nil, zap.NewNop())
	require.NotEmpty(t, m.ScreenshotRoot())
}

func TestManager_NewBrowserManager_NilLogger_NoCrash(t *testing.T) {
	m := NewBrowserManager(nil, nil)
	require.NotNil(t, m)
	require.NotNil(t, m.Logger())
}

func TestDefaultSessionFactory_NilDiscovery_ErrChromiumNotFound(t *testing.T) {
	m := NewBrowserManager(nil, zap.NewNop())
	_, err := defaultSessionFactory(context.Background(), m, OptionsFromEnv())
	require.ErrorIs(t, err, ErrChromiumNotFound)
}
