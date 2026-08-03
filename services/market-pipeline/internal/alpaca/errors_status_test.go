package alpaca

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestProviderErrorClassificationAndPrivacy(t *testing.T) {
	for _, code := range []int{301, 302, 307, 308, 400, 401, 402, 403, 404, 405, 409, 410} {
		err := providerError(code)
		if !err.Permanent || !isPermanent(err) {
			t.Errorf("providerError(%d) = %#v, want permanent", code, err)
		}
		if strings.Contains(err.Error(), "FAKEPACA") || strings.Contains(err.Error(), "LOAD-A") {
			t.Fatalf("private data leaked in %q", err.Error())
		}
	}
	for _, code := range []int{0, 408, 429, 500} {
		err := providerError(code)
		if err.Permanent || isPermanent(err) {
			t.Errorf("providerError(%d) = %#v, want transient", code, err)
		}
	}
	if isPermanent(http.ErrServerClosed) {
		t.Fatal("non-provider error classified as permanent")
	}
	if err := fmt.Errorf("session setup: %w", providerError(http.StatusUnauthorized)); !isPermanent(err) {
		t.Fatalf("wrapped provider error was not classified as permanent: %v", err)
	}
}

func TestStatusEndpointsMetricsAndPrivacy(t *testing.T) {
	status := NewStatus()
	handler := status.Handler()

	assertHTTPResponse(t, handler, "/healthz", http.StatusOK, "text/plain; charset=utf-8", "ok\n")
	assertHTTPResponse(t, handler, "/readyz", http.StatusServiceUnavailable, "text/plain; charset=utf-8", "not ready\n")

	status.SetConnected(true)
	status.SetReady(true)
	status.RecordReconnect()
	status.RecordBackfill()
	status.RecordPublished()
	status.RecordFailure()
	assertHTTPResponse(t, handler, "/readyz", http.StatusOK, "text/plain; charset=utf-8", "ready\n")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics response = %d, %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	want := "market_feed_connected 1\n" +
		"market_feed_ready 1\n" +
		"market_feed_connections_total 1\n" +
		"market_feed_reconnects_total 1\n" +
		"market_feed_backfill_requests_total 1\n" +
		"market_feed_published_events_total 1\n" +
		"market_feed_failures_total 1\n"
	if recorder.Body.String() != want {
		t.Fatalf("metrics = %q, want %q", recorder.Body.String(), want)
	}
	for _, private := range []string{"FAKEPACA_KEY", "FAKEPACA_SECRET", "LOAD-A", "fakepaca"} {
		if strings.Contains(recorder.Body.String(), private) {
			t.Fatalf("metrics leak %q: %q", private, recorder.Body.String())
		}
	}

	status.SetConnected(false)
	status.SetReady(false)
	metrics := status.metrics()
	if !strings.Contains(metrics, "market_feed_connected 0\n") || !strings.Contains(metrics, "market_feed_ready 0\n") {
		t.Fatalf("disconnected metrics = %q", metrics)
	}
}

func TestStatusIsRaceSafe(t *testing.T) {
	status := NewStatus()
	var group sync.WaitGroup
	for index := 0; index < 20; index++ {
		group.Add(1)
		go func(value bool) {
			defer group.Done()
			status.SetConnected(value)
			status.SetReady(value)
			status.RecordReconnect()
			status.RecordBackfill()
			status.RecordPublished()
			status.RecordFailure()
			_ = status.metrics()
		}(index%2 == 0)
	}
	group.Wait()
	metrics := status.metrics()
	for _, want := range []string{
		"market_feed_connections_total 10", "market_feed_reconnects_total 20", "market_feed_backfill_requests_total 20",
		"market_feed_published_events_total 20", "market_feed_failures_total 20",
	} {
		if !strings.Contains(metrics, want) {
			t.Errorf("metrics %q does not contain %q", metrics, want)
		}
	}
}

func assertHTTPResponse(t *testing.T, handler http.Handler, path string, code int, contentType, body string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != code || recorder.Header().Get("Content-Type") != contentType || recorder.Body.String() != body {
		t.Fatalf("GET %s = (%d, %q, %q), want (%d, %q, %q)", path, recorder.Code,
			recorder.Header().Get("Content-Type"), recorder.Body.String(), code, contentType, body)
	}
}
