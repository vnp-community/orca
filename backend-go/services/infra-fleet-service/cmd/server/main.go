// Command server is infra-fleet-service's composition root — the only place
// allowed to know about every layer at once, per
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/infra-fleet-service/internal/config"

	infraagentwsserver "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/agentwsserver"
	infradevserveragent "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	infraeventbus "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	infragrpc "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/grpc"
	inframetrics "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/metrics"
	infrapostgres "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/postgres"
	infrasshconn "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	infrasshrelay "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	infrawebhook "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/webhook"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("infra-fleet-service exited with error", slog.Any("error", err))
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

	// One Repository backs DevServerRepository, ConnectionRepository,
	// ConnectionResolver, and FleetHealthPort; SshTargetStore is a separate
	// value over the same pool for SshTargetRepository/SshTargetResolver —
	// see internal/adapter/postgres's package doc comment for why they can't
	// be the same Go value.
	repo := infrapostgres.New(pool)
	sshTargetStore := infrapostgres.NewSshTargetStore(pool)
	terminalSessionStore := infrapostgres.NewTerminalSessionStore(pool)
	browserProfileStore := infrapostgres.NewBrowserProfileStore(pool)

	// relay-websocket (outbound dial) and direct-websocket (inbound accept,
	// wired below via agentwsserver) are both real, and so is relay-ssh now
	// (deploy agent/out/agent.js over SSH, launch it --stdio, real JSON-RPC
	// session — see adapter/sshrelay's package doc comment), wired in via a
	// Vault client used only for SSH cert issuance (sshconn.SSHCertIssuer).
	// vault.NewClient() only builds the API client object (no network call,
	// see common/secrets's doc comment) — construction failing means
	// VAULT_ADDR is malformed, not that Vault is unreachable, so this stays
	// a startup log warning + relay-ssh left unavailable, not a fatal error:
	// this service's core (dev-server registry, relay-websocket,
	// direct-websocket) has nothing to do with Vault and must not
	// crash-loop over one optional mode's dependency. ORCA_RELAY_BUNDLE_PATH
	// unset is the same kind of "leave relay-ssh unavailable" case, checked
	// lazily inside sshrelay.deploy rather than here, since it's still worth
	// constructing the provisioner (so config wiring is visibly complete)
	// even if deploy will fail until an operator sets the bundle path.
	agentCfg := infradevserveragent.LoadConfigFromEnv()
	var agentOpts []infradevserveragent.Option
	// sshProvisioner is also BulkProvisionFleet's provisioning port
	// (wrapped below) — hoisted out of the if/else so both wiring sites
	// share the one instance instead of dialing SSH twice.
	var sshProvisioner *infrasshrelay.Provisioner
	vaultClient, err := secrets.NewClient()
	if err != nil {
		logger.Warn("failed to construct Vault client — relay-ssh mode will report ErrConnectionModeNotImplemented", slog.Any("error", err))
	} else {
		sshConnector := infrasshconn.NewConnector(vaultClient, infrasshconn.LoadConfigFromEnv())
		sshRelayCfg := infrasshrelay.LoadConfigFromEnv(agentCfg.OrcaVersion)
		if sshRelayCfg.BundlePath == "" {
			logger.Warn("ORCA_RELAY_BUNDLE_PATH is not set — relay-ssh dev servers will fail to provision until it points at a built agent/out/agent.js")
		}
		sshProvisioner = infrasshrelay.NewProvisioner(sshConnector, sshTargetStore, sshRelayCfg)
		agentOpts = append(agentOpts, infradevserveragent.WithRelaySSH(sshProvisioner))
	}

	agentClient := infradevserveragent.New(agentCfg, logger, agentOpts...)
	defer agentClient.Close()

	// bulkProvisioner degrades the same way relay-ssh mode itself does when
	// Vault isn't configured (see the warning above) — BulkProvisionFleet
	// still constructs and serves, every call just fails with a typed,
	// permanent error instead of the service crash-looping over one
	// optional mode's dependency.
	var bulkProvisioner usecase.Provisioner
	if sshProvisioner != nil {
		bulkProvisioner = infrasshrelay.NewBulkProvisioner(sshProvisioner)
	} else {
		bulkProvisioner = unavailableBulkProvisioner{}
	}

	registerDevServerUC := usecase.NewRegisterDevServer(repo)
	resolveConnectionUC := usecase.NewResolveConnection(repo)
	createSshTargetUC := usecase.NewCreateSshTarget(sshTargetStore)
	getFleetHealthUC := usecase.NewGetFleetHealth(repo)
	scanWorkspacePortsUC := usecase.NewScanWorkspacePorts(repo, agentClient)
	listDevServersUC := usecase.NewListDevServers(repo)
	createConnectionUC := usecase.NewCreateConnection(repo)
	relayUC := usecase.NewRelay(repo, agentClient)
	listSshTargetsUC := usecase.NewListSshTargets(sshTargetStore)
	getSshStateUC := usecase.NewGetSshState(sshTargetStore, repo, repo)
	establishConnectionUC := usecase.NewEstablishConnection(sshTargetStore, repo, repo, agentClient)
	killWorkspacePortUC := usecase.NewKillWorkspacePort(repo, agentClient)

	// --- Terminal/PTY (TASK-185) --- one ConnectionStreamLimiter shared by
	// AttachPty across every stream this process serves.
	ptyStreamLimiter := usecase.NewConnectionStreamLimiter(0)
	spawnTerminalSessionUC := usecase.NewSpawnTerminalSession(repo, agentClient, terminalSessionStore, cfg.ServerDeployment)
	resizeTerminalSessionUC := usecase.NewResizeTerminalSession(terminalSessionStore, repo, agentClient)
	killTerminalSessionUC := usecase.NewKillTerminalSession(terminalSessionStore, repo, agentClient)
	stopTerminalProcessUC := usecase.NewStopTerminalProcess(terminalSessionStore, repo, agentClient)
	listTerminalSessionsUC := usecase.NewListTerminalSessions(terminalSessionStore)
	waitTerminalSessionUC := usecase.NewWaitTerminalSession(terminalSessionStore, repo, agentClient)
	focusTerminalSessionUC := usecase.NewFocusTerminalSession(terminalSessionStore)
	getTerminalAgentStatusUC := usecase.NewGetTerminalAgentStatus(terminalSessionStore, repo, agentClient)
	inspectTerminalProcessUC := usecase.NewInspectTerminalProcess(terminalSessionStore, repo, agentClient)
	attachPtyUC := usecase.NewAttachPty(terminalSessionStore, repo, agentClient, ptyStreamLimiter)
	listBrowserProfilesUC := usecase.NewListBrowserProfiles(browserProfileStore)
	createBrowserProfileUC := usecase.NewCreateBrowserProfile(browserProfileStore, uuid.NewString)
	deleteBrowserProfileUC := usecase.NewDeleteBrowserProfile(browserProfileStore)

	// --- Emulator relay (TASK-048) / host capabilities relay (TASK-070) ---
	// Shipped-but-honestly-inert until agent/ gains device.*/host.capabilities
	// — see usecase.EmulatorRelay / usecase.GetHostCapabilities doc comments.
	emulatorRelayUC := usecase.NewEmulatorRelay(repo, agentClient)
	getHostCapabilitiesUC := usecase.NewGetHostCapabilities(repo, agentClient)

	importFleetInventoryUC := usecase.NewImportFleetInventory(sshTargetStore)
	bulkProvisionFleetUC := usecase.NewBulkProvisionFleet(sshTargetStore, repo, bulkProvisioner)
	detectDevServerAgentsUC := usecase.NewDetectDevServerAgents(repo, agentClient)
	checkDevServerPreflightUC := usecase.NewCheckDevServerPreflight(repo, agentClient)

	// --- Fleet health polling (SOL-FLEET-03) ---------------------------
	// Transactional-outbox relay for dev_server.health_degraded — same
	// eventbus.Connect/outbox.Relay pattern as usage-service's identical
	// wiring (see that service's cmd/server/main.go). infra-fleet-service
	// carried no outbox/NATS infrastructure before this pass
	// (migrations/0010_outbox); if NATS is unreachable at startup, outbox
	// rows still get written durably, they just queue up unpublished until
	// a future restart.
	var outboxRelay *outbox.Relay
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, fleet health-change events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "INFRA_FLEET", []string{"orca.infra_fleet.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			outboxRelay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
		}
	}
	var outboxRelayWG sync.WaitGroup
	if outboxRelay != nil {
		outboxRelayWG.Add(1)
		go func() {
			defer outboxRelayWG.Done()
			outboxRelay.Run(ctx)
		}()
	}

	healthEventPublisherUC := infraeventbus.NewHealthPublisher(repo, logger)
	webhookAlerterUC := infrawebhook.NewAlerter(cfg.FleetWebhookURL, nil)
	if cfg.FleetWebhookURL == "" {
		logger.Info("FLEET_WEBHOOK_URL is not set — fleet status-change webhook alerts are disabled")
	}

	fleetMetricsRegistry := prometheus.NewRegistry()
	fleetCollector := inframetrics.NewFleetCollector()
	fleetMetricsRegistry.MustRegister(fleetCollector)

	pollFleetHealthUC := usecase.NewPollFleetHealth(
		repo, repo, agentClient, repo, healthEventPublisherUC, webhookAlerterUC, fleetCollector, logger,
	)
	go pollFleetHealthUC.Run(ctx, cfg.FleetPollInterval)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	infrafleetv1.RegisterInfraFleetServiceServer(grpcServer, infragrpc.New(
		registerDevServerUC,
		resolveConnectionUC,
		createSshTargetUC,
		getFleetHealthUC,
		scanWorkspacePortsUC,
		listDevServersUC,
		createConnectionUC,
		relayUC,
		listSshTargetsUC,
		getSshStateUC,
		establishConnectionUC,
		killWorkspacePortUC,
		spawnTerminalSessionUC,
		resizeTerminalSessionUC,
		killTerminalSessionUC,
		stopTerminalProcessUC,
		listTerminalSessionsUC,
		waitTerminalSessionUC,
		focusTerminalSessionUC,
		getTerminalAgentStatusUC,
		inspectTerminalProcessUC,
		attachPtyUC,
		listBrowserProfilesUC,
		createBrowserProfileUC,
		deleteBrowserProfileUC,
		emulatorRelayUC,
		getHostCapabilitiesUC,
		importFleetInventoryUC,
		bulkProvisionFleetUC,
		detectDevServerAgentsUC,
		checkDevServerPreflightUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})

	// direct-websocket's inbound WS handler ("/agent") and token-issuance
	// endpoint ("/api/agent-token") share this service's existing HTTP
	// server/port rather than opening a new one — see
	// internal/adapter/agentwsserver's package doc comment. slotRegistry is
	// shared between the two so a POST /api/agent-token-issued slot is what
	// the WS handshake later validates against.
	agentWSCfg := infraagentwsserver.LoadConfigFromEnv(cfg.HTTPPort, agentCfg.OrcaVersion)
	if agentWSCfg.APISecret == "" {
		logger.Warn("ORCA_AGENT_API_SECRET is not set — POST/GET /api/agent-token will reject every request (fail-secure); direct-websocket dev servers cannot be registered until it is configured")
	}
	slotRegistry := infraagentwsserver.NewRegistry(infraagentwsserver.DefaultConnectTimeout)
	defer slotRegistry.Stop()
	agentWSServer := infraagentwsserver.New(slotRegistry, agentClient, agentWSCfg, logger)
	agentTokenIssuer := infraagentwsserver.NewTokenIssuer(slotRegistry, agentWSCfg, logger)

	mux := http.NewServeMux()
	mux.Handle("/", healthSrv.Handler())
	mux.Handle("/agent", agentWSServer)
	mux.Handle("/api/agent-token", agentTokenIssuer)
	// Same port as the liveness/agent-WS endpoints above, not a new one —
	// see TASK-FLEET-03-08.
	mux.Handle("/health/metrics", promhttp.HandlerFor(fleetMetricsRegistry, promhttp.HandlerOpts{}))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			errCh <- fmt.Errorf("listening on grpc port: %w", err)
			return
		}
		logger.Info("infra-fleet-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("infra-fleet-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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
	outboxRelayWG.Wait() // outboxRelay.Run already returned once ctx was cancelled above

	return nil
}

// unavailableBulkProvisioner implements usecase.Provisioner with a
// permanent, typed failure — wired in when Vault (and therefore relay-ssh
// mode entirely) isn't configured, matching relay-ssh's own
// ErrConnectionModeNotImplemented degrade-not-crash convention (see
// devserveragent.Client.getOrProvisionSession).
type unavailableBulkProvisioner struct{}

func (unavailableBulkProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (usecase.HandshakeInfo, bool, error) {
	return usecase.HandshakeInfo{}, false, fmt.Errorf("%w: relay-ssh support was not enabled (see WithRelaySSH)", infradevserveragent.ErrConnectionModeNotImplemented)
}
