// routes/metrics.go
// Custom Prometheus metrics handler that includes DB/Redis pool metrics

package routes

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ── DB Pool Gauges ─────────────────────────────────────────────────────────────
var (
	DBPoolMaxConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cont_db_connections_max",
		Help: "Max database connections configured",
	})
	DBPoolOpenConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cont_db_connections_active",
		Help: "Number of active database connections",
	})
	DBPoolIdleConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cont_db_connections_idle",
		Help: "Number of idle database connections",
	})
)

// ── Redis Pool Gauges ─────────────────────────────────────────────────────────
var (
	RedisPoolActiveConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cont_redis_connections_active",
		Help: "Number of active Redis connections",
	})
	RedisPoolIdleConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cont_redis_connections_idle",
		Help: "Number of idle Redis connections",
	})
)

// ── Pool Stats Updater ────────────────────────────────────────────────────────
type poolStatsProvider interface {
	DBPoolStats() struct {
		MaxOpen int
		Open    int
		Idle    int
	}
	RedisPoolStats() struct {
		TotalConns int
		IdleConns  int
	}
}

var poolProvider poolStatsProvider
var poolUpdateOnce sync.Once
var poolStopChan = make(chan struct{})

// RegisterPoolStats starts a background goroutine that updates pool metrics every 10s
func RegisterPoolStats(provider poolStatsProvider) {
	poolProvider = provider
	poolUpdateOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			updatePoolMetrics()
			for {
				select {
				case <-ticker.C:
					updatePoolMetrics()
				case <-poolStopChan:
					return
				}
			}
		}()
	})
}

func updatePoolMetrics() {
	if poolProvider == nil {
		return
	}
	dbStats := poolProvider.DBPoolStats()
	DBPoolMaxConns.Set(float64(dbStats.MaxOpen))
	DBPoolOpenConns.Set(float64(dbStats.Open))
	DBPoolIdleConns.Set(float64(dbStats.Idle))

	redisStats := poolProvider.RedisPoolStats()
	RedisPoolActiveConns.Set(float64(redisStats.TotalConns))
	RedisPoolIdleConns.Set(float64(redisStats.IdleConns))
}

// Metrics returns a handler that exposes Prometheus metrics including pool stats
func Metrics() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		// Update once at request time to get latest values
		updatePoolMetrics()
		h.ServeHTTP(c.Writer, c.Request)
	}
}
