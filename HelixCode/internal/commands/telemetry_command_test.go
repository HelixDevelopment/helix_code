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
	assert.NotEmpty(t, c.Description())
	assert.Contains(t, c.Usage(), "/telemetry")
	assert.Nil(t, c.Aliases())
}

func TestTelemetryCommand_DefaultIsStatus(t *testing.T) {
	c, _ := newTelemetryCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// status output mentions Telemetry status header + Exporter line.
	assert.Contains(t, res.Output, "Telemetry status")
	assert.Contains(t, res.Output, "Exporter")
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
	// Default-deny floor count surfaces (22 in DefaultBlockedAttributeKeys)
	assert.Contains(t, res.Output, "22")
}

func TestTelemetryCommand_StatusUnavailableWhenNoop(t *testing.T) {
	c, prov := newTelemetryCommand(t)
	prov.exporterKind = telemetry.ExporterNoop
	prov.cfg.Enabled = false
	prov.cfg.Exporter = telemetry.ExporterNoop
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.True(t, strings.HasPrefix(res.Output, "Telemetry unavailable"),
		"output should start with 'Telemetry unavailable', got %q", res.Output)
}

func TestTelemetryCommand_StatusUnavailableWhenNilProvider(t *testing.T) {
	c := NewTelemetryCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.True(t, strings.HasPrefix(res.Output, "Telemetry unavailable"),
		"output should start with 'Telemetry unavailable', got %q", res.Output)
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
	// Status block still present
	assert.Contains(t, res.Output, "Telemetry status")
	// All 5 user-supplied keys rendered as bullet points
	for _, k := range prov.cfg.BlockedAttributeKeys {
		assert.Contains(t, res.Output, k, "show output missing user-supplied blocked key %q", k)
	}
	// Default-deny entries also rendered (sample: api_key)
	assert.Contains(t, res.Output, "api_key")
	// Show header for blocked keys list
	assert.Contains(t, res.Output, "Blocked attribute keys")
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
	assert.Contains(t, res.Output, "flushed")
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
