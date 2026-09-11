// Command server is workflow-service's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/workflow-service/internal/config"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"

	workflowgrpc "github.com/stablyai/orca-go/services/workflow-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/infrafleetclient"
	workflowmysql "github.com/stablyai/orca-go/services/workflow-service/internal/adapter/mysql"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/opachecker"
	workflowpostgres "github.com/stablyai/orca-go/services/workflow-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/providerresolver"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/serverresolver"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/serviceclients"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/stepexecutors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/taskclient"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("workflow-service exited with error", slog.Any("error", err))
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

	// Prefer the Vault-Agent-rendered credentials file over the raw env var
	// (see common/secrets.DatabaseCredentialsFromFile's doc comment) —
	// falls back to DATABASE_DSN itself when the file doesn't exist, which
	// is what local dev / this service's testcontainers path still uses.
	// Replaces the previous direct cfg.DatabaseDSN read (CR-DB-002/
	// CR-DB-003, BE-DB-SOL-013 — closes this service's own former "Vault
	// wiring not wired in" gap, same as every other rolled-out service).
	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}

	healthSrv := health.New()

	// workflowStore is the union of every port internal/adapter/postgres
	// and internal/adapter/mysql's Repository types both implement — this
	// composition root passes the SAME `repo` identifier to every
	// TemplateRepository/ExecutionRepository/StepExecutionRepository/
	// outbox.Store call site below (unchanged from before this dialect
	// switch was introduced), so `repo`'s static type after the switch
	// must satisfy all four at once.
	var (
		repo          workflowStore
		approvalStore usecase.ApprovalRepository
	)
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		pgRepo := workflowpostgres.New(pool)
		repo = pgRepo
		approvalStore = workflowpostgres.NewApprovalStore(pool)
		healthSrv.Register("postgres", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return pool.Ping(pingCtx)
		})
	case dbcapability.DialectMySQL:
		driverDSN, err := toMySQLDriverDSN(dsn)
		if err != nil {
			return fmt.Errorf("converting mysql dsn: %w", err)
		}
		db, err := sql.Open("mysql", driverDSN)
		if err != nil {
			return fmt.Errorf("connecting to mysql: %w", err)
		}
		defer db.Close()
		myRepo := workflowmysql.New(db)
		repo = myRepo
		approvalStore = workflowmysql.NewApprovalStore(db)
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

	infraFleetConn, err := infrafleetclient.Dial(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service: %w", err)
	}
	defer func() { _ = infraFleetConn.Close() }()
	infraFleetClient := infrafleetv1.NewInfraFleetServiceClient(infraFleetConn)

	// NEW — tenant-service dial for AgentExecutor's profile-aware env
	// injection (TASK-PRF-04-05/06). infrafleetclient.Dial is reused (same
	// insecure-transport-credentials dial helper, this package's only Dial
	// func) rather than duplicating it per remote service.
	tenantConn, err := infrafleetclient.Dial(cfg.TenantServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing tenant-service: %w", err)
	}
	defer func() { _ = tenantConn.Close() }()
	tenantClient := tenantv1.NewTenantServiceClient(tenantConn)

	// project-service is dialed ONCE and the raw client shared across every
	// port that talks to it (ProjectContextResolver, ServerResolver's
	// "project:<id>" Target resolution, and CleanupWorktreesStepExecutor's
	// candidate-worktree listing below) — all three take the same
	// projectv1.ProjectServiceClient shape, so there is no reason to open
	// three separate connections to the same address.
	projectConn, err := infrafleetclient.Dial(cfg.ProjectServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing project-service: %w", err)
	}
	defer func() { _ = projectConn.Close() }()
	projectClient := projectv1.NewProjectServiceClient(projectConn)

	aiProviderConn, err := infrafleetclient.Dial(cfg.AIProviderServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing ai-provider-service: %w", err)
	}
	defer func() { _ = aiProviderConn.Close() }()
	aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)

	authConn, err := infrafleetclient.Dial(cfg.AuthServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing auth-service: %w", err)
	}
	defer func() { _ = authConn.Close() }()
	authClient := authv1.NewAuthServiceClient(authConn)

	// task-service dial — TaskClient's Engine 3 completion callback
	// (BE-SOL-002/TASK-FT-002-05). Reuses infrafleetclient.Dial (this
	// service's existing shared insecure-dial helper) rather than
	// duplicating the boilerplate a third way.
	taskConn, err := infrafleetclient.Dial(cfg.TaskServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing task-service: %w", err)
	}
	defer func() { _ = taskConn.Close() }()
	taskClient := taskclient.New(taskv1.NewTaskServiceClient(taskConn))

	profileResolver := infrafleetclient.NewProfileResolver(tenantClient)
	projectContextResolver := infrafleetclient.NewProjectContextResolver(projectClient)
	// ServerResolver turns a step's Target string into a connectionId —
	// see domain.AgentStepConfig.Target's doc comment for the four accepted
	// shapes and internal/adapter/serverresolver's doc comment.
	resolver := serverresolver.New(projectClient, infraFleetClient)
	// ProviderResolver picks which ai-provider-service account an Agent
	// step uses — see internal/adapter/providerresolver's doc comment.
	provider := providerresolver.New(aiProviderClient)
	// OPAChecker answers "is this user an admin" for BUG-WF-03's
	// publish-approval gate — see internal/adapter/opachecker's doc comment.
	opaChecker := opachecker.New(authClient)

	// StepExecutorRegistry wiring — all eight step types, per
	// workflow-service.md §4: Condition and Webhook are real, in-process
	// implementations; Agent/Shell/Notification relay to infra-fleet-
	// service's generic Relay RPC (internal/adapter/infrafleetclient) —
	// see that package's doc comments for the best-effort method-name/
	// param-shape caveats (no live Dev Server Agent to verify against).
	// Action/Parallel (TASK-WF-02-07) round out the proto's StepType enum.
	registry := stepexecutors.NewRegistry()
	registry.Register(domain.StepTypeCondition, stepexecutors.NewConditionExecutor())
	registry.Register(domain.StepTypeWebhook, stepexecutors.NewWebhookExecutor(cfg.WebhookAllowlistHosts, &http.Client{Timeout: 30 * time.Second}))
	registry.Register(domain.StepTypeAgent, infrafleetclient.NewAgentExecutor(infraFleetClient, resolver, provider, profileResolver, projectContextResolver))
	registry.Register(domain.StepTypeShell, infrafleetclient.NewShellExecutor(infraFleetClient, resolver))
	registry.Register(domain.StepTypeNotification, infrafleetclient.NewNotificationExecutor(infraFleetClient, resolver))
	// CR-AUTO-003/TASK-BE-AUTO-005
	registry.Register(domain.StepTypeCommitPush, infrafleetclient.NewGitCommitPushExecutor(infraFleetClient))
	registry.Register(domain.StepTypeAction, stepexecutors.NewActionExecutor())
	// Two-phase init: ParallelExecutor needs a reference back to the SAME
	// registry it's about to be registered into (to recursively resolve
	// each sub-step's own executor) — see ParallelExecutor's doc comment.
	parallelExecutor := stepexecutors.NewParallelExecutor()
	parallelExecutor.SetRegistry(registry)
	registry.Register(domain.StepTypeParallel, parallelExecutor)

	// STEP_TYPE_CLEANUP_WORKTREES (BL-AT-04, TASK-AT-04-05) — a further
	// StepExecutor, in-process like Condition/Webhook (no execution-plane
	// relay needed), but with its own two further outbound dependency
	// edges: git-gateway-service (the actual delete, with BR-AT-11/BR-AT-12
	// enforced server-side) and automation-service (BR-AT-14's audit
	// report) — project-service reuses the projectClient dialed above.
	gitGatewayConn, err := serviceclients.Dial(cfg.GitGatewayServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing git-gateway-service: %w", err)
	}
	defer func() { _ = gitGatewayConn.Close() }()
	automationConn, err := serviceclients.Dial(cfg.AutomationServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing automation-service: %w", err)
	}
	defer func() { _ = automationConn.Close() }()

	cleanupProjectClient := serviceclients.NewProjectClient(projectClient)
	gitGatewayClient := serviceclients.NewGitGatewayClient(gitgatewayv1.NewGitGatewayServiceClient(gitGatewayConn))
	cleanupAuditClient := serviceclients.NewCleanupAuditClient(automationv1.NewAutomationServiceClient(automationConn))
	registry.Register(domain.StepTypeCleanupWorktrees, usecase.NewCleanupWorktreesStepExecutor(cleanupProjectClient, gitGatewayClient, cleanupAuditClient))

	createTemplateUC := usecase.NewCreateTemplate(repo)
	executeUC := usecase.NewExecute(repo, repo, repo, registry, taskClient)
	getExecutionUC := usecase.NewGetExecution(repo)
	pauseExecutionUC := usecase.NewPauseExecution(repo)
	resumeExecutionUC := usecase.NewResumeExecution(repo)
	executeAdHocStepUC := usecase.NewExecuteAdHocStep(repo, repo, registry)
	hasActiveExecutionsUC := usecase.NewHasActiveExecutions(repo)
	cancelExecutionUC := usecase.NewCancelExecution(repo)
	listTemplatesUC := usecase.NewListTemplates(repo)
	resolveTemplateUC := usecase.NewResolveTemplate(repo)
	updateTemplateUC := usecase.NewUpdateTemplate(repo)
	cloneTemplateUC := usecase.NewCloneTemplate(resolveTemplateUC, repo)
	listExecutionsUC := usecase.NewListExecutions(repo)
	publishTemplateUC := usecase.NewPublishTemplate(repo, approvalStore, opaChecker)
	resolveApprovalUC := usecase.NewResolveApproval(approvalStore, opaChecker)
	listPendingApprovalsUC := usecase.NewListPendingApprovals(approvalStore, opaChecker)
	generateShareLinkUC := usecase.NewGenerateShareLink(repo)
	rateTemplateUC := usecase.NewRateTemplate(repo)
	previewSharedTemplateUC := usecase.NewPreviewSharedTemplate(repo)
	importSharedTemplateUC := usecase.NewImportSharedTemplate(repo, resolveTemplateUC)
	recoverExecutionsUC := usecase.NewRecoverExecutions(repo, repo, repo, registry, taskClient)

	// Boot-time recovery scan (workflow-service.md §8: "before accepting
	// new Execute calls"), run every time this process boots, not gated
	// behind any flag. Runs synchronously here — but Execute itself only
	// blocks on the (fast, indexed) listing + DAG-reconstruction work; each
	// recovered execution's actual wave dispatch is handed to its own
	// detached background goroutine (see RecoverExecutions.Execute's doc
	// comment), so a slow recovered step cannot delay the gRPC/HTTP
	// listeners below from coming up.
	if err := recoverExecutionsUC.Execute(ctx); err != nil {
		return fmt.Errorf("recovering in-flight workflow executions: %w", err)
	}

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	workflowv1.RegisterWorkflowServiceServer(grpcServer, workflowgrpc.New(
		createTemplateUC, executeUC, getExecutionUC, pauseExecutionUC, resumeExecutionUC, executeAdHocStepUC, hasActiveExecutionsUC,
		cancelExecutionUC, listTemplatesUC, resolveTemplateUC, updateTemplateUC, cloneTemplateUC, listExecutionsUC,
		publishTemplateUC, resolveApprovalUC, listPendingApprovalsUC,
		generateShareLinkUC, previewSharedTemplateUC, importSharedTemplateUC, rateTemplateUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	// Transactional-outbox relay (SOL-PW-04/TASK-PW-04-06, extended by
	// BE-SOL-003/TASK-FT-003-03 for step-level events): Execute's
	// runToCompletion/RecoverExecutions' finish (execution-level) and
	// waveDispatcher.dispatchStep (step-level) all durably enqueue an
	// outbox row in the SAME Postgres transaction as the write they
	// describe (internal/adapter/postgres.Repository.UpdateExecution /
	// UpdateStepExecution) — this ONE relay drains both into NATS. Matches
	// usage-service's/task-service's identical graceful-degradation
	// posture: NATS unavailable at startup does not fail service startup,
	// outbox rows queue durably in Postgres and drain on a future restart
	// once NATS is reachable.
	var relay *outbox.Relay
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		// Stream name "WORKFLOW" matches notification-service's
		// ALREADY-WIRED SubjectBinding{StreamName: "WORKFLOW", ...} —
		// verified present in
		// services/notification-service/internal/adapter/eventbus/consumer.go
		// before picking this name; do not rename it. Also covers
		// step-level subjects (orca.workflow.step.*) — no second stream.
		if err := pub.EnsureStream(ctx, "WORKFLOW", []string{"orca.workflow.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure WORKFLOW stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
			healthSrv.Register("nats", func() error { return nil }) // presence-only: a real liveness probe would ping the connection
		}
	}

	var relayWG sync.WaitGroup
	if relay != nil {
		relayWG.Add(1)
		go func() {
			defer relayWG.Done()
			relay.Run(ctx)
		}()
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
		logger.Info("workflow-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("workflow-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
	// server on shutdown — same pattern usage-service's/task-service's
	// main.go use for their own outbox relay goroutines.
	relayWG.Wait()

	return nil
}

// workflowStore is the union of every port internal/adapter/postgres.Repository
// and internal/adapter/mysql.Repository both implement — see run's `repo`
// declaration for why a single variable needs to satisfy all four
// interfaces simultaneously (CR-DB-002/CR-DB-003, BE-DB-SOL-013).
type workflowStore interface {
	usecase.TemplateRepository
	usecase.ExecutionRepository
	usecase.StepExecutionRepository
	outbox.Store
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — that driver does NOT accept
// a URL directly (sql.Open("mysql", "mysql://...") fails outright).
// Identical to usage-service's toMySQLDriverDSN (the multi-dialect pilot)
// — this conversion is dialect-plumbing, not workflow-service-specific,
// see that copy's doc comment for the two input shapes handled and why.
func toMySQLDriverDSN(dsn string) (string, error) {
	rest, ok := strings.CutPrefix(dsn, "mysql://")
	if !ok {
		rest, ok = strings.CutPrefix(dsn, "tidb://")
	}
	if !ok {
		return "", fmt.Errorf("toMySQLDriverDSN: dsn %q has neither mysql:// nor tidb:// scheme", dsn)
	}

	driverDSN := rest
	if !strings.Contains(rest, "@tcp(") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("toMySQLDriverDSN: parsing dsn: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("toMySQLDriverDSN: dsn %q has no host", dsn)
		}
		userinfo := ""
		if u.User != nil {
			userinfo = u.User.String() + "@"
		}
		driverDSN = fmt.Sprintf("%stcp(%s)%s", userinfo, u.Host, u.Path)
		if u.RawQuery != "" {
			driverDSN += "?" + u.RawQuery
		}
	}

	if !strings.Contains(driverDSN, "parseTime=") {
		sep := "?"
		if strings.Contains(driverDSN, "?") {
			sep = "&"
		}
		driverDSN += sep + "parseTime=true"
	}
	return driverDSN, nil
}
