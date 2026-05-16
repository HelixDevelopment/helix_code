package plugins

import "context"

type BasePlugin struct {
	PluginName    string
	PluginVersion string
	PluginTools   []string
	PluginHooks   []string
}

func (p *BasePlugin) Name() string                           { return p.PluginName }
func (p *BasePlugin) Version() string                         { return p.PluginVersion }
func (p *BasePlugin) Init(ctx context.Context) error          { return nil }
func (p *BasePlugin) Shutdown(ctx context.Context) error      { return nil }
func (p *BasePlugin) Tools() []string                         { return p.PluginTools }
func (p *BasePlugin) Hooks() []string                         { return p.PluginHooks }
