package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeReader struct {
	data []byte
	err  error
}

func (f fakeReader) Latest(context.Context, string) ([]byte, error) { return f.data, f.err }

var validResult = []byte(`{"as_of":"2026-07-21T00:01:30Z","base_currency":"USD","gold_parquet_object":"gold/portfolio_allocations/v1/portfolio=demo/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/allocation.parquet","input_object_count":4,"input_set_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","portfolio_id":"demo","positions":[{"allocation_pct":"100.0000","instrument":"AAPL","market_value":"10.00000000","price":"1.00000000","price_as_of":"2026-07-21T00:00:00Z","quantity":"10.00000000"}],"schema_version":1,"silver_parquet_object":"silver/market_prices/v1/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/part-00000.parquet","total_market_value":"10.00000000"}`)

func TestAllocationReturnsValidatedObject(t *testing.T) {
	server := New(fakeReader{data: validResult}, "demo", []byte("dashboard"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/demo/allocation", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("allocation response must not be cached")
	}
}

func TestReadinessFailsWhenResultIsUnavailable(t *testing.T) {
	server := New(fakeReader{err: errors.New("S3 unavailable")}, "demo", nil)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestHealthDoesNotDependOnObjectStorage(t *testing.T) {
	server := New(fakeReader{err: errors.New("S3 unavailable")}, "demo", nil)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}
