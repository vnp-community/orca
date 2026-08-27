// Command server is auth-service's composition root — the only place
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

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/auth-service/internal/config"

	authbcrypt "github.com/stablyai/orca-go/services/auth-service/internal/adapter/bcrypt"
	authgrpc "github.com/stablyai/orca-go/services/auth-service/internal/adapter/grpc"
	authopaclient "github.com/stablyai/orca-go/services/auth-service/internal/adapter/opaclient"
	authpolicypublisher "github.com/stablyai/orca-go/services/auth-service/internal/adapter/policypublisher"
	authpostgres "github.com/stablyai/orca-go/services/auth-service/internal/adapter/postgres"
	authvault "github.com/stablyai/orca-go/services/auth-service/internal/adapter/vault"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("auth-service exited with error", slog.Any("error", err))
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

	repo := authpostgres.New(pool)
	hasher := authbcrypt.New(cfg.BcryptCost)
	clock := usecase.SystemClock{}

	// Real Vault wiring, mirroring credential-broker-service's cmd/server/
	// main.go pattern — secrets.NewClient() reads VAULT_ADDR/VAULT_TOKEN
	// from the environment (local-dev static token; production
	// authenticates via the Kubernetes auth method through a Vault Agent
	// sidecar instead — see common/secrets.NewClient's doc comment and this
	// service's README "Known gaps").
	vaultClient, err := secrets.NewClient()
	if err != nil {
		return fmt.Errorf("creating vault client: %w", err)
	}
	tokenSigner := authvault.New(vaultClient)
	// Fail startup loudly if the jwt-signing Transit key can't be
	// ensured — every JWT this service issues depends on it existing.
	if err := tokenSigner.Ensure(ctx); err != nil {
		return fmt.Errorf("ensuring jwt-signing transit key: %w", err)
	}

	// opaEvaluator loads/compiles the orca-authz bundle once per distinct
	// query string (common/policy.Evaluator's own cache) and is shared by
	// every requireAdminActor call for this process's lifetime.
	opaEvaluator := policy.NewEvaluator(cfg.OPABundlePath)
	opaClient := authopaclient.New(opaEvaluator)

	loginUC := usecase.NewLogin(repo, repo, repo, hasher, clock, cfg.SessionTTL)
	logoutUC := usecase.NewLogout(repo, repo, clock)
	validateSessionUC := usecase.NewValidateSession(repo, repo, clock)
	createUserUC := usecase.NewCreateUser(repo, repo, hasher, clock, opaClient)
	listUsersUC := usecase.NewListUsers(repo, opaClient)
	updateUserRoleUC := usecase.NewUpdateUserRole(repo, repo, clock, opaClient)
	revokeSessionUC := usecase.NewRevokeSession(repo, repo, repo, clock, opaClient)
	queryAuditLogUC := usecase.NewQueryAuditLog(repo, repo, opaClient)
	issueServiceTokenUC := usecase.NewIssueServiceToken(repo, tokenSigner, clock, cfg.ServiceTokenTTL)
	getJWKSUC := usecase.NewGetJWKS(tokenSigner)

	deactivateUserUC := usecase.NewDeactivateUser(repo, repo, clock, opaClient)
	reactivateUserUC := usecase.NewReactivateUser(repo, repo, clock, opaClient)
	listSessionsForUserUC := usecase.NewListSessionsForUser(repo, repo, opaClient)
	forceRevokeAllSessionsForUserUC := usecase.NewForceRevokeAllSessionsForUser(repo, repo, repo, clock, opaClient)

	// policyPublisher is a logging-only stub — no real OPA bundle-registry
	// integration exists in this codebase yet. See
	// internal/adapter/policypublisher's package doc comment.
	policyPublisher := authpolicypublisher.New(logger)
	createAccessPolicyUC := usecase.NewCreateAccessPolicy(repo, repo, clock, opaClient)
	getAccessPolicyUC := usecase.NewGetAccessPolicy(repo, repo, opaClient)
	listAccessPoliciesUC := usecase.NewListAccessPolicies(repo, repo, opaClient)
	updateAccessPolicyUC := usecase.NewUpdateAccessPolicy(repo, repo, policyPublisher, clock, opaClient)
	deleteAccessPolicyUC := usecase.NewDeleteAccessPolicy(repo, repo, opaClient)
	getAdminStatsUC := usecase.NewGetAdminStats(repo, repo, repo, clock, opaClient)

	listSessionsUC := usecase.NewListSessions(repo, repo, opaClient)
	updateUserUC := usecase.NewUpdateUser(repo, repo, clock, opaClient)

	// Runs once, before the server starts accepting traffic — see
	// internal/usecase/bootstrap.go's doc comment for why this isn't an
	// RPC. No-op unless BOOTSTRAP_TENANT_ID/BOOTSTRAP_ADMIN_EMAIL are set
	// AND no user already exists anywhere.
	bootstrap := usecase.NewBootstrap(repo, repo, hasher, clock)
	generatedPassword, err := bootstrap.EnsureAdmin(ctx, usecase.BootstrapConfig{
		TenantID: cfg.BootstrapTenantID,
		Email:    cfg.BootstrapAdminEmail,
		Password: cfg.BootstrapAdminPassword,
	}, logger)
	if err != nil {
		return fmt.Errorf("bootstrapping admin user: %w", err)
	}
	if generatedPassword != "" {
		// Printed once, at first boot, never stored — same contract the
		// old TS backend used for an auto-generated admin password.
		logger.Warn("auth-service: AUTO-GENERATED ADMIN PASSWORD (save this now, it will not be shown again)",
			slog.String("email", cfg.BootstrapAdminEmail),
			slog.String("password", generatedPassword))
	}

	// Session reaper: purges rows expired/revoked more than 7 days ago.
	// Operational hygiene, not correctness — domain.Session.IsValid already
	// enforces expiry at read time (auth-service.md §8's reaper NFR).
	reaper := usecase.NewReapExpiredSessions(repo, clock, 7*24*time.Hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // frequent enough per the reaper's "operational, not correctness" NFR
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := reaper.Execute(ctx); err != nil {
					logger.Error("session reaper failed", slog.Any("error", err))
				} else if n > 0 {
					logger.Info("session reaper purged rows", slog.Int64("count", n))
				}
			}
		}
	}()

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	authv1.RegisterAuthServiceServer(grpcServer, authgrpc.New(
		loginUC, logoutUC, validateSessionUC,
		createUserUC, listUsersUC, updateUserRoleUC, revokeSessionUC, queryAuditLogUC,
		issueServiceTokenUC, getJWKSUC,
		deactivateUserUC, reactivateUserUC, listSessionsForUserUC, forceRevokeAllSessionsForUserUC,
		createAccessPolicyUC, getAccessPolicyUC, listAccessPoliciesUC, updateAccessPolicyUC, deleteAccessPolicyUC,
		getAdminStatsUC,
		listSessionsUC, updateUserUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})
	// Vault reachability gates readiness — IssueServiceToken/GetJWKS can
	// only serve real signatures/JWKS while Vault is reachable, so a pod
	// that can't reach it should be pulled out of rotation. See
	// internal/adapter/vault.TokenSigner.Ping's doc comment.
	healthSrv.Register("vault", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return tokenSigner.Ping(ctx)
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
		logger.Info("auth-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("auth-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
