package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the in-process notification machinery.
//
// Chaos classes exercised against the REAL *NotificationEngine / *NotificationQueue
// (no fakes of the engine — real registration, real dispatch, real mutex-guarded
// state):
//
//   - channel-panic injection: a registered NotificationChannel whose Send panics
//     mid-dispatch MUST NOT take down the engine. sendToChannels calls Send inside
//     the engine's RLock; an unrecovered panic crashes the whole process (and, when
//     dispatched from a queue worker goroutine, takes the worker down silently).
//     The engine MUST isolate a panicking channel so co-channels still receive the
//     notification and the engine stays usable. (This is the exact bug class found
//     in sibling concurrency-rich packages.)
//   - input-corruption: structurally hostile notification payloads (NaN/Inf
//     metadata, unmarshalable channel/func values, oversized strings, nested
//     cycles) are dispatched. Dispatch + channel-side handling MUST not crash.
//   - state-corruption under contention: the engine is concurrently
//     RegisterChannel'd / AddRule'd / SendDirect'd from many goroutines mid-flight.
//     The engine MUST never panic or race and MUST end self-consistent.
//   - resource pressure: dispatch under bounded memory pressure must not OOM-crash.

// scPanicChannel is a real channel whose Send panics — used to prove the engine
// isolates a misbehaving channel. count records how many times Send was entered.
type scPanicChannel struct {
	name    string
	entered int64
}

func (c *scPanicChannel) Send(ctx context.Context, n *Notification) error {
	atomic.AddInt64(&c.entered, 1)
	panic(fmt.Sprintf("chaos: channel %s Send panic", c.name))
}
func (c *scPanicChannel) GetName() string                   { return c.name }
func (c *scPanicChannel) IsEnabled() bool                   { return true }
func (c *scPanicChannel) GetConfig() map[string]interface{} { return map[string]interface{}{"panic": true} }

var _ NotificationChannel = (*scPanicChannel)(nil)

// TestNotification_Chaos_ChannelPanicIsolation registers a channel whose Send
// panics alongside well-behaved co-channels, then dispatches. A panicking channel
// MUST NOT crash the engine or starve its co-channels, and the engine MUST remain
// usable for subsequent sends. An unrecovered panic — which in synchronous
// dispatch propagates to the caller and in a queue worker kills the goroutine —
// is a §11.4.85(B) Fatal.
func TestNotification_Chaos_ChannelPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "notification_channel_panic_isolation", "process-death")
	engine := NewNotificationEngine()
	ctx := context.Background()

	before := &scChannel{name: "before", enabled: true}
	panicCh := &scPanicChannel{name: "panic"}
	after := &scChannel{name: "after", enabled: true}
	for _, ch := range []NotificationChannel{before, panicCh, after} {
		if err := engine.RegisterChannel(ch); err != nil {
			t.Fatalf("register %s: %v", ch.GetName(), err)
		}
	}

	// Drive the send on a guarded goroutine: if the engine does NOT isolate the
	// panic, it propagates here (caught + recorded Fatal). Channel order in the
	// dispatch loop puts "after" past the panicking channel, so a propagated panic
	// would starve it.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal,
					fmt.Sprintf("dispatch propagated channel panic to caller: %v", p))
			}
		}()
		err := engine.SendDirect(ctx, &Notification{Title: "chaos", Message: "panic-isolation"},
			[]string{"before", "panic", "after"})
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("dispatch surfaced channel error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "dispatch completed despite panicking channel")
		}
	}()

	// Co-channels both before AND after the panicking one must have run — the
	// panic must NOT have starved them.
	if before.count() == 0 || after.count() == 0 {
		rec.Record(stresschaos.Fatal,
			fmt.Sprintf("panicking channel starved co-channels (before=%d after=%d) — not isolated",
				before.count(), after.count()))
	} else {
		rec.Record(stresschaos.Recovered,
			fmt.Sprintf("co-channels survived panic (before=%d after=%d panic-entered=%d)",
				before.count(), after.count(), atomic.LoadInt64(&panicCh.entered)))
	}

	// The engine must remain usable for a fresh send after the panic.
	followUp := &scChannel{name: "followup", enabled: true}
	if err := engine.RegisterChannel(followUp); err != nil {
		t.Fatalf("register followup: %v", err)
	}
	if err := engine.SendDirect(ctx, &Notification{Title: "after-panic"}, []string{"followup"}); err != nil {
		rec.Record(stresschaos.Degraded, fmt.Sprintf("follow-up send errored: %v", err))
	}
	if followUp.count() == 0 {
		rec.Record(stresschaos.Fatal, "engine unusable after panic — follow-up send delivered nothing")
	} else {
		rec.Record(stresschaos.Recovered, "engine still usable after panic")
	}

	rec.AssertNoFatal()
	t.Log("notification engine survived channel-panic injection")
}

