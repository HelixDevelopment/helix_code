package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	content := []byte(`
name: test-plugin
version: 1.0.0
description: A test plugin
author: test
entrypoint: main
`)
	os.WriteFile(manifestPath, content, 0644)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("expected test-plugin, got %s", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", m.Version)
	}
}

func TestParseManifest_Invalid(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte("name: ''\nversion: ''\nentrypoint: ''"), 0644)
	_, err := ParseManifest(manifestPath)
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestLoader_Load(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(manifestPath, []byte("name: my-plugin\nversion: 0.1.0\nentrypoint: plugin.sh"), 0644)
	loader := NewLoader(dir)
	p, err := loader.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "my-plugin" {
		t.Errorf("expected my-plugin, got %s", p.Name())
	}
}

func TestLoader_GetAndList(t *testing.T) {
	loader := NewLoader(t.TempDir())
	p := &BasePlugin{PluginName: "test", PluginVersion: "1.0"}
	loader.mu.Lock()
	loader.plugins["test"] = p
	loader.mu.Unlock()
	got, ok := loader.Get("test")
	if !ok {
		t.Fatal("expected to find plugin")
	}
	if got.Name() != "test" {
		t.Errorf("expected test, got %s", got.Name())
	}
	plugins := loader.List()
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestLoader_Unload(t *testing.T) {
	loader := NewLoader(t.TempDir())
	loader.mu.Lock()
	loader.plugins["test"] = &BasePlugin{PluginName: "test"}
	loader.mu.Unlock()
	err := loader.Unload("test")
	if err != nil {
		t.Fatal(err)
	}
	_, ok := loader.Get("test")
	if ok {
		t.Error("expected plugin to be removed")
	}
}

func TestRegistry_RegisterAndList(t *testing.T) {
	r := NewRegistry()
	p := &BasePlugin{PluginName: "registry-test", PluginVersion: "1.0"}
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	plugins := r.List()
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
	got, ok := r.Get("registry-test")
	if !ok {
		t.Fatal("expected to find plugin")
	}
	if got.Name() != "registry-test" {
		t.Errorf("expected registry-test, got %s", got.Name())
	}
}

func TestRegistry_GetToolPlugins(t *testing.T) {
	r := NewRegistry()
	p := &BasePlugin{
		PluginName:  "tools-plugin",
		PluginTools: []string{"fs_read", "shell"},
	}
	r.Register(p)
	toolPlugins := r.GetToolPlugins("shell")
	if len(toolPlugins) != 1 {
		t.Errorf("expected 1 plugin for shell tool, got %d", len(toolPlugins))
	}
}

func TestBasePlugin(t *testing.T) {
	p := &BasePlugin{
		PluginName:    "base",
		PluginVersion: "1.0",
		PluginTools:   []string{"tool1"},
		PluginHooks:   []string{"hook1"},
	}
	if p.Name() != "base" || p.Version() != "1.0" {
		t.Error("unexpected name or version")
	}
	if len(p.Tools()) != 1 || p.Tools()[0] != "tool1" {
		t.Error("unexpected tools")
	}
	if err := p.Init(context.Background()); err != nil {
		t.Error("Init should succeed")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Error("Shutdown should succeed")
	}
}
