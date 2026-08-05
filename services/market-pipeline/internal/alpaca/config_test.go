package alpaca

import (
	"strings"
	"testing"
	"time"
)

func validTestConfig() Config {
	return Config{
		KeyID:            "FAKEPACA_KEY",
		SecretKey:        "FAKEPACA_SECRET",
		Watchlist:        []string{"LOAD-A", "LOAD-B"},
		TenantID:         "fakepaca",
		StreamURL:        "wss://stream.data.alpaca.markets/v2/iex",
		HistoryURL:       "https://data.alpaca.markets/v2/stocks/bars",
		Feed:             "iex",
		Source:           "fakepaca-iex",
		CheckpointFile:   "/state/fakepaca.json",
		HTTPAddress:      ":8082",
		Topic:            "market.test",
		BackfillLookback: 24 * time.Hour,
		ReplayOverlap:    time.Minute,
		LiveBufferSize:   4,
		ReconnectMinimum: time.Second,
		ReconnectMaximum: 8 * time.Second,
	}
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	t.Setenv("APCA_API_KEY_ID", "FAKEPACA_KEY")
	t.Setenv("APCA_API_SECRET_KEY", "FAKEPACA_SECRET")
	t.Setenv("MARKET_WATCHLIST", "LOAD-B, LOAD-A")
	t.Setenv("MARKET_TENANT_ID", "fakepaca")

	config, err := LoadConfigFromEnvironment()
	if err != nil {
		t.Fatalf("LoadConfigFromEnvironment() error = %v", err)
	}
	if got, want := strings.Join(config.Watchlist, ","), "LOAD-A,LOAD-B"; got != want {
		t.Fatalf("Watchlist = %q, want %q", got, want)
	}
	if config.StreamURL != defaultStreamURL || config.HistoryURL != defaultHistoryURL || config.Feed != defaultFeed ||
		config.Source != defaultSource || config.CheckpointFile != defaultCheckpointFile || config.HTTPAddress != defaultHTTPAddress ||
		config.Topic != defaultTopic || config.BackfillLookback != defaultBackfillLookback || config.ReplayOverlap != defaultReplayOverlap ||
		config.LiveBufferSize != defaultBufferSize {
		t.Fatalf("defaults not applied: %+v", config)
	}

	t.Setenv("ALPACA_STREAM_URL", "ws://127.0.0.1:3001/stream")
	t.Setenv("ALPACA_HISTORY_URL", "http://localhost:3002/history")
	t.Setenv("ALPACA_FEED", "sip")
	t.Setenv("ALPACA_SOURCE", "fakepaca-sip")
	t.Setenv("ALPACA_CHECKPOINT_FILE", "/tmp/fakepaca-checkpoint.json")
	t.Setenv("HTTP_ADDRESS", "127.0.0.1:8083")
	t.Setenv("KAFKA_TOPIC", "market.test.override")
	config, err = LoadConfigFromEnvironment()
	if err != nil {
		t.Fatalf("LoadConfigFromEnvironment() override error = %v", err)
	}
	if config.Feed != "sip" || config.Source != "fakepaca-sip" || config.Topic != "market.test.override" {
		t.Fatalf("overrides not applied: %+v", config)
	}
}

func TestLoadConfigFromEnvironmentRejectsWatchlistBeforeConfig(t *testing.T) {
	t.Setenv("APCA_API_KEY_ID", "")
	t.Setenv("MARKET_WATCHLIST", "")
	_, err := LoadConfigFromEnvironment()
	if err == nil || err.Error() != "MARKET_WATCHLIST is required" {
		t.Fatalf("error = %v", err)
	}

	t.Setenv("MARKET_WATCHLIST", "LOAD-A")
	_, err = LoadConfigFromEnvironment()
	if err == nil || !strings.Contains(err.Error(), "APCA_API_KEY_ID") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigValidateRejectsEveryInvalidField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "key", mutate: func(c *Config) { c.KeyID = "bad key" }, want: "APCA_API_KEY_ID"},
		{name: "secret", mutate: func(c *Config) { c.SecretKey = "bad\nsecret" }, want: "APCA_API_SECRET_KEY"},
		{name: "empty watchlist", mutate: func(c *Config) { c.Watchlist = nil }, want: "between 1 and 30"},
		{name: "large watchlist", mutate: func(c *Config) {
			c.Watchlist = make([]string, maximumWatchlistSize+1)
		}, want: "between 1 and 30"},
		{name: "instrument", mutate: func(c *Config) { c.Watchlist = []string{"private"} }, want: "non-canonical instrument"},
		{name: "unsorted", mutate: func(c *Config) { c.Watchlist = []string{"LOAD-B", "LOAD-A"} }, want: "sorted and unique"},
		{name: "duplicate", mutate: func(c *Config) { c.Watchlist = []string{"LOAD-A", "LOAD-A"} }, want: "sorted and unique"},
		{name: "tenant", mutate: func(c *Config) { c.TenantID = "FAKEPACA" }, want: "canonical private identifier"},
		{name: "source", mutate: func(c *Config) { c.Source = "FAKEPACA" }, want: "canonical source slug"},
		{name: "feed", mutate: func(c *Config) { c.Feed = "private" }, want: "must be iex or sip"},
		{name: "stream", mutate: func(c *Config) { c.StreamURL = "ws://example.invalid/stream" }, want: "must use wss"},
		{name: "stream host", mutate: func(c *Config) { c.StreamURL = "wss://example.invalid/v2/iex" }, want: "not an approved"},
		{name: "stream path", mutate: func(c *Config) { c.StreamURL = "wss://stream.data.alpaca.markets/v2/sip" }, want: "path must match"},
		{name: "history", mutate: func(c *Config) { c.HistoryURL = "http://example.invalid/history" }, want: "must use https"},
		{name: "history host", mutate: func(c *Config) { c.HistoryURL = "https://example.invalid/v2/stocks/bars" }, want: "not an approved"},
		{name: "history path", mutate: func(c *Config) { c.HistoryURL = "https://data.alpaca.markets/v2/private" }, want: "stock bars endpoint"},
		{name: "relative checkpoint", mutate: func(c *Config) { c.CheckpointFile = "private.json" }, want: "clean absolute path"},
		{name: "unclean checkpoint", mutate: func(c *Config) { c.CheckpointFile = "/state/../private.json" }, want: "clean absolute path"},
		{name: "listener", mutate: func(c *Config) { c.HTTPAddress = "8082" }, want: "host:port listener"},
		{name: "empty topic", mutate: func(c *Config) { c.Topic = "  " }, want: "non-empty topic"},
		{name: "topic whitespace", mutate: func(c *Config) { c.Topic = "market\ttest" }, want: "without whitespace"},
		{name: "lookback", mutate: func(c *Config) { c.BackfillLookback = 0 }, want: "windows are invalid"},
		{name: "negative overlap", mutate: func(c *Config) { c.ReplayOverlap = -time.Second }, want: "windows are invalid"},
		{name: "overlap equals lookback", mutate: func(c *Config) { c.ReplayOverlap = c.BackfillLookback }, want: "windows are invalid"},
		{name: "buffer", mutate: func(c *Config) { c.LiveBufferSize = 0 }, want: "buffer size must be positive"},
		{name: "minimum reconnect", mutate: func(c *Config) { c.ReconnectMinimum = 0 }, want: "reconnect bounds are invalid"},
		{name: "maximum reconnect", mutate: func(c *Config) { c.ReconnectMaximum = c.ReconnectMinimum - 1 }, want: "reconnect bounds are invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validTestConfig()
			test.mutate(&config)
			err := config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.want)
			}
		})
	}

	if err := validTestConfig().Validate(); err != nil {
		t.Fatalf("valid Config.Validate() error = %v", err)
	}
}

