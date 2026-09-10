// Command server is usage-service's composition root — the only place
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

	svcconfig "github.com/stablyai/orca-go/services/usage-service/internal/config"

	usagegrpc "github.com/stablyai/orca-go/services/usage-service/internal/adapter/grpc"
	usagemysql "github.com/stablyai/orca-go/services/usage-service/internal/adapter/mysql"
	usagepostgres "github.com/stablyai/orca-go/services/usage-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/usage-service/internal/usecase"

	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("usage-service exited with error", slog.Any("error", err))
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

	// NATS connect moved ahead of tracing.Init (TASK-BE-FFT-008) so pub
	// exists in time to pass to tracing.WithTraceEventPublisher. Same
	// non-fatal degrade posture usage-service already had: rows still
	// write durably to the outbox even when NATS is down at startup.
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "TRACE", []string{"orca.*.trace.span"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure TRACE jetstream stream", slog.Any("error", err))
		}
	}

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint, tracing.WithTraceEventPublisher(pub))
	if err != nil {
		return fmt.Errorf("initializing tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	// Prefer the Vault-Agent-rendered credentials file over the raw env var
	// (see common/secrets.DatabaseCredentialsFromFile's doc comment) —
	// falls back to DATABASE_DSN itself when the file doesn't exist, which
	// is what local dev / this scaffold's testcontainers path still uses.
	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}

	healthSrv := health.New()

	// CR-DB-003: usage-service is the multi-dialect pilot — DATABASE_DSN's
	// scheme picks the adapter at startup, no separate DB_DIALECT env var
	// (see specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-001.md §1).
	// outboxStore is tracked separately from repo: usecase.Repository (the
	// interface repo is typed as, since it now varies by dialect) doesn't
	// include FetchUnpublished/MarkPublished — those are common/outbox.Store,
	// which both concrete Repository types also implement, for the relay
	// below.
	var (
		repo        usecase.Repository
		outboxStore outbox.Store
	)
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		pgRepo := usagepostgres.New(pool)
		repo, outboxStore = pgRepo, pgRepo
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
		myRepo := usagemysql.New(db)
		repo, outboxStore = myRepo, myRepo
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

	// Transactional-outbox relay (Epic G, docs/execution-plan.md): RecordUsageSession
	// durably enqueues an outbox row in the SAME Postgres transaction as the
	// session write (internal/adapter/postgres.Repository.SaveSession) —
	// this relay is what actually gets those rows to NATS. If NATS is
	// unreachable at startup, rows still get written durably (the request
	// path never touches NATS directly), they just queue up unpublished
	// until an operator restarts this process once NATS recovers — this
	// scaffold's Connect calls don't retry mid-run, same limitation every
	// other NATS-consuming service here already carries.
	var relay *outbox.Relay
	// pub already connected above (TASK-BE-FFT-008) — nil here iff
	// eventbus.Connect failed, same non-fatal degrade as before.
	if pub != nil {
		if err := pub.EnsureStream(ctx, "USAGE", []string{"orca.usage.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(outboxStore, pub, outbox.DefaultConfig, logger)
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

	recordUC := usecase.NewRecordUsageSession(repo)
	dailyUC := usecase.NewGetDailyUsage(repo)
	listUC := usecase.NewListSessions(repo)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	usagev1.RegisterUsageServiceServer(grpcServer, usagegrpc.New(recordUC, dailyUC, listUC))
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
		logger.Info("usage-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("usage-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
	// server on shutdown — same pattern notification-service's main.go
	// uses for its own background consumer.
	relayWG.Wait()

	return nil
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — that driver does NOT accept
// a URL directly (sql.Open("mysql", "mysql://...") fails outright).
//
// Handles two input shapes:
//  1. host already wrapped as "tcp(host:port)" — the convention this
//     service's own DATABASE_DSN examples use throughout
//     specs/backend-go/crs/v4/multi-database (see this task's Verify
//     section: `DATABASE_DSN=mysql://root:orca@tcp(localhost:3307)/usage`,
//     and common/testutil.StartMySQL's return value) — passed through
//     as-is after stripping the scheme, since it's already valid driver
//     syntax.
//  2. a plain "mysql://user:pass@host:port/db" URL — parsed with
//     net/url and re-wrapped. net/url.Parse CANNOT parse shape #1 itself
//     (confirmed empirically: it rejects the literal parentheses in
//     "tcp(host:port)" with "invalid port" — not documented behavior this
//     task's original net/url-based sketch accounted for), which is why
//     shape #1 is handled by a plain scheme-prefix strip instead.
//
// parseTime=true is appended when absent, regardless of shape — required
// for TIMESTAMP columns to scan into time.Time/sql.NullTime (discovered
// while implementing internal/adapter/mysql.Repository.ListSessions in
// TASK-BE-DB-005; not mentioned in this task's original DSN-conversion
// sketch, added here for the same reason the test adapter needs it).
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
