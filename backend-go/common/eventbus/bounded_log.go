package eventbus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// BoundedLogConfig describes a JetStream stream used as a hard-bounded replay
// buffer (e.g. MCP SSE resume). Discard=old means a full buffer loses its
// oldest events (readers see a gap) instead of growing without limit.
type BoundedLogConfig struct {
	Stream            string
	Subjects          []string
	MaxMsgsPerSubject int64
	MaxAge            time.Duration
	MaxBytes          int64
}

// LogEntry is one stored message.
type LogEntry struct {
	Subject string
	Seq     uint64
	Header  map[string]string
	Data    []byte
}

// ErrLogEmpty: no message is stored for the subject.
var ErrLogEmpty = errors.New("eventbus: no stored message for subject")

// ErrLogConflict: AppendAfter found a newer message on the subject than the
// caller expected (another writer appended first).
var ErrLogConflict = errors.New("eventbus: subject has a newer message than expected")

// BoundedLog reads and writes a bounded stream. It keeps its own connection.
type BoundedLog struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
	name   string
}

// NewBoundedLog connects and idempotently creates/updates the stream.
func NewBoundedLog(ctx context.Context, url string, cfg BoundedLogConfig) (*BoundedLog, error) {
	nc, err := nats.Connect(url, nats.Name("orca-bounded-log"))
	if err != nil {
		return nil, fmt.Errorf("eventbus: connecting to nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("eventbus: creating jetstream context: %w", err)
	}
	st, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: cfg.Stream, Subjects: cfg.Subjects, Storage: jetstream.MemoryStorage,
		Retention: jetstream.LimitsPolicy, Discard: jetstream.DiscardOld,
		MaxMsgsPerSubject: cfg.MaxMsgsPerSubject, MaxAge: cfg.MaxAge, MaxBytes: cfg.MaxBytes,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("eventbus: ensuring bounded stream %s: %w", cfg.Stream, err)
	}
	return &BoundedLog{nc: nc, js: js, stream: st, name: cfg.Stream}, nil
}

func (l *BoundedLog) Close() { l.nc.Close() }

// Ping reports connectivity (readiness).
func (l *BoundedLog) Ping() error {
	if !l.nc.IsConnected() {
		return errors.New("eventbus: nats disconnected")
	}
	return nil
}

// Append stores one message and returns its stream sequence.
func (l *BoundedLog) Append(ctx context.Context, subject string, header map[string]string, data []byte) (uint64, error) {
	msg := &nats.Msg{Subject: subject, Data: data}
	if len(header) > 0 {
		msg.Header = nats.Header{}
		for k, v := range header {
			msg.Header.Set(k, v)
		}
	}
	ack, err := l.js.PublishMsg(ctx, msg)
	if err != nil {
		return 0, fmt.Errorf("eventbus: appending to %s: %w", subject, err)
	}
	return ack.Sequence, nil
}

// AppendAfter stores one message only if the subject's newest message still
// has stream sequence lastSeq (0 = the subject must be empty). It is the
// compare-and-set that lets several replicas append to one subject while each
// derives the next ordinal from what it read; on a lost race it returns
// ErrLogConflict without storing anything.
func (l *BoundedLog) AppendAfter(ctx context.Context, subject string, lastSeq uint64, header map[string]string, data []byte) (uint64, error) {
	msg := &nats.Msg{Subject: subject, Data: data}
	if len(header) > 0 {
		msg.Header = nats.Header{}
		for k, v := range header {
			msg.Header.Set(k, v)
		}
	}
	ack, err := l.js.PublishMsg(ctx, msg, jetstream.WithExpectLastSequencePerSubject(lastSeq))
	if err != nil {
		var apiErr *jetstream.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence ||
			apiErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequenceConstant) {
			return 0, ErrLogConflict
		}
		return 0, fmt.Errorf("eventbus: appending to %s: %w", subject, err)
	}
	return ack.Sequence, nil
}

// Last returns the newest stored message of subject (ErrLogEmpty if none).
func (l *BoundedLog) Last(ctx context.Context, subject string) (LogEntry, error) {
	raw, err := l.stream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			return LogEntry{}, ErrLogEmpty
		}
		return LogEntry{}, fmt.Errorf("eventbus: reading %s: %w", subject, err)
	}
	return toEntry(raw.Subject, raw.Sequence, raw.Header, raw.Data), nil
}

func toEntry(subject string, seq uint64, h nats.Header, data []byte) LogEntry {
	e := LogEntry{Subject: subject, Seq: seq, Data: data}
	if len(h) > 0 {
		e.Header = make(map[string]string, len(h))
		for k := range h {
			e.Header[k] = h.Get(k)
		}
	}
	return e
}

// First returns the oldest stored message of subject (ErrLogEmpty if none).
func (l *BoundedLog) First(ctx context.Context, subject string) (LogEntry, error) {
	raw, err := l.stream.GetMsg(ctx, 1, jetstream.WithGetMsgSubject(subject))
	if err != nil {
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			return LogEntry{}, ErrLogEmpty
		}
		return LogEntry{}, fmt.Errorf("eventbus: reading %s: %w", subject, err)
	}
	return toEntry(raw.Subject, raw.Sequence, raw.Header, raw.Data), nil
}

// Follow delivers every stored message of subject in order, then live ones,
// until fn returns stop=true or an error, or ctx ends. The ordered consumer is
// gap-free and recreates itself on a missed heartbeat.
func (l *BoundedLog) Follow(ctx context.Context, subject string, fn func(LogEntry) (stop bool, err error)) error {
	cons, err := l.js.OrderedConsumer(ctx, l.name, jetstream.OrderedConsumerConfig{FilterSubjects: []string{subject}})
	if err != nil {
		return fmt.Errorf("eventbus: ordered consumer for %s: %w", subject, err)
	}
	it, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("eventbus: starting iterator: %w", err)
	}
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			it.Stop()
		case <-stopped:
		}
	}()
	defer it.Stop()
	for {
		m, err := it.Next()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				return ctx.Err()
			}
			return fmt.Errorf("eventbus: reading %s: %w", subject, err)
		}
		md, _ := m.Metadata()
		var seq uint64
		if md != nil {
			seq = md.Sequence.Stream
		}
		stop, err := fn(toEntry(m.Subject(), seq, m.Headers(), m.Data()))
		if err != nil || stop {
			return err
		}
	}
}

// Snapshot returns the messages currently stored for subject, oldest first,
// without waiting for new ones (at most the stream's per-subject bound).
func (l *BoundedLog) Snapshot(ctx context.Context, subject string) ([]LogEntry, error) {
	info, err := l.stream.Info(ctx, jetstream.WithSubjectFilter(subject))
	if err != nil {
		return nil, fmt.Errorf("eventbus: stream info: %w", err)
	}
	want := int(info.State.Subjects[subject])
	if want == 0 {
		return nil, nil
	}
	out := make([]LogEntry, 0, want)
	fctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = l.Follow(fctx, subject, func(e LogEntry) (bool, error) {
		out = append(out, e)
		return len(out) >= want, nil
	})
	if err != nil && len(out) < want {
		return out, err
	}
	return out, nil
}

// Purge drops stored messages matching the subject filter (may use wildcards).
func (l *BoundedLog) Purge(ctx context.Context, filter string) error {
	if err := l.stream.Purge(ctx, jetstream.WithPurgeSubject(filter)); err != nil {
		return fmt.Errorf("eventbus: purging %s: %w", filter, err)
	}
	return nil
}
