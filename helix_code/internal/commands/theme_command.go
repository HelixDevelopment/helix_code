// Package commands — theme_command.go (P1-F20-T07).
//
// ThemeCommand implements the /theme slash command with three subcommands:
// status (default), list, show <name>. It is the user-facing surface for
// HelixCode's F20 theme system.
//
// Subcommands:
//
//	/theme              alias of /theme status
//	/theme status       current theme name + depth + active source
//	                    (env / COLORFGBG / default) + custom-loaded indicator
//	/theme list         all available theme names (built-ins + custom)
//	/theme show <name>  render the theme's palette: one sample line per
//	                    role, stylized with that role's color, so the
//	                    operator sees what each role looks like under that
//	                    theme + the active terminal depth.
//
// Anti-bluff contract: /theme show MUST construct a real theme.Styler from
// the requested theme + the active depth and run real text through
// Stylize(). There is no fake-output path. The fake registry used by tests
// is a hexagonal seam that returns real *theme.Theme values; production
// wiring (main.go) hands the command the real *theme.ThemeRegistry.
package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/theme"
)

// ThemeInspector is the subset of *theme.ThemeRegistry that ThemeCommand
// depends on.
//
// Defining the interface in the commands package keeps the slash command
// testable with a fake while still letting main.go pass the real
// *theme.ThemeRegistry directly (Go satisfies interfaces structurally).
//
// Deliberately narrow: only Get / Names / Custom are exposed. /theme is
// observation-only — it never mutates the registry, never reloads YAML.
type ThemeInspector interface {
	Get(name theme.ThemeName) (theme.Theme, error)
	Names() []theme.ThemeName
	Custom() *theme.Theme
}

// ThemeSourceEnv / ThemeSourceCOLORFGBG / ThemeSourceDefault are the three
// canonical CONST-046 message IDs for the "active source" line in
// /theme status. ResolveThemeSource returns one of these IDs at startup;
// handleStatus resolves it to locale-aware display text via tr() so the
// rendered /theme status line adapts to the operator's language.
const (
	ThemeSourceEnv       = "internal_commands_theme_source_env"
	ThemeSourceCOLORFGBG = "internal_commands_theme_source_colorfgbg"
	ThemeSourceDefault   = "internal_commands_theme_source_default"
)

// ResolveThemeSource returns the human-readable name of the signal that
// drove DetectThemeName at startup. It mirrors detect.go's resolution
// order without re-implementing parsing — callers only get back the
// discriminator string for display.
//
// Pure function; takes envLookup for testability. Production callers pass
// os.Getenv. Pragmatic v1 helper: this avoids refactoring DetectThemeName
// to also return its source.
func ResolveThemeSource(envLookup func(string) string) string {
	if raw := envLookup(theme.ThemeNameEnvVar); raw != "" {
		// Match detect.go's accept set so a garbage value (e.g. "banana")
		// does NOT report ThemeSourceEnv — it would fall through to the
		// next signal and the displayed source would be wrong.
		switch theme.ThemeName(raw) {
		case theme.ThemeDark, theme.ThemeLight, theme.ThemeNone:
			return ThemeSourceEnv
		}
	}
	if v := envLookup("COLORFGBG"); v != "" {
		// Conservative: only claim COLORFGBG drove the decision when the
		// value has at least one ';' separator (the fg;bg shape detect.go
		// requires to parse). Anything else falls through to default.
		if strings.Contains(v, ";") {
			return ThemeSourceCOLORFGBG
		}
	}
	return ThemeSourceDefault
}

// ThemeCommand is the /theme slash command.
type ThemeCommand struct {
	registry     ThemeInspector
	activeName   theme.ThemeName
	activeDepth  theme.ColorDepth
	activeSource string
	// styler is the active styler used for /theme status formatting. It is
	// optional — /theme show always builds its own per-call styler so it
	// can render the *requested* theme's palette regardless of which one
	// is active.
	styler *theme.Styler
}

// NewThemeCommand constructs the /theme slash command. activeSource is
// informational only and appears in /theme status. The supplied styler is
// the same one used by handleGenerate (via main.go) — it is NOT used to
// build /theme show output (that uses a per-call styler bound to the
// requested theme).
func NewThemeCommand(registry ThemeInspector, name theme.ThemeName, depth theme.ColorDepth, source string, styler *theme.Styler) *ThemeCommand {
	return &ThemeCommand{
		registry:     registry,
		activeName:   name,
		activeDepth:  depth,
		activeSource: source,
		styler:       styler,
	}
}

// Name returns the slash command name (without the leading slash).
func (c *ThemeCommand) Name() string { return "theme" }

// Aliases returns alternative invocation names. /theme has none.
func (c *ThemeCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *ThemeCommand) Description() string {
	return tr(context.Background(), "internal_commands_theme_description", nil)
}

