package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(url string) *Redis {
	if url == "" {
		url = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr: url,
	})
	return &Redis{client: client}
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Rate limit: increment counter and get current count
func (r *Redis) IncrRateLimit(ctx context.Context, key string, windowSec int64) (int64, error) {
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 0) // refresh TTL
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// Upstream target health
func (r *Redis) SetTargetHealth(ctx context.Context, upstream, target string, healthy bool) error {
	key := fmt.Sprintf("cont:health:%s:%s", upstream, target)
	if healthy {
		return r.client.Del(ctx, key).Err()
	}
	return r.client.Set(ctx, key, "1", 0).Err()
}

// Plugin config cache (avoid hitting Postgres on every request)
func (r *Redis) GetPluginConfig(ctx context.Context, pluginID string) (string, error) {
	val, err := r.client.Get(ctx, "cont:plugin:"+pluginID).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *Redis) SetPluginConfig(ctx context.Context, pluginID string, config string) error {
	return r.client.Set(ctx, "cont:plugin:"+pluginID, config, 0).Err()
}
