package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const sandboxDir = "/tmp/helixcode-plugin-sandbox"

func ExecutePlugin(ctx context.Context, plugin Plugin, action string, args []string) (string, error) {
	manifest, ok := plugin.(*BasePlugin)
	if !ok {
		return "", fmt.Errorf("unsupported plugin type")
	}
	name := manifest.Name()
	pluginDir := filepath.Join(sandboxDir, name)
	os.MkdirAll(pluginDir, 0755)
	entrypoint := filepath.Join("plugins", name, "main")
	if _, err := os.Stat(entrypoint); os.IsNotExist(err) {
		msg := tr(ctx, "internal_plugins_sandbox_entrypoint_not_found", map[string]any{
			"Name":       name,
			"Entrypoint": entrypoint,
		})
		return msg, fmt.Errorf("plugin entrypoint not found: %s", entrypoint)
	}
	cmd := exec.CommandContext(ctx, entrypoint, action)
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = pluginDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("plugin execution failed: %w\nOutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
