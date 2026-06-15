package plugins

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// safePluginNameRegex constrains a plugin name to a single safe path segment.
// A plugin name is used to construct on-disk paths (sandbox dir, entrypoint
// path); permitting `/`, `\`, `..`, leading `.`, NUL or any path separator
// would allow path-traversal → arbitrary out-of-tree binary execution and
// sandbox escape (SECURITY HIGH). The allowlist matches loader/activation's
// pluginInvokeRegex token shape (`^[A-Za-z0-9_-]+$`).
var safePluginNameRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
	if !safePluginNameRegex.MatchString(m.Name) {
		return fmt.Errorf("manifest name %q is unsafe: a plugin name must be a single safe segment matching %s (no '/', '\\', '..', leading '.', NUL or path separators)", m.Name, safePluginNameRegex.String())
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
