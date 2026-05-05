package mcp

import (
	"errors"
	"fmt"
	"os"
	"regexp"

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

func expandEnv(s string) string {
	return envRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[2 : len(m)-1]
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		return ""
	})
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
	for i := range cfg.Servers {
		s := &cfg.Servers[i]
		s.URL = expandEnv(s.URL)
		s.SSEURL = expandEnv(s.SSEURL)
		s.Cwd = expandEnv(s.Cwd)
		for j, c := range s.Command {
			s.Command[j] = expandEnv(c)
		}
		for k, v := range s.Env {
			s.Env[k] = expandEnv(v)
		}
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
			filtered := merged.Servers[:0]
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
func SaveConfig(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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
