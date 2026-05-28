package notification

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the in-process notification machinery.
//
// The unit under stress is the REAL *NotificationEngine + *NotificationQueue —
// their RWMutex-guarded channels map / rules slice / templates map and the real
// RegisterChannel / AddRule / SendDirect / SendNotification / Enqueue / Dequeue
// dispatch path. No fakes of the engine itself: delivery targets an in-process
// recording channel (scChannel) we register, so every PASS proves real dispatch
// happened WITHOUT any flaky external network send (the engine's real channels
// — Slack/Telegram/Discord — talk to the network and are deliberately avoided in
// the load loops per the §11.4.85 no-flaky-network rule). Sustained dispatch
// (N>=100, p50/p95/p99 captured) + N>=10 concurrent producers driving the shared
// channels map under genuine read/write contention (run under -race).

// scChannel is a real in-process NotificationChannel that records every
// delivered notification. It is a genuine channel implementation (satisfies the
// NotificationChannel interface and is dispatched through the real engine path),
// not a mock of the engine under test — only the network egress is replaced by an
// in-memory sink so the stress loop is deterministic and network-free.
type scChannel struct {
	name      string
	enabled   bool
	delivered int64
}

func (c *scChannel) Send(ctx context.Context, n *Notification) error {
	if n == nil {
		return fmt.Errorf("nil notification")
	}
	atomic.AddInt64(&c.delivered, 1)
	// Touch the payload like a real consumer would (forces evaluation).
	_ = fmt.Sprintf("%s/%s", n.Title, n.Message)
	return nil
}
func (c *scChannel) GetName() string                    { return c.name }
func (c *scChannel) IsEnabled() bool                    { return c.enabled }
func (c *scChannel) GetConfig() map[string]interface{}  { return map[string]interface{}{"in_process": true} }
func (c *scChannel) count() int64                       { return atomic.LoadInt64(&c.delivered) }

// TestNotification_Stress_SustainedSendDirect drives the real engine's
// SendDirect dispatch under sustained load (N>=100), recording per-call latency.
// Each iteration dispatches a real notification to a registered in-process
// channel and asserts the delivery counter advanced by exactly one, so the run
// proves real dispatch work — not a no-op.
func TestNotification_Stress_SustainedSendDirect(t *testing.T) {
	engine := NewNotificationEngine()
	ch := &scChannel{name: "sink", enabled: true}
	if err := engine.RegisterChannel(ch); err != nil {
		t.Fatalf("register sink: %v", err)
	}
	ctx := context.Background()

	template := &Notification{
		Title:    "stress",
		Message:  "sustained send-direct",
		Type:     NotificationTypeInfo,
		Priority: NotificationPriorityLow,
	}

	var sent int64
	stresschaos.RunSustainedLoad(t, "notification_sustained_send_direct",
		stresschaos.SustainedConfig{N: 2000, MaxErrorRate: 0.0},
		func(i int) error {
			before := ch.count()
			if err := engine.SendDirect(ctx, template, []string{"sink"}); err != nil {
				return fmt.Errorf("send-direct: %w", err)
			}
			if delta := ch.count() - before; delta != 1 {
				return fmt.Errorf("send-direct delivered %d, want 1", delta)
			}
			atomic.AddInt64(&sent, 1)
			return nil
		})

	if atomic.LoadInt64(&sent) == 0 {
		t.Fatal("engine dispatched zero notifications under sustained load — not real work")
	}
	if got, want := ch.count(), atomic.LoadInt64(&sent); got != want {
		t.Fatalf("total delivery count %d != sent %d", got, want)
	}
	t.Logf("notification sustained send-direct: %d sent, %d delivered", atomic.LoadInt64(&sent), ch.count())
}

// TestNotification_Stress_SustainedSendNotificationWithRules drives the real
// rule-application + template path (SendNotification) under sustained load. A
// rule routes matching notifications to the sink channel and a loaded template
// rewrites the message, so the dispatch exercises applyRules + applyTemplate +
// sendToChannels — all under the engine's RWMutex.
func TestNotification_Stress_SustainedSendNotificationWithRules(t *testing.T) {
	engine := NewNotificationEngine()
	ch := &scChannel{name: "sink", enabled: true}
	if err := engine.RegisterChannel(ch); err != nil {
		t.Fatalf("register sink: %v", err)
	}
	if err := engine.LoadTemplate("tmpl", "[{{.Type}}] {{.Title}}"); err != nil {
		t.Fatalf("load template: %v", err)
	}
	if err := engine.AddRule(NotificationRule{
		Name:     "route-info",
		Condition: "type==info",
		Channels: []string{"sink"},
		Priority: NotificationPriorityMedium,
		Enabled:  true,
		Template: "tmpl",
	}); err != nil {
		t.Fatalf("add rule: %v", err)
	}
	ctx := context.Background()

	template := &Notification{
		Title:    "ruled",
		Message:  "sustained with-rules",
		Type:     NotificationTypeInfo,
		Priority: NotificationPriorityLow,
	}

	var sent int64
	stresschaos.RunSustainedLoad(t, "notification_sustained_with_rules",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			before := ch.count()
			if err := engine.SendNotification(ctx, template); err != nil {
				return fmt.Errorf("send-notification: %w", err)
			}
			if delta := ch.count() - before; delta != 1 {
				return fmt.Errorf("with-rules delivered %d, want 1 (rule routing broken)", delta)
			}
			atomic.AddInt64(&sent, 1)
			return nil
		})

	if atomic.LoadInt64(&sent) == 0 {
		t.Fatal("engine dispatched zero rule-routed notifications under sustained load")
	}
	t.Logf("notification sustained with-rules: %d sent, %d delivered", atomic.LoadInt64(&sent), ch.count())
}

