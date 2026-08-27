package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateConnectionInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type CreateConnectionInput struct {
	DevServerID string
	RepoPath    string
	WorktreeID  string
}

// CreateConnection is the write path for infra.connections
// (migrations/0002_connections) — the real routing model ResolveConnection
// resolves against. Without this, connections would be schema nobody
// writes to; this is what turns a worktree bind into something
// ResolveConnection/Relay can later look up by connectionId.
type CreateConnection struct {
	repo ConnectionRepository
}

func NewCreateConnection(repo ConnectionRepository) *CreateConnection {
	return &CreateConnection{repo: repo}
}

func (uc *CreateConnection) Execute(ctx context.Context, in CreateConnectionInput) (domain.Connection, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	conn, err := domain.NewConnection(uuid.NewString(), tenantID, in.DevServerID, in.RepoPath, in.WorktreeID)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_CONNECTION", err.Error(), err)
	}

	saved, err := uc.repo.CreateConnection(ctx, conn)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_CONNECTION_FAILED", "failed to create connection", err)
	}
	return saved, nil
}
