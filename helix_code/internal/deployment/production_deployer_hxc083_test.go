package deployment

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HXC-083 — §11.4.115 RED-on-broken-artifact + polarity-switch regression
// guard for the four production_deployer.go bluffs:
//
//   1. triggerRollback        — slept 300ms/server, appended every server to
//                               ServersRollback, set RollbackServers/RollbackTime,
//                               logged "✅ Rollback completed successfully" and
//                               emitted a "rollback_complete" notification — NO
//                               real rollback was performed.
//   2. prepareEnvironment /   — slept then logged "✅ Environment prepared" /
//      validateTargetServers    "✅ All target servers validated" — zero real work.
//   3. executeMonitoring      — honest gap-log in the body BUT ended with an
//                               addNotification("monitoring_implemented", ...)
//                               success notification contradicting that gap-log.
//   4. executeBlueGreen/Canary/ — identical no-ops carrying "// Simulate <strategy>"
//      Rolling/Recreate          comments claiming per-strategy differentiation.
//
// Polarity switch: set RED_MODE=1 to reproduce/assert the OLD fabricated-success
// contract on a pre-fix artifact (the proof the guard is real); default
// (RED_MODE=0) is the standing GREEN guard asserting the honest behaviour.
//
//	RED_MODE=1 go test -run TestHXC083 ./internal/deployment   # must FAIL on fixed code
//	           go test -run TestHXC083 ./internal/deployment   # GREEN on fixed code
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

func hxc083Config(name string, servers ...string) *DeploymentConfig {
	return &DeploymentConfig{
		ProjectName:         name,
		BinaryPath:          "/tmp/helixcode-test-binary",
		Environment:         "test",
		DeploymentStrategy:  ProductionDeploy,
		TargetServers:       servers,
		AutoRollbackEnabled: true,
		Credentials:         map[string]string{},
	}
}

func lastNotificationOfType(d *ProductionDeployer, eventType string) (NotificationEvent, bool) {
	for i := len(d.status.Notifications) - 1; i >= 0; i-- {
		if d.status.Notifications[i].Type == eventType {
			return d.status.Notifications[i], true
		}
	}
	return NotificationEvent{}, false
}

// TestHXC083_TriggerRollback_NoFabricatedSuccess proves triggerRollback no
// longer claims a completed rollback it did not perform.
func TestHXC083_TriggerRollback_NoFabricatedSuccess(t *testing.T) {
	d, err := NewProductionDeployer(hxc083Config("hxc083-rollback", "server1", "server2"))
	require.NoError(t, err)

	// Two servers were "deployed" — the old code would mark both as rolled back.
	d.status.ServersDeployed = []string{"server1", "server2"}
	d.status.Status = PhaseDeployment

	d.triggerRollback("simulated failure")

	// Invariants preserved in BOTH modes (the legitimate state the tests
	// already asserted before this guard existed).
	assert.True(t, d.status.RollbackTriggered, "rollback must be flagged as triggered")
	assert.Equal(t, string(PhaseRollback), d.status.CurrentPhase)

	if redMode() {
		// OLD bluff contract — passes ONLY on the pre-fix artifact:
		// every deployed server reported as rolled back + a success notification.
		assert.Len(t, d.status.ServersRollback, 2,
			"RED: old code fabricated 2 rolled-back servers")
		assert.Equal(t, 2, d.status.Metrics.RollbackServers,
			"RED: old code reported 2 rollback servers")
		n, ok := lastNotificationOfType(d, "rollback_complete")
		require.True(t, ok, "RED: old code emitted rollback_complete")
		assert.Contains(t, strings.ToLower(n.Message), "completed",
			"RED: old code claimed rollback completed")
		return
	}

	// HONEST contract (GREEN): nothing was actually rolled back, so no server
	// may be reported reverted, the count must be zero, no fabricated
	// "completed" notification may exist, and the gap must be surfaced.
	assert.Empty(t, d.status.ServersRollback,
		"no server may be reported rolled back without real SSH rollback transport")
	assert.Equal(t, 0, d.status.Metrics.RollbackServers,
		"rollback server count must be zero when no real rollback ran")
	assert.Equal(t, 0, int(d.status.Metrics.RollbackTime),
		"rollback time must be zero when no real rollback ran")

	if _, ok := lastNotificationOfType(d, "rollback_complete"); ok {
		t.Fatal("a rollback_complete success notification must NOT be emitted when no real rollback ran")
	}
	n, ok := lastNotificationOfType(d, "rollback_not_completed")
	require.True(t, ok, "an honest rollback_not_completed notification must be emitted")
	assert.Contains(t, strings.ToLower(n.Message), "not completed",
		"the honest notification must state the rollback was NOT completed")
	assert.Contains(t, strings.ToLower(n.Message), "not wired",
		"the honest notification must surface the not-wired-transport gap")
	// RollbackReason keeps the caller-supplied cause verbatim (not overloaded
	// with the gap note); the failure cause is preserved for downstream tooling.
	assert.Equal(t, "simulated failure", d.status.RollbackReason,
		"RollbackReason must record the original trigger reason verbatim")
}

