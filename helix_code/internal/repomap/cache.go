package repomap

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	// Register types for gob encoding
	gob.Register([]Symbol{})
	gob.Register(Symbol{})
}

// writeOpKind enumerates the disk operations the single background writer
// performs. Replacing the per-call goroutine storm (R1 B07) with one drained
// channel: callers never spawn a goroutine, they just enqueue.
type writeOpKind int

const (
	writeOpSave    writeOpKind = iota // persist an entry to disk
	writeOpRemove                     // delete an entry's file from disk
	writeOpBarrier                    // no-op flush marker — closes done when reached
)

// writeOp is one unit of work for the background writer.
type writeOp struct {
	kind  writeOpKind
	key   string
	entry *cacheEntry   // populated only for writeOpSave
	done  chan struct{} // populated only for writeOpBarrier — closed when reached
}

// RepoCache provides disk-based, content-addressed caching for parsed symbols.
//
// Cache-key design (P2-T03): callers key entries by file path + file mtime
// (see RepoMap.getCacheKey — "<relpath>:<mtime-unix>"). An UNCHANGED file
// resolves to the SAME key on the next index → cache HIT, no re-parse. A
// CHANGED file's mtime advances → a DIFFERENT key → cache MISS → re-parse,
// and Set evicts the file's stale prior-mtime entry so it never lingers and
// can never be served. Persistence is disk-backed so a HIT survives across
// process runs.
//
// Disk persistence is performed by ONE background writer goroutine draining
// writeCh (R1 B07 — replaces the prior one-goroutine-per-Set storm). Callers
// MUST invoke Close (or Wait) before disposing the cache directory, otherwise
// queued writes may still be flushing into a doomed directory at process or
// test exit.
type RepoCache struct {
	cacheDir string
	ttl      time.Duration
	mu       sync.RWMutex
	entries  map[string]*cacheEntry

	// pathIndex maps a file's stable identity (everything before the final
	// ":<mtime>" segment of the cache key) to its currently-live cache key.
	// On Set it lets us evict the previous-mtime entry for the same file so a
	// stale entry can never accumulate or be served.
	pathIndex map[string]string

	// background writer
	writeCh   chan writeOp
	writerWG  sync.WaitGroup
	closeOnce sync.Once
	closed    chan struct{}

	// instrumentation counters (atomic — safe to read without the lock).
	// hits / misses make the "N-1 cache hits after a 1-file edit" anti-bluff
	// proof mechanically checkable.
	hits   atomic.Int64
	misses atomic.Int64
}

// cacheEntry represents a cached item with metadata.
type cacheEntry struct {
	Key       string
	Value     interface{}
	CachedAt  time.Time
	ExpiresAt time.Time
	// SizeBytes is the gob-encoded size measured ONCE at Set time (R1 B15 —
	// previously GetStats re-encoded every value just to measure it). 0 for
	// entries loaded from disk before this field existed; recomputed lazily.
	SizeBytes int64
}

// NewRepoCache creates a new cache instance and starts its background writer.
func NewRepoCache(cacheDir string, ttl time.Duration) (*RepoCache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &RepoCache{
		cacheDir:  cacheDir,
		ttl:       ttl,
		entries:   make(map[string]*cacheEntry),
		pathIndex: make(map[string]string),
		writeCh:   make(chan writeOp, 256),
		closed:    make(chan struct{}),
	}

	// Single background writer drains writeCh — replaces the per-Set goroutine
	// storm (R1 B07). Channel buffer absorbs bursts; Close drains it.
	cache.writerWG.Add(1)
	go cache.writerLoop()

	// Load existing cache entries (disk-backed persistence across runs).
	if err := cache.loadFromDisk(); err != nil {
		// Non-fatal error, just log and continue
		log.Printf("Warning: failed to load cache: %v", err)
	}

	return cache, nil
}

// writerLoop is the single background writer. It drains writeCh until closed,
// then flushes any remaining queued operations so Close loses no persistence.
func (rc *RepoCache) writerLoop() {
	defer rc.writerWG.Done()
	for {
		select {
		case op := <-rc.writeCh:
			rc.execWriteOp(op)
		case <-rc.closed:
			// Drain anything still queued, then exit.
			for {
				select {
				case op := <-rc.writeCh:
					rc.execWriteOp(op)
				default:
					return
				}
			}
		}
	}
}

