package alpaca

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHistoryClientBarsPaginatesAuthenticatesAndSorts(t *testing.T) {
	start := time.Date(2026, 8, 3, 12, 0, 0, 123, time.FixedZone("private", -7*60*60))
	end := start.Add(10 * time.Minute)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet {
			t.Errorf("method = %s", request.Method)
		}
		if request.Header.Get("APCA-API-KEY-ID") != "FAKEPACA_KEY" || request.Header.Get("APCA-API-SECRET-KEY") != "FAKEPACA_SECRET" {
			t.Errorf("authentication headers were not sent")
		}
		query := request.URL.Query()
		wantQuery := map[string]string{
			"symbols": "LOAD-A,LOAD-B", "timeframe": "1Min", "start": start.UTC().Format(time.RFC3339Nano),
			"end": end.UTC().Format(time.RFC3339Nano), "limit": strconv.Itoa(historyPageLimit), "adjustment": "raw", "feed": "iex", "sort": "asc",
		}
		for key, want := range wantQuery {
			if got := query.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		response.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			if query.Get("page_token") != "" {
				t.Errorf("first page token = %q", query.Get("page_token"))
			}
			_, _ = io.WriteString(response, `{"bars":{"LOAD-B":[{"c":202.5,"n":9,"t":"2026-08-03T19:02:00Z"}],"LOAD-A":[{"c":101.25,"n":8,"t":"2026-08-03T19:02:00Z"}]},"next_page_token":"private-token"}`)
			return
		}
		if query.Get("page_token") != "private-token" {
			t.Errorf("second page token = %q", query.Get("page_token"))
		}
		_, _ = io.WriteString(response, `{"bars":{"LOAD-A":[{"c":100,"n":7,"t":"2026-08-03T19:01:00Z"}]},"next_page_token":null}`)
	}))
	defer server.Close()

	client := HistoryClient{URL: server.URL, Feed: "iex", KeyID: "FAKEPACA_KEY", SecretKey: "FAKEPACA_SECRET", HTTP: server.Client()}
	bars, err := client.Bars(context.Background(), []string{"LOAD-A", "LOAD-B"}, start, end)
	if err != nil {
		t.Fatalf("Bars() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	want := []Bar{
		{Instrument: "LOAD-A", Timestamp: time.Date(2026, 8, 3, 19, 1, 0, 0, time.UTC), Close: "100", TradeCount: 7},
		{Instrument: "LOAD-A", Timestamp: time.Date(2026, 8, 3, 19, 2, 0, 0, time.UTC), Close: "101.25", TradeCount: 8},
		{Instrument: "LOAD-B", Timestamp: time.Date(2026, 8, 3, 19, 2, 0, 0, time.UTC), Close: "202.5", TradeCount: 9},
	}
	if !reflect.DeepEqual(bars, want) {
		t.Fatalf("Bars() = %#v, want %#v", bars, want)
	}
}

func TestHistoryClientBarsRejectsInvalidInterval(t *testing.T) {
	client := HistoryClient{}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	for _, start := range []time.Time{now, now.Add(time.Second)} {
		if _, err := client.Bars(context.Background(), []string{"LOAD-A"}, start, now); err == nil {
			t.Fatalf("Bars(%s, %s) succeeded", start, now)
		}
	}
}

func TestHistoryClientBarsPropagatesPageError(t *testing.T) {
	client := historyClientWithTransport(func(*http.Request) (*http.Response, error) {
		return testHTTPResponse(http.StatusTooManyRequests, "FAKEPACA_SECRET LOAD-A"), nil
	})
	_, err := client.Bars(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute))
	var provider ProviderError
	if !errors.As(err, &provider) || provider.Code != http.StatusTooManyRequests {
		t.Fatalf("Bars() error = %#v", err)
	}
}

