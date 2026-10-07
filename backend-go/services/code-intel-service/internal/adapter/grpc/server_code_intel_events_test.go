package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/adapter/broadcaster"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockStreamServer struct {
	grpc.ServerStream
	ctx   context.Context
	sent  []*codeintelv1.CodeIntelPush
	block chan struct{}
}

func newMockStreamServer(ctx context.Context) *mockStreamServer {
	return &mockStreamServer{
		ctx:   ctx,
		block: make(chan struct{}),
	}
}

func (m *mockStreamServer) Context() context.Context {
	return m.ctx
}

func (m *mockStreamServer) Send(p *codeintelv1.CodeIntelPush) error {
	m.sent = append(m.sent, p)
	return nil
}

func TestServer_StreamCodeIntelEvents_StreamsPushEvents(t *testing.T) {
	server := NewCodeIntelServer(nil, nil, nil, nil, nil)
	b := broadcaster.NewCodeIntelPushBroadcaster()
	server.SetBroadcasterAndAuthz(b, nil)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = tenant.WithTenantID(ctx, "tenant-1")

	stream := newMockStreamServer(ctx)

	req := &codeintelv1.StreamCodeIntelEventsRequest{
		Selectors: []*codeintelv1.WorktreeSelector{
			{ProjectId: "p1", WorktreeRef: "wt-1"},
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- server.StreamCodeIntelEvents(req, stream)
	}()

	// Allow subscriber registration
	time.Sleep(20 * time.Millisecond)

	// Publish matching event
	b.Publish(&codeintelv1.CodeIntelPush{
		Kind:       "changed",
		WorktreeId: "wt-1",
		ProjectId:  "p1",
		Reason:     "index_changed",
	})

	// Publish non-matching event (different worktree)
	b.Publish(&codeintelv1.CodeIntelPush{
		Kind:       "changed",
		WorktreeId: "wt-other",
		ProjectId:  "p1",
	})

	time.Sleep(20 * time.Millisecond)
	cancel()

	err := <-done
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	if len(stream.sent) != 1 {
		t.Fatalf("expected exactly 1 matching push streamed, got %d", len(stream.sent))
	}
	if stream.sent[0].WorktreeId != "wt-1" {
		t.Errorf("expected push with worktree wt-1, got %s", stream.sent[0].WorktreeId)
	}
}

func TestServer_StreamCodeIntelEvents_MissingTenantRejects(t *testing.T) {
	server := NewCodeIntelServer(nil, nil, nil, nil, nil)
	stream := newMockStreamServer(context.Background()) // no tenant

	err := server.StreamCodeIntelEvents(&codeintelv1.StreamCodeIntelEventsRequest{}, stream)
	if err == nil {
		t.Fatalf("expected error for unauthenticated request")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", err)
	}
}