// TestNotification_Chaos_QueuePanicIsolation drives the panicking channel through
// a REAL queue worker goroutine. An unrecovered panic in Send executes on the
// worker goroutine (q.processNext -> SendDirect -> sendToChannels -> Send); an
// unisolated panic crashes the whole `go test` process. The queue MUST keep
// delivering co-channel notifications and stay alive.
func TestNotification_Chaos_QueuePanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "notification_queue_panic_isolation", "process-death")
	engine := NewNotificationEngine()

	good := &scChannel{name: "good", enabled: true}
	panicCh := &scPanicChannel{name: "panic"}
	for _, ch := range []NotificationChannel{good, panicCh} {
		if err := engine.RegisterChannel(ch); err != nil {
			t.Fatalf("register %s: %v", ch.GetName(), err)
		}
	}

	q := NewNotificationQueue(engine, 2, 0)
	q.Start()
	defer q.Stop()

	// Enqueue items that route through the panicking channel.
	const items = 8
	for i := 0; i < items; i++ {
		if err := q.Enqueue(&Notification{Title: fmt.Sprintf("p-%d", i)}, []string{"good", "panic"}, 1); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	// Wait for the workers to chew through the queue. If a worker goroutine dies
	// from an unisolated panic, the queue stops draining and this times out.
	deadline := time.Now().Add(10 * time.Second)
	for q.Size() > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if q.Size() != 0 {
		rec.Record(stresschaos.Fatal,
			fmt.Sprintf("queue did not drain (%d left) — a worker goroutine was killed by an unisolated channel panic", q.Size()))
	} else {
		rec.Record(stresschaos.Recovered, "queue fully drained — workers survived channel panics")
	}

	// The good co-channel must still have received deliveries.
	if good.count() == 0 {
		rec.Record(stresschaos.Fatal, "good co-channel received nothing — workers died before dispatching it")
	} else {
		rec.Record(stresschaos.Recovered,
			fmt.Sprintf("queue delivered %d to good co-channel despite %d panics", good.count(), atomic.LoadInt64(&panicCh.entered)))
	}

	rec.AssertNoFatal()
	t.Logf("notification queue survived channel-panic injection: good=%d panic-entered=%d",
		good.count(), atomic.LoadInt64(&panicCh.entered))
}

