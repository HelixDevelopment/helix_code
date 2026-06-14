package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	localstore "digital.vasic.helixmemory/pkg/localstore"
	modstore "digital.vasic.memory/pkg/store"
)

// HelixMemoryProvider is a durable, zero-config memory.MemoryProvider backed by
// the HelixMemory submodule's embedded SQLite local store
// (digital.vasic.helixmemory/pkg/localstore). It is the reuse-correct
// integration per §11.4.74 — HelixCode depends on the published HelixMemory SDK
// rather than reimplementing a store.
//
// "Zero-config" means: it persists OUT-OF-THE-BOX with no API keys and no
// external service. The default DB path lives under the OS user-config dir
// (<user-config>/helix_memory/memory.db) and is overridable via the
// HELIX_MEMORY_DB environment variable — both resolved inside the HelixMemory
// submodule so no project-specific path leaks across the CONST-051(B) boundary.
type HelixMemoryProvider struct {
	store *localstore.SQLiteStore
}

// Compile-time guarantee the provider satisfies the MemoryProvider contract.
var _ MemoryProvider = (*HelixMemoryProvider)(nil)

// NewHelixMemoryProvider opens (or creates) a durable provider at the given
// path. An empty path resolves to the HelixMemory zero-config default
// (honouring HELIX_MEMORY_DB).
func NewHelixMemoryProvider(dbPath string) (*HelixMemoryProvider, error) {
	st, err := localstore.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("helixmemory provider: open store: %w", err)
	}
	return &HelixMemoryProvider{store: st}, nil
}

// NewHelixMemoryProviderDefault opens the provider at the zero-config default
// path. This is what the TUI uses out-of-the-box.
func NewHelixMemoryProviderDefault() (*HelixMemoryProvider, error) {
	return NewHelixMemoryProvider("")
}

// DBPath reports the on-disk database path backing this provider.
func (p *HelixMemoryProvider) DBPath() string { return p.store.Path() }

// Store persists data under a logical key. The key is recorded in metadata so
// Retrieve can find the exact entry; the value is stringified for content
// search. Conversation turns are stored as plain content so Search recalls them
// by substring/word-overlap.
func (p *HelixMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	content := stringifyValue(data)
	mem := &modstore.Memory{
		ID:      keyToID(key),
		Content: content,
		Scope:   modstore.ScopeUser,
		Metadata: map[string]any{
			"key":        key,
			"stored_at":  time.Now().UTC().Format(time.RFC3339Nano),
			"value_type": fmt.Sprintf("%T", data),
		},
	}
	if err := p.store.Add(ctx, mem); err != nil {
		return fmt.Errorf("helixmemory provider: store key %q: %w", key, err)
	}
	return nil
}

// Retrieve returns the content previously stored under key (or an error if
// absent).
func (p *HelixMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	mem, err := p.store.Get(ctx, keyToID(key))
	if err != nil {
		return nil, fmt.Errorf("helixmemory provider: retrieve key %q: %w", key, err)
	}
	return mem.Content, nil
}

// Search performs a content search over the durable store and returns matches
// in the MemorySearchResult shape the rest of HelixCode expects.
func (p *HelixMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	mems, err := p.store.Search(ctx, query, &modstore.SearchOptions{TopK: limit})
	if err != nil {
		return nil, fmt.Errorf("helixmemory provider: search %q: %w", query, err)
	}
	out := make([]MemorySearchResult, 0, len(mems))
	for _, m := range mems {
		key := m.ID
		if k, ok := m.Metadata["key"].(string); ok && k != "" {
			key = k
		}
		out = append(out, MemorySearchResult{
			Key:   key,
			Data:  m.Content,
			Score: m.Score,
		})
	}
	return out, nil
}

// Delete removes the entry stored under key.
func (p *HelixMemoryProvider) Delete(ctx context.Context, key string) error {
	if err := p.store.Delete(ctx, keyToID(key)); err != nil {
		return fmt.Errorf("helixmemory provider: delete key %q: %w", key, err)
	}
	return nil
}

// Clear removes all entries from the durable store.
func (p *HelixMemoryProvider) Clear(ctx context.Context) error {
	mems, err := p.store.List(ctx, "", nil)
	if err != nil {
		return fmt.Errorf("helixmemory provider: clear (list): %w", err)
	}
	for _, m := range mems {
		if err := p.store.Delete(ctx, m.ID); err != nil {
			return fmt.Errorf("helixmemory provider: clear (delete %s): %w", m.ID, err)
		}
	}
	return nil
}

// Health verifies the durable store is reachable by issuing a real query. Unlike
// the pre-existing Redis/Memcached providers (which fail closed because no real
// client is wired), this provider always has a real on-disk backend, so Health
// reflects genuine connectivity.
func (p *HelixMemoryProvider) Health(ctx context.Context) error {
	if _, err := p.store.Count(ctx); err != nil {
		return fmt.Errorf("helixmemory provider: unhealthy: %w", err)
	}
	return nil
}

// Name returns the provider name.
func (p *HelixMemoryProvider) Name() string { return "helixmemory" }

// Type returns the provider type.
func (p *HelixMemoryProvider) Type() string { return "helixmemory-sqlite-local" }

// Close releases the underlying database (WAL-checkpointed).
func (p *HelixMemoryProvider) Close() error { return p.store.Close() }

// Count reports the number of durable memories (used by tests + recall wiring).
func (p *HelixMemoryProvider) Count(ctx context.Context) (int64, error) {
	return p.store.Count(ctx)
}

// stringifyValue renders an arbitrary stored value to searchable content.
func stringifyValue(data interface{}) string {
	switch v := data.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// keyToID derives a stable store ID from a logical key so the same key updates
// the same row (write-through semantics) rather than accumulating duplicates.
func keyToID(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "key:_empty_"
	}
	return "key:" + key
}
