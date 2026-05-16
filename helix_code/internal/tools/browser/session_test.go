package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSession_NextScreenshotPath_Monotonic(t *testing.T) {
	s := &BrowserSession{screenshotDir: t.TempDir()}
	p1 := s.NextScreenshotPath()
	p2 := s.NextScreenshotPath()
	p3 := s.NextScreenshotPath()
	require.NotEqual(t, p1, p2)
	require.NotEqual(t, p2, p3)
	require.True(t, strings.HasSuffix(p1, "1.png"))
	require.True(t, strings.HasSuffix(p2, "2.png"))
	require.True(t, strings.HasSuffix(p3, "3.png"))
	require.Equal(t, filepath.Dir(p1), s.screenshotDir)
}

func TestSession_Close_Idempotent_SyncOnce(t *testing.T) {
	n := 0
	dir := t.TempDir()
	subDir := filepath.Join(dir, "session1")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	s := &BrowserSession{cancel: func() { n++ }, screenshotDir: subDir}
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
	require.Equal(t, 1, n, "cancel must be called exactly once across multiple Close calls")
	// Tempdir removed.
	_, err := os.Stat(subDir)
	require.True(t, os.IsNotExist(err))
}

func TestSession_Run_NilCtx_ErrNoActiveSession(t *testing.T) {
	s := &BrowserSession{}
	err := s.Run(context.Background())
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestSession_Run_NilSession_ErrNoActiveSession(t *testing.T) {
	var s *BrowserSession
	err := s.Run(context.Background())
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestSession_Close_NilSession_NoOp(t *testing.T) {
	var s *BrowserSession
	require.NoError(t, s.Close())
}

func TestSession_Accessors(t *testing.T) {
	dir := t.TempDir()
	s := &BrowserSession{
		screenshotDir: dir,
		chromiumPath:  "/fake/chromium",
		headed:        true,
	}
	require.Equal(t, dir, s.ScreenshotDir())
	require.Equal(t, "/fake/chromium", s.ChromiumPath())
	require.True(t, s.Headed())
}