func TestParseWatchlistAndSecretHelpers(t *testing.T) {
	for _, value := range []string{"", "  "} {
		if _, err := parseWatchlist(value); err == nil {
			t.Fatalf("parseWatchlist(%q) succeeded", value)
		}
	}
	if _, err := parseWatchlist("LOAD-A,private"); err == nil {
		t.Fatal("parseWatchlist accepted a non-canonical instrument")
	}
	if _, err := parseWatchlist("LOAD-A, LOAD-A"); err == nil {
		t.Fatal("parseWatchlist accepted a duplicate")
	}
	got, err := parseWatchlist("LOAD-B,LOAD-A")
	if err != nil || strings.Join(got, ",") != "LOAD-A,LOAD-B" {
		t.Fatalf("parseWatchlist() = %v, %v", got, err)
	}

	if validSecret("") || validSecret("has space") || validSecret("control\x7f") || validSecret(strings.Repeat("x", 513)) {
		t.Fatal("validSecret accepted invalid input")
	}
	if !validSecret(strings.Repeat("x", 512)) {
		t.Fatal("validSecret rejected its maximum-length input")
	}
	if hasDuplicate([]string{"LOAD-A", "LOAD-B"}) || !hasDuplicate([]string{"LOAD-A", "LOAD-A"}) {
		t.Fatal("hasDuplicate returned an unexpected result")
	}
}

func TestValidateEndpointBoundaries(t *testing.T) {
	valid := []struct {
		raw, secure, loopback string
	}{
		{raw: "wss://stream.invalid/private", secure: "wss", loopback: "ws"},
		{raw: "ws://localhost:1234/private", secure: "wss", loopback: "ws"},
		{raw: "ws://127.0.0.1:1234/private", secure: "wss", loopback: "ws"},
		{raw: "http://[::1]:1234/private", secure: "https", loopback: "http"},
	}
	allowed := map[string]struct{}{"stream.invalid": {}}
	for _, test := range valid {
		parsed, err := validateEndpoint(test.raw, "TEST_URL", test.secure, test.loopback, allowed)
		if err != nil {
			t.Errorf("validateEndpoint(%q) error = %v", test.raw, err)
		} else if parsed.String() != test.raw {
			t.Errorf("validateEndpoint(%q) parsed = %q", test.raw, parsed)
		}
	}
	invalid := []string{
		"%", "wss:///missing-host", "wss://user@stream.invalid/path", "wss://stream.invalid/path?q=private",
		"wss://stream.invalid/path#private", "ws://host.invalid/path", "ws://192.0.2.1/path",
	}
	for _, raw := range invalid {
		if _, err := validateEndpoint(raw, "TEST_URL", "wss", "ws", allowed); err == nil {
			t.Errorf("validateEndpoint(%q) succeeded", raw)
		}
	}
	if !isLoopback("localhost") || !isLoopback("127.0.0.1") || !isLoopback("::1") || isLoopback("192.0.2.1") || isLoopback("host.invalid") {
		t.Fatal("isLoopback returned an unexpected result")
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("FAKEPACA_OPTION", "")
	if got := envOrDefault("FAKEPACA_OPTION", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault empty = %q", got)
	}
	t.Setenv("FAKEPACA_OPTION", "configured")
	if got := envOrDefault("FAKEPACA_OPTION", "fallback"); got != "configured" {
		t.Fatalf("envOrDefault configured = %q", got)
	}
}
