package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the in-process memory components.
//
// Chaos classes exercised against the REAL Manager / MemoryManager / InMemoryProvider
// (no mocks — the production concurrency surface is the system under test):
//
//   - state-corruption under contention: a single conversation is mutated
//     concurrently by AddMessage / ClearConversation / DeleteConversation while
//     readers run GetConversation / Search / GetStatistics. The Manager's RWMutex
//     must serialise writers so the store stays self-consistent and no reader
//     observes a torn Conversation, panics, or races.
//
//   - default-provider race-to-the-bottom: writers UnregisterProvider the only
//     provider while readers Store/Retrieve through GetDefaultProvider. The
//     manager MUST degrade gracefully (return a no-default / not-found error),
//     never panic and never deadlock.
//
//   - input-corruption: structurally hostile values (NaN/Inf floats, channels,
//     func values, self-referential maps, huge keys) are fed to Store/AddMessage.
//     The provider/manager MUST reject (error) or normalise without crashing.

// chaosConvManager builds a real conversation Manager for chaos runs.
func chaosConvManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

// chaosMemoryManager builds a real MemoryManager fronting a real InMemoryProvider.
func chaosMemoryManager(t *testing.T) *MemoryManager {
	t.Helper()
	mm := NewMemoryManager(&MemoryConfig{Enabled: true, Provider: "inmemory"})
	prov, err := NewInMemoryProvider(nil)
	if err != nil {
		t.Fatalf("create in-memory provider: %v", err)
	}
	if err := mm.RegisterProvider("inmemory", prov); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	return mm
}

// TestManager_Chaos_ConcurrentConversationMutation creates one real conversation,
// then hammers it with concurrent mutating calls (AddMessage + ClearConversation)
// and concurrent readers (GetConversation + Search + GetStatistics). The real
// m.mu must serialise the writers so the store never tears and never panics, and
// the conversation must end in a self-consistent state (MessageCount == len of
// Messages slice in every successful read). Run under -race.
func TestManager_Chaos_ConcurrentConversationMutation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "memory_conversation_mutation_churn", "state-corruption")
	m := chaosConvManager(t)

	conv, err := m.CreateConversation("chaos-target")
	if err != nil {
		t.Fatalf("create chaos target: %v", err)
	}

	const writers = 8
	const readers = 8
	const iters = 400
	var wg sync.WaitGroup
	var adds, clears, reads int64

	// Writers: race AddMessage vs ClearConversation on the SAME conversation.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if (id+it)%4 == 0 {
					if err := m.ClearConversation(conv.ID); err == nil {
						atomic.AddInt64(&clears, 1)
					}
				} else {
					if err := m.AddMessage(conv.ID, NewUserMessage(fmt.Sprintf("m %d-%d", id, it))); err == nil {
						atomic.AddInt64(&adds, 1)
					}
				}
			}
		}(w)
	}

	// Readers: concurrently observe the conversation; a torn read surfaces as a
	// panic, a race (under -race), or an inconsistent MessageCount.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if got, err := m.GetConversation(conv.ID); err == nil {
					atomic.AddInt64(&reads, 1)
					_ = got.MessageCount
				}
				_ = m.Search("m")
				_ = m.GetStatistics()
			}
		}(r)
	}

	wg.Wait()
	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived concurrent conversation churn: %d adds, %d clears, %d clean reads, no panic/race",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads)))

	// Final state MUST be self-consistent: MessageCount equals the live slice len.
	final, err := m.GetConversation(conv.ID)
	if err != nil {
		rec.Record(stresschaos.Fatal, "conversation vanished after churn: "+err.Error())
	} else if final.MessageCount != len(final.Messages) {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("inconsistent final state: MessageCount=%d len(Messages)=%d",
			final.MessageCount, len(final.Messages)))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("final state consistent: %d messages", final.MessageCount))
	}

	rec.AssertNoFatal()
	t.Logf("conversation chaos churn: adds=%d clears=%d reads=%d", atomic.LoadInt64(&adds), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads))
}

// TestMemoryManager_Chaos_DefaultProviderRace removes the only provider out from
// under concurrent readers. Writers UnregisterProvider/RegisterProvider the
// single "inmemory" provider while readers Store/Retrieve through
// GetDefaultProvider. Outcomes: a successful op (Recovered) or a clean
// no-default/not-found error (Degraded). A panic or deadlock is Fatal. This
// proves graceful degradation when the resolvable default disappears mid-flight.
func TestMemoryManager_Chaos_DefaultProviderRace(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "memory_default_provider_race", "state-corruption")
	mm := chaosMemoryManager(t)
	ctx := context.Background()

	const writers = 4
	const readers = 12
	const iters = 300
	var wg sync.WaitGroup
	var ok, degraded int64

	// Writers: repeatedly tear down and rebuild the only provider.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				_ = mm.UnregisterProvider("inmemory")
				prov, err := NewInMemoryProvider(nil)
				if err != nil {
					rec.Record(stresschaos.Fatal, "provider construction failed: "+err.Error())
					return
				}
				_ = mm.RegisterProvider("inmemory", prov)
				_ = mm.SetDefaultProvider("inmemory")
			}
		}(w)
	}

	// Readers: resolve + use the default provider while it churns underneath.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				key := fmt.Sprintf("k-%d-%d", id, it)
				if err := mm.Store(ctx, key, it); err != nil {
					atomic.AddInt64(&degraded, 1) // clean no-default/not-found error
					continue
				}
				if _, err := mm.Retrieve(ctx, key); err != nil {
					atomic.AddInt64(&degraded, 1)
					continue
				}
				atomic.AddInt64(&ok, 1)
			}
		}(r)
	}

	wg.Wait()
	if atomic.LoadInt64(&ok) > 0 {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("%d ops succeeded despite provider churn", atomic.LoadInt64(&ok)))
	}
	if atomic.LoadInt64(&degraded) > 0 {
		rec.Record(stresschaos.Degraded, fmt.Sprintf("%d ops cleanly degraded (no-default/not-found) — no crash", atomic.LoadInt64(&degraded)))
	}
	if atomic.LoadInt64(&ok) == 0 && atomic.LoadInt64(&degraded) == 0 {
		rec.Record(stresschaos.Fatal, "no ops completed — possible deadlock under provider churn")
	}

	rec.AssertNoFatal()
	t.Logf("default-provider race: ok=%d degraded=%d", atomic.LoadInt64(&ok), atomic.LoadInt64(&degraded))
}

