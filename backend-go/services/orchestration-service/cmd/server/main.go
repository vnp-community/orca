// Command server is orchestration-service's composition root — the only
// place allowed to know about every layer at once, per
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

	svcconfig "github.com/stablyai/orca-go/services/orchestration-service/internal/config"

	orchgrpc "github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/infrafleetclient"
	orchmysql "github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/mysql"
	orchpostgres "github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/taskserviceclient"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// repository is the union of every port internal/adapter/postgres.Repository
// and internal/adapter/mysql.Repository both implement — declared here
// (not in usecase/ports.go) purely so run() can hold one variable across
// the dialect switch below instead of one variable per port interface;
// unlike usage-service's pilot (a single usecase.Repository interface),
// orchestration-service's ports.go has 4 separate repository interfaces
// (OrchestrationTaskRepository/DispatchContextRepository/GateRepository/
// CoordinatorRunRepository), so this composition is main.go's own, not a
// port usecase code depends on.
type repository interface {
	usecase.OrchestrationTaskRepository
	usecase.DispatchContextRepository
	usecase.GateRepository
	usecase.CoordinatorRunRepository
	outbox.Store
}

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("orchestration-service exited with error", slog.Any("error", err))
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

	// CR-DB-002/CR-DB-003 (batch 2): DATABASE_DSN's scheme picks the
	// adapter at startup, no separate DB_DIALECT env var — same posture as
	// usage-service's pilot (BE-DB-SOL-001 §1). Reads via
	// secrets.DatabaseCredentialsFromFile now (previously read cfg.DatabaseDSN
	// directly per the README's "Known gaps: common/secrets is not wired
	// into main.go" — that gap is closed by this rollout, matching every
	// other rolled-out service's wiring).
	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}

	healthSrv := health.New()

	var repo repository
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		repo = orchpostgres.New(pool)
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
		repo = orchmysql.New(db)
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

	// Transactional-outbox relay (BE-SOL-003/TASK-FT-003-01) — same shape
	// as usage-service's cmd/server/main.go wiring: usecases durably
	// enqueue an outbox row in the SAME database transaction as their
	// domain write (internal/adapter/postgres|mysql.Repository), this
	// relay is what actually gets those rows to NATS. If NATS is
	// unreachable at startup, rows still get written durably, they just
	// queue up unpublished until an operator restarts this process once
	// NATS recovers.
	var relay *outbox.Relay
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		// Stream name "ORCHESTRATION" matches notification-service's
		// already-wired SubjectBinding{StreamName: "ORCHESTRATION", ...} —
		// do not rename it.
		if err := pub.EnsureStream(ctx, "ORCHESTRATION", []string{"orca.orchestration.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure ORCHESTRATION stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
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

	// infra-fleet-service (WorkerDispatcher's Relay target) and
	// task-service (TaskServiceReporter's ReportTaskExecutionResult
	// target) — dialed once here and shared, same lazy-dial-then-close
	// pattern as task-service's own cmd/server/main.go dial calls.
	infraFleetConn, err := grpcclient.Dial(cfg.InfraFleetServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing infra-fleet-service: %w", err)
	}
	defer func() { _ = infraFleetConn.Close() }()
	infraFleetClient := infrafleetv1.NewInfraFleetServiceClient(infraFleetConn)
	workerDispatcher := infrafleetclient.NewWorkerDispatcher(infraFleetClient)

	taskServiceConn, err := grpcclient.Dial(cfg.TaskServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing task-service: %w", err)
	}
	defer func() { _ = taskServiceConn.Close() }()
	taskServiceClient := taskv1.NewTaskServiceClient(taskServiceConn)
	taskServiceReporter := taskserviceclient.NewReporter(taskServiceClient)

	// KeyedSerializer is wired ONCE here and shared by every usecase that
	// needs handle-serialized execution — a single shared instance is
	// required for the serialization guarantee to mean anything across
	// different usecases racing on the same key (orchestration-service.md
	// §6/§8).
	serializer := usecase.NewKeyedSerializer(0)

	createDispatchContextUC := usecase.NewCreateDispatchContext(repo, serializer, repo)
	createGateUC := usecase.NewCreateGate(repo, serializer, repo, repo)
	resolveGateUC := usecase.NewResolveGate(repo, serializer)
	updateTaskStatusAndPromoteUC := usecase.NewUpdateTaskStatusAndPromote(repo, serializer, taskServiceReporter, repo)
	getDispatchContextForTaskUC := usecase.NewGetDispatchContextForTask(repo)
	listActiveDispatchContextsForUserUC := usecase.NewListActiveDispatchContextsForUser(repo)
	failDispatchUC := usecase.NewFailDispatch(repo)
	startCoordinatorRunUC := usecase.NewStartCoordinatorRun(repo, serializer)
	getCoordinatorRunUC := usecase.NewGetCoordinatorRun(repo)
	completeCoordinatorRunUC := usecase.NewCompleteCoordinatorRun(repo, serializer)
	failCoordinatorRunUC := usecase.NewFailCoordinatorRun(repo, serializer)
	recordHeartbeatUC := usecase.NewRecordHeartbeat(repo)
	listPendingDecisionGatesUC := usecase.NewListPendingDecisionGates(repo)
	tickDispatchUC := usecase.NewTickDispatch(repo, repo, createDispatchContextUC, workerDispatcher, failDispatchUC, taskServiceReporter)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	orchestrationv1.RegisterOrchestrationServiceServer(grpcServer, orchgrpc.New(
		createDispatchContextUC, createGateUC, resolveGateUC, updateTaskStatusAndPromoteUC, getDispatchContextForTaskUC,
		listActiveDispatchContextsForUserUC, failDispatchUC,
		startCoordinatorRunUC, getCoordinatorRunUC, completeCoordinatorRunUC, failCoordinatorRunUC,
		recordHeartbeatUC, listPendingDecisionGatesUC,
	))
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
		logger.Info("orchestration-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("orchestration-service http (health) listening", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	// TickDispatch is the autonomous coordinator loop orchestration-service.md
	// §2.2 describes: claim+dispatch every ready-unclaimed task, then retry
	// any terminal run whose ReportResult call hasn't succeeded yet. A
	// separate cancelable context (not the shutdown ctx) so the ticker can
	// be stopped explicitly before GracefulStop, rather than racing it.
	tickCtx, tickCancel := context.WithCancel(context.Background())
	defer tickCancel()
	go func() {
		// matches domain.defaultPollIntervalMs; a per-run poll_interval_ms is
		// stored on CoordinatorRun but this scaffold's tick loop runs one
		// global cadence for every run — see SOL-TASKV1-005's "Not in scope"
		// section for the per-run-interval refinement, deliberately deferred.
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-tickCtx.Done():
				return
			case <-ticker.C:
				if err := tickDispatchUC.Execute(context.Background()); err != nil {
					logger.Error("tick dispatch failed", slog.Any("error", err))
					// Deliberately does NOT stop the ticker or exit the
					// process — one failed tick (e.g. a transient Postgres
					// blip) must not take down the whole coordinator; the
					// next tick retries the same ClaimReady scan.
				}
			}
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
	tickCancel()
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	// Wait for the outbox relay goroutine (if started) to observe ctx
	// cancellation and return, so it doesn't outlive the rest of the
	// server on shutdown — same pattern usage-service's main.go uses.
	relayWG.Wait()

	return nil
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — copied verbatim from
// usage-service's cmd/server/main.go (TASK-BE-DB-006) rather than shared
// via a common package, same precedent issue-tracking-service's and
// issue-status-sync's own rollouts already established. See that
// function's doc comment for the full "why not net/url alone" rationale;
// unchanged here.
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
