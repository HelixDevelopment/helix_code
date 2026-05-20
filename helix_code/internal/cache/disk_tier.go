package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiskTier is the L2 disk-backed cache tier. Entries survive process
// restarts. Each entry is one file under cacheDir, named by the SHA-256
// of the key (so arbitrary key strings map to safe filenames). The file
// layout is a small fixed header (8-byte expiry-unix-nanos, 0 = none)
// followed by the raw value bytes — no external codec, no dependency.
//
// The tier degrades gracefully: if cacheDir cannot be created the tier
// reports Available()==false and is skipped, rather than erroring every
// cache operation.
type DiskTier struct {
	cacheDir  string
	ttl       time.Duration
	available bool

	mu sync.RWMutex // serialises file writes per process
}

// diskHeaderLen is the fixed prefix: 8 bytes of expiry (unix nanos).
const diskHeaderLen = 8

// DiskTierConfig configures a DiskTier.
type DiskTierConfig struct {
	// Dir is the directory entries are persisted under. It is created
	// (0o700) if absent.
	Dir string
	// TTL is the per-entry lifetime applied when Set is called with
	// ttl==0. Zero => no expiry.
	TTL time.Duration
}

// NewDiskTier builds an L2 disk tier. A failure to create Dir does not
// error — it yields an unavailable tier so the multi-tier cache simply
// skips L2 and the feature keeps working on L1 (+L3).
func NewDiskTier(cfg DiskTierConfig) *DiskTier {
	t := &DiskTier{cacheDir: cfg.Dir, ttl: cfg.TTL}
	if cfg.Dir == "" {
		return t // unavailable
	}
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return t // unavailable — graceful degradation
	}
	t.available = true
	return t
}

// Name implements Tier.
func (t *DiskTier) Name() string { return "L2-disk" }

// Available implements Tier.
func (t *DiskTier) Available() bool { return t.available }

// path maps a cache key to its on-disk file path.
func (t *DiskTier) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(t.cacheDir, hex.EncodeToString(sum[:]))
}

// Get implements Tier. An expired or unreadable entry reports ErrMiss
// (and an expired entry is purged) so a stale file is never served.
func (t *DiskTier) Get(_ context.Context, key string) ([]byte, error) {
	if !t.available {
		return nil, ErrMiss
	}
	p := t.path(key)

	t.mu.RLock()
	data, err := os.ReadFile(p) // #nosec G304 -- path is a hash under cacheDir
	t.mu.RUnlock()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrMiss
		}
		return nil, err
	}
	if len(data) < diskHeaderLen {
		// Corrupt/truncated file — treat as miss and purge.
		_ = t.Delete(context.Background(), key)
		return nil, ErrMiss
	}
	expNanos := int64(binary.BigEndian.Uint64(data[:diskHeaderLen]))
	if expNanos != 0 && time.Now().UnixNano() > expNanos {
		_ = t.Delete(context.Background(), key)
		return nil, ErrMiss
	}
	value := make([]byte, len(data)-diskHeaderLen)
	copy(value, data[diskHeaderLen:])
	return value, nil
}

// Set implements Tier, writing atomically (temp file + rename) so a
// reader never sees a half-written entry.
func (t *DiskTier) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if !t.available {
		return nil // no-op when L2 unavailable — graceful degradation
	}
	if ttl <= 0 {
		ttl = t.ttl
	}
	var expNanos int64
	if ttl > 0 {
		expNanos = time.Now().Add(ttl).UnixNano()
	}
	buf := make([]byte, diskHeaderLen+len(value))
	binary.BigEndian.PutUint64(buf[:diskHeaderLen], uint64(expNanos))
	copy(buf[diskHeaderLen:], value)

	p := t.path(key)
	tmp := p + ".tmp"

	t.mu.Lock()
	defer t.mu.Unlock()
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Delete implements Tier. Removing an absent entry is not an error.
func (t *DiskTier) Delete(_ context.Context, key string) error {
	if !t.available {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	err := os.Remove(t.path(key))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Close implements Tier. The on-disk data is intentionally retained
// across Close so a subsequent process run can warm-start from L2.
func (t *DiskTier) Close() error { return nil }
