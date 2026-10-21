package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type StartPhaseInput struct {
	RequestID string
	// PhaseTaskID may be empty for a Request without Phases: the whole Plan then starts.
	PhaseTaskID string
	// Internal skips the caller check; the status consumer starts Plans without Phases on its own.
	Internal bool
	// ActorID defaults to the user in ctx.
	ActorID string
}

type StartPhaseResult struct {
	PhaseTaskID    string
	AlreadyStarted bool
	Dispatched     []string
}

// StartPhaseAuthorizer decides whether the caller may start a Phase: the Request owner, an admin, or someone
// who may approve the Phase.
type StartPhaseAuthorizer interface {
	CanStart(ctx context.Context, req domain.Request, phaseApproval *domain.Approval) error
}

// StartPhase releases a Phase for execution. The phase_starts row is the claim, so pressing the button twice
// dispatches once; AdvanceExecution runs either way and is safe to repeat.
type StartPhase struct {
	Requests    RequestReader
	Tasks       TaskClient
	PhaseStarts PhaseStartRepository
	Approvals   ApprovalGateReader
	Authorizer  StartPhaseAuthorizer
	Advance     *AdvanceExecution
	// Access makes the actor able to execute the Plan tree once the caller is authorized to start; nil skips it.
	Access *ExecutionAccess
	Tx     TxRunner
	Outbox OutboxWriter
	// Join refuses the first Start while the test specs generated beside the Plan are not finished; nil disables it.
	Join *TestSpecJoin
}

func (uc *StartPhase) Execute(ctx context.Context, in StartPhaseInput) (StartPhaseResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return StartPhaseResult{}, domain.ErrRequestTenantRequired()
	}
	actor := in.ActorID
	if actor == "" {
		actor, _ = tenant.UserID(ctx)
	}
	if actor == "" {
		return StartPhaseResult{}, domain.ErrRequestReporterRequired()
	}
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return StartPhaseResult{}, err
	}
	if req.Status != domain.RequestStatusExecuting {
		return StartPhaseResult{}, domain.ErrRequestNotExecuting(req.Status)
	}
	tasks, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return StartPhaseResult{}, err
	}
	tree := domain.BuildExecutionTreeOf(req.PlanTaskID, tasks)
	if uc.Join != nil && !executionBegun(tree) {
		// The person pressed Start: say why nothing starts yet, before the Phase is claimed (a refused press leaves no trace).
		if err := uc.Join.Enforce(ctx, req); err != nil {
			return StartPhaseResult{}, err
		}
	}

	if in.PhaseTaskID == "" {
		return uc.startWholePlan(ctx, req, tasks, tree, in, actor)
	}
	phase, ok := tree.Phase(in.PhaseTaskID)
	if !ok || phase.RequestID != req.ID {
		return StartPhaseResult{}, domain.ErrPhaseNotInPlan(in.PhaseTaskID)
	}
	approvals, err := uc.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return StartPhaseResult{}, err
	}
	latest := domain.NewApprovalIndex(approvals).GetLatest(domain.SubjectPhase, phase.ID)
	if !in.Internal {
		if err := uc.Authorizer.CanStart(ctx, req, latest); err != nil {
			return StartPhaseResult{}, err
		}
	}
	flow, _ := domain.FlowFor(req.Type)
	// A continuous run has no Phase gate: the approval of the plan already covers every Phase.
	if flow.HasExecutionGateFor(domain.GatePhase, req.ExecutionMode) && (latest == nil || latest.Status != domain.ApprovalStatusApproved) {
		return StartPhaseResult{}, domain.ErrPhaseNotApproved(phase.ID)
	}
	if err := uc.ensureAccess(ctx, tasks, actor); err != nil {
		return StartPhaseResult{}, err
	}
	if err := uc.checkPredecessors(ctx, tree, phase, actor); err != nil {
		return StartPhaseResult{}, err
	}

	trigger := domain.PhaseStartManual
	if in.Internal {
		trigger = domain.PhaseStartAuto
	}
	inserted, err := claimPhase(ctx, uc.Tx, uc.PhaseStarts, uc.Outbox, req, phase.ID, actor, trigger)
	if err != nil {
		return StartPhaseResult{}, err
	}
	res, err := uc.Advance.Execute(ctx, AdvanceInput{RequestID: req.ID, ContainerID: phase.ID, ActorID: actor})
	return StartPhaseResult{PhaseTaskID: phase.ID, AlreadyStarted: !inserted, Dispatched: res.Dispatched}, err
}

