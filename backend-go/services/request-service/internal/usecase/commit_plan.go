package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type CommitPlanInput struct {
	RequestID string
	Proposal  domain.PlanProposal
}

type CommitPlanResult struct {
	PlanTaskID    string
	PhaseTaskIDs  []string
	TaskIDs       []string
	AlreadyExists bool
}

type planGeneratedPayload struct {
	RequestID    string   `json:"request_id"`
	PlanTaskID   string   `json:"plan_task_id"`
	PhaseTaskIDs []string `json:"phase_task_ids"`
	TaskIDs      []string `json:"task_ids"`
	Type         string   `json:"type"`
	Size         string   `json:"size,omitempty"`
	TaskCount    int      `json:"task_count"`
	Replan       bool     `json:"replan,omitempty"`
}

// CommitPlan is the COMMIT step: it re-validates the proposal (a client or a model may have changed it), has
// task-service write the whole tree in one transaction, then records the Plan on the Request, moves it to
// awaiting_plan_approval and opens the approval, all in one request-service transaction. Every step is safe to
// repeat, so a commit that died half way is finished by calling it again.
type CommitPlan struct {
	requests   RequestRepository
	auth       PlanActorAuthorizer
	writer     TaskPlanWriter
	policies   PlanPolicies
	approvals  ApprovalOpener
	transition RequestTransitioner
	tx         TxRunner
	outbox     OutboxWriter
	limits     domain.PlanLimits
}

type CommitPlanDeps struct {
	Requests   RequestRepository
	Auth       PlanActorAuthorizer
	Writer     TaskPlanWriter
	Policies   PlanPolicies
	Approvals  ApprovalOpener
	Transition RequestTransitioner
	Tx         TxRunner
	Outbox     OutboxWriter
	Limits     domain.PlanLimits
}

func NewCommitPlan(d CommitPlanDeps) *CommitPlan {
	return &CommitPlan{requests: d.Requests, auth: d.Auth, writer: d.Writer, policies: d.Policies, approvals: d.Approvals,
		transition: d.Transition, tx: d.Tx, outbox: d.Outbox, limits: d.Limits}
}

func (uc *CommitPlan) Execute(ctx context.Context, in CommitPlanInput) (CommitPlanResult, error) {
	return uc.run(ctx, in, false)
}

// ExecuteForSystem commits a proposal the server itself obtained (CR-REQ-044 automatic Plan, CR-REQ-045 revision):
// nobody is calling, so the reporter is the actor. Only internal workers call it, never an RPC.
func (uc *CommitPlan) ExecuteForSystem(ctx context.Context, in CommitPlanInput) (CommitPlanResult, error) {
	return uc.run(ctx, in, true)
}

