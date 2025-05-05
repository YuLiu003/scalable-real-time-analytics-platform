package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics
var (
	// storageCount counts storage operations
	storageCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_count",
			Help: "Storage Operation Count",
		},
		[]string{"operation", "status"},
	)

	// storageLatency measures storage operation latency
	storageLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storage_latency_seconds",
			Help:    "Storage Operation Latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// recordsByTenant tracks number of records by tenant
	recordsByTenant = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "records_by_tenant",
			Help: "Number of Records by Tenant",
		},
		[]string{"tenant_id"},
	)
)
