// Package commands — telemetry_command.go (P1-F16-T09).
//
// TelemetryCommand implements the /telemetry slash command with three
// subcommands: status (default), show, flush. It is the user-facing
// surface for HelixCode's F16 OpenTelemetry observability feature.
//
// Subcommands:
//
//	/telemetry          alias of /telemetry status
//	/telemetry status   exporter + endpoint + service name + resource attrs +
//	                    blocked-attribute-key count + batch/export timeouts
//	/telemetry show     same as status with the full blocked-keys list
//	                    (default-deny floor + user-supplied additions)
//	/telemetry flush    calls TelemetryProvider.ForceFlush(ctx); reports
//	                    duration on success or surfaces the error
//
// Anti-bluff contract: /telemetry flush MUST call the provider's real
// ForceFlush. There is no fake-output path. The fake provider used in
// tests is a hexagonal seam — production wiring (T10) hands the command
// the real *telemetry.RealTelemetryProvider.
//
// CONST-042 anchor: status/show output renders only metadata
// (exporter kind, endpoint, service name, resource-attribute keys, the
// deny-list itself). It NEVER renders span attribute VALUES, prompt
// bodies, or credentials. The deny-list count + show listing exist
// precisely so operators can audit the secret floor without ever needing
// to fish raw spans out of stdout.
package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"dev.helix.code/internal/telemetry"
)

// TelemetryProvider is the subset of *telemetry.RealTelemetryProvider
// that TelemetryCommand depends on.
//
// Defining the interface in the commands package keeps the slash command
// testable with a fake while still letting main.go pass the real
// provider directly (Go satisfies interfaces structurally).
//
// Deliberately narrow: only Config / Exporter / ForceFlush are exposed.
// The slash command is observation + a single drain action; it never
// constructs spans or shuts the provider down — those belong to the
// instrumentation decorators (T06/T07/T08) and main.go's lifecycle
// (T10) respectively.
type TelemetryProvider interface {
	Config() telemetry.TelemetryConfig
	Exporter() telemetry.ExporterKind
	ForceFlush(ctx context.Context) error
}

// TelemetryCommand is the /telemetry slash command.
type TelemetryCommand struct {
	provider TelemetryProvider
}

// NewTelemetryCommand constructs the /telemetry slash command. A nil
// provider is allowed: every subcommand reports "telemetry unavailable"
// in that case so the CLI keeps working when telemetry could not be
// constructed (e.g. exporter init failed during boot).
func NewTelemetryCommand(p TelemetryProvider) *TelemetryCommand {
	return &TelemetryCommand{provider: p}
}

// Name returns the slash command name (without the leading slash).
func (c *TelemetryCommand) Name() string { return "telemetry" }

// Aliases returns alternative invocation names. /telemetry has none.
func (c *TelemetryCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *TelemetryCommand) Description() string {
	return "Inspect telemetry status, list blocked attribute keys, or force-flush the exporter."
}

// Usage returns the usage string shown by /help.
func (c *TelemetryCommand) Usage() string {
	return "/telemetry [status|show|flush]"
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — it answers "is telemetry
// running and where is it sending data" which is the most common
// entry-point question.
func (c *TelemetryCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return &CommandResult{Success: true, Output: c.handleStatus(false)}, nil
	case "show":
		return &CommandResult{Success: true, Output: c.handleStatus(true)}, nil
	case "flush":
		return c.handleFlush(ctx)
	default:
		return nil, fmt.Errorf("/telemetry: unknown subcommand %q (want status|show|flush)", sub)
	}
}

