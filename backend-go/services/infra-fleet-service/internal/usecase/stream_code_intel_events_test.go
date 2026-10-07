package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type fakeCodeIntelEventSource struct {
	subscribedDevServerID string
	unsubscribed          bool
	ch                    chan domain.CodeIntelEvent
}

func (f *fakeCodeIntelEventSource) SubscribeCodeIntelEvents(devServerID string) (<-chan domain.CodeIntelEvent, func()) {
	f.subscribedDevServerID = devServerID
	f.ch = make(chan domain.CodeIntelEvent, 1)
	return f.ch, func() {
		f.unsubscribed = true
	}
}

func TestStreamCodeIntelEvents_RequiresTenant(t *testing.T) {
	uc := NewStreamCodeIntelEvents(&fakeDevServerRepository{}, &fakeCodeIntelEventSource{}, NewCodeIntelStreamLimiter(16))
	_, _, err := uc.Execute(context.Background(), "ds-1")
	if err == nil {
		t.Fatal("expected error when no tenant in context")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindUnauthenticated {
		t.Errorf("expected KindUnauthenticated, got %v", appErr.Kind)
	}
}

func TestStreamCodeIntelEvents_RequiresDevServerID(t *testing.T) {
	uc := NewStreamCodeIntelEvents(&fakeDevServerRepository{}, &fakeCodeIntelEventSource{}, NewCodeIntelStreamLimiter(16))
	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected error when devServerId is empty")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindInvalidArgument {
		t.Errorf("expected KindInvalidArgument, got %v", appErr.Kind)
	}
}

func TestStreamCodeIntelEvents_DevServerNotFound(t *testing.T) {
	repo := &fakeDevServerRepository{getErr: errors.New("not found")}
	uc := NewStreamCodeIntelEvents(repo, &fakeCodeIntelEventSource{}, NewCodeIntelStreamLimiter(16))
	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, "ds-missing")
	if err == nil {
		t.Fatal("expected error when dev server not found")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindNotFound {
		t.Errorf("expected KindNotFound, got %v", appErr.Kind)
	}
}

func TestStreamCodeIntelEvents_SuccessAndRelease(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	source := &fakeCodeIntelEventSource{}
	limiter := NewCodeIntelStreamLimiter(16)
	uc := NewStreamCodeIntelEvents(repo, source, limiter)

	ctx := withTenant(context.Background(), "tenant-1")
	events, release, err := uc.Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events == nil || release == nil {
		t.Fatal("expected non-nil events and release")
	}
	if source.subscribedDevServerID != "ds-1" {
		t.Errorf("subscribedDevServerID = %q, want ds-1", source.subscribedDevServerID)
	}

	// Calling release invokes unsubscribe and frees the limiter
	release()
	if !source.unsubscribed {
		t.Error("expected source to be unsubscribed on release")
	}
}

func TestStreamCodeIntelEvents_EnforcesLimit16(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	source := &fakeCodeIntelEventSource{}
	limiter := NewCodeIntelStreamLimiter(16)
	uc := NewStreamCodeIntelEvents(repo, source, limiter)

	ctx := withTenant(context.Background(), "tenant-1")

	var releases []func()
	for i := 0; i < 16; i++ {
		_, rel, err := uc.Execute(ctx, "ds-1")
		if err != nil {
			t.Fatalf("stream %d failed: %v", i, err)
		}
		releases = append(releases, rel)
	}

	// 17th stream should fail
	_, _, err := uc.Execute(ctx, "ds-1")
	if err == nil {
		t.Fatal("expected 17th stream to fail with limiter error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindFailedPrecondition || appErr.Code != "INFRA_CODEINTEL_STREAM_LIMIT" {
		t.Fatalf("expected INFRA_CODEINTEL_STREAM_LIMIT, got %v", err)
	}

	// Release one stream
	releases[0]()

	// Now a new stream succeeds
	_, relNew, err := uc.Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("stream after release failed: %v", err)
	}
	relNew()

	for _, rel := range releases[1:] {
		rel()
	}
}
