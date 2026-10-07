package infrafleetclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/config"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

type devServerSemaphores struct {
	mu    sync.Mutex
	sems  map[string]chan struct{}
	limit int
}

func newDevServerSemaphores(limit int) *devServerSemaphores {
	if limit <= 0 {
		limit = 4
	}
	return &devServerSemaphores{
		sems:  make(map[string]chan struct{}),
		limit: limit,
	}
}

func (s *devServerSemaphores) acquire(ctx context.Context, devServerID string) (func(), error) {
	s.mu.Lock()
	sem, ok := s.sems[devServerID]
	if !ok {
		sem = make(chan struct{}, s.limit)
		s.sems[devServerID] = sem
	}
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	}
}

// Dial constructs a gRPC client connection to infra-fleet-service.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16<<20)),
	)
}

// AgentRPCCaller implements usecase.AgentRPCCaller over InfraFleetServiceClient.
type AgentRPCCaller struct {
	client        infrafleetv1.InfraFleetServiceClient
	cfg           config.Config
	logger        *slog.Logger
	devServerSems *devServerSemaphores
	reconnectGate *reconnectGate
}

func NewAgentRPCCaller(client infrafleetv1.InfraFleetServiceClient, cfg config.Config, logger *slog.Logger) *AgentRPCCaller {
	if logger == nil {
		logger = slog.Default()
	}
	limit := cfg.MaxInflightPerDevServer
	if limit <= 0 {
		limit = 4
	}
	return &AgentRPCCaller{
		client:        client,
		cfg:           cfg,
		logger:        logger,
		devServerSems: newDevServerSemaphores(limit),
		reconnectGate: newReconnectGate(client, cfg.ReconnectWait),
	}
}

func isMutatingMethod(method string) bool {
	return method == "codeintel.reindex" || method == "codeintel.reindexCancel" || method == "codeintel.watch"
}

func isDevServerNotConnected(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return strings.Contains(st.Message(), "INFRA_DEV_SERVER_NOT_CONNECTED")
}

func isTrailerRetryable(trailer metadata.MD) bool {
	if trailer == nil {
		return false
	}
	vals := trailer.Get("x-orca-agent-error-data-bin")
	if len(vals) == 0 {
		return false
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(vals[0]), &data); err != nil {
		return false
	}
	if r, ok := data["retryable"].(bool); ok && r {
		return true
	}
	return false
}

func (c *AgentRPCCaller) Call(ctx context.Context, target usecase.AgentTarget, method string, params map[string]any) (usecase.RawCodeIntelResult, error) {
	start := time.Now()
	timeout := c.cfg.AgentCallTimeout
	if timeout <= 0 {
		timeout = 95 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	callCtx, err := withTenantMetadata(callCtx)
	if err != nil {
		return usecase.RawCodeIntelResult{}, err
	}

	release, err := c.devServerSems.acquire(callCtx, target.DevServerID)
	if err != nil {
		return usecase.RawCodeIntelResult{}, err
	}
	defer release()

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return usecase.RawCodeIntelResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", "failed to marshal params", err)
	}

	req := &infrafleetv1.RelayByDevServerRequest{
		DevServerId: target.DevServerID,
		Method:      method,
		ParamsJson:  string(paramsJSON),
	}

	var trailerMD metadata.MD
	var resp *infrafleetv1.RelayResponse

	attempt := 0
	maxUnavailableRetries := 2
	if isMutatingMethod(method) {
		maxUnavailableRetries = 0
	}

	for {
		trailerMD = metadata.MD{}
		resp, err = c.client.RelayByDevServer(callCtx, req, grpc.Trailer(&trailerMD))
		if err == nil {
			break
		}

		// 1. Reconnect wait gate for not-connected dev server (read methods only)
		if !isMutatingMethod(method) && isDevServerNotConnected(err) && c.cfg.ReconnectWait > 0 {
			reconnectErr := c.reconnectGate.waitForReconnect(callCtx, target.TenantID, target.DevServerID)
			if reconnectErr != nil {
				return usecase.RawCodeIntelResult{}, reconnectErr
			}
			// Reconnected: retry call once
			trailerMD = metadata.MD{}
			resp, err = c.client.RelayByDevServer(callCtx, req, grpc.Trailer(&trailerMD))
			break
		}

		// 2. Retry Unavailable up to 2 times for read methods
		st, isGRPC := status.FromError(err)
		if !isMutatingMethod(method) && isGRPC && st.Code() == codes.Unavailable && attempt < maxUnavailableRetries {
			attempt++
			backoff := 200 * time.Millisecond
			if attempt == 2 {
				backoff = 600 * time.Millisecond
			}
			select {
			case <-callCtx.Done():
				return usecase.RawCodeIntelResult{}, callCtx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// 3. Retry TOOL_FAILED once if trailer marks retryable: true
		if !isMutatingMethod(method) && isTrailerRetryable(trailerMD) && attempt == 0 {
			attempt++
			select {
			case <-callCtx.Done():
				return usecase.RawCodeIntelResult{}, callCtx.Err()
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		break
	}

	dur := time.Since(start)

	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	// Log only tenantId, devServerId, method, outcome, durationMs.
	// Never log params_json, data, or absolute paths.
	c.logger.InfoContext(ctx, "agent rpc call completed",
		"tenantId", target.TenantID,
		"devServerId", target.DevServerID,
		"method", method,
		"outcome", outcome,
		"durationMs", dur.Milliseconds(),
	)

	if err != nil {
		return usecase.RawCodeIntelResult{}, MapRelayError(err, trailerMD)
	}

	maxBytes := c.cfg.AgentMaxResultBytes
	if maxBytes <= 0 {
		maxBytes = 12 * 1024 * 1024
	}

	return decodeRawCodeIntelResult(resp.GetResultJson(), maxBytes)
}
