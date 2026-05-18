// Package main — distributed_test.go.
//
// This file is intentionally NOT build-tagged (matching distributed.go)
// so the round-31 sentinel tests + the round-67 §11.4 SDK-injection
// tests execute on every host that can build the Go toolchain — NOT
// only on hosts with Fyne / X11 (where main.go is buildable). Prior to
// round-67, these tests lived in main_test.go under `//go:build !nogui`
// and were silently uncovered on any host missing libXcursor-devel
// (Article XI §11.9: a green-looking suite that never actually
// executed is a CRITICAL defect).

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------
// Round-31 §11.4 sentinel preservation tests (relocated from
// main_test.go). These assert the CLASS A anti-bluff: with NO
// HarmonyDistributedSDK injected, performSync and DiscoverDevices
// continue to return the round-31 sentinels and continue to refuse to
// advance lastSync.
// ---------------------------------------------------------------------

// TestHarmonyDataSync_PerformSync_NoSDKInjected_ReturnsSentinel asserts
// the round-31 §11.4 anti-bluff fix: performSync MUST return the
// ErrHarmonyDistributedSyncNotImplemented sentinel (and MUST NOT advance
// lastSync) when no HarmonyDistributedSDK has been injected (the default
// for builds without a Harmony OS Go binding). Regression of this
// assertion = regression of the "Last Sync: Just now / Synced Devices: 0
// forever" PASS-bluff.
func TestHarmonyDataSync_PerformSync_NoSDKInjected_ReturnsSentinel(t *testing.T) {
	ds := NewHarmonyDataSync()
	require.NotNil(t, ds)
	require.Nil(t, ds.distributedSDK, "default constructor MUST NOT install an SDK — sentinel branch is the default")

	beforeLastSync := ds.lastSync
	time.Sleep(2 * time.Millisecond)

	err := ds.performSync()
	require.Error(t, err, "performSync MUST return an error in the no-SDK branch (sentinel)")
	require.ErrorIs(t, err, ErrHarmonyDistributedSyncNotImplemented,
		"performSync MUST return ErrHarmonyDistributedSyncNotImplemented — anything else is a regression of the round-31 §11.4 anti-bluff fix")

	assert.Equal(t, beforeLastSync, ds.lastSync,
		"performSync MUST NOT advance lastSync when sync did not actually happen — the previous bluff stamped lastSync = time.Now() unconditionally and reported 'Just now' forever")
}

// TestHarmonyDataSync_GetSyncStatus_NoSDKInjected_ReportsError asserts
// that the 4-tuple return from GetSyncStatus surfaces the sentinel
// error after a failed performSync. Without this, UI / CLI callers
// reading only the (enabled, lastSync, syncedCount) prefix recreate
// the original bluff.
func TestHarmonyDataSync_GetSyncStatus_NoSDKInjected_ReportsError(t *testing.T) {
	ds := NewHarmonyDataSync()
	require.NotNil(t, ds)

	_, _, _, errBefore := ds.GetSyncStatus()
	assert.NoError(t, errBefore, "newly-constructed HarmonyDataSync has no lastSyncErr")

	require.Error(t, ds.performSync())

	enabled, _, syncedCount, errAfter := ds.GetSyncStatus()
	assert.True(t, enabled, "GetSyncStatus continues to report the static syncEnabled flag")
	assert.Equal(t, 0, syncedCount, "no devices were synced — the count MUST remain 0")
	require.Error(t, errAfter, "GetSyncStatus MUST surface the failed-performSync sentinel")
	assert.True(t, errors.Is(errAfter, ErrHarmonyDistributedSyncNotImplemented),
		"GetSyncStatus error MUST be (or wrap) ErrHarmonyDistributedSyncNotImplemented")
}

// TestHarmonyDistributedEngine_DiscoverDevices_NoSDKInjected_EmptyReturnsSentinel
// asserts the round-31 §11.4 anti-bluff fix: DiscoverDevices on an
// engine with no enrolled devices AND no injected SDK MUST return
// ErrHarmonyDiscoveryNotImplemented.
func TestHarmonyDistributedEngine_DiscoverDevices_NoSDKInjected_EmptyReturnsSentinel(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	require.NotNil(t, engine)
	require.Nil(t, engine.distributedSDK, "default constructor MUST NOT install an SDK — sentinel branch is the default")

	devices, err := engine.DiscoverDevices()
	require.Error(t, err, "empty-engine DiscoverDevices MUST return the sentinel (round-31 §11.4 fix)")
	require.ErrorIs(t, err, ErrHarmonyDiscoveryNotImplemented,
		"DiscoverDevices MUST return ErrHarmonyDiscoveryNotImplemented when no devices have been enrolled")
	assert.Empty(t, devices, "no devices have been enrolled — the slice MUST be empty")
}

