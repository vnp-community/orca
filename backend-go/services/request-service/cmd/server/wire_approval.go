package main

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sync"
	"time"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// approvalSweepBatch bounds how many approvals one sweeper tick expires or reminds.
const approvalSweepBatch = 100

// approvalDirectories are the team and admin lookups behind approver checks and notification recipients.
// Without the service address they fail closed (REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE), never return "nobody".
type approvalDirectories struct {
	teams   usecase.TeamMembershipResolver
	admins  usecase.AdminDirectoryResolver
	closers []func()
}

func dialApprovalDirectories(cfg config.Config, log *slog.Logger) (*approvalDirectories, error) {
	d := &approvalDirectories{teams: grpcclient.UnavailableDirectory{}, admins: grpcclient.UnavailableDirectory{}}
	if cfg.TenantServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.TenantServiceAddr)
		if err != nil {
			return nil, err
		}
		d.closers = append(d.closers, func() { _ = conn.Close() })
		d.teams = grpcclient.NewTeamMembershipResolver(tenantv1.NewTenantServiceClient(conn), 0)
	} else {
		log.Warn("TENANT_SERVICE_ADDR unset: team approvers cannot be resolved")
	}
	if cfg.AuthServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.AuthServiceAddr)
		if err != nil {
			d.close()
			return nil, err
		}
		d.closers = append(d.closers, func() { _ = conn.Close() })
		d.admins = grpcclient.NewAdminDirectoryResolver(authv1.NewAuthServiceClient(conn), 0)
	} else {
		log.Warn("AUTH_SERVICE_ADDR unset: role:admin approvers cannot be expanded to users for notifications")
	}
	return d, nil
}

func (d *approvalDirectories) close() {
	for _, c := range d.closers {
		c()
	}
}

// newApprovalNotifier builds the enrichment applied by the outbox relay to orca.request.approval.* events.
func newApprovalNotifier(cfg config.Config, log *slog.Logger, stores *requestStores, dirs *approvalDirectories) *usecase.PublishApprovalNotifications {
	return &usecase.PublishApprovalNotifications{Expander: &usecase.ExpandApprovalRecipients{
		ApproverRepo: stores.approvers, TeamResolver: dirs.teams, AdminResolver: dirs.admins,
		MaxRecipients: cfg.ApprovalNotifyMaxRecipients, Log: log,
	}}
}

// approvalSubjectArtifacts is where the Solution, Plan, Phase, Findings, Answer, Task list and pre-deploy owners
// plug their SubjectArtifacts in (CR-REQ-007/008/012/013/014). Unbound subjects refuse to open (fail closed).
func approvalSubjectArtifacts(extra ...map[domain.SubjectType]usecase.SubjectArtifacts) map[domain.SubjectType]usecase.SubjectArtifacts {
	out := map[domain.SubjectType]usecase.SubjectArtifacts{}
	for _, m := range extra {
		for st, a := range m {
			out[st] = a
		}
	}
	return out
}

type approvalWiring struct {
	// Recorder is what classification uses to open and settle the request_type approval.
	Recorder usecase.ApprovalRecorder
	Server   requestgrpc.ApprovalUseCases
	Admin    *usecase.ManageApprovalPolicies
	Enabled  bool
	// Open and Authorizer are what execution needs to open phase/pre_deploy approvals and to check who may start a Phase.
	Open       *usecase.OpenApproval
	Authorizer *usecase.AuthorizeApprovalDecision

	open        *usecase.OpenApproval
	typeHandler *usecase.RequestTypeApprovalHandler
	expire      *usecase.ExpireApprovals
	remind      *usecase.RemindPendingApprovals
}

// BindConfirm completes the request_type handler once ConfirmRequestType exists (it needs Recorder first).
func (w *approvalWiring) BindConfirm(c usecase.RequestTypeConfirmer) {
	if w.typeHandler != nil {
		w.typeHandler.Confirm = c
	}
}

