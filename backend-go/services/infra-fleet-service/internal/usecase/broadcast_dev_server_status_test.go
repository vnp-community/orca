package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type liveStub struct {
	connected map[string]bool
	runs      map[string]int
}

func (l liveStub) IsConnected(id string) bool    { return l.connected[id] }
func (l liveStub) ActiveExecutions(id string) int { return l.runs[id] }

type tenantStub map[string]string

func (t tenantStub) TenantIDForDevServer(_ context.Context, id string) (string, bool, error) {
	v, ok := t[id]
	return v, ok, nil
}

type collectedStatus struct {
	tenant string
	ev     domain.DevServerStatusEvent
}

type statusSink struct {
	got []collectedStatus
	err error
}

func (s *statusSink) PublishDevServerStatus(_ context.Context, tenantID string, ev domain.DevServerStatusEvent) error {
	s.got = append(s.got, collectedStatus{tenantID, ev})
	return s.err
}

func TestBroadcastDevServerStatus_ABurstBecomesOneSnapshotOfTheCurrentState(t *testing.T) {
	sink := &statusSink{}
	live := liveStub{connected: map[string]bool{"ds-1": true}, runs: map[string]int{"ds-1": 2}}
	b := NewBroadcastDevServerStatus(tenantStub{"ds-1": "t1"}, live, sink, nil)

	for range 5 {
		b.Notify("ds-1")
	}
	b.Flush(context.Background())

	if len(sink.got) != 1 || sink.got[0].tenant != "t1" || sink.got[0].ev != (domain.DevServerStatusEvent{DevServerID: "ds-1", Connected: true, ActiveExecutions: 2}) {
		t.Fatalf("got %+v", sink.got)
	}
	b.Flush(context.Background())
	if len(sink.got) != 1 {
		t.Fatalf("nothing marked, nothing sent: %+v", sink.got)
	}
}

func TestBroadcastDevServerStatus_DisconnectedMeansNoRuns(t *testing.T) {
	sink := &statusSink{}
	live := liveStub{connected: map[string]bool{}, runs: map[string]int{"ds-1": 3}}
	b := NewBroadcastDevServerStatus(tenantStub{"ds-1": "t1"}, live, sink, nil)
	b.Notify("ds-1")
	b.Flush(context.Background())
	if len(sink.got) != 1 || sink.got[0].ev.Connected || sink.got[0].ev.ActiveExecutions != 0 {
		t.Fatalf("got %+v", sink.got)
	}
}

func TestBroadcastDevServerStatus_UnknownDevServerAndPublishFailureAreSurvived(t *testing.T) {
	sink := &statusSink{err: errors.New("nats down")}
	b := NewBroadcastDevServerStatus(tenantStub{"known": "t1"}, liveStub{}, sink, nil)
	b.Notify("ghost")
	b.Notify("known")
	b.Flush(context.Background())
	if len(sink.got) != 1 || sink.got[0].ev.DevServerID != "known" {
		t.Fatalf("a dev server with no tenant is skipped, the failing publish is logged: %+v", sink.got)
	}
}

func TestBroadcastDevServerStatus_RunSendsAfterNotifyAndStopsWithTheContext(t *testing.T) {
	sink := &statusSink{}
	b := NewBroadcastDevServerStatus(tenantStub{"ds-1": "t1"}, liveStub{connected: map[string]bool{"ds-1": true}}, sink, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()

	b.Notify("ds-1")
	deadline := time.Now().Add(3 * time.Second)
	for {
		b.mu.Lock()
		idle := len(b.pending) == 0
		b.mu.Unlock()
		if idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the worker never picked up the notification")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
}
