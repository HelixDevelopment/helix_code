package server

import (
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"dev.helix.code/internal/config"
	"github.com/gin-gonic/gin"
)

// PprofHTTPEnvVar is the environment variable that, when truthy, mounts the
// net/http/pprof debug endpoints on the server router. See setupRoutes for the
// documented endpoint surface.
//
// P0-T01 (speed programme, R4 phased plan §3): the profiling endpoints are an
// opt-in measurement surface — OFF by default so production never exposes the
// profiler. CONST-035: the live /debug/pprof/profile endpoint serving a real
// CPU profile is the anti-bluff proof the mount works end-to-end.
const PprofHTTPEnvVar = "HELIX_PPROF_HTTP"

// pprofHTTPEnabled reports whether the net/http/pprof debug endpoints should
// be mounted: true when HELIX_PPROF_HTTP is truthy, or when the logging level
// is "debug" (the existing whole-server debug gate).
func pprofHTTPEnabled(cfg *config.Config) bool {
	if truthy(os.Getenv(PprofHTTPEnvVar)) {
		return true
	}
	if cfg != nil && strings.EqualFold(cfg.Logging.Level, "debug") {
		return true
	}
	return false
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

// mountPprof registers the standard net/http/pprof handlers on the Gin router
// under /debug/pprof/. It bridges the stdlib http.HandlerFunc surface to Gin
// via gin.WrapF / gin.WrapH so the endpoints behave exactly like the canonical
// net/http/pprof mount.
func (s *Server) mountPprof() {
	group := s.router.Group("/debug/pprof")
	group.GET("/", gin.WrapF(pprof.Index))
	group.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	group.GET("/profile", gin.WrapF(pprof.Profile))
	group.GET("/symbol", gin.WrapF(pprof.Symbol))
	group.POST("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/trace", gin.WrapF(pprof.Trace))
	// The named runtime profiles (heap, goroutine, allocs, block, mutex,
	// threadcreate) are all served by pprof.Handler(name) — Index links to
	// these by name, so register each explicitly.
	for _, name := range []string{"heap", "goroutine", "allocs", "block", "mutex", "threadcreate"} {
		group.GET("/"+name, gin.WrapH(pprof.Handler(name)))
	}
}

// Handler returns the server's configured HTTP handler (the Gin router with
// every route, including the opt-in /debug/pprof/* mount, already installed).
// It is the seam the P0-T01 integration test uses to exercise the pprof
// endpoints against an httptest.Server without binding a real listener.
func (s *Server) Handler() http.Handler {
	return s.router
}

// compile-time assertion that the stdlib handler type is what we expect.
var _ http.HandlerFunc = pprof.Index
