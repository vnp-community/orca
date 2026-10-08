package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	grpchealthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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

	healthSrv := health.New()
	stores, err := openStores(ctx, dsn, dialect.Dialect, healthSrv)
	if err != nil {
		return err
	}
	defer stores.close()
	outboxStore := stores.outbox
	rollout, err := wireRollout(cfg, log, stores) // wraps stores.tx and stores.outboxWriter: keep before any use case is built
	if err != nil {
		return err
	}
	defer rollout.close()

	dirs, err := dialApprovalDirectories(cfg, log)
	if err != nil {
		return err
	}
	defer dirs.close()
	if cfg.ApprovalEnabled {
		// Approval events get recipients, title and deep link just before they are published.
		outboxStore = &eventbusadapter.ApprovalNotificationStore{Store: stores.outbox, Notifier: newApprovalNotifier(cfg, log, stores, dirs), Log: log}
	}

	var outboxRelay *outbox.Relay
	pub, busConsumer, closeBus, err := commoneventbus.Connect(ctx, cfg.NATSURL)
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

	// The registry is shared with the lifecycle use cases (they close pending approvals), so it exists before
	// wireApproval fills it with the subject handlers.
	var approvalRegistry *usecase.SubjectHandlerRegistry
	if cfg.ApprovalEnabled {
		approvalRegistry = usecase.NewSubjectHandlerRegistry()
	}

	execTasks, err := dialExecutionTasks(cfg, log)
	if err != nil {
		return err
	}
	defer execTasks.close()
	lifecycle := wireRequestLifecycle(stores, approvalRegistry, execTasks.guard)
	transitioner := lifecycle.Transition
	var lifecycleWG sync.WaitGroup
	lifecycleCtx, cancelLifecycle := context.WithCancel(ctx)
	defer cancelLifecycle()
	// Solution wiring goes first: its handlers (solution, findings, answer) replace the generic ones in the registry,
	// and it needs the approval pieces back afterwards (the opener) via bindApprovals.
	solution, err := wireSolution(lifecycleCtx, &lifecycleWG, cfg, log, stores, lifecycle, dirs)
	if err != nil {
		return err
	}
	defer solution.close()
	approval, err := wireApproval(lifecycleCtx, &lifecycleWG, cfg, log, stores, lifecycle, approvalRegistry, dirs, solution.handlers, execTasks.subjectArtifacts())
	if err != nil {
		return err
	}
	solution.bindApprovals(approval.SolutionOpener())
	intake, err := wireRequestIntake(cfg, log, stores, transitioner)
	if err != nil {
		return err
	}
	defer intake.close()
	intake.attachIntake(lifecycle, stores)
	rollout.AttachLookup(intake)
	classification, err := wireClassification(lifecycleCtx, &lifecycleWG, cfg, log, stores, transitioner, approval.Recorder, busConsumer)
	if err != nil {
		return err
	}
	defer classification.close()
	rollout.StartSampler(lifecycleCtx, &lifecycleWG)
	approval.BindConfirm(classification.confirm)
	execution, err := wireExecution(lifecycleCtx, &lifecycleWG, cfg, log, stores, lifecycle, approval, execTasks, busConsumer)
	if err != nil {
		return err
	}
	defer execution.close()
	artifact, err := wireArtifact(lifecycleCtx, &lifecycleWG, cfg, log, stores, lifecycle, intake, classification, busConsumer, dirs, solution)
	if err != nil {
		return err
	}
	defer artifact.close()
	security, err := wireSecurity(lifecycleCtx, &lifecycleWG, cfg, log, stores, rollout.SecurityHooks())
	if err != nil {
		return err
	}
	defer security.close()
	registerEntityResolvers(security.authorize, stores.artifact)
	httpHandler, err := intake.webhookHandler(cfg, log, healthSrv.Handler(), security.replay)
	if err != nil {
		return err
	}

	httpHandler = rollout.MetricsHandler(httpHandler)

	// ChainUnary (recovery, tenant extraction, logging) stays first; the security chain (with the flow gate and RPC audit hooks) runs after it.
	grpcServer := grpc.NewServer(append(append([]grpc.ServerOption{grpcmw.ChainUnary(log), grpcmw.StatsHandler()}, security.options...), rollout.ServerOptions()...)...)
	requestv1.RegisterRequestServiceServer(grpcServer, artifact.attach(rollout.WithRPCs(requestgrpc.NewServer(
		usecase.NewGetRequest(stores.requests),
		usecase.NewListRequests(stores.requests),
	).WithExecution(execution.Server))).WithIntake(requestgrpc.IntakeUseCases{Create: intake.createRequest, Lookup: intake.lookup}).
		WithClassification(requestgrpc.ClassificationUseCases{Runner: classification.runner, Confirm: classification.confirm, Change: classification.change, History: classification.history}).
		WithLifecycle(requestgrpc.LifecycleUseCases{Flow: lifecycle.GetFlow, Return: lifecycle.Return, Reopen: lifecycle.Reopen, Cancel: lifecycle.Cancel, SpawnChild: lifecycle.SpawnChild, Links: lifecycle.ListLinks}).
		WithSolution(solution.useCases()).
		WithCompliance(security.compliance))
	if approval.Enabled {
		requestv1.RegisterApprovalServiceServer(grpcServer, requestgrpc.NewApprovalServer(approval.Server))
		requestv1.RegisterApprovalPolicyAdminServiceServer(grpcServer, requestgrpc.NewApprovalPolicyAdminServer(approval.Admin))
	} else {
		log.Info("approval service disabled (REQUEST_APPROVAL_ENABLED=false)")
	}

	requestv1.RegisterAiBudgetAdminServiceServer(grpcServer, requestgrpc.NewAiBudgetAdminServer())

	grpcHealth := grpchealth.NewServer()
	grpchealthv1.RegisterHealthServer(grpcServer, grpcHealth)
	grpcHealth.SetServingStatus("", grpchealthv1.HealthCheckResponse_SERVING)
	grpcHealth.SetServingStatus(requestv1.RequestService_ServiceDesc.ServiceName, grpchealthv1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: httpHandler,
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
	case <-ctx.Done():
		log.Info("context cancelled, shutting down")
	}
	grpcServer.GracefulStop()
	_ = httpServer.Shutdown(context.Background())
	cancelLifecycle()
	lifecycleWG.Wait()
	cancelOutbox()
	if outboxRelay != nil {
		outboxRelayWG.Wait()
	}
	return nil
}
