package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateSshTargetInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type CreateSshTargetInput struct {
	Host                  string
	Port                  int
	UserName              string
	VaultSSHRole          string
	KnownHostsFingerprint string
	JumpHostTargetID      string
}

// CreateSshTarget registers a new SSH target — host/user plus a pointer
// into Vault's SSH secrets engine role, never raw key material (see
// domain.NewSshTarget's invariant).
type CreateSshTarget struct {
	repo SshTargetRepository
}

func NewCreateSshTarget(repo SshTargetRepository) *CreateSshTarget {
	return &CreateSshTarget{repo: repo}
}

func (uc *CreateSshTarget) Execute(ctx context.Context, in CreateSshTargetInput) (domain.SshTarget, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	target, err := domain.NewSshTarget(uuid.NewString(), tenantID, in.Host, in.Port, in.UserName, in.VaultSSHRole, in.KnownHostsFingerprint, in.JumpHostTargetID)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_SSH_TARGET", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, target)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_SSH_TARGET_FAILED", "failed to create ssh target", err)
	}
	return saved, nil
}
