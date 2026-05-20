package cache

import (
	"context"
	"errors"
	"time"

	"dev.helix.code/internal/redis"
	goredis "github.com/redis/go-redis/v9"
)

// RedisBackend is the minimal Redis surface the L3 tier needs. Keeping
// the tier behind this interface (rather than a concrete *redis.Client)
// keeps internal/cache decoupled and project-not-aware (CONST-051(B)):
// any byte-oriented Redis-like store satisfies it, and unit tests can
// supply a fake without a running Redis. internal/redis.Client is
// adapted onto it by redisClientBackend below.
type RedisBackend interface {
	// Enabled reports whether the backend can serve requests. A
	// disabled backend makes the L3 tier report Available()==false,
	// so the multi-tier cache skips it gracefully (no Redis required).
	Enabled() bool
	// Get returns the raw value, ErrMiss when the key is absent, or a
	// real error on a backend failure.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value under key with the given TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete removes key.
	Delete(ctx context.Context, key string) error
}

// RedisTier is the L3 shared cache tier, backed by Redis. Multiple
// HelixCode processes/hosts share L3, so a warm entry survives even a
// full restart of every process. When Redis is disabled or
// unconfigured the tier reports Available()==false and is skipped.
type RedisTier struct {
	backend RedisBackend
	ttl     time.Duration
}

// RedisTierConfig configures a RedisTier.
type RedisTierConfig struct {
	// Backend is the Redis-like store. Nil => the tier is permanently
	// unavailable (still valid: the cache runs L1+L2 only).
	Backend RedisBackend
	// TTL applied to Set calls passing ttl==0. Zero => 1h.
	TTL time.Duration
}

// NewRedisTier builds an L3 Redis tier.
func NewRedisTier(cfg RedisTierConfig) *RedisTier {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisTier{backend: cfg.Backend, ttl: ttl}
}

// Name implements Tier.
func (t *RedisTier) Name() string { return "L3-redis" }

// Available implements Tier — false when there is no backend or Redis
// is disabled, so the multi-tier cache skips L3 without erroring.
func (t *RedisTier) Available() bool {
	return t.backend != nil && t.backend.Enabled()
}

// Get implements Tier.
func (t *RedisTier) Get(ctx context.Context, key string) ([]byte, error) {
	if !t.Available() {
		return nil, ErrMiss
	}
	return t.backend.Get(ctx, key)
}

// Set implements Tier.
func (t *RedisTier) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !t.Available() {
		return nil // no-op — graceful degradation
	}
	if ttl <= 0 {
		ttl = t.ttl
	}
	return t.backend.Set(ctx, key, value, ttl)
}

// Delete implements Tier.
func (t *RedisTier) Delete(ctx context.Context, key string) error {
	if !t.Available() {
		return nil
	}
	return t.backend.Delete(ctx, key)
}

// Close implements Tier. The Redis connection lifecycle is owned by
// whoever constructed the *redis.Client, not by this tier.
func (t *RedisTier) Close() error { return nil }

// --- adapter: internal/redis.Client -> RedisBackend ---

// redisClientBackend adapts the HelixCode *redis.Client onto the
// RedisBackend interface.
type redisClientBackend struct {
	client *redis.Client
}

// NewRedisBackend wraps a HelixCode *redis.Client as a RedisBackend so
// it can drive an L3 RedisTier. A nil client yields a backend that is
// permanently disabled (the L3 tier is then simply skipped).
func NewRedisBackend(client *redis.Client) RedisBackend {
	return &redisClientBackend{client: client}
}

func (b *redisClientBackend) Enabled() bool {
	return b.client != nil && b.client.IsEnabled()
}

func (b *redisClientBackend) Get(ctx context.Context, key string) ([]byte, error) {
	if !b.Enabled() {
		return nil, ErrMiss
	}
	val, err := b.client.Get(ctx, key)
	if err != nil {
		// go-redis returns redis.Nil for a missing key; the wrapped
		// client surfaces it as an error. Treat the empty-result case
		// as a miss so a cache miss never looks like a backend fault.
		if errors.Is(err, goredis.Nil) || val == "" {
			return nil, ErrMiss
		}
		return nil, err
	}
	return []byte(val), nil
}

func (b *redisClientBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !b.Enabled() {
		return nil
	}
	return b.client.Set(ctx, key, value, ttl)
}

func (b *redisClientBackend) Delete(ctx context.Context, key string) error {
	if !b.Enabled() {
		return nil
	}
	return b.client.Del(ctx, key)
}
