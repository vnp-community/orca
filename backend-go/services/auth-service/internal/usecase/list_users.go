package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type ListUsersInput struct {
	TenantID  string
	PageToken string
	PageSize  int32
}

type ListUsersOutput struct {
	Users         []domain.User
	NextPageToken string
}

// ListUsers is an admin-console operation.
type ListUsers struct {
	users UserRepository
}

func NewListUsers(users UserRepository) *ListUsers {
	return &ListUsers{users: users}
}

func (uc *ListUsers) Execute(ctx context.Context, in ListUsersInput) (ListUsersOutput, error) {
	if _, err := requireAdminActor(ctx, uc.users); err != nil {
		return ListUsersOutput{}, err
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	users, next, err := uc.users.ListUsers(ctx, in.TenantID, in.PageToken, pageSize)
	if err != nil {
		return ListUsersOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_USERS_FAILED", "failed to list users", err)
	}
	return ListUsersOutput{Users: users, NextPageToken: next}, nil
}
