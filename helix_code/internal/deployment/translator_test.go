// Unit tests for the internal/deployment package-level translator +
// tr() helper (CONST-046 round-153 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package deployment

import (
	"context"
	"errors"
	"strings"
	"testing"

	deploymenti18n "dev.helix.code/internal/deployment/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(context.Background(), "internal_deployment_already_running", nil)
	if got != "internal_deployment_already_running" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_deployment_no_target_servers_configured", nil)
	if got != "<TR:internal_deployment_no_target_servers_configured>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer). Implementation MUST degrade to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_deployment_validation_security_gate_failed", nil)
	if got != "internal_deployment_validation_security_gate_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_deployment_validation_health_checks_failed", nil)
	if got != "internal_deployment_validation_health_checks_failed" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(deploymenti18n.NoopTranslator{})
	got := tr(context.Background(), "internal_deployment_unknown_phase", nil)
	if got != "internal_deployment_unknown_phase" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// newDeployer is a small helper to slim the call-site tests; the
// configurator callback lets each table row pre-plant state before
// the trigger callback is invoked.
func newDeployer(t *testing.T, configure func(*DeploymentConfig)) *ProductionDeployer {
	t.Helper()
	cfg := &DeploymentConfig{
		ProjectName:   "t",
		BinaryPath:    "/tmp/no-such-binary",
		TargetServers: []string{"s"},
		Credentials:   map[string]string{},
	}
	if configure != nil {
		configure(cfg)
	}
	d, err := NewProductionDeployer(cfg)
	if err != nil {
		t.Fatalf("NewProductionDeployer: %v", err)
	}
	return d
}

// TestProductionDeployer_MigratedErrors_GoThroughTranslator is the
// call-site paired-mutation: with a sentinel translator wired, every
// migrated fmt.Errorf path on ProductionDeployer MUST surface the
// sentinel-wrapped message ID — proving the literal was NOT
// hardcoded anywhere on the path. If a future refactor inlines any
// string, the matching case fails.
func TestProductionDeployer_MigratedErrors_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	ctx := context.Background()

	cases := []struct {
		name      string
		configure func(*DeploymentConfig)
		trigger   func(*ProductionDeployer) error
		wantID    string
	}{
		{
			name: "already_running",
			trigger: func(d *ProductionDeployer) error {
				d.running.Store(true)
				_, err := d.StartProductionDeployment(ctx)
				return err
			},
			wantID: "internal_deployment_already_running",
		},
		{
			name: "unknown_phase",
			trigger: func(d *ProductionDeployer) error {
				_, err := d.executePhase(ctx, DeploymentPhase("phase-x"))
				return err
			},
			wantID: "internal_deployment_unknown_phase",
		},
		{
			name: "unknown_strategy",
			configure: func(c *DeploymentConfig) {
				c.DeploymentStrategy = DeployStrategy("strategy-x")
			},
			trigger: func(d *ProductionDeployer) error {
				_, err := d.executeDeployment(ctx)
				return err
			},
			wantID: "internal_deployment_unknown_strategy",
		},
		{
			name: "no_target_servers_configured",
			configure: func(c *DeploymentConfig) {
				c.TargetServers = []string{}
				c.DeploymentStrategy = ProductionDeploy
			},
			trigger: func(d *ProductionDeployer) error {
				_, err := d.executeProductionDeploy(ctx)
				return err
			},
			wantID: "internal_deployment_no_target_servers_configured",
		},
		{
			name: "validation_no_servers_deployed",
			trigger: func(d *ProductionDeployer) error {
				_, err := d.executeValidation(ctx)
				return err
			},
			wantID: "internal_deployment_validation_no_servers_deployed",
		},
		{
			name:      "validation_security_gate_failed",
			configure: func(c *DeploymentConfig) { c.SecurityGateEnabled = true },
			trigger: func(d *ProductionDeployer) error {
				d.status.ServersDeployed = []string{"srv"}
				d.status.SecurityGateStatus.Passed = false
				_, err := d.executeValidation(ctx)
				return err
			},
			wantID: "internal_deployment_validation_security_gate_failed",
		},
		{
			name:      "validation_performance_gate_failed",
			configure: func(c *DeploymentConfig) { c.PerformanceGateEnabled = true },
			trigger: func(d *ProductionDeployer) error {
				d.status.ServersDeployed = []string{"srv"}
				d.status.PerformanceGate.Passed = false
				_, err := d.executeValidation(ctx)
				return err
			},
			wantID: "internal_deployment_validation_performance_gate_failed",
		},
		{
			name:      "validation_health_checks_failed",
			configure: func(c *DeploymentConfig) { c.HealthCheckEnabled = true },
			trigger: func(d *ProductionDeployer) error {
				d.status.ServersDeployed = []string{"srv"}
				d.status.HealthStatus.Passed = false
				_, err := d.executeValidation(ctx)
				return err
			},
			wantID: "internal_deployment_validation_health_checks_failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDeployer(t, tc.configure)
			err := tc.trigger(d)
			if err == nil {
				t.Fatalf("%s: trigger returned no error", tc.name)
			}
			want := "<TR:" + tc.wantID + ">"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s err = %q, want contain %q — call site bypassed tr()", tc.name, err.Error(), want)
			}
		})
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), executeValidation surfaces the bundle message ID
// — confirming the migration didn't accidentally pass an empty
// string or different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	d := newDeployer(t, nil)
	_, err := d.executeValidation(context.Background())
	if err == nil {
		t.Fatal("executeValidation returned no error")
	}
	if !strings.Contains(err.Error(), "internal_deployment_validation_no_servers_deployed") {
		t.Fatalf("executeValidation err = %q, want raw message ID (Noop echo)", err.Error())
	}
}
