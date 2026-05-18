// Package main — Harmony OS distributed primitives.
//
// This file is intentionally NOT build-tagged so the distributed
// primitives compile under BOTH the GUI build (`!nogui`, default —
// driven by main.go) and the headless CLI build (`nogui` — driven by
// main_nogui.go). Originally these types lived only in main.go under
// `//go:build !nogui` and were therefore impossible to exercise on hosts
// without the Fyne / X11 toolchain (round-67 forensic note: the
// round-31 sentinel tests could not be executed on CI hosts missing
// libXcursor-devel). The relocation enables runtime-evidence capture
// per Article XI §11.9 without changing the public contract of either
// type.
//
// Round-67 §11.4 anti-bluff extension (2026-05-19) adds the
// HarmonyDistributedSDK injection point + SetDistributedSDK methods so
// consumers that DO have a real Harmony OS Go binding (typically via a
// C++/cgo shim around the OpenHarmony native APIs, or an external
// JS-bridge subprocess) can wire performSync + DiscoverDevices to the
// real cluster while keeping the round-31 sentinels as the loud,
// programmatically-detectable default for builds without such a
// binding. The sentinels are NOT replaced by a no-op success path —
// the only way to silence them is to inject a non-nil SDK whose
// SyncData / DiscoverDevices methods return real data.
//
// Round-67 SDK availability investigation (2026-05-19):
//
//   - go.mod / go.sum: ZERO Harmony OS Go SDK dependencies (grep for
//     "harmony", "openharmony", "huawei", "hisys", "hisense" returned
//     only an unrelated Ollama Harmony chat-format parser).
//   - Filesystem sweep of dependencies/ tree: ZERO Harmony OS Go SDK
//     submodules.
//   - Public ecosystem state (2026-05): the Harmony OS distributed
//     data manager + device manager APIs are exposed primarily through
//     ArkTS / JS (`@ohos.distributedData`, `@ohos.distributedHardware`)
//     and C++ (`OHOS::DistributedKv`, `OHOS::DistributedHardware`).
//     There is no first-party Go SDK; the recommended integration path
//     for Go consumers is (a) a C++ shim invoked via cgo, OR (b) a
//     long-lived JS-bridge subprocess speaking JSON over stdio.
//
// Outcome: Outcome B per round-67 plan — architectural extension via
// injection point + sentinel retention as the no-SDK default. Consumers
// that ship their own binding (cgo shim, JS bridge, OEM-specific SDK)
// call SetDistributedSDK at boot to wire real behaviour.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// HarmonyDistributedEngine manages distributed task scheduling across
// Harmony OS devices.
type HarmonyDistributedEngine struct {
	connectedDevices []HarmonyDevice
	taskScheduler    *HarmonyTaskScheduler
	dataSync         *HarmonyDataSync
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc

	// distributedSDK is the round-67 §11.4 injection point. When nil
	// (the default for builds without a Harmony OS Go binding),
	// DiscoverDevices returns ErrHarmonyDiscoveryNotImplemented from
	// the empty-slice branch — preserving the round-31 sentinel. When
	// non-nil, DiscoverDevices delegates to its DiscoverDevices method.
	// Set via SetDistributedSDK. Read under e.mu.
	distributedSDK HarmonyDistributedSDK
}

// HarmonyDevice represents a connected Harmony OS device.
type HarmonyDevice struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Status       string           `json:"status"`
	Capabilities []string         `json:"capabilities"`
	Resources    HarmonyResources `json:"resources"`
	LastSeen     time.Time        `json:"last_seen"`
}

// HarmonyResources represents device resources.
type HarmonyResources struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	GPUUsage    float64 `json:"gpu_usage"`
	Available   bool    `json:"available"`
}

// HarmonyTaskScheduler schedules tasks across Harmony OS ecosystem.
type HarmonyTaskScheduler struct {
	schedulingPolicy string
	taskQueue        []*ScheduledTask
	priorityLevels   map[string]int
	mu               sync.RWMutex
}