// TestHarmonyDistributedEngine_DiscoverDevices_AfterAddDeviceNoSentinel
// asserts the complementary contract: once at least one device is
// enrolled through the AddDevice path (worker-tab enrolment), the
// non-discovery legitimate caller MUST receive a nil error so we
// don't break that workflow.
func TestHarmonyDistributedEngine_DiscoverDevices_AfterAddDeviceNoSentinel(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	require.NotNil(t, engine)

	engine.AddDevice(HarmonyDevice{
		ID:     "device-test-1",
		Name:   "Test Device",
		Type:   "remote_worker",
		Status: "active",
		Resources: HarmonyResources{
			Available: true,
		},
		LastSeen: time.Now(),
	})

	devices, err := engine.DiscoverDevices()
	require.NoError(t, err, "after AddDevice, DiscoverDevices on a non-empty engine MUST NOT return the sentinel")
	require.Len(t, devices, 1)
	assert.Equal(t, "device-test-1", devices[0].ID)
}

// TestSentinelErrorMessages_ContainForensicAnchor asserts that the two
// sentinel error messages contain the §11.4 forensic anchor strings.
func TestSentinelErrorMessages_ContainForensicAnchor(t *testing.T) {
	assert.Contains(t, ErrHarmonyDistributedSyncNotImplemented.Error(),
		"distributed sync has not been wired",
		"sentinel message MUST anchor on the implementation gap")
	assert.Contains(t, ErrHarmonyDistributedSyncNotImplemented.Error(),
		"§11.4",
		"sentinel message MUST carry the §11.4 forensic anchor")

	assert.Contains(t, ErrHarmonyDiscoveryNotImplemented.Error(),
		"distributed device discovery has not been wired",
		"sentinel message MUST anchor on the implementation gap")
	assert.Contains(t, ErrHarmonyDiscoveryNotImplemented.Error(),
		"§11.4",
		"sentinel message MUST carry the §11.4 forensic anchor")
}

// ---------------------------------------------------------------------
// Round-67 §11.4 SDK-injection coverage. These exercise the new
// HarmonyDistributedSDK injection point added in round-67. Tests use a
// purpose-built fakeHarmonySDK fixture LOCAL to this _test.go file —
// per CONST-050(A), unit-test mocks/fakes are PERMITTED only inside
// _test.go files and MUST NOT leak into production code.
// ---------------------------------------------------------------------

// fakeHarmonySDK is a unit-test-only HarmonyDistributedSDK
// implementation. PERMITTED location: this _test.go file ONLY (per
// CONST-050(A)). MUST NOT be referenced from production code under
// applications/harmony_os/ (main.go, main_nogui.go, distributed.go,
// theme.go).
type fakeHarmonySDK struct {
	mu sync.Mutex

	// SyncData behaviour.
	syncDeviceCount int
	syncErr         error
	syncCalls       int
	lastSyncCtx     context.Context

	// DiscoverDevices behaviour.
	discoverDevices []HarmonyDevice
	discoverErr     error
	discoverCalls   int
	lastDiscoverCtx context.Context
}

func (f *fakeHarmonySDK) SyncData(ctx context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncCalls++
	f.lastSyncCtx = ctx
	return f.syncDeviceCount, f.syncErr
}

func (f *fakeHarmonySDK) DiscoverDevices(ctx context.Context) ([]HarmonyDevice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discoverCalls++
	f.lastDiscoverCtx = ctx
	return f.discoverDevices, f.discoverErr
}