// wireApproval builds CR-REQ-009/010. owned holds handlers that subject owners bring themselves (CR-REQ-007/008);
// they replace the generic TransitionSubjectHandler for their subjects.
// It and starts the expiry/reminder sweeper. registry is nil when approval is off:
// everything then degrades to the no-op recorder and no sweeper runs.
func wireApproval(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores,
	lifecycle *requestLifecycle, registry *usecase.SubjectHandlerRegistry, dirs *approvalDirectories,
	owned map[domain.SubjectType]usecase.SubjectHandler, artifacts map[domain.SubjectType]usecase.SubjectArtifacts) (*approvalWiring, error) {
	if registry == nil {
		return &approvalWiring{Recorder: usecase.NoopApprovalRecorder{}}, nil
	}
	w := &approvalWiring{Enabled: true}
	w.typeHandler = &usecase.RequestTypeApprovalHandler{Returner: lifecycle.Return, Requests: stores.locker}
	real := map[domain.SubjectType]usecase.SubjectHandler{domain.SubjectRequestType: w.typeHandler}
	artifacts = approvalSubjectArtifacts(artifacts)
	for _, st := range domain.AllSubjectTypes {
		if st == domain.SubjectRequestType {
			continue
		}
		if h := owned[st]; h != nil {
			real[st] = h
			continue
		}
		real[st] = &usecase.TransitionSubjectHandler{Artifacts: artifacts[st], Transition: lifecycle.Transition, Returner: lifecycle.Return, Requests: stores.locker}
		if artifacts[st] == nil {
			log.Info("approval subject has no backing service bound; opening it is refused", slog.String("subject_type", string(st)))
		}
	}
	if err := fillApprovalRegistry(cfg, log, registry, real); err != nil {
		return nil, fmt.Errorf("approval subject handler registry: %w", err)
	}

	resolver := &usecase.ResolveApproverPolicy{Repo: stores.policies}
	authorizer := &usecase.AuthorizeApprovalDecision{ApproverRepo: stores.approvers, Teams: dirs.teams}
	open := &usecase.OpenApproval{
		Repo: stores.approvals, Tx: stores.txScope, Requests: stores.requests, Locker: stores.locker, Registry: registry, Resolver: resolver,
		ApproverRepo: stores.approvers, Outbox: stores.outboxWriter, Teams: dirs.teams, Admins: dirs.admins, Log: log,
	}
	w.open = open
	w.Open, w.Authorizer = open, authorizer
	w.expire = &usecase.ExpireApprovals{
		Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker, Registry: registry, Returner: lifecycle.Return, Outbox: stores.outboxWriter, Log: log,
	}
	w.remind = &usecase.RemindPendingApprovals{Repo: stores.approvals, Requests: stores.requests, Tx: stores.tx, Outbox: stores.outboxWriter, Log: log}
	w.Recorder = &usecase.RequestTypeApprovalRecorder{Open: open, Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker, Outbox: stores.outboxWriter}
	w.Server = requestgrpc.ApprovalUseCases{
		Request: &usecase.RequestApprovalFromAPI{Open: open, Repo: stores.approvals},
		Decide: &usecase.DecideApproval{
			Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker, Registry: registry, Outbox: stores.outboxWriter, Expirer: w.expire,
			Authorizer: authorizer,
		},
		Cancel:  &usecase.CancelApproval{Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker, Registry: registry, Outbox: stores.outboxWriter},
		Get:     &usecase.GetApproval{Repo: stores.approvals},
		List:    &usecase.ListApprovals{Repo: stores.approvals},
		Pending: &usecase.ListPendingApprovalsForUser{Repo: stores.approvals, Teams: dirs.teams, Log: log},
		Extend:  &usecase.ExtendApproval{Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker},
	}
	w.Admin = &usecase.ManageApprovalPolicies{Repo: stores.policies}

	// A forgotten dependency would otherwise surface as a nil-pointer panic on the first approval; fail at start-up instead.
	for name, v := range map[string]any{
		"OpenApproval": open, "ExpireApprovals": w.expire, "RemindPendingApprovals": w.remind, "RequestTypeApprovalRecorder": w.Recorder,
		"RequestApprovalFromAPI": w.Server.Request, "DecideApproval": w.Server.Decide, "CancelApproval": w.Server.Cancel, "GetApproval": w.Server.Get,
		"ListApprovals": w.Server.List, "ListPendingApprovalsForUser": w.Server.Pending, "ExtendApproval": w.Server.Extend,
		"ManageApprovalPolicies": w.Admin, "RequestTypeApprovalHandler": w.typeHandler,
	} {
		if err := requireWired(name, v, "Log", "Confirm"); err != nil { // Confirm is bound later by BindConfirm
			return nil, err
		}
	}

	wg.Add(1)
	go func() { defer wg.Done(); w.runSweeper(ctx, log, cfg.ApprovalSweepInterval) }()
	return w, nil
}

// runSweeper expires and reminds on a fixed tick. Each claim runs under its own tenant (see ExpireApprovals).
func (w *approvalWiring) runSweeper(ctx context.Context, log *slog.Logger, every time.Duration) {
	if every <= 0 {
		every = time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		w.sweepOnce(ctx, log)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *approvalWiring) sweepOnce(ctx context.Context, log *slog.Logger) {
	if n, err := w.expire.Execute(ctx, approvalSweepBatch); err != nil {
		log.Warn("approval expiry sweep failed", slog.Any("error", err))
	} else if n > 0 {
		log.Info("approvals expired", slog.Int("count", n))
	}
	if n, err := w.remind.Execute(ctx, approvalSweepBatch); err != nil {
		log.Warn("approval reminder sweep failed", slog.Any("error", err))
	} else if n > 0 {
		log.Info("approval reminders sent", slog.Int("count", n))
	}
}

// requireWired reports the first nil dependency field of the struct v points to, except the optional ones.
func requireWired(name string, v any, optional ...string) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("wiring: %s is not a non-nil struct pointer", name)
	}
	st := rv.Elem()
	for i := 0; i < st.NumField(); i++ {
		f := st.Type().Field(i)
		if !f.IsExported() || slices.Contains(optional, f.Name) {
			continue
		}
		switch fv := st.Field(i); fv.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Func, reflect.Slice:
			if fv.IsNil() {
				return fmt.Errorf("wiring: %s.%s is nil", name, f.Name)
			}
		}
	}
	return nil
}

// SolutionOpener lets the solution worker open its approval in the transaction that stores the proposal.
// Nil when approval is off.
func (w *approvalWiring) SolutionOpener() usecase.SolutionApprovalOpener {
	if w.open == nil {
		return nil
	}
	return usecase.OpenApprovalOpener{Approvals: w.open}
}
