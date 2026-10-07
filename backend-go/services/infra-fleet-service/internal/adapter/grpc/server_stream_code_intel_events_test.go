package grpc

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

type fakeStreamCodeIntelEventsServer struct {
	grpc.ServerStream
	ctx  context.Context
	sent []*infrafleetv1.CodeIntelEvent
	mu   sync.Mutex
}

func (f *fakeStreamCodeIntelEventsServer) Context() context.Context {
	return f.ctx
}

func (f *fakeStreamCodeIntelEventsServer) Send(ev *infrafleetv1.CodeIntelEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, ev)
	return nil
}

type fakeStreamSource struct {
	mu   sync.Mutex
	subs map[string][]chan domain.CodeIntelEvent
}

func newFakeStreamSource() *fakeStreamSource {
	return &fakeStreamSource{subs: make(map[string][]chan domain.CodeIntelEvent)}
}

func (f *fakeStreamSource) SubscribeCodeIntelEvents(devServerID string) (<-chan domain.CodeIntelEvent, func()) {
	ch := make(chan domain.CodeIntelEvent, 16)
	f.mu.Lock()
	f.subs[devServerID] = append(f.subs[devServerID], ch)
	f.mu.Unlock()

	return ch, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		current := f.subs[devServerID]
		filtered := make([]chan domain.CodeIntelEvent, 0, len(current))
		for _, c := range current {
			if c != ch {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			delete(f.subs, devServerID)
		} else {
			f.subs[devServerID] = filtered
		}
	}
}

func (f *fakeStreamSource) emit(devServerID string, ev domain.CodeIntelEvent) {
	f.mu.Lock()
	subs := append([]chan domain.CodeIntelEvent(nil), f.subs[devServerID]...)
	f.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (f *fakeStreamSource) subCount(devServerID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subs[devServerID])
}

func withTenantStreamMetadata(ctx context.Context, tenantID string) context.Context {
	md := metadata.Pairs(grpcmw.MetadataTenantID, tenantID)
	return metadata.NewIncomingContext(ctx, md)
}

func TestServer_StreamCodeIntelEvents_UnimplementedWithoutWithCodeIntel(t *testing.T) {
	s := &Server{}
	stream := &fakeStreamCodeIntelEventsServer{ctx: context.Background()}
	err := s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented, got %v", err)
	}
}

func TestServer_StreamCodeIntelEvents_RequiresTenantMetadata(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	source := newFakeStreamSource()
	limiter := usecase.NewCodeIntelStreamLimiter(16)
	streamUC := usecase.NewStreamCodeIntelEvents(repo, source, limiter)

	s := &Server{}
	s.WithCodeIntel(streamUC, nil)

	// Stream without tenant metadata
	stream := &fakeStreamCodeIntelEventsServer{ctx: context.Background()}
	err := s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
	}
}

func TestServer_StreamCodeIntelEvents_DevServerNotFound(t *testing.T) {
	repo := &capsFakeRepo{getErr: errors.New("not found")}
	source := newFakeStreamSource()
	limiter := usecase.NewCodeIntelStreamLimiter(16)
	streamUC := usecase.NewStreamCodeIntelEvents(repo, source, limiter)

	s := &Server{}
	s.WithCodeIntel(streamUC, nil)

	ctx := withTenantStreamMetadata(context.Background(), "tenant-1")
	stream := &fakeStreamCodeIntelEventsServer{ctx: ctx}
	err := s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-missing"}, stream)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}
}