// TestSetDistributedSDK_PropagatesToDataSync asserts the round-67
// contract that calling SetDistributedSDK on the engine ALSO installs
// the SDK on the underlying HarmonyDataSync — a single injection point
// wires both discovery and sync.
func TestSetDistributedSDK_PropagatesToDataSync(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	require.NotNil(t, engine.dataSync)
	require.Nil(t, engine.distributedSDK)
	require.Nil(t, engine.dataSync.distributedSDK)

	fake := &fakeHarmonySDK{}
	engine.SetDistributedSDK(fake)

	assert.Same(t, fake, engine.distributedSDK, "engine MUST hold the injected SDK")
	assert.Same(t, fake, engine.dataSync.distributedSDK,
		"engine.SetDistributedSDK MUST propagate the SDK to the underlying HarmonyDataSync — round-67 contract")
}

// TestSetDistributedSDK_NilImpl_UninstallsCleanly asserts that passing
// nil to SetDistributedSDK UN-installs a previously-injected SDK and
// restores the round-31 sentinel branch. This is useful for test
// isolation + for graceful degradation when the underlying binding
// reports a permanent failure.
func TestSetDistributedSDK_NilImpl_UninstallsCleanly(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	fake := &fakeHarmonySDK{}
	engine.SetDistributedSDK(fake)
	require.Same(t, fake, engine.distributedSDK)

	engine.SetDistributedSDK(nil)
	assert.Nil(t, engine.distributedSDK, "passing nil MUST UN-install the SDK")
	assert.Nil(t, engine.dataSync.distributedSDK, "nil propagation MUST cascade to dataSync")

	// After UN-install, DiscoverDevices on an empty engine MUST return
	// the round-31 sentinel again.
	devices, err := engine.DiscoverDevices()
	require.ErrorIs(t, err, ErrHarmonyDiscoveryNotImplemented,
		"after UN-installing the SDK, the round-31 sentinel branch MUST re-engage")
	assert.Empty(t, devices)
}

// TestPerformSync_WithSDK_DelegatesAndPopulatesState asserts the
// round-67 contract that a successful injected-SDK SyncData advances
// lastSync, populates syncedDevices with deviceCount entries, clears
// lastSyncErr, and returns nil.
func TestPerformSync_WithSDK_DelegatesAndPopulatesState(t *testing.T) {
	ds := NewHarmonyDataSync()
	fake := &fakeHarmonySDK{
		syncDeviceCount: 3,
		syncErr:         nil,
	}
	ds.SetDistributedSDK(fake)

	beforeLastSync := ds.lastSync
	time.Sleep(2 * time.Millisecond)

	err := ds.performSync()
	require.NoError(t, err, "successful SDK SyncData MUST surface nil — the round-67 success contract")

	assert.Equal(t, 1, fake.syncCalls, "performSync MUST delegate to the injected SDK exactly once")
	assert.NotNil(t, fake.lastSyncCtx, "performSync MUST pass a non-nil context to the SDK")

	enabled, gotLastSync, syncedCount, lastErr := ds.GetSyncStatus()
	assert.True(t, enabled)
	assert.True(t, gotLastSync.After(beforeLastSync),
		"successful sync MUST advance lastSync past its pre-call value (round-67 success contract)")
	assert.Equal(t, 3, syncedCount,
		"syncedDevices MUST reflect the deviceCount returned by the SDK")
	assert.NoError(t, lastErr, "successful sync MUST clear lastSyncErr")
}

// TestPerformSync_WithSDK_ErrorPropagates asserts that an SDK error
// surfaces wrapped and — crucially — does NOT advance lastSync. The
// round-31 anti-bluff invariant applies equally to the injected-SDK
// failure branch.
func TestPerformSync_WithSDK_ErrorPropagates(t *testing.T) {
	ds := NewHarmonyDataSync()
	sdkErr := errors.New("simulated harmony binding transport failure")
	fake := &fakeHarmonySDK{
		syncDeviceCount: 0,
		syncErr:         sdkErr,
	}
	ds.SetDistributedSDK(fake)

	beforeLastSync := ds.lastSync
	time.Sleep(2 * time.Millisecond)

	err := ds.performSync()
	require.Error(t, err, "SDK failure MUST be surfaced to caller")
	assert.ErrorIs(t, err, sdkErr, "returned error MUST wrap the SDK's error")
	assert.Contains(t, err.Error(), "injected SDK SyncData",
		"wrapped error MUST identify the layer for forensics")

	// CRITICAL anti-bluff invariant — failed sync MUST NOT advance lastSync.
	assert.Equal(t, beforeLastSync, ds.lastSync,
		"failed SDK sync MUST NOT advance lastSync — same anti-bluff guarantee as the no-SDK sentinel branch")

	_, _, syncedCount, lastErr := ds.GetSyncStatus()
	assert.Equal(t, 0, syncedCount, "failed sync MUST NOT mutate syncedDevices")
	require.Error(t, lastErr, "lastSyncErr MUST be populated with the wrapped SDK error")
	assert.ErrorIs(t, lastErr, sdkErr)
}

