package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/mcp"
)

// MCPCommandDeps wires test seams.
type MCPCommandDeps struct {
	ConfigPath string
	TestServer func(ctx context.Context, name string) error
	Auth       func(ctx context.Context, name string) error
	Logs       func(name string) ([]byte, error)
	List       func() ([]mcp.ClientStatus, error)
}

func newMCPCommand(deps MCPCommandDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP server connections",
	}
	root.AddCommand(newMCPAdd(deps))
	root.AddCommand(newMCPRemove(deps))
	root.AddCommand(newMCPList(deps))
	root.AddCommand(newMCPTest(deps))
	root.AddCommand(newMCPAuth(deps))
	root.AddCommand(newMCPLogs(deps))
	return root
}

func loadOrEmpty(path string) (*mcp.Config, error) {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return &mcp.Config{}, nil
	}
	return mcp.LoadConfig(path)
}

func newMCPAdd(deps MCPCommandDeps) *cobra.Command {
	var transport string
	var command []string
	var url string
	var sseURL string
	var oauth bool
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrEmpty(deps.ConfigPath)
			if err != nil {
				return err
			}
			spec := mcp.ServerSpec{
				Name:      args[0],
				Transport: mcp.TransportType(transport),
				Command:   command,
				URL:       url,
				SSEURL:    sseURL,
				OAuth:     mcp.OAuthSpec{Enabled: oauth},
			}
			cfg.Servers = append(cfg.Servers, spec)
			if err := mcp.SaveConfig(deps.ConfigPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%s)\n", spec.Name, spec.Transport)
			return nil
		},
	}
	cmd.Flags().StringVar(&transport, "transport", "stdio", "transport: stdio|http|sse|ws")
	cmd.Flags().StringSliceVar(&command, "command", nil, "command argv (stdio); repeat the flag")
	cmd.Flags().StringVar(&url, "url", "", "URL (http/sse/ws)")
	cmd.Flags().StringVar(&sseURL, "sse-url", "", "SSE event-stream URL")
	cmd.Flags().BoolVar(&oauth, "oauth", false, "enable OAuth for this server")
	return cmd
}

func newMCPRemove(deps MCPCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrEmpty(deps.ConfigPath)
			if err != nil {
				return err
			}
			out := cfg.Servers[:0]
			found := false
			for _, s := range cfg.Servers {
				if s.Name == args[0] {
					found = true
					continue
				}
				out = append(out, s)
			}
			if !found {
				return fmt.Errorf("mcp: server %q not found", args[0])
			}
			cfg.Servers = out
			if err := mcp.SaveConfig(deps.ConfigPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
}

func newMCPList(deps MCPCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrEmpty(deps.ConfigPath)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tTRANSPORT\tALWAYS-LOAD\tTARGET")
			for _, s := range cfg.Servers {
				target := s.URL
				if s.Transport == mcp.TransportStdio {
					target = strings.Join(s.Command, " ")
				}
				fmt.Fprintf(tw, "%s\t%s\t%t\t%s\n", s.Name, s.Transport, s.AlwaysLoad, target)
			}
			return tw.Flush()
		},
	}
}

func newMCPTest(deps MCPCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Probe a server (connect → tools/list → close)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if deps.TestServer != nil {
				if err := deps.TestServer(ctx, args[0]); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "ready\n")
				return nil
			}
			cfg, err := loadOrEmpty(deps.ConfigPath)
			if err != nil {
				return err
			}
			m := mcp.NewManager()
			m.SetConfig(cfg)
			if err := m.Test(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ready\n")
			return nil
		},
	}
}

func newMCPAuth(deps MCPCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "auth <name>",
		Short: "Run OAuth flow for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if deps.Auth != nil {
				return deps.Auth(ctx, args[0])
			}
			return runOAuthInteractive(ctx, deps.ConfigPath, args[0], cmd.OutOrStdout())
		},
	}
}

