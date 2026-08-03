package alpaca

import (
	"context"
	"crypto/sha1" // #nosec G505 -- WebSocket RFC 6455 requires SHA-1 for its public handshake token.
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestStreamClientConnectAuthenticatesSubscribesAndReads(t *testing.T) {
	timestamp := "2026-08-03T12:00:00Z"
	server := newWebsocketTestServer(t, func(ctx context.Context, connection *websocket.Conn) {
		writeTestWSJSON(t, ctx, connection, []map[string]any{{"T": "success", "msg": "connected"}})
		var auth map[string]string
		readTestWSJSON(t, ctx, connection, &auth)
		wantAuth := map[string]string{"action": "auth", "key": "FAKEPACA_KEY", "secret": "FAKEPACA_SECRET"}
		if !reflect.DeepEqual(auth, wantAuth) {
			t.Errorf("auth = %#v, want %#v", auth, wantAuth)
		}
		writeTestWSJSON(t, ctx, connection, []map[string]any{{"T": "success", "msg": "authenticated"}})
		var subscription struct {
			Action  string   `json:"action"`
			Bars    []string `json:"bars"`
			Updated []string `json:"updatedBars"`
		}
		readTestWSJSON(t, ctx, connection, &subscription)
		if subscription.Action != "subscribe" || !sameStrings(subscription.Bars, []string{"LOAD-A", "LOAD-B"}) ||
			!sameStrings(subscription.Updated, []string{"LOAD-A", "LOAD-B"}) {
			t.Errorf("subscription = %#v", subscription)
		}
		writeTestWSJSON(t, ctx, connection, []map[string]any{{
			"T": "subscription", "bars": []string{"LOAD-B", "LOAD-A"}, "updatedBars": []string{"LOAD-A", "LOAD-B"},
		}})
		writeTestWSJSON(t, ctx, connection, []map[string]any{
			{"T": "success", "msg": "late control"},
			{"T": "subscription", "bars": []string{"LOAD-A"}},
			{"T": "b", "S": "LOAD-A", "c": 101.25, "n": 7, "t": timestamp},
			{"T": "u", "S": "LOAD-B", "c": 202, "n": 8, "t": timestamp},
		})
		writeTestWSJSON(t, ctx, connection, []map[string]any{{"T": "error", "code": 429, "msg": "FAKEPACA_SECRET"}})
	})
	defer server.Close()

	client := StreamClient{URL: websocketURL(server.URL), KeyID: "FAKEPACA_KEY", SecretKey: "FAKEPACA_SECRET", HTTP: server.Client()}
	session, err := client.Connect(context.Background(), []string{"LOAD-A", "LOAD-B"})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	bars, err := session.Read(context.Background())
	if err != nil || len(bars) != 2 || bars[0].Instrument != "LOAD-A" || bars[1].Instrument != "LOAD-B" {
		t.Fatalf("Read() = %#v, %v", bars, err)
	}
	_, err = session.Read(context.Background())
	var provider ProviderError
	if !errors.As(err, &provider) || provider.Code != 429 || strings.Contains(err.Error(), "FAKEPACA_SECRET") {
		t.Fatalf("provider Read() error = %v", err)
	}
	session.Close()
}

func TestProviderHTTPClientRejectsRedirects(t *testing.T) {
	client := NewProviderHTTPClient(3 * time.Second)
	if client.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v", client.Timeout)
	}
	request, err := http.NewRequest(http.MethodGet, "https://redirect.invalid/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect() error = %v", err)
	}
}

func TestStreamClientConnectRejectsHandshakeHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte("FAKEPACA_SECRET LOAD-A"))
	}))
	defer server.Close()
	client := StreamClient{URL: websocketURL(server.URL), HTTP: server.Client()}
	_, err := client.Connect(context.Background(), []string{"LOAD-A"})
	var provider ProviderError
	if !errors.As(err, &provider) || provider.Code != http.StatusUnauthorized || strings.Contains(err.Error(), "FAKEPACA_SECRET") {
		t.Fatalf("Connect() error = %v", err)
	}
}

func TestStreamClientConnectRejectsDialFailure(t *testing.T) {
	client := StreamClient{URL: "://private"}
	_, err := client.Connect(context.Background(), []string{"LOAD-A"})
	if err == nil || err.Error() != "connect to market-data provider" || strings.Contains(err.Error(), "private") {
		t.Fatalf("Connect() error = %v", err)
	}
}

