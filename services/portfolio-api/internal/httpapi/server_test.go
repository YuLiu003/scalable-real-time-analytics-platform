package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/projection"
)

type fakeReader struct {
	data  []byte
	err   error
	calls *int
}

func (f fakeReader) Latest(context.Context, string) ([]byte, error) {
	if f.calls != nil {
		(*f.calls)++
	}
	return f.data, f.err
}

var validResult = []byte(`{"as_of":"2026-07-21T00:01:30Z","base_currency":"USD","benchmark":{"asset_type":"index","display_name":"Synthetic Benchmark D","instrument":"DEMO-BENCH-D","price":"1000.00000000","price_as_of":"2026-07-21T00:01:30Z","valuation_type":"index_level"},"display_name":"Synthetic Fund Portfolio","gold_parquet_object":"gold/portfolio_allocations/v2/portfolio=demo/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/allocation.parquet","input_object_count":4,"input_set_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","portfolio_id":"demo","positions":[{"allocation_pct":"100.0000","asset_type":"etf","display_name":"Synthetic Asset A","instrument":"DEMO-ASSET-A","market_value":"10.00000000","price":"1.00000000","price_as_of":"2026-07-21T00:00:00Z","quantity":"10.00000000","valuation_type":"market_price"}],"schema_version":2,"silver_parquet_object":"silver/market_prices/v1/run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/part-00000.parquet","total_market_value":"10.00000000"}`)

func TestAllocationReturnsValidatedObject(t *testing.T) {
	server := New(fakeReader{data: validResult}, "demo", []byte("dashboard"), "")
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

func TestConfigDescribesSyntheticAndPrivateModes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token string
		want  string
	}{
		{
			name: "synthetic",
			want: `{"schema_version":1,"portfolio_id":"demo","data_mode":"synthetic","access_token_required":false}` + "\n",
		},
		{
			name:  "private",
			token: strings.Repeat("a", 32),
			want:  `{"schema_version":1,"portfolio_id":"demo","data_mode":"private","access_token_required":true}` + "\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := New(fakeReader{}, "demo", nil, tt.token)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Body.String() != tt.want {
				t.Fatalf("config = %d %q, want 200 %q", response.Code, response.Body.String(), tt.want)
			}
			if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("config headers = %v", response.Header())
			}
		})
	}
}

func TestPrivateAllocationRequiresExactBearerTokenBeforeReading(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	for _, tt := range []struct {
		name       string
		headers    []string
		wantStatus int
		wantReads  int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", headers: []string{"Bearer wrong"}, wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", headers: []string{token}, wantStatus: http.StatusUnauthorized},
		{name: "extra whitespace", headers: []string{"Bearer  " + token}, wantStatus: http.StatusUnauthorized},
		{name: "multiple headers", headers: []string{"Bearer " + token, "Bearer " + token}, wantStatus: http.StatusUnauthorized},
		{name: "exact", headers: []string{"Bearer " + token}, wantStatus: http.StatusOK, wantReads: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			server := New(fakeReader{data: validResult, calls: &reads}, "demo", nil, token)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/demo/allocation", nil)
			for _, header := range tt.headers {
				request.Header.Add("Authorization", header)
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != tt.wantStatus || reads != tt.wantReads {
				t.Fatalf("allocation status = %d, reads = %d; want %d, %d", response.Code, reads, tt.wantStatus, tt.wantReads)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private allocation response must not be cached")
			}
			if tt.wantStatus == http.StatusUnauthorized {
				if response.Header().Get("WWW-Authenticate") != "Bearer" || response.Body.String() != "unauthorized\n" {
					t.Fatalf("unauthorized response headers=%v body=%q", response.Header(), response.Body.String())
				}
				if strings.Contains(response.Body.String(), token) {
					t.Fatal("unauthorized response exposed the access token")
				}
			}
		})
	}
}

func TestAuthorizationMatchesExactHeader(t *testing.T) {
	expected := sha256.Sum256([]byte("Bearer secret"))
	if !authorizationMatches([]string{"Bearer secret"}, expected) {
		t.Fatal("exact authorization header did not match")
	}
	if authorizationMatches(nil, expected) || authorizationMatches([]string{"Bearer different"}, expected) {
		t.Fatal("missing or different authorization header matched")
	}
}

