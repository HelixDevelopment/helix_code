package deployment

// Regression guards (§11.4.115 RED→GREEN, §11.4.135) for two reproduced
// defects in production_deployer.go:
//
//   DEFECT-1 (nil-config panic): NewProductionDeployer(config) dereferences
//     config.SecurityGateEnabled BEFORE any nil check, so
//     NewProductionDeployer(nil) panics — even though checkPrerequisites
//     explicitly handles a nil config (nil is a contract-anticipated input).
//
//   DEFECT-4 (zero ScanTime): runSecurityScan set
//     ScanTime: time.Since(time.Now()), which measures ~0 (the duration of
//     time.Now() itself) rather than the elapsed scan time.
//
// Each test carries a RED_MODE polarity switch (default "1" = reproduce the
// defect on the pre-fix artifact). RED_MODE=1 against the broken
// production_deployer.go SEES the defect; the default (post-fix) RED_MODE=0
// asserts the defect is ABSENT — the standing regression guard.

import (
	"context"
	"os"
	"testing"
	"time"
)

// redModeDeployment reports whether the test should reproduce the historical
// defect (RED_MODE=1) rather than assert the fixed behaviour (RED_MODE=0).
func redModeDeployment() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestNewProductionDeployer_NilConfig — DEFECT-1.
//
//   - RED_MODE=1 (broken artifact): NewProductionDeployer(nil) panics on the
//     config.SecurityGateEnabled deref; this test asserts the panic occurs,
//     proving the defect is real.
//   - RED_MODE=0 (fixed artifact, default): NewProductionDeployer(nil) MUST
//     return a non-nil error and MUST NOT panic — the standing guard.
func TestNewProductionDeployer_NilConfig(t *testing.T) {
	if redModeDeployment() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("RED_MODE: expected NewProductionDeployer(nil) to PANIC on pre-fix artifact, but it did not")
			}
		}()
		_, _ = NewProductionDeployer(nil)
		return
	}

	// Fixed behaviour: graceful error, no panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewProductionDeployer(nil) panicked: %v; a nil config must return an error, not panic", r)
		}
	}()

	dep, err := NewProductionDeployer(nil)
	if err == nil {
		t.Fatalf("NewProductionDeployer(nil) returned nil error; a nil config must be rejected with an error")
	}
	if dep != nil {
		t.Fatalf("NewProductionDeployer(nil) returned non-nil deployer alongside an error; must return (nil, err)")
	}

	// A valid config must still succeed — the fix must not break the happy path.
	good, err := NewProductionDeployer(&DeploymentConfig{ProjectName: "guard"})
	if err != nil {
		t.Fatalf("NewProductionDeployer(valid) returned error %v; the fix must not break the valid path", err)
	}
	if good == nil {
		t.Fatalf("NewProductionDeployer(valid) returned nil deployer; the fix must not break the valid path")
	}
}

// TestRunSecurityScan_ScanTimeReflectsDuration — DEFECT-4.
//
//   - RED_MODE=1 (broken artifact): ScanTime is ~0 because time.Since(time.Now())
//     measures the duration of time.Now() itself; this test asserts ScanTime is
//     sub-microsecond, proving the defect is real.
//   - RED_MODE=0 (fixed artifact, default): ScanTime MUST reflect the real
//     elapsed scan duration (strictly positive) — the standing guard.
func TestRunSecurityScan_ScanTimeReflectsDuration(t *testing.T) {
	// runSecurityScan needs an existing binary path to reach the ScanTime field.
	bin, err := os.CreateTemp(t.TempDir(), "helix-scan-binary-*")
	if err != nil {
		t.Fatalf("failed to create temp binary fixture: %v", err)
	}
	if _, err := bin.WriteString("#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatalf("failed to write temp binary fixture: %v", err)
	}
	_ = bin.Close()
	if err := os.Chmod(bin.Name(), 0o755); err != nil {
		t.Fatalf("failed to chmod temp binary fixture: %v", err)
	}

	dep, err := NewProductionDeployer(&DeploymentConfig{
		ProjectName: "scan-guard",
		BinaryPath:  bin.Name(),
	})
	if err != nil {
		t.Fatalf("NewProductionDeployer returned error: %v", err)
	}

	res, err := dep.runSecurityScan(context.Background())
	if err != nil {
		t.Fatalf("runSecurityScan returned error: %v", err)
	}
	if res == nil {
		t.Fatalf("runSecurityScan returned nil result")
	}

	if redModeDeployment() {
		// On the broken artifact ScanTime is the duration of a single time.Now()
		// call — effectively zero, well under a microsecond.
		if res.ScanTime >= time.Microsecond {
			t.Fatalf("RED_MODE: expected broken ScanTime < 1µs on pre-fix artifact, got %v", res.ScanTime)
		}
		return
	}

	// Fixed behaviour: ScanTime reflects the real elapsed scan duration.
	if res.ScanTime <= 0 {
		t.Fatalf("ScanTime = %v; must be strictly positive (real elapsed scan duration), not time.Since(time.Now())", res.ScanTime)
	}
}
