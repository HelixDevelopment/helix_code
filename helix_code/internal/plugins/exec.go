package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sandboxDir is the root under which per-plugin sandbox directories are created.
// It is a var (not a const) only so security tests can redirect it into an
// isolated t.TempDir() — production never reassigns it.
var sandboxDir = "/tmp/helixcode-plugin-sandbox"

func ExecutePlugin(ctx context.Context, plugin Plugin, action string, args []string) (string, error) {
	manifest, ok := plugin.(*BasePlugin)
	if !ok {
		return "", fmt.Errorf("unsupported plugin type")
	}
	name := manifest.Name()

	// DEFENSE-IN-DEPTH (SECURITY HIGH): the plugin name is used to construct
	// on-disk paths below. Even though Manifest.Validate() rejects unsafe names,
	// a Plugin may reach here without having passed Validate (e.g. a BasePlugin
	// constructed directly). Re-validate the name as a single safe segment so an
	// unsanitized name can never traverse out of the sandbox / plugins tree.
	if !safePluginNameRegex.MatchString(name) {
		return "", fmt.Errorf("unsafe plugin name %q: must match %s", name, safePluginNameRegex.String())
	}

	pluginDir := filepath.Join(sandboxDir, name)
	// Verify the resolved sandbox sub-dir stays inside sandboxDir before any
	// MkdirAll, so a hypothetical escaping name cannot create an out-of-sandbox
	// directory.
	if !withinBase(sandboxDir, pluginDir) {
		return "", fmt.Errorf("plugin sandbox path %q escapes sandbox root %q", pluginDir, sandboxDir)
	}
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", fmt.Errorf("create plugin sandbox dir %q: %w", pluginDir, err)
	}
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
	// Verify the resolved entrypoint stays inside the intended plugins root.
	// Belt-and-suspenders: refuse to exec anything that escapes the plugins
	// tree even if a Plugin bypassed Validate.
	absPluginsRoot, rootErr := filepath.Abs("plugins")
	if rootErr != nil {
		return "", fmt.Errorf("plugins root resolve: %w", rootErr)
	}
	if !withinBase(absPluginsRoot, absEntrypoint) {
		return "", fmt.Errorf("plugin entrypoint %q escapes plugins root %q", absEntrypoint, absPluginsRoot)
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

// withinBase reports whether target (after Clean) is base itself or a
// descendant of base. Both arguments should be in the same domain (both
// absolute or both relative). It is a path-traversal guard: a target that
// resolves outside base via ".." segments yields a filepath.Rel result that
// starts with ".." (or an error), which is rejected.
func withinBase(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	if target == base {
		return true
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	// An absolute rel (shouldn't normally happen for same-domain paths) is also
	// outside base.
	return !filepath.IsAbs(rel)
}
