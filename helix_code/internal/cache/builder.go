package cache

import (
	"path/filepath"
	"time"

	"dev.helix.code/internal/redis"
)

// MultiTierConfig is the config-gated description of a 3-tier cache,
// suitable for binding from a YAML config section. The cache is an
// optimization: when Enabled is false, Build returns a zero-tier
// MultiTier that misses every read and no-ops every write, so a caller
// behaves identically to having no cache at all.
type MultiTierConfig struct {
	// Enabled gates the whole cache (P4-T02 is config-gated per the
	// task no-regression constraint). False => pass-through cache.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// L1MaxItems caps the in-memory tier entry count. 0 => 1024.
	L1MaxItems int `yaml:"l1_max_items" json:"l1_max_items"`
	// L1TTL is the in-memory entry lifetime. 0 => no L1 expiry.
	L1TTL time.Duration `yaml:"l1_ttl" json:"l1_ttl"`

	// DiskEnabled gates the L2 disk tier.
	DiskEnabled bool `yaml:"disk_enabled" json:"disk_enabled"`
	// DiskDir is the L2 persistence directory. Empty => L2 disabled.
	DiskDir string `yaml:"disk_dir" json:"disk_dir"`
	// DiskTTL is the L2 entry lifetime. 0 => no L2 expiry.
	DiskTTL time.Duration `yaml:"disk_ttl" json:"disk_ttl"`

	// RedisEnabled gates the L3 Redis tier (also requires a usable
	// *redis.Client passed to Build).
	RedisEnabled bool `yaml:"redis_enabled" json:"redis_enabled"`
	// RedisTTL is the L3 entry lifetime. 0 => 1h.
	RedisTTL time.Duration `yaml:"redis_ttl" json:"redis_ttl"`

	// DefaultTTL applied to Set calls passing ttl==0. 0 => 30m.
	DefaultTTL time.Duration `yaml:"default_ttl" json:"default_ttl"`
}

// Build constructs a MultiTier from cfg. The redisClient is optional —
// pass nil (or a disabled client) and the L3 tier is simply absent;
// the cache still works on L1+L2. namespace is appended to DiskDir so
// distinct cache users (repo-map, embeddings) keep separate L2
// directories under one root. When cfg.Enabled is false the result is
// a zero-tier pass-through cache, guaranteeing no behavioural change
// versus running without the cache at all.
func (cfg MultiTierConfig) Build(redisClient *redis.Client, namespace string) *MultiTier {
	if !cfg.Enabled {
		return New(Config{DefaultTTL: cfg.DefaultTTL})
	}

	var tiers []Tier

	// L1 — always present when the cache is enabled.
	tiers = append(tiers, NewMemoryTier(MemoryTierConfig{
		MaxItems: cfg.L1MaxItems,
		TTL:      cfg.L1TTL,
	}))

	// L2 — disk, present only when explicitly enabled with a directory.
	if cfg.DiskEnabled && cfg.DiskDir != "" {
		dir := cfg.DiskDir
		if namespace != "" {
			dir = filepath.Join(cfg.DiskDir, namespace)
		}
		tiers = append(tiers, NewDiskTier(DiskTierConfig{
			Dir: dir,
			TTL: cfg.DiskTTL,
		}))
	}

	// L3 — Redis, present only when enabled AND a client is supplied.
	// A nil/disabled client makes the tier report Available()==false,
	// so it costs nothing and degrades gracefully.
	if cfg.RedisEnabled {
		tiers = append(tiers, NewRedisTier(RedisTierConfig{
			Backend: NewRedisBackend(redisClient),
			TTL:     cfg.RedisTTL,
		}))
	}

	return New(Config{Tiers: tiers, DefaultTTL: cfg.DefaultTTL})
}