// ScheduledTask represents a task scheduled for distributed execution.
type ScheduledTask struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	DeviceID    string    `json:"device_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// HarmonyDataSync synchronizes data across Harmony OS devices.
type HarmonyDataSync struct {
	syncInterval  time.Duration
	syncEnabled   bool
	lastSync      time.Time
	syncedDevices map[string]time.Time
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc

	// lastSyncErr captures the most recent (*HarmonyDataSync).performSync()
	// outcome. nil means "the last sync attempt actually exchanged state with
	// the Harmony OS distributed data manager"; non-nil means "the last
	// attempt failed and the values reported by GetSyncStatus are stale".
	// Replaces the previous design where performSync stamped lastSync = now
	// unconditionally and reported PASS-bluff via GetSyncStatus.
	lastSyncErr error

	// distributedSDK is the round-67 §11.4 injection point. When nil
	// (the default for builds without a Harmony OS Go binding),
	// performSync returns ErrHarmonyDistributedSyncNotImplemented
	// WITHOUT advancing lastSync — preserving the round-31 sentinel
	// (the "Last Sync: Just now" PASS-bluff anti-regression). When
	// non-nil, performSync delegates to its SyncData method, records
	// the returned device count under syncedDevices keyed by
	// per-device timestamps, and advances lastSync ONLY on success.
	// Set via SetDistributedSDK. Read under ds.mu.
	distributedSDK HarmonyDistributedSDK
}

// HarmonyDistributedSDK is the round-67 §11.4 anti-bluff injection
// interface that consumers implement to wire (*HarmonyDataSync).performSync
// and (*HarmonyDistributedEngine).DiscoverDevices to a real Harmony OS
// distributed-data session + real device-manager session.
//
// Why an interface (not a build tag, not a direct dependency): the
// Harmony OS distributed APIs are not exposed as a first-party Go
// SDK as of 2026-05 (see file-header investigation). The realistic
// integrations are vendor-specific (cgo shim around C++ `OHOS::
// DistributedKv`, JS-bridge subprocess speaking JSON to ArkTS, OEM
// SDK, etc.) — none of which belong inside HelixCode's source tree
// per CONST-051(B) (submodule/decoupling) and Article XII §12.1
// (no-secret-leak: OEM SDKs frequently ship with signing keys baked
// in). The injection point keeps this package fully decoupled while
// providing a clean handshake for consumers that DO have a binding.
//
// Implementations MUST:
//
//   - SyncData: push + pull a full round-trip with the distributed-data
//     manager. Return the number of devices that successfully
//     round-tripped (i.e. count for which we received an ACK). Return
//     a non-nil error on any failure — the caller treats a non-nil
//     error as "do NOT advance lastSync, do NOT populate
//     syncedDevices" (the round-31 anti-bluff contract). Returning
//     (0, nil) is permitted ONLY when the local cluster legitimately
//     has zero peer devices reachable AND the sync handshake itself
//     succeeded — implementations SHOULD prefer returning a
//     descriptive error in that case so the caller can distinguish
//     "no peers" from "transport failure".
//   - DiscoverDevices: enumerate currently reachable Harmony OS
//     devices via the device-manager API. Return the slice (empty
//     slice + nil error means "discovery ran, no devices found",
//     which is HONEST output unlike the round-31 bluff branch).
//     Return a non-nil error on transport / permission / SDK failure.
//
// The interface methods take a context.Context so consumers can
// honour cancellation + deadlines from the caller (UI tick, CLI
// command timeout, monitoring poll).
type HarmonyDistributedSDK interface {
	// SyncData performs a full round-trip with the Harmony OS
	// distributed-data manager (KVManager / SingleKVStore /
	// DeviceKVStore). Returns the number of peer devices that
	// successfully round-tripped, or a non-nil error on failure.
	SyncData(ctx context.Context) (deviceCount int, err error)

	// DiscoverDevices enumerates currently reachable Harmony OS
	// devices via the device-manager API. Returns the discovered
	// devices (possibly empty + nil error for honest "no peers"
	// output) or a non-nil error on transport / SDK failure.
	DiscoverDevices(ctx context.Context) ([]HarmonyDevice, error)
}

