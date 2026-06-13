// routes/metrics.go
// Custom Prometheus metrics handler that includes DB/Redis pool metrics

package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ── DB Pool Stats ─────────────────────────────────────────────────────────────
type dbStatProvider interface {
	MaxOpenConnections() int
	OpenConnections() int
	Idle() int
}

var dbStats dbStatProvider

// ── Redis Pool Stats ──────────────────────────────────────────────────────────
type redisPoolInfo interface {
	TotalConns() int
	IdleConns() int
}

var redisPoolStats redisPoolInfo

// ── Prometheus GaugeFuncs ──────────────────────────────────────────────────────
var DBPoolMax = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name: "cont_db_connections_max",
		Help: "Max database connections configured",
	},
	func() float64 {
		if dbStats != nil {
			return float64(dbStats.MaxOpenConnections())
		}
		return 0
	},
)

var DBPoolOpen = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name: "cont_db_connections_active",
		Help: "Number of active database connections",
	},
	func() float64 {
		if dbStats != nil {
			return float64(dbStats.OpenConnections())
		}
		return 0
	},
)

var DBPoolIdle = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name: "cont_db_connections_idle",
		Help: "Number of idle database connections",
	},
	func() float64 {
		if dbStats != nil {
			return float64(dbStats.Idle())
		}
		return 0
	},
)

var RedisPoolActive = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name: "cont_redis_connections_active",
		Help: "Number of active Redis connections",
	},
	func() float64 {
		if redisPoolStats != nil {
			return float64(redisPoolStats.TotalConns())
		}
		return 0
	},
)

var RedisPoolIdle = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name: "cont_redis_connections_idle",
		Help: "Number of idle Redis connections",
	},
	func() float64 {
		if redisPoolStats != nil {
			return float64(redisPoolStats.IdleConns())
		}
		return 0
	},
)

// RegisterPoolStats registers DB and Redis pool stat suppliers and registers pool metric gauges.
func RegisterPoolStats(getDB func() dbStatProvider, getRedis func() redisPoolInfo) {
	if getDB != nil {
		dbStats = getDB()
	}
	if getRedis != nil {
		redisPoolStats = getRedis()
	}
	// Register pool GaugeFuncs so they appear in /metrics
	// Use Register (not MustRegister) — safe if called multiple times
	prometheus.DefaultRegisterer.Register(DBPoolMax)
	prometheus.DefaultRegisterer.Register(DBPoolOpen)
	prometheus.DefaultRegisterer.Register(DBPoolIdle)
	prometheus.DefaultRegisterer.Register(RedisPoolActive)
	prometheus.DefaultRegisterer.Register(RedisPoolIdle)
}

// Metrics returns a handler that exposes Prometheus metrics including pool stats
func Metrics() gin.HandlerFunc {
	h := promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
