package alpaca

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

func TestNewRunnerDefaultsAndWaitContext(t *testing.T) {
	config := validTestConfig()
	runner := NewRunner(config, &fakeStream{}, &fakeHistory{}, &fakePublisher{}, &fakeCheckpoints{}, NewStatus())
	if runner.Now == nil || runner.Wait == nil || runner.Jitter == nil || !reflect.DeepEqual(runner.Config, config) {
		t.Fatalf("NewRunner() = %#v", runner)
	}
	for index := 0; index < 20; index++ {
		if got := runner.Jitter(5 * time.Millisecond); got < 0 || got > 5*time.Millisecond {
			t.Fatalf("Jitter() = %v", got)
		}
	}
	if err := waitContext(context.Background(), 0); err != nil {
		t.Fatalf("waitContext(0) error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitContext(canceled) error = %v", err)
	}
}

func TestRunnerRunReturnsCheckpointLoadError(t *testing.T) {
	want := errors.New("checkpoint failure")
	runner := newTestRunner()
	runner.Checkpoints = &fakeCheckpoints{loadErr: want}
	if err := runner.Run(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunPrunesInactiveCheckpointCursors(t *testing.T) {
	now := testHistoryStart()
	t.Run("persists selected cursors", func(t *testing.T) {
		checkpoints := &fakeCheckpoints{load: map[string]Cursor{
			"LOAD-A": {Timestamp: now},
			"OLD-A":  {Timestamp: now.Add(-time.Hour)},
		}}
		runner := newTestRunner()
		runner.Checkpoints = checkpoints
		runner.Stream = &fakeStream{connectErr: providerError(401)}
		if err := runner.Run(context.Background()); err == nil {
			t.Fatal("Run() accepted a permanent provider failure")
		}
		if len(checkpoints.saves) != 1 || len(checkpoints.saves[0]) != 1 ||
			!checkpoints.saves[0]["LOAD-A"].Timestamp.Equal(now) {
			t.Fatalf("pruned checkpoint saves = %#v", checkpoints.saves)
		}
	})

	t.Run("returns pruning save failure", func(t *testing.T) {
		want := errors.New("checkpoint save failure")
		stream := &fakeStream{}
		runner := newTestRunner()
		runner.Stream = stream
		runner.Checkpoints = &fakeCheckpoints{
			load:    map[string]Cursor{"OLD-A": {Timestamp: now}},
			saveErr: want,
		}
		if err := runner.Run(context.Background()); !errors.Is(err, want) {
			t.Fatalf("Run() error = %v", err)
		}
		if stream.connectCalls != 0 {
			t.Fatalf("stream connected %d times before checkpoint pruning", stream.connectCalls)
		}
	})
}

func TestRunnerRunStopsOnPermanentProviderError(t *testing.T) {
	runner := newTestRunner()
	runner.Stream = &fakeStream{connectErr: providerError(401)}
	err := runner.Run(context.Background())
	var provider ProviderError
	if !errors.As(err, &provider) || provider.Code != 401 {
		t.Fatalf("Run() error = %#v", err)
	}
	if got := runner.Status.metrics(); !containsAll(got, "market_feed_failures_total 1", "market_feed_reconnects_total 0") {
		t.Fatalf("metrics = %q", got)
	}
}

func TestRunnerRunReconnects429WithBoundedBackoff(t *testing.T) {
	runner := newTestRunner()
	stream := &fakeStream{connectErr: providerError(429)}
	runner.Stream = stream
	runner.Jitter = func(duration time.Duration) time.Duration { return duration }
	wantWaitError := errors.New("wait failure")
	var waits []time.Duration
	runner.Wait = func(_ context.Context, duration time.Duration) error {
		waits = append(waits, duration)
		if len(waits) == 4 {
			return wantWaitError
		}
		return nil
	}
	err := runner.Run(context.Background())
	if !errors.Is(err, wantWaitError) {
		t.Fatalf("Run() error = %v", err)
	}
	wantWaits := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	if !reflect.DeepEqual(waits, wantWaits) || stream.connectCalls != 4 {
		t.Fatalf("waits = %v, connects = %d", waits, stream.connectCalls)
	}
	if got := runner.Status.metrics(); !containsAll(got, "market_feed_failures_total 4", "market_feed_reconnects_total 4") {
		t.Fatalf("metrics = %q", got)
	}
}

func TestRunnerRunTreatsContextCancellationAsCleanShutdown(t *testing.T) {
	t.Run("before session", func(t *testing.T) {
		runner := newTestRunner()
		runner.Stream = &fakeStream{connectErr: errors.New("disconnect")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := runner.Run(ctx); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
	t.Run("during reconnect wait", func(t *testing.T) {
		runner := newTestRunner()
		runner.Stream = &fakeStream{connectErr: errors.New("disconnect")}
		ctx, cancel := context.WithCancel(context.Background())
		runner.Wait = func(context.Context, time.Duration) error {
			cancel()
			return context.Canceled
		}
		if err := runner.Run(ctx); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
}

func TestRunnerRunSessionConnectAndHistoryFailures(t *testing.T) {
	t.Run("connect", func(t *testing.T) {
		runner := newTestRunner()
		want := errors.New("connect failure")
		runner.Stream = &fakeStream{connectErr: want}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
	})
	t.Run("history", func(t *testing.T) {
		runner := newTestRunner()
		session := newFakeSession(sessionResult{err: errors.New("stream disconnected")})
		runner.Stream = &fakeStream{session: session}
		want := errors.New("history failure")
		runner.History = &fakeHistory{err: want}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
		if session.closeCalls != 1 {
			t.Fatalf("session close calls = %d", session.closeCalls)
		}
		if got := runner.Status.metrics(); !containsAll(got, "market_feed_connected 0", "market_feed_ready 0", "market_feed_connections_total 1", "market_feed_backfill_requests_total 1") {
			t.Fatalf("metrics = %q", got)
		}
	})
}

func TestRunnerRunSessionBackfillAndLiveKafkaAckSemantics(t *testing.T) {
	start := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	backfill := validTestBar("LOAD-A", start.Add(-time.Minute))
	olderBackfill := validTestBar("LOAD-B", start.Add(-2*time.Minute))
	live := validTestBar("LOAD-B", start)
	runner := newTestRunner()
	runner.Now = func() time.Time { return start }
	runner.History = &fakeHistory{bars: []Bar{olderBackfill, backfill}}
	session := newFakeSession(sessionResult{bars: []Bar{live}})
	runner.Stream = &fakeStream{session: session}
	ctx, cancel := context.WithCancel(context.Background())
	publisher := &fakePublisher{publish: func(_ context.Context, envelope event.Envelope) error {
		if envelope.PartitionKey == "LOAD-B" && envelope.OccurredAt == start.Add(time.Minute).Format(time.RFC3339Nano) {
			cancel()
		}
		return nil
	}}
	runner.Publisher = publisher
	checkpoints := &fakeCheckpoints{}
	runner.Checkpoints = checkpoints
	if err := runner.runSession(ctx, map[string]Cursor{}); err != nil {
		t.Fatalf("runSession() error = %v", err)
	}
	if publisher.calls != 3 || len(checkpoints.saves) != 2 || session.closeCalls != 1 {
		t.Fatalf("publish calls = %d, saves = %d, close calls = %d", publisher.calls, len(checkpoints.saves), session.closeCalls)
	}
	if checkpoints.saves[0]["LOAD-A"].Timestamp != backfill.Timestamp ||
		checkpoints.saves[0]["LOAD-B"].Timestamp != olderBackfill.Timestamp ||
		checkpoints.saves[1]["LOAD-B"].Timestamp != live.Timestamp {
		t.Fatalf("saved checkpoints = %#v", checkpoints.saves)
	}
	if got := runner.Status.metrics(); !containsAll(got, "market_feed_published_events_total 3", "market_feed_ready 0", "market_feed_connected 0") {
		t.Fatalf("metrics = %q", got)
	}
}

func TestRunnerRunSessionReturnsStreamAndPublishFailures(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		runner := newTestRunner()
		want := errors.New("stream failure")
		runner.Stream = &fakeStream{session: newFakeSession(sessionResult{err: want})}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
	})
	t.Run("backfill publish", func(t *testing.T) {
		runner := newTestRunner()
		runner.Stream = &fakeStream{session: newFakeSession(sessionResult{err: errors.New("stream failure")})}
		runner.History = &fakeHistory{bars: []Bar{validTestBar("LOAD-A", testHistoryStart())}}
		want := errors.New("kafka ack failure")
		runner.Publisher = &fakePublisher{err: want}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
	})
	t.Run("backfill checkpoint", func(t *testing.T) {
		runner := newTestRunner()
		runner.Stream = &fakeStream{session: newFakeSession(sessionResult{err: errors.New("stream failure")})}
		runner.History = &fakeHistory{bars: []Bar{validTestBar("LOAD-A", testHistoryStart())}}
		want := errors.New("checkpoint failure")
		runner.Checkpoints = &fakeCheckpoints{saveErr: want}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
	})
	t.Run("live publish", func(t *testing.T) {
		runner := newTestRunner()
		runner.Stream = &fakeStream{session: newFakeSession(sessionResult{bars: []Bar{validTestBar("LOAD-A", testHistoryStart())}})}
		want := errors.New("kafka ack failure")
		runner.Publisher = &fakePublisher{err: want}
		if err := runner.runSession(context.Background(), map[string]Cursor{}); !errors.Is(err, want) {
			t.Fatalf("runSession() error = %v", err)
		}
	})
}

func TestReadLiveDeliversBarsAndReportsBufferOverflow(t *testing.T) {
	first := validTestBar("LOAD-A", testHistoryStart())
	second := validTestBar("LOAD-B", testHistoryStart())
	session := newFakeSession(sessionResult{bars: []Bar{first, second}})
	output := make(chan Bar, 1)
	failures := make(chan error, 1)
	readLive(context.Background(), session, output, failures)
	if got := <-output; !reflect.DeepEqual(got, first) {
		t.Fatalf("output = %#v, want %#v", got, first)
	}
	if err := <-failures; !errors.Is(err, ErrLiveBufferFull) {
		t.Fatalf("failure = %v", err)
	}
}

func TestReadLiveReturnsReadErrorAndCancellation(t *testing.T) {
	want := errors.New("read failure")
	failures := make(chan error, 1)
	readLive(context.Background(), newFakeSession(sessionResult{err: want}), make(chan Bar, 1), failures)
	if err := <-failures; !errors.Is(err, want) {
		t.Fatalf("failure = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failures = make(chan error, 1)
	readLive(ctx, immediateSession{bars: []Bar{validTestBar("LOAD-A", testHistoryStart())}}, make(chan Bar), failures)
	select {
	case err := <-failures:
		t.Fatalf("unexpected cancellation failure = %v", err)
	default:
	}
	readLive(ctx, immediateSession{err: context.Canceled}, make(chan Bar), failures)
	select {
	case err := <-failures:
		t.Fatalf("unexpected canceled read failure = %v", err)
	default:
	}
}

func TestRunnerBackfillStartUsesOldestOverlappedCursor(t *testing.T) {
	runner := newTestRunner()
	end := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	if got, want := runner.backfillStart(map[string]Cursor{}, end), end.Add(-24*time.Hour); !got.Equal(want) {
		t.Fatalf("backfillStart(empty) = %s, want %s", got, want)
	}
	cursors := map[string]Cursor{
		"LOAD-A": {Timestamp: end.Add(-48 * time.Hour)},
		"LOAD-B": {Timestamp: end.Add(-time.Hour)},
	}
	if got, want := runner.backfillStart(cursors, end), end.Add(-48*time.Hour-time.Minute); !got.Equal(want) {
		t.Fatalf("backfillStart(cursors) = %s, want %s", got, want)
	}
}

func TestRunnerPublishAdvancesCheckpointOnlyAfterKafkaAck(t *testing.T) {
	bar := validTestBar("LOAD-A", testHistoryStart())
	t.Run("invalid envelope", func(t *testing.T) {
		runner := newTestRunner()
		invalid := bar
		invalid.Close = "0"
		if err := runner.publish(context.Background(), map[string]Cursor{}, invalid, true); err == nil {
			t.Fatal("publish() succeeded")
		}
	})
	t.Run("kafka failure", func(t *testing.T) {
		runner := newTestRunner()
		want := errors.New("kafka ack failure")
		publisher := &fakePublisher{err: want}
		checkpoints := &fakeCheckpoints{}
		runner.Publisher = publisher
		runner.Checkpoints = checkpoints
		cursors := map[string]Cursor{}
		if err := runner.publish(context.Background(), cursors, bar, true); !errors.Is(err, want) {
			t.Fatalf("publish() error = %v", err)
		}
		if len(cursors) != 0 || len(checkpoints.saves) != 0 || publisher.calls != 1 {
			t.Fatalf("checkpoint advanced before ack: cursors=%v saves=%v calls=%d", cursors, checkpoints.saves, publisher.calls)
		}
	})
	t.Run("checkpoint failure", func(t *testing.T) {
		runner := newTestRunner()
		want := errors.New("checkpoint failure")
		checkpoints := &fakeCheckpoints{saveErr: want}
		runner.Checkpoints = checkpoints
		cursors := map[string]Cursor{}
		if err := runner.publish(context.Background(), cursors, bar, true); !errors.Is(err, want) {
			t.Fatalf("publish() error = %v", err)
		}
		if len(cursors) != 0 || len(checkpoints.saves) != 1 {
			t.Fatalf("failed checkpoint mutated cursor: cursors=%v saves=%v", cursors, checkpoints.saves)
		}
	})
	t.Run("advance and stale replay", func(t *testing.T) {
		runner := newTestRunner()
		checkpoints := &fakeCheckpoints{}
		runner.Checkpoints = checkpoints
		prior := bar.Timestamp.Add(-time.Minute)
		cursors := map[string]Cursor{"LOAD-A": {Timestamp: prior}, "LOAD-B": {Timestamp: prior}}
		if err := runner.publish(context.Background(), cursors, bar, true); err != nil {
			t.Fatalf("publish() error = %v", err)
		}
		if len(checkpoints.saves) != 1 || !cursors["LOAD-A"].Timestamp.Equal(bar.Timestamp) || !checkpoints.saves[0]["LOAD-B"].Timestamp.Equal(prior) {
			t.Fatalf("advance state: cursors=%v saves=%v", cursors, checkpoints.saves)
		}
		stale := bar
		stale.Timestamp = prior
		if err := runner.publish(context.Background(), cursors, stale, true); err != nil {
			t.Fatalf("stale publish() error = %v", err)
		}
		if len(checkpoints.saves) != 1 {
			t.Fatalf("stale replay saved checkpoint: %v", checkpoints.saves)
		}
		if got := runner.Status.metrics(); !containsAll(got, "market_feed_published_events_total 2") {
			t.Fatalf("metrics = %q", got)
		}
	})
	cloned := cloneCursors(map[string]Cursor{"LOAD-A": {Timestamp: bar.Timestamp}})
	cloned["LOAD-B"] = Cursor{Timestamp: bar.Timestamp}
	if len(cloned) != 2 {
		t.Fatalf("cloneCursors() = %v", cloned)
	}
}

func newTestRunner() *Runner {
	config := validTestConfig()
	return &Runner{
		Config: config, Stream: &fakeStream{}, History: &fakeHistory{}, Publisher: &fakePublisher{}, Checkpoints: &fakeCheckpoints{},
		Status: NewStatus(), Now: func() time.Time { return testHistoryStart() },
		Wait: func(context.Context, time.Duration) error { return nil }, Jitter: func(duration time.Duration) time.Duration { return duration },
	}
}

type fakeCheckpoints struct {
	load    map[string]Cursor
	loadErr error
	saveErr error
	saves   []map[string]Cursor
}

func (fake *fakeCheckpoints) Load() (map[string]Cursor, error) {
	if fake.load == nil {
		return map[string]Cursor{}, fake.loadErr
	}
	return cloneCursors(fake.load), fake.loadErr
}

func (fake *fakeCheckpoints) Save(cursors map[string]Cursor) error {
	fake.saves = append(fake.saves, cloneCursors(cursors))
	return fake.saveErr
}

type fakeStream struct {
	session      Session
	connectErr   error
	connectCalls int
}

func (fake *fakeStream) Connect(_ context.Context, watchlist []string) (Session, error) {
	fake.connectCalls++
	if !reflect.DeepEqual(watchlist, []string{"LOAD-A", "LOAD-B"}) {
		return nil, errors.New("unexpected watchlist")
	}
	return fake.session, fake.connectErr
}

type sessionResult struct {
	bars []Bar
	err  error
}

type immediateSession struct {
	bars []Bar
	err  error
}

func (session immediateSession) Read(context.Context) ([]Bar, error) {
	return session.bars, session.err
}
func (immediateSession) Close() {}

type fakeSession struct {
	results    chan sessionResult
	mu         sync.Mutex
	closeCalls int
}

func newFakeSession(results ...sessionResult) *fakeSession {
	channel := make(chan sessionResult, len(results))
	for _, result := range results {
		channel <- result
	}
	return &fakeSession{results: channel}
}

func (fake *fakeSession) Read(ctx context.Context) ([]Bar, error) {
	select {
	case result := <-fake.results:
		return result.bars, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (fake *fakeSession) Close() {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.closeCalls++
}

type fakeHistory struct {
	bars []Bar
	err  error
}

func (fake *fakeHistory) Bars(context.Context, []string, time.Time, time.Time) ([]Bar, error) {
	return fake.bars, fake.err
}

type fakePublisher struct {
	mu      sync.Mutex
	calls   int
	err     error
	publish func(context.Context, event.Envelope) error
}

func (fake *fakePublisher) Publish(ctx context.Context, envelope event.Envelope) error {
	fake.mu.Lock()
	fake.calls++
	fake.mu.Unlock()
	if fake.publish != nil {
		return fake.publish(ctx, envelope)
	}
	return fake.err
}

func containsAll(value string, substrings ...string) bool {
	for _, substring := range substrings {
		if !strings.Contains(value, substring) {
			return false
		}
	}
	return true
}
