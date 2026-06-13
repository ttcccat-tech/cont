package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// IncrementUsage increments the hourly API request counter for an org.
// Key format: cont:usage:{org_id}:{YYYYMMDDHH}
// TTL: 62 days (covers month boundary + buffer)
func (r *Redis) IncrementUsage(ctx context.Context, orgID string) (int64, error) {
	hour := time.Now().Format("2006010215")
	key := fmt.Sprintf("cont:usage:%s:%s", orgID, hour)
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 62*24*60*60) // 62 days TTL
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// IncrementUsageDetailed increments the hourly usage counter with full dimension info.
// Key format: cont:usage:{org_id}:{YYYYMMDDHH}
// Secondary key for consumer: cont:usage:consumer:{consumer_id}:{YYYYMMDDHH}
// Stores: org_id, consumer_id, route_id, service_id, timestamp, latency, status_code
func (r *Redis) IncrementUsageDetailed(ctx context.Context, orgID, consumerID, routeID, serviceID string, latencyMs int64, statusCode int) error {
	hour := time.Now().Format("2006010215")

	// Multi-exec to write multiple keys atomically
	pipe := r.client.Pipeline()

	// Main org counter
	orgKey := fmt.Sprintf("cont:usage:%s:%s", orgID, hour)
	pipe.Incr(ctx, orgKey)
	pipe.Expire(ctx, orgKey, 62*24*60*60)

	// Consumer counter (only if consumer_id is present)
	if consumerID != "" {
		consumerKey := fmt.Sprintf("cont:usage:consumer:%s:%s", consumerID, hour)
		pipe.Incr(ctx, consumerKey)
		pipe.Expire(ctx, consumerKey, 62*24*60*60)
	}

	// Route counter
	if routeID != "" {
		routeKey := fmt.Sprintf("cont:usage:route:%s:%s", routeID, hour)
		pipe.Incr(ctx, routeKey)
		pipe.Expire(ctx, routeKey, 62*24*60*60)
	}

	// Service counter
	if serviceID != "" {
		serviceKey := fmt.Sprintf("cont:usage:service:%s:%s", serviceID, hour)
		pipe.Incr(ctx, serviceKey)
		pipe.Expire(ctx, serviceKey, 62*24*60*60)
	}

	// Detail log sorted set: cont:usage:detail:{org_id}:{YYYYMMDDHH}
	// Score = timestamp_ns, member = {consumer_id}:{route_id}:{service_id}:{latencyMs}:{statusCode}
	detailKey := fmt.Sprintf("cont:usage:detail:%s:%s", orgID, hour)
	ts := time.Now().UnixNano()
	member := fmt.Sprintf("%s:%s:%s:%d:%d", consumerID, routeID, serviceID, latencyMs, statusCode)
	pipe.ZAdd(ctx, detailKey, redis.Z{Score: float64(ts), Member: member})
	pipe.Expire(ctx, detailKey, 62*24*60*60)

	_, err := pipe.Exec(ctx)
	return err
}

// GetUsage returns the current hourly API request count for an org.
func (r *Redis) GetUsage(ctx context.Context, orgID string) (int64, error) {
	hour := time.Now().Format("2006010215")
	key := fmt.Sprintf("cont:usage:%s:%s", orgID, hour)
	val, err := r.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// HourlyUsage represents a single hour's usage data point
type HourlyUsage struct {
	Hour  string `json:"hour"`  // YYYYMMDDHH format
	Count int64  `json:"count"`
}

// UsageByTimeRange returns usage aggregated by hour for a given key pattern
func (r *Redis) UsageByTimeRange(ctx context.Context, keyPattern string, startHour, endHour string) ([]HourlyUsage, error) {
	// Build list of keys for each hour in range
	var keys []string
	current := startHour
	for current <= endHour {
		key := fmt.Sprintf(keyPattern, current)
		keys = append(keys, key)
		// Advance by 1 hour
		t, _ := time.Parse("2006010215", current)
		current = t.Add(time.Hour).Format("2006010215")
	}

	if len(keys) == 0 {
		return []HourlyUsage{}, nil
	}

	// MGET for all keys
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	results := make([]HourlyUsage, 0, len(keys))
	for i := range keys {
		var count int64
		if vals[i] != nil {
			count, _ = vals[i].(int64)
		}
		hour := time.Now().Format("2006010215")
		if len(keys) == len(vals) {
			t, _ := time.Parse("2006010215", startHour)
			hour = t.Add(time.Duration(i) * time.Hour).Format("2006010215")
		}
		results = append(results, HourlyUsage{Hour: hour, Count: count})
	}
	return results, nil
}

// GetOrgUsageByHour returns hourly usage for an org within a time range
func (r *Redis) GetOrgUsageByHour(ctx context.Context, orgID, startHour, endHour string) ([]HourlyUsage, error) {
	pattern := fmt.Sprintf("cont:usage:%s:%%s", orgID)
	return r.UsageByTimeRange(ctx, pattern, startHour, endHour)
}

// GetConsumerUsageByHour returns hourly usage for a consumer within a time range
func (r *Redis) GetConsumerUsageByHour(ctx context.Context, consumerID, startHour, endHour string) ([]HourlyUsage, error) {
	pattern := fmt.Sprintf("cont:usage:consumer:%s:%%s", consumerID)
	return r.UsageByTimeRange(ctx, pattern, startHour, endHour)
}

// GetTopOrgsByUsage returns top N orgs by total usage in a time range
func (r *Redis) GetTopOrgsByUsage(ctx context.Context, startHour, endHour string, limit int) ([]struct {
	OrgID  string `json:"org_id"`
	Count  int64  `json:"count"`
}, error) {
	// Scan for all org keys in the time range
	pattern := "cont:usage:*"
	keys, _, err := r.client.Scan(ctx, 0, pattern, 1000).Result()
	if err != nil {
		return nil, err
	}

	// Aggregate by org_id
	orgCounts := make(map[string]int64)
	for _, key := range keys {
		// Key format: cont:usage:{org_id}:{YYYYMMDDHH}
		parts := strings.Split(key, ":")
		if len(parts) >= 4 {
			hourPart := parts[len(parts)-1]
			if hourPart >= startHour && hourPart <= endHour {
				orgID := strings.Join(parts[2:len(parts)-1], ":")
				val, _ := r.client.Get(ctx, key).Int64()
				orgCounts[orgID] += val
			}
		}
	}

	// Sort and take top N
	type orgUsage struct {
		OrgID string
		Count int64
	}
	sorted := make([]orgUsage, 0, len(orgCounts))
	for orgID, count := range orgCounts {
		sorted = append(sorted, orgUsage{OrgID: orgID, Count: count})
	}

	// Simple sort by count descending
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Count > sorted[i].Count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	result := make([]struct {
		OrgID  string `json:"org_id"`
		Count  int64  `json:"count"`
	}, len(sorted))
	for i, s := range sorted {
		result[i] = struct {
			OrgID  string `json:"org_id"`
			Count  int64  `json:"count"`
		}{OrgID: s.OrgID, Count: s.Count}
	}
	return result, nil
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
