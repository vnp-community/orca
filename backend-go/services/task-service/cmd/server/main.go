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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/auditclient"
	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
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
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// SystemClock implements usecase.Clock against the real wall clock — the
// only production implementation; execute_task_test.go uses a deterministic
// fake instead (SOL-TG-04's actual_hours computation needs to be testable).
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

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
	// TASK-TG-04-04/BE-SOL-002: dials orchestration-service's
	// StartCoordinatorRun RPC. FLAGGED DEPENDENCY (not covered by this
	// task, orchestration-service's own scope): StartCoordinatorRun's
	// server-side handler — persisting a coordinator_runs row, minting
	// orchestration_tasks rows, starting the state-machine — may not exist
	// yet in orchestration-service. Calling an RPC with no server
	// implementation fails at dial/call time (a real, visible error), not
	// at compile time, so this wiring is safe to land ahead of that
	// landing; confirm it before relying on the complex path in
	// production. StubComplexExecutor (grpcclient's doc comment) remains
	// available as a fallback for environments where that handler isn't up.
	orchConn, err := taskgrpcclient.Dial(cfg.OrchestrationServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing orchestration-service: %w", err)
	}
	defer func() { _ = orchConn.Close() }()
	orchClient := orchestrationv1.NewOrchestrationServiceClient(orchConn)
	complexExecutor := taskgrpcclient.NewComplexExecutor(repo, repo, orchClient)

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
	// execOutputRelay/repo (as usecase.OutboxWriter) back
	// TASK-AG-FLOWTASK-003's throttled mid-run republish — see
	// SimpleExecutor's own doc comment.
	execOutputRelay := taskgrpcclient.NewAgentExecOutputRelay(infraFleetClient)
	aiCompleter := taskgrpcclient.NewAICompleter(infraFleetClient)

	// project-service dependency: ProjectContextResolver's GetProjectContext
	// call (SimpleExecutor's profile-aware env injection, TASK-PRF-04-07/08)
	// AND its GetProject/ListRepos calls (AIDecompose's context bundle,
	// TASK-TG-02-04) — task-service never reads project-service's tables
	// directly. Also WorktreeProvisioner's repo_id resolution
	// (TASK-TG-04-02/SOL-TG-04). One dial, one client, reused by all three.
	projectConn, err := taskgrpcclient.Dial(cfg.ProjectServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing project-service: %w", err)
	}
	defer func() { _ = projectConn.Close() }()
	projectClient := projectv1.NewProjectServiceClient(projectConn)
	projectContextResolver := taskgrpcclient.NewProjectContextResolver(projectClient)

	// profileResolver dials the same tenant-service connection as
	// teamScopeResolver above — SimpleExecutor's profile-aware env
	// injection (TASK-PRF-04-07/08) is a second, independent use of that
	// already-open connection, not a new dial.
	profileResolver := taskgrpcclient.NewProfileResolver(tenantClient)
	simpleExecutor := taskgrpcclient.NewSimpleExecutor(
		repo, repo, projectExecutionResolver, infraFleetClient, profileResolver, projectContextResolver,
		repo, execOutputRelay,
	)

	aiProviderConn, err := taskgrpcclient.Dial(cfg.AIProviderServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing ai-provider-service: %w", err)
	}
	defer func() { _ = aiProviderConn.Close() }()
	aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)
	aiProviderContextResolver := taskgrpcclient.NewAIProviderContextResolver(aiProviderClient)

	// git-gateway-service dependency: TechStackDetector's ReadFile probes
	// (TASK-TG-02-03) and WorktreeProvisioner's CreateWorktree calls
	// (TASK-TG-04-02/SOL-TG-04) — both a genuine scope addition, flagged in
	// their own task Context sections.
	gitGatewayConn, err := taskgrpcclient.Dial(cfg.GitGatewayServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing git-gateway-service: %w", err)
	}
	defer func() { _ = gitGatewayConn.Close() }()
	gitGatewayClient := gitgatewayv1.NewGitGatewayServiceClient(gitGatewayConn)
	techStackDetector := taskgrpcclient.NewTechStackDetector(gitGatewayClient, projectExecutionResolver)
	// worktreeProvisioner implements Execute's reuse-or-create worktree step
	// (TASK-TG-04-02/03/SOL-TG-04) against git-gateway-service's existing
	// CreateWorktree saga, resolving repo_id itself via project-service.
	worktreeProvisioner := taskgrpcclient.NewWorktreeProvisioner(gitGatewayClient, projectClient)

	// workflow-service dial — WorkflowExecutor's Engine 3 dispatch (TASK-FT-002-03).
	workflowConn, err := taskgrpcclient.Dial(cfg.WorkflowServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing workflow-service: %w", err)
	}
	defer func() { _ = workflowConn.Close() }()
	workflowClient := workflowv1.NewWorkflowServiceClient(workflowConn)
	workflowExecutor := taskgrpcclient.NewWorkflowExecutor(workflowClient)

	// opaEvaluator loads/compiles the orca-authz bundle once per distinct
	// query string (common/policy.Evaluator's own cache) and is shared by
	// every ResolvePermission call for this process's lifetime.
	opaEvaluator := policy.NewEvaluator(cfg.OPABundlePath)
	if err := opaEvaluator.Warm(ctx, "data.orca.authz.task.allow"); err != nil {
		return fmt.Errorf("task-service: OPA bundle failed to load at startup (bundle path %q): %w", cfg.OPABundlePath, err)
	}
	opaClient := taskopaclient.New(opaEvaluator)

	// Audit-append client (TASK-BE-018/020, CR-RBAC-005) — ResolvePermission
	// uses this to record every OPA allow/deny decision to auth-service's
	// audit_log. Lazy dial (grpc.NewClient doesn't block on connect), same
	// convention as every other outbound client above.
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("dialing auth-service: %w", err)
	}
	defer func() { _ = authConn.Close() }()
	auditClient := auditclient.New(authv1.NewAuthServiceClient(authConn))

	eventPublisher := taskeventbus.NewPublisher(repo, logger)

	createTaskUC := usecase.NewCreateTask(repo)
	getTaskUC := usecase.NewGetTask(repo)
	addEdgeUC := usecase.NewAddEdge(repo)
	// resolvePermissionUC must be constructed before grantUC — Grant now
	// requires 'manage' access to a task before writing a new grant on it,
	// closing a live authorization gap (TASK-TG-03-01).
	resolvePermissionUC := usecase.NewResolvePermission(repo, repo, teamScopeResolver, opaClient, auditClient)
	grantUC := usecase.NewGrant(repo, resolvePermissionUC, eventPublisher)
	revokeGrantUC := usecase.NewRevokeGrant(repo, resolvePermissionUC, eventPublisher)
	listGrantsUC := usecase.NewListGrants(repo, resolvePermissionUC)
	// repo also implements usecase.ExecutionLinkRepository (adapter/postgres's
	// execution_links.go) — one row per Execute dispatch, across all three
	// engines (BE-SOL-001/CR-FLOW-TASK-001).
	executeTaskUC := usecase.NewExecuteTask(repo, repo, simpleExecutor, complexExecutor, workflowExecutor, resolvePermissionUC, worktreeProvisioner, projectExecutionResolver, SystemClock{}, repo)
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
	// reportExecutionResultUC is the shared inbound completion callback for
	// Engine 2 (orchestration-service) and Engine 3 (workflow-service),
	// TASK-FT-002-04/TASK-TG-04-05 — repo also implements
	// usecase.ExecutionLinkRepository. Called BY orchestration-service/
	// workflow-service only — see server.go's ReportTaskExecutionResult doc
	// comment for the flagged (unresolved) service-identity check this
	// handler is missing.
	reportExecutionResultUC := usecase.NewReportTaskExecutionResult(repo, repo)
	findTaskByNumberUC := usecase.NewFindTaskByNumber(repo)

	// Execution-status mirror consumer (BE-SOL-003/TASK-FT-003-05) —
	// subscribes orca.orchestration.task.statuschanged /
	// orca.workflow.step.completed (published by orchestration-service's
	// and workflow-service's own outbox relays, TASK-FT-003-01/-02/-03) to
	// keep execution_links.status_mirror (TASK-FT-001-01) roughly in sync
	// with the owning engine's real state, for CR-FLOW-TASK-003's Activity
	// Feed to read. If NATS is unreachable at startup, mirroring is simply
	// disabled — repo also implements usecase.ExecutionLinkRepository.
	mirrorExecutionStatusUC := usecase.NewMirrorExecutionStatus(repo)

	// Transactional-outbox relay (TASK-TG-03-07/TASK-PW-04-04/
	// TASK-AG-FLOWTASK-003): Grant/RevokeGrant/UpdateTask durably enqueue an
	// outbox row (internal/adapter/postgres's WriteOutboxEvent/
	// InsertOutboxEvent) and this relay is what actually gets those rows to
	// NATS — same "queue up unpublished until an operator restarts this
	// process" posture as usage-service's identical wiring if NATS is
	// unreachable at startup. The same connection also backs the
	// execution-status mirror consumer above.
	var outboxRelay *outbox.Relay
	pub, consumerBus, closeBus, err := commoneventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, execution-status mirroring and outbox publishing disabled", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		consumer := taskeventbus.NewConsumer(consumerBus, mirrorExecutionStatusUC)
		go consumer.Run(ctx, logger)

		if err := pub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure TASK stream", slog.Any("error", err))
		} else {
			outboxRelay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
		}
	}
	var outboxRelayWG sync.WaitGroup
	if outboxRelay != nil {
		outboxRelayWG.Add(1)
		go func() {
			defer outboxRelayWG.Done()
			outboxRelay.Run(ctx)
		}()
	}

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	taskv1.RegisterTaskServiceServer(grpcServer, taskgrpc.New(
		createTaskUC, getTaskUC, addEdgeUC, grantUC, resolvePermissionUC, executeTaskUC, hasActiveExecutionsUC,
		listTasksUC, updateTaskUC, deleteTaskUC, getDependenciesUC, aiDecomposeUC, aiApplyUC, generateAgentPromptUC,
		revokeGrantUC, listGrantsUC, createPublicLinkUC, revokePublicLinkUC, resolvePublicLinkUC,
		getSubtreeUC, recalculateProgressUC, addCommentUC, listCommentsUC, reportExecutionResultUC, findTaskByNumberUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})
	// outboxRelay (TASK-TG-03-07/SOL-PW-04/TASK-AG-FLOWTASK-003) is already
	// running by this point — see its wiring above. Grant-audit events
	// (Grant/RevokeGrant), task.* domain events (UpdateTask, SOL-PW-04), and
	// SimpleExecutor's agent_output_partial frames all flow through the SAME
	// relay/table, so there is exactly one to report health for here.
	if outboxRelay != nil {
		healthSrv.Register("nats", func() error { return nil }) // presence-only: a real liveness probe would ping the connection
	}

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
	// server on shutdown — same pattern usage-service's/orchestration-service's
	// main.go uses for their own outbox relay goroutines.
	outboxRelayWG.Wait()

	return nil
}