// TestPerformSync_WithSDK_ZeroDevicesSuccess asserts the legitimate
// "honest zero" case: SDK round-trip succeeded but the cluster
// reported zero peers. lastSync IS advanced (handshake succeeded),
// syncedDevices is empty, lastSyncErr is nil. This is the branch
// callers MUST be able to distinguish from the no-SDK sentinel.
func TestPerformSync_WithSDK_ZeroDevicesSuccess(t *testing.T) {
	ds := NewHarmonyDataSync()
	fake := &fakeHarmonySDK{
		syncDeviceCount: 0,
		syncErr:         nil,
	}
	ds.SetDistributedSDK(fake)

	beforeLastSync := ds.lastSync
	time.Sleep(2 * time.Millisecond)

	err := ds.performSync()
	require.NoError(t, err, "zero-device success is legitimate — handshake worked, no peers reachable")

	_, gotLastSync, syncedCount, lastErr := ds.GetSyncStatus()
	assert.True(t, gotLastSync.After(beforeLastSync),
		"successful zero-device sync still advances lastSync — handshake DID succeed")
	assert.Equal(t, 0, syncedCount)
	assert.NoError(t, lastErr)
}

// TestDiscoverDevices_WithSDK_DelegatesAndMerges asserts the round-67
// contract that DiscoverDevices delegates to the injected SDK, merges
// the result into connectedDevices (de-duped by ID), and returns the
// full merged slice with a nil error.
func TestDiscoverDevices_WithSDK_DelegatesAndMerges(t *testing.T) {
	engine := NewHarmonyDistributedEngine()

	// Pre-enrol one device through AddDevice — DiscoverDevices MUST NOT
	// drop it when the SDK returns its own set.
	engine.AddDevice(HarmonyDevice{
		ID:       "pre-enrolled-1",
		Name:     "Pre-Enrolled",
		Type:     "manual",
		Status:   "active",
		Resources: HarmonyResources{Available: true},
		LastSeen: time.Now(),
	})

	fake := &fakeHarmonySDK{
		discoverDevices: []HarmonyDevice{
			{
				ID:       "sdk-discovered-1",
				Name:     "SDK Discovered 1",
				Type:     "harmony_phone",
				Status:   "active",
				Resources: HarmonyResources{Available: true, CPUUsage: 25.0},
				LastSeen: time.Now(),
			},
			{
				ID:       "sdk-discovered-2",
				Name:     "SDK Discovered 2",
				Type:     "harmony_tablet",
				Status:   "active",
				Resources: HarmonyResources{Available: true, CPUUsage: 10.0},
				LastSeen: time.Now(),
			},
		},
	}
	engine.SetDistributedSDK(fake)

	devices, err := engine.DiscoverDevices()
	require.NoError(t, err, "SDK-backed DiscoverDevices MUST return nil error on successful enumeration")
	assert.Equal(t, 1, fake.discoverCalls, "MUST delegate to the injected SDK exactly once")
	assert.NotNil(t, fake.lastDiscoverCtx, "MUST pass a non-nil context to the SDK")

	// Merged set MUST contain BOTH the pre-enrolled device AND the
	// SDK-discovered ones.
	ids := make(map[string]bool, len(devices))
	for _, d := range devices {
		ids[d.ID] = true
	}
	assert.True(t, ids["pre-enrolled-1"], "merged result MUST preserve pre-enrolled devices")
	assert.True(t, ids["sdk-discovered-1"], "merged result MUST include SDK-discovered devices")
	assert.True(t, ids["sdk-discovered-2"], "merged result MUST include SDK-discovered devices")
	assert.Len(t, devices, 3, "no duplicates expected since all IDs differ")
}

