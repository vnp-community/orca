// Command server is ai-provider-service's composition root — the only place
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
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/ai-provider-service/internal/config"

	aiprovidergrpc "github.com/stablyai/orca-go/services/ai-provider-service/internal/adapter/grpc"
	aiprovidergrpcclient "github.com/stablyai/orca-go/services/ai-provider-service/internal/adapter/grpcclient"
	aiproviderpostgres "github.com/stablyai/orca-go/services/ai-provider-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("ai-provider-service exited with error", slog.Any("error", err))
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

	repo := aiproviderpostgres.New(pool)

	// Real credential-broker-service connection — Epic B
	// (docs/execution-plan.md §8). Insecure transport credentials here are
	// a local-dev/scaffold convenience only; production deploys terminate
	// mTLS via the service mesh sidecar, per
	// architecture/07-security-architecture.md. See
	// internal/adapter/grpcclient's package doc comment for the
	// SECURITY-CRITICAL constraint this client must uphold.
	brokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
	}
	defer func() { _ = brokerConn.Close() }()
	broker := aiprovidergrpcclient.New(brokerConn)

	// infra-fleet-service connection — mediates TestConnection's relay to
	// the execution plane (TASK-028); same insecure-transport-credentials
	// local-dev/scaffold convenience as brokerConn above.
	infraFleetConn, err := grpc.NewClient(cfg.InfraFleetServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service at %s: %w", cfg.InfraFleetServiceAddr, err)
	}
	defer func() { _ = infraFleetConn.Close() }()
	infraFleet := aiprovidergrpcclient.NewInfraFleetClient(infraFleetConn)

	createAccountUC := usecase.NewCreateAccount(repo, broker, uuid.NewString, nil)
	resolveProviderUC := usecase.NewResolveProvider(repo)
	rotateKeyUC := usecase.NewRotateKey(repo, broker)
	getUsageTodayUC := usecase.NewGetUsageToday(repo, nil)
	listAccountsUC := usecase.NewListAccounts(repo)
	updateAccountUC := usecase.NewUpdateAccount(repo)
	deleteAccountUC := usecase.NewDeleteAccount(repo)
	writeCredentialUC := usecase.NewWriteCredential(repo, broker)
	testConnectionUC := usecase.NewTestConnection(repo, infraFleet)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	aiproviderv1.RegisterAiProviderServiceServer(grpcServer, aiprovidergrpc.New(
		createAccountUC, resolveProviderUC, rotateKeyUC, getUsageTodayUC,
		listAccountsUC, updateAccountUC, deleteAccountUC, writeCredentialUC, testConnectionUC,
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
		logger.Info("ai-provider-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("ai-provider-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
