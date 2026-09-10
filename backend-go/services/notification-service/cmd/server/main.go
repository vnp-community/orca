// Command server is notification-service's composition root — the only
// place allowed to know about every layer at once, per
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
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/notification-service/internal/config"

	notificationbroadcaster "github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	notificationcredentialbroker "github.com/stablyai/orca-go/services/notification-service/internal/adapter/credentialbroker"
	notificationeventbus "github.com/stablyai/orca-go/services/notification-service/internal/adapter/eventbus"
	webpushsender "github.com/stablyai/orca-go/services/notification-service/internal/adapter/external/webpush"
	notificationgrpc "github.com/stablyai/orca-go/services/notification-service/internal/adapter/grpc"
	notificationpostgres "github.com/stablyai/orca-go/services/notification-service/internal/adapter/postgres"
	notificationpushgateway "github.com/stablyai/orca-go/services/notification-service/internal/adapter/pushgateway"
	notificationvaultsigner "github.com/stablyai/orca-go/services/notification-service/internal/adapter/vaultsigner"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("notification-service exited with error", slog.Any("error", err))
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
	// non-fatal degrade posture notification-service already had:
	// continues without event consumption when NATS is down at startup.
	pub, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, continuing without event consumption", slog.Any("error", err))
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

	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	repo := notificationpostgres.New(pool)
	broadcast := notificationbroadcaster.New()

	// Real credential-broker-service connection — Epic B
	// (docs/execution-plan.md §8), replacing this service's previous direct
	// Vault client. Insecure transport credentials here are a
	// local-dev/scaffold convenience only; production deploys terminate
	// mTLS via the service mesh sidecar, per
	// architecture/07-security-architecture.md. grpc.NewClient doesn't
	// block or error on an unreachable target — SignVapidPayload surfaces a
	// clear error at call time if credential-broker-service is actually
	// unreachable, same graceful-degradation shape the previous direct-Vault
	// version had.
	brokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
	}
	defer func() { _ = brokerConn.Close() }()
	signer := notificationvaultsigner.New(brokerConn)
	// pushCredentials reuses brokerConn — same connection already dialed
	// for signer above, not a second dial to credential-broker-service
	// (BE-MOBILE-SOL-001 §1.1).
	pushCredentials := notificationcredentialbroker.New(brokerConn)

	subscribeUC := usecase.NewSubscribe(repo)
	unregisterPushSubscriptionUC := usecase.NewUnregisterPushSubscription(repo)
	getVapidPublicKeyUC := usecase.NewGetVapidPublicKey(repo)
	listNotificationsUC := usecase.NewListNotifications(repo)
	markAsReadUC := usecase.NewMarkAsRead(repo, broadcast)
	markAllAsReadUC := usecase.NewMarkAllAsRead(repo, broadcast)
	getUnreadCountUC := usecase.NewGetUnreadCount(repo)
	webpushSender := webpushsender.New(nil)
	deliverPushUC := usecase.NewDeliverPush(repo, repo, signer, webpushSender, cfg.VapidContactURI, logger)

	// http2-enabled client shared by both APNs and FCM senders — both
	// speak HTTP/2 (APNs requires it; FCM's HTTP v1 API accepts HTTP/1.1
	// but works fine over HTTP/2 too).
	pushHTTPClient := &http.Client{
		Transport: &http2.Transport{},
		Timeout:   10 * time.Second,
	}
	apnsSender := notificationpushgateway.NewAPNsSender(pushCredentials, pushHTTPClient, "com.stably.orca.mobile")
	fcmSender := notificationpushgateway.NewFCMSender(pushCredentials, pushHTTPClient)
	deliverMobilePushUC := usecase.NewDeliverMobilePush(repo, map[domain.Channel]usecase.PushSender{
		domain.ChannelIOS:     apnsSender,
		domain.ChannelAndroid: fcmSender,
	}, logger)

	handleIncomingEventUC := usecase.NewHandleIncomingEvent(broadcast, repo, repo, deliverPushUC, deliverMobilePushUC, logger)

	var consumerWG sync.WaitGroup
	// cons already connected above (TASK-BE-FFT-008) — nil here iff
	// eventbus.Connect failed, same non-fatal degrade as before.
	if cons != nil {
		consumerAdapter := notificationeventbus.New(cons, handleIncomingEventUC)
		consumerWG.Add(1)
		go func() {
			defer consumerWG.Done()
			// Runs until ctx is cancelled — graceful shutdown for this
			// background loop is "stop accepting new signal, let ctx
			// cancellation propagate", the same mechanism the gRPC/HTTP
			// servers below use.
			consumerAdapter.Run(ctx, logger)
		}()
	}

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	notificationv1.RegisterNotificationServiceServer(grpcServer, notificationgrpc.New(
		subscribeUC, unregisterPushSubscriptionUC, getVapidPublicKeyUC,
		listNotificationsUC, markAsReadUC, markAllAsReadUC, getUnreadCountUC,
		broadcast, signer,
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
		logger.Info("notification-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("notification-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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

	// Graceful shutdown: GracefulStop drains in-flight gRPC calls
	// (including active StreamNotifications streams, which return when
	// their ctx is done) before returning, matching the
	// termination-grace-period expectation in
	// standards/production-readiness-checklist.md.
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	// Wait for the event-consumer background goroutine to observe ctx
	// cancellation and return, so it doesn't outlive the rest of the
	// server on shutdown.
	consumerWG.Wait()

	return nil
}
