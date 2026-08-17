// Command server is git-gateway-service's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Simpler than usage-service's: per git-gateway-service.md §5, this service
// owns no database, so there's no pgxpool to open/close/health-check and no
// migrations to run. It also has no eventbus wiring (§6: "git operations are
// synchronous request/response by nature").
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

	svcconfig "github.com/stablyai/orca-go/services/git-gateway-service/internal/config"

	gitgatewaygrpc "github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/localgit"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("git-gateway-service exited with error", slog.Any("error", err))
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

	// Outbound adapters. Both are stubs in this scaffold — see their doc
	// comments in internal/adapter/grpcclient for what real wiring against
	// infra-fleet-service needs. internal/adapter/localgit is real: it
	// shells out to the host's `git` binary for the host-local dispatch path.
	resolver := grpcclient.NewConnectionResolver()
	local := localgit.New()
	relay := grpcclient.NewRelayExecutor()

	getStatusUC := usecase.NewGetStatus(resolver, local, relay)
	getDiffUC := usecase.NewGetDiff(resolver, local, relay)
	commitUC := usecase.NewCommit(resolver, local, relay)
	pushUC := usecase.NewPush(resolver, local, relay)
	pullUC := usecase.NewPull(resolver, local, relay)
	generateCommitMessageUC := usecase.NewGenerateCommitMessage()

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	gitgatewayv1.RegisterGitGatewayServiceServer(grpcServer, gitgatewaygrpc.New(
		getStatusUC, getDiffUC, commitUC, pushUC, pullUC, generateCommitMessageUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	// No database, so no readiness checker depends on a connection pool —
	// per git-gateway-service.md §8, this service is horizontally scaled
	// precisely because it holds no such state. /readyz reports healthy as
	// soon as the process is up.
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
		logger.Info("git-gateway-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("git-gateway-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
