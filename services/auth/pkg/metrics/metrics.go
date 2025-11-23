package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	DBOperations        *prometheus.CounterVec
	DBOperationDuration *prometheus.HistogramVec
	ActiveSessions      prometheus.Gauge
}

// NewMetrics initializes and returns a new Metrics instance
func NewMetrics(serviceName string) *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: serviceName + "_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    serviceName + "_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		DBOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: serviceName + "_db_operations_total",
				Help: "Total number of database operations",
			},
			[]string{"operation", "collection", "status"},
		),
		DBOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    serviceName + "_db_operation_duration_seconds",
				Help:    "Database operation duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"operation", "collection"},
		),
		ActiveSessions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: serviceName + "_active_sessions",
				Help: "Number of active user sessions",
			},
		),
	}
}
