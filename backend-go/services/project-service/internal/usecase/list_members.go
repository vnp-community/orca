package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListMembersInput struct {
	ProjectID string
}

// ListMembers requires only membership (member or owner) — any-member tier,
// project-service.md §9's "ListMembers ... require[s] any membership."
type ListMembers struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewListMembers(repo ProjectRepository, opa OPAClient) *ListMembers {
	return &ListMembers{repo: repo, opa: opa}
}

func (uc *ListMembers) Execute(ctx context.Context, in ListMembersInput) ([]domain.ProjectMember, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}
	members, err := uc.repo.ListMembers(ctx, in.ProjectID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_MEMBERS_FAILED", "failed to list project members", err)
	}
	return members, nil
}