func newMCPLogs(deps MCPCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Show recent stderr + lifecycle events for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.Logs != nil {
				b, err := deps.Logs(args[0])
				if err != nil {
					return err
				}
				_, _ = cmd.OutOrStdout().Write(b)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "(logs available only on a running helixcode instance via /mcp logs)")
			return nil
		},
	}
}

// runOAuthInteractive performs the PKCE flow against the named server.
func runOAuthInteractive(ctx context.Context, configPath, name string, out interface{ Write([]byte) (int, error) }) error {
	cfg, err := loadOrEmpty(configPath)
	if err != nil {
		return err
	}
	var spec *mcp.ServerSpec
	for i := range cfg.Servers {
		if cfg.Servers[i].Name == name {
			spec = &cfg.Servers[i]
			break
		}
	}
	if spec == nil {
		return fmt.Errorf("mcp: server %q not found", name)
	}
	if !spec.OAuth.Enabled {
		return fmt.Errorf("mcp: server %q has oauth.enabled=false", name)
	}
	authEP := spec.OAuth.AuthEndpoint
	tokEP := spec.OAuth.TokenEndpoint
	if authEP == "" || tokEP == "" {
		base := spec.OAuth.IssuerURL
		if base == "" {
			base = spec.URL
		}
		md, err := mcp.DiscoverAS(ctx, base)
		if err != nil {
			return err
		}
		if authEP == "" {
			authEP = md.AuthorizationEndpoint
		}
		if tokEP == "" {
			tokEP = md.TokenEndpoint
		}
	}
	verifier, challenge, err := mcpGeneratePKCE()
	if err != nil {
		return err
	}
	state, err := mcpGenerateState()
	if err != nil {
		return err
	}
	port, listener, err := allocLoopbackListener()
	if err != nil {
		return err
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	authURL := mcp.BuildAuthorizationURL(mcp.AuthRequest{
		AuthorizationEndpoint: authEP,
		ClientID:              spec.OAuth.ClientID,
		RedirectURI:           redirectURI,
		Scope:                 spec.OAuth.Scope,
		State:                 state,
		CodeChallenge:         challenge,
	})
	fmt.Fprintf(out, "open this URL in your browser:\n  %s\n", authURL)
	_ = openBrowser(authURL)

	code, err := waitForCallback(ctx, listener, state)
	if err != nil {
		return err
	}
	tok, err := mcp.ExchangeCode(ctx, tokEP, code, verifier, spec.OAuth.ClientID, redirectURI)
	if err != nil {
		return err
	}
	dir, err := tokenCacheDir()
	if err != nil {
		return err
	}
	tc := &mcp.TokenCache{Dir: dir}
	if err := tc.Save(name, tok); err != nil {
		return err
	}
	fmt.Fprintf(out, "saved token for %s\n", name)
	return nil
}

func mcpGeneratePKCE() (string, string, error) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	v := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(v))
	return v, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func mcpGenerateState() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func allocLoopbackListener() (int, net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return port, ln, nil
}

func waitForCallback(ctx context.Context, ln net.Listener, wantState string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("state") != wantState {
				http.Error(w, "state mismatch", 400)
				resCh <- result{err: fmt.Errorf("oauth callback: state mismatch")}
				return
			}
			if eqe := q.Get("error"); eqe != "" {
				http.Error(w, eqe, 400)
				resCh <- result{err: fmt.Errorf("oauth callback error: %s", eqe)}
				return
			}
			code := q.Get("code")
			fmt.Fprintln(w, "authorization received; you can close this tab")
			resCh <- result{code: code}
		}),
	}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-resCh:
		if r.err != nil {
			return "", r.err
		}
		if r.code == "" {
			return "", errors.New("oauth callback: empty code")
		}
		return r.code, nil
	}
}

func openBrowser(u string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", u).Start()
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	}
	return fmt.Errorf("unsupported platform for browser open")
}

func tokenCacheDir() (string, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "helixcode", "mcp", "tokens"), nil
}