// execWriteOp performs one disk operation on behalf of the background writer.
func (rc *RepoCache) execWriteOp(op writeOp) {
	switch op.kind {
	case writeOpSave:
		if err := rc.saveToDisk(op.key, op.entry); err != nil {
			log.Printf("Warning: failed to save cache entry: %v", err)
		}
	case writeOpRemove:
		_ = os.Remove(rc.getCacheFilename(op.key))
	case writeOpBarrier:
		if op.done != nil {
			close(op.done)
		}
	}
}

// enqueue submits a write operation to the background writer. If the cache is
// already closed it is performed synchronously so callers (e.g. Cleanup) never
// silently lose a delete.
func (rc *RepoCache) enqueue(op writeOp) {
	select {
	case <-rc.closed:
		rc.execWriteOp(op)
		return
	default:
	}
	select {
	case rc.writeCh <- op:
	case <-rc.closed:
		rc.execWriteOp(op)
	}
}

// pathIdentity returns the stable file-identity portion of a cache key —
// everything up to (but not including) the final ":<mtime>" segment produced
// by RepoMap.getCacheKey. For keys without that shape it returns the key
// unchanged, which simply disables stale-eviction for that key (still correct).
func pathIdentity(key string) string {
	if i := lastColon(key); i >= 0 {
		return key[:i]
	}
	return key
}

// lastColon returns the index of the last ':' in s, or -1.
func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// Get retrieves a value from the cache. A HIT (key present, not expired)
// increments the hit counter; any other outcome increments the miss counter —
// so a changed file (new key → key absent → MISS) is mechanically observable.
func (rc *RepoCache) Get(key string) (interface{}, bool) {
	rc.mu.RLock()
	entry, exists := rc.entries[key]
	rc.mu.RUnlock()

	if !exists {
		rc.misses.Add(1)
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		rc.misses.Add(1)
		return nil, false
	}

	rc.hits.Add(1)
	return entry.Value, true
}

// Set stores a value in the cache. If a prior entry exists for the SAME file
// under a different mtime key, that stale entry is evicted (memory + disk) so
// it can never be served and never accumulates — the cache-correctness
// guarantee for P2-T03 (a changed file's old data is gone).
func (rc *RepoCache) Set(key string, value interface{}) {
	now := time.Now()
	entry := &cacheEntry{
		Key:       key,
		Value:     value,
		CachedAt:  now,
		ExpiresAt: now.Add(rc.ttl),
		SizeBytes: estimateSize(value), // measured ONCE here (R1 B15)
	}

	rc.mu.Lock()
	ident := pathIdentity(key)
	var staleKey string
	if prev, ok := rc.pathIndex[ident]; ok && prev != key {
		// The same file now resolves to a new mtime key — drop the old one.
		staleKey = prev
		delete(rc.entries, prev)
	}
	rc.entries[key] = entry
	rc.pathIndex[ident] = key
	rc.mu.Unlock()

	// Persist via the single background writer (R1 B07 — no per-call goroutine).
	rc.enqueue(writeOp{kind: writeOpSave, key: key, entry: entry})
	if staleKey != "" {
		rc.enqueue(writeOp{kind: writeOpRemove, key: staleKey})
	}
}

// Invalidate removes a specific entry from the cache (memory + disk).
func (rc *RepoCache) Invalidate(key string) {
	rc.mu.Lock()
	delete(rc.entries, key)
	if ident := pathIdentity(key); rc.pathIndex[ident] == key {
		delete(rc.pathIndex, ident)
	}
	rc.mu.Unlock()

	rc.enqueue(writeOp{kind: writeOpRemove, key: key})
}

// InvalidateAll clears the entire cache (memory + disk).
func (rc *RepoCache) InvalidateAll() error {
	rc.mu.Lock()
	rc.entries = make(map[string]*cacheEntry)
	rc.pathIndex = make(map[string]string)
	rc.mu.Unlock()

	// Remove all cache files
	if err := os.RemoveAll(rc.cacheDir); err != nil {
		return err
	}
	// Recreate the directory so the cache stays usable after a full purge.
	return os.MkdirAll(rc.cacheDir, 0755)
}

