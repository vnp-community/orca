package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/eventhub"
	mcpgrpc "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/grpc"
	mcppostgres "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/postgres"
	svcconfig "github.com/stablyai/orca-go/services/mcp-service/internal/config"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// gatewayOnlyMethods are callable only by api-gateway (shared-secret guard).
// DecideApproval is here so an MCP bearer path can never reach it.
var gatewayOnlyMethods = []string{
	"/orca.mcp.v1.McpService/EvaluateToolCall",
	"/orca.mcp.v1.McpService/FilterTools",
	"/orca.mcp.v1.McpService/AuthorizeToolCall",
	"/orca.mcp.v1.McpService/CompleteToolCall",
	"/orca.mcp.v1.McpService/WaitApproval",
	"/orca.mcp.v1.McpService/DecideApproval",
	"/orca.mcp.v1.McpService/GetKillState",
}

type governance struct {
	usecases mcpgrpc.GovernanceUsecases
	expire   *usecase.ExpireApprovals
	cleanup  *usecase.KillSwitchCleanup
	maint    *usecase.ToolCallMaintenance
}

func buildGovernance(repo *mcppostgres.Repository, engine usecase.PolicyEngine, clients usecase.ClientStatusReader,
	revoker usecase.RefreshTokenRevoker, defaults usecase.Defaults, cfg svcconfig.GovernanceConfig, logger *slog.Logger) governance {
	clock := usecase.SystemClock{}
	core := usecase.NewGovernanceCore(repo, repo, repo, engine, clients, clock, defaults, cfg.Usecase, logger)
	red := domain.SecretRedactor{}
	hub := eventhub.New(repo, core.InvalidateTenant)
	return governance{
		usecases: mcpgrpc.GovernanceUsecases{
			Evaluate: usecase.NewEvaluateToolCall(core), Filter: usecase.NewFilterTools(core), Explain: usecase.NewExplainPolicy(core),
			Policies: usecase.NewPolicyAdmin(repo, engine, core, clock), Settings: usecase.NewSettingsAdmin(repo, repo, core, defaults, clock),
			Authorize: usecase.NewAuthorizeToolCall(core, repo, repo, red, clock), Complete: usecase.NewCompleteToolCall(core, repo, clock),
			Wait: usecase.NewWaitApproval(repo, clock, 0), ListApproval: usecase.NewListApprovals(repo, clock),
			Decide: usecase.NewDecideApproval(core, repo, red, clock), KillAdmin: usecase.NewKillSwitchAdmin(repo, clients, core, red, clock),
			KillState: usecase.NewGetKillState(core), Hub: hub,
		},
		expire:  usecase.NewExpireApprovals(repo, clock),
		cleanup: usecase.NewKillSwitchCleanup(repo, repo, repo, revoker, nil, clock, logger),
		maint:   usecase.NewToolCallMaintenance(repo, nil, cfg.Usecase, cfg.ToolCallsRetention, clock),
	}
}

// startWorkers runs the periodic governance jobs; every one is safe on every
// replica at once (conditional UPDATEs / idempotent cleanup).
func (g governance) startWorkers(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, every, retention time.Duration) {
	loop := func(name string, period time.Duration, fn func(context.Context) (int, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(period)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if n, err := fn(ctx); err != nil {
						logger.WarnContext(ctx, "mcp worker failed", slog.String("worker", name), slog.Any("error", err))
					} else if n > 0 {
						logger.InfoContext(ctx, "mcp worker progress", slog.String("worker", name), slog.Int("count", n))
					}
				}
			}
		}()
	}
	loop("expire-approvals", every, func(c context.Context) (int, error) { return g.expire.Execute(c, 100) })
	loop("kill-switch-cleanup", every/2, func(c context.Context) (int, error) { return g.cleanup.Execute(c, 20) })
	loop("reap-interrupted-calls", 3*every, func(c context.Context) (int, error) { return g.maint.ReapInterrupted(c, 100) })
	if retention > 0 {
		loop("purge-tool-calls", time.Hour, func(c context.Context) (int, error) { return g.maint.Purge(c, 1000) })
	}
}
