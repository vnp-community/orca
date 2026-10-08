package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	mysqladapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/mysql"
	postgresadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// executionStores are the stores of CR-REQ-013/014/015, built per dialect next to the shared ones.
type executionStores struct {
	phaseStarts   usecase.PhaseStartRepository
	outcomes      usecase.TaskRunOutcomeRepository
	checks        usecase.RequestCheckRepository
	scanner       usecase.ExecutingRequestScanner
	leases        usecase.ReconcileLeases
	gateApprovals usecase.ApprovalGateReader
	backlog       usecase.BacklogRequestReader
}

func postgresExecutionStores(base *postgresadapter.Repository) executionStores {
	executing := postgresadapter.NewExecutingRequestRepository(base)
	return executionStores{
		phaseStarts: postgresadapter.NewPhaseStartRepository(base), outcomes: postgresadapter.NewTaskRunOutcomeRepository(base),
		checks: postgresadapter.NewRequestCheckRepository(base), scanner: executing, leases: executing,
		gateApprovals: postgresadapter.NewApprovalGateReader(base), backlog: postgresadapter.NewBacklogRequestReader(base),
	}
}

func mysqlExecutionStores(base *mysqladapter.Repository) executionStores {
	executing := mysqladapter.NewExecutingRequestRepository(base)
	return executionStores{
		phaseStarts: mysqladapter.NewPhaseStartRepository(base), outcomes: mysqladapter.NewTaskRunOutcomeRepository(base),
		checks: mysqladapter.NewRequestCheckRepository(base), scanner: executing, leases: executing,
		gateApprovals: mysqladapter.NewApprovalGateReader(base), backlog: mysqladapter.NewBacklogRequestReader(base),
	}
}

// executionTasks is the task-service door plus what other wiring needs from it before execution is built:
// the guard for return/cancel and the approval artifacts for the phase and pre_deploy subjects.
type executionTasks struct {
	client  usecase.TaskClient
	guard   usecase.ExecutionGuard
	closers []func()
}

// dialExecutionTasks never fails on a missing address: the service starts, and every use of task-service reports it.
func dialExecutionTasks(cfg config.Config, log *slog.Logger) (*executionTasks, error) {
	t := &executionTasks{client: grpcclient.UnavailableTaskClient{}}
	if cfg.TaskServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.TaskServiceAddr)
		if err != nil {
			return nil, err
		}
		t.closers = append(t.closers, func() { _ = conn.Close() })
		t.client = grpcclient.NewTaskClient(taskv1.NewTaskServiceClient(conn))
	} else {
		log.Warn("TASK_SERVICE_ADDR unset: execution and the task backlog views are unavailable, and returning an executing request is refused")
	}
	t.guard = &usecase.TaskExecutionGuard{Tasks: t.client}
	return t, nil
}

func (t *executionTasks) subjectArtifacts() map[domain.SubjectType]usecase.SubjectArtifacts {
	return map[domain.SubjectType]usecase.SubjectArtifacts{
		domain.SubjectPhase:     &usecase.PhaseArtifacts{Tasks: t.client},
		domain.SubjectPreDeploy: &usecase.PreDeployArtifacts{Tasks: t.client},
	}
}

func (t *executionTasks) close() {
	for _, c := range t.closers {
		c()
	}
}

type executionWiring struct {
	Server  requestgrpc.ExecutionUseCases
	closers []func()
}

func (w *executionWiring) close() {
	for _, c := range w.closers {
		c()
	}
}

