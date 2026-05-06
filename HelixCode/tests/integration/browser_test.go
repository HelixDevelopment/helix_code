//go:build integration

package integration

// browser_test.go (P2-F23-T09): end-to-end integration tests for the
// F23 cline-style browser tool suite wired against a real chromium
// subprocess. Each test exercises the production path: real
// browser.BrowserManager → real chromedp.ExecAllocator → real chromium
// → real httptest.Server fixture page → real PNG bytes.
//
// Anti-bluff anchor: each PASS captures positive runtime evidence
// (fixture sentinel byte equality, DOM-mutation byte differential,
// PNG-magic + DecodeConfig + size > 1024). Skip is permitted ONLY
// when chromium is absent and MUST emit SKIP-OK: #P2-F23 chromium
// not available.

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func evalInputValue(selector string, out *string) chromedp.Action {
	return chromedp.Value(selector, out, chromedp.ByQuery)
}

const browserFixtureHTML = `<!DOCTYPE html>
<html>
<head><title>F23-FIXTURE</title></head>
<body>
<p id="t">FIXTURE_LOADED_42</p>
<p id="m">UNCLICKED</p>
<button id="b" onclick="document.getElementById('m').textContent='CLICKED_42'">Click me</button>
<input id="in" type="text" value="" />
</body>
</html>`

func chromiumAvailable() bool {
	for _, cmd := range []string{"chromium", "chromium-browser", "google-chrome", "chrome"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}
	return false
}

func skipIfNoChromium(t *testing.T) {
	if !chromiumAvailable() {
		t.Skip("SKIP-OK: #P2-F23 chromium not available")
	}
}

func newBrowserFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(browserFixtureHTML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newBrowserMgr(t *testing.T) *browser.BrowserManager {
	t.Helper()
	mgr := browser.NewBrowserManager(browser.NewDefaultChromeDiscovery(), zap.NewNop())
	t.Cleanup(func() { _ = mgr.CloseSession() })
	return mgr
}

func TestBrowser_Integration_NavigateThenSnapshot_FixtureSentinel(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	snap := tools.NewBrowserSnapshotTool(mgr, browser.OptionsFromEnv())

	res, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)
	require.NotNil(t, res)

	snapRes, err := snap.Execute(context.Background(), map[string]interface{}{"mode": "html"})
	require.NoError(t, err)
	s := snapRes.(browser.Snapshot)
	require.Greater(t, len(s.Content), 100)
	require.Contains(t, s.Content, "FIXTURE_LOADED_42")
	require.Equal(t, "F23-FIXTURE", s.Title)
}

func TestBrowser_Integration_SnapshotText_NoHTMLTags(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	snap := tools.NewBrowserSnapshotTool(mgr, browser.OptionsFromEnv())
	_, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)

	res, err := snap.Execute(context.Background(), map[string]interface{}{"mode": "text"})
	require.NoError(t, err)
	s := res.(browser.Snapshot)
	require.Contains(t, s.Content, "FIXTURE_LOADED_42")
	require.NotContains(t, s.Content, "<p")
	require.NotContains(t, s.Content, "<button")
}

func TestBrowser_Integration_ClickMutatesDOM(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	snap := tools.NewBrowserSnapshotTool(mgr, browser.OptionsFromEnv())
	click := tools.NewBrowserClickTool(mgr, browser.OptionsFromEnv())

	_, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)

	pre, err := snap.Execute(context.Background(), map[string]interface{}{"mode": "html"})
	require.NoError(t, err)
	require.Contains(t, pre.(browser.Snapshot).Content, "UNCLICKED")

	_, err = click.Execute(context.Background(), map[string]interface{}{"selector": "#b"})
	require.NoError(t, err)

	post, err := snap.Execute(context.Background(), map[string]interface{}{"mode": "html"})
	require.NoError(t, err)
	postHTML := post.(browser.Snapshot).Content
	require.Contains(t, postHTML, "CLICKED_42")
	require.NotContains(t, postHTML, ">UNCLICKED<")
}

func TestBrowser_Integration_TypeIntoInput(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	typeT := tools.NewBrowserTypeTool(mgr, browser.OptionsFromEnv())
	_, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)
	_, err = typeT.Execute(context.Background(), map[string]interface{}{"selector": "#in", "text": "HELIX_42"})
	require.NoError(t, err)
	// Read-back via JS-eval — chromedp Value reads .value of the input.
	s, err := mgr.RequireSession()
	require.NoError(t, err)
	var val string
	require.NoError(t, s.RunWithCtx(s.Ctx(), evalInputValue("#in", &val)))
	require.Equal(t, "HELIX_42", val)
}

func TestBrowser_Integration_Screenshot_PNGMagic(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	shot := tools.NewBrowserScreenshotTool(mgr, browser.OptionsFromEnv())
	_, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)
	res, err := shot.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	r := res.(browser.ScreenshotResult)
	require.Greater(t, r.Bytes, int64(1024))
	data, err := os.ReadFile(r.Path)
	require.NoError(t, err)
	require.True(t, bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}))
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Greater(t, cfg.Width, 0)
	require.Greater(t, cfg.Height, 0)
	// File mode 0600 (CONST-042).
	info, err := os.Stat(r.Path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestBrowser_Integration_Close_RequireFailsAfter(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	srv := newBrowserFixtureServer(t)

	nav := tools.NewBrowserNavigateTool(mgr, browser.OptionsFromEnv())
	closeT := tools.NewBrowserCloseTool(mgr)
	snap := tools.NewBrowserSnapshotTool(mgr, browser.OptionsFromEnv())
	_, err := nav.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
	require.NoError(t, err)
	_, err = closeT.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	_, err = snap.Execute(context.Background(), map[string]interface{}{"mode": "html"})
	require.ErrorIs(t, err, browser.ErrNoActiveSession)
}

func TestBrowser_Integration_ConcurrentEnsureSession_SamePointer(t *testing.T) {
	skipIfNoChromium(t)
	mgr := newBrowserMgr(t)
	const N = 5
	var wg sync.WaitGroup
	seen := make([]*browser.BrowserSession, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s, err := mgr.EnsureSession(context.Background())
			require.NoError(t, err)
			seen[idx] = s
		}(i)
	}
	wg.Wait()
	for i := 1; i < N; i++ {
		require.Same(t, seen[0], seen[i])
	}
}
