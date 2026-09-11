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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/issue-status-sync/internal/config"

	issuestatussynceventbus "github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/grpcclient"
	issuestatussyncmysql "github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/mysql"
	issuestatussyncpostgres "github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/postgres"
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
	// added purely to host the dedup cache (migrations/postgres|mysql/0001).
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

	// BE-DB-SOL-003: issue-status-sync's rollout of the usage-service
	// pilot's dialect-switch pattern — DATABASE_DSN's scheme picks the
	// adapter at startup, no separate DB_DIALECT env var (see
	// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-001.md §1).
	var processedEvents usecase.ProcessedEventStore
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		processedEvents = issuestatussyncpostgres.NewProcessedEventsStore(pool)
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
		processedEvents = issuestatussyncmysql.New(db)
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

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

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — that driver does NOT accept
// a URL directly (sql.Open("mysql", "mysql://...") fails outright).
// Identical to usage-service's cmd/server/main.go helper of the same name
// (BE-DB-SOL-001/002's pilot) — duplicated rather than shared because
// cmd/main packages aren't importable across services, not a deliberate
// divergence.
//
// Handles two input shapes:
//  1. host already wrapped as "tcp(host:port)" — the convention this
//     service's own DATABASE_DSN examples use (see common/testutil.StartMySQL's
//     return value) — passed through as-is after stripping the scheme,
//     since it's already valid driver syntax.
//  2. a plain "mysql://user:pass@host:port/db" URL — parsed with net/url
//     and re-wrapped. net/url.Parse CANNOT parse shape #1 itself (rejects
//     the literal parentheses in "tcp(host:port)" with "invalid port"),
//     which is why shape #1 is handled by a plain scheme-prefix strip
//     instead.
//
// parseTime=true is appended when absent, regardless of shape — required
// for TIMESTAMP columns to scan into time.Time/sql.NullTime.
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
