package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestsTotal counts every HTTP request handled, labeled by method,
// route (echo's registered path, e.g. "/openapi/v1.0/transfer-va/inquiry",
// not the raw URL, to keep cardinality bounded) and response status class.
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests processed, labeled by method, route and status code.",
	},
	[]string{"method", "route", "status"},
)

// HTTPRequestDuration observes request latency in seconds, labeled by
// method and route. Used to derive p50/p95/p99 latency in Grafana.
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, labeled by method and route.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "route"},
)
