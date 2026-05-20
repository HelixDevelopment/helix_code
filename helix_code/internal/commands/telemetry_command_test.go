// Package commands — telemetry_command_test.go (P1-F16-T09).
//
// Tests for /telemetry slash command (status/show/flush). Uses a fake
// TelemetryProvider to keep the suite hermetic — we never construct a
// real OTel SDK pipeline here. The fake records ForceFlush calls so we
// can prove /telemetry flush actually delegates (no fmt.Printf simulation).
package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/telemetry"
)

// fakeTelemetryProvider is a hexagonal-seam impl of the
// commands.TelemetryProvider interface used by /telemetry tests.
//
// It records ForceFlush calls (count + last ctx-nil flag) so the tests
// can assert that /telemetry flush actually delegates to the provider
// (anti-bluff: no fmt.Printf + sleep simulation).
type fakeTelemetryProvider struct {
	cfg          telemetry.TelemetryConfig
	exporterKind telemetry.ExporterKind
	flushErr     error

	flushCalls     int
	lastContextNil bool
}

func (f *fakeTelemetryProvider) Config() telemetry.TelemetryConfig { return f.cfg }
func (f *fakeTelemetryProvider) Exporter() telemetry.ExporterKind  { return f.exporterKind }
func (f *fakeTelemetryProvider) ForceFlush(ctx context.Context) error {
	f.flushCalls++
	f.lastContextNil = ctx == nil
	return f.flushErr
}

func newTelemetryCommand(t *testing.T) (*TelemetryCommand, *fakeTelemetryProvider) {
	t.Helper()
	prov := &fakeTelemetryProvider{
		cfg: telemetry.TelemetryConfig{
			Enabled:     true,
			Exporter:    telemetry.ExporterStdout,
			ServiceName: "helixcode",
			ResourceAttrs: map[string]string{
				"env":  "prod",
				"team": "core",
			},
			BlockedAttributeKeys: nil,
			BatchTimeout:         5 * time.Second,
			ExportTimeout:        30 * time.Second,
			Insecure:             false,
		},
		exporterKind: telemetry.ExporterStdout,
	}
	return NewTelemetryCommand(prov), prov
}

func TestTelemetryCommand_NameDescription(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	assert.Equal(t, "telemetry", c.Name())
	// CONST-046: Description/Usage route through the tr() seam; under the
	// default NoopTranslator they echo the message ID (non-empty).
	assert.NotEmpty(t, c.Description())
	assert.NotEmpty(t, c.Usage())
	assert.Nil(t, c.Aliases())
}

func TestTelemetryCommand_DefaultIsStatus(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: status header + exporter label are translated message
	// IDs under the default NoopTranslator (loud echo).
	assert.Contains(t, res.Output, "internal_commands_telemetry_status_header")
	assert.Contains(t, res.Output, "internal_commands_telemetry_label_exporter")
}

func TestTelemetryCommand_StatusShowsExporter(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "stdout")
	assert.Contains(t, res.Output, "helixcode")
	// Resource attrs rendered
	assert.Contains(t, res.Output, "env=prod")
	assert.Contains(t, res.Output, "team=core")
	// Timeouts rendered
	assert.Contains(t, res.Output, "5s")
	assert.Contains(t, res.Output, "30s")
	// CONST-046: the blocked-attr-keys count is interpolated into a
	// translated message; under NoopTranslator the message ID echoes.
	assert.Contains(t, res.Output, "internal_commands_telemetry_label_blocked_attr_keys")
}

func TestTelemetryCommand_StatusUnavailableWhenNoop(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	prov.exporterKind = telemetry.ExporterNoop
	prov.cfg.Enabled = false
	prov.cfg.Exporter = telemetry.ExporterNoop
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: the unavailable banner is a translated message ID under
	// the default NoopTranslator (loud echo).
	assert.True(t, strings.HasPrefix(res.Output, "internal_commands_telemetry_unavailable_banner"),
		"output should start with the unavailable-banner message ID, got %q", res.Output)
}

func TestTelemetryCommand_StatusUnavailableWhenNilProvider(t *testing.T) {
	c := NewTelemetryCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.True(t, strings.HasPrefix(res.Output, "internal_commands_telemetry_unavailable_banner"),
		"output should start with the unavailable-banner message ID, got %q", res.Output)
}

func TestTelemetryCommand_ShowListsBlockedKeys(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	prov.cfg.BlockedAttributeKeys = []string{
		"custom_secret_one",
		"custom_secret_two",
		"custom_secret_three",
		"custom_secret_four",
		"custom_secret_five",
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: status block still present (translated message ID).
	assert.Contains(t, res.Output, "internal_commands_telemetry_status_header")
	// All 5 user-supplied keys rendered as bullet points (raw data,
	// not translated).
	for _, k := range prov.cfg.BlockedAttributeKeys {
		assert.Contains(t, res.Output, k, "show output missing user-supplied blocked key %q", k)
	}
	// Default-deny entries also rendered (sample: api_key)
	assert.Contains(t, res.Output, "api_key")
	// CONST-046: blocked-keys list header is a translated message ID.
	assert.Contains(t, res.Output, "internal_commands_telemetry_blocked_keys_default_header")
}

func TestTelemetryCommand_FlushCallsProvider(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.NoError(t, err)
	assert.Equal(t, 1, prov.flushCalls, "ForceFlush must be invoked exactly once")
	assert.False(t, prov.lastContextNil, "ForceFlush must receive a non-nil context")
}

func TestTelemetryCommand_FlushReportsSuccess(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: flush-success message is a translated message ID under
	// the default NoopTranslator (loud echo).
	assert.Contains(t, res.Output, "internal_commands_telemetry_flush_ok")
}

func TestTelemetryCommand_FlushReportsFailure(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	prov.flushErr = errors.New("collector unreachable")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector unreachable")
}

func TestTelemetryCommand_FlushUnavailableWhenNilProvider(t *testing.T) {
	c := NewTelemetryCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "unavailable")
}

func TestTelemetryCommand_FlushUnavailableWhenNoop(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	prov.exporterKind = telemetry.ExporterNoop
	prov.cfg.Enabled = false
	prov.cfg.Exporter = telemetry.ExporterNoop
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "unavailable")
	// Flush MUST NOT be invoked on a noop provider — there's nothing to flush.
	assert.Equal(t, 0, prov.flushCalls)
}

func TestTelemetryCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}

// --- Round-362 CONST-046 paired-mutation tests -----------------------------
//
// Each asserts the migrated user-facing literal now routes through the
// package tr() seam. With a sentinel translator wired the output MUST
// contain the sentinel-wrapped message ID; an inlined literal fails it.

func TestTelemetryCommand_StatusHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_telemetry_status_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_telemetry_label_exporter>")
}

func TestTelemetryCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newTelemetryCommand(t)
	assert.Equal(t, "<TR:internal_commands_telemetry_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_telemetry_usage>", c.Usage())
}

func TestTelemetryCommand_UnavailableBanner_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewTelemetryCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_telemetry_unavailable_banner>")
}

func TestTelemetryCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newTelemetryCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_telemetry_unknown_subcommand>")
}

func TestTelemetryCommand_FlushOK_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_telemetry_flush_ok>")
}
