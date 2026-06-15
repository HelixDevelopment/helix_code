//go:build integration

package integration

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/server"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// web_browser_e2e_test.go — REAL browser-driven end-to-end exercise of the
// HelixCode web client, proving the full
//
//	browser → server → provider (live Ollama) → browser
//
// round-trip that the CONTINUATION record flagged as an honest gap (no live
// browser→server→provider round-trip had been captured). The sibling
// llm_generate_e2e_test.go proves the HTTP-caller half (a Go http.Client POST
// reaches Ollama and gets a real answer); THIS test proves the missing half:
// the actual web UI — index.html + app.js running inside a real headless
// Chrome — types a prompt into the real DOM input, clicks the real submit
// button, and renders the model's genuine answer into the real output element.
//
// Production path exercised end-to-end:
//
//	chromedp drives real Chrome
//	  → GET /            (server.setupRoutes StaticFile -> web/frontend/index.html)
//	  → GET /static/app.js, /static/app.css (router.Static)
//	  → app.js form#gen-form submit handler (app.js:83) fires
//	  → fetch POST /api/v1/llm/generate (app.js:33)
//	  → server.generateLLM -> resolveLLMProvider (local Ollama default)
//	    -> llm.NewOllamaProvider -> provider.Generate
//	    -> REAL HTTP POST http://localhost:11434/api/chat -> real model tokens
//	  → app.js writes data.content into <pre id="output"> (app.js:42)
//	  → chromedp reads #output back out of the live DOM
//
// Real DOM selectors (verified in the actual client source, NOT invented):
//   - prompt input:  textarea#prompt        web/frontend/index.html:29
//   - send trigger:  button#send (submit)   web/frontend/index.html:35
//                    form#gen-form handler   web/frontend/static/app.js:83
//   - output sink:   <pre id="output">       web/frontend/index.html:41
//                    written by output.textContent = data.content  app.js:42
//   - meta line:     <p id="meta">           web/frontend/index.html:42
//                    "provider=ollama …" on success  app.js:44-49
//
// Anti-bluff (CONST-035 / Article XI §11.9 / §11.4.107): the PASS captures
// POSITIVE runtime evidence — the answer "4" rendered into the live DOM output
// element (proving the real model's answer travelled the whole loop), plus the
// "provider=ollama" meta line (proving a real Ollama provider was constructed
// server-side), plus a screenshot of the rendered page written to a temp path.
// It is NOT a single-frame / metadata-only / launch-only check: it asserts the
// REAL content the model produced, read back out of the browser's own DOM. No
// stub, no fake provider, no canned response.
//
// Per CONST-050(A) this integration test exercises the real system. It SKIPs
// (honestly, never a fake PASS — §11.4.3) when EITHER the live Ollama provider
// is unreachable / has no model, OR no Chrome/headless browser is available on
// the host. Both skip reasons carry the SKIP-OK marker.
//
// Run:
//
//	go test -tags=integration -run TestWebBrowserE2E_GenerateRoundTrip -count=1 -v ./tests/integration/

// macChromeAppPaths mirrors the macOS app-bundle paths chromedp's own
// findExecPath() consults (chromedp/allocate.go), so the availability guard
// agrees with what chromedp will actually auto-discover — on a Mac where Chrome
// is installed as an .app but NOT on $PATH, exec.LookPath alone would falsely
// report "no browser" and SKIP a test that could really run.
var macChromeAppPaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
}

// headlessBrowserAvailable reports whether a Chrome/Chromium chromedp can drive
// exists on this host — checking BOTH the PATH binaries (Linux/most CI) AND the
// macOS app-bundle locations chromedp auto-discovers.
func headlessBrowserAvailable() bool {
	if chromiumAvailable() { // PATH lookup (defined in browser_test.go)
		return true
	}
	for _, p := range macChromeAppPaths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// innerModuleRoot returns the absolute path of the inner Go module root
// (dev.helix.code) computed from THIS test file's location, so the static-file
// routes — which server.setupRoutes registers with paths RELATIVE TO CWD
// ("./web/frontend/index.html", "./web/frontend/static") — resolve regardless
// of the directory `go test` happens to run from.
//
// This file lives at <root>/tests/integration/web_browser_e2e_test.go, so the
// module root is two directories up.
func innerModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must locate this test file")
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	// Sanity-check we landed on the module root, not somewhere else.
	require.FileExists(t, filepath.Join(root, "go.mod"), "computed module root must contain go.mod")
	require.FileExists(t, filepath.Join(root, "web", "frontend", "index.html"),
		"computed module root must contain the web frontend the server serves")
	return root
}

