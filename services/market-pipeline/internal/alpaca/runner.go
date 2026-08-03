package alpaca

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

var ErrLiveBufferFull = errors.New("market-data live buffer is full")

type Publisher interface {
	Publish(context.Context, event.Envelope) error
}

type Runner struct {
	Config      Config
	Stream      Stream
	History     History
	Publisher   Publisher
	Checkpoints Checkpoints
	Status      *Status
	Now         func() time.Time
	Wait        func(context.Context, time.Duration) error
	Jitter      func(time.Duration) time.Duration
}

func NewRunner(config Config, stream Stream, history History, publisher Publisher, checkpoints Checkpoints, status *Status) *Runner {
	return &Runner{
		Config:      config,
		Stream:      stream,
		History:     history,
		Publisher:   publisher,
		Checkpoints: checkpoints,
		Status:      status,
		Now:         time.Now,
		Wait:        waitContext,
		Jitter: func(bound time.Duration) time.Duration {
			return time.Duration(rand.Int64N(int64(bound) + 1))
		},
	}
}

func (runner *Runner) Run(ctx context.Context) error {
	cursors, err := runner.Checkpoints.Load()
	if err != nil {
		return err
	}
	backoff := runner.Config.ReconnectMinimum
	for {
		err := runner.runSession(ctx, cursors)
		if ctx.Err() != nil {
			return nil
		}
		runner.Status.RecordFailure()
		if isPermanent(err) {
			return err
		}
		runner.Status.RecordReconnect()
		if err := runner.Wait(ctx, runner.Jitter(backoff)); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if backoff < runner.Config.ReconnectMaximum/2 {
			backoff *= 2
		} else {
			backoff = runner.Config.ReconnectMaximum
		}
	}
}

func (runner *Runner) runSession(ctx context.Context, cursors map[string]Cursor) error {
	session, err := runner.Stream.Connect(ctx, runner.Config.Watchlist)
	if err != nil {
		return err
	}
	defer session.Close()
	runner.Status.SetConnected(true)
	defer runner.Status.SetConnected(false)
	defer runner.Status.SetReady(false)

	live := make(chan Bar, runner.Config.LiveBufferSize)
	streamErrors := make(chan error, 1)
	go readLive(ctx, session, live, streamErrors)

	connectedAt := runner.Now().UTC()
	start := runner.backfillStart(cursors, connectedAt)
	runner.Status.RecordBackfill()
	backfill, err := runner.History.Bars(ctx, runner.Config.Watchlist, start, connectedAt)
	if err != nil {
		return err
	}
	for _, bar := range backfill {
		if err := runner.publish(ctx, cursors, bar, false); err != nil {
			return err
		}
	}
	if len(backfill) > 0 {
		if err := runner.Checkpoints.Save(cloneCursors(cursors)); err != nil {
			return err
		}
	}
	runner.Status.SetReady(true)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-streamErrors:
			return err
		case bar := <-live:
			if err := runner.publish(ctx, cursors, bar, true); err != nil {
				return err
			}
		}
	}
}

func readLive(ctx context.Context, session Session, output chan<- Bar, failures chan<- error) {
	for {
		bars, err := session.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failures <- err
			return
		}
		for _, bar := range bars {
			select {
			case output <- bar:
			case <-ctx.Done():
				return
			default:
				failures <- ErrLiveBufferFull
				return
			}
		}
	}
}

func (runner *Runner) backfillStart(cursors map[string]Cursor, end time.Time) time.Time {
	start := end
	for _, instrument := range runner.Config.Watchlist {
		candidate := end.Add(-runner.Config.BackfillLookback)
		if cursor, ok := cursors[instrument]; ok {
			candidate = cursor.Timestamp.Add(-runner.Config.ReplayOverlap)
		}
		if candidate.Before(start) {
			start = candidate
		}
	}
	return start
}

func (runner *Runner) publish(ctx context.Context, cursors map[string]Cursor, bar Bar, persist bool) error {
	envelope, err := bar.Envelope(runner.Config.Source, runner.Config.TenantID)
	if err != nil {
		return err
	}
	if err := runner.Publisher.Publish(ctx, envelope); err != nil {
		return err
	}
	current, exists := cursors[bar.Instrument]
	if !exists || bar.Timestamp.After(current.Timestamp) {
		next := Cursor{Timestamp: bar.Timestamp.UTC()}
		if persist {
			checkpoint := cloneCursors(cursors)
			checkpoint[bar.Instrument] = next
			if err := runner.Checkpoints.Save(checkpoint); err != nil {
				return err
			}
		}
		cursors[bar.Instrument] = next
	}
	runner.Status.RecordPublished()
	return nil
}

func cloneCursors(cursors map[string]Cursor) map[string]Cursor {
	cloned := make(map[string]Cursor, len(cursors)+1)
	for instrument, cursor := range cursors {
		cloned[instrument] = cursor
	}
	return cloned
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
