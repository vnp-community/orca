package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/auditdelivery"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/opaclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
)

type securityWiring struct {
	options    []grpc.ServerOption
	compliance requestgrpc.ComplianceUseCases
	replay     *usecase.WebhookReplayGuard
	authorize  *usecase.AuthorizeRequestAction
	closers    []func()
}

func (w *securityWiring) close() {
	for _, c := range w.closers {
		c()
	}
}

// wireSecurity builds the interceptor chain (CR-REQ-035) and starts the security background jobs.
// Startup fails when the policy bundle cannot be evaluated or an RPC is missing from the catalog:
// an unclassified RPC would be unprotected.
// securityHooks are the plug points other features use in the shared chain; order is kept.
type securityHooks struct {
	AfterGuard []grpc.UnaryServerInterceptor // feature flag gate
	AfterAuthz []grpc.UnaryServerInterceptor // RPC audit
}

func wireSecurity(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores, hooks securityHooks) (*securityWiring, error) {
	w := &securityWiring{}
	if err := domain.ValidateCatalog(&requestv1.RequestService_ServiceDesc, &requestv1.ApprovalService_ServiceDesc,
		&requestv1.ApprovalPolicyAdminService_ServiceDesc, &requestv1.AiBudgetAdminService_ServiceDesc); err != nil {
		return nil, fmt.Errorf("security: %w", err)
	}
	policy := opaclient.NewRequestPolicy(cfg.OPABundlePath)
	if err := policy.Warm(ctx); err != nil {
		return nil, fmt.Errorf("security: request policy bundle %q: %w", cfg.OPABundlePath, err)
	}
	if cfg.GatewayInternalToken == "" || cfg.ServiceInternalToken == "" {
		log.Error("GATEWAY_INTERNAL_TOKEN or SERVICE_INTERNAL_TOKEN is empty: the matching RPCs refuse every call until it is set")
	}

	var roles usecase.ProjectRoleResolver = unavailableProjectRoles{}
	if cfg.ProjectServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.ProjectServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		roles = grpcclient.NewProjectRoleResolver(projectv1.NewProjectServiceClient(conn))
	} else {
		log.Warn("PROJECT_SERVICE_ADDR unset: only global admins pass the project-role check")
	}

	var sink usecase.BestEffortAudit
	if cfg.AuthServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.AuthServiceAddr)
		if err != nil {
			w.close()
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		client := auditclient.New(authv1.NewAuthServiceClient(conn))
		sink = client
		d := &auditdelivery.Deliverer{Store: stores.auditOutbox, Client: client, Log: log}
		wg.Add(1)
		go func() { defer wg.Done(); d.Run(ctx) }()
	} else {
		log.Warn("AUTH_SERVICE_ADDR unset: audit entries are queued in request_audit_outbox but not delivered")
	}
	audit := usecase.NewAuditRecorder(sink, stores.auditOutbox, stores.tx, log)

	authorize := usecase.NewAuthorizeRequestAction(stores.requests, usecase.NewApprovalRequestLocator(stores.approvals), roles, policy, audit)
	w.authorize = authorize
	w.options = requestgrpc.SecurityChain(requestgrpc.SecurityChainConfig{
		GatewayToken: cfg.GatewayInternalToken,
		ServiceToken: cfg.ServiceInternalToken,
		Authorize:    authorize,
		Limiter:      usecase.NewRateLimiter(domain.DefaultLimits, nil),
		Gate:         usecase.NewConcurrencyGate(domain.DefaultConcurrencyCaps, stores.concurrency),
		AfterGuard:   hooks.AfterGuard,
		AfterAuthz:   hooks.AfterAuthz,
	})

	key := []byte(cfg.EraseHMACKey)
	if len(key) == 0 {
		log.Warn("REQUEST_ERASE_HMAC_KEY unset: EraseRequest and the retention job refuse to run")
	}
	w.compliance = requestgrpc.ComplianceUseCases{
		Erase: usecase.NewEraseRequest(stores.requests, stores.retention, stores.tx, audit, usecase.UnsupportedTaskContentEraser{}, key, nil),
		Export: usecase.NewExportRequest(usecase.ExportSources{
			Requests: stores.requests, History: stores.history, Solutions: stores.solutions,
			Approvals: stores.approvals, Links: stores.links, Flags: stores.secFlags,
		}, audit, nil),
		ExportAll: usecase.NewExportTenantRequests(stores.requests, stores.secFlags, audit),
	}
	w.replay = usecase.NewWebhookReplayGuard(stores.nonces, nil)

	startRetentionJobs(ctx, wg, cfg, log, stores, audit, key)
	return w, nil
}

// unavailableProjectRoles refuses project-scoped checks when project-service is not configured (fail closed).
type unavailableProjectRoles struct{}

var errNoProjectService = errors.New("project-service is not configured")

func (unavailableProjectRoles) RoleOf(context.Context, string, string, string) (string, error) {
	return "", errNoProjectService
}

func (unavailableProjectRoles) ProjectsOf(context.Context, string, string) ([]string, error) {
	return nil, errNoProjectService
}

const retentionStartupDelay = 30 * time.Second

// startRetentionJobs runs the daily retention pass and the minute nonce prune until ctx ends.
func startRetentionJobs(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores, audit *usecase.AuditRecorder, key []byte) {
	retention := usecase.NewRunRequestRetention(stores.retention, audit, key, nil, log)
	runAt := parseRunAt(cfg.RetentionRunAt, log)
	wg.Add(1)
	go func() {
		defer wg.Done()
		wait := retentionStartupDelay // a restart that skipped the daily slot still catches up
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			if len(key) > 0 {
				sum, err := retention.Execute(tenant.WithActorType(ctx, tenant.ActorSystem))
				if err != nil && ctx.Err() == nil {
					log.Warn("retention run failed", slog.Any("error", err))
				} else {
					log.Info("retention run finished", slog.Int("tenants", sum.Tenants), slog.Int("anonymized", sum.Anonymized))
				}
			}
			wait = untilNext(time.Now().UTC(), runAt)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if _, err := stores.nonces.PruneExpired(ctx, time.Now(), 500); err != nil && ctx.Err() == nil {
					log.Warn("webhook nonce prune failed", slog.Any("error", err))
				}
			}
		}
	}()
}

type timeOfDay struct{ hour, min int }

func parseRunAt(s string, log *slog.Logger) timeOfDay {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		log.Warn("REQUEST_RETENTION_RUN_AT is not HH:MM, using 03:00", slog.String("value", s))
		return timeOfDay{3, 0}
	}
	return timeOfDay{hh, mm}
}

// untilNext is the wait from now (UTC) to the next occurrence of at.
func untilNext(now time.Time, at timeOfDay) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), at.hour, at.min, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}
