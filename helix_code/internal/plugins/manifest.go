package plugins

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest version is required")
	}
	if m.APIVersion == "" {
		m.APIVersion = "v1"
	}
	if m.Entrypoint == "" {
		return fmt.Errorf("manifest entrypoint is required")
	}
	return nil
}
