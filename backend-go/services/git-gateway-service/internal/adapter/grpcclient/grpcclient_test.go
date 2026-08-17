package grpcclient

import (
	"context"
	"errors"
	"testing"
)

func TestConnectionResolver_StubAlwaysReportsNotConnected(t *testing.T) {
	r := NewConnectionResolver()
	conn, err := r.ResolveConnection(context.Background(), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Connected {
		t.Error("expected stub ConnectionResolver to always report Connected=false")
	}
}

func TestRelayExecutor_StubMethodsReturnNotImplemented(t *testing.T) {
	r := NewRelayExecutor()
	ctx := context.Background()

	if _, err := r.GetStatus(ctx, "/repo"); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("GetStatus: expected ErrRelayNotImplemented, got %v", err)
	}
	if _, err := r.GetDiff(ctx, "/repo", false); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("GetDiff: expected ErrRelayNotImplemented, got %v", err)
	}
	if _, err := r.Commit(ctx, "/repo", "msg", nil); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("Commit: expected ErrRelayNotImplemented, got %v", err)
	}
	if _, err := r.Push(ctx, "/repo", "origin", "main"); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("Push: expected ErrRelayNotImplemented, got %v", err)
	}
	if _, err := r.Pull(ctx, "/repo"); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("Pull: expected ErrRelayNotImplemented, got %v", err)
	}
}
