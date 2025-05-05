package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestCount tracks the number of HTTP requests
	RequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "visualization_request_count",
			Help: "Total number of requests to the visualization service",
		},
		[]string{"path", "method", "status"},
	)

	// ResponseTime tracks request response time
	ResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "visualization_response_time_seconds",
			Help:    "Response time of the visualization service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

	// WebSocketConnections tracks active WebSocket connections
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "visualization_websocket_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	// WebSocketMessages tracks WebSocket messages sent
	WebSocketMessages = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "visualization_websocket_messages_sent",
			Help: "Total number of WebSocket messages sent",
		},
	)

	// DataServiceLatency tracks latency of data service requests
	DataServiceLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "data_service_request_latency_seconds",
			Help:    "Latency of requests to the data service",
			Buckets: prometheus.DefBuckets,
		},
	)

	// DataServiceErrors tracks errors from data service
	DataServiceErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "data_service_request_errors",
			Help: "Total number of errors from the data service",
		},
	)
)