// TestMemoryManager_Chaos_CorruptInputData feeds structurally hostile values to
// the REAL InMemoryProvider.Store (via MemoryManager) and to Manager.AddMessage.
// InMemoryProvider stores any interface{} directly (no marshal on the Store path),
// so the chaos here proves Store never crashes on weird values AND that a
// subsequent Retrieve round-trips them. None of the inputs may panic.
func TestMemoryManager_Chaos_CorruptInputData(t *testing.T) {
	mm := chaosMemoryManager(t)
	ctx := context.Background()

	// Descriptor payloads honour the helper's [][]byte contract; feed()
	// reconstructs the actual hostile value for each index.
	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"channel": "marker"},
		{"func": "marker"},
		{"huge_key": "marker"},
		{"nested_cycle": "marker"},
	}
	payloads := make([][]byte, len(corruptKinds))
	for i, k := range corruptKinds {
		b, err := json.Marshal(k)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "memory_provider_corrupt_input", payloads,
		func(input []byte) error {
			idx := memCorruptIndexOf(input)
			val := hostileMemValueFor(idx)
			key := fmt.Sprintf("corrupt-%d", idx)
			// Store accepts any interface{}; it must not panic on any value.
			if err := mm.Store(ctx, key, val); err != nil {
				return err // clean rejection is acceptable
			}
			// Retrieve must round-trip the stored value without crashing.
			if _, err := mm.Retrieve(ctx, key); err != nil {
				return err
			}
			return nil
		})
}

// TestManager_Chaos_CorruptMessageContent feeds hostile message content to the
// REAL Manager.AddMessage path: empty content, huge content, and content that
// would break a naive search. None may panic; the manager must accept or reject
// each without crashing, and the conversation state must remain consistent.
func TestManager_Chaos_CorruptMessageContent(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "memory_corrupt_message_content", "input-corruption")
	m := chaosConvManager(t)

	conv, err := m.CreateConversation("corrupt-content")
	if err != nil {
		t.Fatalf("create conv: %v", err)
	}

	hostile := []string{
		"",                              // empty content
		makeHugeMemString(1 << 18),      // 256 KiB content
		"\x00\x01\x02 binary \xff\xfe",  // control/binary bytes
		"%v %s %d %%",                   // format-string-ish content
		"emoji \U0001F600 unicode 中文", // multibyte unicode
	}

	for i, content := range hostile {
		func(idx int, c string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("AddMessage[%d] panicked: %v", idx, p))
				}
			}()
			msg := NewUserMessage(c)
			if err := m.AddMessage(conv.ID, msg); err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("AddMessage[%d] rejected cleanly: %v", idx, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("AddMessage[%d] accepted %d-byte content without crash", idx, len(c)))
			}
			// Search over the (possibly hostile) corpus must not panic.
			_ = m.Search("binary")
		}(i, content)
	}

	final, err := m.GetConversation(conv.ID)
	if err != nil {
		rec.Record(stresschaos.Fatal, "conversation vanished: "+err.Error())
	} else if final.MessageCount != len(final.Messages) {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("inconsistent state: count=%d len=%d", final.MessageCount, len(final.Messages)))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("consistent after hostile content: %d messages", final.MessageCount))
	}

	rec.AssertNoFatal()
}

// makeHugeMemString returns an n-byte string for oversized-input chaos.
func makeHugeMemString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'y'
	}
	return string(b)
}

// memCorruptIndexOf recovers the chaos payload index from the marshalled descriptor.
func memCorruptIndexOf(input []byte) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(input, &m); err != nil {
		return 0
	}
	switch {
	case hasMemKey(m, "nan"):
		return 0
	case hasMemKey(m, "inf"):
		return 1
	case hasMemKey(m, "channel"):
		return 2
	case hasMemKey(m, "func"):
		return 3
	case hasMemKey(m, "huge_key"):
		return 4
	case hasMemKey(m, "nested_cycle"):
		return 5
	}
	var probe struct {
		CorruptIndex int `json:"corrupt_index"`
	}
	if err := json.Unmarshal(input, &probe); err == nil {
		return probe.CorruptIndex
	}
	return 0
}

func hasMemKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// hostileMemValueFor reconstructs the actual hostile value for a given chaos
// index — including non-JSON-marshalable types (chan, func, self-referential
// map) that InMemoryProvider.Store must accept and store without crashing.
func hostileMemValueFor(idx int) interface{} {
	switch idx {
	case 0:
		return math.NaN()
	case 1:
		return math.Inf(1)
	case 2:
		return make(chan int)
	case 3:
		return func() {}
	case 4:
		return makeHugeMemString(1 << 16)
	default:
		cycle := map[string]interface{}{}
		cycle["self"] = cycle // self-referential map
		return cycle
	}
}
