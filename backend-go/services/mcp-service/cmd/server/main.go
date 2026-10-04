// Command server is mcp-service's composition root — the only place that
// knows every layer (specs/backend-go/architecture/03).
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

	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	mcpauthclient "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/authclient"
	mcpgrpc "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/grpc"
	mcpmetrics "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/metrics"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/policyengine"
	mcppostgres "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/postgres"
	svcconfig "github.com/stablyai/orca-go/services/mcp-service/internal/config"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("mcp-service exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

// requirePostgres fails fast on any other dialect (D2): the schema relies on
// RLS, JSONB and partial indexes with no MySQL equivalent.
func requirePostgres(dsn string) error {
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}
	if caps.Dialect != dbcapability.DialectPostgres {
		return fmt.Errorf("mcp-service supports PostgreSQL only (got dialect %q): DATABASE_DSN must use postgres:// or postgresql://", caps.Dialect)
	}
	return nil
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

	// NATS is non-fatal at startup: outbox rows are written durably either way.
	pub, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, tracing.TraceStreamName, []string{tracing.TraceStreamSubjects}); err != nil {
			logger.WarnContext(ctx, "failed to ensure TRACE jetstream stream", slog.Any("error", err))
		}
	}

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint, tracing.WithTraceEventPublisher(pub))
	if err != nil {
		return fmt.Errorf("initializing tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	if err := requirePostgres(dsn); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(pingCtx)
	})

	repo := mcppostgres.New(pool)

	var relayWG sync.WaitGroup
	// /metrics on the health port: sessions_active is sampled from the DB every
	// 30s so a scrape never queries it (BE-MCP-SOL-015 section C).
	metricsSet := mcpmetrics.New(repo, logger).WithKillSwitches(repo)
	relayWG.Add(1)
	go func() {
		defer relayWG.Done()
		metricsSet.Run(ctx, 30*time.Second)
	}()
	if pub != nil {
		if err := pub.EnsureStream(ctx, "MCP", []string{"orca.mcp.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure MCP jetstream stream", slog.Any("error", err))
		} else {
			relay := outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
			healthSrv.Register("nats", func() error { return nil })
			relayWG.Add(1)
			go func() {
				defer relayWG.Done()
				relay.Run(ctx)
			}()
		}
	}

	defaults := usecase.Defaults{TenantEnabled: cfg.TenantDefaultEnabled, MaxTokenDays: cfg.DefaultMaxTokenDays}
	getServerInfoUC := usecase.NewGetServerInfo(repo, defaults)

	authzCfg, err := svcconfig.LoadAuthorization()
	if err != nil {
		return err
	}
	var mcpServer mcpv1.McpServiceServer = mcpgrpc.New(getServerInfoUC)
	// Governance needs auth-service for client standing and refresh-token
	// revocation; both stay nil (fail closed / skipped) when OAuth is off.
	var clientStatuses usecase.ClientStatusReader
	var tokenRevoker usecase.RefreshTokenRevoker
	var patSuspender usecase.PatSuspender
	if authzCfg.Enabled {
		authConn, err := mcpauthclient.Dial(authzCfg.AuthServiceAddr, authzCfg.InternalToken)
		if err != nil {
			return fmt.Errorf("dialing auth-service: %w", err)
		}
		defer func() { _ = authConn.Close() }()
		as := mcpauthclient.New(authv1.NewAuthServiceClient(authConn), authzCfg.AuthCallDeadline)
		clientStatuses, tokenRevoker = mcpauthclient.NewClientStatuses(as), mcpauthclient.NewGrantTokenRevoker(as)
		patSuspender = mcpauthclient.NewPatSuspender(as)
		clock := usecase.SystemClock{}
		consentCfg := usecase.ConsentConfig{ConsentTTL: authzCfg.ConsentTTL, Issuer: authzCfg.Issuer}
		reconcileUC := usecase.NewReconcileGrantRevocations(repo, as, clock)
		mcpServer = mcpgrpc.WithAuthorization(mcpgrpc.New(getServerInfoUC), mcpgrpc.AuthorizationUsecases{
			CreateConsent: usecase.NewCreateConsentRequest(repo, repo, as, clock, defaults, consentCfg),
			GetConsent:    usecase.NewGetConsentRequest(repo, clock),
			DecideConsent: usecase.NewDecideConsent(repo, as, clock, consentCfg),
			ListGrants:    usecase.NewListGrants(repo, as),
			RevokeGrant:   usecase.NewRevokeGrant(repo, as, clock),
			ListClients:   usecase.NewListOAuthClients(repo, as),
			SetStatus:     usecase.NewSetOAuthClientStatus(repo, as, repo, clock),
		})
		// Retries revocations that couldn't reach auth-service the first time.
		relayWG.Add(1)
		go func() {
			defer relayWG.Done()
			runGrantReconciler(ctx, logger, reconcileUC, authzCfg.ReconcileEvery, authzCfg.ReconcileBatch)
		}()
	}

	govCfg, err := svcconfig.LoadGovernance()
	if err != nil {
		return err
	}
	engine := policyengine.New(govCfg.BundlePath, logger)
	// A broken bundle must stop the service, not deny every call at runtime.
	if err := engine.Warm(ctx); err != nil {
		return fmt.Errorf("loading OPA bundle %q: %w", govCfg.BundlePath, err)
	}
	gov := buildGovernance(repo, engine, clientStatuses, tokenRevoker, defaults, govCfg, logger)
	if patSuspender != nil {
		gov.cleanup.WithPatSuspender(patSuspender)
	}
	mcpServer = mcpgrpc.WithGovernance(mcpServer, gov.usecases)
	mcpServer = mcpgrpc.WithPrompts(mcpServer, usecase.NewPromptAdmin(repo, usecase.SystemClock{}))
	sessCfg, err := svcconfig.LoadSessions()
	if err != nil {
		return err
	}
	mcpServer = wireSessions(ctx, &relayWG, logger, mcpServer, repo, sessCfg)
	gov.startWorkers(ctx, &relayWG, logger, govCfg.WorkerInterval, govCfg.ToolCallsRetention)
	extReg, err := buildExternalRegistry(repo, defaults, authzCfg, logger)
	if err != nil {
		return err
	}
	defer extReg.closeAll()
	extReg.startHealthWorker(ctx, &relayWG, logger)
	if cons != nil {
		relayWG.Add(1)
		go func() {
			defer relayWG.Done()
			gov.usecases.Hub.Run(ctx, cons, func(subject string, err error) {
				logger.WarnContext(ctx, "mcp event hub subscription ended", slog.String("subject", subject), slog.Any("error", err))
			})
		}()
	} else {
		logger.Warn("event bus unavailable: StreamEvents is empty and caches refresh only by TTL")
	}

	serverOpts := []grpc.ServerOption{grpcmw.ChainUnary(logger), grpcmw.StatsHandler()}
	if govCfg.InternalCallerToken != "" {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(internalcaller.Guard(govCfg.InternalCallerToken, append(append([]string{}, gatewayOnlyMethods...), registryInternalMethods...)...)))
		// Unary interceptors never see streams, so StreamEvents needs its own.
		serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(internalcaller.StreamGuard(govCfg.InternalCallerToken, gatewayOnlyStreamMethods...)))
	} else {
		logger.Warn("MCP_INTERNAL_CALLER_TOKEN is empty: gateway-only governance RPCs and StreamEvents rely on network policy alone")
	}
	grpcServer := grpc.NewServer(serverOpts...)
	mcpv1.RegisterMcpServiceServer(grpcServer, mcpServer)
	mcpv1.RegisterMcpRegistryServiceServer(grpcServer, extReg.server)
	reflection.Register(grpcServer)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: healthAndMetricsMux(healthSrv.Handler(), metricsSet.Handler()),
	}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			errCh <- fmt.Errorf("listening on grpc port: %w", err)
			return
		}
		logger.Info("mcp-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("mcp-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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

	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	relayWG.Wait()
	return nil
}

// runGrantReconciler periodically re-sends pending grant revocations to
// auth-service until ctx is cancelled.
func runGrantReconciler(ctx context.Context, logger *slog.Logger, uc *usecase.ReconcileGrantRevocations, every time.Duration, batch int) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := uc.Execute(ctx, batch); err != nil {
				logger.WarnContext(ctx, "grant revocation reconcile failed", slog.Any("error", err))
			} else if n > 0 {
				logger.InfoContext(ctx, "grant revocations propagated", slog.Int("count", n))
			}
		}
	}
}

// healthAndMetricsMux keeps /healthz and /readyz as they were and adds the
// Prometheus scrape endpoint on the same internal port.
func healthAndMetricsMux(health, metrics http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", health)
	mux.Handle("/metrics", metrics)
	return mux
}
