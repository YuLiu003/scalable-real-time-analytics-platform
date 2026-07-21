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

var validResult = []byte(`{"as_of":"2026-07-21T00:01:30Z","base_currency":"USD","benchmark":{"asset_type":"index","display_name":"S&P 500 Index","instrument":"SP500","price":"6500.00000000","price_as_of":"2026-07-21T00:01:30Z","valuation_type":"index_level"},"display_name":"Synthetic Fund Portfolio","gold_parquet_object":"gold/portfolio_allocations/v2/portfolio=demo/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/allocation.parquet","input_object_count":4,"input_set_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","portfolio_id":"demo","positions":[{"allocation_pct":"100.0000","asset_type":"etf","display_name":"Invesco QQQ","instrument":"QQQ","market_value":"10.00000000","price":"1.00000000","price_as_of":"2026-07-21T00:00:00Z","quantity":"10.00000000","valuation_type":"market_price"}],"schema_version":2,"silver_parquet_object":"silver/market_prices/v1/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/part-00000.parquet","total_market_value":"10.00000000"}`)

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
