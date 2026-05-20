package cache

import (
	"context"
	"container/list"
	"sync"
	"time"
)

// MemoryTier is the L1 in-memory cache tier: the fastest layer, bounded
// by a maximum entry count with LRU eviction. It is always available.
type MemoryTier struct {
	mu       sync.Mutex
	maxItems int
	ttl      time.Duration
	entries  map[string]*list.Element
	lru      *list.List // front = most recently used
}

// memEntry is one in-memory cache record.
type memEntry struct {
	key     string
	value   []byte
	expires time.Time // zero = no expiry
}

// MemoryTierConfig configures a MemoryTier.
type MemoryTierConfig struct {
	// MaxItems caps the entry count; the least-recently-used entry is
	// evicted when the cap is exceeded. Zero/negative => 1024.
	MaxItems int
	// TTL is the per-entry lifetime; an expired entry reports as a
	// miss and is purged lazily on access. Zero => no expiry.
	TTL time.Duration
}

// NewMemoryTier builds an L1 in-memory tier.
func NewMemoryTier(cfg MemoryTierConfig) *MemoryTier {
	max := cfg.MaxItems
	if max <= 0 {
		max = 1024
	}
	return &MemoryTier{
		maxItems: max,
		ttl:      cfg.TTL,
		entries:  make(map[string]*list.Element, max),
		lru:      list.New(),
	}
}

// Name implements Tier.
func (t *MemoryTier) Name() string { return "L1-memory" }

// Available implements Tier — the in-memory tier is always available.
func (t *MemoryTier) Available() bool { return true }

// Get implements Tier. An expired entry is purged and reported missing.
func (t *MemoryTier) Get(_ context.Context, key string) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.entries[key]
	if !ok {
		return nil, ErrMiss
	}
	ent := el.Value.(*memEntry)
	if !ent.expires.IsZero() && time.Now().After(ent.expires) {
		t.removeElement(el)
		return nil, ErrMiss
	}
	t.lru.MoveToFront(el)
	// Return a copy so callers cannot mutate the cached buffer.
	out := make([]byte, len(ent.value))
	copy(out, ent.value)
	return out, nil
}

// Set implements Tier, evicting the LRU entry when over capacity.
func (t *MemoryTier) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = t.ttl
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	buf := make([]byte, len(value))
	copy(buf, value)

	t.mu.Lock()
	defer t.mu.Unlock()

	if el, ok := t.entries[key]; ok {
		ent := el.Value.(*memEntry)
		ent.value = buf
		ent.expires = exp
		t.lru.MoveToFront(el)
		return nil
	}
	el := t.lru.PushFront(&memEntry{key: key, value: buf, expires: exp})
	t.entries[key] = el
	for t.lru.Len() > t.maxItems {
		if back := t.lru.Back(); back != nil {
			t.removeElement(back)
		}
	}
	return nil
}

// Delete implements Tier.
func (t *MemoryTier) Delete(_ context.Context, key string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if el, ok := t.entries[key]; ok {
		t.removeElement(el)
	}
	return nil
}

// Close implements Tier.
func (t *MemoryTier) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = make(map[string]*list.Element)
	t.lru.Init()
	return nil
}

// Len returns the current entry count (test/metrics helper).
func (t *MemoryTier) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lru.Len()
}

// removeElement drops an LRU element from both the list and the map.
// Caller must hold t.mu.
func (t *MemoryTier) removeElement(el *list.Element) {
	t.lru.Remove(el)
	delete(t.entries, el.Value.(*memEntry).key)
}
