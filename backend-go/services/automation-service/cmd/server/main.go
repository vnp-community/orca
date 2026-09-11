// Command server is automation-service's composition root — the only place
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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/dbcapability"
	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/automation-service/internal/config"

	automationeventbus "github.com/stablyai/orca-go/services/automation-service/internal/adapter/eventbus"
	automationgrpc "github.com/stablyai/orca-go/services/automation-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/grpc/interceptors"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/grpcclient"
	automationmysql "github.com/stablyai/orca-go/services/automation-service/internal/adapter/mysql"
	automationpostgres "github.com/stablyai/orca-go/services/automation-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/scheduler"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("automation-service exited with error", slog.Any("error", err))
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

	// Prefer the Vault-Agent-rendered credentials file over the raw env var
	// (see common/secrets.DatabaseCredentialsFromFile's doc comment) —
	// falls back to DATABASE_DSN itself when the file doesn't exist, which
	// is what local dev / this scaffold's testcontainers path still uses.
	// Replaces the previous direct cfg.DatabaseDSN read (CR-DB-002/
	// CR-DB-003, BE-DB-SOL-012 — closes this service's own "Vault (common/
	// secrets) is not wired here" README gap, same as usage-service/
	// annotation-service's identical rollout step).
	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}

	healthSrv := health.New()

	// CR-DB-003: DATABASE_DSN's scheme picks the adapter at startup, no
	// separate DB_DIALECT env var — same factory pattern as usage-service
	// (the multi-dialect pilot), see
	// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-001.md §1.
	// automationRepo/claimer and runRepo/outboxStore are tracked as
	// separate-but-same-value variable pairs (mirroring usage-service's
	// repo/outboxStore split): the usecase.AutomationRepository/
	// AutomationRunRepository interfaces don't themselves embed
	// usecase.DueAutomationClaimer/common/outbox.Store, so scheduler.New and
	// outbox.NewRelay below need their own narrower-typed handles onto the
	// exact same concrete repository.
	var (
		automationRepo usecase.AutomationRepository
		claimer        usecase.DueAutomationClaimer
		runRepo        usecase.AutomationRunRepository
		outboxStore    outbox.Store
	)
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		pgAutomationRepo := automationpostgres.NewAutomationRepository(pool)
		runCompletedPublisher := automationeventbus.NewRunCompletedPublisher()
		pgRunRepo := automationpostgres.NewAutomationRunRepository(pool, runCompletedPublisher)
		automationRepo, claimer, runRepo, outboxStore = pgAutomationRepo, pgAutomationRepo, pgRunRepo, pgRunRepo
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
		myAutomationRepo := automationmysql.NewAutomationRepository(db)
		myRunRepo := automationmysql.NewAutomationRunRepository(db)
		automationRepo, claimer, runRepo, outboxStore = myAutomationRepo, myAutomationRepo, myRunRepo, myRunRepo
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

	// Real gRPC connection to workflow-service — RunNow's whole reason for
	// existing (see automation-service.md §2/§6). Insecure transport
	// credentials here are a local-dev/scaffold convenience only; production
	// deploys terminate mTLS via the service mesh sidecar, per
	// architecture/07-security-architecture.md, not a disabled-security
	// choice made in application code.
	workflowConn, err := grpc.NewClient(cfg.WorkflowServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("dialing workflow-service at %s: %w", cfg.WorkflowServiceAddr, err)
	}
	defer func() { _ = workflowConn.Close() }()
	workflowExecutor := grpcclient.New(workflowConn)

	// CR-AUTO-003/TASK-BE-AUTO-006 + TASK-BE-AUTO-004's rewire: RunNow now
	// dispatches through ExecuteAutomationChain internally (see
	// usecase.NewRunNow's doc comment), so create_pr action dispatch needs
	// a real PullRequestCreator here — same insecure-transport-credentials
	// local-dev/scaffold convention as workflowConn above (production
	// terminates mTLS via the service mesh sidecar).
	scmConn, err := grpc.NewClient(cfg.ScmIntegrationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("dialing scm-integration-service at %s: %w", cfg.ScmIntegrationServiceAddr, err)
	}
	defer func() { _ = scmConn.Close() }()
	pullRequestCreator := grpcclient.NewScmClient(scmConn)

	createAutomationUC := usecase.NewCreateAutomation(automationRepo)
	runNowUC := usecase.NewRunNow(automationRepo, runRepo, workflowExecutor, usecase.WithPullRequestCreator(pullRequestCreator))
	listRunsUC := usecase.NewListRuns(runRepo)
	handleExternalTriggerUC := usecase.NewHandleExternalTrigger(runNowUC)
	listAutomationsUC := usecase.NewListAutomations(automationRepo)
	updateAutomationUC := usecase.NewUpdateAutomation(automationRepo)
	deleteAutomationUC := usecase.NewDeleteAutomation(automationRepo)
	writeCleanupReportUC := usecase.NewWriteCleanupReport(runRepo)
	handleEventTriggerUC := usecase.NewHandleEventTrigger(automationRepo, runNowUC, logger)

	// TASK-BE-AUTO-008: a second ChainUnaryInterceptor option composes with
	// grpcmw.ChainUnary's (grpc-go appends chainUnaryInts across multiple
	// options, executed in registration order) — this stays
	// automation-service-local, not a common/grpcmw change, matching
	// credential-broker-service's precedent of not extending the shared
	// interceptor stack for a single service's own gate.
	grpcServer := grpc.NewServer(
		grpcmw.ChainUnary(logger),
		grpc.ChainUnaryInterceptor(interceptors.RequireTenantForExternalTrigger()),
		grpcmw.StatsHandler(),
	)
	automationv1.RegisterAutomationServiceServer(grpcServer, automationgrpc.New(
		createAutomationUC, runNowUC, listRunsUC, handleExternalTriggerUC,
		listAutomationsUC, updateAutomationUC, deleteAutomationUC, writeCleanupReportUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	// In-process scheduler ticker — see automation-service.md §7. Every
	// replica runs one; claimer (the same concrete repository as
	// automationRepo, see the dialect switch above) implements
	// usecase.DueAutomationClaimer's SKIP LOCKED claim query on both
	// dialects, so concurrent replicas' ticks never dispatch the same due
	// occurrence twice. Started as a goroutine sharing the same top-level
	// shutdown ctx every other goroutine here watches.
	schedulerTicker := scheduler.New(claimer, runNowUC, cfg.SchedulerInterval, cfg.SchedulerBatchSize, logger)
	go schedulerTicker.Run(ctx)

	// Transactional-outbox relay for orca.automation.run.completed
	// (TASK-AT-02-04): UpdateStatus durably enqueues an outbox row in the
	// SAME Postgres transaction as the terminal status write
	// (internal/adapter/postgres.AutomationRunRepository.UpdateStatus) —
	// this relay is what actually gets those rows to NATS. Also wires the
	// durable event-trigger consumer (TASK-AT-03-05): if NATS is
	// unreachable at startup, both are skipped and this replica logs a
	// warning rather than failing to start — matching usage-service's same
	// "outbox rows queue up until a future restart" posture.
	var relay *outbox.Relay
	pub, sub, closeBus, err := commoneventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart and event triggers won't fire", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "AUTOMATION", []string{"orca.automation.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(outboxStore, pub, outbox.DefaultConfig, logger)
		}
		eventConsumer := automationeventbus.NewConsumer(sub, handleEventTriggerUC)
		go eventConsumer.Run(ctx, logger)
	}

	var relayWG sync.WaitGroup
	if relay != nil {
		relayWG.Add(1)
		go func() {
			defer relayWG.Done()
			relay.Run(ctx)
		}()
	}

	// healthSrv itself (and its "postgres"/"mysql" dialect check) was
	// created earlier, alongside the dialect switch — only the
	// workflow-service dependency check is registered here.
	healthSrv.Register("workflow-service", func() error {
		state := workflowConn.GetState()
		// Idle/Connecting are not failures — the connection is lazy and
		// established on first use; only a confirmed-bad state should pull
		// this replica out of readiness.
		if state.String() == "TRANSIENT_FAILURE" {
			return fmt.Errorf("workflow-service connection state: %s", state)
		}
		return nil
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
		logger.Info("automation-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("automation-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
	// standards/production-readiness-checklist.md. schedulerTicker.Done()
	// waits for any in-flight tick to finish committing its claim batch
	// rather than abandoning it mid-dispatch — ctx is already cancelled at
	// this point (it's the same ctx Run watches), so this only waits for
	// work already underway, never starts new work.
	grpcServer.GracefulStop()
	<-schedulerTicker.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	return nil
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — that driver does NOT accept
// a URL directly (sql.Open("mysql", "mysql://...") fails outright). Same
// two-input-shape handling as usage-service's (the pilot) toMySQLDriverDSN
// — see that copy's doc comment for the full "tcp(host:port)" vs plain-URL
// rationale — with one addition specific to this service:
// clientFoundRows=true is always appended. automation-service's
// AutomationRepository.Update and AutomationRunRepository.UpdateStatus both
// decide "not found" from sql.Result.RowsAffected() == 0 (mirroring
// internal/adapter/postgres 1:1) — go-sql-driver/mysql's DEFAULT
// RowsAffected() semantics count only rows whose VALUES actually changed,
// so a no-op retry (identical field values) would misreport "not found"
// (the exact pitfall BE-DB-SOL-005 §3.1 documents for annotation-service's
// UpdateAnnotation, which that adapter fixed with an extra SELECT per
// call instead). clientFoundRows=true sets MySQL's CLIENT_FOUND_ROWS
// connection flag, which restores Postgres's "matched rows" RowsAffected()
// semantics for the WHOLE connection — a driver-level fix instead of a
// per-method workaround, verified against real MySQL 8 in
// TestAutomationRepository_Update_NoopRetryStillSucceeds
// (internal/adapter/mysql/repository_test.go).
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
		driverDSN += dsnParamSep(driverDSN) + "parseTime=true"
	}
	if !strings.Contains(driverDSN, "clientFoundRows=") {
		driverDSN += dsnParamSep(driverDSN) + "clientFoundRows=true"
	}
	return driverDSN, nil
}

// dsnParamSep returns the correct separator ("?" for the first query param,
// "&" for every subsequent one) for appending another key=value pair to
// driverDSN.
func dsnParamSep(driverDSN string) string {
	if strings.Contains(driverDSN, "?") {
		return "&"
	}
	return "?"
}
