package usecase

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestWriteAuthorizer says who may record checks on a Request. The rule for "who may write" is still open
// (CR-REQ-010); the port keeps it out of the use case.
type RequestWriteAuthorizer interface {
	CanWrite(ctx context.Context, req domain.Request) error
}

type RecordRequestCheckInput struct {
	RequestID   string
	Kind        string
	Status      string
	MetricsJSON string
	Summary     string
	TaskID      string
}

// RecordRequestCheck appends a measurement. It never updates or deletes: the newest row per kind is the one that counts.
// The source is decided from who is calling, never from the payload, and orca_verified cannot be recorded here.
type RecordRequestCheck struct {
	Requests   RequestReader
	Checks     RequestCheckRepository
	Authorizer RequestWriteAuthorizer
}

func (uc *RecordRequestCheck) Execute(ctx context.Context, in RecordRequestCheckInput) (domain.RequestCheck, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.RequestCheck{}, domain.ErrRequestTenantRequired()
	}
	kind, status := domain.CheckKind(in.Kind), domain.CheckStatus(in.Status)
	if !kind.Valid() {
		return domain.RequestCheck{}, domain.ErrCheckInvalidMetrics("unknown check kind " + in.Kind)
	}
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return domain.RequestCheck{}, err
	}
	if err := uc.Authorizer.CanWrite(ctx, req); err != nil {
		return domain.RequestCheck{}, err
	}
	if !domain.CheckAllowedNow(kind, req.Type, req.Status) {
		return domain.RequestCheck{}, domain.ErrCheckNotAllowedNow(kind, req.Status, req.Type)
	}
	metrics := json.RawMessage(in.MetricsJSON)
	if len(metrics) == 0 {
		metrics = json.RawMessage(`{}`)
	}
	if err := domain.ValidateCheckSubmission(kind, status, metrics, in.Summary); err != nil {
		return domain.RequestCheck{}, err
	}
	check := domain.RequestCheck{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, Kind: kind, Status: status, Metrics: metrics,
		Summary: in.Summary, Source: domain.CheckSourceManual, TaskID: in.TaskID,
	}
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		// An agent acts through a user's session; the record names the agent, not the user, so nobody is blamed for its numbers.
		check.Source = domain.CheckSourceAgent
	} else {
		check.RecordedBy, _ = tenant.UserID(ctx)
	}
	return uc.Checks.Append(ctx, check)
}

// ListRequestChecks returns every row oldest first with the newest of each kind flagged Effective.
type ListRequestChecks struct {
	Requests   RequestReader
	Checks     RequestCheckRepository
	Visibility RequestVisibility
}

func (uc *ListRequestChecks) Execute(ctx context.Context, requestID string, kind string) ([]domain.RequestCheck, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := requireVisible(ctx, uc.Visibility, req); err != nil {
		return nil, err
	}
	checks, err := uc.Checks.ListByRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	checks = domain.MarkEffective(checks)
	if kind == "" {
		return checks, nil
	}
	out := checks[:0:0]
	for _, c := range checks {
		if string(c.Kind) == kind {
			out = append(out, c)
		}
	}
	return out, nil
}

// requireVisible reports the Request as not found to a caller who may not see it, so existence does not leak.
func requireVisible(ctx context.Context, v RequestVisibility, req domain.Request) error {
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	visible, err := v.Filter(ctx, domain.DecisionActor{UserID: userID, Role: role, IsMachine: tenant.ActorType(ctx) == tenant.ActorAgent}, []domain.Request{req})
	if err != nil {
		return err
	}
	if len(visible) == 0 {
		return domain.ErrRequestNotFound(req.ID)
	}
	return nil
}

// ParticipantWriteAuthorizer allows the reporter, an admin, or a user who approved a gate of the Request.
type ParticipantWriteAuthorizer struct {
	Approvals ApprovalGateReader
}

var _ RequestWriteAuthorizer = (*ParticipantWriteAuthorizer)(nil)

func (a *ParticipantWriteAuthorizer) CanWrite(ctx context.Context, req domain.Request) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	if userID == "" {
		return domain.ErrCheckForbidden()
	}
	if userID == req.ReporterID || role == "admin" {
		return nil
	}
	approvals, err := a.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return err
	}
	for _, ap := range approvals {
		if ap.DecidedBy != nil && *ap.DecidedBy == userID {
			return nil
		}
	}
	return domain.ErrCheckForbidden()
}
