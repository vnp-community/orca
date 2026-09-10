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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/notification-service/internal/config"

	notificationbroadcaster "github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	notificationeventbus "github.com/stablyai/orca-go/services/notification-service/internal/adapter/eventbus"
	notificationapns "github.com/stablyai/orca-go/services/notification-service/internal/adapter/external/apns"
	notificationfcm "github.com/stablyai/orca-go/services/notification-service/internal/adapter/external/fcm"
	notificationwebpush "github.com/stablyai/orca-go/services/notification-service/internal/adapter/external/webpush"
	notificationgrpc "github.com/stablyai/orca-go/services/notification-service/internal/adapter/grpc"
	notificationauthclient "github.com/stablyai/orca-go/services/notification-service/internal/adapter/grpcclient/authclient"
	notificationnacl "github.com/stablyai/orca-go/services/notification-service/internal/adapter/nacl"
	notificationpostgres "github.com/stablyai/orca-go/services/notification-service/internal/adapter/postgres"
	notificationvaultsigner "github.com/stablyai/orca-go/services/notification-service/internal/adapter/vaultsigner"
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
	brokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
	}
	defer func() { _ = brokerConn.Close() }()
	signer := notificationvaultsigner.New(brokerConn)

	// BL-MB-02 (SOL-MB-02): mobile push delivery pipeline — buffered
	// notifications + per-event preferences (TASK-MB-02-06), device
	// shared-secret resolution + E2E sealing (TASK-MB-02-07), APNs/FCM/Web
	// Push transports (TASK-MB-02-07/08).
	bufferStore := notificationpostgres.NewBufferedNotificationStore(pool)
	preferenceStore := notificationpostgres.NewNotificationPreferenceStore(pool)
	sealer := notificationnacl.New()
	webpushClient := notificationwebpush.New()

	// auth-service connection — resolves a paired mobile device's shared
	// secret (SOL-MB-01's ResolveDeviceSharedSecret, internal-only RPC).
	// Dialing doesn't block/error on an unreachable target; a call-time
	// failure there just means that one subscription's delivery gets
	// buffered (BR-MB-07), same graceful-degradation shape as every other
	// peer-service client in this composition root.
	deviceSecrets, err := notificationauthclient.New(cfg.AuthServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing auth-service at %s: %w", cfg.AuthServiceAddr, err)
	}
	defer func() { _ = deviceSecrets.Close() }()

	// APNs/FCM's own Transit-mediated credential (TASK-MB-02-08) — never
	// VAPID. vaultClient construction only fails on a malformed
	// VAULT_ADDR, not on Vault being unreachable (see
	// infra-fleet-service's identical precedent) — a nil vaultClient here
	// (or unset APNS_*/FCM_* identifiers) means the ios/android channels
	// degrade to a clear per-delivery error rather than a process crash;
	// the web/VAPID channel is entirely unaffected.
	var apnsClient usecase.APNsClient
	var fcmClient usecase.FCMClient
	vaultClient, err := secrets.NewClient()
	if err != nil {
		logger.Warn("failed to construct Vault client — APNs/FCM push (ios/android channels) will be unavailable", slog.Any("error", err))
	} else {
		apnsClient = notificationapns.New(vaultClient, notificationapns.Config{
			TeamID: cfg.APNsTeamID, KeyID: cfg.APNsKeyID, Topic: cfg.APNsTopic, Endpoint: cfg.APNsEndpoint,
		})
		fcmClient = notificationfcm.New(vaultClient, notificationfcm.Config{
			ProjectID: cfg.FCMProjectID, ServiceAccountEmail: cfg.FCMServiceAccountEmail,
		})
		if cfg.APNsTeamID == "" || cfg.APNsKeyID == "" || cfg.APNsTopic == "" {
			logger.Warn("APNS_TEAM_ID/APNS_KEY_ID/APNS_TOPIC not fully configured — ios push will fail with a clear config error until set")
		}
		if cfg.FCMProjectID == "" || cfg.FCMServiceAccountEmail == "" {
			logger.Warn("FCM_PROJECT_ID/FCM_SERVICE_ACCOUNT_EMAIL not configured — android push will fail with a clear config error until set")
		}
	}

	deliverPushUC := usecase.NewDeliverPush(repo, deviceSecrets, sealer, signer, webpushClient, bufferStore, preferenceStore, apnsClient, fcmClient, logger)

	subscribeUC := usecase.NewSubscribe(repo)
	unregisterPushSubscriptionUC := usecase.NewUnregisterPushSubscription(repo)
	getVapidPublicKeyUC := usecase.NewGetVapidPublicKey(repo)
	handleIncomingEventUC := usecase.NewHandleIncomingEvent(broadcast, repo, deliverPushUC, logger)

	var consumerWG sync.WaitGroup
	_, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, continuing without event consumption", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
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

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	notificationv1.RegisterNotificationServiceServer(grpcServer, notificationgrpc.New(subscribeUC, unregisterPushSubscriptionUC, getVapidPublicKeyUC, broadcast, signer, bufferStore))
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