// wireExecution builds the execution loop (CR-REQ-013), the type policies (CR-REQ-014) and the backlog views
// (CR-REQ-015) and starts their background parts: the task-outcome, status and gate consumers (need NATS) and
// the reconcile loop.
func wireExecution(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores,
	lifecycle *requestLifecycle, approval *approvalWiring, tasks *executionTasks, bus *commoneventbus.Consumer) (*executionWiring, error) {
	w := &executionWiring{}
	ex := stores.execution
	settings := usecase.ExecutionSettings{
		MaxParallelTasks: cfg.MaxParallelTasks, MaxTaskAttempts: cfg.MaxTaskAttempts, AutoCompleteTasks: cfg.AutoCompleteTasks,
		DispatchRetryWindow: cfg.DispatchRetryWindow, ReconcileQuiet: cfg.ReconcileQuiet,
	}
	policies := domain.NewPolicyRegistry(domain.PolicyDeps{
		Checks:    &usecase.CheckReaderFromRepository{Checks: ex.checks},
		Approvals: &usecase.ApprovalLookupFromGateReader{Approvals: ex.gateApprovals},
	})
	var opener usecase.ApprovalOpener // nil when approval is off: gated tasks then refuse to dispatch
	var authorizer usecase.ApprovalAuthorizer
	if approval.Enabled {
		opener, authorizer = approval.Open, approval.Authorizer
	}
	var members usecase.ProjectMembership = grpcclient.NoProjectMembership{}
	if cfg.ProjectServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.ProjectServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		members = grpcclient.NewProjectMembershipClient(projectv1.NewProjectServiceClient(conn))
	} else {
		log.Warn("PROJECT_SERVICE_ADDR unset: backlog views and checks show a request only to its reporter and admins")
	}
	visibility := &usecase.MemberRequestVisibility{Members: members}

	actors := &usecase.ApprovalExecutionActors{Approvals: ex.gateApprovals, PhaseStarts: ex.phaseStarts}
	advance := &usecase.AdvanceExecution{
		Requests: stores.requests, Tasks: tasks.client, Outcomes: ex.outcomes, PhaseStarts: ex.phaseStarts, Policies: policies,
		Approvals: opener, Settings: settings, Log: log,
	}
	evaluate := &usecase.EvaluateExecution{
		Tasks: tasks.client, Outcomes: ex.outcomes, PhaseStarts: ex.phaseStarts, Checks: ex.checks, Approvals: ex.gateApprovals, Open: opener,
		Advance: advance, Actors: actors, Policies: policies, Transition: lifecycle.Transition, Returner: lifecycle.Return,
		FollowUps: &usecase.SpawnFollowUps{Spawn: lifecycle.SpawnChild, Log: log}, Tx: stores.tx, Outbox: stores.outboxWriter, Settings: settings, Log: log,
	}
	report := &usecase.ReportTaskOutcome{
		Requests: stores.requests, Tasks: tasks.client, Outcomes: ex.outcomes, Processed: stores.processed, Evaluate: evaluate,
		Tx: stores.tx, Outbox: stores.outboxWriter, Settings: settings, Log: log,
	}
	startExecution := &usecase.StartExecution{Requests: stores.requests, Evaluate: evaluate}
	resume := &usecase.ResumeAfterGate{Requests: stores.requests, Evaluate: evaluate}
	host, _ := os.Hostname()
	reconcile := &usecase.ReconcileExecutingRequests{
		Scanner: ex.scanner, Leases: ex.leases, Requests: stores.requests, Evaluate: evaluate, Settings: settings,
		Owner: fmt.Sprintf("%s-%d", host, os.Getpid()), Log: log,
	}
	w.Server = requestgrpc.ExecutionUseCases{
		StartPhase: &usecase.StartPhase{
			Requests: stores.requests, Tasks: tasks.client, PhaseStarts: ex.phaseStarts, Approvals: ex.gateApprovals,
			Authorizer: &usecase.ApprovalStartPhaseAuthorizer{Approvals: authorizer}, Advance: advance, Tx: stores.tx, Outbox: stores.outboxWriter,
		},
		ReportOutcome: report,
		RecordCheck: &usecase.RecordRequestCheck{
			Requests: stores.requests, Checks: ex.checks, Authorizer: &usecase.ParticipantWriteAuthorizer{Approvals: ex.gateApprovals},
		},
		ListChecks: &usecase.ListRequestChecks{Requests: stores.requests, Checks: ex.checks, Visibility: visibility},
		Backlog: &usecase.ListBacklog{
			Requests: &usecase.ListBacklogRequests{Reader: ex.backlog, Visibility: visibility},
			Tasks: &usecase.ListBacklogTasks{
				Requests: ex.backlog, Approvals: ex.gateApprovals, Tasks: tasks.client, Outcomes: ex.outcomes, Visibility: visibility, Log: log,
			},
		},
	}

	// A forgotten dependency would surface as a nil-pointer panic on the first event; fail at start-up instead.
	optional := []string{"Log", "Now", "Approvals", "Open", "FollowUps", "Authorizer"}
	for name, v := range map[string]any{
		"AdvanceExecution": advance, "EvaluateExecution": evaluate, "ReportTaskOutcome": report, "StartExecution": startExecution,
		"ResumeAfterGate": resume, "ReconcileExecutingRequests": reconcile, "StartPhase": w.Server.StartPhase,
		"RecordRequestCheck": w.Server.RecordCheck, "ListRequestChecks": w.Server.ListChecks,
		"ListBacklogRequests": w.Server.Backlog.Requests, "ListBacklogTasks": w.Server.Backlog.Tasks,
	} {
		if err := requireWired(name, v, optional...); err != nil {
			return nil, err
		}
	}

	wg.Add(1)
	go func() { defer wg.Done(); reconcile.RunReconcileLoop(ctx, cfg.ReconcileInterval) }()
	if bus == nil {
		log.Warn("eventbus unavailable: execution is driven only by StartPhase and the reconcile loop")
		return w, nil
	}
	run := func(name string, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(ctx); err != nil {
				log.Warn("execution consumer stopped", slog.String("consumer", name), slog.Any("error", err))
			}
		}()
	}
	run("task-outcome", eventbusadapter.NewTaskOutcomeConsumer(bus, report).Run)
	run("request-status", eventbusadapter.NewRequestStatusConsumer(bus, startExecution).Run)
	run("approval-gate", eventbusadapter.NewApprovalDecidedConsumer(bus, resume).Run)
	return w, nil
}
