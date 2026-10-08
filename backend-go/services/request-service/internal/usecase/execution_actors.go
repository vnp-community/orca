package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ApprovalExecutionActors picks the user whose permissions task-service checks when request-service dispatches
// on its own (consumers, reconcile): whoever started the Phase, else whoever approved the Plan.
type ApprovalExecutionActors struct {
	Approvals   ApprovalGateReader
	PhaseStarts PhaseStartRepository
}

var _ ExecutionActors = (*ApprovalExecutionActors)(nil)

func (a *ApprovalExecutionActors) ActorFor(ctx context.Context, req domain.Request, phaseID string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", domain.ErrRequestTenantRequired()
	}
	if phaseID != "" {
		starts, err := a.PhaseStarts.ListByRequest(ctx, req.ID)
		if err != nil {
			return "", err
		}
		for _, s := range starts {
			if s.PhaseTaskID == phaseID {
				return s.StartedBy, nil
			}
		}
	}
	approvals, err := a.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return "", err
	}
	var best string
	var bestAt time.Time
	for _, ap := range approvals {
		if ap.Status != domain.ApprovalStatusApproved || ap.DecidedBy == nil || ap.SubjectType == domain.SubjectPhase {
			continue
		}
		at := ap.CreatedAt
		if ap.DecidedAt != nil {
			at = *ap.DecidedAt
		}
		if best == "" || at.After(bestAt) {
			best, bestAt = *ap.DecidedBy, at
		}
	}
	if best != "" {
		return best, nil
	}
	return req.ReporterID, nil
}

// ApprovalLookupFromGateReader adapts the gate approval reader to the lookup type policies use.
type ApprovalLookupFromGateReader struct {
	Approvals ApprovalGateReader
}

var _ domain.ApprovalLookup = (*ApprovalLookupFromGateReader)(nil)

func (l *ApprovalLookupFromGateReader) LatestApproval(ctx context.Context, requestID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, domain.ErrRequestTenantRequired()
	}
	approvals, err := l.Approvals.ListGateApprovals(ctx, tenantID, []string{requestID})
	if err != nil {
		return nil, err
	}
	return domain.NewApprovalIndex(approvals).GetLatest(st, subjectID), nil
}

// CheckReaderFromRepository adapts the check repository to the reader type policies use.
type CheckReaderFromRepository struct {
	Checks RequestCheckRepository
}

var _ domain.CheckReader = (*CheckReaderFromRepository)(nil)

func (r *CheckReaderFromRepository) LatestCheck(ctx context.Context, requestID string, kind domain.CheckKind) (domain.RequestCheck, bool, error) {
	return r.Checks.Latest(ctx, requestID, kind)
}