// Wait blocks until the background writer has drained every operation queued
// at the moment Wait was called. A barrier op is enqueued FIFO behind the
// caller's prior writes; when the writer reaches it, every earlier op is done.
func (rc *RepoCache) Wait() {
	select {
	case <-rc.closed:
		// Writer has stopped; Close already drained the queue.
		return
	default:
	}
	done := make(chan struct{})
	rc.enqueue(writeOp{kind: writeOpBarrier, done: done})
	<-done
}

// Close drains all pending disk writes and stops the background writer.
// Callers MUST invoke Close before disposing the cache directory.
func (rc *RepoCache) Close() error {
	rc.closeOnce.Do(func() {
		close(rc.closed)
	})
	rc.writerWG.Wait()
	return nil
}

// Size returns the number of cached entries.
func (rc *RepoCache) Size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.entries)
}

// Stats returns the live hit / miss instrumentation counters. Used by the
// P2-T03 anti-bluff proof: after a 1-file edit and a re-index, an N-file repo
// must report N-1 hits and 1 miss.
func (rc *RepoCache) Stats() (hits, misses int64) {
	return rc.hits.Load(), rc.misses.Load()
}

// ResetStats zeroes the hit / miss counters (for scoped measurement).
func (rc *RepoCache) ResetStats() {
	rc.hits.Store(0)
	rc.misses.Store(0)
}

// Cleanup removes expired entries (memory + disk).
func (rc *RepoCache) Cleanup() int {
	rc.mu.Lock()
	now := time.Now()
	removed := 0
	var staleKeys []string

	for key, entry := range rc.entries {
		if now.After(entry.ExpiresAt) {
			delete(rc.entries, key)
			if ident := pathIdentity(key); rc.pathIndex[ident] == key {
				delete(rc.pathIndex, ident)
			}
			staleKeys = append(staleKeys, key)
			removed++
		}
	}
	rc.mu.Unlock()

	for _, k := range staleKeys {
		rc.enqueue(writeOp{kind: writeOpRemove, key: k})
	}
	return removed
}

// GetStats returns cache statistics. Size is summed from the per-entry
// SizeBytes measured at Set time (R1 B15 — no longer re-encodes every value).
func (rc *RepoCache) GetStats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	stats := CacheStats{
		TotalEntries:   len(rc.entries),
		ExpiredEntries: 0,
		TotalSize:      0,
	}

	now := time.Now()
	for _, entry := range rc.entries {
		if now.After(entry.ExpiresAt) {
			stats.ExpiredEntries++
		}
		size := entry.SizeBytes
		if size == 0 {
			// Entry loaded from a pre-SizeBytes disk file — measure once and
			// memoize so the next GetStats is free.
			size = estimateSize(entry.Value)
			entry.SizeBytes = size
		}
		stats.TotalSize += size
	}

	return stats
}

// CacheStats provides statistics about the cache.
type CacheStats struct {
	TotalEntries   int
	ExpiredEntries int
	TotalSize      int64 // Bytes
}

// saveToDisk persists a cache entry to disk.
func (rc *RepoCache) saveToDisk(key string, entry *cacheEntry) error {
	filename := rc.getCacheFilename(key)

	// Create directory if needed
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Encode entry
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode cache entry: %w", err)
	}

	// Write to disk atomically with retry for macOS file system quirks
	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Try rename first (most efficient)
	if err := os.Rename(tempFile, filename); err != nil {
		// Rename failed, try copy + remove approach (more robust on macOS)
		if err := rc.atomicCopy(tempFile, filename); err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("failed to save cache file: %w", err)
		}
		os.Remove(tempFile)
	}

	return nil
}

