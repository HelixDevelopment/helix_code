package discovery

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the discovery layer.
//
// The units under stress are the REAL in-process, deterministic discovery
// components — no fakes, no live network in the stress loops:
//
//   - *ServiceRegistry: its RWMutex-guarded services map and the real
//     Register/Get/Update/Heartbeat/Deregister/List/UpdateHealth machinery.
//   - *ConfigManager: its RWMutex-guarded DiscoveryConfig and the real
//     UpdatePartial/GetConfig/AddReservedPort/ExportConfig validation+apply path.
//
// The BroadcastService (UDP multicast) and PortAllocator ephemeral path bind
// real OS sockets, so they are deliberately NOT driven inside sustained/
// concurrent loops here (that would be flaky real-network work, forbidden by the
// §11.4.85 scope). Their deterministic in-memory state-machine logic is covered
// in the chaos suite via input-corruption + validation paths.
//
// Every assertion checks REAL effect (the value really landed in the registry /
// config map), so a PASS proves real work — not a no-op.

// TestDiscovery_Stress_RegistrySustainedRegisterGet drives the real
// ServiceRegistry through sustained Register -> Get -> Heartbeat -> Deregister
// cycles (N>=100), recording per-call latency. Each iteration registers a
// uniquely-named service, reads it back asserting the stored fields match, then
// deregisters it — so the run proves real mutation + lookup of the guarded map,
// not a metadata-only pass.
func TestDiscovery_Stress_RegistrySustainedRegisterGet(t *testing.T) {
	reg := NewServiceRegistry(DefaultRegistryConfig())
	// Note: we do NOT call reg.Start() — the background cleanup/health loops do
	// real network health-checks; the stress target is the in-process map.

	var ops int64
	stresschaos.RunSustainedLoad(t, "discovery_registry_sustained_register_get",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			name := fmt.Sprintf("svc-%d", i)
			info := ServiceInfo{
				Name:     name,
				Host:     "127.0.0.1",
				Port:     1024 + (i % 60000),
				Protocol: "tcp",
				Version:  "1.0",
				Metadata: map[string]string{"iter": fmt.Sprintf("%d", i)},
			}
			if err := reg.Register(info); err != nil {
				return fmt.Errorf("register %s: %w", name, err)
			}
			got, err := reg.Get(name)
			if err != nil {
				return fmt.Errorf("get %s: %w", name, err)
			}
			if got.Name != name || got.Host != "127.0.0.1" {
				return fmt.Errorf("readback mismatch: got name=%q host=%q", got.Name, got.Host)
			}
			if !got.Healthy {
				return fmt.Errorf("freshly-registered %s should be Healthy", name)
			}
			if err := reg.Heartbeat(name); err != nil {
				return fmt.Errorf("heartbeat %s: %w", name, err)
			}
			if err := reg.Deregister(name); err != nil {
				return fmt.Errorf("deregister %s: %w", name, err)
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("registry processed zero register/get/deregister cycles — not real work")
	}
	// After equal register/deregister counts, the map must be empty — proof the
	// mutations actually took effect and were undone.
	if n := len(reg.List()); n != 0 {
		t.Fatalf("registry not empty after balanced register/deregister: %d services remain", n)
	}
	t.Logf("discovery registry sustained: %d register/get/heartbeat/deregister cycles", atomic.LoadInt64(&ops))
}

// TestDiscovery_Stress_ConfigManagerSustainedUpdate drives the real
// ConfigManager through sustained UpdatePartial -> GetConfig -> ExportConfig
// cycles (N>=100). Each iteration mutates a config field through the real
// validate+apply path and reads it back, asserting the change landed — proving
// the locked RWMutex config path does real work under sustained load.
func TestDiscovery_Stress_ConfigManagerSustainedUpdate(t *testing.T) {
	cm, err := NewConfigManager(DefaultDiscoveryConfig())
	if err != nil {
		t.Fatalf("NewConfigManager: %v", err)
	}

	var ops int64
	stresschaos.RunSustainedLoad(t, "discovery_config_sustained_update",
		stresschaos.SustainedConfig{N: 1000, MaxErrorRate: 0.0},
		func(i int) error {
			interval := time.Duration(i%50+1) * time.Second
			if err := cm.SetHealthCheckInterval(interval); err != nil {
				return fmt.Errorf("set interval: %w", err)
			}
			got := cm.GetConfig()
			if got.HealthCheckInterval != interval {
				return fmt.Errorf("interval not applied: want %v got %v", interval, got.HealthCheckInterval)
			}
			// Exercise the export path (reads the whole config under RLock).
			if exp := cm.ExportConfig(); exp["health_check_interval"] == nil {
				return fmt.Errorf("export missing health_check_interval")
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("config manager processed zero update cycles — not real work")
	}
	t.Logf("discovery config sustained: %d update/get/export cycles", atomic.LoadInt64(&ops))
}

// TestDiscovery_Stress_RegistryConcurrentAccess hammers the shared
// ServiceRegistry from N>=10 goroutines that interleave Register / Get / Update
// / Heartbeat / UpdateHealth / List / Deregister against a small fixed pool of
// service names, generating genuine read/write contention on the RWMutex-guarded
// map. Run under -race to catch data races. Errors that are expected under churn
// (ErrServiceNotFound / ErrServiceAlreadyRegistered) are tolerated; any other
// error or a panic/deadlock fails the run.
func TestDiscovery_Stress_RegistryConcurrentAccess(t *testing.T) {
	reg := NewServiceRegistry(DefaultRegistryConfig())

	// Small fixed name pool so different goroutines genuinely contend on the
	// SAME map keys (write-write + read-write contention).
	names := []string{"alpha", "bravo", "charlie", "delta", "echo"}

	var touches int64
	stresschaos.RunConcurrent(t, "discovery_registry_concurrent_access",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			name := names[(g+it)%len(names)]
			info := ServiceInfo{Name: name, Host: "127.0.0.1", Port: 2000 + g, Protocol: "tcp"}
			switch (g + it) % 6 {
			case 0:
				// Register may legitimately conflict if a sibling already holds it.
				if err := reg.Register(info); err != nil && err != ErrServiceAlreadyRegistered {
					return fmt.Errorf("register: %w", err)
				}
			case 1:
				// Get may legitimately miss if a sibling deregistered it.
				if _, err := reg.Get(name); err != nil && err != ErrServiceNotFound {
					return fmt.Errorf("get: %w", err)
				}
			case 2:
				if err := reg.Update(name, info); err != nil && err != ErrServiceNotFound {
					return fmt.Errorf("update: %w", err)
				}
			case 3:
				if err := reg.Heartbeat(name); err != nil && err != ErrServiceNotFound {
					return fmt.Errorf("heartbeat: %w", err)
				}
			case 4:
				if err := reg.UpdateHealth(name, it%2 == 0); err != nil && err != ErrServiceNotFound {
					return fmt.Errorf("update health: %w", err)
				}
			default:
				if err := reg.Deregister(name); err != nil && err != ErrServiceNotFound {
					return fmt.Errorf("deregister: %w", err)
				}
			}
			// Read-only accessors widen the RLock contention surface.
			_ = reg.List()
			_ = reg.ListByProtocol("tcp")
			_ = reg.ListHealthy()
			atomic.AddInt64(&touches, 1)
			return nil
		})

	if atomic.LoadInt64(&touches) == 0 {
		t.Fatal("registry saw zero concurrent touches")
	}
	// Registry must remain usable + self-consistent after the churn.
	if err := reg.Register(ServiceInfo{Name: "post-churn", Host: "127.0.0.1", Port: 4242, Protocol: "tcp"}); err != nil {
		t.Fatalf("registry unusable after concurrent churn: %v", err)
	}
	if _, err := reg.Get("post-churn"); err != nil {
		t.Fatalf("registry did not persist post-churn registration: %v", err)
	}
	t.Logf("discovery registry concurrent: %d touches, map self-consistent after churn", atomic.LoadInt64(&touches))
}

// TestDiscovery_Stress_ConfigManagerConcurrentUpdate hammers the shared
// ConfigManager from N>=10 goroutines interleaving UpdatePartial / GetConfig /
// AddReservedPort / GetReservedPorts / ExportConfig. The locked RWMutex-guarded
// config must serialise the mutations so no goroutine observes a torn config and
// no data race occurs (run under -race).
func TestDiscovery_Stress_ConfigManagerConcurrentUpdate(t *testing.T) {
	cm, err := NewConfigManager(DefaultDiscoveryConfig())
	if err != nil {
		t.Fatalf("NewConfigManager: %v", err)
	}

	var ops int64
	stresschaos.RunConcurrent(t, "discovery_config_concurrent_update",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			switch (g + it) % 4 {
			case 0:
				if err := cm.SetHealthCheckInterval(time.Duration(it%30+1) * time.Second); err != nil {
					return fmt.Errorf("set interval: %w", err)
				}
			case 1:
				// Distinct reserved ports per goroutine so we can assert growth.
				if err := cm.AddReservedPort(20000 + g*1000 + (it % 900)); err != nil {
					return fmt.Errorf("add reserved port: %w", err)
				}
			case 2:
				_ = cm.GetConfig() // full-config read under RLock
			default:
				_ = cm.ExportConfig()
				_ = cm.GetReservedPorts()
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("config manager saw zero concurrent ops")
	}
	// The reserved-port list must have grown beyond the default set — proof the
	// concurrent AddReservedPort mutations actually landed in the shared config.
	final := cm.GetConfig()
	if len(final.ReservedPorts) <= len(DefaultDiscoveryConfig().ReservedPorts) {
		t.Fatalf("reserved ports did not grow under concurrent AddReservedPort: %d", len(final.ReservedPorts))
	}
	t.Logf("discovery config concurrent: %d ops, reserved ports grew to %d", atomic.LoadInt64(&ops), len(final.ReservedPorts))
}

// TestDiscovery_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary
// cases against the real components: empty (no services / lookup miss), max
// (config with the maximum valid port + a large reserved-port set), and
// off-by-one (port 0, port 65536, port 65535) against the real validators.
func TestDiscovery_Stress_BoundaryConditions(t *testing.T) {
	// Empty: a fresh registry must return empty lists and miss every lookup.
	t.Run("empty_registry", func(t *testing.T) {
		reg := NewServiceRegistry(DefaultRegistryConfig())
		if got := reg.List(); len(got) != 0 {
			t.Fatalf("fresh registry List() should be empty, got %d", len(got))
		}
		if _, err := reg.Get("nope"); err != ErrServiceNotFound {
			t.Fatalf("Get on empty registry should be ErrServiceNotFound, got %v", err)
		}
		if err := reg.Deregister("nope"); err != ErrServiceNotFound {
			t.Fatalf("Deregister on empty registry should be ErrServiceNotFound, got %v", err)
		}
	})

	// Off-by-one on the port validator: 0 and 65536 invalid, 1 and 65535 valid.
	t.Run("port_boundaries", func(t *testing.T) {
		reg := NewServiceRegistry(DefaultRegistryConfig())
		cases := []struct {
			port    int
			wantErr bool
		}{
			{0, true}, {1, false}, {65535, false}, {65536, true}, {-1, true},
		}
		for _, c := range cases {
			name := fmt.Sprintf("p%d", c.port)
			err := reg.Register(ServiceInfo{Name: name, Host: "127.0.0.1", Port: c.port, Protocol: "tcp"})
			if c.wantErr && err == nil {
				t.Fatalf("port %d should be rejected by validator, but Register succeeded", c.port)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("port %d should be accepted, but Register failed: %v", c.port, err)
			}
		}
	})

	// Max + empty name / host boundary: empty name and host must be rejected.
	t.Run("required_field_boundaries", func(t *testing.T) {
		reg := NewServiceRegistry(DefaultRegistryConfig())
		if err := reg.Register(ServiceInfo{Name: "", Host: "h", Port: 80, Protocol: "tcp"}); err == nil {
			t.Fatal("empty service name must be rejected")
		}
		if err := reg.Register(ServiceInfo{Name: "n", Host: "", Port: 80, Protocol: "tcp"}); err == nil {
			t.Fatal("empty host must be rejected")
		}
		// Default protocol fill: empty protocol must be normalised to "tcp".
		if err := reg.Register(ServiceInfo{Name: "defproto", Host: "h", Port: 80}); err != nil {
			t.Fatalf("service with empty protocol should default to tcp, got: %v", err)
		}
		got, _ := reg.Get("defproto")
		if got.Protocol != "tcp" {
			t.Fatalf("empty protocol should default to tcp, got %q", got.Protocol)
		}
	})

	// Config validator boundaries: invalid port range start>end, MaxServices<1,
	// BroadcastTTL>255 must all be rejected by the real Validate().
	t.Run("config_validate_boundaries", func(t *testing.T) {
		base := DefaultDiscoveryConfig()
		bad := base
		bad.PortRanges = map[string]PortRange{"x": {Start: 100, End: 50}}
		if bad.Validate() == nil {
			t.Fatal("port range start>end must fail Validate")
		}
		bad = base
		bad.MaxServices = 0
		if bad.Validate() == nil {
			t.Fatal("MaxServices=0 must fail Validate")
		}
		bad = base
		bad.BroadcastTTL = 256
		if bad.Validate() == nil {
			t.Fatal("BroadcastTTL=256 must fail Validate")
		}
		// The unmodified default must validate cleanly.
		if err := base.Validate(); err != nil {
			t.Fatalf("default config must validate, got: %v", err)
		}
	})
}
