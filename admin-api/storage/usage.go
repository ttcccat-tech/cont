package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// IncrUsageRequest represents the request body for IncrUsage
type IncrUsageRequest struct {
	OrgID       string `json:"org_id"`
	ConsumerID  string `json:"consumer_id"`
	RouteID     string `json:"route_id"`
	ServiceID   string `json:"service_id"`
	LatencyMs   int64  `json:"latency_ms"`
	StatusCode  int    `json:"status_code"`
}

// IncrUsageResponse represents the response from IncrUsage
type IncrUsageResponse struct {
	Success bool  `json:"success"`
	Count   int64 `json:"count"`
}

// IncrUsage increments the hourly API request counter and stores detailed usage info.
// Key format: cont:usage:{org_id}:{YYYYMMDDHH}
// Hash field: {consumer_id}:{route_id} -> JSON payload
// TTL: 62 days (covers month boundary + buffer)
func (r *Redis) IncrUsage(ctx context.Context, orgID, consumerID, routeID, serviceID string, latencyMs int64, statusCode int) (int64, error) {
	hour := time.Now().Format("2006010215")

	// Main org counter using INCR
	orgKey := fmt.Sprintf("cont:usage:%s:%s", orgID, hour)
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, orgKey)
	pipe.Expire(ctx, orgKey, 62*24*60*60) // 62 days TTL

	// Hash storage for detailed info: HSET cont:usage:{org_id}:{YYYYMMDDHH} {consumer_id}:{route_id} {json}
	hashKey := orgKey
	fieldKey := fmt.Sprintf("%s:%s", consumerID, routeID)
	detailJSON, _ := json.Marshal(map[string]interface{}{
		"consumer_id": consumerID,
		"route_id":    routeID,
		"service_id":  serviceID,
		"latency_ms":  latencyMs,
		"status_code": statusCode,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	pipe.HSet(ctx, hashKey, fieldKey, string(detailJSON))
	pipe.Expire(ctx, hashKey, 62*24*60*60)

	// Consumer counter (only if consumer_id is present)
	if consumerID != "" {
		consumerKey := fmt.Sprintf("cont:usage:consumer:%s:%s", consumerID, hour)
		pipe.Incr(ctx, consumerKey)
		pipe.Expire(ctx, consumerKey, 62*24*60*60)
	}

	// Route counter (only if route_id is present)
	if routeID != "" {
		routeKey := fmt.Sprintf("cont:usage:route:%s:%s", routeID, hour)
		pipe.Incr(ctx, routeKey)
		pipe.Expire(ctx, routeKey, 62*24*60*60)
	}

	// Service counter (only if service_id is present)
	if serviceID != "" {
		serviceKey := fmt.Sprintf("cont:usage:service:%s:%s", serviceID, hour)
		pipe.Incr(ctx, serviceKey)
		pipe.Expire(ctx, serviceKey, 62*24*60*60)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