// startWholePlan is the no-Phase path: the Plan itself is the unit that starts.
func (uc *StartPhase) startWholePlan(ctx context.Context, req domain.Request, tasks []domain.TaskView, tree domain.ExecutionTree, in StartPhaseInput, actor string) (StartPhaseResult, error) {
	if tree.HasPhases() {
		return StartPhaseResult{}, domain.ErrPhaseRequired()
	}
	if !in.Internal {
		if err := uc.Authorizer.CanStart(ctx, req, nil); err != nil {
			return StartPhaseResult{}, err
		}
	}
	if err := uc.ensureAccess(ctx, tasks, actor); err != nil {
		return StartPhaseResult{}, err
	}
	res, err := uc.Advance.Execute(ctx, AdvanceInput{RequestID: req.ID, ActorID: actor})
	return StartPhaseResult{Dispatched: res.Dispatched}, err
}

// ensureAccess runs after the caller is authorized to start and before the claim, so a person task-service would
// refuse leaves no phase_starts row behind that makes them the actor of every later dispatch.
func (uc *StartPhase) ensureAccess(ctx context.Context, tasks []domain.TaskView, actor string) error {
	if uc.Access == nil {
		return nil
	}
	return uc.Access.EnsureOnRoots(ctx, tasks, actor)
}

// checkPredecessors requires every Phase this one depends_on to be done or cancelled.
func (uc *StartPhase) checkPredecessors(ctx context.Context, tree domain.ExecutionTree, phase domain.TaskView, actor string) error {
	if tree.Plan == nil {
		return nil
	}
	sub, err := uc.Tasks.GetSubtree(asExecutionActor(ctx, actor), tree.Plan.ID)
	if err != nil {
		return err
	}
	// task-service hides what the actor has no grant on, and edges of a hidden Phase vanish with it: without this
	// check an actor who cannot see the Phase would pass the predecessor test with no edges to look at.
	if _, visible := findTask(sub.Tasks, phase.ID); !visible {
		return domain.ErrExecuteForbidden()
	}
	for _, edge := range sub.DependsOn {
		if edge.From != phase.ID {
			continue
		}
		if dep, ok := tree.Phase(edge.To); ok && !taskFinishedStatus(dep.Status) {
			return domain.ErrPhasePredecessorNotDone(phase.ID)
		}
	}
	return nil
}

// claimPhase records "this Phase was released" and emits phase.started in the same transaction. The phase_starts row is the
// claim, so releasing twice (a double click, a repeated event) dispatches once. trigger tells a person's click from Orca
// starting the Phase itself.
func claimPhase(ctx context.Context, tx TxRunner, starts PhaseStartRepository, outbox OutboxWriter, req domain.Request, phaseID, actor string, trigger domain.PhaseStartTrigger) (bool, error) {
	tenantID, _ := tenant.TenantID(ctx)
	var inserted bool
	err := tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		inserted, err = starts.TryStart(ctx, domain.PhaseStart{TenantID: tenantID, PhaseTaskID: phaseID, RequestID: req.ID, StartedBy: actor, Trigger: trigger})
		if err != nil || !inserted {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectPhaseStarted, domain.PhaseStartedPayload{RequestID: req.ID, PhaseTaskID: phaseID, StartedBy: actor})
		if err != nil {
			return err
		}
		return outbox.InsertOutboxEvent(ctx, ev)
	})
	return inserted, err
}

// ApprovalStartPhaseAuthorizer allows the Request's reporter, an admin, or anyone who could decide the Phase approval.
type ApprovalStartPhaseAuthorizer struct {
	Approvals ApprovalAuthorizer
}

func (a *ApprovalStartPhaseAuthorizer) CanStart(ctx context.Context, req domain.Request, phaseApproval *domain.Approval) error {
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	if userID == "" {
		return domain.ErrNoUser
	}
	if userID == req.ReporterID || role == "admin" {
		return nil
	}
	if phaseApproval != nil && a.Approvals != nil {
		err := a.Approvals.CanDecide(ctx, req, *phaseApproval)
		// A requester who is also an approver is still someone who may approve the Phase.
		if err == nil || errors.Is(err, domain.ErrSelfApprovalForbidden) {
			return nil
		}
	}
	return domain.ErrForbiddenToStart()
}
