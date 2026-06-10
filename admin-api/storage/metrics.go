package storage

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kong_nginx_requests_total",
			Help: "Total number of requests",
		},
		[]string{"method", "scheme", "service", "route", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kong_request_duration_ms",
			Help:    "Request duration in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
		},
		[]string{"method", "route"},
	)

	HTTPConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kong_nginx_connections_total",
			Help: "Number of connections",
		},
		[]string{"state"},
	)

	// Kong-specific metrics
	ServiceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kong_service_latency_ms",
			Help:    "Service latency in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"service_name", "service_id"},
	)

	UpstreamLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kong_upstream_latency_ms",
			Help:    "Upstream latency in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"upstream_name", "upstream_id"},
	)

	// Database metrics
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cont_db_query_duration_ms",
			Help:    "Database query duration in milliseconds",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 25, 50, 100},
		},
		[]string{"operation", "entity"},
	)

	DBConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cont_db_connections_active",
			Help: "Number of active database connections",
		},
	)

	// Redis metrics
	RedisConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cont_redis_connections_active",
			Help: "Number of active Redis connections",
		},
	)

	RedisLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cont_redis_latency_ms",
			Help:    "Redis operation latency in milliseconds",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 25, 50},
		},
		[]string{"operation"},
	)

	// Business metrics
	EntitiesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cont_entities_total",
			Help: "Total number of entities in the database",
		},
		[]string{"entity"},
	)

	ConsumerLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kong_consumer_latency_ms",
			Help:    "Consumer request latency in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250},
		},
		[]string{"consumer_id"},
	)

	// Plugin metrics
	PluginInvocationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kong_plugin_invocations_total",
			Help: "Total number of plugin invocations",
		},
		[]string{"plugin_name", "phase", "result"},
	)
)
