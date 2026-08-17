// Command server is api-gateway's composition root — the only place
// allowed to know about every layer at once, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Unlike every other service, api-gateway runs no gRPC server of its own:
// per api-gateway.md §1/§7, it is a pure gRPC CLIENT to all 16 other
// services, never a downstream dependency of any of them ("It is called by
// frontend... mobile... and CLI clients... No internal service calls
// api-gateway"). Exposing a trivial gRPC health service here would add a
// listener nothing in the system is designed to call — the HTTP
// /healthz+/readyz server below is this service's only health surface,
// same convention every other service uses (common/health).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/api-gateway/internal/config"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/httpgateway"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wsbridge"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("api-gateway exited with error", slog.Any("error", err))
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

	// The two downstream services this scaffold really dials — see
	// README.md "what's really wired". grpc.NewClient doesn't block or
	// error on an unreachable target; connection health is surfaced via
	// /readyz below instead, so a downstream outage degrades readiness
	// rather than crashing the gateway at startup.
	usageConn, err := gatewaygrpc.Dial(cfg.UsageServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing usage-service: %w", err)
	}
	defer func() { _ = usageConn.Close() }()
	usageClient := usagev1.NewUsageServiceClient(usageConn)

	notificationConn, err := gatewaygrpc.Dial(cfg.NotificationServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing notification-service: %w", err)
	}
	defer func() { _ = notificationConn.Close() }()
	notificationClient := notificationv1.NewNotificationServiceClient(notificationConn)

	authValidator := usecase.NewAuthValidator()
	rateLimiter := usecase.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	registry := domain.NewDefaultServiceRegistry()

	// wsHandler opens one StreamNotifications gRPC call per WS connection,
	// scoped to that connection's own context (cancelled on WS close, see
	// wsbridge.Handler.ServeHTTP) — never a single shared stream fanned
	// out to multiple users, per api-gateway.md §3's "a connection is
	// never fanned out to more than one" rule.
	wsHandler := wsbridge.New(logger, authValidator, func(streamCtx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		streamCtx = gatewaygrpc.AttachIdentity(streamCtx, usecase.Identity{UserID: userID})
		return notificationClient.StreamNotifications(streamCtx, &notificationv1.StreamNotificationsRequest{UserId: userID})
	})

	router := httpgateway.NewRouter(httpgateway.Deps{
		Logger:        logger,
		Registry:      registry,
		AuthValidator: authValidator,
		RateLimiter:   rateLimiter,
		UsageClient:   usageClient,
		WSHandler:     wsHandler.ServeHTTP,
	})

	healthSrv := health.New()
	healthSrv.Register("usage-service", grpcConnHealthCheck(usageConn))
	healthSrv.Register("notification-service", grpcConnHealthCheck(notificationConn))

	publicServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.PublicPort),
		Handler: router,
	}
	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: healthSrv.Handler(),
	}

	errCh := make(chan error, 2)

	go func() {
		logger.Info("api-gateway public REST/WS edge listening", slog.Int("port", cfg.PublicPort))
		if err := publicServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("public http server: %w", err)
		}
	}()

	go func() {
		logger.Info("api-gateway http (health) listening", slog.Int("port", cfg.HTTPPort))
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("health http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown: stop accepting new connections and let in-flight
	// requests (and open WS bridges, via wsbridge's context cancellation
	// on server shutdown) drain within the grace period, matching
	// standards/production-readiness-checklist.md.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = publicServer.Shutdown(shutdownCtx)
	_ = healthServer.Shutdown(shutdownCtx)

	return nil
}

// grpcConnHealthCheck reports readiness from the ClientConn's connectivity
// state — not a full RPC round trip, just "is this connection not
// currently failing", cheap enough to run on every /readyz poll.
func grpcConnHealthCheck(conn *grpc.ClientConn) health.Checker {
	return func() error {
		state := conn.GetState()
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			return fmt.Errorf("connection state: %s", state)
		}
		return nil
	}
}