// ErrHarmonyDistributedSyncNotImplemented is returned by
// (*HarmonyDataSync).performSync() and surfaced through GetSyncStatus
// when the Harmony OS distributed-data SDK has not been injected via
// SetDistributedSDK.
//
// Forensic anchor (round-31 §11.4 audit, 2026-05-18): the previous
// implementation only did `ds.lastSync = time.Now()` + a log line and
// returned no error. helix-harmony's distributed-sync UI (the
// `createDistributedServicesTab` card) and the nogui `distributed sync`
// CLI command therefore reported "Sync Status: Enabled / Last Sync:
// Just now / Synced Devices: 0" forever regardless of actual cluster
// state — a §11.4 CRITICAL PASS-bluff because the surface promised
// cross-device synchronization while the body did nothing of the kind.
//
// Round-67 §11.4 extension (2026-05-19): the sentinel survives as the
// default no-SDK behaviour. To clear it, a consumer that has a real
// Harmony OS Go binding (cgo shim around `OHOS::DistributedKv`, JS
// bridge to ArkTS `@ohos.distributedData`, OEM SDK, etc.) calls
// (*HarmonyDataSync).SetDistributedSDK(impl) at boot — performSync
// then delegates to impl.SyncData(ctx) and only stamps lastSync on
// success. There is NO no-op success path: an absent binding ALWAYS
// produces this sentinel.
var ErrHarmonyDistributedSyncNotImplemented = errors.New(
	"harmony_os: distributed sync has not been wired to the real Harmony OS " +
		"distributed-data SDK — performSync previously only stamped " +
		"lastSync=time.Now() and logged success, reporting 'Synced Devices: 0' " +
		"forever regardless of actual cluster state (§11.4 CRITICAL: " +
		"helix-harmony distributed sync is a no-op). Implement against the " +
		"Harmony OS distributed data manager API (KVManager / SingleKVStore / " +
		"DeviceKVStore) and inject via (*HarmonyDataSync).SetDistributedSDK, " +
		"or remove the command and document non-support",
)

// ErrHarmonyDiscoveryNotImplemented is returned by
// (*HarmonyDistributedEngine).DiscoverDevices when invoked without
// any devices having been added through AddDevice AND no
// HarmonyDistributedSDK has been injected via SetDistributedSDK.
//
// Forensic anchor (round-31 §11.4 audit, 2026-05-18): the previous
// implementation returned `e.connectedDevices` unconditionally with
// no error and the inline comment "In a real implementation, this
// would use Harmony OS distributed device discovery / For now, we
// return the currently connected devices". On a fresh app launch
// (no AddDevice calls yet) the workers-tab "Discover Devices" button
// always reported "Found 0 Harmony devices" with no indication that
// discovery was a stub. That is a §11.4 HIGH PASS-bluff: the surface
// promised real distributed discovery, the body did nothing.
//
// Round-67 §11.4 extension (2026-05-19): when a HarmonyDistributedSDK
// is injected via SetDistributedSDK, DiscoverDevices delegates to its
// DiscoverDevices method and merges the result into connectedDevices
// (de-duped by Device.ID). In that branch an empty result + nil error
// is honest "discovery ran, no peers found" output. The sentinel
// survives only when NO SDK is injected AND no devices have been
// enrolled via AddDevice.
var ErrHarmonyDiscoveryNotImplemented = errors.New(
	"harmony_os: distributed device discovery has not been wired to the " +
		"real Harmony OS device manager — DiscoverDevices previously logged " +
		"and returned an empty list, so helix-harmony 'distributed discover' " +
		"always reports 'No devices found' regardless of cluster state " +
		"(§11.4 HIGH: discovery feature is a no-op). Implement against the " +
		"Harmony OS device discovery API (DeviceManager) and inject via " +
		"(*HarmonyDistributedEngine).SetDistributedSDK, or remove the " +
		"command and document non-support",
)

