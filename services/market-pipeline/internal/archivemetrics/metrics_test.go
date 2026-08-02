package archivemetrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRecorderRejectsInvalidConfigurationAndObservation(t *testing.T) {
	if _, err := New("private"); err == nil {
		t.Fatal("New() accepted an unbounded scope")
	}
	recorder, err := New("baseline")
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Observe("unknown", time.Second, time.Second); err == nil {
		t.Fatal("Observe() accepted an unknown outcome")
	}
	if err := recorder.Observe("error", -time.Second, 0); err == nil {
		t.Fatal("Observe() accepted a negative duration")
	}
}

func TestMetricsHandlerRendersBoundedPrometheusState(t *testing.T) {
	recorder, err := New("scale")
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Observe("created", 4*time.Millisecond, 12*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Observe("duplicate", 2*time.Second, 12*time.Second); err != nil {
		t.Fatal(err)
	}
	handler := recorder.Handler()

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, fragment := range []string{
		`market_archiver_events_total{scope="scale",outcome="created"} 1`,
		`market_archiver_events_total{scope="scale",outcome="duplicate"} 1`,
		`market_archiver_events_total{scope="scale",outcome="quarantined"} 0`,
		`market_archiver_processing_duration_seconds_bucket{scope="scale",le="0.005"} 1`,
		`market_archiver_processing_duration_seconds_bucket{scope="scale",le="+Inf"} 2`,
		`market_archiver_durable_latency_seconds_bucket{scope="scale",le="10"} 1`,
		`market_archiver_durable_latency_seconds_bucket{scope="scale",le="+Inf"} 2`,
		`market_archiver_processing_duration_seconds_count{scope="scale"} 2`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("metrics body does not contain %q:\n%s", fragment, text)
		}
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q", contentType)
	}

	notFound := httptest.NewRecorder()
	handler.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d", notFound.Code)
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d", method.Code)
	}
}
