// Command server is issue-status-sync's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Unlike every other service in this codebase, issue-status-sync exposes NO
// inbound gRPC surface at all — it is a pure fan-in event consumer
// (SOL-PI-03): it subscribes to project-service's/scm-integration-service's
// outbox-published worktree/PR lifecycle events and calls
// issue-tracking-service/scm-integration-service to sync the linked issue's
// status. There is no issuestatussync.proto because nothing calls this
// service synchronously.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/issue-status-sync/internal/config"

	issuestatussynceventbus "github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("issue-status-sync exited with error", slog.Any("error", err))
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

	// issuestatussync.processed_events — this service's only database,
	// added purely to host the dedup cache (migrations/0001).
	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()
	processedEvents := postgres.NewProcessedEventsStore(pool)

	// Outbound clients — issue-tracking-service/scm-integration-service for
	// the actual status write, project-service for the BR-PI-07 opt-out
	// re-check.
	issueTrackingConn, err := grpc.NewClient(cfg.IssueTrackingServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing issue-tracking-service: %w", err)
	}
	defer func() { _ = issueTrackingConn.Close() }()
	issueTrackingClient := grpcclient.NewIssueTrackingClient(issuetrackingv1.NewIssueTrackingServiceClient(issueTrackingConn))

	scmConn, err := grpc.NewClient(cfg.SCMIntegrationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing scm-integration-service: %w", err)
	}
	defer func() { _ = scmConn.Close() }()
	scmClient := grpcclient.NewScmClient(scmintegrationv1.NewScmIntegrationServiceClient(scmConn))

	projectConn, err := grpc.NewClient(cfg.ProjectServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing project-service: %w", err)
	}
	defer func() { _ = projectConn.Close() }()
	projectClient := grpcclient.NewProjectClient(projectv1.NewProjectServiceClient(projectConn))

	syncIssueStatusUC := usecase.NewSyncIssueStatus(issueTrackingClient, scmClient, projectClient, processedEvents, logger)

	_, consumer, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		return fmt.Errorf("connecting to nats: %w", err)
	}
	defer func() { _ = closeBus() }()
	subscriber := issuestatussynceventbus.New(consumer, syncIssueStatusUC, logger)

	var consumerWG sync.WaitGroup
	consumerWG.Add(1)
	consumerErrCh := make(chan error, 1)
	go func() {
		defer consumerWG.Done()
		if err := subscriber.Run(ctx); err != nil {
			consumerErrCh <- err
		}
	}()

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
		logger.Info("issue-status-sync http (health) listening", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight event handling")
	case err := <-consumerErrCh:
		return fmt.Errorf("event consumer: %w", err)
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	consumerWG.Wait()

	return nil
}
