// Command server is task-service's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/task-service/internal/config"

	taskeventbus "github.com/stablyai/orca-go/services/task-service/internal/adapter/eventbus"
	taskgrpc "github.com/stablyai/orca-go/services/task-service/internal/adapter/grpc"
	taskgrpcclient "github.com/stablyai/orca-go/services/task-service/internal/adapter/grpcclient"
	taskopaclient "github.com/stablyai/orca-go/services/task-service/internal/adapter/opaclient"
	taskpostgres "github.com/stablyai/orca-go/services/task-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("task-service exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := svcconfig.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := logging.New(cfg.ServiceName, version)
	slog.SetDefault(logger)

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("initializing tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	repo := taskpostgres.New(pool)

	// Complex (orchestration-service) execution dispatch is real as of
	// TASK-TG-04-04: dials orchestration-service's StartCoordinatorRun RPC.
	// FLAGGED DEPENDENCY (not covered by this task, orchestration-service's
	// own scope): StartCoordinatorRun's server-side handler — persisting a
	// coordinator_runs row, minting orchestration_tasks rows, starting the
	// state-machine — may not exist yet in orchestration-service. Calling
	// an RPC with no server implementation fails at dial/call time (a real,
	// visible error), not at compile time, so this wiring is safe to land
	// ahead of that landing; confirm it before relying on the complex path
	// in production. StubComplexExecutor (grpcclient's doc comment) remains
	// available as a fallback for environments where that handler isn't up.
	orchConn, err := taskgrpcclient.Dial(cfg.OrchestrationServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing orchestration-service: %w", err)
	}
	defer func() { _ = orchConn.Close() }()
	orchClient := orchestrationv1.NewOrchestrationServiceClient(orchConn)
	complexExecutor := taskgrpcclient.NewComplexExecutor(orchClient, repo, repo)

	tenantConn, err := taskgrpcclient.Dial(cfg.TenantServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing tenant-service: %w", err)
	}
	defer func() { _ = tenantConn.Close() }()
	tenantClient := tenantv1.NewTenantServiceClient(tenantConn)
	teamScopeResolver := taskgrpcclient.NewTeamScopeResolver(tenantClient)

	infraFleetConn, err := taskgrpcclient.Dial(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service: %w", err)
	}
	defer func() { _ = infraFleetConn.Close() }()
	infraFleetClient := infrafleetv1.NewInfraFleetServiceClient(infraFleetConn)
	projectExecutionResolver := taskgrpcclient.NewProjectExecutionResolver(infraFleetClient)
	simpleExecutor := taskgrpcclient.NewSimpleExecutor(repo, repo, projectExecutionResolver, infraFleetClient)
	aiCompleter := taskgrpcclient.NewAICompleter(infraFleetClient)

	aiProviderConn, err := taskgrpcclient.Dial(cfg.AIProviderServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing ai-provider-service: %w", err)
	}
	defer func() { _ = aiProviderConn.Close() }()
	aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)
	aiProviderContextResolver := taskgrpcclient.NewAIProviderContextResolver(aiProviderClient)

	// git-gateway-service dependency: TechStackDetector's ReadFile probes
	// (TASK-TG-02-03) and WorktreeProvisioner's CreateWorktree calls
	// (TASK-TG-04-02) — both a genuine scope addition, flagged in their own
	// task Context sections.
	gitGatewayConn, err := taskgrpcclient.Dial(cfg.GitGatewayServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing git-gateway-service: %w", err)
	}
	defer func() { _ = gitGatewayConn.Close() }()
	gitGatewayClient := gitgatewayv1.NewGitGatewayServiceClient(gitGatewayConn)
	techStackDetector := taskgrpcclient.NewTechStackDetector(gitGatewayClient, projectExecutionResolver)

	// project-service dependency: ProjectContextResolver's GetProject/
	// ListRepos calls (TASK-TG-02-04) — task-service never reads
	// project-service's tables directly.
	projectConn, err := taskgrpcclient.Dial(cfg.ProjectServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing project-service: %w", err)
	}
	defer func() { _ = projectConn.Close() }()
	projectClient := projectv1.NewProjectServiceClient(projectConn)
	projectContextResolver := taskgrpcclient.NewProjectContextResolver(projectClient)

	// opaEvaluator loads/compiles the orca-authz bundle once per distinct
	// query string (common/policy.Evaluator's own cache) and is shared by
	// every ResolvePermission call for this process's lifetime.
	opaEvaluator := policy.NewEvaluator(cfg.OPABundlePath)
	opaClient := taskopaclient.New(opaEvaluator)

	// Transactional-outbox relay (TASK-TG-03-07): Grant/RevokeGrant durably
	// enqueue an audit-event outbox row (internal/adapter/postgres's
	// WriteOutboxEvent) via internal/adapter/eventbus.Publisher; this relay
	// is what actually gets those rows to NATS. Same "queue up unpublished
	// until an operator restarts this process" posture as usage-service's
	// identical wiring if NATS is unreachable at startup.
	var outboxRelay *outbox.Relay
	natsPub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := natsPub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			outboxRelay = outbox.NewRelay(repo, natsPub, outbox.DefaultConfig, logger)
		}
	}
	var relayWG sync.WaitGroup
	if outboxRelay != nil {
		relayWG.Add(1)
		go func() {
			defer relayWG.Done()
			outboxRelay.Run(ctx)
		}()
	}
	eventPublisher := taskeventbus.New(repo, logger)

	createTaskUC := usecase.NewCreateTask(repo)
	getTaskUC := usecase.NewGetTask(repo)
	addEdgeUC := usecase.NewAddEdge(repo)
	// resolvePermissionUC must be constructed before grantUC — Grant now
	// requires 'manage' access to a task before writing a new grant on it,
	// closing a live authorization gap (TASK-TG-03-01).
	resolvePermissionUC := usecase.NewResolvePermission(repo, repo, teamScopeResolver, opaClient)
	grantUC := usecase.NewGrant(repo, resolvePermissionUC, eventPublisher)
	revokeGrantUC := usecase.NewRevokeGrant(repo, resolvePermissionUC, eventPublisher)
	listGrantsUC := usecase.NewListGrants(repo, resolvePermissionUC)
	// worktreeProvisioner implements Execute's reuse-or-create worktree step
	// (TASK-TG-04-02/03) against git-gateway-service's existing
	// CreateWorktree saga, resolving repo_id itself via project-service.
	worktreeProvisioner := taskgrpcclient.NewWorktreeProvisioner(gitGatewayClient, projectClient)
	executeTaskUC := usecase.NewExecuteTask(repo, repo, simpleExecutor, complexExecutor, resolvePermissionUC, worktreeProvisioner, projectExecutionResolver, usecase.SystemClock{})
	hasActiveExecutionsUC := usecase.NewHasActiveExecutions(repo)
	listTasksUC := usecase.NewListTasks(repo)
	updateTaskUC := usecase.NewUpdateTask(repo, repo)
	deleteTaskUC := usecase.NewDeleteTask(repo)
	getDependenciesUC := usecase.NewGetDependencies(repo, repo)
	// repo also implements usecase.VelocityResolver (RecentCompletedTasks is
	// task-service's own data — no client adapter needed, see
	// adapter/postgres/velocity.go's doc comment).
	aiDecomposeUC := usecase.NewAIDecompose(
		repo, repo, aiProviderContextResolver, projectExecutionResolver,
		projectContextResolver, techStackDetector, repo, aiCompleter,
	)
	// repo also implements usecase.TxRunner (internal/adapter/postgres's
	// RunInTx) — AIApply needs its create-subtask+add-edge loop to run in
	// one transaction (TASK-224 Gap 2), not the standalone createTaskUC/
	// addEdgeUC instances above (those stay wired to the plain CreateTask/
	// AddEdge RPCs, which don't need a shared transaction).
	aiApplyUC := usecase.NewAIApply(repo)
	generateAgentPromptUC := usecase.NewGenerateAgentPrompt(repo, aiProviderContextResolver, projectExecutionResolver, aiCompleter)
	// TASK-TG-03-08's public/anonymous share-link flow. See server.go's
	// ResolvePublicLink doc comment for why api-gateway is NOT wired to
	// expose this yet. shareLinkStore is its own type (not repo) — see
	// adapter/postgres/share_links.go's doc comment for the method-name
	// collision that requires this.
	shareLinkStore := taskpostgres.NewShareLinkStore(pool)
	createPublicLinkUC := usecase.NewCreatePublicLink(shareLinkStore, resolvePermissionUC)
	revokePublicLinkUC := usecase.NewRevokePublicLink(shareLinkStore, resolvePermissionUC, repo)
	resolvePublicLinkUC := usecase.NewResolvePublicLink(shareLinkStore)
	// TASK-TG-01-08: subtree/progress/comments RPCs. repo also implements
	// usecase.GrantRepository (ListGrantsForAncestors) for GetSubtree's
	// per-node visibility filter.
	getSubtreeUC := usecase.NewGetSubtree(repo, repo, teamScopeResolver)
	recalculateProgressUC := usecase.NewRecalculateProgress(repo)
	addCommentUC := usecase.NewAddComment(repo)
	listCommentsUC := usecase.NewListComments(repo)
	// reportExecutionResultUC (TASK-TG-04-05) is the complex path's inbound
	// completion callback, called BY orchestration-service only — see
	// server.go's ReportTaskExecutionResult doc comment for the flagged
	// (unresolved) service-identity check this handler is missing.
	reportExecutionResultUC := usecase.NewReportTaskExecutionResult(repo)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	taskv1.RegisterTaskServiceServer(grpcServer, taskgrpc.New(
		createTaskUC, getTaskUC, addEdgeUC, grantUC, resolvePermissionUC, executeTaskUC, hasActiveExecutionsUC,
		listTasksUC, updateTaskUC, deleteTaskUC, getDependenciesUC, aiDecomposeUC, aiApplyUC, generateAgentPromptUC,
		revokeGrantUC, listGrantsUC, createPublicLinkUC, revokePublicLinkUC, resolvePublicLinkUC,
		getSubtreeUC, recalculateProgressUC, addCommentUC, listCommentsUC, reportExecutionResultUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: healthSrv.Handler(),
	}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			errCh <- fmt.Errorf("listening on grpc port: %w", err)
			return
		}
		logger.Info("task-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("task-service http (health) listening", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown: GracefulStop drains in-flight gRPC calls before
	// returning, matching the termination-grace-period expectation in
	// standards/production-readiness-checklist.md.
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	// Wait for the outbox relay goroutine (if started) to observe ctx
	// cancellation and return, so it doesn't outlive the rest of the
	// server on shutdown — same pattern usage-service's main.go uses.
	relayWG.Wait()

	return nil
}
