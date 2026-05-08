package plugins

import "context"

type Plugin interface {
	Name() string
	Version() string
	Init(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Tools() []string
	Hooks() []string
}

type Manifest struct {
	Name         string            `yaml:"name"`
	Version      string            `yaml:"version"`
	Description  string            `yaml:"description"`
	Author       string            `yaml:"author"`
	APIVersion   string            `yaml:"api_version"`
	Dependencies []string          `yaml:"dependencies"`
	Capabilities []string          `yaml:"capabilities"`
	Entrypoint   string            `yaml:"entrypoint"`
	Sandbox      bool              `yaml:"sandbox"`
	Env          map[string]string `yaml:"env"`
}