func TestStreamClientConnectReportsAuthenticationAndSubscriptionWriteFailures(t *testing.T) {
	tests := []struct {
		name      string
		messages  []string
		failWrite int
		want      string
	}{
		{name: "authenticate", messages: []string{`[{"T":"success","msg":"connected"}]`}, failWrite: 1, want: "authenticate with market-data provider"},
		{name: "subscribe", messages: []string{
			`[{"T":"success","msg":"connected"}]`, `[{"T":"success","msg":"authenticated"}]`,
		}, failWrite: 2, want: "subscribe to market-data provider"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &failingWebsocketBody{reader: strings.NewReader(strings.Join(test.messages, "")), failWrite: test.failWrite}
			client := StreamClient{
				URL: "ws://localhost/private",
				HTTP: &http.Client{Transport: testRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					key := request.Header.Get("Sec-WebSocket-Key")
					digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")) // #nosec G401 -- mandated by RFC 6455.
					return &http.Response{
						StatusCode: http.StatusSwitchingProtocols,
						Header: http.Header{
							"Connection":           []string{"Upgrade"},
							"Upgrade":              []string{"websocket"},
							"Sec-Websocket-Accept": []string{base64.StdEncoding.EncodeToString(digest[:])},
						},
						Body: body,
					}, nil
				})},
			}
			for index, message := range test.messages {
				test.messages[index] = string(serverTextFrame([]byte(message)))
			}
			body.reader = strings.NewReader(strings.Join(test.messages, ""))
			_, err := client.Connect(context.Background(), []string{"LOAD-A"})
			if err == nil || err.Error() != test.want {
				t.Fatalf("Connect() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestStreamClientConnectRejectsControlAndSubscriptionMessages(t *testing.T) {
	tests := []struct {
		name   string
		stage  string
		value  any
		want   string
		status int
	}{
		{name: "connected malformed", stage: "connected", value: map[string]any{"T": "success"}, want: "invalid control message"},
		{name: "connected multiple", stage: "connected", value: []map[string]any{{"T": "success", "msg": "connected"}, {"T": "success"}}, want: "invalid control message"},
		{name: "connected provider", stage: "connected", value: []map[string]any{{"T": "error", "code": 403}}, status: 403},
		{name: "connected unexpected", stage: "connected", value: []map[string]any{{"T": "success", "msg": "private"}}, want: "unexpected control message"},
		{name: "authenticated invalid", stage: "authenticated", value: []map[string]any{}, want: "invalid control message"},
		{name: "authenticated provider", stage: "authenticated", value: []map[string]any{{"T": "error", "code": 401}}, status: 401},
		{name: "authenticated unexpected", stage: "authenticated", value: []map[string]any{{"T": "success", "msg": "private"}}, want: "unexpected control message"},
		{name: "subscription invalid", stage: "subscription", value: []map[string]any{}, want: "did not confirm"},
		{name: "subscription provider", stage: "subscription", value: []map[string]any{{"T": "error", "code": 409}}, status: 409},
		{name: "subscription type", stage: "subscription", value: []map[string]any{{"T": "success", "bars": []string{"LOAD-A"}, "updatedBars": []string{"LOAD-A"}}}, want: "unexpected subscription"},
		{name: "subscription bars", stage: "subscription", value: []map[string]any{{"T": "subscription", "bars": []string{"LOAD-B"}, "updatedBars": []string{"LOAD-A"}}}, want: "unexpected subscription"},
		{name: "subscription updated", stage: "subscription", value: []map[string]any{{"T": "subscription", "bars": []string{"LOAD-A"}, "updatedBars": []string{"LOAD-B"}}}, want: "unexpected subscription"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newWebsocketTestServer(t, func(ctx context.Context, connection *websocket.Conn) {
				connected := any([]map[string]any{{"T": "success", "msg": "connected"}})
				if test.stage == "connected" {
					connected = test.value
				}
				writeTestWSJSON(t, ctx, connection, connected)
				if test.stage == "connected" {
					return
				}
				var auth any
				readTestWSJSON(t, ctx, connection, &auth)
				authenticated := any([]map[string]any{{"T": "success", "msg": "authenticated"}})
				if test.stage == "authenticated" {
					authenticated = test.value
				}
				writeTestWSJSON(t, ctx, connection, authenticated)
				if test.stage == "authenticated" {
					return
				}
				var subscription any
				readTestWSJSON(t, ctx, connection, &subscription)
				writeTestWSJSON(t, ctx, connection, test.value)
			})
			defer server.Close()
			client := StreamClient{URL: websocketURL(server.URL), KeyID: "FAKEPACA_KEY", SecretKey: "FAKEPACA_SECRET", HTTP: server.Client()}
			_, err := client.Connect(context.Background(), []string{"LOAD-A"})
			if err == nil {
				t.Fatal("Connect() succeeded")
			}
			if test.status != 0 {
				var provider ProviderError
				if !errors.As(err, &provider) || provider.Code != test.status {
					t.Fatalf("Connect() error = %#v, want provider %d", err, test.status)
				}
			} else if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Connect() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestWebsocketSessionReadValidationDisconnectAndCancellation(t *testing.T) {
	tests := []struct {
		name  string
		write func(context.Context, *websocket.Conn)
		want  string
	}{
		{name: "malformed", write: func(ctx context.Context, connection *websocket.Conn) {
			_ = connection.Write(ctx, websocket.MessageText, []byte(`private`))
		}, want: "invalid message batch"},
		{name: "invalid bar", write: func(ctx context.Context, connection *websocket.Conn) {
			writeTestWSJSON(t, ctx, connection, []map[string]any{{"T": "b", "S": "LOAD-A", "c": 0, "n": 1, "t": "2026-08-03T12:00:00Z"}})
		}, want: "invalid positive close"},
		{name: "unsupported", write: func(ctx context.Context, connection *websocket.Conn) {
			writeTestWSJSON(t, ctx, connection, []map[string]any{{"T": "private"}})
		}, want: "unsupported message type"},
		{name: "disconnect", write: func(context.Context, *websocket.Conn) {}, want: "stream disconnected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newWebsocketTestServer(t, test.write)
			defer server.Close()
			connection, _, err := websocket.Dial(context.Background(), websocketURL(server.URL), &websocket.DialOptions{HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			session := &websocketSession{connection: connection}
			_, err = session.Read(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Read() error = %v, want substring %q", err, test.want)
			}
			session.Close()
		})
	}

	server := newWebsocketTestServer(t, func(ctx context.Context, connection *websocket.Conn) {
		<-ctx.Done()
	})
	defer server.Close()
	connection, _, err := websocket.Dial(context.Background(), websocketURL(server.URL), &websocket.DialOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	session := &websocketSession{connection: connection}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = session.Read(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Read() cancellation error = %v", err)
	}
	session.Close()
}

func TestSameStrings(t *testing.T) {
	left := []string{"LOAD-B", "LOAD-A"}
	right := []string{"LOAD-A", "LOAD-B"}
	if !sameStrings(left, right) || !reflect.DeepEqual(left, []string{"LOAD-B", "LOAD-A"}) || !reflect.DeepEqual(right, []string{"LOAD-A", "LOAD-B"}) {
		t.Fatal("sameStrings did not compare unordered copies")
	}
	if sameStrings([]string{"LOAD-A"}, right) || sameStrings([]string{"LOAD-A", "LOAD-B"}, []string{"LOAD-A", "LOAD-C"}) {
		t.Fatal("sameStrings accepted unequal values")
	}
}

func newWebsocketTestServer(t *testing.T, serve func(context.Context, *websocket.Conn)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(response, request, nil)
		if err != nil {
			t.Errorf("websocket.Accept() error = %v", err)
			return
		}
		defer connection.CloseNow()
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		serve(ctx, connection)
	}))
}

func websocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func writeTestWSJSON(t *testing.T, ctx context.Context, connection *websocket.Conn, value any) {
	t.Helper()
	if err := wsjson.Write(ctx, connection, value); err != nil {
		t.Errorf("wsjson.Write() error = %v", err)
	}
}

func readTestWSJSON(t *testing.T, ctx context.Context, connection *websocket.Conn, value any) {
	t.Helper()
	if err := wsjson.Read(ctx, connection, value); err != nil {
		t.Errorf("wsjson.Read() error = %v", err)
	}
}

type failingWebsocketBody struct {
	reader    io.Reader
	writes    int
	failWrite int
}

func (body *failingWebsocketBody) Read(data []byte) (int, error) { return body.reader.Read(data) }

func (body *failingWebsocketBody) Write(data []byte) (int, error) {
	body.writes++
	if body.writes == body.failWrite {
		return 0, errors.New("FAKEPACA_SECRET write")
	}
	return len(data), nil
}

func (body *failingWebsocketBody) Close() error { return nil }

func serverTextFrame(payload []byte) []byte {
	if len(payload) >= 126 {
		panic("test WebSocket payload is too large")
	}
	return append([]byte{0x81, byte(len(payload))}, payload...)
}
