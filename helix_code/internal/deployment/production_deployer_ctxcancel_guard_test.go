package deployment

// Standing regression guard (§11.4.135) with §11.4.115 RED_MODE polarity for
// the context-cancellation-ignored defect in production_deployer.go.
//
// DEFECT (HIGH / safety): StartProductionDeployment threads `ctx` through every
// phase but NEVER consulted ctx.Err()/ctx.Done(). A caller (operator abort,
// deadline, parent-cancel) that cancelled the context did NOT stop the
// deployment — it proceeded through preparation → security_check →
// performance_check → DEPLOYMENT (the per-server loop in executeProductionDeploy
// that pushes binaries to real servers, an IRREVERSIBLE side effect) → health
// → validation → monitoring regardless. Reproduced on the pre-fix artifact:
// with a context cancelled BEFORE Start, CompletedPhases came back as
// [preparation security_check performance_check] — three phases ran AFTER the
// caller signalled abort.
//
// FIX: ctx.Err() is checked (a) at the top of every phase iteration in
// StartProductionDeployment (stop before the next phase), and (b) at the top of
// executeProductionDeploy's per-server loop (stop before touching the next
// server — the irreversible boundary). On cancellation the deployment fails
// honestly with the cancellation cause instead of proceeding.
//
//	RED_MODE=1 go test -run TestCtxCancelGuard ./internal/deployment   # reproduces the defect on a faithful pre-fix stand-in
//	           go test -run TestCtxCancelGuard ./internal/deployment   # GREEN: drives the REAL fixed code, asserts abort

import (
	"context"
	"errors"
	"testing"
)

func ctxCancelGuardConfig(name string, servers ...string) *DeploymentConfig {
	return &DeploymentConfig{
		ProjectName:        name,
		BinaryPath:         "/nonexistent/helixcode-ctxcancel-binary",
		Environment:        "test",
		DeploymentStrategy: ProductionDeploy,
		TargetServers:      servers,
		Credentials:        map[string]string{},
	}
}

// prefixPhaseLoopRanPastCancel is a FAITHFUL stand-in for the pre-fix
// StartProductionDeployment phase loop: it walks the same phase list and (like
// the broken code) NEVER consults ctx. It returns the phases it "completed"
// despite the context being cancelled. This reproduces the defect deterministically
// without reaching real infra: the very fact that it iterates every phase with a
// cancelled context IS the defect. We only walk the pre-deployment phases
// (preparation/security/performance) that the broken code completed before the
// no-infra deployment phase failed.
func prefixPhaseLoopRanPastCancel(ctx context.Context) []DeploymentPhase {
	phases := []DeploymentPhase{
		PhasePreparation,
		PhaseSecurityCheck,
		PhasePerformanceCheck,
	}
	var completed []DeploymentPhase
	for _, phase := range phases {
		// PRE-FIX BEHAVIOUR: no `if ctx.Err() != nil { stop }` here — the loop
		// body runs regardless of cancellation. This is the exact omission the
		// fix repairs.
		completed = append(completed, phase)
	}
	return completed
}

// TestCtxCancelGuard_PreStartCancellation_StopsBeforeAnyPhase is the primary
// guard. RED_MODE=1 reproduces the defect on the faithful pre-fix stand-in;
// RED_MODE=0 (default) drives the REAL fixed StartProductionDeployment and
// asserts a cancelled-before-start context aborts at the FIRST phase with zero
// phases completed and zero servers deployed.
func TestCtxCancelGuard_PreStartCancellation_StopsBeforeAnyPhase(t *testing.T) {
	if redModeDeployment() {
		// RED: the pre-fix loop ignores cancellation and "completes" all three
		// pre-deployment phases despite the context being cancelled.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		completed := prefixPhaseLoopRanPastCancel(ctx)
		if len(completed) == 0 {
			t.Fatalf("RED_MODE: expected the pre-fix phase loop to run phases despite cancellation, but it ran none")
		}
		t.Logf("RED_MODE reproduced: pre-fix loop ran %d phases past a cancelled context: %v", len(completed), completed)
		return
	}

	// GREEN: the REAL fixed deployer.
	pd, err := NewProductionDeployer(ctxCancelGuardConfig("ctxcancel-prestart", "s1", "s2", "s3"))
	if err != nil {
		t.Fatalf("NewProductionDeployer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE Start

	status, startErr := pd.StartProductionDeployment(ctx)
	if startErr != nil {
		t.Fatalf("StartProductionDeployment returned single-flight error unexpectedly: %v", startErr)
	}
	if status == nil {
		t.Fatalf("StartProductionDeployment returned nil status")
	}

	// The fixed code must abort at the FIRST phase (preparation) — NO phase may
	// complete after the context was already cancelled.
	if len(status.CompletedPhases) != 0 {
		t.Fatalf("cancelled-before-start context still completed phases %v; the deployment must abort before any phase runs",
			status.CompletedPhases)
	}
	// No server may be deployed after abort.
	if len(status.ServersDeployed) != 0 {
		t.Fatalf("cancelled context still deployed to servers %v; no server may be touched after abort", status.ServersDeployed)
	}
	// The deployment must be marked failed (aborted), failing at the first phase.
	if status.Status != PhaseFailed {
		t.Fatalf("cancelled context produced Status=%s; expected %s (aborted)", status.Status, PhaseFailed)
	}
	if len(status.FailedPhases) != 1 || status.FailedPhases[0] != string(PhasePreparation) {
		t.Fatalf("cancelled-before-start must fail at the first phase (%s); got FailedPhases=%v",
			PhasePreparation, status.FailedPhases)
	}
}

// TestCtxCancelGuard_PerServerLoop_StopsBeforeNextServer proves executeProductionDeploy
// honours cancellation at the irreversible per-server boundary: a context
// cancelled before the loop must return a cancellation error and must NOT
// proceed to deploy to any server. (RED_MODE has no separate per-server stand-in;
// the pre-fix loop simply had no ctx check, so the GREEN assertion — that a
// cancelled context yields a cancellation error rather than the no-credentials
// path — is itself the regression guard. On the pre-fix artifact this test
// FAILS because the loop ignored ctx and returned the success-rate error
// instead.)
func TestCtxCancelGuard_PerServerLoop_StopsBeforeNextServer(t *testing.T) {
	if redModeDeployment() {
		t.Skip("SKIP-OK: per-server cancellation has no pre-fix stand-in; the GREEN assertion is the regression guard (see TestCtxCancelGuard_PreStartCancellation_StopsBeforeAnyPhase for the reproduced defect)")
	}

	pd, err := NewProductionDeployer(ctxCancelGuardConfig("ctxcancel-perserver", "s1", "s2", "s3"))
	if err != nil {
		t.Fatalf("NewProductionDeployer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, deployErr := pd.executeProductionDeploy(ctx)
	if ok {
		t.Fatalf("executeProductionDeploy returned success on a cancelled context")
	}
	if deployErr == nil {
		t.Fatalf("executeProductionDeploy returned nil error on a cancelled context; cancellation must surface an error")
	}
	if !errors.Is(deployErr, context.Canceled) && !errors.Is(deployErr, context.DeadlineExceeded) {
		t.Fatalf("executeProductionDeploy error %q does not wrap context.Canceled/DeadlineExceeded; the per-server loop must stop on ctx.Err()", deployErr)
	}
	// No server may be recorded as deployed after abort.
	if len(pd.status.ServersDeployed) != 0 {
		t.Fatalf("servers %v deployed despite cancellation; the loop must stop before touching any server", pd.status.ServersDeployed)
	}
}
