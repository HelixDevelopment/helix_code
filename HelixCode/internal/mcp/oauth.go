package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ASMetadata is the subset of RFC 8414 fields we use.
type ASMetadata struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	RegistrationEndpoint   string   `json:"registration_endpoint,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported []string `json:"response_types_supported,omitempty"`
}

// SavedToken is the persisted on-disk format for an OAuth token.
type SavedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

// AuthRequest captures the inputs for building an authorization URL.
type AuthRequest struct {
	AuthorizationEndpoint string
	ClientID              string
	RedirectURI           string
	Scope                 string
	State                 string
	CodeChallenge         string
}

// generatePKCE returns (verifier, challenge) per RFC 7636.
// Verifier is 64 base64url characters (48 bytes of entropy → 64 chars).
// Challenge is SHA-256(verifier), base64url-no-padding.
func generatePKCE() (string, string, error) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// randState returns a 32-byte base64url state parameter.
func randState() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// DiscoverAS fetches /.well-known/oauth-authorization-server (RFC 8414).
func DiscoverAS(ctx context.Context, baseURL string) (*ASMetadata, error) {
	u := strings.TrimRight(baseURL, "/") + "/.well-known/oauth-authorization-server"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mcp oauth: AS metadata status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var md ASMetadata
	if err := json.Unmarshal(body, &md); err != nil {
		return nil, fmt.Errorf("mcp oauth: parse AS metadata: %w", err)
	}
	return &md, nil
}

// BuildAuthorizationURL composes the user-facing authorization URL per RFC 6749 + RFC 7636.
func BuildAuthorizationURL(r AuthRequest) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", r.ClientID)
	q.Set("redirect_uri", r.RedirectURI)
	if r.Scope != "" {
		q.Set("scope", r.Scope)
	}
	q.Set("state", r.State)
	q.Set("code_challenge", r.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	sep := "?"
	if strings.Contains(r.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	return r.AuthorizationEndpoint + sep + q.Encode()
}

// ExchangeCode performs the RFC 6749 authorization-code grant with PKCE.
func ExchangeCode(ctx context.Context, tokenEndpoint, code, verifier, clientID, redirectURI string) (*SavedToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", clientID)
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp oauth: token exchange status %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("mcp oauth: parse token: %w", err)
	}
	tok := &SavedToken{
		AccessToken:  raw.AccessToken,
		TokenType:    raw.TokenType,
		RefreshToken: raw.RefreshToken,
	}
	if raw.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	return tok, nil
}

// TokenCache persists OAuth tokens at <Dir>/<server>.json with mode 0600.
type TokenCache struct {
	Dir string
}

func (c *TokenCache) path(server string) string {
	return filepath.Join(c.Dir, server+".json")
}

// Save writes the token to <Dir>/<server>.json with mode 0600.
func (c *TokenCache) Save(server string, tok *SavedToken) error {
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		return fmt.Errorf("mcp oauth cache: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	path := c.path(server)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return nil
}

// Load reads the token from <Dir>/<server>.json. Refuses to read if mode is not 0600.
func (c *TokenCache) Load(server string) (*SavedToken, error) {
	path := c.path(server)
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("mcp oauth cache: refusing %s: mode is %v, want 0600", path, st.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok SavedToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}