// Usage returns the usage string shown by /help.
func (c *ThemeCommand) Usage() string {
	return tr(context.Background(), "internal_commands_theme_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — it answers "what theme is
// active and what's driving the choice" which is the most common entry-
// point question.
func (c *ThemeCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return &CommandResult{Success: true, Output: c.handleStatus(ctx)}, nil
	case "list":
		return &CommandResult{Success: true, Output: c.handleList(ctx)}, nil
	case "show":
		out, err := c.handleShow(ctx, args[1:])
		if err != nil {
			return nil, err
		}
		return &CommandResult{Success: true, Output: out}, nil
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_theme_err_unknown_subcommand", map[string]any{"Sub": sub}))
	}
}

// handleStatus renders the active-theme block.
//
// The block always shows: name, depth, source. The "Custom" line reports
// whether a user-loaded YAML theme is registered and, when one is, its
// name. This is informational so an operator running /theme status can see
// at a glance whether their theme.yaml took effect.
func (c *ThemeCommand) handleStatus(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_theme_status_header", nil) + "\n")

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_theme_label_name", nil), string(c.activeName))
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_theme_label_depth", nil), c.activeDepth.String())
	sourceID := c.activeSource
	if sourceID == "" {
		sourceID = ThemeSourceDefault
	}
	// activeSource holds a CONST-046 message ID (ThemeSource* constants);
	// resolve to locale-aware display text here.
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_theme_label_source", nil), tr(ctx, sourceID, nil))

	customLine := tr(ctx, "internal_commands_theme_custom_none", nil)
	if c.registry != nil {
		if cust := c.registry.Custom(); cust != nil {
			customLine = tr(ctx, "internal_commands_theme_custom_loaded", map[string]any{"Name": string(cust.Name)})
		}
	}
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_theme_label_custom", nil), customLine)
	tw.Flush()
	return sb.String()
}

// handleList renders the available-themes block.
//
// Built-ins are tagged "(built-in)"; the user-loaded custom theme (if any)
// is tagged "(user, loaded from theme.yaml)" so the operator can see which
// entry came from disk.
func (c *ThemeCommand) handleList(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_theme_list_header", nil) + "\n")

	if c.registry == nil {
		sb.WriteString(tr(ctx, "internal_commands_theme_registry_unavailable", nil) + "\n")
		return sb.String()
	}

	var custom *theme.Theme = c.registry.Custom()
	for _, n := range c.registry.Names() {
		tag := tr(ctx, "internal_commands_theme_tag_builtin", nil)
		if custom != nil && n == custom.Name {
			// Names() may include the custom theme's name; mark it as such.
			if !n.IsValid() {
				tag = tr(ctx, "internal_commands_theme_tag_user_loaded", nil)
			} else {
				// Edge case: custom theme reuses a built-in name (e.g.
				// "dark"). Get() will resolve to the custom theme; we tag
				// it as user-loaded so the operator knows the built-in
				// got overridden.
				tag = tr(ctx, "internal_commands_theme_tag_user_override", nil)
			}
		}
		fmt.Fprintf(&sb, "  - %s %s\n", string(n), tag)
	}
	return sb.String()
}

// handleShow renders one sample line per role for the requested theme,
// stylized with that role's color at the active depth.
//
// Rationale: showing "info" / "warn" / "error" / "highlight" / "dim" with
// each role's actual ANSI bytes lets the operator see what their terminal
// will render — verifying both the palette and the depth detection in one
// glance. The styler is built per-call from the requested theme + the
// active depth so the output reflects the operator's terminal capability,
// not a hardcoded depth.
func (c *ThemeCommand) handleShow(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%s", tr(ctx, "internal_commands_theme_err_show_missing_name", nil))
	}
	if c.registry == nil {
		return "", fmt.Errorf("%s", tr(ctx, "internal_commands_theme_err_show_registry_unavailable", nil))
	}
	name := theme.ThemeName(args[0])
	t, err := c.registry.Get(name)
	if err != nil {
		return "", fmt.Errorf("%s: %w", tr(ctx, "internal_commands_theme_err_show_prefix", nil), err)
	}

	// Per-call styler bound to the REQUESTED theme + the ACTIVE depth.
	// This is the load-bearing decision: the operator wants to see the
	// requested theme's palette as their terminal would render it, not as
	// a different theme's palette renders it.
	styler := theme.NewStyler(t, c.activeDepth)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", tr(ctx, "internal_commands_theme_show_heading", map[string]any{
		"Name":  string(t.Name),
		"Depth": c.activeDepth.String(),
	}))
	for _, role := range theme.AllRoles() {
		sample := tr(ctx, "internal_commands_theme_sample_text", map[string]any{"Role": string(role)})
		styled := styler.Stylize(role, sample)
		fmt.Fprintf(&sb, "  %-10s %s\n", string(role), styled)
	}
	return sb.String(), nil
}
