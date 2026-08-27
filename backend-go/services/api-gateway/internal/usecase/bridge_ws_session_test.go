package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type fakeNotificationStream struct {
	frames []Frame
	i      int
}

func (s *fakeNotificationStream) Recv() (Frame, error) {
	if s.i >= len(s.frames) {
		return Frame{}, io.EOF
	}
	f := s.frames[s.i]
	s.i++
	return f, nil
}

type fakeWSWriter struct {
	written []Frame
}

func (w *fakeWSWriter) WriteJSON(frame Frame) error {
	w.written = append(w.written, frame)
	return nil
}

func TestBridgeWSSession_PumpsFramesUntilStreamEnds(t *testing.T) {
	stream := &fakeNotificationStream{frames: []Frame{
		{ID: "1", Type: "task.completed", PayloadJSON: `{"a":1}`},
		{ID: "2", Type: "workflow.execution.failed", PayloadJSON: `{"b":2}`},
	}}
	writer := &fakeWSWriter{}

	err := BridgeWSSession(context.Background(), slog.Default(), "conn-1", stream, writer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(writer.written) != 2 {
		t.Fatalf("got %d frames written, want 2", len(writer.written))
	}
	if writer.written[0].ID != "1" || writer.written[1].ID != "2" {
		t.Fatalf("frames written out of order: %+v", writer.written)
	}
}

func TestBridgeWSSession_StopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := &fakeNotificationStream{frames: []Frame{{ID: "1"}}}
	writer := &fakeWSWriter{}

	err := BridgeWSSession(ctx, slog.Default(), "conn-1", stream, writer)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want context.Canceled", err)
	}
}

type erroringStream struct{}

func (erroringStream) Recv() (Frame, error) { return Frame{}, errors.New("upstream broke") }

func TestBridgeWSSession_PropagatesUpstreamError(t *testing.T) {
	err := BridgeWSSession(context.Background(), slog.Default(), "conn-1", erroringStream{}, &fakeWSWriter{})
	if err == nil || err.Error() != "upstream broke" {
		t.Fatalf("got error %v, want upstream broke", err)
	}
}
