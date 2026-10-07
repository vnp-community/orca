package usecase

import "context"

func BindRepo(ctx context.Context, tenantID, repoID string) error {
	return nil
}

func ListBindings(ctx context.Context, tenantID string) ([]string, error) {
	return nil, nil
}