// ErrPowerMetricsNotAvailable signals that
// (*HarmonyDistributedEngine).findPowerEfficientDevice could not honour
// the "power_efficient" scheduling policy with real power telemetry
// because the HarmonyResources type does not carry a power-consumption
// field (no battery level, no watts-drawn, no thermal-design-power
// envelope). The caller (ScheduleTask) consumes the returned device
// nonetheless — using lowest-aggregate-resource-usage as a proxy
// (CPUUsage + MemoryUsage + GPUUsage minimisation, which positively
// correlates with active-state power draw) — but logs the sentinel so
// the surface does not claim it has performed real power-efficient
// scheduling.
//
// Forensic anchor (round-34 §11.4 audit, 2026-05-18): the previous
// implementation returned the FIRST active+available device with the
// comment "in a real implementation, would consider power metrics".
// The caller (ScheduleTask, "power_efficient" branch) routed real
// workloads to that device, certifying to operators that the platform
// was making power-conscious scheduling decisions when it was making
// arrival-order decisions. That is a §11.4 HIGH PASS-bluff.
var ErrPowerMetricsNotAvailable = errors.New(
	"harmony_os: power-efficient scheduling has no real power telemetry — " +
		"HarmonyResources does not carry battery level, watts drawn, or thermal " +
		"envelope fields. Falling back to lowest-aggregate-resource-usage as a " +
		"proxy. Wire HarmonyOS PowerManager API + extend HarmonyResources to " +
		"clear this sentinel",
)