// TestWebBrowserE2E_GenerateRoundTrip drives a real headless Chrome against the
// real HelixCode web client served by the real server, with a live Ollama
// behind /api/v1/llm/generate, and asserts the model's genuine answer reaches
// the browser DOM.
func TestWebBrowserE2E_GenerateRoundTrip(t *testing.T) {
	// --- Gate 1: live provider (mirror llm_generate_e2e_test.go) ------------
	model, reachable := liveOllamaModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real browser→server→provider round-trip") //nolint
	}
	if model == "" {
		t.Skip("SKIP-OK: local Ollama is reachable but no model is installed; pull a model (e.g. `ollama pull qwen2.5:3b`) to exercise the browser round-trip") //nolint
	}

	// --- Gate 2: headless browser (mirror browser_test.go's honest skip) ----
	if !headlessBrowserAvailable() {
		t.Skip("SKIP-OK: #P2-F23 chromium not available; cannot drive the real web client") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s with a real headless browser", model, ollamaEndpoint)

	// --- CWD fix: serve the frontend ----------------------------------------
	// server.setupRoutes registers static routes relative to CWD. go test runs
	// with CWD = this package dir (tests/integration), where those relative
	// paths do NOT resolve. Chdir to the inner module root so they do, and
	// restore the original CWD afterwards.
	root := innerModuleRoot(t)
	origCWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root), "must chdir to module root so the frontend static files resolve")
	t.Cleanup(func() { _ = os.Chdir(origCWD) })

	// Ensure no cloud provider is named so resolveLLMProvider falls to the
	// local Ollama default — the exact out-of-the-box server path.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	// --- Boot the real server ----------------------------------------------
	port := freePort(t)
	srv := server.New(minimalServerConfig(port), nil, nil)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	base := "http://127.0.0.1:" + itoa(port)

	// Wait for the listener (or fail fast on serve error).
	require.Eventually(t, func() bool {
		select {
		case err := <-serveErr:
			t.Fatalf("server failed to start: %v", err)
			return false
		default:
		}
		c, derr := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 200*time.Millisecond)
		if derr != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 10*time.Second, 100*time.Millisecond, "server must come up on its port")

	// --- Verify the frontend genuinely serves BEFORE driving the browser ----
	// Confirms the CWD fix worked: GET / must return the real index.html (with
	// the real #prompt/#send/#output DOM the test drives), not a 404.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
		require.NoError(t, rerr)
		resp, rerr := http.DefaultClient.Do(req)
		require.NoError(t, rerr, "GET / must succeed so the browser has a page to load")
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET / must serve the frontend (CWD fix working)")
		html := string(body[:n])
		require.Contains(t, html, `id="prompt"`, "served index.html must contain the real prompt input")
		require.Contains(t, html, `id="send"`, "served index.html must contain the real send button")
		require.Contains(t, html, `id="output"`, "served index.html must contain the real output element")
	}

	// --- Drive the real browser --------------------------------------------
	// Headless Chrome via chromedp's default ExecAllocator (it auto-discovers
	// the Chrome binary, incl. the macOS app bundle).
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	// Generous budget: real model generation can take tens of seconds.
	runCtx, cancelRun := context.WithTimeout(browserCtx, 150*time.Second)
	defer cancelRun()

	const prompt = "What is 2+2? Answer with the number only."

	var renderedOutput string
	var renderedMeta string
	var screenshot []byte

	err = chromedp.Run(runCtx,
		// Load the real web client.
		chromedp.Navigate(base+"/"),
		// Wait for the real prompt input to exist + be visible.
		chromedp.WaitVisible(`#prompt`, chromedp.ByID),
		// Type the discovered live model into the real #model input
		// (web/frontend/index.html:25). app.js buildBody() (app.js:24-29)
		// omits the model when this is blank, in which case the server
		// defaults to "llama3.2"; we instead drive the model THIS host
		// actually has installed so the real generation path can run —
		// exactly as the sibling llm_generate_e2e_test.go names the model.
		chromedp.SendKeys(`#model`, model, chromedp.ByID),
		// Type the checkable prompt into the real textarea.
		chromedp.SendKeys(`#prompt`, prompt, chromedp.ByID),
		// Click the real submit button — fires the form#gen-form handler that
		// POSTs /api/v1/llm/generate.
		chromedp.Click(`#send`, chromedp.ByID),
		// Wait until the real output element is non-empty: app.js writes
		// data.content into #output ONLY after the server's real provider
		// response returns. Poll the live DOM until it is populated.
		chromedp.Poll(
			`document.querySelector('#output') && document.querySelector('#output').textContent.trim().length > 0`,
			nil,
			chromedp.WithPollingTimeout(140*time.Second),
			chromedp.WithPollingInterval(250*time.Millisecond),
		),
		// Read the rendered answer + meta back out of the live DOM.
		chromedp.Text(`#output`, &renderedOutput, chromedp.ByID, chromedp.NodeVisible),
		chromedp.Text(`#meta`, &renderedMeta, chromedp.ByID),
		chromedp.CaptureScreenshot(&screenshot),
	)
	require.NoError(t, err, "the full browser→server→Ollama→browser round-trip must complete")

	// --- Persist the screenshot (captured runtime evidence) -----------------
	shotPath := filepath.Join(t.TempDir(), "web_browser_e2e_rendered.png")
	require.NoError(t, os.WriteFile(shotPath, screenshot, 0o600))
	require.Greater(t, len(screenshot), 1024, "screenshot must be a real non-trivial PNG")

	t.Logf("rendered #output DOM text (real model answer): %q", strings.TrimSpace(renderedOutput))
	t.Logf("rendered #meta DOM text: %q", strings.TrimSpace(renderedMeta))
	t.Logf("screenshot of rendered page written to: %s (%d bytes)", shotPath, len(screenshot))

	// --- Anti-bluff assertions (§11.4.107: assert the REAL content) ---------
	require.NotEmpty(t, strings.TrimSpace(renderedOutput),
		"the browser must render the real model output into #output")

	// The model's genuine answer to 2+2 must contain "4" — proving the answer
	// travelled browser→server→ollama→browser and was rendered in the live DOM.
	// A simulated/canned client would not reliably solve arithmetic; this
	// asserts on real model output read out of the browser's own DOM.
	assert.Contains(t, renderedOutput, "4",
		"the live model's real answer to 'What is 2+2?' must be rendered in the browser #output; got %q", renderedOutput)

	// The success meta line names the real provider that was constructed
	// server-side — further proof a real Ollama provider was invoked, not a
	// canned response (app.js sets this only on a real 200 from the endpoint).
	assert.Contains(t, renderedMeta, "provider=ollama",
		"the rendered #meta must name the real provider invoked server-side; got %q", renderedMeta)
}