// handleStatus renders the telemetry status block.
//
// When verbose=true (called from /telemetry show) we additionally render
// the full effective deny-list (default floor + user-supplied additions)
// as bullet points so operators can audit CONST-042 coverage.
//
// When the provider is nil OR the active exporter is ExporterNoop we
// short-circuit with a single "Telemetry unavailable: <reason>" line —
// the rest of the table would be misleading (it would imply telemetry
// is wired when it's actually a no-op).
func (c *TelemetryCommand) handleStatus(verbose bool) string {
	if c.provider == nil {
		return unavailableMessage("provider not initialised") + "\n"
	}
	exp := c.provider.Exporter()
	if exp == telemetry.ExporterNoop {
		return unavailableMessage("no exporter configured (set OTEL_EXPORTER_OTLP_PROTOCOL or OTEL_TRACES_EXPORTER=console to enable)") + "\n"
	}

	cfg := c.provider.Config()

	var sb strings.Builder
	sb.WriteString("Telemetry status\n")

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Enabled:\t%t\n", cfg.Enabled)
	fmt.Fprintf(tw, "  Exporter:\t%s\n", string(exp))

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = telemetry.DefaultServiceName
	}
	fmt.Fprintf(tw, "  Service name:\t%s\n", serviceName)

	endpoint := cfg.Endpoint
	if endpoint == "" {
		switch exp {
		case telemetry.ExporterStdout:
			endpoint = "(n/a for stdout)"
		default:
			endpoint = "(default)"
		}
	}
	fmt.Fprintf(tw, "  Endpoint:\t%s\n", endpoint)

	fmt.Fprintf(tw, "  Resource attrs:\t%s\n", formatResourceAttrs(cfg.ResourceAttrs))

	defaultCount := len(telemetry.DefaultBlockedAttributeKeys)
	userCount := len(cfg.BlockedAttributeKeys)
	totalCount := defaultCount + userCount
	fmt.Fprintf(tw, "  Blocked attr keys:\t%d (default %d + %d user)\n",
		totalCount, defaultCount, userCount)

	fmt.Fprintf(tw, "  Batch timeout:\t%s\n", durationOrDefault(cfg.BatchTimeout, telemetry.DefaultBatchTimeout))
	fmt.Fprintf(tw, "  Export timeout:\t%s\n", durationOrDefault(cfg.ExportTimeout, telemetry.DefaultExportTimeout))
	fmt.Fprintf(tw, "  Insecure:\t%t\n", cfg.Insecure)
	tw.Flush()

	if verbose {
		sb.WriteString("\nBlocked attribute keys (default-deny CONST-042):\n")
		for _, k := range telemetry.DefaultBlockedAttributeKeys {
			fmt.Fprintf(&sb, "  - %s\n", k)
		}
		if userCount > 0 {
			sb.WriteString("\nUser-supplied blocked attribute keys:\n")
			for _, k := range cfg.BlockedAttributeKeys {
				fmt.Fprintf(&sb, "  - %s\n", k)
			}
		}
	}

	return sb.String()
}

// handleFlush asks the provider to drain buffered spans/metrics and
// reports either the wall-clock duration on success or the underlying
// error on failure.
//
// When the provider is nil OR the active exporter is ExporterNoop there
// is nothing to flush; we report "unavailable" without invoking
// ForceFlush (calling ForceFlush on a noop provider would succeed but
// the report would be misleading — there is no buffer to drain).
func (c *TelemetryCommand) handleFlush(ctx context.Context) (*CommandResult, error) {
	if c.provider == nil {
		return &CommandResult{
			Success: true,
			Output:  "telemetry unavailable: provider not initialised",
		}, nil
	}
	if c.provider.Exporter() == telemetry.ExporterNoop {
		return &CommandResult{
			Success: true,
			Output:  "telemetry unavailable: no exporter configured",
		}, nil
	}

	start := time.Now()
	if err := c.provider.ForceFlush(ctx); err != nil {
		return nil, fmt.Errorf("/telemetry flush: %w", err)
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("telemetry flushed in %s", elapsed),
	}, nil
}

// unavailableMessage formats the canonical "telemetry unavailable: ..."
// banner used by every disabled-path branch so the wording stays
// consistent across status / show / flush.
func unavailableMessage(reason string) string {
	return "Telemetry unavailable: " + reason
}

// formatResourceAttrs renders the resource-attributes map as a stable
// "k=v k=v" string. Keys are sorted alphabetically so the output is
// reproducible across runs (Go map iteration order is randomised).
func formatResourceAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+attrs[k])
	}
	return strings.Join(parts, " ")
}

// durationOrDefault returns d.String() when d > 0, otherwise the package
// default's String(). Mirrors applyDefaults in provider.go so /telemetry
// status reports the values the SDK actually uses, not raw zeros from
// the config struct.
func durationOrDefault(d, fallback time.Duration) string {
	if d <= 0 {
		return fallback.String()
	}
	return d.String()
}
