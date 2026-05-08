package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Loader struct {
	pluginDir string
	plugins   map[string]Plugin
	mu        sync.RWMutex
}

func NewLoader(pluginDir string) *Loader {
	return &Loader{
		pluginDir: pluginDir,
		plugins:   make(map[string]Plugin),
	}
}

func (l *Loader) Load(ctx context.Context, manifestPath string) (Plugin, error) {
	manifest, err := ParseManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	for _, dep := range manifest.Dependencies {
		if _, ok := l.plugins[dep]; !ok {
			return nil, fmt.Errorf("missing dependency: %s", dep)
		}
	}
	plugin := &BasePlugin{
		PluginName:    manifest.Name,
		PluginVersion: manifest.Version,
		PluginTools:   manifest.Capabilities,
	}
	if err := plugin.Init(ctx); err != nil {
		return nil, fmt.Errorf("init plugin: %w", err)
	}
	l.mu.Lock()
	l.plugins[manifest.Name] = plugin
	l.mu.Unlock()
	return plugin, nil
}

func (l *Loader) LoadAll(ctx context.Context) ([]Plugin, error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var loaded []Plugin
	for _, entry := range entries {
		if entry.IsDir() {
			manifestPath := filepath.Join(l.pluginDir, entry.Name(), "manifest.yaml")
			if _, err := os.Stat(manifestPath); err == nil {
				p, err := l.Load(ctx, manifestPath)
				if err != nil {
					continue
				}
				loaded = append(loaded, p)
			}
		}
	}
	return loaded, nil
}

func (l *Loader) Get(name string) (Plugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.plugins[name]
	return p, ok
}

func (l *Loader) List() []Plugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Plugin, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, p)
	}
	return result
}

func (l *Loader) Unload(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	p.Shutdown(context.Background())
	delete(l.plugins, name)
	return nil
}
