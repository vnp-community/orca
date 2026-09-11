// Command server is scm-integration-service's composition root — the only
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/logging"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tracing"

	svcconfig "github.com/stablyai/orca-go/services/scm-integration-service/internal/config"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/azuredevops"
	scmbackoff "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/backoff"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/bitbucket"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/credentialbroker"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/gitea"
	scmgithub "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/github"
	scmgitlab "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/gitlab"
	scmgrpc "github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/mysql"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/oauth"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/oauthstate"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/providerregistry"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/webhookverify"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// outboxStore is the union of usecase.OutboxEnqueuer (CreatePullRequest/
// MergePullRequest/ReceiveWebhook's enqueue path) and common/outbox.Store
// (the relay's poll/publish/mark-published path) — both
// postgres.OutboxRepository and mysql.OutboxRepository satisfy this,
// letting outboxRepo below stay a single dialect-agnostic variable, same
// shape as rateLimitCache/issueListCache/webhookDeliveries.
type outboxStore interface {
	usecase.OutboxEnqueuer
	outbox.Store
}

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("scm-integration-service exited with error", slog.Any("error", err))
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

	// rate_limit_cache (migrations/{postgres,mysql}/0001_init.up.sql) — this
	// service's first real database connection. webhook_delivery_log lives
	// in the same migration; its own repository (webhookDeliveries, below)
	// was wired in TASK-PI-03-06. CR-DB-002/CR-DB-003: DATABASE_DSN's
	// scheme picks the adapter at startup, same dialect-factory pattern as
	// usage-service's pilot (BE-DB-SOL-001) — no separate DB_DIALECT env
	// var.
	dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
	if err != nil {
		return fmt.Errorf("resolving database credentials: %w", err)
	}
	caps, err := dbcapability.DetectDialectFromDSN(dsn)
	if err != nil {
		return fmt.Errorf("detecting database dialect: %w", err)
	}

	healthSrv := health.New()

	var (
		rateLimitCache    usecase.RateLimitCache
		issueListCache    usecase.IssueListCache
		outboxRepo        outboxStore
		webhookDeliveries usecase.WebhookDeliveryStore
	)
	switch caps.Dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connecting to postgres: %w", err)
		}
		defer pool.Close()
		rateLimitCache = postgres.New(pool)
		// issue_list_cache (migrations/0002) — BR-PI-01's 5-minute cache in
		// front of ListIssues, sibling of rateLimitCache above.
		issueListCache = postgres.NewIssueListCache(pool)
		outboxRepo = postgres.NewOutboxRepository(pool)
		// webhook_delivery_log (migrations/0001) — BUG-PI-03/TASK-PI-03-06's
		// first writer for this table.
		webhookDeliveries = postgres.NewWebhookDeliveryRepository(pool)
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
		rateLimitCache = mysql.New(db)
		issueListCache = mysql.NewIssueListCache(db)
		outboxRepo = mysql.NewOutboxRepository(db)
		webhookDeliveries = mysql.NewWebhookDeliveryRepository(db)
		healthSrv.Register("mysql", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(pingCtx)
		})
	default:
		return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
	}
	backoffExecutor := scmbackoff.New(3, 0, 0) // BR-PI-03: 3 attempts, default base/max delay
	webhookVerifier := webhookverify.New(cfg.GitHubWebhookSecret, cfg.GitLabWebhookToken)

	// githubProjectsAdapter/gitlabMRAdapter are the SAME instances registered
	// below in registry's map — one GitHub adapter satisfying both
	// usecase.ScmProvider and usecase.GitHubProjectsProvider, one GitLab
	// adapter satisfying both usecase.ScmProvider and
	// usecase.GitLabMergeRequestProvider (SOL-012/SOL-013's "still one
	// adapter, not a second client" design note).
	githubProjectsAdapter := scmgithub.New(nil, cfg.GitHubBaseURL)
	gitlabMRAdapter := scmgitlab.New(nil, cfg.GitLabBaseURL)
	registry := providerregistry.New(map[domain.ScmProvider]usecase.ScmProvider{
		domain.ScmProviderGitHub:      githubProjectsAdapter,
		domain.ScmProviderGitLab:      gitlabMRAdapter,
		domain.ScmProviderBitbucket:   bitbucket.New(nil, cfg.BitbucketBaseURL),
		domain.ScmProviderAzureDevOps: azuredevops.New(nil, cfg.AzureDevOpsBaseURL),
		domain.ScmProviderGitea:       gitea.New(nil, cfg.GiteaBaseURL),
	})

	// One OAuthExchanger per provider (§9.1) — a provider whose OAuth app
	// isn't configured (empty ClientID) is left out of this map entirely,
	// so StartOAuthFlow reports SCM_PROVIDER_UNSUPPORTED for it instead of
	// attempting a doomed exchange against an empty client_id.
	oauthExchangers := map[domain.ScmProvider]usecase.OAuthExchanger{}
	for provider, providerCfg := range map[domain.ScmProvider]svcconfig.OAuthProviderConfig{
		domain.ScmProviderGitHub:      cfg.OAuth.GitHub,
		domain.ScmProviderGitLab:      cfg.OAuth.GitLab,
		domain.ScmProviderBitbucket:   cfg.OAuth.Bitbucket,
		domain.ScmProviderAzureDevOps: cfg.OAuth.AzureDevOps,
		domain.ScmProviderGitea:       cfg.OAuth.Gitea,
	} {
		if providerCfg.ClientID == "" {
			continue
		}
		oauthExchangers[provider] = oauth.New(nil, oauth.Config{
			AuthorizeURL: providerCfg.AuthorizeURL,
			TokenURL:     providerCfg.TokenURL,
			ClientID:     providerCfg.ClientID,
			ClientSecret: providerCfg.ClientSecret,
			Scope:        providerCfg.Scope,
		})
	}
	oauthRegistry := providerregistry.NewOAuth(oauthExchangers)
	stateCodec := oauthstate.New(cfg.OAuthStateSecret)

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
	credentials := credentialbroker.New(brokerConn)

	// Transactional-outbox relay (SOL-PI-03) — CreatePullRequest/
	// MergePullRequest durably enqueue via outboxRepo.Enqueue; this relay is
	// what actually gets those rows to NATS. Mirrors
	// issue-tracking-service/cmd/server/main.go's own wiring exactly.
	var relay *outbox.Relay
	pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
	if err != nil {
		logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
	} else {
		defer func() { _ = closeBus() }()
		if err := pub.EnsureStream(ctx, "SCM", []string{"orca.scm.>"}); err != nil {
			logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
		} else {
			relay = outbox.NewRelay(outboxRepo, pub, outbox.DefaultConfig, logger)
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

	listIssuesUC := usecase.NewListIssues(credentials, registry, issueListCache, backoffExecutor)
	listPullRequestsUC := usecase.NewListPullRequests(credentials, registry)
	listWorkItemsUC := usecase.NewListWorkItems(credentials, registry)
	getRateLimitStatusUC := usecase.NewGetRateLimitStatus(credentials, registry, rateLimitCache)
	getAuthStatusUC := usecase.NewGetAuthStatus(credentials)
	startOAuthFlowUC := usecase.NewStartOAuthFlow(oauthRegistry, stateCodec, nil)
	completeOAuthFlowUC := usecase.NewCompleteOAuthFlow(oauthRegistry, stateCodec, credentials)
	revokeAuthUC := usecase.NewRevokeAuth(credentials)
	setIntegrationCredentialUC := usecase.NewSetIntegrationCredential(credentials)
	getIntegrationCredentialStatusUC := usecase.NewGetIntegrationCredentialStatus(credentials)
	listIntegrationCredentialsUC := usecase.NewListIntegrationCredentials(credentials)

	// SOL-012 shape 1/2 — GitHub PR/issue mutations + repo/branch resolution
	// (TASK-076). registry already fans out per-provider; these usecases
	// resolve the concrete adapter the same way every other usecase above
	// does.
	mergePullRequestUC := usecase.NewMergePullRequest(credentials, registry, outboxRepo, logger)
	requestPullRequestReviewersUC := usecase.NewRequestPullRequestReviewers(credentials, registry)
	removePullRequestReviewersUC := usecase.NewRemovePullRequestReviewers(credentials, registry)
	setPullRequestAutoMergeUC := usecase.NewSetPullRequestAutoMerge(credentials, registry)
	updateIssueUC := usecase.NewUpdateIssue(credentials, registry)
	updatePullRequestUC := usecase.NewUpdatePullRequest(credentials, registry)
	starRepositoryUC := usecase.NewStarRepository(credentials, registry)
	getPullRequestForBranchUC := usecase.NewGetPullRequestForBranch(credentials, registry)
	resolveRepoSlugUC := usecase.NewResolveRepoSlug(credentials, registry)

	// createPullRequestUC composes updateIssueUC in-process (BR-CR-19's
	// best-effort linked-issue update), so updateIssueUC must be
	// constructed before this line — same ordering concern as
	// git-gateway-service's historyUC/generateCommitMessageUC. It also
	// durably enqueues orca.scm.pull_request.created via outboxRepo
	// (SOL-PI-03) — the same outboxRepo instance mergePullRequestUC enqueues
	// to below.
	createPullRequestUC := usecase.NewCreatePullRequest(credentials, registry, updateIssueUC, outboxRepo, logger)
	suggestPullRequestReviewersUC := usecase.NewSuggestPullRequestReviewers(credentials, registry)

	// SOL-012 shape 3 — GitHub Projects v2 (TASK-079). githubProjectsAdapter
	// is the SAME *github.Client instance registered in registry's map below
	// — one GitHub adapter satisfying both usecase.ScmProvider and
	// usecase.GitHubProjectsProvider, per SOL-012's design note.
	listAccessibleProjectsUC := usecase.NewListAccessibleProjects(credentials, githubProjectsAdapter)
	resolveProjectRefUC := usecase.NewResolveProjectRef(credentials, githubProjectsAdapter)
	listProjectViewsUC := usecase.NewListProjectViews(credentials, githubProjectsAdapter)
	viewProjectTableUC := usecase.NewViewProjectTable(credentials, githubProjectsAdapter)
	updateProjectItemFieldUC := usecase.NewUpdateProjectItemField(credentials, githubProjectsAdapter)
	clearProjectItemFieldUC := usecase.NewClearProjectItemField(credentials, githubProjectsAdapter)
	getWorkItemDetailsBySlugUC := usecase.NewGetWorkItemDetailsBySlug(credentials, githubProjectsAdapter)
	updateIssueBySlugUC := usecase.NewUpdateIssueBySlug(credentials, githubProjectsAdapter)
	updatePullRequestBySlugUC := usecase.NewUpdatePullRequestBySlug(credentials, githubProjectsAdapter)
	updateIssueTypeBySlugUC := usecase.NewUpdateIssueTypeBySlug(credentials, githubProjectsAdapter)
	listIssueTypesBySlugUC := usecase.NewListIssueTypesBySlug(credentials, githubProjectsAdapter)
	listAssignableUsersBySlugUC := usecase.NewListAssignableUsersBySlug(credentials, githubProjectsAdapter)
	listLabelsBySlugUC := usecase.NewListLabelsBySlug(credentials, githubProjectsAdapter)
	addIssueCommentBySlugUC := usecase.NewAddIssueCommentBySlug(credentials, githubProjectsAdapter)
	updateIssueCommentBySlugUC := usecase.NewUpdateIssueCommentBySlug(credentials, githubProjectsAdapter)
	deleteIssueCommentBySlugUC := usecase.NewDeleteIssueCommentBySlug(credentials, githubProjectsAdapter)
	listIssueCommentsBySlugUC := usecase.NewListIssueCommentsBySlug(credentials, githubProjectsAdapter)

	// SOL-013 — GitLab-specific (TASK-084). gitlabMRAdapter is the SAME
	// *gitlab.Client instance registered in registry's map below.
	listMergeRequestsUC := usecase.NewListMergeRequests(credentials, gitlabMRAdapter)
	resolveMergeRequestDiscussionUC := usecase.NewResolveMergeRequestDiscussion(credentials, gitlabMRAdapter)
	getWorkItemDetailsUC := usecase.NewGetWorkItemDetails(credentials, gitlabMRAdapter)

	// SOL-014 — hostedReview.getCreationEligibility (TASK-088). Reuses the
	// same getAuthStatusUC instance the GetAuthStatus RPC already uses.
	checkHostedReviewEligibilityUC := usecase.NewCheckHostedReviewEligibility(credentials, registry, getAuthStatusUC)

	// BUG-PI-01/SOL-PI-01 (TASK-PI-01-06) and BUG-PI-04/SOL-PI-04
	// (TASK-PI-04-02) additions.
	getLinkedPullRequestsForIssueUC := usecase.NewGetLinkedPullRequestsForIssue(credentials, registry)
	submitReviewUC := usecase.NewSubmitReview(credentials, registry)

	// BUG-PI-03/SOL-PI-03 (TASK-PI-03-06). outboxRepo is the SAME instance
	// createPullRequestUC/mergePullRequestUC already enqueue to above — one
	// outbox table, multiple publishers, per SOL-PI-03's design.
	receiveWebhookUC := usecase.NewReceiveWebhook(webhookVerifier, webhookDeliveries, outboxRepo)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	scmintegrationv1.RegisterScmIntegrationServiceServer(grpcServer, scmgrpc.New(
		listIssuesUC, createPullRequestUC, listPullRequestsUC, listWorkItemsUC, getRateLimitStatusUC,
		getAuthStatusUC, startOAuthFlowUC, completeOAuthFlowUC, revokeAuthUC,
		mergePullRequestUC, requestPullRequestReviewersUC, removePullRequestReviewersUC,
		setPullRequestAutoMergeUC, updateIssueUC, updatePullRequestUC, starRepositoryUC, getPullRequestForBranchUC, resolveRepoSlugUC,
		listAccessibleProjectsUC, resolveProjectRefUC, listProjectViewsUC, viewProjectTableUC,
		updateProjectItemFieldUC, clearProjectItemFieldUC, getWorkItemDetailsBySlugUC,
		updateIssueBySlugUC, updatePullRequestBySlugUC, updateIssueTypeBySlugUC,
		listIssueTypesBySlugUC, listAssignableUsersBySlugUC, listLabelsBySlugUC,
		addIssueCommentBySlugUC, updateIssueCommentBySlugUC, deleteIssueCommentBySlugUC,
		listMergeRequestsUC, resolveMergeRequestDiscussionUC, getWorkItemDetailsUC,
		checkHostedReviewEligibilityUC,
		setIntegrationCredentialUC, getIntegrationCredentialStatusUC, listIntegrationCredentialsUC,
		suggestPullRequestReviewersUC,
		listIssueCommentsBySlugUC, getLinkedPullRequestsForIssueUC, submitReviewUC,
		receiveWebhookUC,
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
		logger.Info("scm-integration-service grpc listening", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		logger.Info("scm-integration-service http (health) listening", slog.Int("port", cfg.HTTPPort))
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

	relayWG.Wait()

	return nil
}

// toMySQLDriverDSN converts a "mysql://"/"tidb://" DATABASE_DSN into
// go-sql-driver/mysql's own DSN format
// ("user:pass@tcp(host:port)/dbname?params") — copied verbatim from
// usage-service/cmd/server/main.go (BE-DB-SOL-002's pilot): pure
// DSN-plumbing, not specific to any one service. See that copy's doc
// comment for the two input shapes handled and why parseTime=true is
// always appended.
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
