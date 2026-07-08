package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestCORSMiddleware_RED_WildcardOriginWithCredentials_Forbidden is a
// §11.4.115 RED-baseline-on-the-broken-artifact regression test (§11.4.135
// permanent guard). It captures the confirmed CORS spec violation: the
// pre-fix CORSMiddleware unconditionally emitted BOTH
// "Access-Control-Allow-Origin: *" AND "Access-Control-Allow-Credentials:
// true" on every response. That combination is forbidden by the Fetch/CORS
// spec (RFC 6454 / Fetch §3.2.3) — real browsers reject a wildcard
// Allow-Origin whenever credentials mode is "include", and any
// implementation that instead reflects the wildcard as the literal request
// Origin lets ANY origin make credentialed cross-origin requests (a
// genuine cross-origin credential-theft vector).
//
// RED_MODE semantics (§11.4.115 polarity switch): this test asserts the
// DEFECT IS ABSENT post-fix — i.e. it is the GREEN regression guard. The
// historical RED run (defect PRESENT) was captured against the pre-fix
// commit and is preserved below in the doc comment as the §11.4.5/§11.4.69
// captured evidence trail:
//
//	=== RUN   TestCORSMiddleware_RED_WildcardOriginWithCredentials_Forbidden
//	    cors_security_test.go:41: PRE-FIX CAPTURED: Access-Control-Allow-Origin="*" Access-Control-Allow-Credentials="true" (forbidden combo present)
//	--- PASS: TestCORSMiddleware_RED_WildcardOriginWithCredentials_Forbidden (0.00s)
//
// After the fix (allowlist-based origin echo, §11.4.74 reuse of the
// HELIX_WS_ALLOWED_ORIGINS config pattern as HELIX_CORS_ALLOWED_ORIGINS),
// this test asserts the forbidden combo can never occur, for any origin —
// allowed or not.
func TestCORSMiddleware_RED_WildcardOriginWithCredentials_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	// Allowlist a specific origin so we can prove even an ALLOWED origin
	// never gets back a literal "*" for Allow-Origin.
	router.Use(CORSMiddleware([]string{"https://app.example.com"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	for _, origin := range []string{
		"https://app.example.com", // allowlisted
		"https://evil.example.com", // NOT allowlisted
		"", // no Origin header (non-browser / same-origin caller)
	} {
		req, _ := http.NewRequest("GET", "/test", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
		allowCreds := w.Header().Get("Access-Control-Allow-Credentials")

		// The forbidden combination MUST NEVER occur: a wildcard
		// Allow-Origin together with Allow-Credentials: true.
		if allowOrigin == "*" && allowCreds == "true" {
			t.Fatalf("FORBIDDEN CORS COMBO PRESENT for origin %q: Access-Control-Allow-Origin=%q Access-Control-Allow-Credentials=%q — wildcard origin MUST NEVER be paired with credentials (Fetch/CORS spec)", origin, allowOrigin, allowCreds)
		}
		// Never a bare wildcard at all now that we're allowlist-driven.
		assert.NotEqual(t, "*", allowOrigin, "origin %q: Access-Control-Allow-Origin must never be a wildcard", origin)
	}
}

// TestCORSMiddleware_DisallowedOrigin_NoAllowHeaders proves the fixed
// middleware default-denies: a request from an origin NOT in the
// allowlist gets no Access-Control-Allow-Origin and no
// Access-Control-Allow-Credentials header at all (so the browser blocks
// the cross-origin credentialed read).
func TestCORSMiddleware_DisallowedOrigin_NoAllowHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORSMiddleware([]string{"https://app.example.com"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "disallowed origin must not get Allow-Origin")
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"), "disallowed origin must not get Allow-Credentials")
}

// TestCORSMiddleware_AllowedOrigin_EchoedWithVary proves the fixed
// middleware, for an allowlisted origin, echoes that SPECIFIC origin back
// (never "*"), sets Vary: Origin (so caches don't leak the response across
// origins), and only then sets Allow-Credentials: true.
func TestCORSMiddleware_AllowedOrigin_EchoedWithVary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORSMiddleware([]string{"https://app.example.com", "https://admin.example.com"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}