func TestServer_StreamCodeIntelEvents_StreamsEventsWithOptionalPercent(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	source := newFakeStreamSource()
	limiter := usecase.NewCodeIntelStreamLimiter(16)
	streamUC := usecase.NewStreamCodeIntelEvents(repo, source, limiter)

	s := &Server{}
	s.WithCodeIntel(streamUC, nil)

	ctx, cancel := context.WithCancel(withTenantStreamMetadata(context.Background(), "tenant-1"))
	defer cancel()

	stream := &fakeStreamCodeIntelEventsServer{ctx: ctx}

	done := make(chan error, 1)
	go func() {
		done <- s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
	}()

	// Wait for subscription to register
	for i := 0; i < 50; i++ {
		if source.subCount("ds-1") > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	percent42 := int32(42)
	source.emit("ds-1", domain.CodeIntelEvent{
		Kind:          domain.CodeIntelEventKindIndexChanged,
		WorkspaceRoot: "/work/orca",
		Percent:       nil,
	})
	source.emit("ds-1", domain.CodeIntelEvent{
		Kind:          domain.CodeIntelEventKindReindexProgress,
		WorkspaceRoot: "/work/orca",
		JobID:         "job-1",
		Percent:       &percent42,
	})

	// Wait for events to be received
	for i := 0; i < 50; i++ {
		stream.mu.Lock()
		count := len(stream.sent)
		stream.mu.Unlock()
		if count >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for StreamCodeIntelEvents to exit")
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 sent events, got %d", len(stream.sent))
	}

	if stream.sent[0].Percent != nil {
		t.Errorf("expected sent[0].Percent == nil, got %v", stream.sent[0].Percent)
	}
	if stream.sent[1].Percent == nil || *stream.sent[1].Percent != 42 {
		t.Errorf("expected sent[1].Percent == 42, got %v", stream.sent[1].Percent)
	}
}

func TestServer_StreamCodeIntelEvents_EnforcesLimit16AndReleasesOnCancel(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	source := newFakeStreamSource()
	limiter := usecase.NewCodeIntelStreamLimiter(16)
	streamUC := usecase.NewStreamCodeIntelEvents(repo, source, limiter)

	s := &Server{}
	s.WithCodeIntel(streamUC, nil)

	var cancels []context.CancelFunc
	var wg sync.WaitGroup

	for i := 0; i < 16; i++ {
		ctx, cancel := context.WithCancel(withTenantStreamMetadata(context.Background(), "tenant-1"))
		cancels = append(cancels, cancel)
		stream := &fakeStreamCodeIntelEventsServer{ctx: ctx}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
		}()
	}

	// Wait for all 16 subscriptions
	for i := 0; i < 100; i++ {
		if source.subCount("ds-1") == 16 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 17th stream should fail with FailedPrecondition
	ctx17 := withTenantStreamMetadata(context.Background(), "tenant-1")
	stream17 := &fakeStreamCodeIntelEventsServer{ctx: ctx17}
	err17 := s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream17)
	if status.Code(err17) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for 17th stream, got %v", status.Code(err17))
	}

	// Cancel first stream
	cancels[0]()

	// Wait for slot to be released
	for i := 0; i < 100; i++ {
		if source.subCount("ds-1") == 15 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now a new stream can connect
	ctxNew, cancelNew := context.WithCancel(withTenantStreamMetadata(context.Background(), "tenant-1"))
	streamNew := &fakeStreamCodeIntelEventsServer{ctx: ctxNew}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, streamNew)
	}()

	for i := 0; i < 100; i++ {
		if source.subCount("ds-1") == 16 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel all remaining
	cancelNew()
	for _, c := range cancels[1:] {
		c()
	}
	wg.Wait()
}

func TestServer_StreamCodeIntelEvents_GoroutineLeakCheck(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	source := newFakeStreamSource()
	limiter := usecase.NewCodeIntelStreamLimiter(16)
	streamUC := usecase.NewStreamCodeIntelEvents(repo, source, limiter)

	s := &Server{}
	s.WithCodeIntel(streamUC, nil)

	// Warm up
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(withTenantStreamMetadata(context.Background(), "tenant-1"))
		stream := &fakeStreamCodeIntelEventsServer{ctx: ctx}
		done := make(chan error, 1)
		go func() {
			done <- s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
		}()
		cancel()
		<-done
	}

	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(withTenantStreamMetadata(context.Background(), "tenant-1"))
		stream := &fakeStreamCodeIntelEventsServer{ctx: ctx}
		done := make(chan error, 1)
		go func() {
			done <- s.StreamCodeIntelEvents(&infrafleetv1.StreamCodeIntelEventsRequest{DevServerId: "ds-1"}, stream)
		}()
		cancel()
		<-done
	}

	runtime.GC()
	afterGoroutines := runtime.NumGoroutine()

	if afterGoroutines > baselineGoroutines+5 {
		t.Errorf("possible goroutine leak: baseline=%d, after=%d", baselineGoroutines, afterGoroutines)
	}
}
