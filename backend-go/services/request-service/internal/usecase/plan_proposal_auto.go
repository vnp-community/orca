package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// planAutoReader fills the auto-plan block of a Plan read: whether Orca would plan this project by itself, and the
// latest job.
type planAutoReader struct {
	jobs PlanAutoJobRepository
	sw   PlanAutoSwitch
}

// WithAutoPlan lets the Plan read carry the automatic generation state. Pass a switch that is off when the feature is
// not running, so a screen learns to use the manual flow.
func (uc *GetPlanProposal) WithAutoPlan(jobs PlanAutoJobRepository, sw PlanAutoSwitch) *GetPlanProposal {
	uc.auto = &planAutoReader{jobs: jobs, sw: sw}
	return uc
}

// read returns nil when the reader is not configured. A failed settings read reports "not enabled": a screen must never
// believe in automation it cannot confirm.
func (r *planAutoReader) read(ctx context.Context, req domain.Request) (*PlanAutoView, error) {
	if r == nil {
		return nil, nil
	}
	enabled, err := r.sw.Enabled(ctx, req.ProjectID)
	if err != nil {
		slog.WarnContext(ctx, "plan auto-generation switch unreadable", slog.String("request_id", req.ID), slog.Any("error", err))
		enabled = false
	}
	view := PlanAutoView{Enabled: enabled}
	if r.jobs == nil {
		return &view, nil
	}
	job, found, err := r.jobs.LatestByRequest(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	// The outcome stays readable after the commit moved the Request on (done at awaiting_plan_approval). A finished job
	// next to a Plan of a Request that is planning again is history of an earlier round, not news about this one.
	if !found || (job.State.IsFinal() && req.Status == domain.RequestStatusPlanning && req.PlanTaskID != "") {
		return &view, nil
	}
	view = planAutoViewOf(job, enabled)
	return &view, nil
}
