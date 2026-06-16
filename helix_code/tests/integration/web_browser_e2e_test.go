//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/auth"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/server"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webBrowserRedMode reports whether the §11.4.115 polarity switch RED_WEB_AUTH is
// armed. Default ("0") = standing GREEN guard (the authenticated login→generate
// journey MUST complete and render real content). "1" = reproduce-the-defect on
// the PRE-FIX frontend (a frontend that POSTs /generate with NO Authorization
// header gets 401, #output never populates, the poll times out) — proving the
// guard genuinely catches the unauth-frontend regression rather than passing
// vacuously.
func webBrowserRedMode(t *testing.T) bool {
	t.Helper()
	return strings.TrimSpace(os.Getenv("RED_WEB_AUTH")) == "1"
}

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
// the browser DOM — through the REAL authenticated user journey.
//
// §11.4.1 fix-A-creates-B / §11.4.118 discovery: a sibling stream correctly
// landed authMiddleware on the paid /api/v1/llm/{generate,stream} + /specify
// route groups (server.go: the llmCost + specify groups). That security gate is
// CORRECT and is NOT reverted here (§11.4.120 reconcile-don't-revert). But the
// web frontend POSTed those endpoints with NO Authorization header and had no
// token mechanism — so post-gate the web UI got 401 and #output never
// populated. THE FIX (web/frontend): a real login form mints a JWT via
// /api/v1/auth/login, stores it in sessionStorage, and sends it as
// `Authorization: Bearer <token>` on the paid calls. THIS test proves the fix
// end-to-end through the real browse→sign-in→generate journey (§11.4.143).
//
// §11.4.115 polarity: GREEN (default) drives the full authenticated journey and
// asserts the model's real answer renders. RED (RED_WEB_AUTH=1) deletes the
// stored token AFTER login and just before submit, reproducing the pre-fix
// unauth-frontend defect — generate returns 401, #output stays empty, the poll
// times out — proving the guard genuinely catches the regression.
//
// Auth uses the REAL database (CONST-050(A) / §11.4): a real user is registered
// against real PostgreSQL, the real /api/v1/auth/login mints a real JWT the real
// authMiddleware (VerifyJWTWithDB) accepts. No mock, no fake, no hardcoded
// shared credential — the username/password are unique per run.
func TestWebBrowserE2E_GenerateRoundTrip(t *testing.T) {
	red := webBrowserRedMode(t)

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

	// --- Gate 3: real database (auth backend) -------------------------------
	// VerifyJWTWithDB needs a real active user row; a nil-DB server can never
	// authenticate (auth.go returns ErrAuthBackendUnavailable). Connect to the
	// real PostgreSQL the full-test stack provides; honest SKIP if absent.
	dbCfg, ok := realDBConfigFromEnv()
	if !ok {
		t.Skip("SKIP-OK: no real database configured (set DB_HOST/HELIX_DATABASE_HOST); the web login→generate journey requires the real auth backend") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s with a real headless browser + real auth DB %s:%d (RED=%v)", model, ollamaEndpoint, dbCfg.Host, dbCfg.Port, red)

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

	// --- Connect the real DB + register a real user with known creds --------
	// Used to drive the real login form. Distinct from realAuthedServer (which
	// returns a token, not the password) because the BROWSER must type real
	// credentials into the real form — the faithful end-user journey.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	realDB, derr := database.New(dbCfg)
	if derr != nil {
		t.Skipf("SKIP-OK: real database at %s:%d unreachable: %v; cannot exercise the web auth journey", dbCfg.Host, dbCfg.Port, derr) //nolint
	}
	if pingErr := realDB.Pool.Ping(dbCtx); pingErr != nil {
		realDB.Pool.Close()
		t.Skipf("SKIP-OK: real database at %s:%d did not answer ping: %v", dbCfg.Host, dbCfg.Port, pingErr) //nolint
	}
	t.Cleanup(func() { realDB.Pool.Close() })

	authSvc := auth.NewAuthService(
		auth.AuthConfig{JWTSecret: authJWTSecret, TokenExpiry: time.Hour, BcryptCost: 4},
		auth.NewAuthDB(realDB.Pool),
	)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	username := "web_e2e_" + suffix
	password := "web-e2e-password-" + suffix // unique per run; never hardcoded/shared
	if _, regErr := authSvc.Register(dbCtx, username, fmt.Sprintf("%s@e2e.test", username), password, "Web E2E User"); regErr != nil {
		t.Skipf("SKIP-OK: could not register a real test user against %s:%d (schema migrated?): %v", dbCfg.Host, dbCfg.Port, regErr) //nolint
	}

	// --- Boot the real server WITH the real DB so login + auth work ---------
	port := freePort(t)
	srv := server.New(realServerConfig(port), realDB, nil)
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
		require.Contains(t, html, `id="login-username"`, "served index.html must contain the real login username input (the auth-gap fix)")
		require.Contains(t, html, `id="login-password"`, "served index.html must contain the real login password input")
		require.Contains(t, html, `id="login-send"`, "served index.html must contain the real sign-in button")
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
	var authStatusText string
	var screenshot []byte

	// --- Stage 1: the REAL login journey (the auth-gap fix) -----------------
	// Drive the real login form: type the registered credentials into the real
	// #login-username / #login-password inputs, click the real #login-send
	// button. app.js loginOnce() POSTs /api/v1/auth/login, reads the server's
	// real `token`, and stores it in sessionStorage. We wait until the live
	// #auth-status DOM text reports authentication (set by refreshAuthStatus()
	// only after a token is actually stored) — positive proof the JWT was
	// minted by the real login endpoint and persisted client-side.
	loginErr := chromedp.Run(runCtx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(`#login-username`, chromedp.ByID),
		chromedp.SendKeys(`#login-username`, username, chromedp.ByID),
		chromedp.SendKeys(`#login-password`, password, chromedp.ByID),
		chromedp.Click(`#login-send`, chromedp.ByID),
		// Poll the live DOM until the auth status reports a stored token.
		chromedp.Poll(
			`/authenticated/.test((document.querySelector('#auth-status')||{}).textContent||'')`,
			nil,
			chromedp.WithPollingTimeout(20*time.Second),
			chromedp.WithPollingInterval(200*time.Millisecond),
		),
		chromedp.Text(`#auth-status`, &authStatusText, chromedp.ByID),
	)
	require.NoError(t, loginErr, "the real browser login journey must complete (token minted via /api/v1/auth/login and stored)")
	t.Logf("post-login #auth-status DOM text: %q", strings.TrimSpace(authStatusText))
	require.Contains(t, authStatusText, "authenticated",
		"the login form must mint+store a real JWT (sessionStorage); got auth status %q", authStatusText)
	// Sanity: the token is genuinely present in sessionStorage post-login.
	var tokenStored bool
	require.NoError(t, chromedp.Run(runCtx,
		chromedp.Evaluate(`!!window.sessionStorage.getItem('helixcode.jwt')`, &tokenStored)))
	require.True(t, tokenStored, "a real JWT must be stored in sessionStorage after the real login")

	// --- §11.4.115 RED reproduction: strip the token before generating ------
	// In RED mode, delete the just-stored token so the generate POST goes out
	// WITH NO Authorization header — exactly the pre-fix frontend behaviour.
	// The auth gate then returns 401, app.js surfaces the error, #output never
	// populates, and the poll below times out — proving the GREEN guard catches
	// the unauth-frontend regression rather than passing vacuously.
	if red {
		require.NoError(t, chromedp.Run(runCtx,
			chromedp.Evaluate(`window.sessionStorage.removeItem('helixcode.jwt'); true`, nil)))
		t.Log("RED: stored JWT removed before generate — reproducing the pre-fix unauthenticated-frontend defect")
	}

	// --- Stage 2: the authenticated generate journey -----------------------
	genErr := chromedp.Run(runCtx,
		chromedp.WaitVisible(`#prompt`, chromedp.ByID),
		// Type the discovered live model into the real #model input
		// (web/frontend/index.html). app.js buildBody() omits the model when
		// this is blank, in which case the server defaults to "llama3.2"; we
		// instead drive the model THIS host actually has installed so the real
		// generation path can run — exactly as llm_generate_e2e_test.go does.
		chromedp.SendKeys(`#model`, model, chromedp.ByID),
		// Type the checkable prompt into the real textarea.
		chromedp.SendKeys(`#prompt`, prompt, chromedp.ByID),
		// Click the real submit button — fires the form#gen-form handler that
		// POSTs /api/v1/llm/generate WITH the Authorization: Bearer header
		// (authHeaders() in app.js attaches the stored JWT).
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

	if red {
		// PRE-FIX defect reproduced: with no token, the generate POST is 401,
		// #output never populates, so the poll inside genErr times out (a
		// deadline-exceeded error). That non-nil error IS the captured defect.
		require.Error(t, genErr,
			"RED expects the unauth-frontend defect PRESENT: a tokenless generate must NOT populate #output (poll must time out)")
		// Capture what the user actually saw: the #meta error line (401).
		var redMeta string
		_ = chromedp.Run(runCtx, chromedp.Text(`#meta`, &redMeta, chromedp.ByID)) //nolint
		t.Logf("RED captured: generate without token failed to render output; genErr=%v; #meta=%q", genErr, strings.TrimSpace(redMeta))
		return
	}

	require.NoError(t, genErr, "the full authenticated browser→server→Ollama→browser round-trip must complete")

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
