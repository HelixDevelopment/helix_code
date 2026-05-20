// Package httpclient provides a shared tuned HTTP/2 transport factory for
// HelixCode's LLM providers.
//
// It is a deliberately dependency-free leaf package: it imports only the
// standard library so that any provider package can import it without
// risking an import cycle (the parent internal/providers package already
// imports internal/llm, so the factory cannot live there directly).
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// httpclient.go — shared tuned HTTP/2 transport factory for LLM providers.
//
// Rationale (speed programme Phase 1, task P1-T01):
//
//   - R1 bottleneck B03: 8 of 9 LLM providers
//     (openai/deepseek/gemini/azure/copilot/cerebras/koboldai/local +
//     local_llm_manager) constructed `&http.Client{Timeout: ...}` with Go's
//     DEFAULT http.Transport. The default transport caps
//     MaxIdleConnsPerHost at 2 — so a burst of concurrent or rapid-fire
//     requests to a single provider endpoint opens and closes TCP+TLS
//     connections repeatedly, paying a full TLS handshake per call.
//   - R3 §4.7: the fix is one shared, tuned *http.Transport reused across
//     all providers — Groq already proved the pattern (groq_provider.go).
//
// This factory centralises that tuning so every provider gets connection
// pooling and an explicit HTTP/2 hint. ONLY connection-pooling behaviour
// changes — request bodies, headers, auth, endpoints, and error handling
// are untouched by callers that migrate to this factory.
const (
	// transportMaxIdleConns is the total idle-connection cap across all
	// hosts. ~256 comfortably covers multi-provider concurrent usage.
	transportMaxIdleConns = 256

	// transportMaxIdleConnsPerHost is the per-host idle-connection cap.
	// Go's default is 2 (the B03 bottleneck). 64 lets a burst of
	// concurrent requests to a single provider reuse warm connections
	// instead of churning TLS handshakes.
	transportMaxIdleConnsPerHost = 64

	// transportIdleConnTimeout is how long an idle connection is kept
	// in the pool before being closed.
	transportIdleConnTimeout = 90 * time.Second

	// transportTLSHandshakeTimeout bounds the TLS handshake.
	transportTLSHandshakeTimeout = 10 * time.Second

	// transportExpectContinueTimeout bounds the wait for a 100-continue
	// response when an Expect: 100-continue header is sent.
	transportExpectContinueTimeout = 1 * time.Second

	// dialTimeout bounds the TCP connect phase of a new connection.
	dialTimeout = 30 * time.Second

	// dialKeepAlive is the interval between TCP keep-alive probes on
	// established connections.
	dialKeepAlive = 30 * time.Second
)

// NewTunedTransport returns a fresh *http.Transport tuned for the
// concurrent / rapid-fire request pattern of LLM providers.
//
// The transport sets MaxIdleConnsPerHost well above Go's default of 2,
// keeps idle connections warm for IdleConnTimeout, and sets
// ForceAttemptHTTP2 so HTTP/2 is negotiated over TLS where the server
// supports it. Each call returns a new instance — a transport maintains
// its own connection pool, so callers that want a shared pool must share
// the *http.Client (see NewHTTPClient).
func NewTunedTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: dialKeepAlive,
		}).DialContext,
		MaxIdleConns:          transportMaxIdleConns,
		MaxIdleConnsPerHost:   transportMaxIdleConnsPerHost,
		IdleConnTimeout:       transportIdleConnTimeout,
		TLSHandshakeTimeout:   transportTLSHandshakeTimeout,
		ExpectContinueTimeout: transportExpectContinueTimeout,
		ForceAttemptHTTP2:     true,
	}
}

// NewHTTPClient returns an *http.Client backed by a tuned transport
// (NewTunedTransport) with the supplied request timeout.
//
// The timeout is the caller-configurable per-request deadline — it is
// the same Timeout the providers previously set on their bare
// &http.Client{}. A non-positive timeout means no per-request deadline
// (matching net/http semantics); callers should normally pass an
// explicit timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NewTunedTransport(),
	}
}
