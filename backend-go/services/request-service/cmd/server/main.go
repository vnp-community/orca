package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/dbcapability"
	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	mysqladapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/mysql"
	postgresadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config) error {
	log := logging.New(cfg.ServiceName, "dev")
	apperrors.SetLogger(log)
	log.Info("request-service starting", slog.Bool("request_flow_enabled", cfg.RequestFlowEnabled))

	shutdownTracer, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		log.WarnContext(ctx, "tracer init failed", slog.Any("error", err))
	} else {
		defer func() {
			if err := shutdownTracer(context.Background()); err != nil {
				log.WarnContext(ctx, "tracer shutdown failed", slog.Any("error", err))
			}
		}()
	}

	dsn := cfg.DatabaseDSN
	if cfg.DatabaseCredentialsFile != "" {
		creds, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
		if err != nil {
			return fmt.Errorf("read database credentials: %w", err)
		}
		dsn = creds
	}

	dialect, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("unrecognized DSN scheme: %w", err)
	}

	var outboxStore outbox.Store
	var approvalRepo usecase.ApprovalRepository
	healthSrv := health.New()
	var closeDB func()

	switch dialect.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("postgres connect: %w", err)
		}
		closeDB = pool.Close
		repo := postgresadapter.New(pool)
		outboxStore = repo
		approvalRepo = repo

		healthSrv.Register("postgres", func() error {
			return pool.Ping(context.Background())
		})
	case dbcapability.DialectMySQL:
		mysqlDSN, err := toMySQLDriverDSN(dsn)
		if err != nil {
			return fmt.Errorf("format mysql dsn: %w", err)
		}
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			return fmt.Errorf("mysql open: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(0)
		closeDB = func() { _ = db.Close() }
		repo := mysqladapter.New(db)
		outboxStore = repo
		approvalRepo = repo

		healthSrv.Register("mysql", func() error {
			return db.PingContext(context.Background())
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", dialect.Dialect)
	}
	defer closeDB()

	var outboxRelay *outbox.Relay
	pub, _, closeBus, err := commoneventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		log.WarnContext(ctx, "eventbus unavailable, outbox publishing disabled", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"}); err != nil {
			log.WarnContext(ctx, "failed to ensure REQUEST stream", slog.Any("error", err))
		} else {
			outboxRelay = outbox.NewRelay(outboxStore, pub, outbox.DefaultConfig, log)
			healthSrv.Register("nats", func() error { return nil })
		}
	}

	outboxCtx, cancelOutbox := context.WithCancel(ctx)
	defer cancelOutbox()
	var outboxRelayWG sync.WaitGroup
	if outboxRelay != nil {
		outboxRelayWG.Add(1)
		go func() {
			defer outboxRelayWG.Done()
			outboxRelay.Run(outboxCtx)
		}()
	}

	registry := usecase.NewSubjectHandlerRegistry()
	noopHandler := &usecase.NoopSubjectHandler{Reason: "Not implemented", Logger: log}
	for _, st := range domain.AllSubjectTypes {
		registry.Register(st, noopHandler)
	}

	if err := registry.MustCoverAll(); err != nil {
		return fmt.Errorf("approval subject handler registry: %w", err)
	}
	if !cfg.AllowNoopApprovalHandlers {
		for _, st := range domain.AllSubjectTypes {
			if _, isNoop := registry.Get(st).(*usecase.NoopSubjectHandler); isNoop {
				return fmt.Errorf("noop approval handler found for %s but REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS is false", st)
			}
		}
	}

	// Just checking approvalRepo is assigned
	_ = approvalRepo

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(log), grpcmw.StatsHandler())
	requestv1.RegisterRequestServiceServer(grpcServer, requestgrpc.NewServer())
	requestv1.RegisterApprovalServiceServer(grpcServer, requestgrpc.NewApprovalServer())

	reflection.Register(grpcServer)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: healthSrv.Handler(),
	}

	errCh := make(chan error, 2)
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			errCh <- fmt.Errorf("listen grpc: %w", err)
			return
		}
		log.Info("request-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("serve grpc: %w", err)
		}
	}()

	go func() {
		log.Info("request-service http listening", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("serve http: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		log.Info("received signal, shutting down", slog.String("signal", sig.String()))
		grpcServer.GracefulStop()
		_ = httpServer.Shutdown(context.Background())
		cancelOutbox()
		if outboxRelay != nil {
			outboxRelayWG.Wait()
		}
	}

	return nil
}
