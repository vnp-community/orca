// Command server is issue-tracking-service's composition root — the only
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
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/issue-tracking-service/internal/config"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/credential"
	issuetrackinggrpc "github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/jira"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/linear"
	issuetrackingpostgres "github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/providerregistry"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("issue-tracking-service exited with error", slog.Any("error", err))
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

	// Postgres pool — Epic G (docs/execution-plan.md): this service's first
	// database, added purely to host the transactional-outbox table. Jira
	// and Linear remain the systems of record for issue data itself (design
	// doc §2); every read is still live against the provider, nothing is
	// cached as a queryable copy here (design doc §5's thin operational
	// tables are a later, separate addition, not needed for
	// ListIssues/CreateIssue/LinkIssue).
	dsn := cfg.DatabaseDSN
	if dsn == "" {
		return errors.New("DATABASE_DSN is required (or a Vault-Agent-rendered credentials file — not wired in this scaffold)")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	repo := issuetrackingpostgres.New(pool)
	// connectionRepo is the same Repository value as repo — connections.go
	// adds the usecase.ConnectionRepository methods to the one Repository
	// type this service already has.
	connectionRepo := repo

	registry := providerregistry.New().
		Register(domain.ProviderJira, jira.New(nil)).
		Register(domain.ProviderLinear, linear.New(nil))

	// Real credential-broker-service connection — Epic B
	// (docs/execution-plan.md §8). Insecure transport credentials here are
	// a local-dev/scaffold convenience only; production deploys terminate
	// mTLS via the service mesh sidecar, per
	// architecture/07-security-architecture.md.
	brokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
	}
	defer func() { _ = brokerConn.Close() }()
	credentialResolver := credential.New(brokerConn, connectionRepo)

	healthSrv := health.New()
	healthSrv.Register("postgres", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})

	// Transactional-outbox relay (Epic G, docs/execution-plan.md):
	// LinkIssue durably enqueues an outbox row via repo.Enqueue — this
	// relay is what actually gets those rows to NATS. If NATS is
	// unreachable at startup, rows still get written durably (LinkIssue
	// never touches NATS directly), they just queue up unpublished until a
	// future restart — this scaffold's Connect calls don't retry mid-run,
	// same limitation every other NATS-consuming service here already
	// carries.
	var relay *outbox.Relay
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "ISSUETRACKING", []string{"orca.issuetracking.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
			healthSrv.Register("nats", func() error { return nil }) // presence-only: a real liveness probe would ping the connection
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

	listIssuesUC := usecase.NewListIssues(registry, credentialResolver)
	createIssueUC := usecase.NewCreateIssue(registry, credentialResolver)
	linkIssueUC := usecase.NewLinkIssue(repo)

	connectUC := usecase.NewConnect(registry, credentialResolver, connectionRepo)
	disconnectUC := usecase.NewDisconnect(connectionRepo)
	selectWorkspaceUC := usecase.NewSelectWorkspace(connectionRepo)
	getConnectionStatusUC := usecase.NewGetConnectionStatus(connectionRepo)
	testConnectionUC := usecase.NewTestConnection(registry, credentialResolver)

	searchIssuesUC := usecase.NewSearchIssues(registry, credentialResolver)
	getIssueUC := usecase.NewGetIssue(registry, credentialResolver)
	updateIssueUC := usecase.NewUpdateIssue(registry, credentialResolver)
	addIssueCommentUC := usecase.NewAddIssueComment(registry, credentialResolver)
	listIssueCommentsUC := usecase.NewListIssueComments(registry, credentialResolver)

	listProjectsUC := usecase.NewListProjects(registry, credentialResolver)
	listIssueTypesUC := usecase.NewListIssueTypes(registry, credentialResolver)
	listCreateFieldsUC := usecase.NewListCreateFields(registry, credentialResolver)
	listAssignableUsersUC := usecase.NewListAssignableUsers(registry, credentialResolver)
	listPrioritiesUC := usecase.NewListPriorities(registry, credentialResolver)
	listTransitionsUC := usecase.NewListTransitions(registry, credentialResolver)
	getProjectStatusOrderUC := usecase.NewGetProjectStatusOrder(registry, credentialResolver)

	createProjectUC := usecase.NewCreateProject(registry, credentialResolver)
	getProjectUC := usecase.NewGetProject(registry, credentialResolver)

	listTeamsUC := usecase.NewListTeams(registry, credentialResolver)
	listTeamLabelsUC := usecase.NewListTeamLabels(registry, credentialResolver)
	listTeamMembersUC := usecase.NewListTeamMembers(registry, credentialResolver)
	getCustomViewUC := usecase.NewGetCustomView(registry, credentialResolver)
	listWorkflowStatesUC := usecase.NewListWorkflowStates(registry, credentialResolver)

	// credentials.* group (TASK-041) — credentialResolver (adapter/credential.Resolver)
	// also satisfies CredentialWriter/CredentialStatusReader/CredentialLister/
	// CredentialRevoker; no new dial needed.
	setIntegrationCredentialUC := usecase.NewSetIntegrationCredential(credentialResolver)
	getIntegrationCredentialStatusUC := usecase.NewGetIntegrationCredentialStatus(credentialResolver)
	listIntegrationCredentialsUC := usecase.NewListIntegrationCredentials(credentialResolver)
	revokeAuthUC := usecase.NewRevokeAuth(credentialResolver)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	issuetrackingv1.RegisterIssueTrackingServiceServer(grpcServer, issuetrackinggrpc.New(issuetrackinggrpc.Deps{
		ListIssues:  listIssuesUC,
		CreateIssue: createIssueUC,
		LinkIssue:   linkIssueUC,

		Connect:             connectUC,
		Disconnect:          disconnectUC,
		SelectWorkspace:     selectWorkspaceUC,
		GetConnectionStatus: getConnectionStatusUC,
		TestConnection:      testConnectionUC,

		SearchIssues:      searchIssuesUC,
		GetIssue:          getIssueUC,
		UpdateIssue:       updateIssueUC,
		AddIssueComment:   addIssueCommentUC,
		ListIssueComments: listIssueCommentsUC,

		ListProjects:          listProjectsUC,
		ListIssueTypes:        listIssueTypesUC,
		ListCreateFields:      listCreateFieldsUC,
		ListAssignableUsers:   listAssignableUsersUC,
		ListPriorities:        listPrioritiesUC,
		ListTransitions:       listTransitionsUC,
		GetProjectStatusOrder: getProjectStatusOrderUC,

		CreateProject: createProjectUC,
		GetProject:    getProjectUC,

		ListTeams:          listTeamsUC,
		ListTeamLabels:     listTeamLabelsUC,
		ListTeamMembers:    listTeamMembersUC,
		GetCustomView:      getCustomViewUC,
		ListWorkflowStates: listWorkflowStatesUC,

		SetIntegrationCredential:       setIntegrationCredentialUC,
		GetIntegrationCredentialStatus: getIntegrationCredentialStatusUC,
		ListIntegrationCredentials:     listIntegrationCredentialsUC,
		RevokeAuth:                     revokeAuthUC,
	}))
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
		logger.Info("issue-tracking-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("issue-tracking-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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

	// Wait for the outbox relay goroutine (if started) to observe ctx
	// cancellation and return, so it doesn't outlive the rest of the
	// server on shutdown — same pattern notification-service's main.go
	// uses for its own background consumer.
	relayWG.Wait()

	return nil
}