func TestHistoryClientBarsValidatesProviderData(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "unrequested", body: `{"bars":{"LOAD-B":[]}}`, want: "unrequested instrument"},
		{name: "timestamp", body: `{"bars":{"LOAD-A":[{"c":1,"n":1,"t":"private"}]}}`, want: "invalid timestamp"},
		{name: "bar", body: `{"bars":{"LOAD-A":[{"c":0,"n":1,"t":"2026-08-03T12:00:00Z"}]}}`, want: "invalid positive close"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := historyClientWithTransport(func(*http.Request) (*http.Response, error) {
				return testHTTPResponse(http.StatusOK, test.body), nil
			})
			_, err := client.Bars(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Bars() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestHistoryClientBarsRejectsRepeatedAndExcessPagination(t *testing.T) {
	repeatedCalls := 0
	repeated := historyClientWithTransport(func(*http.Request) (*http.Response, error) {
		repeatedCalls++
		return testHTTPResponse(http.StatusOK, `{"bars":{},"next_page_token":"repeat"}`), nil
	})
	_, err := repeated.Bars(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "repeated a pagination token") || repeatedCalls != 2 {
		t.Fatalf("repeated Bars() error = %v, calls = %d", err, repeatedCalls)
	}

	pages := 0
	excess := historyClientWithTransport(func(request *http.Request) (*http.Response, error) {
		pages++
		next := strconv.Itoa(pages)
		return testHTTPResponse(http.StatusOK, `{"bars":{},"next_page_token":"`+next+`"}`), nil
	})
	_, err = excess.Bars(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "exceeded the pagination bound") || pages != maximumHistoryPages {
		t.Fatalf("excess Bars() error = %v, pages = %d", err, pages)
	}
}

func TestHistoryClientPageRequestFailures(t *testing.T) {
	start := testHistoryStart()
	client := historyClientWithTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("FAKEPACA_SECRET transport")
	})
	_, err := client.page(context.Background(), []string{"LOAD-A"}, start, start.Add(time.Minute), "")
	if err == nil || err.Error() != "request historical market data" {
		t.Fatalf("generic page error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.page(ctx, []string{"LOAD-A"}, start, start.Add(time.Minute), "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled page error = %v", err)
	}

	client.URL = "%"
	_, err = client.page(context.Background(), []string{"LOAD-A"}, start, start.Add(time.Minute), "")
	if err == nil || !strings.Contains(err.Error(), "endpoint is invalid") {
		t.Fatalf("invalid endpoint error = %v", err)
	}

	client.URL = "https://history.invalid/v2/test"
	_, err = client.page(nil, []string{"LOAD-A"}, start, start.Add(time.Minute), "")
	if err == nil || !strings.Contains(err.Error(), "build historical provider request") {
		t.Fatalf("nil context error = %v", err)
	}
}

func TestHistoryClientPageStatusBodyAndJSONValidation(t *testing.T) {
	tests := []struct {
		name      string
		transport func(*http.Request) (*http.Response, error)
		want      string
		code      int
	}{
		{name: "provider status", want: "status 429", code: 429, transport: func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(429, "FAKEPACA_SECRET LOAD-A"), nil
		}},
		{name: "read", want: "read historical", transport: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: failingReadCloser{}}, nil
		}},
		{name: "oversize", want: "size bound", transport: func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(http.StatusOK, strings.Repeat(" ", maximumHistoryBody+1)), nil
		}},
		{name: "malformed", want: "invalid JSON", transport: func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(http.StatusOK, `{`), nil
		}},
		{name: "nil bars", want: "invalid JSON", transport: func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(http.StatusOK, `{"bars":null}`), nil
		}},
		{name: "multiple", want: "multiple JSON values", transport: func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(http.StatusOK, `{"bars":{}} {}`), nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := historyClientWithTransport(test.transport)
			_, err := client.page(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute), "private-token")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("page() error = %v, want substring %q", err, test.want)
			}
			if strings.Contains(err.Error(), "FAKEPACA_SECRET") || strings.Contains(err.Error(), "LOAD-A") || strings.Contains(err.Error(), "private-token") {
				t.Fatalf("page() leaked private data: %v", err)
			}
			if test.code != 0 {
				var provider ProviderError
				if !errors.As(err, &provider) || provider.Code != test.code {
					t.Fatalf("page() error = %#v, want provider code %d", err, test.code)
				}
			}
		})
	}
}

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (function testRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func historyClientWithTransport(function testRoundTripFunc) HistoryClient {
	return HistoryClient{
		URL: "https://history.invalid/v2/test", Feed: "iex", KeyID: "FAKEPACA_KEY", SecretKey: "FAKEPACA_SECRET",
		HTTP: &http.Client{Transport: function},
	}
}

func testHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func testHistoryStart() time.Time {
	return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("FAKEPACA_SECRET read") }
func (failingReadCloser) Close() error             { return nil }

func TestHistoryPageEncodesExistingEndpointQuery(t *testing.T) {
	var got url.Values
	client := historyClientWithTransport(func(request *http.Request) (*http.Response, error) {
		got = request.URL.Query()
		return testHTTPResponse(http.StatusOK, `{"bars":{}}`), nil
	})
	client.URL += "?existing=private"
	_, err := client.page(context.Background(), []string{"LOAD-A"}, testHistoryStart(), testHistoryStart().Add(time.Minute), "token")
	if err != nil {
		t.Fatalf("page() error = %v", err)
	}
	if got.Get("existing") != "private" || got.Get("page_token") != "token" {
		t.Fatalf("query = %v", got)
	}
}
