package rules

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss 表示缓存中没有可用数据。
var ErrCacheMiss = errors.New("cache miss")

// Cache 抽象规则缓存的基本操作。
type Cache interface {
	Get(ctx context.Context) ([]Rule, error)
	Set(ctx context.Context, rules []Rule) error
	Invalidate(ctx context.Context) error
}

// RedisCache 使用 Redis 作为规则缓存。
type RedisCache struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

// NewRedisCache 创建 Redis 缓存适配器。
func NewRedisCache(client *redis.Client, key string, ttl time.Duration) *RedisCache {
	return &RedisCache{
		client: client,
		key:    key,
		ttl:    ttl,
	}
}

func (c *RedisCache) Get(ctx context.Context) ([]Rule, error) {
	raw, err := c.client.Get(ctx, c.key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	var rules []Rule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *RedisCache) Set(ctx context.Context, rules []Rule) error {
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	if c.ttl <= 0 {
		return c.client.Set(ctx, c.key, raw, 0).Err()
	}
	return c.client.Set(ctx, c.key, raw, c.ttl).Err()
}

func (c *RedisCache) Invalidate(ctx context.Context) error {
	return c.client.Del(ctx, c.key).Err()
}
