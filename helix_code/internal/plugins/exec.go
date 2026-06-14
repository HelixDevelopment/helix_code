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
	// Resolve the entrypoint to an ABSOLUTE path. The execution runs with
	// cmd.Dir set to the sandbox directory (cwd isolation), and Go resolves a
	// RELATIVE cmd.Path against cmd.Dir — so a relative "plugins/<name>/main"
	// would be looked up inside the sandbox and never found. Resolving against
	// the process CWD up-front makes the entrypoint location independent of the
	// sandboxed working directory.
	entrypoint := filepath.Join("plugins", name, "main")
	absEntrypoint, absErr := filepath.Abs(entrypoint)
	if absErr != nil {
		return "", fmt.Errorf("plugin entrypoint resolve: %w", absErr)
	}
	if _, err := os.Stat(absEntrypoint); os.IsNotExist(err) {
		msg := tr(ctx, "internal_plugins_sandbox_entrypoint_not_found", map[string]any{
			"Name":       name,
			"Entrypoint": entrypoint,
		})
		return msg, fmt.Errorf("plugin entrypoint not found: %s", absEntrypoint)
	}
	cmd := exec.CommandContext(ctx, absEntrypoint, action)
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = pluginDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("plugin execution failed: %w\nOutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
