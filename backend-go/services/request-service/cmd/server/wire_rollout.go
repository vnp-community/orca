package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/stablyai/orca-go/common/auditclient"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/audit"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/metrics"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
)

// rolloutWiring holds CR-REQ-024/025: the request_flow_enabled gate, audit, metrics and the internal lookup.
type rolloutWiring struct {
	Flow    *usecase.FlowSettings
	Metrics *metrics.Set
	Audit   usecase.RPCAuditRecorder

	cfg     config.Config
	log     *slog.Logger
	stores  *requestStores
	closers []func()
}

// wireRollout must run right after openStores and before any use case is built: it wraps the
// transaction runners and the outbox writer in stores so audit and metrics are derived from committed
// events, without the use cases knowing about either.
func wireRollout(cfg config.Config, log *slog.Logger, stores *requestStores) (*rolloutWiring, error) {
	w := &rolloutWiring{cfg: cfg, log: log, stores: stores, Metrics: metrics.New(), Audit: usecase.NoopRPCAuditRecorder{}}
	if cfg.AuthServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.AuthServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		w.Audit = audit.NewAuthAuditRecorder(auditclient.New(authv1.NewAuthServiceClient(conn)))
	} else {
		log.Warn("AUTH_SERVICE_ADDR unset: Request decisions are not written to the audit log")
	}
	stores.observer = w.Metrics
	w.Flow = usecase.NewFlowSettings(stores.flowSettings, cfg.RequestFlowEnabled, w.Audit, log)

	hooked := usecase.CommitHooks{Inner: stores.txScope}
	stores.tx, stores.txScope = hooked, hooked
	stores.outboxWriter = &usecase.TappedOutbox{Inner: stores.outboxWriter, Taps: []usecase.OutboxTap{
		&usecase.AuditTap{Recorder: w.Audit, Log: log},
		w.Metrics,
	}}
	return w, nil
}

// SecurityHooks places the flag gate right after actor type and the RPC audit right after authz, so a gated
// call is not audited as a decision and audit sees the authorized actor. The internal-caller guard for
// LookupRequestBySource now comes from the security chain (RPC catalog marks it internal).
func (w *rolloutWiring) SecurityHooks() securityHooks {
	return securityHooks{
		AfterGuard: []grpc.UnaryServerInterceptor{requestgrpc.FlowGate(w.Flow.Effective)},
		AfterAuthz: []grpc.UnaryServerInterceptor{requestgrpc.AuditRPC(w.Audit)},
	}
}

// ServerOptions adds the stream flag gate; unary gating goes through SecurityHooks.
func (w *rolloutWiring) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.ChainStreamInterceptor(requestgrpc.StreamFlowGate(w.Flow.Effective))}
}

// AttachLookup gives LookupRequestBySource its internal-RPC semantics: active Requests only, any site when
// none is given, and not-found while the flow is off so issue-status-sync keeps its worktree/PR sync.
func (w *rolloutWiring) AttachLookup(i *intakeWiring) {
	i.lookup = usecase.NewLookupRequestBySource(w.stores.requests, w.stores.idempotency,
		usecase.WithActiveSourceFinder(w.stores.sourceFinder), usecase.WithLookupFlagGate(w.Flow.Effective))
}

// WithRPCs attaches the flag RPCs to the gRPC server.
func (w *rolloutWiring) WithRPCs(s *requestgrpc.Server) *requestgrpc.Server {
	return s.WithFlowSettings(w.Flow)
}

// MetricsHandler serves /metrics next to the health routes.
func (w *rolloutWiring) MetricsHandler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", w.Metrics.Handler())
	mux.Handle("/", next)
	return mux
}

// StartSampler refreshes the database gauges until ctx ends; wg lets shutdown wait for it.
func (w *rolloutWiring) StartSampler(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Metrics.RunSampler(ctx, w.stores.samples, metrics.StuckThresholds(w.cfg.StuckThresholds), w.cfg.MetricsSampleInterval, w.log)
	}()
}

func (w *rolloutWiring) close() {
	for _, c := range w.closers {
		c()
	}
}
