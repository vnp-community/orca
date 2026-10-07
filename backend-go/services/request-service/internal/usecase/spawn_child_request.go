package usecase

import "context"

type SpawnChildRequest struct {}

func (uc *SpawnChildRequest) Execute(ctx context.Context, parentID string) (string, error) {
	return "", nil
}
