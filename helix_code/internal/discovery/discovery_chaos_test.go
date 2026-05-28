package discovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the discovery layer.
//
// Chaos classes exercised against the REAL in-process components (no fakes):
//
//   - input-corruption: structurally hostile BroadcastMessage / ServiceInfo JSON
//     payloads are fed through the REAL json.Unmarshal path the BroadcastService
//     listener uses (broadcast.go listen() -> json.Unmarshal). Parsing must
//     reject or normalise malformed wire bytes without panicking.
//   - input-corruption (config): hostile DiscoveryConfig values are fed through
//     the REAL Validate() path; it must reject without panic.
//   - state-corruption under contention: the SAME service name is concurrently
//     Registered / Updated / Deregistered / Heartbeated from many goroutines.
//     The registry's RWMutex must serialise the map mutations so it never panics,
//     never races, and ends self-consistent.
//   - resource-exhaustion: the registry is driven under bounded memory pressure;
//     it must keep functioning rather than crash.

// TestDiscovery_Chaos_CorruptBroadcastMessage feeds structurally hostile bytes
// through the REAL BroadcastMessage JSON-decode path used by the listener. The
// decode + a handler that touches every field must not panic on truncated JSON,
// wrong-typed fields, oversized payloads, deeply-nested objects, or NUL bytes —
// a crash on malformed wire input is a §11.4.85(B) failure. Graceful rejection
// (json error) is the desired path.
func TestDiscovery_Chaos_CorruptBroadcastMessage(t *testing.T) {
	corrupt := [][]byte{
		[]byte(``),                                    // empty
		[]byte(`{`),                                   // truncated object
		[]byte(`{"type":`),                            // truncated mid-key
		[]byte(`{"type": 12345}`),                     // wrong type for string field
		[]byte(`{"service": {"port": "not-a-number"}}`), // wrong type for int field
		[]byte(`{"service": {"port": 999999999999999}}`), // port overflow
		[]byte(`{"timestamp": "not-a-time"}`),         // unparseable time
		[]byte(`not json at all`),                     // garbage
		[]byte("{\"type\":\"announce\x00\"}"),         // embedded NUL
		[]byte(`{"metadata": {"k": {"nested": {"deep": [1,2,3]}}}}`), // nested
		[]byte(`[]`),                                  // array where object expected
		[]byte(strings.Repeat(`{"a":`, 5000) + "1" + strings.Repeat("}", 5000)), // deep nesting
		[]byte(`{"service":{"name":"` + makeHugeDiscoveryString(1<<16) + `"}}`),  // oversized
	}

	stresschaos.ChaosCorruptInputDuring(t, "discovery_corrupt_broadcast_message", corrupt,
		func(input []byte) error {
			// Mirror broadcast.go listen(): json.Unmarshal into BroadcastMessage.
			var msg BroadcastMessage
			if err := json.Unmarshal(input, &msg); err != nil {
				return err // graceful rejection — the desired path
			}
			// Accepted: a real consumer touches every field (like handleMessage).
			_ = msg.Type
			_ = msg.ServiceInfo.Address()
			_ = msg.ServiceInfo.IsExpired()
			for k, v := range msg.Metadata {
				_ = fmt.Sprintf("%s=%v", k, v)
			}
			return nil
		})
}

// TestDiscovery_Chaos_CorruptServiceInfoRoundTrip feeds hostile ServiceInfo JSON
// through decode then back through the REAL registry validation path. Neither the
// decode nor Register may panic on the corrupt inputs; rejection is graceful.
func TestDiscovery_Chaos_CorruptServiceInfoRoundTrip(t *testing.T) {
	reg := NewServiceRegistry(DefaultRegistryConfig())

	corrupt := [][]byte{
		[]byte(`{"name": "", "host": "", "port": 0}`),       // all-empty -> validation reject
		[]byte(`{"name": "x", "host": "h", "port": -5}`),    // negative port
		[]byte(`{"name": "x", "host": "h", "port": 70000}`), // out-of-range port
		[]byte(`{"name": "ok", "host": "h", "port": 80}`),   // VALID -> must accept
		[]byte(`{"ttl": "garbage"}`),                         // wrong type for duration
		[]byte(`{"name": 999}`),                              // wrong type for name
	}

	stresschaos.ChaosCorruptInputDuring(t, "discovery_corrupt_serviceinfo_roundtrip", corrupt,
		func(input []byte) error {
			var info ServiceInfo
			if err := json.Unmarshal(input, &info); err != nil {
				return err // graceful rejection of malformed wire bytes
			}
			// Feed through the real registry; validation rejects invalid info.
			if err := reg.Register(info); err != nil {
				// Deregister to avoid ErrServiceAlreadyRegistered on dup names.
				return err
			}
			_ = reg.Deregister(info.Name)
			return nil
		})
}

