package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics
var (
	// requestCount counts processing requests
	//nolint:unused // To be used in future processing metrics collection
	requestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "process_request_count",
			Help: "Processing Request Count",
		},
		[]string{"status"},
	)

	// processingLatency measures processing latency
	//nolint:unused // To be used in future processing metrics collection
	processingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "processing_latency_seconds",
			Help:    "Processing latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	// tenantMessagesProcessed counts messages processed per tenant
	//nolint:unused // To be used in future tenant metrics collection
	tenantMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tenant_messages_processed",
			Help: "Messages processed by tenant",
		},
		[]string{"tenant_id"},
	)

	// tenantProcessingLatency measures processing latency per tenant
	//nolint:unused // To be used in future tenant metrics collection
	tenantProcessingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tenant_processing_latency_seconds",
			Help:    "Processing latency by tenant",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tenant_id"},
	)
)