// atomicCopy performs an atomic copy operation for macOS compatibility.
func (rc *RepoCache) atomicCopy(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// loadFromDisk loads all cache entries from disk. A corrupt or partial cache
// file is treated as a MISS (the file is removed and skipped) — never a crash.
func (rc *RepoCache) loadFromDisk() error {
	// P2-T01: filepath.WalkDir — lazy fs.DirEntry, no per-entry stat.
	return filepath.WalkDir(rc.cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if d.IsDir() {
			return nil
		}

		// Skip temp files
		if filepath.Ext(path) == ".tmp" {
			return nil
		}

		// Load entry — a corrupt/partial file decodes with an error and is
		// removed, so a damaged cache degrades to a clean miss, never a panic.
		entry, err := rc.loadEntryFromFile(path)
		if err != nil {
			os.Remove(path)
			return nil
		}

		// Check if expired
		if time.Now().After(entry.ExpiresAt) {
			os.Remove(path)
			return nil
		}

		rc.entries[entry.Key] = entry
		rc.pathIndex[pathIdentity(entry.Key)] = entry.Key
		return nil
	})
}

// loadEntryFromFile loads a single cache entry from a file. A truncated or
// otherwise-corrupt file returns an error — never a panic — so the caller can
// treat it as a miss.
func (rc *RepoCache) loadEntryFromFile(filename string) (entry *cacheEntry, err error) {
	defer func() {
		// gob.Decode on a malformed/partial stream may panic on some inputs;
		// recover so a corrupt cache file is a miss, never a process crash.
		if r := recover(); r != nil {
			entry = nil
			err = fmt.Errorf("corrupt cache file %s: %v", filename, r)
		}
	}()

	data, readErr := os.ReadFile(filename)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", readErr)
	}

	var e cacheEntry
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if decErr := decoder.Decode(&e); decErr != nil {
		return nil, fmt.Errorf("failed to decode cache entry: %w", decErr)
	}

	return &e, nil
}

// getCacheFilename generates a filename for a cache key.
func (rc *RepoCache) getCacheFilename(key string) string {
	// Hash the key to create a safe filename
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	// Use first 2 chars for subdirectory (better distribution)
	subdir := hashStr[:2]

	// Use shorter filename - just first 16 chars of hash
	filename := hashStr[:16] + ".cache"

	return filepath.Join(rc.cacheDir, subdir, filename)
}

// estimateSize estimates the gob-encoded byte size of a value.
func estimateSize(value interface{}) int64 {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(value); err != nil {
		return 0
	}
	return int64(buf.Len())
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up
// expired entries. Send on / close the returned channel to stop it.
func (rc *RepoCache) StartCleanupRoutine(interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				removed := rc.Cleanup()
				if removed > 0 {
					log.Printf("Cache cleanup: removed %d expired entries", removed)
				}
			case <-stop:
				return
			case <-rc.closed:
				return
			}
		}
	}()

	return stop
}

// GetOrCompute retrieves a value from cache or computes it if not found.
func (rc *RepoCache) GetOrCompute(key string, compute func() (interface{}, error)) (interface{}, error) {
	if value, found := rc.Get(key); found {
		return value, nil
	}

	value, err := compute()
	if err != nil {
		return nil, err
	}

	rc.Set(key, value)
	return value, nil
}

// Has checks if a key exists in the cache (without affecting hit/miss stats).
func (rc *RepoCache) Has(key string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, exists := rc.entries[key]
	if !exists {
		return false
	}
	return !time.Now().After(entry.ExpiresAt)
}

// Keys returns all cache keys.
func (rc *RepoCache) Keys() []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	keys := make([]string, 0, len(rc.entries))
	for key := range rc.entries {
		keys = append(keys, key)
	}

	return keys
}

// Export exports cache entries to a writer (for backup).
func (rc *RepoCache) Export(writer *os.File) error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	encoder := gob.NewEncoder(writer)
	return encoder.Encode(rc.entries)
}

// Import imports cache entries from a reader (for restore).
func (rc *RepoCache) Import(reader *os.File) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	decoder := gob.NewDecoder(reader)
	entries := make(map[string]*cacheEntry)

	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode cache: %w", err)
	}

	// Filter out expired entries
	now := time.Now()
	for key, entry := range entries {
		if now.Before(entry.ExpiresAt) {
			rc.entries[key] = entry
			rc.pathIndex[pathIdentity(key)] = key
		}
	}

	return nil
}

// SetTTL updates the TTL for the cache.
func (rc *RepoCache) SetTTL(ttl time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ttl = ttl
}

// GetTTL returns the current TTL.
func (rc *RepoCache) GetTTL() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ttl
}
