package mcp

import (
	stdctx "context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML schema for .helixcode/mcp.yml.
type Config struct {
	Servers []ServerSpec `yaml:"servers"`
}

// ServerSpec defines one MCP server.
type ServerSpec struct {
	Name       string            `yaml:"name"`
	Transport  TransportType     `yaml:"transport"`
	Command    []string          `yaml:"command,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Cwd        string            `yaml:"cwd,omitempty"`
	URL        string            `yaml:"url,omitempty"`
	SSEURL     string            `yaml:"sseURL,omitempty"`
	OAuth      OAuthSpec         `yaml:"oauth,omitempty"`
	AlwaysLoad bool              `yaml:"alwaysLoad,omitempty"`
	// ReadOnly marks every tool this server exposes as a pure read
	// (approval.LevelReadOnly) when registered with the agent tool
	// registry. Set it for servers that only expose non-mutating tools
	// (e.g. a filesystem server limited to read_file/list_directory/search)
	// so the ReadOnlyOnly agent tool loop is allowed to offer + execute them.
	// When false (default), the server's tools keep the conservative
	// LevelEdit default and are blocked by a read-only-only loop.
	ReadOnly bool `yaml:"readOnly,omitempty"`

	// rawEnv preserves the ORIGINAL (unexpanded) values of Env / URL / SSEURL /
	// Cwd / Command exactly as they appeared on disk — before any ${ENV}
	// reference was expanded into a plaintext runtime value. SaveConfig writes
	// these originals back so an expanded plaintext secret (CONST-042/§11.4.10)
	// is NEVER persisted to disk. Unexported + non-serialised: it is recomputed
	// on every LoadConfig and never marshalled into the YAML itself.
	rawEnv     map[string]string `yaml:"-"`
	rawURL     string            `yaml:"-"`
	rawSSEURL  string            `yaml:"-"`
	rawCwd     string            `yaml:"-"`
	rawCommand []string          `yaml:"-"`
	// rawSet marks that the raw originals above were captured from disk (so
	// SaveConfig knows it may safely restore them). Programmatically-built
	// configs (e.g. via the CLI) leave it false and are saved as-is.
	rawSet bool `yaml:"-"`
}

// OAuthSpec describes the OAuth configuration for a server.
type OAuthSpec struct {
	Enabled       bool   `yaml:"enabled,omitempty"`
	ClientID      string `yaml:"clientID,omitempty"`
	Scope         string `yaml:"scope,omitempty"`
	IssuerURL     string `yaml:"issuerURL,omitempty"`
	AuthEndpoint  string `yaml:"authEndpoint,omitempty"`
	TokenEndpoint string `yaml:"tokenEndpoint,omitempty"`
}

var envRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv returns the expanded string and a list of any env var names that
// were referenced but not set. Missing vars expand to "" (compatible with the
// historical behaviour) and the list lets the caller emit a single warning.
func expandEnv(s string) (string, []string) {
	var missing []string
	out := envRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[2 : len(m)-1]
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		missing = append(missing, key)
		return ""
	})
	return out, missing
}

// LoadConfig reads and validates a single YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp config: read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mcp config: parse %s: %w", path, err)
	}
	missing := map[string]bool{}
	expand := func(s string) string {
		out, miss := expandEnv(s)
		for _, k := range miss {
			missing[k] = true
		}
		return out
	}
	for i := range cfg.Servers {
		s := &cfg.Servers[i]
		// Capture the ORIGINAL (unexpanded) values before expansion so
		// SaveConfig can write the ${ENV} references back instead of the
		// expanded plaintext secret (CONST-042/§11.4.10). ${ENV} expansion
		// then happens only into the live runtime fields.
		s.rawURL = s.URL
		s.rawSSEURL = s.SSEURL
		s.rawCwd = s.Cwd
		s.rawCommand = append([]string(nil), s.Command...)
		s.rawEnv = make(map[string]string, len(s.Env))
		for k, v := range s.Env {
			s.rawEnv[k] = v
		}
		s.rawSet = true

		s.URL = expand(s.URL)
		s.SSEURL = expand(s.SSEURL)
		s.Cwd = expand(s.Cwd)
		for j, c := range s.Command {
			s.Command[j] = expand(c)
		}
		for k, v := range s.Env {
			s.Env[k] = expand(v)
		}
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		log.Print(tr(stdctx.Background(), "internal_mcp_config_env_vars_unset", map[string]any{"Path": path, "Keys": keys}))
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadMerged loads userPath then projectPath, with project overriding by name.
// Either path may be empty or non-existent.
func LoadMerged(userPath, projectPath string) (*Config, error) {
	merged := &Config{}
	addAll := func(c *Config) {
		for _, s := range c.Servers {
			merged.Servers = append(merged.Servers, s)
		}
	}
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			c, err := LoadConfig(userPath)
			if err != nil {
				return nil, err
			}
			addAll(c)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err == nil {
			c, err := LoadConfig(projectPath)
			if err != nil {
				return nil, err
			}
			byName := map[string]bool{}
			for _, s := range c.Servers {
				byName[s.Name] = true
			}
			filtered := make([]ServerSpec, 0, len(merged.Servers))
			for _, s := range merged.Servers {
				if !byName[s.Name] {
					filtered = append(filtered, s)
				}
			}
			merged.Servers = filtered
			addAll(c)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return merged, nil
}

// SaveConfig writes the config back to YAML at path.
//
// Two CONST-042/§11.4.10 defense-in-depth guarantees:
//  1. The file is written mode 0600 (owner-only) so an MCP config can never be
//     world-readable.
//  2. For any server loaded from disk, the ORIGINAL (unexpanded) ${ENV}
//     references are written back instead of the expanded plaintext values, so
//     a load->save round-trip never persists a secret to disk. Servers built
//     programmatically (rawSet == false) are saved as-is.
func SaveConfig(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	out := cfg.forPersist()
	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// forPersist returns a copy of the config in which every server loaded from
// disk has its expanded ${ENV} values replaced by the original unexpanded
// references captured at load time. The receiver is never mutated.
func (c *Config) forPersist() *Config {
	out := &Config{Servers: make([]ServerSpec, len(c.Servers))}
	for i, s := range c.Servers {
		if s.rawSet {
			s.URL = s.rawURL
			s.SSEURL = s.rawSSEURL
			s.Cwd = s.rawCwd
			s.Command = append([]string(nil), s.rawCommand...)
			if s.rawEnv != nil {
				env := make(map[string]string, len(s.rawEnv))
				for k, v := range s.rawEnv {
					env[k] = v
				}
				s.Env = env
			}
		}
		out.Servers[i] = s
	}
	return out
}

// Validate checks the config for required fields.
func (c *Config) Validate() error {
	seen := map[string]bool{}
	for i, s := range c.Servers {
		if s.Name == "" {
			return fmt.Errorf("mcp config: server %d: empty name", i)
		}
		if seen[s.Name] {
			return fmt.Errorf("mcp config: duplicate server name %q", s.Name)
		}
		seen[s.Name] = true
		if err := s.Transport.Validate(); err != nil {
			return fmt.Errorf("mcp config: server %s: %w", s.Name, err)
		}
		switch s.Transport {
		case TransportStdio:
			if len(s.Command) == 0 {
				return fmt.Errorf("mcp config: server %s: stdio requires command", s.Name)
			}
		case TransportHTTP, TransportSSE, TransportWS:
			if s.URL == "" {
				return fmt.Errorf("mcp config: server %s: %s requires url", s.Name, s.Transport)
			}
		}
	}
	return nil
}
