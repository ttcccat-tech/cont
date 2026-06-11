package storage

import (
	"context"
	"fmt"
	"strings"

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

// Get all target health statuses for an upstream
func (r *Redis) GetTargetHealthStatuses(ctx context.Context, upstream string) (map[string]bool, error) {
	keyPattern := fmt.Sprintf("cont:health:%s:*", upstream)
	keys, err := r.client.Keys(ctx, keyPattern).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for _, k := range keys {
		// key format: cont:health:{upstream}:{target}
		target := strings.TrimPrefix(k, fmt.Sprintf("cont:health:%s:", upstream))
		result[target] = true // key exists = unhealthy
	}
	return result, nil
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
