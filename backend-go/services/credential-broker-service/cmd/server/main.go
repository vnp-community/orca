// Command server is credential-broker-service's composition root — the only
// place allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// This is the one service in the whole backend-go scaffold where
// common/secrets.NewClient() is wired for real, non-stub use: the
// Transit/KV v2 methods it exposes are exactly this service's reason to
// exist (credential-broker-service.md §6/§9).
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

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/credential-broker-service/internal/config"

	credentialgrpc "github.com/stablyai/orca-go/services/credential-broker-service/internal/adapter/grpc"
	credentialpostgres "github.com/stablyai/orca-go/services/credential-broker-service/internal/adapter/postgres"
	credentialvault "github.com/stablyai/orca-go/services/credential-broker-service/internal/adapter/vault"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("credential-broker-service exited with error", slog.Any("error", err))
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

	repo := credentialpostgres.New(pool)

	// Real Vault wiring — see this file's package doc comment and
	// internal/adapter/vault's doc comment. secrets.NewClient() reads
	// VAULT_ADDR/VAULT_TOKEN from the environment (local-dev static token;
	// production authenticates via the Kubernetes auth method through a
	// Vault Agent sidecar instead — see common/secrets.NewClient's doc
	// comment and this service's README "Known gaps").
	vaultClient, err := secrets.NewClient()
	if err != nil {
		return fmt.Errorf("creating vault client: %w", err)
	}
	store := credentialvault.New(vaultClient)

	// repo also implements usecase.TxRunner (RunInTx) — write/rotate/revoke
	// use it so their metadata mutation and audit append commit together;
	// see internal/usecase/ports.go's TxRunner doc comment.
	writeUC := usecase.NewWriteCredential(store, repo)
	resolveUC := usecase.NewResolveCredential(repo, repo, store)
	rotateUC := usecase.NewRotateCredential(repo, store, repo)
	revokeUC := usecase.NewRevokeCredential(repo, store, repo)
	getMetadataUC := usecase.NewGetCredentialMetadata(repo)
	resolveByOwnerUC := usecase.NewResolveCredentialByOwner(repo, repo, store)
	revokeByOwnerUC := usecase.NewRevokeCredentialByOwner(repo, store, repo)
	signVapidUC := usecase.NewSignVapidPayload(store)
	getMetadataByOwnerUC := usecase.NewGetCredentialMetadataByOwner(repo)
	listByCategoryUC := usecase.NewListCredentialsByCategory(repo)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	credentialbrokerv1.RegisterCredentialBrokerServiceServer(grpcServer, credentialgrpc.New(
		writeUC, resolveUC, rotateUC, revokeUC, getMetadataUC, resolveByOwnerUC, revokeByOwnerUC, signVapidUC,
		getMetadataByOwnerUC, listByCategoryUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})
	// Vault reachability gates readiness — per credential-broker-service.md
	// §8, this service fails closed (deny resolve/write/rotate) when Vault
	// is unreachable, so a pod that can't reach Vault should be pulled out
	// of rotation, not left serving requests it can only fail. See
	// internal/adapter/vault.SecretStore.Ping's doc comment for the
	// heuristic this check relies on.
	healthSrv.Register("vault", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return store.Ping(ctx)
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
		logger.Info("credential-broker-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("credential-broker-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
