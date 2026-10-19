package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// FixTaskWriter creates the single task of a hotfix in task-service.
type FixTaskWriter interface {
	CreateFixTask(ctx context.Context, in PlanTreeInput) (taskID string, err error)
}

// HotfixFixTaskResult says which task is the hotfix's fix task and whether this call made it.
type HotfixFixTaskResult struct {
	TaskID string
	// Created is false when an equivalent task from an earlier attempt was reused.
	Created bool
}

// EnsureHotfixFixTask gives a hotfix its one fix task while the Request is still analyzing: the hotfix flow has no
// Plan stage (CR-REQ-012 section 2.6), so the diagnosis result is the only input. It is safe to repeat: a retried
// or re-run analysis finds the task made before instead of making a second one.
type EnsureHotfixFixTask struct {
	tasks    TaskClient
	writer   FixTaskWriter
	policies PlanPolicies
	limits   domain.PlanLimits
}

func NewEnsureHotfixFixTask(tasks TaskClient, writer FixTaskWriter, policies PlanPolicies, limits domain.PlanLimits) *EnsureHotfixFixTask {
	return &EnsureHotfixFixTask{tasks: tasks, writer: writer, policies: policies, limits: limits}
}

// AppliesTo reports whether the Request's flow gives it a single fix task instead of a Plan.
func (uc *EnsureHotfixFixTask) AppliesTo(req domain.Request) bool {
	flow, err := domain.FlowFor(req.Type)
	return err == nil && domain.PlanShapeFor(flow, req.Size) == domain.ShapeSingleTask
}

// Ensure validates the task derived from the diagnosis exactly like a committed Plan, then reuses or creates it.
func (uc *EnsureHotfixFixTask) Ensure(ctx context.Context, req domain.Request, diagnosis []byte) (HotfixFixTaskResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return HotfixFixTaskResult{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" {
		return HotfixFixTaskResult{}, domain.ErrRequestReporterRequired() // the creator gets the owner grant on the task
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return HotfixFixTaskResult{}, domain.ErrPlanNotApplicable(req.Type)
	}
	shape := domain.PlanShapeFor(flow, req.Size)
	if shape != domain.ShapeSingleTask {
		return HotfixFixTaskResult{}, domain.ErrPlanNotApplicable(req.Type)
	}
	diag, err := domain.ParseDiagnosisDocument(diagnosis)
	if err == nil {
		err = diag.Validate()
	}
	if err != nil {
		return HotfixFixTaskResult{}, err
	}
	proposal := domain.NormalizePlanProposal(domain.HotfixFixProposal(req, diag, uc.limits.MaxTitleRunes))
	if err := domain.ValidatePlanProposal(req.Type, shape, uc.limits, proposal); err != nil {
		return HotfixFixTaskResult{}, err
	}
	if err := uc.policies.PolicyFor(req.Type).PlanPreconditions(ctx, req, proposal); err != nil {
		return HotfixFixTaskResult{}, err
	}
	stored, err := EncodePlanProposal(proposal)
	if err != nil {
		return HotfixFixTaskResult{}, err
	}

	existing, err := uc.tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return HotfixFixTaskResult{}, domain.ErrHotfixFixTaskUnavailable(err)
	}
	want := proposal.Tasks[0]
	var keep string
	var stale []string
	for _, t := range existing {
		if !t.IsWorkingTask() || t.ParentID != "" || t.Status == domain.TaskStatusCancelled {
			continue
		}
		switch {
		case t.Status != domain.TaskStatusOpen:
			return HotfixFixTaskResult{TaskID: t.ID}, nil // already started or finished: never rewrite work in flight
		case keep == "" && t.Title == want.Title && t.Description == want.Description:
			keep = t.ID
		default:
			stale = append(stale, t.ID) // left by an earlier diagnosis that has since changed
		}
	}
	// Retired before the new one is made, so a failure in between leaves no second fix task for the approver to see.
	for _, id := range stale {
		if err := uc.tasks.SetStatus(ctx, id, domain.TaskStatusCancelled); err != nil {
			return HotfixFixTaskResult{}, domain.ErrHotfixFixTaskUnavailable(err)
		}
	}
	if keep != "" {
		return HotfixFixTaskResult{TaskID: keep}, nil
	}
	id, err := uc.writer.CreateFixTask(ctx, PlanTreeInput{
		Request: req, Shape: shape, Proposal: proposal, AIPlanJSON: stored, CreatorID: actor,
	})
	if err != nil {
		return HotfixFixTaskResult{}, err
	}
	return HotfixFixTaskResult{TaskID: id, Created: true}, nil
}

// Discard cancels a fix task this analysis created when the analysis then failed to commit, so a Request that
// moved on does not keep an orphan task. Best effort: the next Ensure retires any leftover as stale anyway.
func (uc *EnsureHotfixFixTask) Discard(ctx context.Context, taskID string) {
	if err := uc.tasks.SetStatus(ctx, taskID, domain.TaskStatusCancelled); err != nil {
		slog.WarnContext(ctx, "could not cancel the hotfix fix task of a failed analysis", slog.String("task_id", taskID), slog.Any("error", err))
	}
}