// TestNotification_Chaos_CorruptPayload dispatches structurally hostile
// notification payloads to the REAL engine. Dispatch and channel-side handling
// (which ranges over Metadata + formats values, mirroring real channels like
// Telegram) must not panic on NaN/Inf floats, unmarshalable channel/func values,
// oversized strings, or nested cycles — a crash on malformed input is a
// §11.4.85(B) failure.
func TestNotification_Chaos_CorruptPayload(t *testing.T) {
	engine := NewNotificationEngine()

	// A real consumer-style channel that touches Title/Message/Metadata exactly
	// like the production Telegram channel does (it ranges Metadata into the body).
	consumer := &scConsumerChannel{name: "consumer"}
	if err := engine.RegisterChannel(consumer); err != nil {
		t.Fatalf("register consumer: %v", err)
	}
	ctx := context.Background()

	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"channel": "unmarshalable-marker-chan"},
		{"func": "unmarshalable-marker-func"},
		{"huge_key": makeHugeNotifString(1 << 16)},
		{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}},
	}
	payloads := make([][]byte, len(corruptKinds))
	for i, k := range corruptKinds {
		b, err := json.Marshal(k)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "notification_corrupt_payload", payloads,
		func(input []byte) error {
			idx := corruptNotifIndexOf(input)
			data := hostileNotifDataFor(idx)
			// Dispatching must not panic regardless of payload. A channel error is
			// graceful; a panic is fatal (caught by the helper).
			return engine.SendDirect(ctx, &Notification{
				Title:    "corrupt",
				Message:  makeHugeNotifString(1024),
				Metadata: data,
			}, []string{"consumer"})
		})
}

// TestNotification_Chaos_ConcurrentRegisterSendChurn hammers the engine with
// concurrent RegisterChannel / AddRule / SendDirect / GetChannelStats from many
// goroutines. The real engine mutex must serialise map/slice mutations so the
// engine never panics or races and ends self-consistent. Run under -race.
func TestNotification_Chaos_ConcurrentRegisterSendChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "notification_register_send_churn", "state-corruption")
	engine := NewNotificationEngine()
	ctx := context.Background()

	shared := &scChannel{name: "shared", enabled: true}
	if err := engine.RegisterChannel(shared); err != nil {
		t.Fatalf("register shared: %v", err)
	}

	const goroutines = 12
	const iters = 300
	var wg sync.WaitGroup
	var regs, rules, sends int64

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
				switch (id + it) % 3 {
				case 0:
					ch := &scChannel{name: fmt.Sprintf("g%d-i%d", id, it), enabled: true}
					if engine.RegisterChannel(ch) == nil {
						atomic.AddInt64(&regs, 1)
					}
				case 1:
					if engine.AddRule(NotificationRule{
						Name:    fmt.Sprintf("rule-%d-%d", id, it),
						Enabled: true,
					}) == nil {
						atomic.AddInt64(&rules, 1)
					}
				default:
					// SendDirect reads the (concurrently mutating) channels map.
					_ = engine.SendDirect(ctx, &Notification{Title: fmt.Sprintf("g%d", id), Type: NotificationTypeInfo}, []string{"shared"})
					atomic.AddInt64(&sends, 1)
				}
				// Read-only accessor widens the RLock contention surface.
				_ = engine.GetChannelStats()
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived register/rule/send churn: %d regs, %d rules, %d sends, no panic/race",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&rules), atomic.LoadInt64(&sends)))

	// Final state must be self-consistent and the engine must still dispatch.
	var finalHit int64
	finalCh := &scCallbackChannel{name: "final-probe", onSend: func() { atomic.AddInt64(&finalHit, 1) }}
	if err := engine.RegisterChannel(finalCh); err != nil {
		rec.Record(stresschaos.Fatal, "could not register final probe after churn: "+err.Error())
	}
	if err := engine.SendDirect(ctx, &Notification{Title: "final"}, []string{"final-probe"}); err != nil {
		rec.Record(stresschaos.Degraded, "final send surfaced channel error: "+err.Error())
	}
	if atomic.LoadInt64(&finalHit) == 0 {
		rec.Record(stresschaos.Fatal, "engine did not dispatch to a fresh channel after churn — map corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "engine dispatches correctly after churn — state self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("notification churn: regs=%d rules=%d sends=%d", atomic.LoadInt64(&regs), atomic.LoadInt64(&rules), atomic.LoadInt64(&sends))
}

// TestNotification_Chaos_ResourcePressure dispatches through the real engine
// under bounded memory pressure (§11.4.85(B)(4)). The engine must keep
// delivering, never OOM-crash.
func TestNotification_Chaos_ResourcePressure(t *testing.T) {
	engine := NewNotificationEngine()
	ch := &scChannel{name: "sink", enabled: true}
	if err := engine.RegisterChannel(ch); err != nil {
		t.Fatalf("register sink: %v", err)
	}
	ctx := context.Background()

	stresschaos.ChaosResourcePressureDuring(t, "notification_resource_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 500; i++ {
				if err := engine.SendDirect(ctx, &Notification{
					Title:   fmt.Sprintf("pressure-%d", i),
					Message: makeHugeNotifString(4096),
				}, []string{"sink"}); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("send %d errored under pressure: %v", i, err))
					return
				}
			}
			if ch.count() < 500 {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("delivered %d/500 under pressure", ch.count()))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("delivered all %d under pressure", ch.count()))
			}
		})
}

