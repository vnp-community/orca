// Command server is workflow-service's composition root — the only place
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
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/workflow-service/internal/config"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"

	workflowgrpc "github.com/stablyai/orca-go/services/workflow-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/infrafleetclient"
	workflowpostgres "github.com/stablyai/orca-go/services/workflow-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/stepexecutors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
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

	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	repo := workflowpostgres.New(pool)

	infraFleetConn, err := infrafleetclient.Dial(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service: %w", err)
	}
	defer func() { _ = infraFleetConn.Close() }()
	infraFleetClient := infrafleetv1.NewInfraFleetServiceClient(infraFleetConn)

	// StepExecutorRegistry wiring — all five step types, per
	// workflow-service.md §4: Condition and Webhook are real, in-process
	// implementations; Agent/Shell/Notification relay to infra-fleet-
	// service's generic Relay RPC (internal/adapter/infrafleetclient) —
	// see that package's doc comments for the best-effort method-name/
	// param-shape caveats (no live Dev Server Agent to verify against).
	registry := stepexecutors.NewRegistry()
	registry.Register(domain.StepTypeCondition, stepexecutors.NewConditionExecutor())
	registry.Register(domain.StepTypeWebhook, stepexecutors.NewWebhookExecutor(cfg.WebhookAllowlistHosts, &http.Client{Timeout: 30 * time.Second}))
	registry.Register(domain.StepTypeAgent, infrafleetclient.NewAgentExecutor(infraFleetClient))
	registry.Register(domain.StepTypeShell, infrafleetclient.NewShellExecutor(infraFleetClient))
	registry.Register(domain.StepTypeNotification, infrafleetclient.NewNotificationExecutor(infraFleetClient))

	createTemplateUC := usecase.NewCreateTemplate(repo)
	executeUC := usecase.NewExecute(repo, repo, repo, registry)
	getExecutionUC := usecase.NewGetExecution(repo)
	pauseExecutionUC := usecase.NewPauseExecution(repo)
	resumeExecutionUC := usecase.NewResumeExecution(repo)
	executeAdHocStepUC := usecase.NewExecuteAdHocStep(repo, repo, registry)
	hasActiveExecutionsUC := usecase.NewHasActiveExecutions(repo)
	cancelExecutionUC := usecase.NewCancelExecution(repo)
	listTemplatesUC := usecase.NewListTemplates(repo)
	resolveTemplateUC := usecase.NewResolveTemplate(repo)
	updateTemplateUC := usecase.NewUpdateTemplate(repo)
	recoverExecutionsUC := usecase.NewRecoverExecutions(repo, repo, repo, registry)

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

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	workflowv1.RegisterWorkflowServiceServer(grpcServer, workflowgrpc.New(
		createTemplateUC, executeUC, getExecutionUC, pauseExecutionUC, resumeExecutionUC, executeAdHocStepUC, hasActiveExecutionsUC,
		cancelExecutionUC, listTemplatesUC, resolveTemplateUC, updateTemplateUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})

	// Transactional-outbox relay (SOL-PW-04, TASK-PW-04-06): Execute's
	// runToCompletion/RecoverExecutions' finish durably enqueue an outbox
	// row in the SAME Postgres transaction as the execution's terminal
	// status write (internal/adapter/postgres.Repository.UpdateExecution)
	// — this relay is what actually gets those rows to NATS. Matches
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
		// before picking this name; do not rename it.
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
