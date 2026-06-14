package plugins

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// LoadPlugins scans dir for plugin sub-directories (each containing a
// manifest.yaml) and loads every one into a fresh Loader, returning it. A
// front-end calls this ONCE at startup; the returned *Loader is then handed to
// MaybeRunPlugin on each user prompt. A missing dir yields an empty (non-nil)
// Loader with no error so the front-end degrades gracefully when no plugins are
// installed.
func LoadPlugins(ctx context.Context, dir string) (*Loader, error) {
	l := NewLoader(dir)
	if _, err := l.LoadAll(ctx); err != nil {
		return nil, fmt.Errorf("load plugins from %q: %w", dir, err)
	}
	return l, nil
}

// pluginInvokeRegex recognises the plugin-invocation syntax in a user prompt:
//
//	@plugin:<name> <action> [args...]
//
// <name> and <action> are token-shaped; everything after <action> is captured
// as a single raw args string (split on whitespace by the caller).
var pluginInvokeRegex = regexp.MustCompile(`^@plugin:([A-Za-z0-9_-]+)\s+([A-Za-z0-9_-]+)\s*(.*)$`)

// MaybeRunPlugin inspects a raw user prompt for the `@plugin:<name> <action>
// <args>` syntax. When the prompt matches AND the named plugin is loaded in l,
// it executes the plugin's real entrypoint via ExecutePlugin and returns
// (output, true, err). When the prompt does NOT match the syntax it returns
// ("", false, nil) so the front-end falls through to the ordinary prompt path.
//
// A prompt that matches the syntax but names an unloaded plugin returns
// ("", true, err) — ran=true signals "this WAS a plugin invocation" so the
// front-end surfaces the error rather than silently sending it to the LLM.
//
// The execution is real (os/exec via ExecutePlugin); there is no simulation.
func MaybeRunPlugin(ctx context.Context, l *Loader, prompt string) (output string, ran bool, err error) {
	m := pluginInvokeRegex.FindStringSubmatch(strings.TrimSpace(prompt))
	if m == nil {
		return "", false, nil
	}
	name := m[1]
	action := m[2]
	var args []string
	if rest := strings.TrimSpace(m[3]); rest != "" {
		args = strings.Fields(rest)
	}

	if l == nil {
		return "", true, fmt.Errorf("plugin %q invoked but no plugin loader is configured", name)
	}
	p, ok := l.Get(name)
	if !ok {
		return "", true, fmt.Errorf("plugin %q is not loaded", name)
	}

	out, execErr := ExecutePlugin(ctx, p, action, args)
	if execErr != nil {
		return "", true, fmt.Errorf("plugin %q action %q failed: %w", name, action, execErr)
	}
	return out, true, nil
}
