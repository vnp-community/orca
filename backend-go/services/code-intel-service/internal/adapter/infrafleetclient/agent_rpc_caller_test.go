package infrafleetclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/config"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient
	mu               sync.Mutex
	relayFunc        func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error)
	capturedRequests []*infrafleetv1.RelayByDevServerRequest
}

func (f *fakeInfraFleetClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.mu.Lock()
	f.capturedRequests = append(f.capturedRequests, in)
	fn := f.relayFunc
	f.mu.Unlock()

	if fn != nil {
		return fn(ctx, in, opts...)
	}
	return &infrafleetv1.RelayResponse{
		ResultJson: `{"sources":["gitnexus"],"headCommit":"abc","stale":false,"data":{"nodes":[]}}`,
	}, nil
}

func TestAgentRPCCaller_Call(t *testing.T) {
	cfg := config.Config{
		AgentCallTimeout:        5 * time.Second,
		MaxInflightPerDevServer: 4,
		AgentMaxResultBytes:     12 * 1024 * 1024, // 12 MiB
	}
	target := usecase.AgentTarget{
		TenantID:      "t-test",
		DevServerID:   "ds-test",
		WorkspaceRoot: "/workspace/repo",
	}

	t.Run("MissingTenantInContextFails", func(t *testing.T) {
		client := &fakeInfraFleetClient{}
		caller := NewAgentRPCCaller(client, cfg, nil)
		_, err := caller.Call(context.Background(), target, "codeintel.status", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err == nil {
			t.Fatal("expected error when tenant missing in ctx")
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "INFRA_NO_TENANT" {
			t.Errorf("expected INFRA_NO_TENANT, got %v", err)
		}
	})

	t.Run("SuccessfulCallDecodesRawCodeIntelResult", func(t *testing.T) {
		client := &fakeInfraFleetClient{}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		res, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{
			"workspaceRoot": "/workspace/repo",
			"topN":          50,
		})
		if err != nil {
			t.Fatalf("unexpected call error: %v", err)
		}
		if len(res.Sources) != 1 || res.Sources[0] != "gitnexus" || res.HeadCommit != "abc" {
			t.Errorf("unexpected res fields: %+v", res)
		}
		if len(res.Data) == 0 {
			t.Error("expected non-empty Data")
		}
	})

	t.Run("8MiBPayloadSucceeds", func(t *testing.T) {
		// Generate 8 MiB payload
		largePayload := strings.Repeat("A", 8*1024*1024)
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{
					ResultJson: fmt.Sprintf(`{"sources":["gitnexus"],"data":{"large":"%s"}}`, largePayload),
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		res, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err != nil {
			t.Fatalf("8 MiB payload should succeed, got: %v", err)
		}
		if len(res.Data) == 0 {
			t.Error("expected data to be populated")
		}
	})

	t.Run("13MiBPayloadFailsWithOutputTooLarge", func(t *testing.T) {
		// Generate 13 MiB payload
		largePayload := strings.Repeat("A", 13*1024*1024)
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{
					ResultJson: fmt.Sprintf(`{"sources":["gitnexus"],"data":{"large":"%s"}}`, largePayload),
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err == nil {
			t.Fatal("expected 13 MiB payload to fail")
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "CODEINTEL_OUTPUT_TOO_LARGE" {
			t.Errorf("expected CODEINTEL_OUTPUT_TOO_LARGE, got %v", err)
		}
	})

	t.Run("MissingOrEmptyDataFieldReturnsResultInvalid", func(t *testing.T) {
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{
					ResultJson: `{"sources":["gitnexus"],"headCommit":"abc"}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err == nil {
			t.Fatal("expected error on missing data field")
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "CODEINTEL_RESULT_INVALID" {
			t.Errorf("expected CODEINTEL_RESULT_INVALID, got %v", err)
		}
	})

	t.Run("MalformedJSONReturnsResultInvalid", func(t *testing.T) {
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{
					ResultJson: `{not-json}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err == nil {
			t.Fatal("expected error on malformed JSON")
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "CODEINTEL_RESULT_INVALID" {
			t.Errorf("expected CODEINTEL_RESULT_INVALID, got %v", err)
		}
	})

	t.Run("EmbeddedJSONRPCErrorEnvelopeReturnsToolFailed", func(t *testing.T) {
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{
					ResultJson: `{"jsonrpc":"2.0","error":{"code":-32000,"message":"git failure"}}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
		if err == nil {
			t.Fatal("expected error on embedded error envelope")
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "CODEINTEL_TOOL_FAILED" {
			t.Errorf("expected CODEINTEL_TOOL_FAILED, got %v", err)
		}
	})

	t.Run("LoggingDoesNotContainParamsJsonDataOrAbsolutePaths", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
		client := &fakeInfraFleetClient{}
		caller := NewAgentRPCCaller(client, cfg, logger)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		secretParam := "super-secret-param-value"
		_, err := caller.Call(ctx, target, "codeintel.overview", map[string]any{
			"workspaceRoot": "/secret/abs/path/to/project",
			"key":           secretParam,
		})
		if err != nil {
			t.Fatalf("unexpected call error: %v", err)
		}

		logOutput := logBuf.String()
		if strings.Contains(logOutput, secretParam) {
			t.Errorf("log output must NOT contain params: %s", logOutput)
		}
		if strings.Contains(logOutput, "/secret/abs/path/to/project") {
			t.Errorf("log output must NOT contain absolute workspaceRoot path: %s", logOutput)
		}
		if !strings.Contains(logOutput, "tenantId") || !strings.Contains(logOutput, "devServerId") {
			t.Errorf("log output should contain safe metadata: %s", logOutput)
		}
	})

	t.Run("SemaphoreLimitsConcurrencyTo4", func(t *testing.T) {
		var inFlight int32
		var maxInFlight int32
		client := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				curr := atomic.AddInt32(&inFlight, 1)
				for {
					max := atomic.LoadInt32(&maxInFlight)
					if curr <= max || atomic.CompareAndSwapInt32(&maxInFlight, max, curr) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond)
				atomic.AddInt32(&inFlight, -1)
				return &infrafleetv1.RelayResponse{
					ResultJson: `{"sources":["gitnexus"],"data":{"ok":true}}`,
				}, nil
			},
		}
		caller := NewAgentRPCCaller(client, cfg, nil)
		ctx := tenant.WithTenantID(context.Background(), "t-test")

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = caller.Call(ctx, target, "codeintel.overview", map[string]any{"workspaceRoot": "/workspace/repo"})
			}()
		}
		wg.Wait()

		if maxInFlight > 4 {
			t.Errorf("maxInFlight was %d, expected <= 4", maxInFlight)
		}
	})
}
