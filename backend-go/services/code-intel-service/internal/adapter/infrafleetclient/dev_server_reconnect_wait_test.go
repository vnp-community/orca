package infrafleetclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/config"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

type reconnectFakeClient struct {
	infrafleetv1.InfraFleetServiceClient
	mu               sync.Mutex
	probeCalls       int32
	connected        bool
	relayCalls       int32
	trailerToReturn  metadata.MD
	relayErrorToReturn error
}

func (f *reconnectFakeClient) IsDevServerConnected(ctx context.Context, in *infrafleetv1.IsDevServerConnectedRequest, opts ...grpc.CallOption) (*infrafleetv1.IsDevServerConnectedResponse, error) {
	atomic.AddInt32(&f.probeCalls, 1)
	f.mu.Lock()
	conn := f.connected
	f.mu.Unlock()
	return &infrafleetv1.IsDevServerConnectedResponse{Connected: conn}, nil
}

func (f *reconnectFakeClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	atomic.AddInt32(&f.relayCalls, 1)
	f.mu.Lock()
	tr := f.trailerToReturn
	err := f.relayErrorToReturn
	f.mu.Unlock()

	for _, opt := range opts {
		if trOpt, ok := opt.(grpc.TrailerCallOption); ok && tr != nil && trOpt.TrailerAddr != nil {
			*trOpt.TrailerAddr = metadata.Join(*trOpt.TrailerAddr, tr)
		}
	}

	if err != nil {
		return nil, err
	}
	return &infrafleetv1.RelayResponse{
		ResultJson: `{"sources":["gitnexus"],"data":{"ok":true}}`,
	}, nil
}

func TestReconnectWait_ReindexFailsImmediately(t *testing.T) {
	client := &reconnectFakeClient{
		relayErrorToReturn: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: offline"),
	}
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 4,
		ReconnectWait:           2 * time.Second,
	}
	caller := NewAgentRPCCaller(client, cfg, nil)
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	target := usecase.AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace",
	}

	// 1. Reindex must NOT wait or poll probe, must fail immediately
	_, err := caller.Call(ctx, target, "codeintel.reindex", map[string]any{"workspaceRoot": "/workspace"})
	if err == nil {
		t.Fatal("expected error for reindex while offline")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "CODEINTEL_DEV_SERVER_OFFLINE" {
		t.Errorf("expected CODEINTEL_DEV_SERVER_OFFLINE, got %v", err)
	}
	if atomic.LoadInt32(&client.probeCalls) != 0 {
		t.Errorf("probe should not be called for reindex, got %d", atomic.LoadInt32(&client.probeCalls))
	}
	if atomic.LoadInt32(&client.relayCalls) != 1 {
		t.Errorf("reindex should attempt RelayByDevServer exactly once, got %d", atomic.LoadInt32(&client.relayCalls))
	}
}

func TestReconnectWait_WatchFailsImmediately(t *testing.T) {
	client := &reconnectFakeClient{
		relayErrorToReturn: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: offline"),
	}
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 4,
		ReconnectWait:           2 * time.Second,
	}
	caller := NewAgentRPCCaller(client, cfg, nil)
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	target := usecase.AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace",
	}

	// Watch must NOT wait or poll probe
	_, err := caller.Call(ctx, target, "codeintel.watch", map[string]any{"workspaceRoot": "/workspace"})
	if err == nil {
		t.Fatal("expected error for watch while offline")
	}
	if atomic.LoadInt32(&client.probeCalls) != 0 {
		t.Errorf("probe should not be called for watch, got %d", atomic.LoadInt32(&client.probeCalls))
	}
}

func TestReconnectWait_ReconnectedAfterDelay(t *testing.T) {
	client := &reconnectFakeClient{
		relayErrorToReturn: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: offline"),
	}
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 4,
		ReconnectWait:           2 * time.Second,
	}
	caller := NewAgentRPCCaller(client, cfg, nil)
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	target := usecase.AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace",
	}

	// Transition to connected after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		client.mu.Lock()
		client.connected = true
		client.relayErrorToReturn = nil
		client.mu.Unlock()
	}()

	res, err := caller.Call(ctx, target, "codeintel.status", map[string]any{"workspaceRoot": "/workspace"})
	if err != nil {
		t.Fatalf("expected successful call after reconnect, got: %v", err)
	}
	if len(res.Data) == 0 {
		t.Error("expected non-empty data in result")
	}

	// RelayByDevServer called first time (offline), then second time (after reconnect)
	if atomic.LoadInt32(&client.relayCalls) != 2 {
		t.Errorf("expected 2 relay calls (initial + post-reconnect), got %d", atomic.LoadInt32(&client.relayCalls))
	}
}

