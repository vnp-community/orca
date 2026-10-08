package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ApprovalAuthorizer decides whether the ctx caller may approve or reject; req is the locked Request.
type ApprovalAuthorizer interface {
	CanDecide(ctx context.Context, req domain.Request, ap domain.Approval) error
}
