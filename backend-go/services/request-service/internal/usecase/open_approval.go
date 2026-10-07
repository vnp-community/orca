package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var (
	ErrSubjectTypeNotAllowed = errors.New("subject type not allowed for this request")
)

type OpenApprovalInput struct {
	RequestID      string
	SubjectType    domain.SubjectType
	IdempotencyKey *string
	Request        domain.Request // fake Request object for now
}

type OpenApproval struct {
	Repo         ApprovalRepository
	Tx           TxRunner
	Registry     *SubjectHandlerRegistry
	Resolver     *ResolveApproverPolicy
	ApproverRepo ApprovalApproverRepository
	Outbox       OutboxWriter
}

func (uc *OpenApproval) Execute(ctx context.Context, txCtx context.Context, in OpenApprovalInput) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if !in.SubjectType.Valid() {
		return nil, domain.ErrApprovalSubjectTypeInvalid
	}

	// Fake FlowFor logic: assuming all are allowed for this dummy implementation
	// In real logic: FlowFor(req.Type) allowed checks

	handler := uc.Registry.Get(in.SubjectType)
	if handler == nil {
		return nil, errors.New("no handler registered for subject type")
	}

	subjectID, digest, err := handler.ValidateForRequest(ctx, txCtx, in.Request, in.SubjectType)
	if err != nil {
		return nil, err
	}

	policy, err := uc.Resolver.Resolve(ctx, in.Request, in.SubjectType)
	if err != nil {
		return nil, err
	}

	userID, _ := tenant.UserID(ctx)
	expandedUsers := []string{userID} // Stub team resolver expansion
	eligible := domain.EligibleApprovers(policy.Approvers, in.Request.ReporterID, policy.AllowRequesterApprove, expandedUsers)
	if len(eligible) == 0 {
		return nil, errors.New("REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER")
	}

	var dueAt *time.Time
	if policy.DueAfter != nil {
		t := time.Now().UTC().Add(*policy.DueAfter) // Stub NowDB
		dueAt = &t
	}

	now := time.Now().UTC()
	a := domain.Approval{
		ID:                  uuid.NewString(),
		TenantID:            tenantID,
		RequestID:           in.Request.ID,
		SubjectType:         in.SubjectType,
		SubjectID:           subjectID,
		Stage:               in.Request.Stage,
		Status:              domain.ApprovalStatusPending,
		RequestedBy:         userID,
		DueAt:               dueAt,
		Version:             1,
		SubjectDigest:       digest,
		SelfApprovalAllowed: policy.AllowRequesterApprove,
		IdempotencyKey:      in.IdempotencyKey,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	err = uc.Tx.InTx(txCtx, func(txCtx context.Context) error {
		if insertErr := uc.Repo.Insert(txCtx, a); insertErr != nil {
			if errors.Is(insertErr, ErrPendingExists) {
				existing, findErr := uc.Repo.FindPendingBySubject(txCtx, tenantID, in.SubjectType, subjectID)
				if findErr != nil {
					return findErr
				}
				if existing != nil && existing.SubjectDigest == digest {
					// idempotent return
					a = *existing
					return nil
				}
				return ErrPendingExists
			}
			return insertErr
		}

		if err := uc.ApproverRepo.InsertSnapshot(txCtx, a.ID, tenantID, policy.Approvers); err != nil {
			return err
		}

		ev, outErr := NewOutboxEvent(ctx, "orca.request.approval.requested", map[string]any{
			"approval_id":  a.ID,
			"request_id":   a.RequestID,
			"subject_type": a.SubjectType,
			"subject_id":   a.SubjectID,
			"stage":        a.Stage,
			"requested_by": a.RequestedBy,
			"due_at":       a.DueAt,
		})
		if outErr != nil {
			return outErr
		}
		return uc.Outbox.InsertOutboxEvent(txCtx, ev)
	})

	if err != nil {
		return nil, err
	}
	return &a, nil
}