func (uc *CommitPlan) run(ctx context.Context, in CommitPlanInput, system bool) (CommitPlanResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return CommitPlanResult{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" && !system {
		return CommitPlanResult{}, domain.ErrRequestReporterRequired()
	}
	req, err := uc.requests.Get(ctx, in.RequestID)
	if err != nil {
		return CommitPlanResult{}, err
	}
	if system {
		if actor == "" {
			actor = req.ReporterID
		}
		if actor == "" {
			return CommitPlanResult{}, domain.ErrRequestReporterRequired()
		}
		ctx = tenant.WithUserID(ctx, actor)
	} else if err := uc.auth.AuthorizePlan(ctx, req); err != nil {
		return CommitPlanResult{}, err
	}
	// A Plan that is already waiting for approval is a replay of a commit that succeeded: answer from task-service.
	replay := req.Status == domain.RequestStatusAwaitingPlanApproval && req.PlanTaskID != ""
	if req.Status != domain.RequestStatusPlanning && !replay {
		return CommitPlanResult{}, domain.ErrPlanWrongState(req.Status)
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return CommitPlanResult{}, domain.ErrPlanNotApplicable(req.Type)
	}
	shape := domain.PlanShapeFor(flow, req.Size)
	proposal := domain.NormalizePlanProposal(in.Proposal)
	if err := domain.ValidatePlanProposal(req.Type, shape, uc.limits, proposal); err != nil {
		return CommitPlanResult{}, err
	}
	if err := uc.policies.PolicyFor(req.Type).PlanPreconditions(ctx, req, proposal); err != nil {
		return CommitPlanResult{}, err
	}
	stored, err := EncodePlanProposal(proposal)
	if err != nil {
		return CommitPlanResult{}, err
	}

	// task-service is idempotent per Request, so it is called before our transaction: a failure here changes nothing.
	treeIn := PlanTreeInput{Request: req, Shape: shape, Proposal: proposal, AIPlanJSON: stored, CreatorID: actor}
	if !replay {
		treeIn.SupersedesPlanID = req.PlanTaskID // a replan retires the Plan it replaces
	}
	tree, err := uc.writer.CreatePlanTree(ctx, treeIn)
	if errorHasCode(err, "REQUEST_PLAN_SUPERSEDE_MISMATCH") && treeIn.SupersedesPlanID != "" {
		// The Plan recorded here is no longer the active one in task-service (cancelled meanwhile); ask for a plain create.
		treeIn.SupersedesPlanID = ""
		tree, err = uc.writer.CreatePlanTree(ctx, treeIn)
	}
	if err != nil {
		return CommitPlanResult{}, err
	}
	result := CommitPlanResult{PlanTaskID: tree.PlanTaskID, PhaseTaskIDs: tree.PhaseTaskIDs, TaskIDs: tree.TaskIDs, AlreadyExists: tree.AlreadyExists}
	if replay {
		return result, nil
	}
	if err := uc.record(ctx, req, flow, tree, req.PlanTaskID != "", actor); err != nil {
		return CommitPlanResult{}, err
	}
	return result, nil
}

// record is the request-service half: Plan id, event, status change and approval together.
func (uc *CommitPlan) record(ctx context.Context, req domain.Request, flow domain.FlowDefinition, tree PlanTreeResult, replan bool, actor string) error {
	return uc.tx.InTx(ctx, func(ctx context.Context) error {
		cur, err := uc.requests.Get(ctx, req.ID)
		if err != nil {
			return err
		}
		if cur.Status == domain.RequestStatusAwaitingPlanApproval && cur.PlanTaskID == tree.PlanTaskID {
			return nil // the other side of a commit race (server or browser) already recorded this very Plan
		}
		if cur.Status != domain.RequestStatusPlanning {
			return domain.ErrPlanWrongState(cur.Status)
		}
		if cur.PlanTaskID != tree.PlanTaskID {
			cur.PlanTaskID = tree.PlanTaskID
			if _, err := uc.requests.Update(ctx, cur, cur.Version); err != nil {
				return err
			}
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectPlanGenerated, planGeneratedPayload{
			RequestID: req.ID, PlanTaskID: tree.PlanTaskID, PhaseTaskIDs: tree.PhaseTaskIDs, TaskIDs: tree.TaskIDs,
			Type: string(req.Type), Size: string(req.Size), TaskCount: len(tree.TaskIDs), Replan: replan,
		})
		if err != nil {
			return err
		}
		if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
		from := domain.RequestStatusPlanning
		if _, err := uc.transition.Execute(ctx, TransitionInput{
			RequestID: req.ID, Trigger: domain.TriggerPlanReady, ExpectedFrom: &from, ActorID: actor, ActorKind: domain.ActorKindUser,
		}); err != nil {
			return err
		}
		// Opened after the transition: an approval records the status it was opened in and is only valid there.
		subject, ok := domain.StartGateSubject(flow.StartGate)
		if !ok {
			return domain.ErrPlanNotApplicable(req.Type)
		}
		_, err = uc.approvals.Execute(ctx, OpenApprovalInput{RequestID: req.ID, SubjectType: subject, SubjectID: tree.PlanTaskID, RequestedBy: actor})
		return err
	})
}
