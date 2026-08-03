package alpaca

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	handshakeTimeout = 10 * time.Second
	maximumMessage   = 1 << 20
)

type Session interface {
	Read(context.Context) ([]Bar, error)
	Close()
}

type Stream interface {
	Connect(context.Context, []string) (Session, error)
}

type StreamClient struct {
	URL       string
	KeyID     string
	SecretKey string
	HTTP      *http.Client
}

// NewProviderHTTPClient rejects redirects so credentials never cross the
// endpoint allowlist validated at startup.
func NewProviderHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (client StreamClient) Connect(ctx context.Context, watchlist []string) (Session, error) {
	handshake, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	connection, response, err := websocket.Dial(handshake, client.URL, &websocket.DialOptions{
		HTTPClient: client.HTTP,
	})
	if err != nil {
		if response != nil {
			return nil, providerError(response.StatusCode)
		}
		return nil, errors.New("connect to market-data provider")
	}
	connection.SetReadLimit(maximumMessage)
	closeOnError := true
	defer func() {
		if closeOnError {
			connection.CloseNow()
		}
	}()
	if err := expectControl(handshake, connection, "success", "connected"); err != nil {
		return nil, err
	}
	if err := wsjson.Write(handshake, connection, map[string]string{
		"action": "auth",
		"key":    client.KeyID,
		"secret": client.SecretKey,
	}); err != nil {
		return nil, errors.New("authenticate with market-data provider")
	}
	if err := expectControl(handshake, connection, "success", "authenticated"); err != nil {
		return nil, err
	}
	if err := wsjson.Write(handshake, connection, map[string]any{
		"action":      "subscribe",
		"bars":        watchlist,
		"updatedBars": watchlist,
	}); err != nil {
		return nil, errors.New("subscribe to market-data provider")
	}
	var subscribed []wireMessage
	if err := wsjson.Read(handshake, connection, &subscribed); err != nil || len(subscribed) != 1 {
		return nil, errors.New("market-data provider did not confirm the subscription")
	}
	message := subscribed[0]
	if message.Type == "error" {
		return nil, providerError(message.Code)
	}
	if message.Type != "subscription" || !sameStrings(message.Bars, watchlist) || !sameStrings(message.Updated, watchlist) {
		return nil, errors.New("market-data provider confirmed an unexpected subscription")
	}
	closeOnError = false
	return &websocketSession{connection: connection}, nil
}

func expectControl(ctx context.Context, connection *websocket.Conn, messageType, messageText string) error {
	var messages []wireMessage
	if err := wsjson.Read(ctx, connection, &messages); err != nil || len(messages) != 1 {
		return errors.New("market-data provider returned an invalid control message")
	}
	message := messages[0]
	if message.Type == "error" {
		return providerError(message.Code)
	}
	if message.Type != messageType || message.Message != messageText {
		return errors.New("market-data provider returned an unexpected control message")
	}
	return nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

type websocketSession struct {
	connection *websocket.Conn
}

func (session *websocketSession) Read(ctx context.Context) ([]Bar, error) {
	_, data, err := session.connection.Read(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("market-data stream disconnected")
	}
	messages, err := decodeWireMessages(data)
	if err != nil {
		return nil, err
	}
	bars := make([]Bar, 0, len(messages))
	for _, message := range messages {
		switch message.Type {
		case "b", "u":
			bar, err := message.bar()
			if err != nil {
				return nil, err
			}
			bars = append(bars, bar)
		case "error":
			return nil, providerError(message.Code)
		case "success", "subscription":
			// Control messages can race with data batches and carry no market data.
		default:
			return nil, errors.New("market-data provider returned an unsupported message type")
		}
	}
	return bars, nil
}

func (session *websocketSession) Close() {
	session.connection.CloseNow()
}
