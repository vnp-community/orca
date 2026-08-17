// Command server is issue-tracking-service's composition root — the only
// place allowed to know about every layer at once, per
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
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/issue-tracking-service/internal/config"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/credential"
	issuetrackingeventbus "github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/eventbus"
	issuetrackinggrpc "github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/jira"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/linear"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/providerregistry"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("issue-tracking-service exited with error", slog.Any("error", err))
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

	// No Postgres pool: issue-tracking-service owns no database. Jira and
	// Linear remain the systems of record (design doc §2); every read is
	// live against the provider, nothing is cached as a queryable copy
	// (design doc §5's thin operational tables are a later addition, not
	// needed for ListIssues/CreateIssue/LinkIssue).

	registry := providerregistry.New().
		Register(domain.ProviderJira, jira.New(nil)).
		Register(domain.ProviderLinear, linear.New(nil))

	// STUB: see internal/adapter/credential's package doc — replace with a
	// real credential-broker-service client before deploying anywhere real
	// tenant secrets exist.
	credentialResolver := credential.NewStubResolver()

	healthSrv := health.New()

	var publisher usecase.EventPublisher
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, continuing without event publishing", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "ISSUETRACKING", []string{"orca.issuetracking.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			publisher = issuetrackingeventbus.New(pub)
			healthSrv.Register("nats", func() error { return nil }) // presence-only: a real liveness probe would ping the connection
		}
	}

	listIssuesUC := usecase.NewListIssues(registry, credentialResolver)
	createIssueUC := usecase.NewCreateIssue(registry, credentialResolver)
	linkIssueUC := usecase.NewLinkIssue(publisher)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	issuetrackingv1.RegisterIssueTrackingServiceServer(grpcServer, issuetrackinggrpc.New(listIssuesUC, createIssueUC, linkIssueUC))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

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
		logger.Info("issue-tracking-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("issue-tracking-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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

	return nil
}