// TestDiscoverDevices_WithSDK_DedupesById asserts that a device
// returned by the SDK with the same ID as a pre-enrolled one REPLACES
// the pre-enrolled entry rather than appearing twice (matches the
// existing AddDevice semantics).
func TestDiscoverDevices_WithSDK_DedupesById(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	engine.AddDevice(HarmonyDevice{
		ID:     "shared-id",
		Name:   "Pre-Enrolled Version",
		Status: "stale",
	})

	fake := &fakeHarmonySDK{
		discoverDevices: []HarmonyDevice{
			{
				ID:     "shared-id",
				Name:   "SDK-Discovered Version",
				Status: "active",
			},
		},
	}
	engine.SetDistributedSDK(fake)

	devices, err := engine.DiscoverDevices()
	require.NoError(t, err)
	require.Len(t, devices, 1, "de-dup by ID MUST keep cardinality at 1")
	assert.Equal(t, "SDK-Discovered Version", devices[0].Name,
		"newer SDK-returned entry MUST replace the pre-enrolled one")
	assert.Equal(t, "active", devices[0].Status)
}

// TestDiscoverDevices_WithSDK_EmptyResultHonestOutput asserts that an
// SDK that legitimately reports zero discovered devices produces an
// HONEST empty result + nil error — NOT the round-31 sentinel. The
// distinction matters: empty-slice + nil-error means "discovery
// actually ran and found nothing"; sentinel means "discovery never
// ran". With an SDK installed, the latter is impossible.
func TestDiscoverDevices_WithSDK_EmptyResultHonestOutput(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	fake := &fakeHarmonySDK{
		discoverDevices: nil,
		discoverErr:     nil,
	}
	engine.SetDistributedSDK(fake)

	devices, err := engine.DiscoverDevices()
	require.NoError(t, err, "empty SDK result + nil error is HONEST output — round-67 contract")
	assert.Empty(t, devices, "empty SDK result + no pre-enrolled devices MUST yield empty slice")
	assert.NotErrorIs(t, err, ErrHarmonyDiscoveryNotImplemented,
		"with an SDK installed, the round-31 sentinel branch MUST NOT engage — that would be a regression")
}

// TestDiscoverDevices_WithSDK_ErrorDoesNotMutateState asserts the
// anti-bluff invariant that a failed discovery MUST NOT silently erase
// locally-enrolled devices (transport failure != "cluster reset").
func TestDiscoverDevices_WithSDK_ErrorDoesNotMutateState(t *testing.T) {
	engine := NewHarmonyDistributedEngine()
	engine.AddDevice(HarmonyDevice{
		ID:     "pre-enrolled-survivor",
		Status: "active",
	})

	sdkErr := errors.New("simulated harmony device-manager transport failure")
	fake := &fakeHarmonySDK{
		discoverDevices: nil,
		discoverErr:     sdkErr,
	}
	engine.SetDistributedSDK(fake)

	devices, err := engine.DiscoverDevices()
	require.Error(t, err, "SDK failure MUST be surfaced")
	assert.ErrorIs(t, err, sdkErr)
	assert.Contains(t, err.Error(), "injected SDK DiscoverDevices",
		"wrapped error MUST identify the layer for forensics")
	assert.Nil(t, devices, "on error, returned slice MUST be nil (caller must check err first)")

	// Internal state MUST NOT have been wiped — the pre-enrolled
	// device survives so a transient transport failure does not
	// corrupt the local registry.
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	require.Len(t, engine.connectedDevices, 1, "transport failure MUST NOT erase pre-enrolled devices")
	assert.Equal(t, "pre-enrolled-survivor", engine.connectedDevices[0].ID)
}

// TestPerformSync_WithSDK_RebuildsSyncedDevicesPerRound asserts that
// syncedDevices is rebuilt per successful sync (matches GetSyncStatus's
// "most recent sync's device count" contract) rather than monotonically
// growing across calls.
func TestPerformSync_WithSDK_RebuildsSyncedDevicesPerRound(t *testing.T) {
	ds := NewHarmonyDataSync()
	fake := &fakeHarmonySDK{syncDeviceCount: 5}
	ds.SetDistributedSDK(fake)

	require.NoError(t, ds.performSync())
	_, _, count1, _ := ds.GetSyncStatus()
	assert.Equal(t, 5, count1)

	// Second round: SDK reports fewer devices — syncedDevices count
	// MUST shrink, not stay at 5.
	fake.mu.Lock()
	fake.syncDeviceCount = 2
	fake.mu.Unlock()

	require.NoError(t, ds.performSync())
	_, _, count2, _ := ds.GetSyncStatus()
	assert.Equal(t, 2, count2,
		"syncedDevices MUST be rebuilt per round — GetSyncStatus's third return reflects the MOST RECENT sync's device count, not a monotonic union")
}