// scConsumerChannel is a real channel that touches Title/Message/Metadata the way
// a production channel (e.g. Telegram) does — forcing evaluation of hostile values.
type scConsumerChannel struct {
	name string
	seen int64
}

func (c *scConsumerChannel) Send(ctx context.Context, n *Notification) error {
	body := fmt.Sprintf("<b>%s</b>\n\n%s", n.Title, n.Message)
	for k, v := range n.Metadata {
		body += fmt.Sprintf("\n• %s: %v", k, v)
	}
	_ = body
	atomic.AddInt64(&c.seen, 1)
	return nil
}
func (c *scConsumerChannel) GetName() string                   { return c.name }
func (c *scConsumerChannel) IsEnabled() bool                   { return true }
func (c *scConsumerChannel) GetConfig() map[string]interface{} { return map[string]interface{}{} }

var _ NotificationChannel = (*scConsumerChannel)(nil)

// scCallbackChannel is a real channel that fires a callback on each Send — used
// when a test needs to observe delivery via an external counter.
type scCallbackChannel struct {
	name   string
	onSend func()
}

func (c *scCallbackChannel) Send(ctx context.Context, n *Notification) error {
	if c.onSend != nil {
		c.onSend()
	}
	return nil
}
func (c *scCallbackChannel) GetName() string                   { return c.name }
func (c *scCallbackChannel) IsEnabled() bool                   { return true }
func (c *scCallbackChannel) GetConfig() map[string]interface{} { return map[string]interface{}{} }

var _ NotificationChannel = (*scCallbackChannel)(nil)

// corruptNotifIndexOf recovers the chaos payload index from the descriptor.
func corruptNotifIndexOf(input []byte) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(input, &m); err != nil {
		return 0
	}
	if _, ok := m["corrupt_index"]; ok {
		var probe struct {
			CorruptIndex int `json:"corrupt_index"`
		}
		if json.Unmarshal(input, &probe) == nil {
			return probe.CorruptIndex
		}
	}
	switch {
	case hasNotifKey(m, "channel"):
		return 2
	case hasNotifKey(m, "func"):
		return 3
	case hasNotifKey(m, "huge_key"):
		return 4
	case hasNotifKey(m, "nested"):
		return 5
	case hasNotifKey(m, "nan"):
		return 0
	case hasNotifKey(m, "inf"):
		return 1
	}
	return 0
}

func hasNotifKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// hostileNotifDataFor reconstructs the actual hostile Metadata map for a chaos
// index, including types (chan, func) that cannot survive []byte serialisation
// but exercise dispatch + any formatting paths.
func hostileNotifDataFor(idx int) map[string]interface{} {
	switch idx {
	case 0:
		return map[string]interface{}{"nan": math.NaN()}
	case 1:
		return map[string]interface{}{"inf": math.Inf(1)}
	case 2:
		return map[string]interface{}{"channel": make(chan int)}
	case 3:
		return map[string]interface{}{"func": func() {}}
	case 4:
		return map[string]interface{}{"huge_key": makeHugeNotifString(1 << 16)}
	default:
		return map[string]interface{}{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}}
	}
}
