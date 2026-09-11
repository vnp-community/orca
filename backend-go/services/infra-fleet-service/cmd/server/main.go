// Command server is infra-fleet-service's composition root — the only place
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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/infra-fleet-service/internal/config"

	infraagentwsserver "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/agentwsserver"
	infrabackendrelaysshprovisioner "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/backendrelaysshprovisioner"
	infradevserveragent "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	infraephemeralsshconn "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/ephemeralsshconn"
	infraeventbus "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	infragrpc "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/grpc"
	infragrpcclient "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/grpcclient"
	inframetrics "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/metrics"
	infrafleetmysql "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/mysql"
	infraportalloc "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/portalloc"
	infraportevents "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/portevents"
	infrapostgres "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/postgres"
	infrasshconn "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	infrasshrelay "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	infrawebhook "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/webhook"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
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

	// NATS connect moved ahead of tracing.Init (TASK-BE-FFT-008) so pub
	// exists in time to pass to tracing.WithTraceEventPublisher. Same
	// non-fatal degrade posture infra-fleet-service already had: rows
	// still write durably to the outbox even when NATS is down at startup.
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, dev-server-disconnected alerts will queue until a future restart", slog.Any("error", err))
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

	// repo is bound to ONE concrete adapter (infrapostgres.Repository or
	// infrafleetmysql.Repository) per caps.Dialect below and used at every
	// call site throughout this file exactly the way the single-dialect
	// `repo := infrapostgres.New(pool)` line used to be — repoAll's method
	// set is the union of every DB-backed usecase port Repository
	// implements (10 interfaces, per internal/adapter/mysql/shared.go's
	// compile-time assertions), so a repoAll value satisfies any ONE of
	// them implicitly wherever a narrower interface parameter is expected
	// below, without touching those call sites individually — same
	// technique as task-service's cmd/server/main.go, sized to this
	// service's larger (15-repository) port surface. The other 14
	// repositories are each used through exactly one usecase port, so they
	// stay typed as that single interface instead. All 15 are constructed
	// together here — rather than scattered at each one's original call
	// site further down — so the postgres/mysql branch is written once per
	// repository; SshTargetStore stays a separate Go value from Repository
	// (SshTargetRepository/SshTargetResolver) — see
	// internal/adapter/postgres's package doc comment for why they can't be
	// the same value.
	type repoAll interface {
		usecase.DevServerRepository
		usecase.ConnectionRepository
		usecase.ConnectionResolver
		usecase.FleetHealthPort
		usecase.FleetHealthWriter
		usecase.PollLockPort
		usecase.FleetConnectivityRepository
		usecase.FleetHealthPollerRepository
		usecase.OutboxWriter
		outbox.Store
		infraeventbus.OutboxEnqueuer // EnqueueOutboxEvent — distinct, narrower interface than usecase.OutboxWriter's InsertOutboxEvent
	}

	// rateLimitedOutboxStore is outbox.Store (the transactional-outbox
	// relay's port) plus Enqueue — the one extra method
	// infraeventbus.New's local rateLimitedOutboxEnqueuer interface needs
	// from AgentRateLimitedOutboxStore, distinct from outbox.Store's
	// FetchUnpublished/MarkPublished.
	type rateLimitedOutboxStore interface {
		outbox.Store
		Enqueue(ctx context.Context, rec outbox.Record) error
	}

	var (
		repo                        repoAll
		sshTargetStore              usecase.SshTargetRepository
		terminalSessionStore        usecase.TerminalSessionRepository
		browserProfileStore         usecase.BrowserProfileRepository
		scrollbackStore             usecase.TerminalScrollbackSnapshotRepository
		agentTokenStore             usecase.AgentTokenRepository
		agentSessionStore           usecase.AgentSessionRepository
		agentRateLimitedOutboxStore rateLimitedOutboxStore
		queuedPromptStore           usecase.QueuedPromptRepository
		ephemeralVmRuntimeStore     usecase.EphemeralVmRuntimeRepository
		devServerGroupStore         usecase.DevServerGroupRepository
		devServerGroupGrantStore    usecase.DevServerGroupGrantRepository
		devServerAccessRequestStore usecase.DevServerAccessRequestRepository
		portForwardStore            usecase.PortForwardRepository
		ephemeralVmSshTargetStore   usecase.EphemeralVmSshTargetRepository
	)
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		pgRepo := infrapostgres.New(pool)
		repo = pgRepo
		sshTargetStore = infrapostgres.NewSshTargetStore(pool)
		terminalSessionStore = infrapostgres.NewTerminalSessionStore(pool)
		browserProfileStore = infrapostgres.NewBrowserProfileStore(pool)
		scrollbackStore = infrapostgres.NewTerminalScrollbackSnapshotStore(pool)
		agentTokenStore = infrapostgres.NewAgentTokenStore(pool)
		agentSessionStore = infrapostgres.NewAgentSessionStore(pool)
		agentRateLimitedOutboxStore = infrapostgres.NewAgentRateLimitedOutboxStore(pool)
		queuedPromptStore = infrapostgres.NewQueuedPromptStore(pool)
		ephemeralVmRuntimeStore = infrapostgres.NewEphemeralVmRuntimeStore(pool)
		devServerGroupStore = infrapostgres.NewDevServerGroupStore(pool)
		devServerGroupGrantStore = infrapostgres.NewDevServerGroupGrantStore(pool)
		devServerAccessRequestStore = infrapostgres.NewDevServerAccessRequestStore(pool)
		portForwardStore = infrapostgres.NewPortForwardStore(pool)
		ephemeralVmSshTargetStore = infrapostgres.NewEphemeralVmSshTargetStore(pool)
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
		myRepo := infrafleetmysql.New(db)
		repo = myRepo
		sshTargetStore = infrafleetmysql.NewSshTargetStore(db)
		terminalSessionStore = infrafleetmysql.NewTerminalSessionStore(db)
		browserProfileStore = infrafleetmysql.NewBrowserProfileStore(db)
		scrollbackStore = infrafleetmysql.NewTerminalScrollbackSnapshotStore(db)
		agentTokenStore = infrafleetmysql.NewAgentTokenStore(db)
		agentSessionStore = infrafleetmysql.NewAgentSessionStore(db)
		agentRateLimitedOutboxStore = infrafleetmysql.NewAgentRateLimitedOutboxStore(db)
		queuedPromptStore = infrafleetmysql.NewQueuedPromptStore(db)
		ephemeralVmRuntimeStore = infrafleetmysql.NewEphemeralVmRuntimeStore(db)
		devServerGroupStore = infrafleetmysql.NewDevServerGroupStore(db)
		devServerGroupGrantStore = infrafleetmysql.NewDevServerGroupGrantStore(db)
		devServerAccessRequestStore = infrafleetmysql.NewDevServerAccessRequestStore(db)
		portForwardStore = infrafleetmysql.NewPortForwardStore(db)
		ephemeralVmSshTargetStore = infrafleetmysql.NewEphemeralVmSshTargetStore(db)
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}

	// Transactional-outbox relay (Epic G, docs/execution-plan.md;
	// TASK-AUTH-05-08): EstablishConnection durably enqueues an outbox row
	// in the SAME Postgres transaction as the connection write
	// (internal/adapter/postgres.Repository.CreateConnectionWithOutbox) —
	// this relay is what actually gets those rows to NATS. Mirrors
	// usage-service's cmd/server/main.go exactly: if NATS is unreachable at
	// startup, connection writes still succeed (the request path never
	// touches NATS directly), rows just queue up unpublished until an
	// operator restarts this process once NATS recovers.
	// Reuses the single NATS connection established above (line ~87) for
	// TRACE — pub is nil when NATS was unreachable at startup, which
	// degrades this relay the same non-fatal way it always has.
	var relay *outbox.Relay
	if pub != nil {
		if err := pub.EnsureStream(ctx, "INFRAFLEET", []string{"orca.infrafleet.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
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
	credentialBrokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
	}
	defer credentialBrokerConn.Close()
	credentialBrokerClient := infragrpcclient.New(credentialBrokerConn)

	agentCfg := infradevserveragent.LoadConfigFromEnv()
	// sshProvisioner is also BulkProvisionFleet's provisioning port
	// (wrapped below) — hoisted out of the if/else so both wiring sites
	// share the one instance instead of dialing SSH twice.
	var sshProvisioner *infrasshrelay.Provisioner
	// agentTokenSource resolves a relay-websocket DevServer's current bearer
	// token fresh on every dial (never cached across process restarts) — see
	// TASK-AWS-01-03/SOL-AWS-01, replacing the former single deployment-wide
	// ORCA_AGENT_TOKEN.
	agentOpts := []infradevserveragent.Option{
		infradevserveragent.WithAgentTokens(agentTokenSource{tokens: agentTokenStore, broker: credentialBrokerClient}),
	}
	vaultClient, err := secrets.NewClient()
	if err != nil {
		logger.Warn("failed to construct Vault client — relay-ssh mode will report ErrConnectionModeNotImplemented", slog.Any("error", err))
	} else {
		sshConnectionCap := infrasshconn.NewCap()
		sshConnector := infrasshconn.NewConnector(vaultClient, sshTargetStore, infrasshconn.LoadConfigFromEnv(), sshConnectionCap)
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

	// terminalLiveStates is the shared per-pod quiescence registry
	// AttachPty writes and GetTerminalAgentStatus reads (TASK-MB-02-01/02) —
	// constructed once here so both usecases share the exact same instance.
	terminalLiveStates := &sync.Map{}

	// Agent-lifecycle push-notification events (TASK-MB-02-01) — best-effort,
	// reuses the SAME NATS connection as the outbox relay above (pub is nil
	// when NATS was unreachable at startup): a nil pub degrades this to "no
	// mobile push notifications", never a fatal error, mirroring
	// tenant-service's eventbus wiring.
	var lifecycleEvents usecase.LifecycleEventPublisher
	if pub != nil {
		if err := pub.EnsureStream(ctx, infraeventbus.StreamName, []string{"orca.infra.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			lifecycleEvents = infraeventbus.NewLifecyclePublisher(pub)
			healthSrv.Register("nats", func() error { return nil }) // presence-only: a real liveness probe would ping the connection
		}
	}

	registerDevServerUC := usecase.NewRegisterDevServer(repo)
	resolveDirectWebSocketDevServerUC := usecase.NewResolveDirectWebSocketDevServer(repo)
	createSshTargetUC := usecase.NewCreateSshTarget(sshTargetStore)
	// bulkProvisionFleetUC (TASK-BE-FLEET-001/003, CR-FLEET-001) needs
	// bulkProvisioner (the relay-ssh SSH-connect -> prereq-check -> deploy ->
	// handshake pipeline, or its unavailable-degrade stand-in) constructed
	// above — NOT createSshTargetUC/registerDevServerUC, which is a
	// different, incompatible shape BulkProvisionFleet no longer takes.
	bulkProvisionFleetUC := usecase.NewBulkProvisionFleet(sshTargetStore, repo, bulkProvisioner)
	getFleetHealthUC := usecase.NewGetFleetHealth(repo)
	scanWorkspacePortsUC := usecase.NewScanWorkspacePorts(repo, agentClient)
	listDevServersUC := usecase.NewListDevServers(repo)
	listDevServersByTagUC := usecase.NewListDevServersByTag(repo, repo)
	createConnectionUC := usecase.NewCreateConnection(repo)
	relayUC := usecase.NewRelay(repo, agentClient)
	relayStreamUC := usecase.NewRelayStream(repo, agentClient)
	relayByDevServerUC := usecase.NewRelayByDevServer(repo, agentClient)
	// streamFileChangesUC (BACKLOG-003) — repo implements both
	// ConnectionResolver and DevServerRepository, same dual-role convention
	// relayUC/relayByDevServerUC each use separately.
	streamFileChangesUC := usecase.NewStreamFileChanges(repo, repo, agentClient)
	isDevServerConnectedUC := usecase.NewIsDevServerConnected(repo, agentClient)
	// streamAgentExecOutputUC backs the StreamExecOutput RPC
	// (TASK-AG-FLOWTASK-002/003) — task-service's SimpleExecutor consumes
	// it alongside its own Relay('agent.execPrompt') call.
	streamAgentExecOutputUC := usecase.NewStreamAgentExecOutput(repo, agentClient)
	listSshTargetsUC := usecase.NewListSshTargets(sshTargetStore)
	getSshStateUC := usecase.NewGetSshState(sshTargetStore, repo, repo)

	// Audit-append client (TASK-BE-018/022, CR-RBAC-005) — EstablishConnection
	// uses this to record every "ssh.connect" allow/deny outcome to
	// auth-service's audit_log. Lazy dial (grpc.NewClient doesn't block on
	// connect), same convention as every other outbound client above.
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("dialing auth-service: %w", err)
	}
	defer func() { _ = authConn.Close() }()
	auditClient := auditclient.New(authv1.NewAuthServiceClient(authConn))

	establishConnectionUC := usecase.NewEstablishConnection(sshTargetStore, repo, repo, agentClient, auditClient)
	killWorkspacePortUC := usecase.NewKillWorkspacePort(repo, agentClient)
	// TeardownConnection (BE-SOL-STORAGE-003 §5, TASK-BE-STORAGE-012) — the
	// confirmed-logout explicit-close path; repo implements both
	// ConnectionResolver (lookup) and ConnectionRepository (persist), same
	// dual-role convention as its use elsewhere in this file.
	teardownConnectionUC := usecase.NewTeardownConnection(repo, repo, terminalSessionStore, agentClient)

	// --- Terminal/PTY (TASK-185) --- one ConnectionStreamLimiter shared by
	// AttachPty across every stream this process serves.
	ptyStreamLimiter := usecase.NewConnectionStreamLimiter(0)
	// resolveConnectionUC (TASK-BE-EVM-018, BE-SOL-EVM-004 §6c) needs
	// ephemeralVmRuntimeStore (constructed in the dialect switch above) for
	// its HiddenTargetID lookup.
	resolveConnectionUC := usecase.NewResolveConnection(repo, ephemeralVmRuntimeStore)
	resolveConnectionUC.Sessions = handshakeInfoProvider{client: agentClient} // TASK-INT-03-02: optional node_version enrichment
	spawnTerminalSessionUC := usecase.NewSpawnTerminalSession(repo, repo, agentClient, terminalSessionStore, ephemeralVmRuntimeStore, cfg.ServerDeployment)
	resizeTerminalSessionUC := usecase.NewResizeTerminalSession(terminalSessionStore, repo, repo, agentClient)
	killTerminalSessionUC := usecase.NewKillTerminalSession(terminalSessionStore, repo, repo, agentClient)
	stopTerminalProcessUC := usecase.NewStopTerminalProcess(terminalSessionStore, repo, repo, agentClient)
	listTerminalSessionsUC := usecase.NewListTerminalSessions(terminalSessionStore)
	waitTerminalSessionUC := usecase.NewWaitTerminalSession(terminalSessionStore, repo, repo, agentClient)
	focusTerminalSessionUC := usecase.NewFocusTerminalSession(terminalSessionStore)
	getTerminalAgentStatusUC := usecase.NewGetTerminalAgentStatus(terminalSessionStore, repo, repo, agentClient, terminalLiveStates, lifecycleEvents, queuedPromptStore)
	inspectTerminalProcessUC := usecase.NewInspectTerminalProcess(terminalSessionStore, repo, repo, agentClient)
	attachPtyUC := usecase.NewAttachPty(terminalSessionStore, repo, repo, agentClient, ptyStreamLimiter, terminalLiveStates, lifecycleEvents)
	screencastStreamLimiter := usecase.NewConnectionStreamLimiter(0)
	attachScreencastUC := usecase.NewAttachScreencast(repo, agentClient, screencastStreamLimiter)
	listBrowserProfilesUC := usecase.NewListBrowserProfiles(browserProfileStore)
	createBrowserProfileUC := usecase.NewCreateBrowserProfile(browserProfileStore, uuid.NewString)
	deleteBrowserProfileUC := usecase.NewDeleteBrowserProfile(browserProfileStore)
	// dispatchPrompt/getQueuedPrompt (TASK-MB-03-05) share queuedPromptStore
	// with getTerminalAgentStatusUC above — the SAME instance the
	// ready-transition queue-drain hook needs.
	dispatchPromptUC := usecase.NewDispatchPrompt(terminalSessionStore, repo, repo, agentClient, queuedPromptStore)
	getQueuedPromptUC := usecase.NewGetQueuedPrompt(terminalSessionStore, repo, repo, queuedPromptStore)

	// --- Emulator relay (TASK-048) / host capabilities relay (TASK-070) ---
	// Shipped-but-honestly-inert until agent/ gains device.*/host.capabilities
	// — see usecase.EmulatorRelay / usecase.GetHostCapabilities doc comments.
	emulatorRelayUC := usecase.NewEmulatorRelay(repo, agentClient)
	getHostCapabilitiesUC := usecase.NewGetHostCapabilities(repo, agentClient)

	// --- Terminal scrollback persistence (SOL-TM-03) ---
	saveTerminalScrollbackSnapshotUC := usecase.NewSaveTerminalScrollbackSnapshot(scrollbackStore, usecase.RealClock{})
	getTerminalScrollbackSnapshotUC := usecase.NewGetTerminalScrollbackSnapshot(scrollbackStore)
	deleteTerminalScrollbackSnapshotsUC := usecase.NewDeleteTerminalScrollbackSnapshots(scrollbackStore)

	// --- CLI agent access (BUG-CLI-02) ---
	getAgentTerminalSessionUC := usecase.NewGetAgentTerminalSession(repo, terminalSessionStore)
	sendTerminalInputUC := usecase.NewSendTerminalInput(terminalSessionStore, repo, repo, agentClient)
	getTerminalScrollbackUC := usecase.NewGetTerminalScrollback(terminalSessionStore, repo, repo, agentClient)

	importFleetInventoryUC := usecase.NewImportFleetInventory(sshTargetStore)
	detectDevServerAgentsUC := usecase.NewDetectDevServerAgents(repo, agentClient)
	checkDevServerPreflightUC := usecase.NewCheckDevServerPreflight(repo, agentClient)

	// --- Fleet health polling (SOL-FLEET-03) ---------------------------
	// dev_server.health_degraded publishes through the SAME outbox relay
	// constructed above for ssh.connect (TASK-AUTH-05-08) — both events
	// land in the one infra_fleet.outbox_events table (migrations/0010_outbox)
	// via the shared repo Store, so a second eventbus.Connect/outbox.Relay
	// pair here would just be a redundant NATS connection polling the same
	// rows. EnsureStream is still called per subject pattern this service
	// publishes to, per that method's doc comment.
	if pub != nil {
		if err := pub.EnsureStream(ctx, "INFRA_FLEET", []string{"orca.infra_fleet.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		}
	}

	healthEventPublisherUC := infraeventbus.NewHealthPublisher(repo, logger)
	webhookAlerterUC := infrawebhook.NewAlerter(cfg.FleetWebhookURL, nil)
	if cfg.FleetWebhookURL == "" {
		logger.Info("FLEET_WEBHOOK_URL is not set — fleet status-change webhook alerts are disabled")
	}

	fleetMetricsRegistry := prometheus.NewRegistry()
	fleetCollector := inframetrics.NewFleetCollector()
	fleetMetricsRegistry.MustRegister(fleetCollector)

	// pollFleetHealthUC combines SOL-FLEET-03's CPU/RAM/disk sampling +
	// status-change event/webhook/metrics wiring with BE-SOL-STORAGE-003's
	// connection degraded/reestablish state machine + outbox disconnect
	// alert — see usecase.PollFleetHealth's doc comment. repo implements
	// FleetHealthPollerRepository/FleetHealthWriter/OutboxWriter/
	// PollLockPort/ConnectionRepository all at once, same dual/multi-role
	// convention as its use elsewhere in this file. Driven by the ticker
	// goroutine further down (poll once on startup, then every
	// cfg.FleetPollInterval), not a self-ticking Run call here.
	pollFleetHealthUC := usecase.NewPollFleetHealth(
		repo, repo, repo, agentClient, repo, repo, terminalSessionStore, healthEventPublisherUC, webhookAlerterUC, fleetCollector, logger,
	)

	// --- Persistent agent tokens (BL-AWS-03) ---
	createAgentTokenUC := usecase.NewCreateAgentToken(agentTokenStore, repo, credentialBrokerClient)
	listAgentTokensUC := usecase.NewListAgentTokens(agentTokenStore)
	revokeAgentTokenUC := usecase.NewRevokeAgentToken(agentTokenStore, agentClient)

	// --- Auto port-forwarding (SOL-SSH-04) --- portForwardStore
	// constructed in the dialect switch above.
	portAllocator := infraportalloc.NewAllocator()
	createPortForwardUC := usecase.NewCreatePortForward(portForwardStore, portAllocator)
	listPortForwardsUC := usecase.NewListPortForwards(portForwardStore)
	deletePortForwardUC := usecase.NewDeletePortForward(portForwardStore)

	// --- Port-forward push notifications (TASK-SSH-04-08, BR-SSH-15) ---
	// One shared Broadcaster: usecase.PollWorkspacePorts.Run's future caller
	// gets it as its PortForwardEventPublisher (see
	// usecase.NewPollWorkspacePorts), and StreamPortForwardEvents subscribes
	// to it per connectionId. NOTE: PollWorkspacePorts.Run() is not yet
	// started anywhere in this composition root — its EstablishConnection
	// wiring is a separate, already-flagged follow-up (see
	// TASK-SSH-04-06-poll-workspace-ports-loop.md's Status note) — so no
	// events flow yet in production, but the broadcaster/RPC plumbing this
	// task owns is complete and ready for that wiring once it lands.
	portEventsBroadcaster := infraportevents.NewBroadcaster()

	// --- Agent sessions (TASK-AG-01..05) ---

	// ai-provider-service dial — SwitchAgentAccount's first outbound call to
	// ai-provider-service (TASK-AG-04-03, a new infra --> aiprov edge).
	aiProviderConn, err := infragrpcclient.Dial(cfg.AIProviderServiceAddr)
	if err != nil {
		logger.Error("failed to dial ai-provider-service", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = aiProviderConn.Close() }()
	aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)
	aiProviderResolver := infragrpcclient.NewAIProviderResolver(aiProviderClient)

	killAgentSessionUC := usecase.NewKillAgentSession(agentSessionStore, repo, agentClient, nil) // writeActivity: see TASK-AG-02-03/06

	// TASK-AG-05-06: AgentOutputClassifier needs a real AgentStatusPublisher,
	// which needs a live NATS connection — degrade gracefully (classifierUC
	// stays nil, StartAgentSession skips launching it, see that usecase's
	// nil-safe classifier field) rather than crash-looping this whole
	// service over an optional event-delivery dependency, same posture
	// every other NATS-consuming service in this codebase already takes
	// (see e.g. usage-service's cmd/server/main.go).
	var classifierUC *usecase.AgentOutputClassifier
	natsPub, _, closeEventBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.Warn("eventbus unavailable — agent.statusChanged/agent.rateLimited will not be published", slog.Any("error", err))
	} else {
		defer func() { _ = closeEventBus() }()
		if err := natsPub.EnsureStream(ctx, "INFRA", []string{"orca.infra.agent.>"}); err != nil {
			logger.Warn("failed to ensure jetstream stream for agent events", slog.Any("error", err))
		} else {
			agentStatusPublisher := infraeventbus.New(natsPub, agentRateLimitedOutboxStore)
			classifierUC = usecase.NewAgentOutputClassifier(agentSessionStore, agentClient, agentStatusPublisher, killAgentSessionUC)

			// agent:rateLimited outbox relay — mirrors usage-service's
			// relay-startup call site's shape (cmd/server/main.go).
			agentRateLimitedRelay := outbox.NewRelay(agentRateLimitedOutboxStore, natsPub, outbox.Config{PollInterval: 500 * time.Millisecond, BatchSize: 100}, logger)
			go agentRateLimitedRelay.Run(ctx)
		}
	}

	// TASK-AG-03-06: best-effort agent.hook consumer — one goroutine per dev
	// server this process has resolved a connection for, guarded against
	// duplicate starts. This is intentionally simple (a map + mutex in
	// main.go, not a separate registry type) since it has exactly one
	// caller today (StartAgentSession.Execute, via the callback below —
	// covers ResumeAgentSession too, since it delegates to the same
	// Execute).
	var (
		hookConsumersMu sync.Mutex
		hookConsumers   = map[string]bool{} // dev server id -> already started
	)
	ensureAgentHookConsumer := func(ctx context.Context, tenantID string, devServer domain.DevServer) {
		hookConsumersMu.Lock()
		defer hookConsumersMu.Unlock()
		if hookConsumers[devServer.ID] {
			return
		}
		hookConsumers[devServer.ID] = true
		recorder := usecase.NewRecordAgentHookProviderSession(agentSessionStore)
		go recorder.Run(context.Background(), tenantID, devServer, agentClient)
	}

	startAgentSessionUC := usecase.NewStartAgentSession(repo, agentClient, agentSessionStore, classifierUC, ensureAgentHookConsumer)
	stopAgentSessionUC := usecase.NewStopAgentSession(agentSessionStore, repo, agentClient)
	resumeAgentSessionUC := usecase.NewResumeAgentSession(agentSessionStore, repo, startAgentSessionUC)
	switchAgentAccountUC := usecase.NewSwitchAgentAccount(agentSessionStore, killAgentSessionUC, aiProviderResolver, startAgentSessionUC, resumeAgentSessionUC)

	// --- CR-DS-006 Phase 2 / CR-DS-007 / CR-DS-008 (dev server access
	// control) --- devServerGroupStore/devServerGroupGrantStore/
	// devServerAccessRequestStore constructed in the dialect switch above.
	approveDevServerUC := usecase.NewApproveDevServer(repo)
	rejectDevServerUC := usecase.NewRejectDevServer(repo)
	assignDevServerGroupUC := usecase.NewAssignDevServerGroup(repo)
	createDevServerGroupUC := usecase.NewCreateDevServerGroup(devServerGroupStore)
	listDevServerGroupsUC := usecase.NewListDevServerGroups(devServerGroupStore)
	grantDevServerGroupAccessUC := usecase.NewGrantDevServerGroupAccess(devServerGroupGrantStore)
	revokeDevServerGroupAccessUC := usecase.NewRevokeDevServerGroupAccess(devServerGroupGrantStore)
	listDevServerGroupGrantsUC := usecase.NewListDevServerGroupGrants(devServerGroupGrantStore)
	listDevServersForUserUC := usecase.NewListDevServersForUser(repo, devServerGroupStore, devServerGroupGrantStore)
	createAccessRequestUC := usecase.NewCreateAccessRequest(devServerAccessRequestStore)
	listPendingAccessRequestsUC := usecase.NewListPendingAccessRequests(devServerAccessRequestStore)
	resolveAccessRequestUC := usecase.NewResolveAccessRequest(devServerAccessRequestStore, devServerGroupGrantStore)

	// --- Ephemeral VM (SOL-004 Group 1/2a, TASK-002/004) ---
	// ephemeralVmRuntimeStore constructed in the dialect switch above,
	// reused here.
	listEphemeralVmRuntimesUC := usecase.NewListEphemeralVmRuntimes(ephemeralVmRuntimeStore)
	// --- Ephemeral VM ssh-type provisioner wiring (TASK-BE-EVM-012/014) ---
	// Hướng A (agent-outbound) only — Hướng B (backend-relay-deploy,
	// TASK-BE-EVM-013) wires its own usecase.WithSshProvisioner option
	// separately; cfg.EphemeralVmSshMode's default
	// ("backend-relay-deploy") stays inert here until that wiring lands.
	// ephemeralVmSshTargetStore constructed in the dialect switch above.
	var ephemeralVmRelayOpts []usecase.EphemeralVmRelayOption
	if cfg.EphemeralVmSshMode == "agent-outbound" {
		// GAP 1/2 FIX (TASK-BE-EVM-016, BE-SOL-EVM-004 §6a/§6b): no Vault
		// client and no devServer-resolver placeholder needed anymore —
		// AgentOutboundSshProvisioner no longer resolves credential
		// material from Vault (identityFile was never a Vault pointer,
		// see domain.EphemeralVmSshTarget's doc comment), and
		// EphemeralVmSshProvisioner.Provision now receives sourceDevServer
		// directly from EphemeralVmRelay.Provision instead of needing a
		// separate lookup port. repo satisfies usecase.ConnectionRepository
		// (used to register a real infra.connections row), same as it
		// already does for Hướng B just below.
		agentOutboundProvisioner := usecase.NewAgentOutboundSshProvisioner(
			agentClient, repo, ephemeralVmSshTargetStore)
		ephemeralVmRelayOpts = append(ephemeralVmRelayOpts, usecase.WithSshProvisioner(agentOutboundProvisioner))
	} else {
		// Hướng B (backend-relay-deploy, TASK-BE-EVM-013/017) — the default
		// mode (config.Load's EphemeralVmSshMode fail-safe default). Reuses
		// the EXACT same agent/out/agent.js deploy+launch+handshake
		// pipeline as the ordinary relay-ssh wiring just above
		// (infrasshrelay.Provisioner), only swapping auth
		// (adapter/ephemeralsshconn's recipe-credential dial instead of
		// adapter/sshconn's Vault-cert dial) — see
		// adapter/backendrelaysshprovisioner's package doc comment. Does
		// not call Vault at all (TASK-BE-EVM-017: credential bytes come
		// from DevServerAgentClient.ReadCredentialFile against the source
		// dev server instead), so it stays available even when Vault
		// client construction failed.
		backendRelaySshRelayCfg := infrasshrelay.LoadConfigFromEnv(agentCfg.OrcaVersion)
		if backendRelaySshRelayCfg.BundlePath == "" {
			logger.Warn("ORCA_RELAY_BUNDLE_PATH is not set — EPHEMERAL_VM_SSH_MODE=backend-relay-deploy ssh-type ephemeral VMs will fail to provision until it points at a built agent/out/agent.js")
		}
		// ephemeralVmSshTargetStore (TASK-BE-EVM-019, BE-SOL-EVM-004 §6d) is
		// the SAME store Hướng A's audit row uses (migrations/0015/0016) —
		// only one of Hướng A/B is active per deployment (EphemeralVmSshMode),
		// so no write-owner conflict; here it backs TOFU host-key
		// fingerprint persistence instead of the identity-path audit trail.
		backendRelaySshProvisioner := infrabackendrelaysshprovisioner.NewProvisioner(
			repo, repo, ephemeralVmRuntimeStore, agentClient,
			backendRelaySshRelayCfg, infraephemeralsshconn.LoadConfigFromEnv(), uuid.NewString,
			ephemeralVmSshTargetStore)
		ephemeralVmRelayOpts = append(ephemeralVmRelayOpts, usecase.WithSshProvisioner(backendRelaySshProvisioner))
	}
	ephemeralVmRelayUC := usecase.NewEphemeralVmRelay(repo, agentClient, ephemeralVmRuntimeStore, ephemeralVmRelayOpts...)

	// --- Fleet connectivity summary (CR-STORAGE-007, TASK-BE-STORAGE-006) ---
	getFleetConnectivitySummaryUC := usecase.NewGetFleetConnectivitySummary(repo)

	// PickByTag (TASK-WF-002-04) — closes workflow-service's
	// TargetKindFleetTag gap. repo satisfies both DevServerRepository and
	// ConnectionResolver (same combined repository every other usecase
	// above already passes as both), devServerGroupStore satisfies
	// DevServerGroupRepository.
	pickByTagUC := usecase.NewPickByTag(devServerGroupStore, repo, repo, agentClient)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	infrafleetv1.RegisterInfraFleetServiceServer(grpcServer, infragrpc.New(
		registerDevServerUC,
		resolveConnectionUC,
		createSshTargetUC,
		getFleetHealthUC,
		scanWorkspacePortsUC,
		listDevServersUC,
		listDevServersByTagUC,
		createConnectionUC,
		relayUC,
		relayStreamUC,
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
		attachScreencastUC,
		listBrowserProfilesUC,
		createBrowserProfileUC,
		deleteBrowserProfileUC,
		emulatorRelayUC,
		getHostCapabilitiesUC,
		saveTerminalScrollbackSnapshotUC,
		getTerminalScrollbackSnapshotUC,
		deleteTerminalScrollbackSnapshotsUC,
		getAgentTerminalSessionUC,
		sendTerminalInputUC,
		getTerminalScrollbackUC,
		importFleetInventoryUC,
		bulkProvisionFleetUC,
		detectDevServerAgentsUC,
		checkDevServerPreflightUC,
		createAgentTokenUC,
		listAgentTokensUC,
		revokeAgentTokenUC,
		teardownConnectionUC,
		createPortForwardUC,
		listPortForwardsUC,
		deletePortForwardUC,
		portEventsBroadcaster,
		startAgentSessionUC,
		stopAgentSessionUC,
		killAgentSessionUC,
		resumeAgentSessionUC,
		switchAgentAccountUC,
		dispatchPromptUC,
		getQueuedPromptUC,
		terminalLiveStates,
		approveDevServerUC,
		rejectDevServerUC,
		assignDevServerGroupUC,
		createDevServerGroupUC,
		listDevServerGroupsUC,
		grantDevServerGroupAccessUC,
		revokeDevServerGroupAccessUC,
		listDevServerGroupGrantsUC,
		listDevServersForUserUC,
		createAccessRequestUC,
		listPendingAccessRequestsUC,
		resolveAccessRequestUC,
		relayByDevServerUC,
		isDevServerConnectedUC,
		streamAgentExecOutputUC,
		listEphemeralVmRuntimesUC,
		ephemeralVmRelayUC,
		getFleetConnectivitySummaryUC,
		streamFileChangesUC,
		pickByTagUC,
	))
	reflection.Register(grpcServer) // convenient for grpcurl during local dev; keep enabled behind the mesh, not the public internet

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
	agentWSServer.Sessions = agentClient                              // TASK-AWS-02-03: agentClient already implements LiveSessionCount
	agentWSServer.Tokens = agentTokenValidator{repo: agentTokenStore} // TASK-AWS-03-06: persistent-token handshake fallback
	agentTokenIssuer := infraagentwsserver.NewTokenIssuer(slotRegistry, agentWSCfg, logger, resolveDirectWebSocketDevServerUC, ephemeralVmRuntimeStore)

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

	// Fleet-health poller — specs/backend-go/services/infra-fleet-service.md
	// §8's 30s cadence. A poll failure never reaches errCh: one bad tick
	// (e.g. a transient DB error) should not take the whole service down,
	// only skip that round — see PollFleetHealth's own doc comment for the
	// per-dev-server error handling this relies on.
	go func() {
		const fleetHealthPollInterval = 30 * time.Second
		// Poll once immediately on startup — otherwise a freshly-started
		// service (or one that just came back up) leaves every dev server
		// unreachable-by-default in fleet_health for a full interval before
		// its first real sample lands.
		if err := pollFleetHealthUC.Execute(ctx); err != nil {
			logger.WarnContext(ctx, "fleet health poll failed", slog.Any("error", err))
		}
		ticker := time.NewTicker(fleetHealthPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pollFleetHealthUC.Execute(ctx); err != nil {
					logger.WarnContext(ctx, "fleet health poll failed", slog.Any("error", err))
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
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	// Wait for the outbox relay goroutine (if started) to observe ctx
	// cancellation and return, so it doesn't outlive the rest of the
	// server on shutdown — same pattern usage-service/notification-service's
	// own main.go use for their background loops.
	relayWG.Wait()

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

// agentTokenSource adapts usecase.AgentTokenRepository +
// usecase.CredentialBrokerClient into devserveragent.AgentTokenSource —
// resolving a relay-websocket DevServer's current bearer token fresh on
// every dial, never cached across process restarts, so a revoked token is
// honored on the very next reconnect (TASK-AWS-01-03, SOL-AWS-01). Kept
// here in the composition root rather than as its own adapter package,
// since it exists purely to combine two other adapters' already-dialed
// clients — the same "the only place allowed to know about every layer at
// once" reasoning as this file's other wiring.
type agentTokenSource struct {
	tokens usecase.AgentTokenRepository
	broker usecase.CredentialBrokerClient
}

func (s agentTokenSource) TokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
	tok, found, err := s.tokens.ActiveForDevServer(ctx, devServer.TenantID, devServer.ID)
	if err != nil {
		return "", fmt.Errorf("resolving active agent token for dev server %s: %w", devServer.ID, err)
	}
	if !found {
		return "", fmt.Errorf("no active agent token registered for dev server %s", devServer.ID)
	}
	plaintext, err := s.broker.ResolveCredential(ctx, tok.CredentialRefID)
	if err != nil {
		return "", fmt.Errorf("resolving agent token credential for dev server %s: %w", devServer.ID, err)
	}
	return string(plaintext), nil
}

// agentTokenValidator adapts usecase.AgentTokenRepository into
// agentwsserver.TokenValidator — direct-websocket's persistent-token
// handshake fallback once Registry.Consume misses (TASK-AWS-03-06). Thin
// composition-root wrapper, same pattern as agentTokenSource above.
type agentTokenValidator struct {
	repo usecase.AgentTokenRepository
}

func (v agentTokenValidator) FindActiveByHash(ctx context.Context, hash string) (devServerID, tokenID string, found bool, err error) {
	t, found, err := v.repo.FindActiveByHash(ctx, hash)
	if err != nil || !found {
		return "", "", found, err
	}
	return t.DevServerID, t.ID, true, nil
}

func (v agentTokenValidator) TouchLastUsed(ctx context.Context, tokenID string) {
	_ = v.repo.TouchLastUsed(ctx, tokenID) // best-effort, never blocks the handshake on its result
}

// handshakeInfoProvider adapts devserveragent.Client.HandshakeInfoFor into
// usecase.HandshakeInfoProvider — ResolveConnection's optional Node-version
// enrichment (TASK-INT-03-02). Thin composition-root wrapper, same pattern
// as agentTokenSource/agentTokenValidator above.
type handshakeInfoProvider struct {
	client *infradevserveragent.Client
}

func (p handshakeInfoProvider) NodeVersionFor(devServerID string) (string, bool) {
	info, ok := p.client.HandshakeInfoFor(devServerID)
	if !ok {
		return "", false
	}
	return info.NodeVersion, true
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — copied verbatim from
// usage-service/cmd/server/main.go (see that file's doc comment for the
// full two-input-shape/parseTime rationale); this is pure DSN-plumbing, not
// service-specific, and there is no shared package for it yet — same
// precedent every prior rollout service has followed.
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
