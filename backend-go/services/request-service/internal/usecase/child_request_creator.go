package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ChildRequestPayload is the wire shape of orca.request.request.created for a spawned child
// (parent_request_id and link_reason are additive to the plain creation event).
type ChildRequestPayload struct {
	RequestID       string `json:"request_id"`
	ProjectID       string `json:"project_id"`
	Number          int64  `json:"number"`
	SourceProvider  string `json:"source_provider"`
	ReporterID      string `json:"reporter_id"`
	ParentRequestID string `json:"parent_request_id"`
	LinkReason      string `json:"link_reason"`
	TypeHint        string `json:"type_hint,omitempty"`
}

// IdempotentChildCreator creates a child in the caller's transaction. It is the default
// ChildRequestCreator until CR-REQ-004's CreateWithinTx can take over behind the same port.
type IdempotentChildCreator struct {
	repo        RequestRepository
	idempotency RequestIdempotencyRepository
	outbox      OutboxWriter
}

var _ ChildRequestCreator = (*IdempotentChildCreator)(nil)

func NewIdempotentChildCreator(repo RequestRepository, idempotency RequestIdempotencyRepository, outbox OutboxWriter) *IdempotentChildCreator {
	return &IdempotentChildCreator{repo: repo, idempotency: idempotency, outbox: outbox}
}

// CreateChild must run inside InTx. The claim goes first so a losing concurrent caller
// burns no request number and blocks until the winner commits.
func (c *IdempotentChildCreator) CreateChild(ctx context.Context, in ChildRequestInput) (ChildRequestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ChildRequestResult{}, domain.ErrRequestTenantRequired()
	}
	id := uuid.NewString()
	existing, claimed, err := c.idempotency.Claim(ctx, string(in.Provider), "user:"+in.ReporterID, in.IdempotencyKey, id)
	if err != nil {
		return ChildRequestResult{}, err
	}
	if !claimed {
		prior, err := c.repo.Get(ctx, existing)
		if err != nil {
			return ChildRequestResult{}, err
		}
		return ChildRequestResult{Request: prior, Created: false}, nil
	}
	r, err := domain.NewRequest(domain.NewRequestInput{
		TenantID: tenantID, ProjectID: in.ProjectID, Title: in.Title, Body: in.Body,
		SourceProvider: string(in.Provider), ReporterID: in.ReporterID,
	})
	if err != nil {
		return ChildRequestResult{}, err
	}
	r.ID = id
	if r.Number, err = c.repo.NextNumber(ctx); err != nil {
		return ChildRequestResult{}, err
	}
	if err := c.repo.Create(ctx, r); err != nil {
		return ChildRequestResult{}, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectRequestCreated, ChildRequestPayload{
		RequestID: r.ID, ProjectID: r.ProjectID, Number: r.Number, SourceProvider: string(in.Provider), ReporterID: in.ReporterID,
		ParentRequestID: in.ParentRequestID, LinkReason: string(in.LinkReason), TypeHint: string(in.TypeHint),
	})
	if err != nil {
		return ChildRequestResult{}, err
	}
	if err := c.outbox.InsertOutboxEvent(ctx, ev); err != nil {
		return ChildRequestResult{}, err
	}
	return ChildRequestResult{Request: r, Created: true}, nil
}
