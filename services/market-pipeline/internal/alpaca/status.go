package alpaca

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Status struct {
	mu               sync.RWMutex
	connected        bool
	ready            bool
	connections      uint64
	reconnects       uint64
	backfillRequests uint64
	published        uint64
	failures         uint64
}

func NewStatus() *Status {
	return &Status{}
}

func (status *Status) SetConnected(value bool) {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.connected = value
	if value {
		status.connections++
	}
}

func (status *Status) SetReady(value bool) {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.ready = value
}

func (status *Status) RecordReconnect() {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.reconnects++
}

func (status *Status) RecordBackfill() {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.backfillRequests++
}

func (status *Status) RecordPublished() {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.published++
}

func (status *Status) RecordFailure() {
	status.mu.Lock()
	defer status.mu.Unlock()
	status.failures++
}

func (status *Status) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(response http.ResponseWriter, _ *http.Request) {
		status.mu.RLock()
		ready := status.ready
		status.mu.RUnlock()
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !ready {
			http.Error(response, "not ready", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ready\n"))
	})
	mux.HandleFunc("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = response.Write([]byte(status.metrics()))
	})
	return mux
}

func (status *Status) metrics() string {
	status.mu.RLock()
	defer status.mu.RUnlock()
	connected := 0
	if status.connected {
		connected = 1
	}
	ready := 0
	if status.ready {
		ready = 1
	}
	values := []string{
		fmt.Sprintf("market_feed_connected %d", connected),
		fmt.Sprintf("market_feed_ready %d", ready),
		fmt.Sprintf("market_feed_connections_total %d", status.connections),
		fmt.Sprintf("market_feed_reconnects_total %d", status.reconnects),
		fmt.Sprintf("market_feed_backfill_requests_total %d", status.backfillRequests),
		fmt.Sprintf("market_feed_published_events_total %d", status.published),
		fmt.Sprintf("market_feed_failures_total %d", status.failures),
	}
	return strings.Join(values, "\n") + "\n"
}