// NewHarmonyDistributedEngine creates a new distributed engine.
func NewHarmonyDistributedEngine() *HarmonyDistributedEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &HarmonyDistributedEngine{
		connectedDevices: make([]HarmonyDevice, 0),
		taskScheduler: &HarmonyTaskScheduler{
			schedulingPolicy: "balanced",
			taskQueue:        make([]*ScheduledTask, 0),
			priorityLevels:   map[string]int{"low": 1, "normal": 2, "high": 3, "critical": 4},
		},
		dataSync: NewHarmonyDataSync(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// NewHarmonyDataSync creates a new data sync manager.
func NewHarmonyDataSync() *HarmonyDataSync {
	ctx, cancel := context.WithCancel(context.Background())
	return &HarmonyDataSync{
		syncInterval:  30 * time.Second,
		syncEnabled:   true,
		lastSync:      time.Now(),
		syncedDevices: make(map[string]time.Time),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetDistributedSDK installs a HarmonyDistributedSDK implementation on
// the distributed engine. The injected SDK is consulted by
// DiscoverDevices; when nil, DiscoverDevices falls back to the
// round-31 sentinel + AddDevice-only contract. Passing nil
// deliberately UN-installs a previously-injected SDK (useful for
// test isolation + graceful degradation when the underlying binding
// reports a permanent failure).
//
// The SDK is also propagated to the underlying HarmonyDataSync so a
// single injection point wires BOTH discovery and sync — matching
// the round-67 contract that the two capabilities ship together (a
// consumer with a real device-manager binding nearly always has a
// distributed-data-manager binding too; both live in the same OHOS
// distributed subsystem).
func (e *HarmonyDistributedEngine) SetDistributedSDK(impl HarmonyDistributedSDK) {
	e.mu.Lock()
	e.distributedSDK = impl
	ds := e.dataSync
	e.mu.Unlock()

	if ds != nil {
		ds.SetDistributedSDK(impl)
	}
}

// SetDistributedSDK installs a HarmonyDistributedSDK implementation on
// the data-sync receiver. See HarmonyDistributedEngine.SetDistributedSDK
// for the round-67 design rationale. Passing nil deliberately
// UN-installs a previously-injected SDK.
func (ds *HarmonyDataSync) SetDistributedSDK(impl HarmonyDistributedSDK) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.distributedSDK = impl
}

// DiscoverDevices discovers nearby Harmony OS devices. Decision tree:
//
//  1. If a HarmonyDistributedSDK has been injected via
//     SetDistributedSDK, delegate to its DiscoverDevices method, merge
//     the discovered devices into connectedDevices (de-duped by
//     Device.ID, newer entry wins), and return the FULL connected
//     slice with a nil error. A non-nil error from the SDK is
//     returned verbatim and the connectedDevices slice is NOT mutated
//     (transport failure must not silently rewrite local state).
//  2. If no SDK is injected AND at least one HarmonyDevice has been
//     added through AddDevice, return the slice with a nil error
//     (the AddDevice call-path is the legitimate non-discovery
//     enrolment surface — worker-tab manual enrolment — and remains
//     functional).
//  3. If no SDK is injected AND the slice is empty, return it with
//     ErrHarmonyDiscoveryNotImplemented so callers can distinguish
//     "real discovery genuinely found zero devices" (an honest output
//     once an SDK is injected — covered by branch 1's empty-slice +
//     nil-error case) from "discovery never ran". The UI and CLI MUST
//     surface this error instead of printing the previous "Found 0
//     Harmony devices" / "No devices found" bluff message.
//
// Round-67 §11.4 extension (2026-05-19): branch 1 is new. The
// pre-round-67 contract (branches 2 + 3) is preserved exactly when
// no SDK is injected, so the existing round-31 sentinel tests
// continue to pass without modification.
func (e *HarmonyDistributedEngine) DiscoverDevices() ([]HarmonyDevice, error) {
	e.mu.Lock()
	sdk := e.distributedSDK
	e.mu.Unlock()

	if sdk != nil {
		// Branch 1: delegate to injected SDK. Use the engine's
		// context so cancellation propagates to the binding.
		discovered, err := sdk.DiscoverDevices(e.ctx)
		if err != nil {
			// Surface verbatim; do NOT mutate connectedDevices on
			// transport failure (round-67 anti-bluff: a failed
			// discovery must NOT silently erase locally-enrolled
			// devices).
			return nil, fmt.Errorf("harmony_os: injected SDK DiscoverDevices: %w", err)
		}

		// Merge into connectedDevices, de-duped by ID (newer entry
		// wins; matches AddDevice's existing replace-by-ID semantics).
		e.mu.Lock()
		defer e.mu.Unlock()
		for _, d := range discovered {
			replaced := false
			for i := range e.connectedDevices {
				if e.connectedDevices[i].ID == d.ID {
					e.connectedDevices[i] = d
					replaced = true
					break
				}
			}
			if !replaced {
				e.connectedDevices = append(e.connectedDevices, d)
			}
		}
		// Return a defensive copy so callers can't mutate internal
		// state through the returned slice header.
		out := make([]HarmonyDevice, len(e.connectedDevices))
		copy(out, e.connectedDevices)
		return out, nil
	}

	// Branches 2 + 3: pre-round-67 sentinel contract.
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.connectedDevices) == 0 {
		return e.connectedDevices, ErrHarmonyDiscoveryNotImplemented
	}
	return e.connectedDevices, nil
}

// AddDevice adds a device to the distributed network.
func (e *HarmonyDistributedEngine) AddDevice(device HarmonyDevice) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if device already exists
	for i, d := range e.connectedDevices {
		if d.ID == device.ID {
			e.connectedDevices[i] = device
			return
		}
	}
	e.connectedDevices = append(e.connectedDevices, device)
}

// RemoveDevice removes a device from the distributed network.
func (e *HarmonyDistributedEngine) RemoveDevice(deviceID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, d := range e.connectedDevices {
		if d.ID == deviceID {
			e.connectedDevices = append(e.connectedDevices[:i], e.connectedDevices[i+1:]...)
			return
		}
	}
}

// ScheduleTask schedules a task across available devices.
func (e *HarmonyDistributedEngine) ScheduleTask(taskType, description string, priority int) (*ScheduledTask, error) {
	e.mu.RLock()
	devices := e.connectedDevices
	e.mu.RUnlock()

	// Find the best device based on scheduling policy
	var targetDevice *HarmonyDevice
	switch e.taskScheduler.schedulingPolicy {
	case "balanced":
		targetDevice = e.findBalancedDevice(devices)
	case "performance":
		targetDevice = e.findPerformanceDevice(devices)
	case "power_efficient":
		targetDevice = e.findPowerEfficientDevice(devices)
	default:
		targetDevice = e.findBalancedDevice(devices)
	}

	task := &ScheduledTask{
		ID:          fmt.Sprintf("htask-%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Priority:    priority,
		Status:      "scheduled",
		CreatedAt:   time.Now(),
	}

	if targetDevice != nil {
		task.DeviceID = targetDevice.ID
	} else {
		task.DeviceID = "local"
	}

	e.taskScheduler.mu.Lock()
	e.taskScheduler.taskQueue = append(e.taskScheduler.taskQueue, task)
	e.taskScheduler.mu.Unlock()

	return task, nil
}

// findBalancedDevice finds a device with balanced resource usage.
func (e *HarmonyDistributedEngine) findBalancedDevice(devices []HarmonyDevice) *HarmonyDevice {
	var best *HarmonyDevice
	bestScore := float64(-1)

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}

		// Score based on available resources (lower usage = higher score)
		score := (100 - d.Resources.CPUUsage) + (100 - d.Resources.MemoryUsage)
		if score > bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// findPerformanceDevice finds the device with best performance characteristics.
func (e *HarmonyDistributedEngine) findPerformanceDevice(devices []HarmonyDevice) *HarmonyDevice {
	var best *HarmonyDevice
	bestScore := float64(-1)

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}

		// Prioritize devices with GPU and low CPU usage
		score := (100 - d.Resources.CPUUsage)
		if d.Resources.GPUUsage > 0 {
			score += 50 - d.Resources.GPUUsage/2
		}
		if score > bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// findPowerEfficientDevice picks the available device that — in the
// absence of real HarmonyOS power telemetry on the HarmonyDevice /
// HarmonyResources types — minimises aggregate active-resource usage
// (CPU+Memory+GPU). This is a documented proxy, not a real
// power-consumption measurement; see ErrPowerMetricsNotAvailable for
// the §11.4 forensic anchor explaining the gap.
//
// Returns nil if no device is active+available.
func (e *HarmonyDistributedEngine) findPowerEfficientDevice(devices []HarmonyDevice) *HarmonyDevice {
	// Loudly mark the gap in runtime evidence captures.
	log.Printf("harmony_os: %v", ErrPowerMetricsNotAvailable)

	var best *HarmonyDevice
	const noCandidate = float64(1 << 30)
	bestScore := noCandidate

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}
		score := d.Resources.CPUUsage + d.Resources.MemoryUsage + d.Resources.GPUUsage
		if score < bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// GetScheduledTasks returns all scheduled tasks.
func (e *HarmonyDistributedEngine) GetScheduledTasks() []*ScheduledTask {
	e.taskScheduler.mu.RLock()
	defer e.taskScheduler.mu.RUnlock()

	tasks := make([]*ScheduledTask, len(e.taskScheduler.taskQueue))
	copy(tasks, e.taskScheduler.taskQueue)
	return tasks
}

// Stop stops the distributed engine.
func (e *HarmonyDistributedEngine) Stop() {
	e.cancel()
	e.dataSync.Stop()
}

// StartSync starts the data synchronization process.
func (ds *HarmonyDataSync) StartSync() {
	if !ds.syncEnabled {
		return
	}

	go func() {
		ticker := time.NewTicker(ds.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ds.ctx.Done():
				return
			case <-ticker.C:
				ds.performSync()
			}
		}
	}()
}

// Stop stops the data sync process.
func (ds *HarmonyDataSync) Stop() {
	ds.cancel()
}

// performSync performs the actual data synchronization. Decision tree:
//
//  1. If a HarmonyDistributedSDK has been injected via
//     SetDistributedSDK, delegate to its SyncData method. On success
//     (nil error), advance lastSync to time.Now(), populate
//     syncedDevices with the device-count-derived entries, and clear
//     lastSyncErr. On failure (non-nil error), preserve the
//     pre-round-31 anti-bluff invariant: do NOT advance lastSync, do
//     NOT mutate syncedDevices, record the error in lastSyncErr, and
//     return the error wrapped with context.
//  2. If no SDK is injected, return ErrHarmonyDistributedSyncNotImplemented
//     WITHOUT advancing lastSync (the round-31 contract — preserves
//     the "Last Sync: Just now" anti-regression).
//
// Round-67 §11.4 extension (2026-05-19): branch 1 is new. The
// pre-round-67 contract (branch 2) is preserved exactly when no SDK
// is injected, so the existing round-31 sentinel tests continue to
// pass without modification.
func (ds *HarmonyDataSync) performSync() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.distributedSDK != nil {
		// Branch 1: delegate to injected SDK. Use the receiver's
		// context so cancellation propagates to the binding.
		deviceCount, err := ds.distributedSDK.SyncData(ds.ctx)
		if err != nil {
			// Anti-bluff invariant: failed sync MUST NOT advance
			// lastSync (the round-31 "Last Sync: Just now" anti-bluff
			// applies equally to a real-SDK failure as to the
			// no-SDK sentinel branch).
			wrapped := fmt.Errorf("harmony_os: injected SDK SyncData: %w", err)
			ds.lastSyncErr = wrapped
			log.Printf("Harmony data sync FAILED via injected SDK: %v (lastSync timestamp NOT advanced)", wrapped)
			return wrapped
		}

		// Success: advance lastSync, populate syncedDevices,
		// clear lastSyncErr. The SDK reports a device COUNT (not
		// per-device IDs — keeping the SDK surface minimal); record
		// synthetic positional keys ("device-0", "device-1", ...)
		// so GetSyncStatus's third return continues to reflect the
		// number of devices that round-tripped.
		now := time.Now()
		ds.lastSync = now
		// Rebuild syncedDevices from this round-trip's outcome —
		// callers expect len(syncedDevices) to match the most
		// recent sync's device count, not an ever-growing union.
		ds.syncedDevices = make(map[string]time.Time, deviceCount)
		for i := 0; i < deviceCount; i++ {
			ds.syncedDevices[fmt.Sprintf("device-%d", i)] = now
		}
		ds.lastSyncErr = nil
		return nil
	}

	// Branch 2: no SDK injected — round-31 sentinel contract.
	ds.lastSyncErr = ErrHarmonyDistributedSyncNotImplemented
	log.Printf("Harmony data sync NOT performed: %v (lastSync timestamp NOT advanced)", ds.lastSyncErr)
	return ds.lastSyncErr
}

// GetSyncStatus returns the current sync status. The final error
// return is non-nil if the most recent performSync call failed —
// callers MUST inspect it instead of trusting the (enabled, lastSync,
// syncedDevices) tuple in isolation, otherwise they recreate the
// PASS-bluff fixed in round-31 §11.4.
func (ds *HarmonyDataSync) GetSyncStatus() (bool, time.Time, int, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return ds.syncEnabled, ds.lastSync, len(ds.syncedDevices), ds.lastSyncErr
}