// TestNotification_Stress_ConcurrentRegisterSendRule hammers the shared engine
// state from N>=10 goroutines that interleave RegisterChannel + AddRule +
// SendDirect + GetChannelStats, asserting no deadlock, no goroutine leak, and no
// data race (run under -race) on the RWMutex-guarded maps/slices. Each goroutine
// registers its own channel then dispatches to it, so real read/write contention
// is generated against the channels map.
func TestNotification_Stress_ConcurrentRegisterSendRule(t *testing.T) {
	engine := NewNotificationEngine()
	ctx := context.Background()

	// A shared sink so different goroutines also contend on the SAME map key on
	// dispatch (read contention) while registering distinct keys (write).
	shared := &scChannel{name: "shared", enabled: true}
	if err := engine.RegisterChannel(shared); err != nil {
		t.Fatalf("register shared: %v", err)
	}

	var dispatches int64
	stresschaos.RunConcurrent(t, "notification_concurrent_register_send_rule",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			// Register a goroutine-and-iteration-unique channel under write-lock.
			own := &scChannel{name: fmt.Sprintf("ch-%d-%d", g, it), enabled: true}
			// Duplicate names across iterations are expected to be rejected — that
			// is a legitimate engine response, not an error for this contention test.
			_ = engine.RegisterChannel(own)
			// Add a rule under write-lock (contends with the registrations above).
			_ = engine.AddRule(NotificationRule{
				Name:     fmt.Sprintf("rule-%d-%d", g, it),
				Channels: []string{"shared"},
				Enabled:  true,
			})
			// Dispatch to the shared sink under read-lock (contends with writers).
			n := &Notification{Title: fmt.Sprintf("g%d", g), Message: "concurrent", Type: NotificationTypeInfo}
			if err := engine.SendDirect(ctx, n, []string{"shared"}); err != nil {
				return fmt.Errorf("send-direct: %w", err)
			}
			atomic.AddInt64(&dispatches, 1)
			// Read-only accessor widens the RLock surface.
			_ = engine.GetChannelStats()
			return nil
		})

	if atomic.LoadInt64(&dispatches) == 0 {
		t.Fatal("engine dispatched zero notifications under concurrent load")
	}
	if shared.count() == 0 {
		t.Fatal("shared sink received nothing after concurrent dispatch — map mutations lost")
	}
	t.Logf("notification concurrent: %d dispatches, %d delivered to shared sink",
		atomic.LoadInt64(&dispatches), shared.count())
}

// TestNotification_Stress_ConcurrentQueue hammers the REAL *NotificationQueue
// from N>=10 goroutines that concurrently Enqueue while real worker goroutines
// Dequeue + dispatch, asserting the mutex-guarded queue slice survives the
// contention with no race and the stats reconcile.
func TestNotification_Stress_ConcurrentQueue(t *testing.T) {
	engine := NewNotificationEngine()
	ch := &scChannel{name: "sink", enabled: true}
	if err := engine.RegisterChannel(ch); err != nil {
		t.Fatalf("register sink: %v", err)
	}
	// 8 workers each process one item per 100ms tick (the queue's documented
	// design), so steady-state throughput is ~80 items/sec. The enqueue count
	// below (12 goroutines x 25 = 300) is sized to drain comfortably within the
	// deadline while still generating heavy concurrent mutex contention on the
	// shared queue slice during the enqueue burst.
	q := NewNotificationQueue(engine, 8, 0) // 0 = unbounded
	q.Start()
	defer q.Stop()

	var enqueued int64
	stresschaos.RunConcurrent(t, "notification_concurrent_queue",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 25, Timeout: 25 * time.Second},
		func(g, it int) error {
			n := &Notification{Title: fmt.Sprintf("q-%d-%d", g, it), Message: "queued", Type: NotificationTypeInfo}
			if err := q.Enqueue(n, []string{"sink"}, 1); err != nil {
				return fmt.Errorf("enqueue: %w", err)
			}
			atomic.AddInt64(&enqueued, 1)
			_ = q.Size()
			return nil
		})

	if atomic.LoadInt64(&enqueued) == 0 {
		t.Fatal("queue accepted zero enqueues under concurrent load")
	}

	// Drain: workers tick every 100ms; wait until the queue empties (bounded).
	deadline := time.Now().Add(20 * time.Second)
	for q.Size() > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if sz := q.Size(); sz != 0 {
		t.Fatalf("queue did not drain: %d items remain after 15s — workers stalled", sz)
	}
	stats := q.GetStats()
	if stats.Enqueued != atomic.LoadInt64(&enqueued) {
		t.Fatalf("stats.Enqueued=%d != enqueued=%d", stats.Enqueued, atomic.LoadInt64(&enqueued))
	}
	if ch.count() == 0 {
		t.Fatal("no notifications delivered through the queue — workers never dispatched")
	}
	t.Logf("notification concurrent queue: %d enqueued, dequeued=%d succeeded=%d delivered=%d",
		atomic.LoadInt64(&enqueued), stats.Dequeued, stats.Succeeded, ch.count())
}

