// Package cache provides a 3-tier read-through cache (P4-T02, speed
// programme Phase 4) for context-build and embedding hot paths.
//
// §11.4.74 Catalogue-Check (no-match, 2026-05-20): the owned submodule
// dependencies/vasic-digital/Cache provides a generic cache.Cache
// interface and a 2-tier distributed.TwoLevel (L1 memory + L2 remote),
// but NO disk tier — so it does not supply the memory→disk→Redis 3-tier
// this task requires (<80% fit). Rather than add a cross-module go.mod
// dependency (which would risk the inner module build mid-Phase-4), this
// package is built decoupled and project-not-aware (CONST-051(B)) so it
// can itself become a catalogue entry. It mirrors the submodule's
// read-through/promote-upward design.
//
// Tier model:
//
//	L1  in-memory   — fastest, process-local, bounded entry count
//	L2  disk        — survives process restarts, persisted under cacheDir
//	L3  Redis       — shared across processes/hosts (optional)
//
// A read checks L1→L2→L3; a hit at a lower tier populates every upper
// tier (populate-upward). A write or invalidation clears the entry in
// ALL tiers (coherence — a stale entry must never be served). A tier
// being unavailable (e.g. no Redis configured) degrades gracefully to
// the available tiers and never errors the operation: the cache is an
// optimization, not a hard dependency.
package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrMiss is returned by a Tier.Get when the key is absent. It is a
// sentinel, not a failure — callers translate it into a cache miss and
// fall through to the next tier.
var ErrMiss = errors.New("cache: miss")

// Tier is one storage layer of the multi-tier cache. Implementations
// MUST be safe for concurrent use. A Tier that is unavailable (a Redis
// backend with Redis disabled, say) should report Available()==false so
// the MultiTier skips it instead of erroring.
type Tier interface {
	// Name is a short human-readable tier label (for stats/benchmarks).
	Name() string

	// Available reports whether this tier can currently serve requests.
	// An unavailable tier is skipped entirely — never errors the op.
	Available() bool

	// Get returns the value for key, or ErrMiss if absent. Any other
	// error is a real backend failure.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores value under key with the given TTL (0 = tier default).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes key. Deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error

	// Close releases any resources held by the tier.
	Close() error
}

// Stats is a snapshot of per-tier and aggregate cache counters.
type Stats struct {
	Hits       map[string]uint64 // tier name -> hit count
	Misses     uint64            // requests that missed every tier
	Sets       uint64            // Set calls
	Deletes    uint64            // Delete calls
	TierErrors map[string]uint64 // tier name -> non-fatal backend errors
}

// MultiTier is a read-through cache layering tiers fastest-first. The
// slice order IS the lookup order: index 0 is consulted first.
type MultiTier struct {
	tiers      []Tier
	defaultTTL time.Duration

	mu         sync.RWMutex
	hits       map[string]*uint64
	tierErrors map[string]*uint64
	misses     uint64
	sets       uint64
	deletes    uint64
}

// Config configures a MultiTier cache.
type Config struct {
	// Tiers ordered fastest-first (L1, L2, L3, …). Empty or nil tiers
	// are dropped; a MultiTier with zero usable tiers is a valid
	// pass-through (every Get misses, every Set is a no-op) so the
	// feature degrades gracefully when the cache is config-disabled.
	Tiers []Tier
	// DefaultTTL applied to Set calls passing ttl==0.
	DefaultTTL time.Duration
}

// New builds a MultiTier from cfg. Nil entries in cfg.Tiers are removed.
func New(cfg Config) *MultiTier {
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	m := &MultiTier{
		defaultTTL: ttl,
		hits:       make(map[string]*uint64),
		tierErrors: make(map[string]*uint64),
	}
	for _, t := range cfg.Tiers {
		if t == nil {
			continue
		}
		m.tiers = append(m.tiers, t)
		var h, e uint64
		m.hits[t.Name()] = &h
		m.tierErrors[t.Name()] = &e
	}
	return m
}

