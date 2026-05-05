package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCE_VerifierAndChallenge(t *testing.T) {
	v, c, err := GeneratePKCE()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(v), 43)
	assert.LessOrEqual(t, len(v), 128)
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	assert.Equal(t, want, c)
}

func TestOAuth_DiscoverASMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&ASMetadata{
			Issuer:                "https://example.com",
			AuthorizationEndpoint: "https://example.com/authz",
			TokenEndpoint:         "https://example.com/token",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	md, err := DiscoverAS(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/authz", md.AuthorizationEndpoint)
}

func TestOAuth_ExchangeCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "code-xyz", r.Form.Get("code"))
		assert.NotEmpty(t, r.Form.Get("code_verifier"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "tok-1",
			"token_type":    "Bearer",
			"refresh_token": "ref-1",
			"expires_in":    3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	tok, err := ExchangeCode(context.Background(), srv.URL+"/token", "code-xyz", "verifier-12345-abcdefghij-klmnopqrst-uvwxyz", "client-id", "")
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
}

func TestTokenCache_PersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	tc := &TokenCache{Dir: dir}
	tok := &SavedToken{AccessToken: "tok-1", RefreshToken: "ref-1", TokenType: "Bearer"}
	require.NoError(t, tc.Save("server-a", tok))
	path := filepath.Join(dir, "server-a.json")
	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), st.Mode().Perm())
	got, err := tc.Load("server-a")
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got.AccessToken)
}

func TestTokenCache_RefuseLooseMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server-b.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"access_token":"x"}`), 0644))
	require.NoError(t, os.Chmod(path, 0644))
	tc := &TokenCache{Dir: dir}
	_, err := tc.Load("server-b")
	require.Error(t, err)
}

func TestTokenCache_OverwriteEnforcesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server-c.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0644))
	require.NoError(t, os.Chmod(path, 0644))
	tc := &TokenCache{Dir: dir}
	require.NoError(t, tc.Save("server-c", &SavedToken{AccessToken: "t-1"}))
	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), st.Mode().Perm())
	got, err := tc.Load("server-c")
	require.NoError(t, err)
	assert.Equal(t, "t-1", got.AccessToken)
}

func TestAuthorizationURL_BuildsExpected(t *testing.T) {
	u := BuildAuthorizationURL(AuthRequest{
		AuthorizationEndpoint: "https://example.com/authz",
		ClientID:              "cid",
		RedirectURI:           "http://127.0.0.1:9000/callback",
		Scope:                 "tools",
		State:                 "st-1",
		CodeChallenge:         "ch-1",
	})
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	q := parsed.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "cid", q.Get("client_id"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "ch-1", q.Get("code_challenge"))
	assert.Equal(t, "st-1", q.Get("state"))
}