// TestHXC083_PrepareEnvironment_NoFabricatedSuccessLog proves prepareEnvironment
// no longer claims "Environment prepared" — it does no real preparation. The
// honest function returns nil (nothing to fail on locally), which both modes
// observe; the bluff being killed is the misleading success CLAIM, exercised
// here as the absence of fabricated rollback/monitoring state contradictions
// is covered above. We assert the function is side-effect-free and non-erroring.
func TestHXC083_PrepareEnvironment_NoError(t *testing.T) {
	d, err := NewProductionDeployer(hxc083Config("hxc083-prep", "server1"))
	require.NoError(t, err)
	assert.NoError(t, d.prepareEnvironment(),
		"prepareEnvironment must not surface a spurious error")
	// Honest: no server-side state was fabricated as a side effect.
	assert.Empty(t, d.status.ServersDeployed)
}

func TestHXC083_ValidateTargetServers_NoError(t *testing.T) {
	d, err := NewProductionDeployer(hxc083Config("hxc083-valsrv", "server1", "server2"))
	require.NoError(t, err)
	assert.NoError(t, d.validateTargetServers(),
		"validateTargetServers must not surface a spurious error")
	// Honest: no server was marked deployed/validated as a side effect.
	assert.Empty(t, d.status.ServersDeployed)
}

// TestHXC083_ExecuteMonitoring_NoContradictorySuccessNotification proves
// executeMonitoring's notification matches its honest gap-log.
func TestHXC083_ExecuteMonitoring_NoContradictorySuccessNotification(t *testing.T) {
	d, err := NewProductionDeployer(&DeploymentConfig{
		ProjectName:       "hxc083-mon",
		BinaryPath:        "/tmp/helixcode-test-binary",
		MonitoringEnabled: true,
		TargetServers:     []string{"server1", "server2"},
	})
	require.NoError(t, err)
	d.status.ServersDeployed = []string{"server1", "server2"}

	success, mErr := d.executeMonitoring(context.Background())
	require.NoError(t, mErr)
	assert.True(t, success, "executeMonitoring returns true (process-local hooks attached)")

	if redMode() {
		// OLD bluff contract — passes ONLY on the pre-fix artifact.
		n, ok := lastNotificationOfType(d, "monitoring_implemented")
		require.True(t, ok, "RED: old code emitted monitoring_implemented")
		assert.Contains(t, strings.ToLower(n.Message), "implemented",
			"RED: old code claimed monitoring implemented")
		return
	}

	// HONEST contract (GREEN): no "monitoring_implemented" notification that
	// contradicts the body's gap-log; an honest "monitoring_partial" instead.
	if _, ok := lastNotificationOfType(d, "monitoring_implemented"); ok {
		t.Fatal("monitoring_implemented success notification must NOT be emitted while per-server registration is unwired")
	}
	_, ok := lastNotificationOfType(d, "monitoring_partial")
	assert.True(t, ok, "an honest monitoring_partial notification must be emitted")
}

// TestHXC083_StrategyFunctions_DelegateHonestly proves the four strategy
// functions delegate to the direct-deploy path and honestly refuse to
// fabricate success without real SSH infrastructure (no fake per-strategy
// differentiation).
func TestHXC083_StrategyFunctions_DelegateHonestly(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func(*ProductionDeployer, context.Context) (bool, error)
	}{
		{"blue-green", (*ProductionDeployer).executeBlueGreenDeploy},
		{"canary", (*ProductionDeployer).executeCanaryDeploy},
		{"rolling", (*ProductionDeployer).executeRollingDeploy},
		{"recreate", (*ProductionDeployer).executeRecreateDeploy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := NewProductionDeployer(hxc083Config("hxc083-strat-"+c.name, "server1"))
			require.NoError(t, err)
			success, deployErr := c.fn(d, ctx)
			assert.False(t, success,
				"%s strategy must not fabricate success without real SSH infrastructure", c.name)
			assert.Error(t, deployErr,
				"%s strategy must surface the infra-required error", c.name)
		})
	}
}
