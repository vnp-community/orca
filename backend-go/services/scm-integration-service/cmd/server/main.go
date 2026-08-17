// Command server is scm-integration-service's composition root — the only
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

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/scm-integration-service/internal/config"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/azuredevops"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/bitbucket"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/credentialbroker"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/gitea"
	scmgithub "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/github"
	scmgitlab "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/gitlab"
	scmgrpc "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/providerregistry"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("scm-integration-service exited with error", slog.Any("error", err))
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

	// No database wired here: scm-integration-service owns no business data
	// (§5 — every issue/PR/MR read hits the provider's live API on every
	// call) and this scaffold doesn't implement the operational
	// rate_limit_cache/webhook_delivery_log tables yet — see README's
	// "Known gaps".

	registry := providerregistry.New(map[domain.ScmProvider]usecase.ScmProvider{
		domain.ScmProviderGitHub:      scmgithub.New(nil, cfg.GitHubBaseURL),
		domain.ScmProviderGitLab:      scmgitlab.New(nil, cfg.GitLabBaseURL),
		domain.ScmProviderBitbucket:   bitbucket.New(),
		domain.ScmProviderAzureDevOps: azuredevops.New(),
		domain.ScmProviderGitea:       gitea.New(),
	})

	// STUB — see internal/adapter/credentialbroker's package doc: this
	// resolves a fake token, not a real per-tenant OAuth credential.
	credentials := credentialbroker.NewStubResolver()

	listIssuesUC := usecase.NewListIssues(credentials, registry)
	createPullRequestUC := usecase.NewCreatePullRequest(credentials, registry)
	listPullRequestsUC := usecase.NewListPullRequests(credentials, registry)
	getRateLimitStatusUC := usecase.NewGetRateLimitStatus(credentials, registry)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	scmintegrationv1.RegisterScmIntegrationServiceServer(grpcServer, scmgrpc.New(
		listIssuesUC, createPullRequestUC, listPullRequestsUC, getRateLimitStatusUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()

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
		logger.Info("scm-integration-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("scm-integration-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
