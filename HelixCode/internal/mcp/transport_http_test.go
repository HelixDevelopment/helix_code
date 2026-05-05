package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestHTTPTransport_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		var req MCPMessage
		require.NoError(t, json.Unmarshal(body, &req))
		resp := MCPMessage{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&resp)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPConfig{URL: srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	resp, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1", resp.ID)
	assert.Equal(t, TransportHTTP, tr.Type())
}

func TestHTTPTransport_BearerHeader(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(&MCPMessage{JSONRPC: "2.0", ID: "1", Result: map[string]any{}})
	}))
	defer srv.Close()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "tok-xyz"})
	tr := NewHTTPTransport(HTTPConfig{URL: srv.URL, TokenSource: ts})
	ctx := context.Background()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	_, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-xyz", seenAuth)
}

func TestHTTPTransport_401WithOAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", 401)
	}))
	defer srv.Close()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "expired"})
	tr := NewHTTPTransport(HTTPConfig{URL: srv.URL, TokenSource: ts, OAuthEnabled: true})
	ctx := context.Background()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	_, err := tr.Recv(ctx)
	require.ErrorIs(t, err, ErrOAuthRequired)
}

func TestHTTPTransport_4xxNoOAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", 400)
	}))
	defer srv.Close()
	tr := NewHTTPTransport(HTTPConfig{URL: srv.URL})
	ctx := context.Background()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	_, err := tr.Recv(ctx)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "400") || isErrProtocol(err))
}

func isErrProtocol(err error) bool {
	for ; err != nil; err = unwrapErr(err) {
		if err == ErrProtocol {
			return true
		}
	}
	return false
}

func unwrapErr(e error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := e.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
