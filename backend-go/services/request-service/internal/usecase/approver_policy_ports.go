package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ApproverDecision struct {
	DueAt               *time.Time
	SelfApprovalAllowed bool
}

type ApproverPolicy interface {
	Resolve(ctx context.Context, req domain.Request, st domain.SubjectType) (ApproverDecision, error)
}

type ApprovalAuthorizer interface {
	CanDecide(ctx context.Context, req domain.Request, ap domain.Approval) error
}

// Temporary implementations removed by BE-REQ-SOL-010