// Get performs a read-through lookup. It checks each tier in order;
// the first hit is returned AND propagated to every preceding (faster)
// tier so subsequent reads resolve higher up. A tier reporting
// Available()==false is skipped. A non-fatal tier error is counted and
// skipped — it never aborts the lookup, because a cache miss must
// always be a safe fallback. Returns (value, true) on hit, (nil,
// false) on a miss across every tier.
func (m *MultiTier) Get(ctx context.Context, key string) ([]byte, bool) {
	for i, t := range m.tiers {
		if !t.Available() {
			continue
		}
		val, err := t.Get(ctx, key)
		if err != nil {
			if errors.Is(err, ErrMiss) {
				continue
			}
			m.incr(m.tierErrors[t.Name()])
			continue
		}
		if val == nil {
			continue
		}
		m.incr(m.hits[t.Name()])
		// Populate-upward: write the hit into every faster tier so the
		// next read resolves there. Failures here are non-fatal — the
		// value is still returned to the caller.
		m.populateUpward(ctx, i, key, val)
		return val, true
	}
	m.mu.Lock()
	m.misses++
	m.mu.Unlock()
	return nil, false
}

// populateUpward writes val into every tier with an index below
// hitIdx (the faster tiers that missed). Best-effort: errors ignored.
func (m *MultiTier) populateUpward(ctx context.Context, hitIdx int, key string, val []byte) {
	for j := 0; j < hitIdx; j++ {
		t := m.tiers[j]
		if !t.Available() {
			continue
		}
		_ = t.Set(ctx, key, val, m.defaultTTL)
	}
}

// Set writes value to EVERY tier. This is intentional, not just an
// optimisation: a write that updated only some tiers would leave a
// stale value in the others, which a later Get could serve — exactly
// the coherence bug this cache must not have. ttl==0 uses DefaultTTL.
// Unavailable tiers are skipped; an available tier's error is counted
// and the write continues to the rest, returning the first such error
// so the caller can observe a partial failure without the cache
// breaking the feature.
func (m *MultiTier) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = m.defaultTTL
	}
	m.mu.Lock()
	m.sets++
	m.mu.Unlock()

	var firstErr error
	for _, t := range m.tiers {
		if !t.Available() {
			continue
		}
		if err := t.Set(ctx, key, value, ttl); err != nil {
			m.incr(m.tierErrors[t.Name()])
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Delete (invalidation) removes key from EVERY tier — the coherence
// core of this cache. After Delete returns, no tier may still hold the
// entry; a subsequent Get must miss (or re-fetch a fresh value). Every
// tier is attempted even if an earlier one errors, so one flaky tier
// can never leave a stale entry behind in another. The first error is
// returned for observability.
func (m *MultiTier) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	m.deletes++
	m.mu.Unlock()

	var firstErr error
	for _, t := range m.tiers {
		if !t.Available() {
			continue
		}
		if err := t.Delete(ctx, key); err != nil {
			m.incr(m.tierErrors[t.Name()])
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Invalidate is an alias for Delete, named for the read-side intent.
func (m *MultiTier) Invalidate(ctx context.Context, key string) error {
	return m.Delete(ctx, key)
}

// Stats returns a snapshot of the cache counters.
func (m *MultiTier) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := Stats{
		Hits:       make(map[string]uint64, len(m.hits)),
		Misses:     m.misses,
		Sets:       m.sets,
		Deletes:    m.deletes,
		TierErrors: make(map[string]uint64, len(m.tierErrors)),
	}
	for name, p := range m.hits {
		s.Hits[name] = atomic.LoadUint64(p)
	}
	for name, p := range m.tierErrors {
		s.TierErrors[name] = atomic.LoadUint64(p)
	}
	return s
}

// TierNames returns the configured tier names, fastest-first.
func (m *MultiTier) TierNames() []string {
	names := make([]string, len(m.tiers))
	for i, t := range m.tiers {
		names[i] = t.Name()
	}
	return names
}

// Close closes every tier, returning the first error encountered.
func (m *MultiTier) Close() error {
	var firstErr error
	for _, t := range m.tiers {
		if err := t.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiTier) incr(p *uint64) {
	if p != nil {
		atomic.AddUint64(p, 1)
	}
}