// TestPerformSync_WithSDK_RecoversFromFailureOnNextSuccess asserts
// that a successful sync after a previously-failed one clears
// lastSyncErr (the receiver does not get "stuck" in the error state).
func TestPerformSync_WithSDK_RecoversFromFailureOnNextSuccess(t *testing.T) {
	ds := NewHarmonyDataSync()
	sdkErr := errors.New("transient failure")
	fake := &fakeHarmonySDK{syncErr: sdkErr}
	ds.SetDistributedSDK(fake)

	require.Error(t, ds.performSync())
	_, _, _, lastErr := ds.GetSyncStatus()
	require.Error(t, lastErr, "first sync failed — lastSyncErr MUST be populated")

	// Clear the SDK error → next sync succeeds.
	fake.mu.Lock()
	fake.syncErr = nil
	fake.syncDeviceCount = 1
	fake.mu.Unlock()

	require.NoError(t, ds.performSync(), "second sync MUST succeed once the SDK clears")
	_, _, _, lastErrAfter := ds.GetSyncStatus()
	assert.NoError(t, lastErrAfter, "successful sync after failure MUST clear lastSyncErr — receiver MUST NOT stay stuck")
}

// TestHarmonyDistributedSDK_InterfaceShape is a compile-time-style
// guard ensuring the fake satisfies the interface (also caught by
// every other test that calls SetDistributedSDK, but kept as an
// explicit assertion so a future interface widening is loudly visible
// in the test diff).
func TestHarmonyDistributedSDK_InterfaceShape(t *testing.T) {
	var _ HarmonyDistributedSDK = (*fakeHarmonySDK)(nil)

	// Document the expected method signatures via direct invocation
	// on the type-asserted fake, so the test fails to compile if
	// signatures drift.
	var sdk HarmonyDistributedSDK = &fakeHarmonySDK{
		syncDeviceCount: 42,
		discoverDevices: []HarmonyDevice{{ID: "compile-time-check"}},
	}
	ctx := context.Background()

	count, err := sdk.SyncData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 42, count)

	devices, err := sdk.DiscoverDevices(ctx)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, "compile-time-check", devices[0].ID)
}

// TestSentinelMessages_DocumentInjectionPath asserts that the
// round-67-updated sentinel messages cite the SetDistributedSDK
// injection point as the resolution path (operator-friendly forensics
// per Article XI §11.9).
func TestSentinelMessages_DocumentInjectionPath(t *testing.T) {
	assert.Contains(t, ErrHarmonyDistributedSyncNotImplemented.Error(),
		"SetDistributedSDK",
		"round-67 sentinel MUST cite the injection point as the resolution path")
	assert.Contains(t, ErrHarmonyDiscoveryNotImplemented.Error(),
		"SetDistributedSDK",
		"round-67 sentinel MUST cite the injection point as the resolution path")
}

// TestPerformSync_WithSDK_DeviceCountFormat documents the
// syncedDevices key format for forensics — callers reading raw
// syncedDevices keys see "device-<index>" placeholders rather than
// real peer IDs (the SDK exposes only a count, not per-device IDs,
// keeping the SDK surface minimal). When a future SDK widening exposes
// real peer IDs, this test will need updating in lockstep with the
// HarmonyDistributedSDK.SyncData signature change.
func TestPerformSync_WithSDK_DeviceCountFormat(t *testing.T) {
	ds := NewHarmonyDataSync()
	fake := &fakeHarmonySDK{syncDeviceCount: 2}
	ds.SetDistributedSDK(fake)

	require.NoError(t, ds.performSync())

	ds.mu.RLock()
	defer ds.mu.RUnlock()
	require.Len(t, ds.syncedDevices, 2)
	_, ok0 := ds.syncedDevices[fmt.Sprintf("device-%d", 0)]
	_, ok1 := ds.syncedDevices[fmt.Sprintf("device-%d", 1)]
	assert.True(t, ok0, "syncedDevices MUST use 'device-<index>' placeholder keys (SDK exposes count only)")
	assert.True(t, ok1)
}
