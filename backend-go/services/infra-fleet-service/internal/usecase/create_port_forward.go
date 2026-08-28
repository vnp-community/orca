package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreatePortForwardInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type CreatePortForwardInput struct {
	ConnectionID string
	RemotePort   int
}

// CreatePortForward registers a manually-requested local:remote forward —
// allocates a local port (portalloc.Allocator) and persists the record.
// Unlike PollWorkspacePorts' auto-detected forwards, this straightforward
// CRUD path does not itself open an sshconn.Tunnel (see this task's own
// scope note) — it records the binding an operator/future caller can act on.
type CreatePortForward struct {
	repo  PortForwardRepository
	alloc PortAllocator
}

func NewCreatePortForward(repo PortForwardRepository, alloc PortAllocator) *CreatePortForward {
	return &CreatePortForward{repo: repo, alloc: alloc}
}

func (uc *CreatePortForward) Execute(ctx context.Context, in CreatePortForwardInput) (domain.PortForward, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.PortForward{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	id := uuid.NewString()
	localPort, err := uc.alloc.Allocate(id)
	if err != nil {
		return domain.PortForward{}, apperrors.New(apperrors.KindInternal, "INFRA_PORT_ALLOC_FAILED", "failed to allocate a local port", err)
	}

	pf := domain.PortForward{
		ID:           id,
		TenantID:     tenantID,
		ConnectionID: in.ConnectionID,
		LocalPort:    localPort,
		RemotePort:   in.RemotePort,
		Status:       domain.PortForwardStatusActive,
	}
	saved, err := uc.repo.Create(ctx, pf)
	if err != nil {
		uc.alloc.Release(localPort)
		return domain.PortForward{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_PORT_FORWARD_FAILED", "failed to create port forward", err)
	}
	return saved, nil
}