func TestReconnectWait_TimeoutReturnsDevServerOffline(t *testing.T) {
	client := &reconnectFakeClient{
		relayErrorToReturn: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: offline"),
		connected:          false,
	}
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 4,
		ReconnectWait:           200 * time.Millisecond, // short budget for fast test
	}
	caller := NewAgentRPCCaller(client, cfg, nil)
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	target := usecase.AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace",
	}

	_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace"})
	if err == nil {
		t.Fatal("expected error when reconnect budget expires")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "CODEINTEL_DEV_SERVER_OFFLINE" {
		t.Errorf("expected CODEINTEL_DEV_SERVER_OFFLINE, got %v", err)
	}
}

func TestReconnectWait_Coalescing50ConcurrentRequests(t *testing.T) {
	client := &reconnectFakeClient{
		relayErrorToReturn: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: offline"),
		connected:          false,
	}
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 50,
		ReconnectWait:           200 * time.Millisecond,
	}
	caller := NewAgentRPCCaller(client, cfg, nil)
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	target := usecase.AgentTarget{
		TenantID:      "t-1",
		DevServerID:   "ds-1",
		WorkspaceRoot: "/workspace",
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = caller.Call(ctx, target, "codeintel.status", map[string]any{"workspaceRoot": "/workspace"})
		}()
	}
	wg.Wait()

	// 50 concurrent requests for the same tenant|devServerID must share a single polling loop.
	// In 200ms, a single loop polls at most 1-2 times. If each spawned its own loop, probeCalls would be ~50+.
	probeCount := atomic.LoadInt32(&client.probeCalls)
	if probeCount > 5 {
		t.Errorf("expected single shared polling loop (< 5 probes), got %d probes", probeCount)
	}
}

func TestReconnectWait_TwoTenantsDoNotShareGate(t *testing.T) {
	gate := newReconnectGate(&reconnectFakeClient{}, 100*time.Millisecond)
	k1 := gate.key("tenant-1", "ds-1")
	k2 := gate.key("tenant-2", "ds-1")
	if k1 == k2 {
		t.Errorf("gates should not match across tenants: %q == %q", k1, k2)
	}
}

func TestReadOnlyRetry_UnavailableAndToolFailed(t *testing.T) {
	t.Run("RetryUnavailableSucceedsOnSecondAttempt", func(t *testing.T) {
		var callCount int32
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				c := atomic.AddInt32(&callCount, 1)
				if c == 1 {
					return nil, status.Error(codes.Unavailable, "temporary network glitch")
				}
				return &infrafleetv1.RelayResponse{
					ResultJson: `{"sources":["gitnexus"],"data":{"ok":true}}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, config.Config{AgentCallTimeout: 5 * time.Second}, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-1")
		target := usecase.AgentTarget{TenantID: "t-1", DevServerID: "ds-1", WorkspaceRoot: "/app"}

		res, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/app"})
		if err != nil {
			t.Fatalf("expected retry to succeed: %v", err)
		}
		if len(res.Data) == 0 {
			t.Error("expected valid data after retry")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("RetryToolFailedWithRetryableTrailerSucceeds", func(t *testing.T) {
		var callCount int32
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				c := atomic.AddInt32(&callCount, 1)
				if c == 1 {
					for _, opt := range opts {
						if trOpt, ok := opt.(grpc.TrailerCallOption); ok && trOpt.TrailerAddr != nil {
							*trOpt.TrailerAddr = metadata.Pairs("x-orca-agent-error-data-bin", `{"retryable":true}`)
						}
					}
					return nil, status.Error(codes.Internal, "CODEINTEL_TOOL_FAILED: transient error")
				}
				return &infrafleetv1.RelayResponse{
					ResultJson: `{"sources":["gitnexus"],"data":{"ok":true}}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, config.Config{AgentCallTimeout: 5 * time.Second}, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-1")
		target := usecase.AgentTarget{TenantID: "t-1", DevServerID: "ds-1", WorkspaceRoot: "/app"}

		res, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/app"})
		if err != nil {
			t.Fatalf("expected retryable TOOL_FAILED to succeed on retry: %v", err)
		}
		if len(res.Data) == 0 {
			t.Error("expected valid data after retry")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})
}