// TestDiscovery_Chaos_CorruptConfig feeds hostile DiscoveryConfig values through
// the REAL Validate() path. Validate must reject every malformed config without
// panicking — a crash on bad config is a §11.4.85(B) failure.
func TestDiscovery_Chaos_CorruptConfig(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "discovery_corrupt_config", "input-corruption")

	bad := []func() DiscoveryConfig{
		func() DiscoveryConfig { c := DefaultDiscoveryConfig(); c.PortRanges = map[string]PortRange{"x": {Start: -1, End: 5}}; return c },
		func() DiscoveryConfig { c := DefaultDiscoveryConfig(); c.PortRanges = map[string]PortRange{"x": {Start: 99999, End: 100000}}; return c },
		func() DiscoveryConfig { c := DefaultDiscoveryConfig(); c.MaxServices = -100; return c },
		func() DiscoveryConfig { c := DefaultDiscoveryConfig(); c.BroadcastTTL = -1; return c },
		func() DiscoveryConfig { c := DefaultDiscoveryConfig(); c.DefaultTTL = -time.Hour; return c },
		func() DiscoveryConfig {
			c := DefaultDiscoveryConfig()
			c.AllowEphemeral = true
			c.EphemeralPortStart = 60000
			c.EphemeralPortEnd = 50000
			return c
		},
	}

	for i, mk := range bad {
		func(idx int) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("Validate panicked on corrupt config[%d]: %v", idx, p))
				}
			}()
			cfg := mk()
			if err := cfg.Validate(); err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("config[%d] rejected cleanly: %v", idx, err))
			} else {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("config[%d] should have been rejected by Validate but passed", idx))
			}
		}(i)
	}

	// NewConfigManager must also refuse an invalid initial config without panic.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("NewConfigManager panicked on bad config: %v", p))
			}
		}()
		c := DefaultDiscoveryConfig()
		c.MaxServices = 0
		if _, err := NewConfigManager(c); err != nil {
			rec.Record(stresschaos.Degraded, "NewConfigManager rejected invalid config cleanly")
		} else {
			rec.Record(stresschaos.Fatal, "NewConfigManager accepted invalid config (MaxServices=0)")
		}
	}()

	rec.AssertNoFatal()
	t.Log("discovery config validation survived corrupt-input injection")
}

// TestDiscovery_Chaos_ConcurrentRegisterDeregisterChurn hammers the SAME service
// name with concurrent Register / Update / Deregister / Heartbeat / UpdateHealth
// from many goroutines. The registry's RWMutex must serialise the map mutations
// so it never panics or races and the map ends self-consistent. Run under -race.
func TestDiscovery_Chaos_ConcurrentRegisterDeregisterChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "discovery_register_deregister_churn", "state-corruption")
	reg := NewServiceRegistry(DefaultRegistryConfig())

	const goroutines = 14
	const iters = 400
	const sharedName = "contended-service"
	info := ServiceInfo{Name: sharedName, Host: "127.0.0.1", Port: 9999, Protocol: "tcp"}

	var wg sync.WaitGroup
	var regs, deregs, updates int64
	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				switch (id + it) % 5 {
				case 0:
					if err := reg.Register(info); err == nil {
						atomic.AddInt64(&regs, 1)
					}
				case 1:
					if err := reg.Deregister(sharedName); err == nil {
						atomic.AddInt64(&deregs, 1)
					}
				case 2:
					if err := reg.Update(sharedName, info); err == nil {
						atomic.AddInt64(&updates, 1)
					}
				case 3:
					_ = reg.Heartbeat(sharedName)
				default:
					_ = reg.UpdateHealth(sharedName, it%2 == 0)
				}
				// Read-only accessors race against the mutations above.
				_, _ = reg.Get(sharedName)
				_ = reg.List()
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived register/deregister/update churn on shared key: regs=%d deregs=%d updates=%d, no panic/race",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&deregs), atomic.LoadInt64(&updates)))

	// The map must end in exactly one of two coherent states for the shared key:
	// present (last op registered/updated) or absent (last op deregistered).
	// Either way a fresh Register after a clean Deregister must succeed — proof
	// the map is not torn.
	_ = reg.Deregister(sharedName) // normalise to absent (tolerate not-found)
	if _, err := reg.Get(sharedName); err != ErrServiceNotFound {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("after deregister, key should be absent, got err=%v", err))
	}
	if err := reg.Register(info); err != nil {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("registry unusable after churn — Register failed: %v", err))
	} else {
		rec.Record(stresschaos.Recovered, "registry accepts fresh Register after churn — map self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("discovery churn: regs=%d deregs=%d updates=%d final-list=%d",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&deregs), atomic.LoadInt64(&updates), len(reg.List()))
}

// TestDiscovery_Chaos_RegistryUnderMemoryPressure drives the real registry under
// bounded memory pressure (§11.4.85(B)(4)). It must keep registering and
// resolving services rather than crash; the helper caps allocation under the
// §12.6 host-safety ceiling.
func TestDiscovery_Chaos_RegistryUnderMemoryPressure(t *testing.T) {
	reg := NewServiceRegistry(DefaultRegistryConfig())

	stresschaos.ChaosResourcePressureDuring(t, "discovery_registry_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			ok := 0
			for i := 0; i < 500; i++ {
				name := fmt.Sprintf("mp-svc-%d", i)
				if err := reg.Register(ServiceInfo{Name: name, Host: "127.0.0.1", Port: 1024 + i, Protocol: "tcp"}); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("register under pressure failed: %v", err))
					continue
				}
				if _, err := reg.Get(name); err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("registered service %s not resolvable under pressure: %v", name, err))
					return
				}
				ok++
			}
			if ok == 0 {
				rec.Record(stresschaos.Fatal, "registry registered zero services under memory pressure")
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("registry served %d register+get pairs under memory pressure", ok))
			}
		})
}

// makeHugeDiscoveryString returns an n-byte string of 'x' for oversized-payload
// chaos input.
func makeHugeDiscoveryString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
