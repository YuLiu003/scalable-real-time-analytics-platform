package alpaca

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

const (
	defaultStreamURL        = "wss://stream.data.alpaca.markets/v2/iex"
	defaultHistoryURL       = "https://data.alpaca.markets/v2/stocks/bars"
	defaultFeed             = "iex"
	defaultSource           = "alpaca-iex"
	defaultCheckpointFile   = "/state/checkpoint.json"
	defaultHTTPAddress      = ":8082"
	defaultTopic            = "market.prices"
	defaultBackfillLookback = 7 * 24 * time.Hour
	defaultReplayOverlap    = 2 * time.Minute
	defaultBufferSize       = 2048
	maximumWatchlistSize    = 30
)

// Config contains runtime-only provider settings. Callers must not log this
// value because it includes credentials and the private watchlist.
type Config struct {
	KeyID            string
	SecretKey        string
	Watchlist        []string
	TenantID         string
	StreamURL        string
	HistoryURL       string
	Feed             string
	Source           string
	CheckpointFile   string
	HTTPAddress      string
	Topic            string
	BackfillLookback time.Duration
	ReplayOverlap    time.Duration
	LiveBufferSize   int
	ReconnectMinimum time.Duration
	ReconnectMaximum time.Duration
}

// LoadConfigFromEnvironment validates configuration without including private
// values in returned errors.
func LoadConfigFromEnvironment() (Config, error) {
	config := Config{
		KeyID:            os.Getenv("APCA_API_KEY_ID"),
		SecretKey:        os.Getenv("APCA_API_SECRET_KEY"),
		TenantID:         os.Getenv("MARKET_TENANT_ID"),
		StreamURL:        envOrDefault("ALPACA_STREAM_URL", defaultStreamURL),
		HistoryURL:       envOrDefault("ALPACA_HISTORY_URL", defaultHistoryURL),
		Feed:             envOrDefault("ALPACA_FEED", defaultFeed),
		Source:           envOrDefault("ALPACA_SOURCE", defaultSource),
		CheckpointFile:   envOrDefault("ALPACA_CHECKPOINT_FILE", defaultCheckpointFile),
		HTTPAddress:      envOrDefault("HTTP_ADDRESS", defaultHTTPAddress),
		Topic:            envOrDefault("KAFKA_TOPIC", defaultTopic),
		BackfillLookback: defaultBackfillLookback,
		ReplayOverlap:    defaultReplayOverlap,
		LiveBufferSize:   defaultBufferSize,
		ReconnectMinimum: time.Second,
		ReconnectMaximum: 30 * time.Second,
	}
	watchlist, err := parseWatchlist(os.Getenv("MARKET_WATCHLIST"))
	if err != nil {
		return Config{}, err
	}
	config.Watchlist = watchlist
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if !validSecret(c.KeyID) {
		return errors.New("APCA_API_KEY_ID is required and must contain no whitespace or control characters")
	}
	if !validSecret(c.SecretKey) {
		return errors.New("APCA_API_SECRET_KEY is required and must contain no whitespace or control characters")
	}
	if len(c.Watchlist) == 0 || len(c.Watchlist) > maximumWatchlistSize {
		return fmt.Errorf("MARKET_WATCHLIST must contain between 1 and %d unique instruments", maximumWatchlistSize)
	}
	for _, instrument := range c.Watchlist {
		if !event.ValidInstrument(instrument) {
			return errors.New("MARKET_WATCHLIST contains a non-canonical instrument")
		}
	}
	if !sort.StringsAreSorted(c.Watchlist) || hasDuplicate(c.Watchlist) {
		return errors.New("MARKET_WATCHLIST must be sorted and unique after parsing")
	}
	if !event.ValidTenantID(c.TenantID) {
		return errors.New("MARKET_TENANT_ID must be a canonical private identifier")
	}
	if !event.ValidSource(c.Source) {
		return errors.New("ALPACA_SOURCE must be a canonical source slug")
	}
	if c.Feed != "iex" && c.Feed != "sip" {
		return errors.New("ALPACA_FEED must be iex or sip")
	}
	streamEndpoint, err := validateEndpoint(
		c.StreamURL,
		"ALPACA_STREAM_URL",
		"wss",
		"ws",
		map[string]struct{}{
			"stream.data.alpaca.markets":         {},
			"stream.data.sandbox.alpaca.markets": {},
		},
	)
	if err != nil {
		return err
	}
	if !isLoopback(streamEndpoint.Hostname()) && streamEndpoint.Path != "/v2/"+c.Feed {
		return errors.New("ALPACA_STREAM_URL path must match ALPACA_FEED")
	}
	historyEndpoint, err := validateEndpoint(
		c.HistoryURL,
		"ALPACA_HISTORY_URL",
		"https",
		"http",
		map[string]struct{}{"data.alpaca.markets": {}},
	)
	if err != nil {
		return err
	}
	if !isLoopback(historyEndpoint.Hostname()) && historyEndpoint.Path != "/v2/stocks/bars" {
		return errors.New("ALPACA_HISTORY_URL must use the stock bars endpoint")
	}
	if !filepath.IsAbs(c.CheckpointFile) || filepath.Clean(c.CheckpointFile) != c.CheckpointFile {
		return errors.New("ALPACA_CHECKPOINT_FILE must be a clean absolute path")
	}
	if _, _, err := net.SplitHostPort(c.HTTPAddress); err != nil {
		return errors.New("HTTP_ADDRESS must be a host:port listener")
	}
	if strings.TrimSpace(c.Topic) == "" || strings.ContainsAny(c.Topic, " \t\r\n") {
		return errors.New("KAFKA_TOPIC must be a non-empty topic name without whitespace")
	}
	if c.BackfillLookback <= 0 || c.ReplayOverlap < 0 || c.ReplayOverlap >= c.BackfillLookback {
		return errors.New("backfill and replay windows are invalid")
	}
	if c.LiveBufferSize < 1 {
		return errors.New("live buffer size must be positive")
	}
	if c.ReconnectMinimum <= 0 || c.ReconnectMaximum < c.ReconnectMinimum {
		return errors.New("reconnect bounds are invalid")
	}
	return nil
}

func parseWatchlist(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("MARKET_WATCHLIST is required")
	}
	items := strings.Split(value, ",")
	watchlist := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		instrument := strings.TrimSpace(item)
		if !event.ValidInstrument(instrument) {
			return nil, errors.New("MARKET_WATCHLIST contains a non-canonical instrument")
		}
		if _, exists := seen[instrument]; exists {
			return nil, errors.New("MARKET_WATCHLIST contains a duplicate instrument")
		}
		seen[instrument] = struct{}{}
		watchlist = append(watchlist, instrument)
	}
	sort.Strings(watchlist)
	return watchlist, nil
}

func validSecret(value string) bool {
	return value != "" && len(value) <= 512 && strings.IndexFunc(value, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) == -1
}

func hasDuplicate(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return true
		}
	}
	return false
}

func validateEndpoint(raw, name, secureScheme, loopbackScheme string, allowedHosts map[string]struct{}) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must be an absolute endpoint without credentials, query, or fragment", name)
	}
	host := parsed.Hostname()
	if parsed.Scheme == secureScheme {
		if _, allowed := allowedHosts[host]; !allowed {
			return nil, fmt.Errorf("%s host is not an approved Alpaca endpoint", name)
		}
		return parsed, nil
	}
	if parsed.Scheme != loopbackScheme || !isLoopback(host) {
		return nil, fmt.Errorf("%s must use %s, except loopback tests may use %s", name, secureScheme, loopbackScheme)
	}
	return parsed, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
