package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	mcpgrpc "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/grpc"
	mcppostgres "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/postgres"
	svcconfig "github.com/stablyai/orca-go/services/mcp-service/internal/config"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// sessionGatewayOnlyMethods: only api-gateway talks about sessions (BE-MCP-SOL-004).
var sessionGatewayOnlyMethods = []string{
	"/orca.mcp.v1.McpService/CreateSession",
	"/orca.mcp.v1.McpService/GetSessionBySecret",
	"/orca.mcp.v1.McpService/TouchSession",
	"/orca.mcp.v1.McpService/CloseSession",
	"/orca.mcp.v1.McpService/ListSessions",
	"/orca.mcp.v1.McpService/ListSessionsAdmin",
	"/orca.mcp.v1.McpService/OpenStream",
	"/orca.mcp.v1.McpService/HeartbeatStream",
	"/orca.mcp.v1.McpService/CloseStream",
}

func init() { gatewayOnlyMethods = append(gatewayOnlyMethods, sessionGatewayOnlyMethods...) }

// wireSessions adds the session RPCs and starts the idle reaper.
func wireSessions(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, base mcpv1.McpServiceServer, repo *mcppostgres.Repository, cfg svcconfig.SessionConfig) mcpv1.McpServiceServer {
	clock := usecase.SystemClock{}
	reap := usecase.NewReapIdleSessions(repo, clock, cfg.IdleTTL)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runSessionReaper(ctx, logger, reap, cfg.ReapInterval, cfg.ReapBatchLimit)
	}()
	return mcpgrpc.WithSessions(base, usecase.NewSessions(repo, clock))
}

func runSessionReaper(ctx context.Context, logger *slog.Logger, uc *usecase.ReapIdleSessions, every time.Duration, batch int) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := uc.Execute(ctx, batch); err != nil {
				logger.WarnContext(ctx, "idle session reap failed", slog.Any("error", err))
			} else if n > 0 {
				logger.InfoContext(ctx, "idle sessions closed", slog.Int("count", n))
			}
		}
	}
}