// TestNotification_Stress_BoundaryConditions exercises the §11.4.85(A)(3)
// boundary cases against the real engine + queue: (empty) dispatch to NO
// channels and to an unknown channel must be a clean no-op nil; (max) a single
// notification fanned to many registered channels must reach every one; an empty
// payload and a huge payload must both dispatch without crash; (off-by-one) a
// bounded queue at capacity must reject the next enqueue cleanly.
func TestNotification_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	t.Run("no_channels", func(t *testing.T) {
		engine := NewNotificationEngine()
		// Dispatch with no Channels at all — clean no-op.
		if err := engine.SendDirect(ctx, &Notification{Title: "x"}, nil); err != nil {
			t.Fatalf("send to zero channels must be clean no-op, got: %v", err)
		}
		// Dispatch to an unknown channel name — skipped with a warning, no error.
		if err := engine.SendDirect(ctx, &Notification{Title: "x"}, []string{"does-not-exist"}); err != nil {
			t.Fatalf("send to unknown channel must be clean no-op, got: %v", err)
		}
	})

	t.Run("many_channels", func(t *testing.T) {
		engine := NewNotificationEngine()
		const many = 200
		chs := make([]*scChannel, many)
		names := make([]string, many)
		for i := 0; i < many; i++ {
			chs[i] = &scChannel{name: fmt.Sprintf("ch-%d", i), enabled: true}
			names[i] = chs[i].name
			if err := engine.RegisterChannel(chs[i]); err != nil {
				t.Fatalf("register ch-%d: %v", i, err)
			}
		}
		if err := engine.SendDirect(ctx, &Notification{Title: "fan", Message: "out"}, names); err != nil {
			t.Fatalf("fan-out send: %v", err)
		}
		for i := 0; i < many; i++ {
			if chs[i].count() != 1 {
				t.Fatalf("channel %d delivered %d, want 1", i, chs[i].count())
			}
		}
	})

	t.Run("empty_and_huge_payload", func(t *testing.T) {
		engine := NewNotificationEngine()
		ch := &scChannel{name: "sink", enabled: true}
		if err := engine.RegisterChannel(ch); err != nil {
			t.Fatalf("register: %v", err)
		}
		// Empty payload.
		if err := engine.SendDirect(ctx, &Notification{}, []string{"sink"}); err != nil {
			t.Fatalf("empty payload send: %v", err)
		}
		// Huge payload (1 MiB title + message).
		huge := makeHugeNotifString(1 << 20)
		if err := engine.SendDirect(ctx, &Notification{Title: huge, Message: huge}, []string{"sink"}); err != nil {
			t.Fatalf("huge payload send: %v", err)
		}
		if ch.count() != 2 {
			t.Fatalf("delivered %d, want 2 (empty + huge)", ch.count())
		}
	})

	t.Run("bounded_queue_rejects_overflow", func(t *testing.T) {
		engine := NewNotificationEngine()
		// Do NOT start workers — so the queue fills and stays full.
		q := NewNotificationQueue(engine, 1, 2) // maxSize=2
		n := &Notification{Title: "x"}
		if err := q.Enqueue(n, nil, 0); err != nil {
			t.Fatalf("first enqueue: %v", err)
		}
		if err := q.Enqueue(n, nil, 0); err != nil {
			t.Fatalf("second enqueue: %v", err)
		}
		// Third must be rejected cleanly with an error — not a panic, not silent.
		if err := q.Enqueue(n, nil, 0); err == nil {
			t.Fatal("enqueue beyond maxSize must return an error, got nil (overflow not bounded)")
		}
		if q.Size() != 2 {
			t.Fatalf("queue size %d after overflow attempt, want 2", q.Size())
		}
	})
}

// makeHugeNotifString returns an n-byte string of 'x' for oversized-payload tests.
func makeHugeNotifString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// Ensure the recording channel really satisfies the production interface.
var _ NotificationChannel = (*scChannel)(nil)