func TestContributionProjectionReturnsValidatedScenarios(t *testing.T) {
	server := New(fakeReader{}, "demo", nil, strings.Repeat("p", 32))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projections/contributions", strings.NewReader(`{
		"initial_investment":"1000.00",
		"contribution_amount":"100.00",
		"contribution_frequency":"monthly",
		"years":1,
		"annual_return_pct":"0",
		"return_variance_pct":"0",
		"annual_inflation_pct":"0",
		"annual_expense_ratio_pct":"0"
	}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("projection response = %d, headers=%v, body=%s", response.Code, response.Header(), response.Body.String())
	}
	var result projection.Response
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Scenarios[1].EndingBalance != "2200.00" || result.Disclaimer != projection.Disclaimer {
		t.Fatalf("projection = %+v", result)
	}
}

func TestContributionProjectionRejectsInvalidAndOversizedRequests(t *testing.T) {
	server := New(fakeReader{}, "demo", nil, "")
	for _, body := range []string{
		`{"initial_investment":"invalid"}`,
		`{"initial_investment":"0","contribution_amount":"0","contribution_frequency":"monthly","years":0,"annual_return_pct":"0","return_variance_pct":"0","annual_inflation_pct":"0","annual_expense_ratio_pct":"0"}`,
		strings.Repeat("x", maximumProjectionRequestBytes+1),
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/projections/contributions", strings.NewReader(body))
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("projection status = %d, want 400; body=%s", response.Code, response.Body.String())
		}
	}
}

func TestReadinessFailsWhenResultIsUnavailable(t *testing.T) {
	server := New(fakeReader{err: errors.New("S3 unavailable")}, "demo", nil, "")
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestHealthDoesNotDependOnObjectStorage(t *testing.T) {
	server := New(fakeReader{err: errors.New("S3 unavailable")}, "demo", nil, strings.Repeat("p", 32))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}

func TestReadinessReturnsReadyForValidResult(t *testing.T) {
	server := New(fakeReader{data: validResult}, "demo", nil, strings.Repeat("p", 32))
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "ready\n" {
		t.Fatalf("readiness = %d %q", response.Code, response.Body.String())
	}
}

func TestAllocationRejectsUnknownPortfolioAndUnavailableResult(t *testing.T) {
	tests := []struct {
		name      string
		reader    fakeReader
		portfolio string
		want      int
	}{
		{name: "unknown route portfolio", reader: fakeReader{data: validResult}, portfolio: "other", want: http.StatusNotFound},
		{name: "invalid result", reader: fakeReader{data: []byte(`{}`)}, portfolio: "demo", want: http.StatusServiceUnavailable},
		{name: "reader error", reader: fakeReader{err: errors.New("unavailable")}, portfolio: "demo", want: http.StatusServiceUnavailable},
		{
			name:      "result portfolio mismatch",
			reader:    fakeReader{data: []byte(strings.ReplaceAll(string(validResult), "demo", "other"))},
			portfolio: "demo",
			want:      http.StatusServiceUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := New(tt.reader, "demo", nil, "")
			request := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/"+tt.portfolio+"/allocation", nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status = %d, want %d", response.Code, tt.want)
			}
		})
	}
}

func TestIndexServesDashboardAndRejectsUnknownPath(t *testing.T) {
	server := New(fakeReader{}, "demo", []byte(`<h1>dashboard</h1><form id="projection-form"></form>`), strings.Repeat("p", 32))
	for _, tt := range []struct {
		path string
		want int
	}{
		{path: "/", want: http.StatusOK},
		{path: "/unknown", want: http.StatusNotFound},
	} {
		request := httptest.NewRequest(http.MethodGet, tt.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tt.want {
			t.Fatalf("GET %s status = %d, want %d", tt.path, response.Code, tt.want)
		}
		if tt.path == "/" && !strings.Contains(response.Body.String(), `id="projection-form"`) {
			t.Fatal("dashboard does not expose the projection form")
		}
	}
}
