package eventbus

import (
	"context"
	"sync"
	"time"
)

// DedupPublisher publishes with a deterministic message id (Publisher does).
type DedupPublisher interface {
	PublishDedup(ctx context.Context, subject string, event Event) error
}

// AsyncPublishConfig bounds AsyncPublisher. Zero fields take the defaults.
type AsyncPublishConfig struct {
	Attempts       int           // total tries per event (default 5)
	BaseDelay      time.Duration // first backoff (default 200ms), doubled per retry
	MaxDelay       time.Duration // backoff cap (default 5s)
	AttemptTimeout time.Duration // per-publish deadline (default 3s)
	MaxInFlight    int           // events retrying at once (default 64)
}

func (c AsyncPublishConfig) withDefaults() AsyncPublishConfig {
	if c.Attempts <= 0 {
		c.Attempts = 5
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = 200 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 5 * time.Second
	}
	if c.AttemptTimeout <= 0 {
		c.AttemptTimeout = 3 * time.Second
	}
	if c.MaxInFlight <= 0 {
		c.MaxInFlight = 64
	}
	return c
}

// Publish outcomes reported to AsyncPublisher's OnResult hook.
const (
	AsyncPublished = "published"
	AsyncFailed    = "failed"  // every attempt failed
	AsyncDropped   = "dropped" // too many events already retrying
)

// AsyncPublisher publishes through JetStream without blocking the caller:
// each event is retried with exponential backoff in its own goroutine, and the
// outcome (published / failed after all attempts / dropped when saturated) is
// reported to OnResult, so a lost event is always counted, never silent.
type AsyncPublisher struct {
	pub    DedupPublisher
	cfg    AsyncPublishConfig
	sem    chan struct{}
	wg     sync.WaitGroup
	sleep  func(time.Duration)
	result func(subject, outcome string, err error)
}

// NewAsyncPublisher wraps pub. onResult may be nil.
func NewAsyncPublisher(pub DedupPublisher, cfg AsyncPublishConfig, onResult func(subject, outcome string, err error)) *AsyncPublisher {
	cfg = cfg.withDefaults()
	if onResult == nil {
		onResult = func(string, string, error) {}
	}
	return &AsyncPublisher{pub: pub, cfg: cfg, sem: make(chan struct{}, cfg.MaxInFlight), sleep: time.Sleep, result: onResult}
}

// SetSleep replaces the backoff sleeper (tests).
func (a *AsyncPublisher) SetSleep(fn func(time.Duration)) { a.sleep = fn }

// Publish schedules event and returns immediately. event.ID must be the
// deterministic id the stream dedupes on.
func (a *AsyncPublisher) Publish(subject string, event Event) {
	select {
	case a.sem <- struct{}{}:
	default:
		a.result(subject, AsyncDropped, nil)
		return
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { <-a.sem }()
		var err error
		delay := a.cfg.BaseDelay
		for attempt := 1; attempt <= a.cfg.Attempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), a.cfg.AttemptTimeout)
			err = a.pub.PublishDedup(ctx, subject, event)
			cancel()
			if err == nil {
				a.result(subject, AsyncPublished, nil)
				return
			}
			if attempt < a.cfg.Attempts {
				a.sleep(delay)
				if delay *= 2; delay > a.cfg.MaxDelay {
					delay = a.cfg.MaxDelay
				}
			}
		}
		a.result(subject, AsyncFailed, err)
	}()
}

// Wait blocks until every scheduled event finished or ctx ends (shutdown/tests).
func (a *AsyncPublisher) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { a.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
