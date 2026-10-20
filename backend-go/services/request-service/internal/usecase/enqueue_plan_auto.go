package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanAutoEvent is what the consumer reads from a status_changed event.
type PlanAutoEvent struct {
	ID        string
	TenantID  string
	RequestID string
	ProjectID string
	To        string
	Trigger   string
}

// EnqueuePlanAuto turns the status_changed event that puts a Request in planning into a stored job (CR-REQ-044). It only
// inserts: the worker does everything else, so there is one execution path and a redelivered event costs nothing.
type EnqueuePlanAuto struct {
	jobs      PlanAutoJobRepository
	processed ProcessedEventRepository
	requests  RequestRepository
	solutions SolutionStore
	sw        PlanAutoSwitch
	policy    domain.PlanAutoPolicy
	tx        TxRunner
	now       func() time.Time
}

func NewEnqueuePlanAuto(jobs PlanAutoJobRepository, processed ProcessedEventRepository, requests RequestRepository, solutions SolutionStore, sw PlanAutoSwitch, policy domain.PlanAutoPolicy, tx TxRunner) *EnqueuePlanAuto {
	return &EnqueuePlanAuto{jobs: jobs, processed: processed, requests: requests, solutions: solutions, sw: sw, policy: policy, tx: tx,
		now: func() time.Time { return time.Now().UTC() }}
}

// Handle returns an error only when redelivery can help (database, settings read); a business reason to do nothing is
// a normal return. The processed mark rolls back with the transaction, so a failed attempt is retried whole.
func (uc *EnqueuePlanAuto) Handle(ctx context.Context, ev PlanAutoEvent) error {
	if ev.RequestID == "" || domain.RequestStatus(ev.To) != domain.RequestStatusPlanning {
		return nil
	}
	trigger := domain.Trigger(ev.Trigger)
	if trigger != domain.TriggerAnalysisApproved && trigger != domain.TriggerTypeConfirmed {
		return nil
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	return uc.tx.InTx(ctx, func(txCtx context.Context) error {
		already, err := uc.processed.MarkProcessed(txCtx, ev.ID, domain.SubjectRequestStatusChanged)
		if err != nil || already {
			return err
		}
		req, err := uc.requests.Get(txCtx, ev.RequestID)
		if isNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		on, err := uc.sw.Enabled(txCtx, req.ProjectID)
		if err != nil {
			return err
		}
		if !on {
			slog.DebugContext(txCtx, "plan auto-generation off", slog.String("request_id", req.ID))
			return nil
		}
		if req.Status != domain.RequestStatusPlanning {
			return nil // a person moved the Request on while the event was in flight
		}
		flow, err := domain.FlowFor(req.Type)
		if err != nil || !domain.WantsAutoPlan(req, flow, trigger, domain.RequestStatus(ev.To)) {
			slog.DebugContext(txCtx, "plan auto-generation not wanted", slog.String("request_id", req.ID), slog.String("type", string(req.Type)), slog.String("trigger", ev.Trigger))
			return nil
		}
		anchor, ok, err := uc.anchorFor(txCtx, req, flow, trigger, ev.ID)
		if err != nil {
			return err
		}
		if !ok {
			slog.WarnContext(txCtx, "plan auto-generation skipped: no approved analysis", slog.String("request_id", req.ID))
			return nil
		}
		now := uc.now()
		inserted, err := uc.jobs.Insert(txCtx, domain.PlanAutoJob{
			TenantID: tenantID, RequestID: req.ID, Anchor: anchor, Origin: domain.PlanAutoOriginEvent, ProjectID: req.ProjectID,
			State: domain.PlanAutoPending, Attempt: 1, MaxAttempts: uc.policy.MaxAttempts(), NextCheckAt: now, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if !inserted {
			slog.InfoContext(txCtx, "plan auto-generation job already exists", slog.String("request_id", req.ID))
		}
		return nil
	})
}

// anchorFor reads the Solutions itself rather than through latestApprovedAnalysis, which hides a read failure as "none":
// here a failure must be redelivered, not turned into a processed event without a job.
func (uc *EnqueuePlanAuto) anchorFor(ctx context.Context, req domain.Request, flow domain.FlowDefinition, trigger domain.Trigger, eventID string) (string, bool, error) {
	if trigger == domain.TriggerTypeConfirmed {
		return domain.TypeConfirmAnchor(eventID), true, nil
	}
	if flow.AnalysisKind == domain.AnalysisNone {
		return "", false, nil
	}
	all, err := uc.solutions.ListByRequest(ctx, SolutionListFilter{RequestID: req.ID, Status: domain.SolutionStatusApproved})
	if err != nil {
		return "", false, err
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Kind == domain.SolutionKindSolution || all[i].Kind == domain.SolutionKindDiagnosis {
			return domain.SolutionAnchor(all[i].ID), true, nil
		}
	}
	return "", false, nil
}
